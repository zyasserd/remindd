package core

// ExitError is an error with a prescribed process exit code.
type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string { return e.Message }

func AsExitError(err error, target **ExitError) bool {
	if err == nil {
		return false
	}
	e, ok := err.(*ExitError)
	if !ok {
		return false
	}
	*target = e
	return true
}
