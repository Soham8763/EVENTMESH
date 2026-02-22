package repository

import (
	"database/sql"

	"eventmesh/workflow-orchestrator/internal/model"
)

type WorkflowRepository struct {
	db *sql.DB
}

func NewWorkflowRepository(db *sql.DB) *WorkflowRepository {
	return &WorkflowRepository{db: db}
}

func (r *WorkflowRepository) LoadDefinitions() ([]model.WorkflowDefinition, error) {
	rows, err := r.db.Query(`
		SELECT name, steps, created_at
		FROM workflow_definitions
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var defs []model.WorkflowDefinition

	for rows.Next() {
		var d model.WorkflowDefinition
		if err := rows.Scan(&d.Name, &d.Steps, &d.CreatedAt); err != nil {
			return nil, err
		}
		defs = append(defs, d)
	}

	return defs, nil
}
