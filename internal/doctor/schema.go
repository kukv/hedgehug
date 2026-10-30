package doctor

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// queryKeys are the keys that queries read before they read properties.
var queryKeys = []string{"project", "is", "sort", "group", "blocks", "label", "assignee"}

// schemaCodes are the problems of a task that come from the schema.
var schemaCodes = []string{
	schema.CodeUnknownKind, schema.CodeUnknownStatus, schema.CodeMissingTitle,
	schema.CodeBadValue, schema.CodePlacement, schema.CodeRequired,
}

const brokenHint = "restore schema.yaml with `git restore schema.yaml`, or fix it by hand and commit"

// SchemaFile decodes schema.yaml and checks it. The schema is nil when data
// does not decode; the findings are then that alone.
func SchemaFile(data []byte) (*schema.Schema, []Finding) {
	sc, err := schema.Decode(data)
	if err != nil {
		return nil, []Finding{{Check: "schema", Where: schema.Path, Msg: err.Error(), Hint: brokenHint}}
	}
	var fs []Finding
	for _, p := range sc.Check() {
		fs = append(fs, Finding{Check: "schema", Where: schema.Path + ": " + p.Path, Msg: p.Msg})
	}
	return sc, fs
}

// Schema finds where the schema of s does not fit the store: the problems of
// schema.yaml, templates it names that are missing, properties that queries
// cannot select by, tasks that break the schema, and overrides of projects
// that are gone. The findings are sorted. The error is for the checks of
// doctor that read files; these checks read nothing that can fail.
func Schema(s *store.Store, ix *index.Index) ([]Finding, error) {
	sc := s.Schema()
	var fs []Finding
	for _, p := range sc.Check() {
		fs = append(fs, Finding{Check: "schema", Where: schema.Path + ": " + p.Path, Msg: p.Msg})
	}
	fs = append(fs, missingTemplates(s, sc)...)
	for _, p := range sc.Props {
		if slices.Contains(queryKeys, p.Name) {
			fs = append(fs, Finding{
				Check: "prop_name", Where: schema.Path + ": props." + p.Name,
				Msg:  "queries cannot select by a property with this name",
				Hint: "rename the property with `hedgehug schema migrate prop`",
			})
		}
	}
	for _, e := range ix.Tasks() {
		for _, p := range ix.Problems(e.ID) {
			if slices.Contains(schemaCodes, p.Code) {
				fs = append(fs, Finding{Check: "task", Where: ix.ShortID(e.ID) + " " + e.Title, Msg: p.Field + ": " + p.Msg})
			}
		}
	}
	for i, sco := range sc.Scoped {
		s, ok := strings.CutPrefix(sco.Where, "project:")
		if !ok {
			continue // Check has reported it
		}
		id, err := model.ParseID(s)
		if err != nil {
			continue
		}
		if e, found := ix.Task(id); !found || e.DeletedAt != nil {
			fs = append(fs, Finding{
				Check: "override", Where: fmt.Sprintf("%s: scoped[%d].where", schema.Path, i),
				Msg:  sco.Where + " is not a project of this store, or is deleted",
				Hint: "restore the project, or remove the override",
			})
		}
	}
	Sort(fs)
	return fs, nil
}

// missingTemplates finds the templates that schema.yaml names and
// templates/ does not have.
func missingTemplates(s *store.Store, sc *schema.Schema) []Finding {
	var fs []Finding
	check := func(where, name string) {
		var msg string
		if err := model.ValidTemplateName(name); err != nil {
			msg = fmt.Sprintf("template name %q is not valid: %v", name, err)
		} else if _, err := s.Template(name); errors.Is(err, store.ErrNotFound) {
			msg = fmt.Sprintf("template %q does not exist", name)
		} else if err != nil {
			// The file is there but does not decode; doctor reports that.
			return
		}
		if msg != "" {
			fs = append(fs, Finding{
				Check: "template", Where: schema.Path + ": " + where, Msg: msg,
				Hint: "add templates/" + name + ".md or change the name",
			})
		}
	}
	for i, k := range sc.Kinds {
		if k.Template != "" {
			check(fmt.Sprintf("kinds[%d].template", i), k.Template)
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
