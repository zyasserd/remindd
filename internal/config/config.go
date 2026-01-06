package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"remindd/internal/duration"
	"remindd/internal/xdg"
)

var nameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type Config struct {
	NotificationWindow *NotificationWindow `yaml:"notificationWindow"`
	Reminders          map[string]Reminder `yaml:"reminders"`
}

type NotificationWindow struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

type Reminder struct {
	Type  string `yaml:"type"`
	Label string `yaml:"label"`

	Snooze  *Snooze `yaml:"snooze"`
	Action  *Action `yaml:"action"`
	Interval string `yaml:"interval"`
	LastDone *LastDone `yaml:"lastDone"`
	Check   *Check   `yaml:"check"`
	Trigger *Trigger `yaml:"trigger"`
}

type Snooze struct {
	Default string `yaml:"default"`
}

type Action struct {
	Label   string `yaml:"label"`
	Command string `yaml:"command"`
}

type LastDone struct {
	Command string `yaml:"command"`
}

type Check struct {
	Interval string `yaml:"interval"`
	Command  string `yaml:"command"`
}

type Trigger struct {
	Consecutive int `yaml:"consecutive"`
}

func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	if cfg.Reminders == nil {
		cfg.Reminders = map[string]Reminder{}
	}
	return &cfg, nil
}

func ConfigPath() (string, error) {
	if v := os.Getenv("REMINDD_CONFIG"); v != "" {
		return v, nil
	}
	h, err := xdg.ConfigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "remindd", "config.yaml"), nil
}

func (c *Config) Validate() error {
	if c.NotificationWindow != nil {
		if strings.TrimSpace(c.NotificationWindow.Start) == "" || strings.TrimSpace(c.NotificationWindow.End) == "" {
			return errors.New("notificationWindow requires start and end")
		}
		if _, err := parseHHMM(c.NotificationWindow.Start); err != nil {
			return fmt.Errorf("notificationWindow.start: %w", err)
		}
		if _, err := parseHHMM(c.NotificationWindow.End); err != nil {
			return fmt.Errorf("notificationWindow.end: %w", err)
		}
	}

	if len(c.Reminders) == 0 {
		return errors.New("no reminders configured")
	}

	names := make([]string, 0, len(c.Reminders))
	for name := range c.Reminders {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		r := c.Reminders[name]
		if strings.TrimSpace(name) == "" {
			return errors.New("reminder name must be non-empty")
		}
		if !nameRe.MatchString(name) {
			return fmt.Errorf("reminder %q: invalid name (allowed: [A-Za-z0-9_-]+)", name)
		}
		if strings.TrimSpace(r.Type) == "" {
			return fmt.Errorf("reminder %q: missing type", name)
		}
		if r.Type != "interval" && r.Type != "condition" {
			return fmt.Errorf("reminder %q: invalid type %q", name, r.Type)
		}
		if strings.TrimSpace(r.Label) == "" {
			return fmt.Errorf("reminder %q: missing label", name)
		}

		defSnooze := "1d"
		if r.Snooze != nil && strings.TrimSpace(r.Snooze.Default) != "" {
			defSnooze = r.Snooze.Default
		}
		if _, err := duration.Parse(defSnooze); err != nil {
			return fmt.Errorf("reminder %q: snooze.default: %w", name, err)
		}

		switch r.Type {
		case "interval":
			if strings.TrimSpace(r.Interval) == "" {
				return fmt.Errorf("reminder %q: interval is required", name)
			}
			if _, err := duration.Parse(r.Interval); err != nil {
				return fmt.Errorf("reminder %q: interval: %w", name, err)
			}
			// lastDone.command is optional; if omitted/empty, fall back to per-reminder state.lastDone.
		case "condition":
			if r.Check == nil {
				return fmt.Errorf("reminder %q: check is required", name)
			}
			if strings.TrimSpace(r.Check.Interval) == "" {
				return fmt.Errorf("reminder %q: check.interval is required", name)
			}
			if _, err := duration.Parse(r.Check.Interval); err != nil {
				return fmt.Errorf("reminder %q: check.interval: %w", name, err)
			}
			if strings.TrimSpace(r.Check.Command) == "" {
				return fmt.Errorf("reminder %q: check.command is required", name)
			}
			if r.Trigger == nil {
				return fmt.Errorf("reminder %q: trigger is required", name)
			}
			if r.Trigger.Consecutive < 1 {
				return fmt.Errorf("reminder %q: trigger.consecutive must be >= 1", name)
			}
		}

		if r.Action != nil {
			if strings.TrimSpace(r.Action.Command) == "" {
				return fmt.Errorf("reminder %q: action.command is required if action is present", name)
			}
		}
	}

	return nil
}

func parseHHMM(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("expected HH:MM")
	}
	if len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, fmt.Errorf("expected HH:MM")
	}
	hh := parts[0]
	mm := parts[1]
	var h, m int
	_, err := fmt.Sscanf(hh, "%02d", &h)
	if err != nil {
		return 0, fmt.Errorf("invalid hour")
	}
	_, err = fmt.Sscanf(mm, "%02d", &m)
	if err != nil {
		return 0, fmt.Errorf("invalid minute")
	}
	if h < 0 || h > 23 {
		return 0, fmt.Errorf("hour out of range")
	}
	if m < 0 || m > 59 {
		return 0, fmt.Errorf("minute out of range")
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, nil
}

func (c *Config) InNotificationWindow(t time.Time) (bool, error) {
	if c.NotificationWindow == nil {
		return true, nil
	}
	start, err := parseHHMM(c.NotificationWindow.Start)
	if err != nil {
		return false, err
	}
	end, err := parseHHMM(c.NotificationWindow.End)
	if err != nil {
		return false, err
	}

	if start == end {
		return true, nil
	}

	now := time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute
	if start < end {
		return now >= start && now < end, nil
	}
	// wrap-around
	return now >= start || now < end, nil
}
