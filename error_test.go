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
}
