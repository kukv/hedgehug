package ghcall_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func TestIs(t *testing.T) {
	err := fmt.Errorf("read: %w", &ghcall.Failure{Kind: ghcall.NotFound, Repo: "org/web", Number: 3})
	if !ghcall.Is(err, ghcall.NotFound) || ghcall.Is(err, ghcall.Unreachable) {
		t.Error("Is does not see the wrapped failure")
	}
	if ghcall.Is(errors.New("x"), ghcall.NotFound) {
		t.Error("Is matched a plain error")
	}
}

func TestErrorNamesOnlyKindAndRepo(t *testing.T) {
	f := &ghcall.Failure{Kind: ghcall.NotFound, Repo: "org/web", Number: 3}
	if got := f.Error(); got != "github: not_found (org/web#3)" {
		t.Errorf("Error() = %q", got)
	}
	if got := (&ghcall.Failure{Kind: ghcall.Unreachable}).Error(); got != "github: unreachable" {
		t.Errorf("Error() = %q", got)
	}
}
