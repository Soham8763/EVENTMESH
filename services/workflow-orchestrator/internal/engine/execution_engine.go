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
