package shell

import (
	"bytes"
	"os/exec"
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
