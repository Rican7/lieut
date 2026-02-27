package lieut

import (
	"errors"
	"testing"
)

func TestErrWithStatusCode(t *testing.T) {
	errMsg := "test error"
	errCode := 107

	err := errors.New(errMsg)

	statusCodeErr := ErrWithStatusCode(err, errCode)
	if statusCodeErr == nil {
		t.Fatal("ErrWithStatusCode returned nil")
	}

	if gotMsg := statusCodeErr.Error(); gotMsg != errMsg {
		t.Errorf("err.Error() returned %q, wanted %q", gotMsg, errMsg)
	}

	if gotCode := statusCodeErr.StatusCode(); gotCode != errCode {
		t.Errorf("err.Error() returned %v, wanted %v", gotCode, errCode)
	}

	if !errors.Is(statusCodeErr, err) {
		t.Error("errors.Is(statusCodeErr, err) returned false, wanted true")
	}
}

func TestErrHelpRequested(t *testing.T) {
	if got := ErrHelpRequested.Error(); got != "help requested" {
		t.Errorf("ErrHelpRequested.Error() returned %q, wanted %q", got, "help requested")
	}
}

func TestErrWithHelpRequested(t *testing.T) {
	errMsg := "test error"

	err := errors.New(errMsg)

	helpRequestedErr := ErrWithHelpRequested(err)
	if helpRequestedErr == nil {
		t.Fatal("ErrWithHelpRequested returned nil")
	}

	if gotMsg := helpRequestedErr.Error(); gotMsg != errMsg {
		t.Errorf("err.Error() returned %q, wanted %q", gotMsg, errMsg)
	}

	if !errors.Is(helpRequestedErr, ErrHelpRequested) {
		t.Error("errors.Is(helpRequestedErr, ErrHelpRequested) returned false, wanted true")
	}

	if !errors.Is(helpRequestedErr, err) {
		t.Error("errors.Is(helpRequestedErr, err) returned false, wanted true")
	}
}
