package usecase_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
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

func TestAsError(t *testing.T) {
	t.Parallel()
	if usecase.AsError(nil) != nil {
		t.Error("AsError(nil) is not nil")
	}
	// The outermost *Error of the chain.
	inner := usecase.NewErr(usecase.KindNotFound, "task.not_found", errors.New("x"), "input", "ME-9")
	if got := usecase.AsError(fmt.Errorf("wrapped: %w", inner)); got != inner {
		t.Errorf("AsError(wrapped) = %+v; want the inner *Error", got)
	}
	// A lower-layer error is classified: Kind and Code from the chain.
	cases := []struct {
		err  error
		kind usecase.Kind
		code string
	}{
		{fmt.Errorf("x: %w", store.ErrNotFound), usecase.KindNotFound, "not_found"},
		{&store.ValidationError{}, usecase.KindInvalid, "schema.violation"},
		{store.ErrLocked, usecase.KindConflict, "store.locked"},
		{store.ErrStoreGone, usecase.KindConflict, "store.gone"},
		{fmt.Errorf("a: %w", replicate.ErrDirty), usecase.KindConflict, "sync.dirty"},
		{fmt.Errorf("a: %w", replicate.ErrUnrelated), usecase.KindConflict, "sync.unrelated"},
		{fmt.Errorf("a: %w", replicate.ErrForcePush), usecase.KindConflict, "sync.force_push"},
		{fmt.Errorf("a: %w", replicate.ErrFormatDiverged), usecase.KindConflict, "sync.format_diverged"},
		{model.ErrOldFormat, usecase.KindInvalid, "store.old_format"},
		{fmt.Errorf("a: %w", model.ErrNewerFormat), usecase.KindInvalid, "store.newer_format"},
		{config.ErrNoStores, usecase.KindConflict, "store.none"},
		{errors.New("other"), usecase.KindUnknown, ""},
	}
	for _, c := range cases {
		got := usecase.AsError(c.err)
		if got == nil || got.Kind != c.kind || got.Code != c.code || got.Error() != c.err.Error() {
			t.Errorf("AsError(%v) = %+v; want %v %q with the same message", c.err, got, c.kind, c.code)
		}
		if usecase.KindOf(c.err) != c.kind {
			t.Errorf("KindOf(%v) = %v; want %v", c.err, usecase.KindOf(c.err), c.kind)
		}
	}
}

// A task that is not found says task.not_found; a store error for something
// else does not.
func TestNotFoundCodes(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.Resolve(ix, "ME-9")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "task.not_found")
	if e := usecase.AsError(err); e.Args["input"] != "ME-9" {
		t.Errorf("Args = %v; want input ME-9", e.Args)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.TaskBytes(ix.Workspaces()[0].ULID) // no task has this ULID
	usecasetest.WantErr(t, err, usecase.KindNotFound, "not_found")
	_, err = comments.OpenComment(ts.Session, "01JZZZZZZZZZZZZZZZZZZZZZZZ")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "comment.not_found")
	_, err = usecase.ResolveWorkspace(ix, "nope")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "workspace.not_found")
	_, err = notifications.InboxRead(ts.Session, []string{"01JZZZZZZZZZZZZZZZZZZZZZZZ"}, false)
	usecasetest.WantErr(t, err, usecase.KindNotFound, "inbox.not_found")
}
