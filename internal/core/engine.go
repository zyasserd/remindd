package core

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"remindd/internal/config"
	"remindd/internal/exitcode"
	"remindd/internal/notify"
	"remindd/internal/shell"
	"remindd/internal/state"
)

type Engine struct {
	cfg *config.Config
}

func NewEngine(cfg *config.Config) *Engine { return &Engine{cfg: cfg} }

func (e *Engine) CheckAll(now time.Time) error {
	names := make([]string, 0, len(e.cfg.Reminders))
	for name := range e.cfg.Reminders {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		rc := e.cfg.Reminders[name]
		st, err := state.Load(name)
		if err != nil {
			return &ExitError{Code: exitcode.StateIO, Message: fmt.Sprintf("state error for %s: %v", name, err)}
		}

		inWindow, err := e.cfg.InNotificationWindow(now)
		if err != nil {
			return &ExitError{Code: exitcode.ConfigError, Message: fmt.Sprintf("notifyWindow error: %v", err)}
		}
		if state.IsSnoozed(now, st) || !inWindow {
			continue
		}

		due, body, changed, err := e.evalOne(now, name, rc, st)
		if err != nil {
			return err
		}

		if due {
			// notify once per cycle per reminder.
			actions := []notify.Action{}
			defaultSnooze, _ := e.defaultSnooze(rc)
			actions = append(actions, notify.Action{Key: "snooze", Label: "Snooze"})
			if rc.Action != nil && strings.TrimSpace(rc.Action.Command) != "" {
				label := strings.TrimSpace(rc.Action.Label)
				if label == "" {
					label = "Run"
				}
				actions = append([]notify.Action{{Key: "run", Label: label}}, actions...)
			}

			selected, nerr := notify.Send(notify.Request{
				Title:   rc.Label,
				Body:    body,
				Actions: actions,
				Wait:    true,
			})
			if nerr != nil {
				return &ExitError{Code: exitcode.ConfigError, Message: fmt.Sprintf("notify-send failed: %v", nerr)}
			}
			nowUnix := now.Unix()
			st.LastNotifiedAt = &nowUnix
			changed = true
			if selected == "run" {
				if err := e.runActionWithState(now, name, rc, st); err != nil {
					return err
				}
				if err := state.Save(name, st); err != nil {
					return &ExitError{Code: exitcode.StateIO, Message: fmt.Sprintf("state save error for %s: %v", name, err)}
				}
				continue
			}
			if selected == "snooze" {
				if err := e.snoozeWithState(now, name, rc, st, defaultSnooze); err != nil {
					return err
				}
				if err := state.Save(name, st); err != nil {
					return &ExitError{Code: exitcode.StateIO, Message: fmt.Sprintf("state save error for %s: %v", name, err)}
				}
				continue
			}
		}

		if changed {
			if err := state.Save(name, st); err != nil {
				return &ExitError{Code: exitcode.StateIO, Message: fmt.Sprintf("state save error for %s: %v", name, err)}
			}
		}
	}

	return nil
}

func (e *Engine) evalOne(now time.Time, name string, rc config.Reminder, st *state.State) (due bool, body string, changed bool, err error) {
	switch rc.Type {
	case "interval":
		if rc.Every <= 0 {
			return false, "", false, &ExitError{Code: exitcode.ConfigError, Message: fmt.Sprintf("%s: every must be > 0", name)}
		}
		interval := time.Duration(rc.Every) * time.Second
		lastDoneUnix, err := e.resolveLastDoneUnix(rc, st)
		if err != nil {
			return false, "", false, &ExitError{Code: exitcode.ConfigError, Message: fmt.Sprintf("%s: %v", name, err)}
		}

		dueAt := time.Unix(lastDoneUnix, 0).Add(interval)
		overdue := now.Sub(dueAt)
		if overdue > 0 {
			return true, formatIntervalBody(overdue), false, nil
		}
		return false, "", false, nil

	case "condition":
		if rc.Every <= 0 {
			return false, "", false, &ExitError{Code: exitcode.ConfigError, Message: fmt.Sprintf("%s: every must be > 0", name)}
		}
		if strings.TrimSpace(rc.ConditionCommand) == "" {
			return false, "", false, &ExitError{Code: exitcode.ConfigError, Message: fmt.Sprintf("%s: conditionCommand is required", name)}
		}
		if rc.Trigger < 1 {
			return false, "", false, &ExitError{Code: exitcode.ConfigError, Message: fmt.Sprintf("%s: trigger must be >= 1", name)}
		}
		checkInterval := time.Duration(rc.Every) * time.Second
		if st.LastCheckAt != nil {
			last := time.Unix(*st.LastCheckAt, 0)
			if now.Sub(last) < checkInterval {
				return false, "", false, nil
			}
		}

		res, err := shell.Run(rc.ConditionCommand)
		if err != nil {
			return false, "", false, &ExitError{Code: exitcode.ConfigError, Message: fmt.Sprintf("%s: conditionCommand failed: %v", name, err)}
		}

		nowUnix := now.Unix()
		st.LastCheckAt = &nowUnix
		changed = true

		if res.ExitCode == 0 {
			st.TrueStreak += 1
			if st.FirstTrueAt == nil {
				st.FirstTrueAt = &nowUnix
			}
		} else {
			st.TrueStreak = 0
			st.FirstTrueAt = nil
		}

		if st.TrueStreak >= rc.Trigger {
			return true, fmt.Sprintf("Condition true for %d consecutive checks", st.TrueStreak), true, nil
		}
		return false, "", true, nil

	default:
		return false, "", false, &ExitError{Code: exitcode.ConfigError, Message: fmt.Sprintf("%s: unknown reminder type %q", name, rc.Type)}
	}
}

