package core

import (
	"strings"
	"testing"
	"time"

	"remindd/internal/config"
	"remindd/internal/state"
)

func TestFormatListLine_IntervalDue(t *testing.T) {
	cfg := &config.Config{Reminders: map[string]config.Reminder{}}
	eng := NewEngine(cfg)

	rc := config.Reminder{Type: "interval", Label: "Stretch", Interval: &config.Interval{Duration: 60}}
	st := &state.State{} // lastDone missing => 0
	now := time.Unix(100, 0).UTC()

	line, err := eng.FormatListLine(now, "demo", rc, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fields := strings.Split(line, "\t")
	if len(fields) != 5 {
		t.Fatalf("expected 5 tab-separated fields, got %d: %q", len(fields), line)
	}
	if fields[2] != "DUE" {
		t.Fatalf("expected status DUE, got %q", fields[2])
	}
	if fields[3] != "1m" {
		t.Fatalf("expected freq 1m, got %q", fields[3])
	}
	if !strings.Contains(fields[4], "Overdue") {
		t.Fatalf("expected overdue info, got %q", fields[4])
	}
}

func TestFormatListLine_IntervalNotDue(t *testing.T) {
	cfg := &config.Config{Reminders: map[string]config.Reminder{}}
	eng := NewEngine(cfg)

	rc := config.Reminder{Type: "interval", Label: "Stretch", Interval: &config.Interval{Duration: 3600}}
	st := &state.State{}
	now := time.Unix(100, 0).UTC()

	line, err := eng.FormatListLine(now, "demo", rc, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fields := strings.Split(line, "\t")
	if fields[2] != "OK" {
		t.Fatalf("expected status OK, got %q", fields[2])
	}
	if fields[3] != "1h" {
		t.Fatalf("expected freq 1h, got %q", fields[3])
	}
	if !strings.Contains(fields[4], "Due in ") {
		t.Fatalf("expected due-in info, got %q", fields[4])
	}
}

func TestFormatListLine_SnoozedShowsUntil(t *testing.T) {
	cfg := &config.Config{Reminders: map[string]config.Reminder{}}
	eng := NewEngine(cfg)

	rc := config.Reminder{Type: "interval", Label: "Stretch", Interval: &config.Interval{Duration: 60}}
	now := time.Unix(100, 0).UTC()
	until := now.Add(1 * time.Hour).Unix()
	st := &state.State{SnoozedUntil: &until}

	line, err := eng.FormatListLine(now, "demo", rc, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fields := strings.Split(line, "\t")
	if fields[2] != "SNOOZED" {
		t.Fatalf("expected status SNOOZED, got %q", fields[2])
	}
	if fields[3] != "1m" {
		t.Fatalf("expected freq 1m, got %q", fields[3])
	}
	if !strings.Contains(fields[4], "until ") {
		t.Fatalf("expected until info, got %q", fields[4])
	}
}

func TestFormatListLine_OutsideWindow(t *testing.T) {
	cfg := &config.Config{NotificationWindow: &config.NotificationWindow{Start: "18:00", End: "22:00"}, Reminders: map[string]config.Reminder{}}
	eng := NewEngine(cfg)

	rc := config.Reminder{Type: "condition", Label: "Thing", Condition: &config.Condition{Interval: 60, Command: "true", Trigger: 2}}
	st := &state.State{}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)

	line, err := eng.FormatListLine(now, "demo", rc, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fields := strings.Split(line, "\t")
	if fields[2] != "OUTSIDE_WINDOW" {
		t.Fatalf("expected status OUTSIDE_WINDOW, got %q", fields[2])
	}
	if fields[3] != "1m" {
		t.Fatalf("expected freq 1m, got %q", fields[3])
	}
}

func TestFormatListLine_LabelSingleLine(t *testing.T) {
	cfg := &config.Config{Reminders: map[string]config.Reminder{}}
	eng := NewEngine(cfg)

	rc := config.Reminder{Type: "interval", Label: "Line1\nLine2", Interval: &config.Interval{Duration: 60}}
	st := &state.State{}
	now := time.Unix(100, 0).UTC()

	line, err := eng.FormatListLine(now, "demo", rc, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(line, "\n") {
		t.Fatalf("expected single-line output, got %q", line)
	}
}
