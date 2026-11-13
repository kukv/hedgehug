package apierr_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func TestFrom(t *testing.T) {
	var se huma.StatusError
	if err := apierr.From(fmt.Errorf("x: %w", refusal.New("タイトルを入力してください"))); !errors.As(err, &se) || se.GetStatus() != http.StatusUnprocessableEntity || se.Error() != "タイトルを入力してください" {
		t.Errorf("refusal: %v", err)
	}
	if err := apierr.From(port.ErrStoreNotFound); !errors.As(err, &se) || se.GetStatus() != http.StatusNotFound {
		t.Errorf("not found: %v", err)
	}
	if err := apierr.From(fmt.Errorf("x: %w", port.ErrInvalidOperation)); !errors.As(err, &se) || se.GetStatus() != http.StatusBadRequest {
		t.Errorf("invalid operation: %v", err)
	}
	boom := errors.New("boom")
	if err := apierr.From(boom); err != boom {
		t.Errorf("other: %v", err)
	}
	if apierr.From(nil) != nil {
		t.Error("nil")
	}
}
