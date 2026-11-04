package usecase_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

func TestErrorKeepsMessage(t *testing.T) {
	t.Parallel()
	base := errors.New("ME-1 is already deleted")
	e := usecase.NewErr(usecase.KindConflict, "task.deleted", base, "id", "ME-1")
	if e.Error() != base.Error() {
		t.Errorf("Error() = %q; want %q", e.Error(), base.Error())
	}
	if !errors.Is(e, base) {
		t.Error("errors.Is does not see the wrapped error")
	}
	if e.Args["id"] != "ME-1" {
		t.Errorf("Args = %v", e.Args)
	}
}

func TestKindOfNil(t *testing.T) {
	t.Parallel()
	if got := usecase.KindOf(nil); got != usecase.KindUnknown {
		t.Errorf("KindOf(nil) = %v; want KindUnknown", got)
	}
	if usecase.Classify(nil) != nil {
		t.Error("classify(nil) is not nil")
	}
}

func TestKindOfClassifiesLowerLayers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		want usecase.Kind
	}{
		{fmt.Errorf("x: %w", store.ErrNotFound), usecase.KindNotFound},
		{store.ErrLocked, usecase.KindConflict},
		{&store.ValidationError{}, usecase.KindInvalid},
		{errors.New("other"), usecase.KindUnknown},
		{usecase.NewErr(usecase.KindUsage, "args.bad", errors.New("u")), usecase.KindUsage},
	}
	for _, c := range cases {
		if got := usecase.KindOf(c.err); got != c.want {
			t.Errorf("KindOf(%v) = %v; want %v", c.err, got, c.want)
		}
	}
}
