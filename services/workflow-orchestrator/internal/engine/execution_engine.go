package engine

import (
	"context"
	"database/sql"
	"encoding/json"

	"eventmesh/internal/events"
	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/workflow-orchestrator/internal/model"
	"eventmesh/workflow-orchestrator/internal/producer"

	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type ExecutionEngine struct {
	db              *sql.DB
	producer        *producer.Producer
	failureProducer *producer.FailureProducer
	publisher       *events.EventPublisher
}

func NewExecutionEngine(db *sql.DB, p *producer.Producer, fp *producer.FailureProducer, ep *events.EventPublisher) *ExecutionEngine {
	return &ExecutionEngine{db: db, producer: p, failureProducer: fp, publisher: ep}
}

func (e *ExecutionEngine) HandleTrigger(
	ctx context.Context,
	trigger events.WorkflowTriggerEvent,
) error {
	tr := otel.Tracer("workflow-orchestrator")
	ctx, span := tr.Start(ctx, "HandleTrigger")
	defer span.End()

	span.SetAttributes(
		attribute.String("workflow", trigger.WorkflowName),
		attribute.String("trigger_id", trigger.TriggerID),
		attribute.String("correlation_id", trigger.CorrelationID),
	)

	tx, err := e.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	execID := uuid.New().String()

	// Load workflow definition
	var stepsJSON []byte

	err = tx.QueryRow(`
		SELECT steps
		FROM workflow_definitions
		WHERE name=$1
	`,
		trigger.WorkflowName,
	).Scan(&stepsJSON)

	if err != nil {
		return err
	}

	var steps []map[string]interface{}
	if err := json.Unmarshal(stepsJSON, &steps); err != nil {
		return err
	}

	// Write execution state directly in the orchestrator (source of truth)
	_, err = tx.Exec(`
		INSERT INTO workflow_executions
		(id, workflow_name, status, created_at, tenant_id, trigger_id)
		VALUES ($1, $2, 'RUNNING', $3, $4, $5)
	`, execID, trigger.WorkflowName, time.Now(), trigger.TenantID, trigger.TriggerID)
	if err != nil {
		return err
	}

	// Insert all steps as PENDING
	for i, step := range steps {
		stepName, _ := step["step"].(string)
		_, err := tx.Exec(`
			INSERT INTO workflow_step_executions
			(id, workflow_execution_id, step_name, status, step_index)
			VALUES (gen_random_uuid(), $1, $2, 'PENDING', $3)
		`, execID, stepName, i)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	logger.Log.Info("workflow execution created",
		zap.String("execution_id", execID),
		zap.String("workflow", trigger.WorkflowName))

	// Emit WorkflowStarted event (for state-projector read model and audit trail)
	e.publisher.Publish(ctx, execID, events.WorkflowStartedEvent{
		BaseEvent: events.BaseEvent{
			EventID:     uuid.New().String(),
			EventType:   events.WorkflowStarted,
			ExecutionID: execID,
			Timestamp:   time.Now(),
		},
		WorkflowID: trigger.WorkflowName,
		Steps:      steps,
		TenantID:   trigger.TenantID,
		TriggerID:  trigger.TriggerID,
	})

	metrics.WorkflowExecutions.Inc()
	metrics.StepExecutions.WithLabelValues("workflow", "started").Inc()
	metrics.WorkflowsStarted.Inc()

	// State is guaranteed to be in PostgreSQL — advance immediately
	return e.AdvanceExecution(ctx, execID, trigger.CorrelationID)
}

func (e *ExecutionEngine) HandleResult(ctx context.Context, r model.TaskResult) error {
	tr := otel.Tracer("workflow-orchestrator")
	ctx, span := tr.Start(ctx, "HandleResult")
	defer span.End()

	span.SetAttributes(
		attribute.String("execution_id", r.WorkflowExecutionID),
		attribute.String("step", r.StepName),
		attribute.String("status", r.Status),
		attribute.String("correlation_id", r.CorrelationID),
	)

	logger.Log.Info("handling task result",
		zap.String("step", r.StepName),
		zap.String("execution_id", r.WorkflowExecutionID),
		zap.String("status", r.Status),
		zap.String("correlation_id", r.CorrelationID))

	tx, err := e.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if r.Status == model.StepFailed {
		// Emit StepFailed event for the state-projector
		e.publisher.Publish(ctx, r.WorkflowExecutionID, events.StepFailedEvent{
			BaseEvent: events.BaseEvent{
				EventID:     uuid.New().String(),
				EventType:   events.StepFailed,
				ExecutionID: r.WorkflowExecutionID,
				Timestamp:   time.Now(),
			},
			StepName: r.StepName,
			Error:    safelyGetError(r.Error),
		})

		metrics.StepExecutions.WithLabelValues(r.StepName, r.Status).Inc()

		var retryCount int
		err = tx.QueryRow(`
			SELECT retry_count
			FROM workflow_step_executions
			WHERE workflow_execution_id=$1
			  AND step_name=$2
		`, r.WorkflowExecutionID, r.StepName).Scan(&retryCount)

		if err != nil {
			return err
		}

		if retryCount >= 3 {
			// Max retries exceeded — mark workflow as FAILED
			_, err = tx.Exec(`
				UPDATE workflow_executions
				SET status = $1, updated_at = NOW()
				WHERE id = $2
			`, model.WorkflowFailed, r.WorkflowExecutionID)
			if err != nil {
				return err
			}

			_, err = tx.Exec(`
				UPDATE workflow_step_executions
				SET status = $1, last_error = $2, updated_at = NOW()
				WHERE workflow_execution_id = $3 AND step_name = $4
			`, model.StepFailed, safelyGetError(r.Error), r.WorkflowExecutionID, r.StepName)
			if err != nil {
				return err
			}

			if err := tx.Commit(); err != nil {
				return err
			}

			metrics.WorkflowsFailed.Inc()

			e.failureProducer.EmitFailure(ctx, producer.FailureEvent{
				Type:          "WORKFLOW_FAILED",
				WorkflowID:    r.WorkflowExecutionID,
				StepName:      r.StepName,
				CorrelationID: r.CorrelationID,
				Reason:        "retry_limit_exceeded",
			})

			logger.Log.Error("workflow failed — retry limit exceeded",
				zap.String("step", r.StepName),
				zap.String("execution_id", r.WorkflowExecutionID),
				zap.Int("retries", retryCount))

			return nil
		}

		// Retriable failure — reset step to PENDING and increment retry count
		_, err = tx.Exec(`
			UPDATE workflow_step_executions
			SET status = $1, retry_count = retry_count + 1, last_error = $2, updated_at = NOW()
			WHERE workflow_execution_id = $3 AND step_name = $4
		`, model.StepPending, safelyGetError(r.Error), r.WorkflowExecutionID, r.StepName)
		if err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		metrics.RetryCount.Inc()
		logger.Log.Warn("step failed, retrying",
			zap.String("step", r.StepName),
			zap.String("execution_id", r.WorkflowExecutionID),
			zap.Int("retry_count", retryCount+1),
			zap.String("correlation_id", r.CorrelationID))

		// Re-advance to retry the step
		return e.AdvanceExecution(ctx, r.WorkflowExecutionID, r.CorrelationID)
	}

	// Step succeeded — update status directly
	_, err = tx.Exec(`
		UPDATE workflow_step_executions
		SET status = $1, updated_at = NOW()
		WHERE workflow_execution_id = $2 AND step_name = $3
	`, model.StepSuccess, r.WorkflowExecutionID, r.StepName)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Emit StepCompleted event for the state-projector
	e.publisher.Publish(ctx, r.WorkflowExecutionID, events.StepCompletedEvent{
		BaseEvent: events.BaseEvent{
			EventID:     uuid.New().String(),
			EventType:   events.StepCompleted,
			ExecutionID: r.WorkflowExecutionID,
			Timestamp:   time.Now(),
		},
		StepName: r.StepName,
		Result:   r.Status,
	})

	metrics.StepExecutions.WithLabelValues(r.StepName, r.Status).Inc()

	// Advance to the next step
	return e.AdvanceExecution(ctx, r.WorkflowExecutionID, r.CorrelationID)
}

func (e *ExecutionEngine) HandleWorkflowDefined(ctx context.Context, event events.WorkflowDefinedEvent) error {
	stepsJSON, err := json.Marshal(event.Steps)
	if err != nil {
		return err
	}

	_, err = e.db.Exec(`
		INSERT INTO workflow_definitions (name, steps, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (name) DO UPDATE SET steps = $2
	`, event.WorkflowName, stepsJSON, event.Timestamp)

	if err != nil {
		logger.Log.Error("failed to register workflow", zap.Error(err), zap.String("workflow", event.WorkflowName))
		return err
	}

	logger.Log.Info("workflow registered", zap.String("workflow", event.WorkflowName))
	return nil
}

func safelyGetError(err *string) string {
	if err == nil {
		return ""
	}
	return *err
}
