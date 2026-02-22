package model

type TaskResult struct {
	TaskID              string  `json:"task_id"`
	WorkflowExecutionID string  `json:"workflow_execution_id"`
	StepName            string  `json:"step_name"`
	Status              string  `json:"status"`
	Error               *string `json:"error"`
}