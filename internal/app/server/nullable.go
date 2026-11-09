package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
)

// Nullable is a field of a request that may be left out, null or a value: a
// PATCH clears a field with null and leaves out one it does not change, which
// a pointer cannot tell apart.
type Nullable[T any] struct {
	Set   bool // the request has the field, null included
	Value *T   // nil for null
}

// UnmarshalJSON reads a value, or null.
func (n *Nullable[T]) UnmarshalJSON(data []byte) error {
	n.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		n.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	n.Value = &v
	return nil
}

// Schema describes the field as a T that may also be null.
func (Nullable[T]) Schema(r huma.Registry) *huma.Schema {
	s := *huma.SchemaFromType(r, reflect.TypeFor[T]())
	s.Nullable = true
	return &s
}

// given is the field of a request that has v, when v is not nil.
func given[T any](v *T) Nullable[T] { return Nullable[T]{Set: v != nil, Value: v} }

// text is a number field as the text hedgehug set reads.
func (n Nullable[T]) text() Nullable[string] {
	out := Nullable[string]{Set: n.Set}
	if n.Value != nil {
		s := fmt.Sprint(*n.Value)
		out.Value = &s
	}
	return out
}
