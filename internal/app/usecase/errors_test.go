package usecase_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
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

func TestDescribe(t *testing.T) {
	t.Parallel()
	ve := &store.ValidationError{Violations: []schema.Violation{{Field: "kind", Msg: "bug is not a kind"}, {Field: "title", Msg: "is empty"}}}
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"violations", fmt.Errorf("wrapped: %w", ve), "the change breaks the schema:\n  kind: bug is not a kind\n  title: is empty"},
		{"locked", fmt.Errorf("x: %w", usecase.ErrLocked), "another hedgehug is writing to the store; try again"},
		{"other", errors.New("boom"), "boom"},
	}
	for _, c := range cases {
		if got := usecase.Describe(c.err); got != c.want {
			t.Errorf("%s: Describe = %q; want %q", c.name, got, c.want)
		}
	}
}

func TestViolationsOf(t *testing.T) {
	t.Parallel()
	vs := []schema.Violation{{Field: "title", Msg: "is empty"}}
	got, ok := usecase.ViolationsOf(fmt.Errorf("w: %w", &store.ValidationError{Violations: vs}))
	if !ok || len(got) != 1 || got[0] != vs[0] {
		t.Errorf("ViolationsOf = %v, %v; want %v, true", got, ok, vs)
	}
	if got, ok := usecase.ViolationsOf(errors.New("x")); ok || got != nil {
		t.Errorf("ViolationsOf(other) = %v, %v; want nil, false", got, ok)
	}
}

func TestProblemCode(t *testing.T) {
	t.Parallel()
	if got := usecase.ProblemCode(usecase.NewErr(usecase.KindNotFound, "task.gone", errors.New("x"))); got != "task.gone" {
		t.Errorf("ProblemCode = %q; want task.gone", got)
	}
	if got := usecase.ProblemCode(fmt.Errorf("w: %w", store.ErrLocked)); got != "store.locked" {
		t.Errorf("ProblemCode(locked) = %q; want store.locked", got)
	}
	if got := usecase.ProblemCode(errors.New("boom")); got != "internal" {
		t.Errorf("ProblemCode(other) = %q; want internal", got)
	}
}
