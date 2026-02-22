package engine

import (
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"

	"eventmesh/workflow-orchestrator/internal/model"
)

type ExecutionEngine struct {
	db *sql.DB
}

func NewExecutionEngine(db *sql.DB) *ExecutionEngine {
	return &ExecutionEngine{db: db}
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
	for _, s := range steps {

		stepID := uuid.New().String()

		_, err := tx.Exec(`
			INSERT INTO workflow_step_executions
			(id, workflow_execution_id, step_name, status)
			VALUES ($1,$2,$3,'PENDING')
		`,
			stepID,
			execID,
			s["step"],
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
