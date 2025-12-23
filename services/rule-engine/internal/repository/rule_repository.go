package repository

import (
	"database/sql"

	"eventmesh/rule-engine/internal/model"
)

type RuleRepository struct {
	db *sql.DB
}

func NewRuleRepository(db *sql.DB) *RuleRepository {
	return &RuleRepository{db: db}
}

func (r *RuleRepository) LoadActiveRules() ([]model.Rule, error) {
	rows, err := r.db.Query(`
		SELECT id, tenant_id, event_type, workflow_name, is_active, created_at
		FROM rules
		WHERE is_active = TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []model.Rule

	for rows.Next() {
		var rule model.Rule
		if err := rows.Scan(
			&rule.ID,
			&rule.TenantID,
			&rule.EventType,
			&rule.WorkflowName,
			&rule.IsActive,
			&rule.CreatedAt,
		); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}
