package internal

import (
	"encoding/json"
	"log"

	"eventmesh/internal/events"
)

func (p *Projector) handleEvent(data []byte) {
	var base events.BaseEvent
	err := json.Unmarshal(data, &base)
	if err != nil {
		log.Println("event decode error:", err)
		return
	}

	switch base.EventType {
	case events.WorkflowStarted:
		p.handleWorkflowStarted(data)
	case events.StepScheduled:
		p.handleStepScheduled(data)
	case events.StepCompleted:
		p.handleStepCompleted(data)
	case events.WorkflowCompleted:
		p.handleWorkflowCompleted(data)
	}
}

func (p *Projector) handleWorkflowStarted(data []byte) {
	var event events.WorkflowStartedEvent
	json.Unmarshal(data, &event)

	tx, err := p.db.Begin()
	if err != nil {
		log.Println("transaction error:", err)
		return
	}
	defer tx.Rollback()

	// Insert execution
	_, err = tx.Exec(`
        INSERT INTO workflow_executions
        (id, workflow_name, status, created_at, tenant_id, trigger_id)
        VALUES ($1, $2, 'RUNNING', $3, $4, $5)
        ON CONFLICT (id) DO UPDATE SET status = 'RUNNING'
    `,
		event.ExecutionID,
		event.WorkflowID,
		event.Timestamp,
		event.TenantID,
		event.TriggerID,
	)

	if err != nil {
		log.Println("workflow insert error:", err)
		return
	}

	// Insert all steps as PENDING
	for i, s := range event.Steps {
		_, err := tx.Exec(`
            INSERT INTO workflow_step_executions
            (id, workflow_execution_id, step_name, status, step_index)
            VALUES (gen_random_uuid(), $1, $2, 'PENDING', $3)
            ON CONFLICT (workflow_execution_id, step_name) DO NOTHING
        `,
			event.ExecutionID,
			s["step"],
			i,
		)
		if err != nil {
			log.Println("step projection error:", err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Println("commit error:", err)
	}
}

func (p *Projector) handleStepScheduled(data []byte) {
	var event events.StepScheduledEvent
	json.Unmarshal(data, &event)

	_, err := p.db.Exec(`
        INSERT INTO workflow_step_executions
        (id, workflow_execution_id, step_name, status)
        VALUES (gen_random_uuid(), $1, $2, 'RUNNING')
        ON CONFLICT (workflow_execution_id, step_name)
        DO UPDATE SET status = 'RUNNING', updated_at = NOW()
    `,
		event.ExecutionID,
		event.StepName,
	)

	if err != nil {
		log.Println("step insert/update error:", err)
	}
}

func (p *Projector) handleStepCompleted(data []byte) {
	var event events.StepCompletedEvent
	json.Unmarshal(data, &event)

	_, err := p.db.Exec(`
        UPDATE workflow_step_executions
        SET status = 'SUCCESS', updated_at = NOW()
        WHERE workflow_execution_id = $1 AND step_name = $2
    `,
		event.ExecutionID,
		event.StepName,
	)

	if err != nil {
		log.Println("step update error:", err)
	}
}

func (p *Projector) handleWorkflowCompleted(data []byte) {
	var event events.WorkflowCompletedEvent
	json.Unmarshal(data, &event)

	_, err := p.db.Exec(`
        UPDATE workflow_executions
        SET status = 'COMPLETED', updated_at = NOW()
        WHERE id = $1
    `,
		event.ExecutionID,
	)

	if err != nil {
		log.Println("workflow update error:", err)
	}
}
