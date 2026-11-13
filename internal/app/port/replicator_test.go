package port

import (
	"errors"
	"testing"
)

func TestSaveErrorUnwraps(t *testing.T) {
	var err error = &SaveError{Err: ErrInvalidOperation}
	var se *SaveError
	if !errors.Is(err, ErrInvalidOperation) || !errors.As(err, &se) || errors.Is(err, ErrWriteDenied) {
		t.Errorf("SaveError = %v", err)
	}
}
