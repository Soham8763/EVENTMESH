package executor

import (
	"log"
	"time"

	"eventmesh/worker/internal/model"
)

type CreateProfileExecutor struct{}

func (c *CreateProfileExecutor) Execute(task model.WorkflowTask) error {
	log.Printf("creating profile for execution %s\n", task.WorkflowExecutionID)
	time.Sleep(40 * time.Second)
	log.Printf("finished creating profile for execution %s\n", task.WorkflowExecutionID)
	return nil
}
