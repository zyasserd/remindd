package config

import "testing"

func TestValidateIntervalWithoutLastDoneCommand(t *testing.T) {
	cfg := &Config{
		Reminders: map[string]Reminder{
			"a": {
				Type:     "interval",
				Label:    "A",
				Interval: "1d",
				// LastDone omitted => OK
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}
