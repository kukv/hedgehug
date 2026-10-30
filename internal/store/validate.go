package store

import (
	"strings"

	"github.com/kukv/hedgehug/internal/schema"
)

// SchemaError means a schema written with PutSchema has problems.
type SchemaError struct{ Problems []schema.Problem }

func (e *SchemaError) Error() string {
	msgs := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		msgs[i] = p.String()
	}
	return "invalid schema: " + strings.Join(msgs, "; ")
}
