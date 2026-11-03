package model

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// NumbersPath is the record of the numbers of purged tasks.
const NumbersPath = ".hedgehug/numbers.yaml"

// Numbers is, for each id prefix, the largest number of a purged task.
// Numbering counts it, so a purged task's number is never given again.
type Numbers map[string]int

// DecodeNumbers decodes numbers.yaml: a map from id prefix to number. An
// empty file is an empty record. The prefixes and numbers are not checked
// here; doctor reports the bad ones.
func DecodeNumbers(data []byte) (Numbers, error) {
	n := Numbers{}
	if err := yaml.Unmarshal(data, &n); err != nil {
		return nil, fmt.Errorf("%s: %w", NumbersPath, err)
	}
	if n == nil {
		n = Numbers{}
	}
	return n, nil
}

// Encode writes the prefixes in order, after a comment saying what the
// file is.
func (n Numbers) Encode() ([]byte, error) {
	var b strings.Builder
	b.WriteString("# The largest number of the purged tasks, for each id prefix. Numbering\n# never gives these numbers again.\n")
	keys := make([]string, 0, len(n))
	for k := range n {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %d\n", k, n[k])
	}
	return []byte(b.String()), nil
}
