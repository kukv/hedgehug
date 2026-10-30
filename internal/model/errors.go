// Package model holds Hedgehug's file types and their codecs. It reads and
// writes bytes only: no disk, locks or git.
package model

import "fmt"

// DecodeError reports a field that could not be decoded. Line is the line in
// the file, or 0 when unknown.
type DecodeError struct {
	Field string
	Line  int
	Err   error
}

func (e *DecodeError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s (line %d): %v", e.Field, e.Line, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Field, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }
