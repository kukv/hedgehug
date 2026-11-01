// Package view reads, writes and checks the saved views of a store:
// views/<name>.yaml, a layout and a query with the settings of each layout.
package view

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Builtins are the fields of a task that a table column can name besides
// the properties of the schema. They are named as the keys of the query.
var Builtins = []string{"id", "title", "kind", "status", "assignee", "label", "parent", "created", "updated"}

// IsBuiltin reports whether col is one of Builtins.
func IsBuiltin(col string) bool { return slices.Contains(Builtins, col) }

// reserved are the names Windows refuses as file names, whatever the
// extension.
var reserved = []string{
	"CON", "PRN", "AUX", "NUL",
	"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
	"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
}

// ValidName reports whether name can name a view on every system: 1 to 64
// characters, none of / \ : * ? " < > | or a control character, no space
// or dot at either end, and not a name Windows reserves.
func ValidName(name string) error {
	n := utf8.RuneCountInString(name)
	base, _, _ := strings.Cut(name, ".")
	switch {
	case !utf8.ValidString(name):
		return fmt.Errorf("view name %q is not valid UTF-8", name)
	case n == 0:
		return errors.New("view name is empty")
	case n > 64:
		return fmt.Errorf("view name %q is longer than 64 characters", name)
	case strings.ContainsAny(name, `/\:*?"<>|`):
		return fmt.Errorf(`view name %q contains one of / \ : * ? " < > |`, name)
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return fmt.Errorf("view name %q contains a control character", name)
	case strings.TrimFunc(name, unicode.IsSpace) != name:
		return fmt.Errorf("view name %q starts or ends with a space", name)
	case strings.HasPrefix(name, ".") || strings.HasSuffix(name, "."):
		return fmt.Errorf("view name %q starts or ends with a dot", name)
	case slices.Contains(reserved, strings.ToUpper(strings.TrimRightFunc(base, unicode.IsSpace))):
		return fmt.Errorf("view name %q is reserved on Windows", name)
	}
	return nil
}

// SameName reports whether a and b name the same view: names that differ
// only in case are one name, since a file system may not tell them apart.
func SameName(a, b string) bool { return strings.EqualFold(a, b) }
