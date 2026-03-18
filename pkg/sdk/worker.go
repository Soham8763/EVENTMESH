package sdk

import (
)

type Worker struct {
	handlers map[string]StepHandler
}

func NewWorker() *Worker {
	return &Worker{
		handlers: make(map[string]StepHandler),
	}
}

func (w *Worker) Register(stepName string, handler StepHandler) {
	w.handlers[stepName] = handler
}
