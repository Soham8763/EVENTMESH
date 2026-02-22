package executor

import (
	"log"

	"eventmesh/worker/internal/model"
)

type CreateProfileExecutor struct{}

func (c *CreateProfileExecutor) Execute(task model.WorkflowTask) error {
	log.Printf("creating profile for execution %s\n", task.WorkflowExecutionID)
	return nil
}