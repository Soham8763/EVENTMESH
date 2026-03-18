package events

import "fmt"

const (
	TopicIncomingEvents   = "events"
	TopicWorkflowTriggers = "workflow_triggers"
	TopicExecutionEvents       = "execution_events"
	TopicWorkerResults         = "worker_results"
	TopicWorkflowRegistrations = "workflow_registrations"
)

func TaskTopic(stepName string) string {
	return fmt.Sprintf("task.%s", stepName)
}
