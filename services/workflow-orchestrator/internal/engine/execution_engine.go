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


	if err := tx.Commit(); err != nil {
		return err
	}

	logger.Log.Info("workflow execution created (events only)",
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
		Steps:      steps,
		TenantID:   trigger.TenantID,
		TriggerID:  trigger.TriggerID,
	})

	metrics.WorkflowsStarted.Inc()

	// Small delay to allow state-projector to populate the read-model
	// In a real pure event processor, Advance would be triggered by a consumer
	time.Sleep(100 * time.Millisecond)

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
