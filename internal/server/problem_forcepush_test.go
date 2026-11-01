package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/kukv/hedgehug/internal/sync"
)

// A force push nobody recorded is something to check, not a fault of the
// server: it is a conflict, not a 500.
func TestProblemOfAnUnexpectedForcePushIsAConflict(t *testing.T) {
	t.Parallel()
	p := problemOf(fmt.Errorf("sync org-a: %w", sync.ErrForcePush))
	if p.Status != http.StatusConflict || p.Type != "sync.force_push" {
		t.Errorf("problem = %d %q, want 409 sync.force_push", p.Status, p.Type)
	}
}
