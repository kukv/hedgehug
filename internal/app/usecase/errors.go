// Package usecase holds the operations of hedgehug that the CLI and the GUI
// server share. They return typed results and print nothing.
package usecase

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// Kind is what an error means to the caller: the CLI turns it into an exit
// code, the server into an HTTP status.
type Kind int

const (
	KindUnknown Kind = iota
	KindNotFound
	KindAmbiguous
	KindInvalid
	KindConflict
	KindUsage
)

// Error is an error of an operation. Its message is the wrapped error's, so
// the CLI prints what it printed before; Code and Args let the GUI choose
// its own words.
type Error struct {
	Kind Kind
	Code string
	Args map[string]any
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// NewErr wraps err; args are key, value pairs.
func NewErr(k Kind, code string, err error, args ...any) *Error {
	e := &Error{Kind: k, Code: code, Err: err}
	if len(args) > 0 {
		e.Args = map[string]any{}
		for i := 0; i+1 < len(args); i += 2 {
			e.Args[args[i].(string)] = args[i+1]
		}
	}
	return e
}

// AsError is the *Error that err stands for: the outermost one in its chain,
// or else what classify makes of the error of a lower layer. It is nil for a
// nil err, and has KindUnknown for an error nothing classifies. The server
// reads Kind, Code and Args from it.
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Classify(err)
}

// KindOf is the Kind of err: that of its AsError.
func KindOf(err error) Kind {
	if e := AsError(err); e != nil {
		return e.Kind
	}
	return KindUnknown
}

// Classify gives the errors of the lower layers a Kind and a Code.
func Classify(err error) *Error {
	var ae *index.AmbiguousError
	var ve *store.ValidationError
	var wt *store.WorkspaceTakenError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ae):
		return NewErr(KindAmbiguous, "task.ambiguous", err, "input", ae.Input)
	case errors.Is(err, store.ErrNotFound):
		return NewErr(KindNotFound, "not_found", err)
	case errors.As(err, &ve):
		return NewErr(KindInvalid, "schema.violation", err)
	case errors.As(err, &wt):
		return NewErr(KindConflict, "workspace.taken", err)
	case errors.Is(err, store.ErrLocked):
		return NewErr(KindConflict, "store.locked", err)
	case errors.Is(err, store.ErrStoreGone):
		return NewErr(KindConflict, "store.gone", err)
	case errors.Is(err, replicate.ErrDirty):
		return NewErr(KindConflict, "sync.dirty", err)
	case errors.Is(err, replicate.ErrUnrelated):
		return NewErr(KindConflict, "sync.unrelated", err)
	case errors.Is(err, replicate.ErrForcePush):
		return NewErr(KindConflict, "sync.force_push", err)
	case errors.Is(err, config.ErrNoStores):
		return NewErr(KindConflict, "store.none", err)
	}
	return &Error{Kind: KindUnknown, Err: err}
}

// ErrFindings is returned with a result that found problems (auth status):
// the result is complete, and the CLI exits 1 once it has printed it.
var ErrFindings = errors.New("problems found")

// ChoiceRequired is what an operation returns when it needs the caller to
// pick one of candidates and none was given.
func ChoiceRequired(what string, candidates []string) *Error {
	return NewErr(KindUsage, "choice.required",
		fmt.Errorf("%s is needed; pick one of: %s", what, strings.Join(candidates, ", ")),
		"what", what, "candidates", candidates)
}

// ConfirmRequired is what an operation returns when it needs a confirmation
// and the caller gave none (--yes).
func ConfirmRequired(what string) *Error {
	return NewErr(KindUsage, "confirm.required", fmt.Errorf("%s: confirm with --yes", what), "what", what)
}

// ErrLocked is the error of a write that found the store locked by another
// hedgehug.
var ErrLocked = store.ErrLocked

// ViolationsOf is the violations of a write the schema refused, or ok=false
// when err is not that.
func ViolationsOf(err error) (vs []schema.Violation, ok bool) {
	var ve *store.ValidationError
	if errors.As(err, &ve) {
		return ve.Violations, true
	}
	return nil, false
}

// ViolationsText is the text of a refused write: what it breaks, and the
// violations on the lines after it.
func ViolationsText(vs []schema.Violation) string {
	var b strings.Builder
	b.WriteString("the change breaks the schema:")
	for _, v := range vs {
		b.WriteString("\n  " + v.String())
	}
	return b.String()
}

// Describe is err as the CLI prints it after "hedgehug: " and the server
// gives it as the detail of a problem.
func Describe(err error) string {
	if vs, ok := ViolationsOf(err); ok {
		return ViolationsText(vs)
	}
	if errors.Is(err, ErrLocked) {
		return "another hedgehug is writing to the store; try again"
	}
	return err.Error()
}

// ProblemCode is the type of the problem err makes: the Code of its AsError,
// or "internal" when there is none.
func ProblemCode(err error) string {
	if e := AsError(err); e != nil && e.Code != "" {
		return e.Code
	}
	return "internal"
}
