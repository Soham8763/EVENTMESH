package events

type EventType string

const (
	WorkflowStarted   EventType = "workflow.started"
	WorkflowCompleted EventType = "workflow.completed"
	WorkflowFailed    EventType = "workflow.failed"

	StepScheduled EventType = "step.scheduled"
	StepStarted   EventType = "step.started"
	StepCompleted EventType = "step.completed"
	StepFailed    EventType = "step.failed"

	RetryScheduled EventType = "retry.scheduled"
	WorkflowDefined EventType = "workflow.defined"
)
