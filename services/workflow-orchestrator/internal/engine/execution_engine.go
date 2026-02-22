package engine

import (
	"database/sql"
	"encoding/json"
	"log"

	"github.com/google/uuid"

	"eventmesh/workflow-orchestrator/internal/model"
	"eventmesh/workflow-orchestrator/internal/producer"
)

type ExecutionEngine struct {
	db       *sql.DB
	producer *producer.Producer
}

func NewExecutionEngine(db *sql.DB, p *producer.Producer) *ExecutionEngine {
	return &ExecutionEngine{db: db, producer: p}
}

func (e *ExecutionEngine) HandleTrigger(
	trigger model.WorkflowTriggerEvent,
) error {

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

	log.Printf("engine: created execution id=%s for workflow=%s", execID, trigger.WorkflowName)

	return e.AdvanceExecution(execID)
}

func (e *ExecutionEngine) HandleResult(r model.TaskResult) error {
	log.Printf("engine: handling result for step=%s execution=%s status=%s",
		r.StepName, r.WorkflowExecutionID, r.Status)

	tx, err := e.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

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
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if r.Status == model.StepSuccess {
		return e.AdvanceExecution(r.WorkflowExecutionID)
	}

	return nil
}
