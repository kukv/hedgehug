package model

import (
	"fmt"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"
)

// RewritesPath is the record of the history rewrites of a store.
const RewritesPath = ".hedgehug/rewrites.yaml"

// Rewrite is one record of rewrites.yaml: a history rewrite that removed
// tasks (hedgehug purge --rewrite-history).
type Rewrite struct {
	At     time.Time `yaml:"at"`
	Before string    `yaml:"before"` // the head of origin before the rewrite
	ULIDs  []ULID    `yaml:"ulids"`
}

// DecodeRewrites decodes rewrites.yaml, an array of records. An empty file
// has none.
func DecodeRewrites(data []byte) ([]Rewrite, error) {
	var rs []Rewrite
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("%s: %w", RewritesPath, err)
	}
	for i, r := range rs {
		if r.At.IsZero() || r.Before == "" {
			return nil, fmt.Errorf("%s: record %d needs at and before", RewritesPath, i+1)
		}
	}
	return rs, nil
}

// EncodeRewrites writes the records, after a comment saying what the file
// is.
func EncodeRewrites(rs []Rewrite) ([]byte, error) {
	var b strings.Builder
	b.WriteString("# The history rewrites of this store. Clones use it to take a rewrite in.\n")
	if len(rs) == 0 {
		b.WriteString("[]\n")
		return []byte(b.String()), nil
	}
	out, err := yaml.Marshal(rs)
	if err != nil {
		return nil, err
	}
	b.Write(out)
	return []byte(b.String()), nil
}
