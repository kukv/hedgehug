package server

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// The store formats have problems of their own: an old store has to be
// converted and a newer one needs a newer hedgehug, which no retry changes
// (422, not 409); a sync that finds the two sides in different formats is a
// conflict.
func TestProblemOfTheStoreFormats(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		err    error
		status int
		typ    string
	}{
		{model.ErrOldFormat, http.StatusUnprocessableEntity, "store.old_format"},
		{fmt.Errorf("cannot open me: %w", model.ErrNewerFormat), http.StatusUnprocessableEntity, "store.newer_format"},
		{fmt.Errorf("sync me: %w", usecase.ErrFormatDiverged), http.StatusConflict, "sync.format_diverged"},
		{usecase.NewErr(usecase.KindConflict, "store.convert_unsynced", errors.New("not in sync")), http.StatusConflict, "store.convert_unsynced"},
	} {
		if p := problemOf(c.err); p.Status != c.status || p.Type != c.typ {
			t.Errorf("problemOf(%v) = %d %q, want %d %q", c.err, p.Status, p.Type, c.status, c.typ)
		}
	}
	if p := problemOf(model.ErrOldFormat); p.Detail != "hh store convert で新しい形式にしてください" {
		t.Errorf("detail = %q", p.Detail)
	}
}
