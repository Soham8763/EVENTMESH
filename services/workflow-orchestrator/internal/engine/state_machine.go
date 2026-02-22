package engine

import (
	"database/sql"
	"time"

	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/workflow-orchestrator/internal/model"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (e *ExecutionEngine) AdvanceExecution(execID string) error {
	logger.Log.Info("advancing execution", zap.String("execution_id", execID))

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
		CreatedAt:           time.Now().UTC(),
	}

	if err := e.producer.Publish(task.TenantID, task); err != nil {
		return err
	}

	logger.Log.Info("emitted task",
		zap.String("step", stepName),
		zap.String("execution_id", execID))

	return tx.Commit()
}
