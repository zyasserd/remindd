package notify

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"
)

type Capabilities struct {
	HasActions bool
	HasWait    bool
}

var (
	capOnce sync.Once
	caps    Capabilities
)

func DetectCapabilities() Capabilities {
	capOnce.Do(func() {
		path, err := exec.LookPath("notify-send")
		if err != nil {
			caps = Capabilities{}
			return
		}
		cmd := exec.Command(path, "--help")
		var b bytes.Buffer
		cmd.Stdout = &b
		cmd.Stderr = &b
		_ = cmd.Run()
		help := b.String()
		caps.HasActions = strings.Contains(help, "--action")
		caps.HasWait = strings.Contains(help, "--wait")
	})
	return caps
}

type Action struct {
	Key   string
	Label string
}

type Request struct {
	Title   string
	Body    string
	Actions []Action
	Wait    bool
}

// Send sends a notification.
// If Wait is true and notify-send supports it, returns the selected action key (or "").
func Send(req Request) (string, error) {
	path, err := exec.LookPath("notify-send")
	if err != nil {
		return "", err
	}

	caps := DetectCapabilities()
	args := []string{"--app-name=remindd"}
	if req.Wait && caps.HasWait {
		args = append(args, "--wait")
	}
	if len(req.Actions) > 0 && caps.HasActions {
		for _, a := range req.Actions {
			args = append(args, "--action", a.Key+"="+a.Label)
		}
	}
	args = append(args, req.Title)
	if strings.TrimSpace(req.Body) != "" {
		args = append(args, req.Body)
	}

	cmd := exec.Command(path, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		// On some systems notify-send returns non-zero for capability mismatches.
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}
