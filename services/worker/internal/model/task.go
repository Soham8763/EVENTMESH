package model

import "time"

type WorkflowTask struct {
	TaskID              string    `json:"task_id"`
	WorkflowExecutionID string    `json:"workflow_execution_id"`
	StepName            string    `json:"step_name"`
	TenantID            string    `json:"tenant_id"`
	CorrelationID       string    `json:"correlation_id"`
	CreatedAt           time.Time `json:"created_at"`
}
