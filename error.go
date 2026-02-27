package lieut

// constError is an error type backed by a string constant, preventing sentinel
// error values from being accidentally reassigned.
type constError string

func (e constError) Error() string {
	return string(e)
}

// StatusCodeError represents an error that reports an associated status code.
type StatusCodeError interface {
	error

	// StatusCode returns the status code of the error, which can be used by an
	// app's execution error to know which status code to return.
	StatusCode() int
}

type statusCodeError struct {
	error

	statusCode int
}

// ErrWithStatusCode takes an error and a status code and returns a type that
// satisfies StatusCodeError.
func ErrWithStatusCode(err error, statusCode int) StatusCodeError {
	return &statusCodeError{error: err, statusCode: statusCode}
}

// StatusCode returns the status code of the error.
func (e *statusCodeError) StatusCode() int {
	return e.statusCode
}

// Unwrap returns the wrapped error to support error chain inspection via
// errors.Is and errors.As.
func (e *statusCodeError) Unwrap() error {
	return e.error
}

// ErrHelpRequested is a sentinel error that, when returned from an Executor or
// init function, signals that help should be displayed to the user.
//
// To wrap your own error alongside ErrHelpRequested, use ErrWithHelpRequested.
const ErrHelpRequested constError = "help requested"

type helpRequestedError struct {
	error
}

// ErrWithHelpRequested wraps an error alongside ErrHelpRequested, returning an
// error that satisfies errors.Is for both ErrHelpRequested and the wrapped
// error. When returned from an Executor or init function, the help message will
// be displayed to the user.
//
// Wrapping an error with an empty Error() string will suppress the error output
// but still display help.
func ErrWithHelpRequested(err error) error {
	return &helpRequestedError{error: err}
}

// Is reports whether the target error matches ErrHelpRequested.
func (e *helpRequestedError) Is(target error) bool {
	return target == ErrHelpRequested
}

// Unwrap returns the wrapped error to support error chain inspection via
// errors.Is and errors.As.
func (e *helpRequestedError) Unwrap() error {
	return e.error
}
