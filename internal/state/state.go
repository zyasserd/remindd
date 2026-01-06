package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"remindd/internal/xdg"
)

type State struct {
	LastDone       *int64 `yaml:"lastDone,omitempty"`
	TrueStreak     int    `yaml:"trueStreak,omitempty"`
	FirstTrueAt    *int64 `yaml:"firstTrueAt,omitempty"`
	SnoozedUntil   *int64 `yaml:"snoozedUntil,omitempty"`
	LastNotifiedAt *int64 `yaml:"lastNotifiedAt,omitempty"`
	LastCheckAt    *int64 `yaml:"lastCheckAt,omitempty"`
}

func Load(name string) (*State, error) {
	p, err := Path(name)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{}, nil
		}
		return nil, err
	}
	var st State
	if err := yaml.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func Save(name string, st *State) error {
	p, err := Path(name)
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	b, err := yaml.Marshal(st)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*.yaml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, p); err != nil {
		return err
	}
	return nil
}

func Path(name string) (string, error) {
	h, err := xdg.StateHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "remindd", "state", fmt.Sprintf("%s.yaml", name)), nil
}

func IsSnoozed(now time.Time, st *State) bool {
	if st == nil || st.SnoozedUntil == nil {
		return false
	}
	return now.Unix() < *st.SnoozedUntil
}
