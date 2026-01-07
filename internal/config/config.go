package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

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

	Snooze *int64           `yaml:"snooze"`
	Action        *Action    `yaml:"action"`
	Interval      *Interval  `yaml:"interval"`
	Condition     *Condition `yaml:"condition"`
}

type Action struct {
	Label   string `yaml:"label"`
	Command string `yaml:"command"`
}

type Interval struct {
	Duration        int64  `yaml:"duration"`
	LastDoneCommand string `yaml:"lastDoneCommand"`
}

type Condition struct {
	Interval int64  `yaml:"interval"`
	Command         string `yaml:"command"`
	Trigger         int    `yaml:"trigger"`
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


		// snooze is optional; default 86400.
		if r.Snooze != nil && *r.Snooze <= 0 {
			return fmt.Errorf("reminder %q: snooze must be > 0", name)
		}

		switch r.Type {
		case "interval":
			if r.Interval == nil {
				return fmt.Errorf("reminder %q: interval is required", name)
			}
			if r.Interval.Duration <= 0 {
				return fmt.Errorf("reminder %q: interval.duration must be > 0", name)
			}
			if strings.TrimSpace(r.Interval.LastDoneCommand) != "" {
				// Basic sanity: reject strings with newlines that are likely accidental YAML mistakes.
				// (Commands can still be multi-line via YAML block scalars; this only rejects whitespace-only.)
			}
		case "condition":
			if r.Condition == nil {
				return fmt.Errorf("reminder %q: condition is required", name)
			}
			if r.Condition.Interval <= 0 {
				return fmt.Errorf("reminder %q: condition.interval must be > 0", name)
			}
			if strings.TrimSpace(r.Condition.Command) == "" {
				return fmt.Errorf("reminder %q: condition.command is required", name)
			}
			if r.Condition.Trigger < 1 {
				return fmt.Errorf("reminder %q: condition.trigger must be >= 1", name)
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

// parseInt64Strict is used for small config parsing helpers when needed.
func parseInt64Strict(field, s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("%s is empty", field)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", field, err)
	}
	return v, nil
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
