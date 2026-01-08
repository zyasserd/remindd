package shell

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"syscall"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func Run(command string) (Result, error) {
	cmd := exec.Command("/bin/sh", "-c", command)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	res := Result{Stdout: outBuf.String(), Stderr: errBuf.String(), ExitCode: 0}
	if err == nil {
		return res, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		res.ExitCode = ee.ExitCode()
		return res, nil
	}
	return Result{}, err
}

// RunDetached starts command in a way that survives a systemd oneshot cgroup.
//
// On systemd systems, it uses:
//
//	systemd-run --user --scope --collect /bin/sh -c <command>
//
// which creates a transient scope for the command.
//
// If systemd-run is not available, it falls back to starting /bin/sh -c <command>
// in a new session (setsid) with stdio disconnected.
func RunDetached(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("empty command")
	}

	if path, err := exec.LookPath("systemd-run"); err == nil {
		cmd := exec.Command(path, "--user", "--scope", "--collect", "/bin/sh", "-c", command)
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			msg := string(bytes.TrimSpace(errBuf.Bytes()))
			if msg == "" {
				msg = string(bytes.TrimSpace(outBuf.Bytes()))
			}
			if msg == "" {
				return err
			}
			return errors.New(msg)
		}
		return nil
	}

	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	// Best-effort: prevent holding on to any inherited fds.
	cmd.ExtraFiles = nil
	// Ensure it doesn't inherit a cwd that might disappear.
	cmd.Dir = "/"
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	return nil
}
