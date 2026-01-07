package config

import "testing"

func TestValidateIntervalWithoutLastDoneCommand(t *testing.T) {
	cfg := &Config{
		Reminders: map[string]Reminder{
			"a": {
				Type:     "interval",
				Label:    "A",
				Interval: &Interval{Duration: 86400},
				// interval.lastDoneCommand omitted => OK
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}
