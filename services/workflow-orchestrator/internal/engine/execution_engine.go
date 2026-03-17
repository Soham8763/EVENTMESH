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
	trigger model.WorkflowTriggerEvent,
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

	// Insert execution
	_, err = tx.Exec(`
		INSERT INTO workflow_executions
		(id, tenant_id, workflow_name, trigger_id, status)
		VALUES ($1,$2,$3,$4,'CREATED')
	`,
		execID,
		trigger.TenantID,
		trigger.WorkflowName,
		trigger.TriggerID,
	)
	if err != nil {
		return err
	}

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

	// Insert step executions
	for i, s := range steps {

		stepID := uuid.New().String()

		_, err := tx.Exec(`
			INSERT INTO workflow_step_executions
			(id, workflow_execution_id, step_name, status, step_index)
			VALUES ($1,$2,$3,'PENDING',$4)
		`,
			stepID,
			execID,
			s["step"],
			i,
		)
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

	// Emit WorkflowStarted event
	e.publisher.Publish(ctx, execID, events.WorkflowStartedEvent{
		BaseEvent: events.BaseEvent{
			EventID:     uuid.New().String(),
			EventType:   events.WorkflowStarted,
			ExecutionID: execID,
			Timestamp:   time.Now(),
		},
		WorkflowID: trigger.WorkflowName,
	})

	metrics.WorkflowsStarted.Inc()

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

	// Emit Step Result event
	eventType := events.StepCompleted
	if r.Status == model.StepFailed {
		eventType = events.StepFailed
	}

	if eventType == events.StepCompleted {
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
	} else {
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
	}

	// update step status
	_, err = tx.Exec(`
		UPDATE workflow_step_executions
		SET status=$1, last_error=$2
		WHERE workflow_execution_id=$3
		  AND step_name=$4
	`,
		r.Status,
		r.Error,
		r.WorkflowExecutionID,
		r.StepName,
	)
	if err != nil {
		return err
	}

	if r.Status == model.StepFailed {

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

			// fail workflow
			_, err = tx.Exec(`
				UPDATE workflow_executions
				SET status=$1
				WHERE id=$2
			`, model.WorkflowFailed, r.WorkflowExecutionID)

			if err != nil {
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

			return tx.Commit()
		}

		// increment retry
		_, err = tx.Exec(`
			UPDATE workflow_step_executions
			SET retry_count = retry_count + 1,
			    status=$1
			WHERE workflow_execution_id=$2
			  AND step_name=$3
		`,
			model.StepPending,
			r.WorkflowExecutionID,
			r.StepName,
		)

		if err != nil {
			return err
		}

		metrics.RetryCount.Inc()
		logger.Log.Warn("step retried",
			zap.String("step", r.StepName),
			zap.String("execution_id", r.WorkflowExecutionID),
			zap.String("correlation_id", r.CorrelationID))
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if r.Status == model.StepSuccess {
		return e.AdvanceExecution(ctx, r.WorkflowExecutionID, r.CorrelationID)
	}

	return nil
}

func safelyGetError(err *string) string {
	if err == nil {
		return ""
	}
	return *err
}
