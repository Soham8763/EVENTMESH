package engine

import (
	"context"
	"database/sql"
	"time"

	"eventmesh/internal/events"
	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/workflow-orchestrator/internal/model"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func (e *ExecutionEngine) AdvanceExecution(ctx context.Context, execID string, correlationID string) error {
	tr := otel.Tracer("workflow-orchestrator")
	ctx, span := tr.Start(ctx, "AdvanceExecution")
	defer span.End()

	span.SetAttributes(
		attribute.String("execution_id", execID),
		attribute.String("correlation_id", correlationID),
	)

	tx, err := e.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	var currentStep int
	var tenantID string

	err = tx.QueryRow(`
		SELECT status, current_step, tenant_id
		FROM workflow_executions
		WHERE id=$1
	`, execID).Scan(&status, &currentStep, &tenantID)

	if err != nil {
		return err
	}

	if status == model.WorkflowCompleted ||
		status == model.WorkflowFailed {
		return nil
	}

	// find next pending step
	row := tx.QueryRow(`
		SELECT id, step_name
		FROM workflow_step_executions
		WHERE workflow_execution_id=$1
		  AND status=$2
		ORDER BY step_index
		LIMIT 1
	`, execID, model.StepPending)

	var stepID string
	var stepName string

	err = row.Scan(&stepID, &stepName)

	if err == sql.ErrNoRows {
		// no steps left → workflow complete
		_, err = tx.Exec(`
			UPDATE workflow_executions
			SET status=$1
			WHERE id=$2
		`, model.WorkflowCompleted, execID)

		if err != nil {
			return err
		}

		metrics.WorkflowsCompleted.Inc()

		// Emit WorkflowCompleted event
		e.publisher.Publish(ctx, execID, events.WorkflowCompletedEvent{
			BaseEvent: events.BaseEvent{
				EventID:     uuid.New().String(),
				EventType:   events.WorkflowCompleted,
				ExecutionID: execID,
				Timestamp:   time.Now(),
			},
		})

		return tx.Commit()
	}

	if err != nil {
		return err
	}

	// mark step running
	_, err = tx.Exec(`
		UPDATE workflow_step_executions
		SET status=$1
		WHERE id=$2
	`, model.StepRunning, stepID)

	if err != nil {
		return err
	}

	// mark workflow running
	_, err = tx.Exec(`
		UPDATE workflow_executions
		SET status=$1
		WHERE id=$2
	`, model.WorkflowRunning, execID)

	if err != nil {
		return err
	}

	// emit task to Kafka
	task := model.WorkflowTask{
		TaskID:              uuid.New().String(),
		WorkflowExecutionID: execID,
		StepName:            stepName,
		TenantID:            tenantID,
		CorrelationID:       correlationID,
		CreatedAt:           time.Now().UTC(),
	}

	// Emit task to step-specific Kafka topic
	taskTopic := events.TaskTopic(stepName)
	if err := e.producer.Publish(ctx, execID, task, taskTopic); err != nil {
		return err
	}

	logger.Log.Info("emitted task",
		zap.String("step", stepName),
		zap.String("execution_id", execID),
		zap.String("correlation_id", correlationID))

	// Emit StepScheduled event
	e.publisher.Publish(ctx, execID, events.StepScheduledEvent{
		BaseEvent: events.BaseEvent{
			EventID:     uuid.New().String(),
			EventType:   events.StepScheduled,
			ExecutionID: execID,
			Timestamp:   time.Now(),
		},
		StepName: stepName,
	})

	return tx.Commit()
}
