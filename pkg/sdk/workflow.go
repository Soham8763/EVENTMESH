package sdk

import "context"

var defaultClient *Client

func SetDefaultClient(c *Client) {
	defaultClient = c
}

type Workflow struct {
	Name   string
	Steps  []Step
	client *Client
}

func (w *Workflow) Register(ctx context.Context) error {
	client := w.client
	if client == nil {
		client = defaultClient
	}
	if client == nil {
		return nil // Or handle as error: errors.New("no client available")
	}
	return client.RegisterWorkflow(ctx, w)
}

func NewWorkflow(name string) *Workflow {
	return &Workflow{
		Name:   name,
		Steps:  []Step{},
		client: defaultClient,
	}
}

func (w *Workflow) Step(name string, handler StepHandler) *Workflow {
	step := Step{
		Name:    name,
		Handler: handler,
	}

	w.Steps = append(w.Steps, step)
	return w
}
