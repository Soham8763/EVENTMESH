package executor

import "eventmesh/worker/internal/model"

type Executor interface {
	Execute(task model.WorkflowTask) error
}