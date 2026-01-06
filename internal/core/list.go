package core

import (
	"fmt"
	"strings"
	"time"

	"remindd/internal/config"
	"remindd/internal/duration"
	"remindd/internal/state"
)

func (e *Engine) FormatListLine(now time.Time, name string, rc config.Reminder, st *state.State) (string, error) {
	inWindow, err := e.cfg.InNotificationWindow(now)
	if err != nil {
		return "", err
	}
	snoozed := state.IsSnoozed(now, st)

	dueStr := "no"
	detail := ""
	if snoozed {
		dueStr = "snoozed"
	} else if !inWindow {
		dueStr = "outside-window"
	} else {
		switch rc.Type {
		case "interval":
			interval, _ := duration.Parse(rc.Interval)
			last, err := e.resolveLastDoneUnix(rc, st)
			if err != nil {
				return "", err
			}
			dueAt := time.Unix(last, 0).Add(interval)
			if now.After(dueAt) {
				dueStr = "YES"
				detail = formatIntervalBody(now.Sub(dueAt))
			} else {
				detail = fmt.Sprintf("due at %s", dueAt.Format(time.RFC3339))
			}
		case "condition":
			if st.TrueStreak >= rc.Trigger.Consecutive {
				dueStr = "YES"
			}
			detail = fmt.Sprintf("streak=%d/%d", st.TrueStreak, rc.Trigger.Consecutive)
		}
	}

	// Keep stable, one-line output.
	label := strings.ReplaceAll(rc.Label, "\n", " ")
	return fmt.Sprintf("%s\t%s\t%s\t%s", name, rc.Type, dueStr, strings.TrimSpace(label+" "+detail)), nil
}
