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
// the old form: a relation in props, where the newest format has it as a field
// of the task. The finding is task.legacy_field, and hh doctor --fix moves it,
// with the function that reads a task of a past commit (domain/upgrade).

// legacyKind is the kind of the repair that moves what an older hedgehug left
// in props to the field of the task.
const legacyKind = "legacy_field"

// legacyNames are the names in the props of t that are relations of a task in
// the newest format.
func legacyNames(t *model.Task) []string {
	var out []string
	for _, f := range t.Props {
		if slices.Contains(upgrade.LegacyRelationNames, f.Key) {
			out = append(out, f.Key)
		}
	}
	return out
}

// legacyFieldFindings are the tasks that hold a relation in props. --fix moves
// it when the values are ULIDs and the field of the task has none; the hint
// says what is to be done where it does not.
func legacyFieldFindings(ix *index.Index) []usecase.Finding {
	var out []usecase.Finding
	for _, e := range ix.Tasks() {
		if e.Broken != "" {
			continue
		}
		for _, p := range e.Props {
			if !slices.Contains(upgrade.LegacyRelationNames, p.Key) {
				continue
			}
			msg, hint := "is a relation of the task in this format, not a property; an older hedgehug wrote it", "hedgehug doctor --fix moves it to the field "+p.Key
			switch {
			case len(e.Targets(p.Key)) > 0:
				hint = "the field " + p.Key + " has values of its own, so which is newer cannot be told and --fix leaves it; decide with hedgehug edit"
			case !onlyULIDs(p.Values):
				hint = "a value is not the ULID of a task, so --fix leaves it; fix it with hedgehug edit"
			}
			field := "props." + p.Key
			out = append(out, usecase.TaskFinding(e, index.Problem{Code: "legacy_field", Field: field, Msg: msg}, field+": "+msg, hint))
		}
	}
	return out
}

func onlyULIDs(values []string) bool {
	for _, v := range values {
		if _, err := model.ParseULID(v); err != nil {
			return false
		}
	}
	return len(values) > 0
}

// fixLegacyFields moves, in one commit, the relations that tasks hold in props
// to their fields, as hh store convert does, and says which task lost which. A
// task whose field has values of its own is left as it is: which is newer
// cannot be told, and the finding stays.
func fixLegacyFields(ss *usecase.Session, s *store.Store) ([]Fixed, error) {
	files := map[string][]byte{}
	var out []Fixed
	for t, err := range s.Tasks() {
		if err != nil || len(legacyNames(t)) == 0 || len(upgrade.LegacyConflicts(t)) > 0 {
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
		out = append(out, Fixed{
			ULID: t.ULID.String(), ID: t.ID, Title: t.Title, Kind: legacyKind, Value: value,
			Line: fmt.Sprintf("repaired: %s: %s props.%s moved to the fields of the task", name, legacyKind, strings.Join(moved, ", props.")),
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
