// Package ops holds the operations of hedgehug that the CLI and the GUI
// server share. They return typed results and print nothing.
package ops

import (
	"errors"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/store"
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

// KindOf is the Kind of err: that of the outermost *Error, or the one the
// error of a lower layer stands for.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	if c := classify(err); c != nil {
		return c.Kind
	}
	return KindUnknown
}

// classify gives the errors of the lower layers a Kind and a Code.
func classify(err error) *Error {
	var ae *index.AmbiguousError
	var ve *store.ValidationError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ae):
		return newErr(KindAmbiguous, "task.ambiguous", err, "input", ae.Input)
	case errors.Is(err, store.ErrNotFound):
		return newErr(KindNotFound, "task.not_found", err)
	case errors.As(err, &ve):
		return newErr(KindInvalid, "schema.violation", err)
	case errors.Is(err, store.ErrLocked):
		return newErr(KindConflict, "store.locked", err)
	case errors.Is(err, config.ErrNoStores):
		return newErr(KindConflict, "store.none", err)
	}
	return &Error{Kind: KindUnknown, Err: err}
}
