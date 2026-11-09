package maintenance

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// An older hedgehug that pushes to a store in a newer format writes a task in
// the old form: a relation, or a priority, start, due, estimate or points, in
// props, where the newest format has it as a field of the task. The finding is
// task.legacy_field, and hh doctor --fix moves it, with the functions that read
// a task of a past commit (domain/upgrade).

// legacyKind is the kind of the repair that moves what an older hedgehug left
// in props to the field of the task.
const legacyKind = "legacy_field"

// isLegacyName is whether name is a relation or one of the five built-in
// properties, which a task of the newest format holds as fields.
func isLegacyName(name string) bool {
	return slices.Contains(upgrade.LegacyRelationNames, name) || slices.Contains(upgrade.LegacyBuiltinNames, name)
}

// legacyNames are the names in the props of t that are fields of a task in the
// newest format.
func legacyNames(t *model.Task) []string {
	var out []string
	for _, f := range t.Props {
		if isLegacyName(f.Key) {
			out = append(out, f.Key)
		}
	}
	return out
}

// legacyFieldFindings are the tasks that hold a relation or one of the five
// built-in properties in props. --fix moves them when the relations are ULIDs,
// the other values are of the type of their field and the field of the task has
// none; the hint says what is to be done where it does not.
func legacyFieldFindings(s *store.Store, ix *index.Index) []usecase.Finding {
	var out []usecase.Finding
	for _, e := range ix.Tasks() {
		if e.Broken != "" || !slices.ContainsFunc(e.Props, func(p index.Prop) bool { return isLegacyName(p.Key) }) {
			continue
		}
		t, err := s.Task(e.ULID)
		if err != nil {
			continue
		}
		// --fix moves a task whole or not at all: what keeps it from moving
		// is said on each of its findings, in the hint and in the args
		hint := "hedgehug doctor --fix moves it to the field of the task"
		reason, names, why := legacyBlocker(t, ix.Schema().Priorities)
		if why != "" {
			hint = "hedgehug doctor --fix leaves this task as it is, because " + why + "; decide with hedgehug edit"
		}
		for _, name := range legacyNames(t) {
			msg := "is a field of the task in this format, not a property; an older hedgehug wrote it"
			field := "props." + name
			f := usecase.TaskFinding(e, index.Problem{Code: "legacy_field", Field: field, Msg: msg}, field+": "+msg, hint)
			// args: the field, the message, then why --fix leaves the task
			// (conflict, bad_value, or "" when it moves it) and the names
			// that keep it, without props.
			f.Args = append(f.Args, reason)
			f.Args = append(f.Args, names...)
			out = append(out, f)
		}
	}
	return out
}

// legacyBlocker says why --fix does not move the fields of t: the reason
// (conflict or bad_value), the names that cause it, and the reason in words.
// All are empty when it does move them. priorities is the list of the schema.
func legacyBlocker(t *model.Task, priorities []string) (reason string, names []string, why string) {
	if c := upgrade.LegacyConflicts(t); len(c) > 0 {
		return "conflict", c, "the field " + c[0] + " has values of its own, so which is newer cannot be told"
	}
	if c := upgrade.LegacyBuiltinConflicts(t); len(c) > 0 {
		return "conflict", c, "the field " + c[0] + " has a value of its own, so which is newer cannot be told"
	}
	if b := upgrade.LegacyBadValues(t); len(b) > 0 {
		return "bad_value", b, "props." + strings.Join(b, ", props.") + " has a value that is not the ULID of a task"
	}
	if b := upgrade.LegacyBuiltinBadValues(t, priorities); len(b) > 0 {
		return "bad_value", b, "props." + strings.Join(b, ", props.") + " has a value that is not of the type of the field"
	}
	return "", nil, ""
}

func legacyBlocked(t *model.Task, priorities []string) bool {
	_, _, why := legacyBlocker(t, priorities)
	return why != ""
}

// fixLegacyFields moves, in one commit, the relations and the five built-in
// properties that tasks hold in props to their fields, as hh store convert does,
// and says which task lost which, and the time of a date that was made of one.
// A task whose field has values of its own, or a value that cannot be read as
// the type of its field, is left as it is: nothing is moved without losing
// something, and the finding stays.
func fixLegacyFields(ss *usecase.Session, s *store.Store) ([]Fixed, error) {
	files := map[string][]byte{}
	var out []Fixed
	priorities := s.Schema().Priorities
	for t, err := range s.Tasks() {
		if err != nil || len(legacyNames(t)) == 0 || legacyBlocked(t, priorities) {
			continue
		}
		raw, err := s.TaskBytes(t.ULID)
		if err != nil {
			return nil, err
		}
		fixed, err := upgrade.MoveLegacyFields(raw)
		if err != nil {
			return nil, err
		}
		fixed, cut, err := upgrade.MoveLegacyBuiltins(fixed, priorities)
		if err != nil {
			return nil, err
		}
		if string(fixed) == string(raw) {
			continue
		}
		after, err := model.DecodeTask(fixed)
		if err != nil {
			return nil, err
		}
		var moved []string
		for _, n := range legacyNames(t) {
			if !slices.Contains(legacyNames(after), n) {
				moved = append(moved, n)
			}
		}
		files[model.TaskPath(t.ULID)] = fixed
		name := t.ID
		if name == "" {
			name = t.ULID.String()
		}
		value, _ := json.Marshal(moved) // a list of strings always encodes
		line := fmt.Sprintf("repaired: %s: %s props.%s moved to the fields of the task", name, legacyKind, strings.Join(moved, ", props."))
		for _, c := range cut { // a date made of a time says what it lost
			line += fmt.Sprintf("; the time of props.%s was left out: %s", c.Prop, c.Value)
		}
		out = append(out, Fixed{
			ULID: t.ULID.String(), ID: t.ID, Title: t.Title, Kind: legacyKind, Value: value, Line: line,
		})
	}
	if len(files) == 0 {
		return nil, nil
	}
	head, err := s.Repo().Head()
	if err != nil {
		return nil, err
	}
	if _, err := ss.CommitFiles(s, head, files, store.Op{Name: "repair", Detail: legacyKind}); err != nil {
		return nil, err
	}
	return out, nil
}
