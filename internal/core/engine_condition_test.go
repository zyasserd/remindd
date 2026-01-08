package core

import (
	"testing"
	"time"

	"remindd/internal/config"
	"remindd/internal/state"
)

func TestEvalOne_ConditionDueMessageUsesDuration(t *testing.T) {
	eng := NewEngine(&config.Config{Reminders: map[string]config.Reminder{}})

	rc := config.Reminder{
		Type:             "condition",
		Every:            60,
		Trigger:          3,
		ConditionCommand: "true",
	}
	st := &state.State{TrueStreak: 2}
	now := time.Unix(1000, 0).UTC()

	due, body, changed, err := eng.evalOne(now, "demo", rc, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !due {
		t.Fatalf("expected due=true")
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if body != "Condition true for at least 3 minutes" {
		t.Fatalf("unexpected body: %q", body)
	}
}
