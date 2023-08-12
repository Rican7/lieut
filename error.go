package lieut

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
