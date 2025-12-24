package matcher

import (
	"eventmesh/rule-engine/internal/model"
)

type Matcher struct {
	rules []model.Rule
}

func NewMatcher(rules []model.Rule) *Matcher {
	return &Matcher{rules: rules}
}

func (m *Matcher) Match(event model.EventEnvelope) []model.MatchResult {
	var results []model.MatchResult

	for _, rule := range m.rules {
		// Tenant isolation
		if rule.TenantID != event.TenantID {
			continue
		}

		// Event type match
		if rule.EventType != event.EventType {
			continue
		}

		results = append(results, model.MatchResult{
			RuleID:       rule.ID,
			TenantID:     rule.TenantID,
			WorkflowName: rule.WorkflowName,
			EventID:      event.EventID,
		})
	}

	return results
}
