package core

import (
	"testing"
	"time"

	"remindd/internal/config"
	"remindd/internal/state"
)

func TestResolveLastDoneUnixFallsBackToState(t *testing.T) {
	eng := NewEngine(&config.Config{Reminders: map[string]config.Reminder{}})
	rc := config.Reminder{Type: "interval", Interval: &config.Interval{Duration: 86400}}
	st := &state.State{}

	v, err := eng.resolveLastDoneUnix(rc, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 0 {
		t.Fatalf("expected 0, got %d", v)
	}

	now := time.Date(2026, 1, 6, 12, 0, 0, 0, time.UTC).Unix()
	st.LastDone = &now
	v, err = eng.resolveLastDoneUnix(rc, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != now {
		t.Fatalf("expected %d, got %d", now, v)
	}
}
