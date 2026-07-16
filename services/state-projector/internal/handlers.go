package internal

import (
	"encoding/json"

	"eventmesh/internal/events"
	"eventmesh/pkg/logger"

	"go.uber.org/zap"
)

func (p *Projector) handleEvent(data []byte) error {
	var base events.BaseEvent
	err := json.Unmarshal(data, &base)
	if err != nil {
		logger.Log.Error("event decode error", zap.Error(err))
		return err
	}

	switch base.EventType {
	case events.WorkflowStarted:
		return p.handleWorkflowStarted(data)
	case events.StepScheduled:
		return p.handleStepScheduled(data)
	case events.StepCompleted:
		return p.handleStepCompleted(data)
	case events.StepFailed:
		return p.handleStepFailed(data)
	case events.WorkflowCompleted:
		return p.handleWorkflowCompleted(data)
	}

	return nil
}

// handleWorkflowStarted updates the read model. The orchestrator is the source
// of truth and writes execution state directly to PostgreSQL. The projector
// uses ON CONFLICT DO NOTHING so it is safe to replay or skip.
func (p *Projector) handleWorkflowStarted(data []byte) error {
	var event events.WorkflowStartedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	tx, err := p.db.Begin()
	if err != nil {
		logger.Log.Error("transaction error", zap.Error(err))
		return err
	}
	defer tx.Rollback()

	// Idempotent: if the orchestrator already wrote this record, skip it
	_, err = tx.Exec(`
        INSERT INTO workflow_executions
        (id, workflow_name, status, created_at, tenant_id, trigger_id)
        VALUES ($1, $2, 'RUNNING', $3, $4, $5)
        ON CONFLICT (id) DO NOTHING
    `,
		event.ExecutionID,
		event.WorkflowID,
		event.Timestamp,
		event.TenantID,
		event.TriggerID,
	)

	if err != nil {
		logger.Log.Error("workflow insert error", zap.Error(err))
		return err
	}

	// Insert all steps as PENDING (idempotent)
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
			logger.Log.Error("step projection error", zap.Error(err))
		}
	}

	if err := tx.Commit(); err != nil {
		logger.Log.Error("commit error", zap.Error(err))
		return err
	}

	return nil
}

func (p *Projector) handleStepScheduled(data []byte) error {
	var event events.StepScheduledEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	// Idempotent update: only update if status hasn't already advanced past RUNNING
	_, err := p.db.Exec(`
        UPDATE workflow_step_executions
        SET status = 'RUNNING', updated_at = NOW()
        WHERE workflow_execution_id = $1 AND step_name = $2
          AND status = 'PENDING'
    `,
		event.ExecutionID,
		event.StepName,
	)

	if err != nil {
		logger.Log.Error("step scheduled update error", zap.Error(err))
		return err
	}

	return nil
}

func (p *Projector) handleStepCompleted(data []byte) error {
	var event events.StepCompletedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	// Idempotent update: only update if status hasn't already been set
	_, err := p.db.Exec(`
        UPDATE workflow_step_executions
        SET status = 'SUCCESS', updated_at = NOW()
        WHERE workflow_execution_id = $1 AND step_name = $2
          AND status IN ('RUNNING', 'PENDING')
    `,
		event.ExecutionID,
		event.StepName,
	)

	if err != nil {
		logger.Log.Error("step update error", zap.Error(err))
		return err
	}

	return nil
}

func (p *Projector) handleStepFailed(data []byte) error {
	var event events.StepFailedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	_, err := p.db.Exec(`
        UPDATE workflow_step_executions
        SET last_error = $1, updated_at = NOW()
        WHERE workflow_execution_id = $2 AND step_name = $3
    `,
		event.Error,
		event.ExecutionID,
		event.StepName,
	)

	if err != nil {
		logger.Log.Error("step failed update error", zap.Error(err))
		return err
	}

	return nil
}

func (p *Projector) handleWorkflowCompleted(data []byte) error {
	var event events.WorkflowCompletedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	// Idempotent update
	_, err := p.db.Exec(`
        UPDATE workflow_executions
        SET status = 'COMPLETED', updated_at = NOW()
        WHERE id = $1 AND status != 'COMPLETED'
    `,
		event.ExecutionID,
	)

	if err != nil {
		logger.Log.Error("workflow update error", zap.Error(err))
		return err
	}

	return nil
}
