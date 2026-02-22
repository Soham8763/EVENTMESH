package engine

import "eventmesh/workflow-orchestrator/internal/model"

type ResultHandler interface {
	HandleResult(model.TaskResult) error
}
