package config

import "testing"

func TestValidateIntervalWithoutLastDoneCommand(t *testing.T) {
	cfg := &Config{
		Reminders: map[string]Reminder{
			"a": {
				Type:  "interval",
				Label: "A",
				Every: 86400,
				// lastDoneOverride omitted => OK
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestValidateIntervalRejectsConditionCommand(t *testing.T) {
	cfg := &Config{
		Reminders: map[string]Reminder{
			"a": {
				Type:             "interval",
				Label:            "A",
				Every:            60,
				ConditionCommand: "true",
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestValidateIntervalRejectsTrigger(t *testing.T) {
	cfg := &Config{
		Reminders: map[string]Reminder{
			"a": {
				Type:    "interval",
				Label:   "A",
				Every:   60,
				Trigger: 1,
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error, got nil")
	}
}
