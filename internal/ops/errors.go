// Package ops holds the operations of hedgehug that the CLI and the GUI
// server share. They return typed results and print nothing.
package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
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

// newErr wraps err; args are key, value pairs.
func newErr(k Kind, code string, err error, args ...any) *Error {
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
	return classify(err)
}

// KindOf is the Kind of err: that of its AsError.
func KindOf(err error) Kind {
	if e := AsError(err); e != nil {
		return e.Kind
	}
	return KindUnknown
}

// classify gives the errors of the lower layers a Kind and a Code.
func classify(err error) *Error {
	var ae *index.AmbiguousError
	var ve *store.ValidationError
	var wt *store.WorkspaceTakenError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ae):
		return newErr(KindAmbiguous, "task.ambiguous", err, "input", ae.Input)
	case errors.Is(err, store.ErrNotFound):
		return newErr(KindNotFound, "not_found", err)
	case errors.As(err, &ve):
		return newErr(KindInvalid, "schema.violation", err)
	case errors.As(err, &wt):
		return newErr(KindConflict, "workspace.taken", err)
	case errors.Is(err, store.ErrLocked):
		return newErr(KindConflict, "store.locked", err)
	case errors.Is(err, sync.ErrDirty):
		return newErr(KindConflict, "sync.dirty", err)
	case errors.Is(err, sync.ErrUnrelated):
		return newErr(KindConflict, "sync.unrelated", err)
	case errors.Is(err, sync.ErrForcePush):
		return newErr(KindConflict, "sync.force_push", err)
	case errors.Is(err, config.ErrNoStores):
		return newErr(KindConflict, "store.none", err)
	}
	return &Error{Kind: KindUnknown, Err: err}
}

// ErrFindings is returned with a result that found problems (auth status):
// the result is complete, and the CLI exits 1 once it has printed it.
var ErrFindings = errors.New("problems found")

// choiceRequired is what an operation returns when it needs the caller to
// pick one of candidates and none was given.
func choiceRequired(what string, candidates []string) *Error {
	return newErr(KindUsage, "choice.required",
		fmt.Errorf("%s is needed; pick one of: %s", what, strings.Join(candidates, ", ")),
		"what", what, "candidates", candidates)
}

// confirmRequired is what an operation returns when it needs a confirmation
// and the caller gave none (--yes).
func confirmRequired(what string) *Error {
	return newErr(KindUsage, "confirm.required", fmt.Errorf("%s: confirm with --yes", what), "what", what)
}
