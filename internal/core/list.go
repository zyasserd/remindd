package core

import (
	"fmt"
	"strings"
	"time"

	"remindd/internal/config"
	"remindd/internal/state"
)

func (e *Engine) FormatListLine(now time.Time, name string, rc config.Reminder, st *state.State) (string, error) {
	inWindow, err := e.cfg.InNotificationWindow(now)
	if err != nil {
		return "", err
	}
	snoozed := state.IsSnoozed(now, st)

	status := "OK"
	freq := ""
	info := ""
	switch rc.Type {
	case "interval":
		if rc.Interval != nil {
			freq = formatFrequencySeconds(rc.Interval.Duration)
		}
	case "condition":
		if rc.Condition != nil {
			freq = formatFrequencySeconds(rc.Condition.Interval)
		}
	}
	if snoozed {
		status = "SNOOZED"
		if st != nil && st.SnoozedUntil != nil {
			info = fmt.Sprintf("until %s", time.Unix(*st.SnoozedUntil, 0).Format(time.RFC3339))
		}
	} else if !inWindow {
		status = "OUTSIDE_WINDOW"
	} else {
		switch rc.Type {
		case "interval":
			if rc.Interval == nil || rc.Interval.Duration <= 0 {
				return "", fmt.Errorf("%s: interval.duration must be > 0", name)
			}
			interval := time.Duration(rc.Interval.Duration) * time.Second
			last, err := e.resolveLastDoneUnix(rc, st)
			if err != nil {
				return "", err
			}
			dueAt := time.Unix(last, 0).Add(interval)
			if now.After(dueAt) {
				status = "DUE"
				info = formatIntervalBody(now.Sub(dueAt))
			} else {
				info = fmt.Sprintf("Due in %s", formatUntil(dueAt.Sub(now)))
			}
		case "condition":
			if rc.Condition == nil {
				return "", fmt.Errorf("%s: condition is required", name)
			}
			if st.TrueStreak >= rc.Condition.Trigger {
				status = "DUE"
			}
			info = fmt.Sprintf("streak=%d/%d", st.TrueStreak, rc.Condition.Trigger)
		}
	}

	// Keep stable, one-line output.
	_ = strings.ReplaceAll(rc.Label, "\n", " ")
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s", name, rc.Type, status, strings.TrimSpace(freq), strings.TrimSpace(info)), nil
}

func formatUntil(d time.Duration) string {
	if d <= 0 {
		return "<1 minute"
	}
	if d >= 24*time.Hour {
		days := int64(d / (24 * time.Hour))
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	if d >= time.Hour {
		h := int64(d / time.Hour)
		if h == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", h)
	}
	if d < time.Minute {
		return "<1 minute"
	}
	m := int64(d / time.Minute)
	if m == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", m)
}

func formatFrequencySeconds(secs int64) string {
	if secs <= 0 {
		return ""
	}
	d := time.Duration(secs) * time.Second
	if d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int64(d/(24*time.Hour)))
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int64(d/time.Hour))
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int64(d/time.Minute))
	}
	return fmt.Sprintf("%ds", secs)
}
