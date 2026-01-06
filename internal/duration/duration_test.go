package duration

import "testing"

func TestParseDayWeek(t *testing.T) {
	if _, err := Parse("1d"); err != nil {
		t.Fatalf("expected ok: %v", err)
	}
	if _, err := Parse("2w"); err != nil {
		t.Fatalf("expected ok: %v", err)
	}
	if _, err := Parse("0d"); err == nil {
		t.Fatalf("expected error")
	}
}
