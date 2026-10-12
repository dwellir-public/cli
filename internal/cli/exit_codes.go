package cli

import "errors"

// Exit codes from AGENTS.md.
const (
	exitError      = 1
	exitAuth       = 2
	exitNotFound   = 3
	exitValidation = 4
)

// exitCodeError carries an exit code for an error that a formatter already
// rendered. Commands opt in with withExitCode; every other error exits 1.
type exitCodeError struct {
	err  error
	code int
}

func (e *exitCodeError) Error() string { return e.err.Error() }
func (e *exitCodeError) Unwrap() error { return e.err }

// withExitCode attaches the exit code for an error code such as
// "validation_error". A nil error stays nil.
func withExitCode(err error, errorCode string) error {
	if err == nil {
		return nil
	}
	return &exitCodeError{err: err, code: exitCodeFor(errorCode)}
}

func exitCodeFor(errorCode string) int {
	switch errorCode {
	case "not_authenticated", "forbidden":
		return exitAuth
	case "not_found":
		return exitNotFound
	case "validation_error":
		return exitValidation
	default:
		return exitError
	}
}

// ExitCode returns the process exit code for an error from Execute.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var coded *exitCodeError
	if errors.As(err, &coded) {
		return coded.code
	}
	return exitError
}
