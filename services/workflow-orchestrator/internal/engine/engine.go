package engine

import "eventmesh/workflow-orchestrator/internal/model"

type Engine interface {
	HandleTrigger(trigger model.WorkflowTriggerEvent) error
}
