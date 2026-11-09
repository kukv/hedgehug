package usecase

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// queryKeys are the keys that queries read before they read properties.
var queryKeys = []string{
	"project", "is", "sort", "group", "blocks", "label", "assignee",
	"causes", "relates", "duplicates", "clones",
	"caused_by", "related_by", "duplicated_by", "cloned_by",
}

// schemaCodes are the problems of a task that come from the schema.
var schemaCodes = []string{
	schema.CodeUnknownKind, schema.CodeUnknownStatus, schema.CodeMissingTitle,
	schema.CodeBadValue, schema.CodePlacement, schema.CodeRequired,
}

const BrokenSchemaHint = "restore schema.yaml with `git restore schema.yaml`, or fix it by hand and commit"

// CheckSchemaFile decodes schema.yaml and checks it. The schema is nil when data
// does not decode; the findings are then that alone.
func CheckSchemaFile(data []byte) (*schema.Schema, []Finding) {
	sc, err := schema.Decode(data)
	if err != nil {
		return nil, []Finding{{Check: "schema", Where: schema.Path, Msg: err.Error(), Hint: BrokenSchemaHint, Code: "schema.unreadable", Args: []string{err.Error()}}}
	}
	var fs []Finding
	for _, p := range sc.Check() {
		fs = append(fs, Finding{Check: "schema", Where: schema.Path + ": " + p.Path, Msg: p.Msg, Code: p.Code, Args: p.Args})
	}
	return sc, fs
}

// CheckSchema finds where the schema of s does not fit the store: the problems of
// schema.yaml, templates it names that are missing, properties that queries
// cannot select by, tasks that break the schema, and overrides of projects
// that are gone. The findings are sorted. The error is for the checks of
// doctor that read files; these checks read nothing that can fail.
func CheckSchema(s *store.Store, ix *index.Index) ([]Finding, error) {
	sc := s.Schema()
	var fs []Finding
	for _, p := range sc.Check() {
		fs = append(fs, Finding{Check: "schema", Where: schema.Path + ": " + p.Path, Msg: p.Msg, Code: p.Code, Args: p.Args})
	}
	fs = append(fs, missingTemplates(s, sc)...)
	for _, p := range sc.Props {
		// A name that is a field of the task is refused already, as prop_builtin.
		if slices.Contains(queryKeys, p.Name) && !slices.Contains(model.TaskKeys(), p.Name) {
			fs = append(fs, Finding{
				Check: "prop_name", Where: schema.Path + ": props." + p.Name,
				Msg:  "queries cannot select by a property with this name",
				Hint: "rename the property with `hedgehug schema migrate prop`",
				Code: "prop_name.query_key", Args: []string{p.Name},
			})
		}
	}
	for _, e := range ix.Tasks() {
		for _, p := range ix.Problems(e.ULID) {
			if slices.Contains(schemaCodes, p.Code) {
				fs = append(fs, TaskFinding(e, p, p.Field+": "+p.Msg, ""))
			}
		}
	}
	for i, sco := range sc.Scoped {
		s, ok := strings.CutPrefix(sco.Where, "project:")
		if !ok {
			continue // Check has reported it
		}
		ulid, err := model.ParseULID(s)
		if err != nil {
			continue
		}
		if e, found := ix.Task(ulid); !found || e.DeletedAt != nil {
			fs = append(fs, Finding{
				Check: "override", Where: fmt.Sprintf("%s: scoped[%d].where", schema.Path, i),
				Msg:  sco.Where + " is not a project of this store, or is deleted",
				Hint: "restore the project, or remove the override",
				Code: "override.not_project", Args: []string{sco.Where},
			})
		}
	}
	fs = append(fs, automationFindings(s, sc, ix)...)
	SortFindings(fs)
	return fs, nil
}

// automationFindings are the problems of automations.yaml. A file that does
// not decode is one finding.
func automationFindings(s *store.Store, sc *schema.Schema, ix *index.Index) []Finding {
	f, err := s.Automations()
	if err != nil {
		return []Finding{{
			Check: "automation", Where: model.AutomationsPath, Msg: err.Error(),
			Code: "automation.unreadable", Args: []string{err.Error()},
			Hint: "restore " + model.AutomationsPath + " with `git restore " + model.AutomationsPath + "`, or fix it by hand and commit",
		}}
	}
	isProject := func(u model.ULID) bool {
		e, ok := ix.Task(u)
		return ok && e.DeletedAt == nil && e.Kind == ProjectKind
	}
	var fs []Finding
	for _, p := range f.Check(sc, isProject) {
		fs = append(fs, Finding{Check: "automation", Where: model.AutomationsPath + ": " + p.Path, Msg: p.Msg, Code: p.Code, Args: p.Args})
	}
	return fs
}

// missingTemplates finds the templates that schema.yaml names and
// templates/ does not have.
func missingTemplates(s *store.Store, sc *schema.Schema) []Finding {
	var fs []Finding
	check := func(where, name string) {
		var msg, code string
		args := []string{name}
		if err := model.ValidTemplateName(name); err != nil {
			msg, code = fmt.Sprintf("template name %q is not valid: %v", name, err), "template.bad_name"
			args = append(args, err.Error())
		} else if _, err := s.Template(name); errors.Is(err, store.ErrNotFound) {
			msg, code = fmt.Sprintf("template %q does not exist", name), "template.missing"
		} else if err != nil {
			// The file is there but does not decode; doctor reports that.
			return
		}
		if msg != "" {
			fs = append(fs, Finding{
				Check: "template", Where: schema.Path + ": " + where, Msg: msg,
				Hint: "add templates/" + name + ".md or change the name",
				Code: code, Args: args,
			})
		}
	}
	if sc.Templates != nil {
		for _, kind := range slices.Sorted(maps.Keys(sc.Templates.Default)) {
			check("templates.default."+kind, sc.Templates.Default[kind])
		}
	}
	for i, sco := range sc.Scoped {
		if sco.Templates == nil {
			continue
		}
		for j, n := range sco.Templates.Only {
			check(fmt.Sprintf("scoped[%d].templates.only[%d]", i, j), n)
		}
		for _, kind := range slices.Sorted(maps.Keys(sco.Templates.Default)) {
			check(fmt.Sprintf("scoped[%d].templates.default.%s", i, kind), sco.Templates.Default[kind])
		}
	}
	return fs
}

// TaskWhere names the task e in a finding: its id, ULID and title, or the
// ULID and title when it has no id (a file that does not decode).
// TaskFinding is the finding of the problem p of the task e, with the message
// msg and the hint hint. Its code is task.<the code of p>, its args the field
// and the message of p.
func TaskFinding(e *index.Entry, p index.Problem, msg, hint string) Finding {
	return Finding{
		Check: "task", Where: TaskWhere(e), Task: e.ULID.String(), Msg: msg, Hint: hint,
		Code: "task." + p.Code, Args: []string{p.Field, p.Msg},
	}
}

func TaskWhere(e *index.Entry) string {
	if e.ID == "" {
		return strings.TrimSpace(e.ULID.String() + " " + e.Title)
	}
	return strings.TrimSpace(e.ID + " (" + e.ULID.String() + ") " + e.Title)
}
