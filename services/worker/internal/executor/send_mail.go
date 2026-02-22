package executor

import (
	"log"
	"time"

	"eventmesh/worker/internal/model"
)

type SendEmailExecutor struct{}

func (s *SendEmailExecutor) Execute(task model.WorkflowTask) error {
	log.Printf("sending email for execution %s\n", task.WorkflowExecutionID)
	time.Sleep(40 * time.Second)
	log.Printf("finished sending email for execution %s\n", task.WorkflowExecutionID)
	return nil
}
