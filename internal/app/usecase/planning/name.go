// Package planning is the operations on the sprints and the releases of a
// store.
package planning

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// reservedNames are the words a query or --to reads as something else.
var reservedNames = []string{"current", "next", "none", "backlog"}

const maxNameLen = 64

// badName refuses a name that a query could not write: what is sprint or
// release.
func badName(what, name string) error {
	bad := func(why string) error {
		return usecase.NewErr(usecase.KindUsage, what+".bad_name", fmt.Errorf("%q cannot name a %s: %s", name, what, why), "value", name)
	}
	switch {
	case name == "" || utf8.RuneCountInString(name) > maxNameLen:
		return bad(fmt.Sprintf("use 1 to %d characters", maxNameLen))
	case name != strings.TrimSpace(name):
		return bad("it starts or ends with a space")
	case strings.ContainsAny(name, `,"`) || strings.ContainsFunc(name, unicode.IsControl):
		return bad(`a comma, a double quote or a control character is not allowed`)
	case slices.Contains(reservedNames, strings.ToLower(name)):
		return bad(fmt.Sprintf("%s is a reserved word", strings.ToLower(name)))
	}
	return nil
}

// writeWorkspace is the workspace a new sprint or release goes to: arg, the
// context, or the only one of the store.
func writeWorkspace(ss *usecase.Session, ix *index.Index, arg string) (model.ULID, error) {
	ws, err := usecase.PickWorkspace(ss, ix, arg)
	if err != nil || !ws.IsZero() {
		return ws, err
	}
	return usecase.OnlyWorkspace(ix)
}

// parseDate reads a date of key; an empty value is no date.
func parseDate(key, v string) (model.Date, error) {
	if v == "" {
		return model.Date{}, nil
	}
	d, err := model.ParseDate(v)
	if err != nil {
		return model.Date{}, usecase.NewErr(usecase.KindUsage, "assign.bad_value", fmt.Errorf("%s: %q is not a date (YYYY-MM-DD)", key, v), "key", key, "value", v)
	}
	return d, nil
}

func dateText(d model.Date) string {
	if d.IsZero() {
		return ""
	}
	return d.String()
}

// workspaceRef names the workspace ws for a view.
func workspaceRef(ix *index.Index, ws model.ULID) usecase.WorkspaceRef {
	r := usecase.WorkspaceRef{ULID: ws.String()}
	if w, ok := ix.Workspace(ws); ok {
		r.Name, r.IDPrefix, r.Known = w.Name, w.IDPrefix, true
	}
	return r
}
