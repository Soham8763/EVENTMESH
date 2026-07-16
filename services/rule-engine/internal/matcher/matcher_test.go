package matcher

import (
	"sync"
	"testing"

	"eventmesh/rule-engine/internal/model"
)

func TestMatcherMatch(t *testing.T) {
	initialRules := []model.Rule{
		{
			ID:           "rule-1",
			TenantID:     "tenant-A",
			EventType:    "user.signup",
			WorkflowName: "welcome_workflow",
			IsActive:     true,
		},
		{
			ID:           "rule-2",
			TenantID:     "tenant-A",
			EventType:    "order.created",
			WorkflowName: "order_workflow",
			IsActive:     true,
		},
		{
			ID:           "rule-3",
			TenantID:     "tenant-B",
			EventType:    "user.signup",
			WorkflowName: "other_workflow",
			IsActive:     true,
		},
	}

	m := NewMatcher(initialRules)

	// Test case 1: Match for tenant-A user.signup
	res1 := m.Match(model.EventEnvelope{
		TenantID:  "tenant-A",
		EventType: "user.signup",
		EventID:   "evt-123",
	})
	if len(res1) != 1 || res1[0].WorkflowName != "welcome_workflow" {
		t.Errorf("Expected match for welcome_workflow, got: %+v", res1)
	}

	// Test case 2: Match for tenant-A order.created
	res2 := m.Match(model.EventEnvelope{
		TenantID:  "tenant-A",
		EventType: "order.created",
		EventID:   "evt-456",
	})
	if len(res2) != 1 || res2[0].WorkflowName != "order_workflow" {
		t.Errorf("Expected match for order_workflow, got: %+v", res2)
	}

	// Test case 3: No match (wrong event type)
	res3 := m.Match(model.EventEnvelope{
		TenantID:  "tenant-A",
		EventType: "non.existent",
	})
	if len(res3) != 0 {
		t.Errorf("Expected 0 matches, got: %d", len(res3))
	}
}

func TestMatcherReloadAndConcurrency(t *testing.T) {
	initialRules := []model.Rule{
		{
			ID:           "rule-1",
			TenantID:     "tenant-A",
			EventType:    "user.signup",
			WorkflowName: "welcome_workflow",
			IsActive:     true,
		},
	}

	m := NewMatcher(initialRules)

	// Verify initial match
	res := m.Match(model.EventEnvelope{
		TenantID:  "tenant-A",
		EventType: "user.signup",
	})
	if len(res) != 1 {
		t.Fatalf("Expected 1 match initially")
	}

	// Spawn concurrent readers and a writer to reload rules
	var wg sync.WaitGroup
	start := make(chan struct{})

	// Readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 100; j++ {
				m.Match(model.EventEnvelope{
					TenantID:  "tenant-A",
					EventType: "user.signup",
				})
			}
		}()
	}

	// Writer reloading rules
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		newRules := []model.Rule{
			{
				ID:           "rule-1-new",
				TenantID:     "tenant-A",
				EventType:    "user.signup",
				WorkflowName: "welcome_workflow_v2",
				IsActive:     true,
			},
		}
		m.Reload(newRules)
	}()

	close(start) // trigger simultaneous execution
	wg.Wait()

	// Verify reload completed successfully
	finalRes := m.Match(model.EventEnvelope{
		TenantID:  "tenant-A",
		EventType: "user.signup",
	})
	if len(finalRes) != 1 || finalRes[0].WorkflowName != "welcome_workflow_v2" {
		t.Errorf("Expected reloaded rule welcome_workflow_v2, got: %+v", finalRes)
	}
}
