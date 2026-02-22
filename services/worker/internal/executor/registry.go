package executor

import "fmt"

type Registry struct {
	executors map[string]Executor
}

func NewRegistry() *Registry {
	return &Registry{
		executors: make(map[string]Executor),
	}
}

func (r *Registry) Register(step string, e Executor) {
	r.executors[step] = e
}

func (r *Registry) Get(step string) (Executor, error) {
	exec, ok := r.executors[step]
	if !ok {
		return nil, fmt.Errorf("no executor for step: %s", step)
	}
	return exec, nil
}