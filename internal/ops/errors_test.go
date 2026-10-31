package ops

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/store"
)

func TestErrorKeepsMessage(t *testing.T) {
	base := errors.New("ME-1 is already deleted")
	e := newErr(KindConflict, "task.deleted", base, "id", "ME-1")
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

func TestKindOfClassifiesLowerLayers(t *testing.T) {
	cases := []struct {
		err  error
		want Kind
	}{
		{fmt.Errorf("x: %w", store.ErrNotFound), KindNotFound},
		{store.ErrLocked, KindConflict},
		{&store.ValidationError{}, KindInvalid},
		{errors.New("other"), KindUnknown},
		{newErr(KindUsage, "args.bad", errors.New("u")), KindUsage},
	}
	for _, c := range cases {
		if got := KindOf(c.err); got != c.want {
			t.Errorf("KindOf(%v) = %v; want %v", c.err, got, c.want)
		}
	}
}
