package events

type RetryScheduledEvent struct {
	BaseEvent
	StepName string `json:"step_name"`
	Reason   string `json:"reason"`
}
