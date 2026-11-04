package usecase

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/store"
)

func TestErrorKeepsMessage(t *testing.T) {
	t.Parallel()
	base := errors.New("ME-1 is already deleted")
	e := NewErr(KindConflict, "task.deleted", base, "id", "ME-1")
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
	if got := KindOf(nil); got != KindUnknown {
		t.Errorf("KindOf(nil) = %v; want KindUnknown", got)
	}
	if Classify(nil) != nil {
		t.Error("classify(nil) is not nil")
	}
}

func TestKindOfClassifiesLowerLayers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		want Kind
	}{
		{fmt.Errorf("x: %w", store.ErrNotFound), KindNotFound},
		{store.ErrLocked, KindConflict},
		{&store.ValidationError{}, KindInvalid},
		{errors.New("other"), KindUnknown},
		{NewErr(KindUsage, "args.bad", errors.New("u")), KindUsage},
	}
	for _, c := range cases {
		if got := KindOf(c.err); got != c.want {
			t.Errorf("KindOf(%v) = %v; want %v", c.err, got, c.want)
		}
	}
}