func (e *Engine) RunAction(now time.Time, name string) error {
	rc, ok := e.cfg.Reminders[name]
	if !ok {
		return &ExitError{Code: exitcode.UnknownName, Message: fmt.Sprintf("unknown reminder: %s", name)}
	}
	st, err := state.Load(name)
	if err != nil {
		return &ExitError{Code: exitcode.StateIO, Message: fmt.Sprintf("state error for %s: %v", name, err)}
	}
	if err := e.runActionWithState(now, name, rc, st); err != nil {
		return err
	}
	if err := state.Save(name, st); err != nil {
		return &ExitError{Code: exitcode.StateIO, Message: fmt.Sprintf("state save error for %s: %v", name, err)}
	}
	return nil
}

func (e *Engine) runActionWithState(now time.Time, name string, rc config.Reminder, st *state.State) error {
	if rc.Action == nil || strings.TrimSpace(rc.Action.Command) == "" {
		return &ExitError{Code: exitcode.ConfigError, Message: fmt.Sprintf("%s: no action configured", name)}
	}
	// Actions are fire-and-forget: start detached and consider success as "started".
	if err := shell.RunDetached(rc.Action.Command); err != nil {
		return &ExitError{Code: exitcode.ActionFail, Message: fmt.Sprintf("%s: action failed to start: %v", name, err)}
	}

	nowUnix := now.Unix()
	st.SnoozedUntil = nil
	switch rc.Type {
	case "interval":
		st.LastDone = &nowUnix
	case "condition":
		st.TrueStreak = 0
		st.FirstTrueAt = nil
	}
	return nil
}

func (e *Engine) Snooze(now time.Time, name string, d time.Duration) error {
	rc, ok := e.cfg.Reminders[name]
	if !ok {
		return &ExitError{Code: exitcode.UnknownName, Message: fmt.Sprintf("unknown reminder: %s", name)}
	}
	st, err := state.Load(name)
	if err != nil {
		return &ExitError{Code: exitcode.StateIO, Message: fmt.Sprintf("state error for %s: %v", name, err)}
	}
	if err := e.snoozeWithState(now, name, rc, st, d); err != nil {
		return err
	}
	if err := state.Save(name, st); err != nil {
		return &ExitError{Code: exitcode.StateIO, Message: fmt.Sprintf("state save error for %s: %v", name, err)}
	}
	return nil
}

func (e *Engine) snoozeWithState(now time.Time, name string, rc config.Reminder, st *state.State, d time.Duration) error {
	if d <= 0 {
		return &ExitError{Code: exitcode.ConfigError, Message: "snooze duration must be > 0"}
	}
	t := now.Add(d).Unix()
	st.SnoozedUntil = &t
	return nil
}

func (e *Engine) defaultSnooze(rc config.Reminder) (time.Duration, error) {
	secs := int64(86400)
	if rc.Snooze != nil {
		secs = *rc.Snooze
	}
	if secs <= 0 {
		return 0, fmt.Errorf("snooze must be > 0")
	}
	return time.Duration(secs) * time.Second, nil
}

func parseUnixSeconds(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty stdout")
	}
	return strconv.ParseInt(s, 10, 64)
}

func formatIntervalBody(overdue time.Duration) string {
	if overdue <= 0 {
		return "Due now"
	}
	if overdue >= 24*time.Hour {
		days := int64(overdue / (24 * time.Hour))
		if days == 1 {
			return "Overdue by 1 day"
		}
		return fmt.Sprintf("Overdue by %d days", days)
	}
	if overdue >= time.Hour {
		h := int64(overdue / time.Hour)
		if h == 1 {
			return "Overdue by 1 hour"
		}
		return fmt.Sprintf("Overdue by %d hours", h)
	}
	if overdue < time.Minute {
		return "Overdue by <1 minute"
	}
	m := int64(overdue / time.Minute)
	if m == 1 {
		return "Overdue by 1 minute"
	}
	return fmt.Sprintf("Overdue by %d minutes", m)
}

func (e *Engine) resolveLastDoneUnix(rc config.Reminder, st *state.State) (int64, error) {
	if strings.TrimSpace(rc.LastDoneOverride) == "" {
		if st == nil || st.LastDone == nil {
			return 0, nil
		}
		return *st.LastDone, nil
	}

	res, err := shell.Run(rc.LastDoneOverride)
	if err != nil {
		return 0, fmt.Errorf("lastDoneOverride failed: %v", err)
	}
	lastDoneUnix, err := parseUnixSeconds(res.Stdout)
	if err != nil {
		return 0, fmt.Errorf("lastDoneOverride stdout invalid unix timestamp: %v", err)
	}
	return lastDoneUnix, nil
}
