package sdk

import "context"

type StepHandler func(ctx context.Context, input []byte) error

type Step struct {
	Name    string
	Handler StepHandler
}
