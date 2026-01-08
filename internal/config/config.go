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
	NotifyWindow *NotifyWindow       `yaml:"notifyWindow"`
	Reminders    map[string]Reminder `yaml:"reminders"`
}

type NotifyWindow struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type Reminder struct {
	Type  string `yaml:"type"`
	Label string `yaml:"label"`

	Action           *Action `yaml:"action"`
	Snooze           *int64  `yaml:"snooze"`
	LastDoneOverride string  `yaml:"lastDoneOverride"`
	Every            int64   `yaml:"every"`
	ConditionCommand string  `yaml:"conditionCommand"`
	Trigger          int     `yaml:"trigger"`
}

type Action struct {
	Label   string `yaml:"label"`
	Command string `yaml:"command"`
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
	if c.NotifyWindow != nil {
		if strings.TrimSpace(c.NotifyWindow.From) == "" || strings.TrimSpace(c.NotifyWindow.To) == "" {
			return errors.New("notifyWindow requires from and to")
		}
		if _, err := parseHHMM(c.NotifyWindow.From); err != nil {
			return fmt.Errorf("notifyWindow.from: %w", err)
		}
		if _, err := parseHHMM(c.NotifyWindow.To); err != nil {
			return fmt.Errorf("notifyWindow.to: %w", err)
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
		if r.Every <= 0 {
			return fmt.Errorf("reminder %q: every must be > 0", name)
		}

		switch r.Type {
		case "interval":
			if strings.TrimSpace(r.ConditionCommand) != "" {
				return fmt.Errorf("reminder %q: conditionCommand is only allowed for type=condition", name)
			}
			if r.Trigger != 0 {
				return fmt.Errorf("reminder %q: trigger is only allowed for type=condition", name)
			}
		case "condition":
			if strings.TrimSpace(r.ConditionCommand) == "" {
				return fmt.Errorf("reminder %q: conditionCommand is required", name)
			}
			if r.Trigger < 1 {
				return fmt.Errorf("reminder %q: trigger must be >= 1", name)
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
	if c.NotifyWindow == nil {
		return true, nil
	}
	start, err := parseHHMM(c.NotifyWindow.From)
	if err != nil {
		return false, err
	}
	end, err := parseHHMM(c.NotifyWindow.To)
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
