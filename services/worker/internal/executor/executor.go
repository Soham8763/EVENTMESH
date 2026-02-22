package executor

import (
	"context"
	"eventmesh/worker/internal/model"
)

type Executor interface {
	Execute(ctx context.Context, task model.WorkflowTask) error
}
