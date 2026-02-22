package model

import "time"

type StepExecution struct {
	ID                  string
	WorkflowExecutionID string
	StepName            string
	Status              string
	RetryCount          int
	LastError           *string
	UpdatedAt           time.Time
}
