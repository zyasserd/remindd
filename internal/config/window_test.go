package config

import (
	"testing"
	"time"
)

func TestWindowWrapAround(t *testing.T) {
	cfg := &Config{NotificationWindow: &NotificationWindow{Start: "22:00", End: "06:00"}}
	in, err := cfg.InNotificationWindow(time.Date(2026, 1, 1, 23, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if !in {
		t.Fatalf("expected in")
	}
	in, err = cfg.InNotificationWindow(time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if in {
		t.Fatalf("expected out")
	}
}
