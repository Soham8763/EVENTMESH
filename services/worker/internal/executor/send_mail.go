package executor

import (
	"log"

	"eventmesh/worker/internal/model"
)

type SendEmailExecutor struct{}

func (s *SendEmailExecutor) Execute(task model.WorkflowTask) error {
	log.Printf("sending email for execution %s\n", task.WorkflowExecutionID)
	return nil
}