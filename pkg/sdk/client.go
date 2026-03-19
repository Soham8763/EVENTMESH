package sdk

import (
	"context"
	"time"

	"eventmesh/internal/events"

	"github.com/google/uuid"
)

type Client struct {
	publisher *events.EventPublisher
}

func NewClient(brokers []string) *Client {
	publisher := events.NewEventPublisher(brokers)

	return &Client{
		publisher: publisher,
	}
}

func (c *Client) NewWorkflow(name string) *Workflow {
	return &Workflow{
		Name:   name,
		Steps:  []Step{},
		client: c,
	}
}

func (c *Client) RegisterWorkflow(ctx context.Context, w *Workflow) error {
	steps := make([]map[string]interface{}, len(w.Steps))
	for i, s := range w.Steps {
		steps[i] = map[string]interface{}{
			"step": s.Name,
		}
	}

	event := events.WorkflowDefinedEvent{
		BaseEvent: events.BaseEvent{
			EventID:     uuid.New().String(),
			EventType:   events.WorkflowDefined,
			Timestamp:   time.Now(),
			ExecutionID: "n/a", // Registration is a system-level event
		},
		WorkflowName: w.Name,
		Steps:        steps,
	}

	return c.publisher.PublishToTopic(ctx, events.TopicWorkflowRegistrations, w.Name, event)
}

func (c *Client) StartWorkflow(ctx context.Context, workflowName string, executionID string) error {
	event := events.WorkflowTriggerEvent{
		TriggerID:     executionID,
		EventID:       uuid.New().String(),
		WorkflowName:  workflowName,
		CorrelationID: executionID,
		TriggeredAt:   time.Now(),
	}

	return c.publisher.PublishToTopic(ctx, events.TopicWorkflowTriggers, executionID, event)
}
