package maintenance

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// A label of a template (an item or a child at any depth) that is not in the
// list of the schema is a finding of its own, matched without regard to case.
func TestSchemaTemplateLabelsNotInTheList(t *testing.T) {
	t.Parallel()
	_, dir := newStore(t)
	s := rewriteSchema(t, dir, func(sc *schema.Schema) { sc.Labels = []string{"bug"} })
	tpl := &model.Template{Name: "bugfix", Item: model.TemplateItem{
		Kind: "task", Title: "Fix", Labels: []string{"Bug", "wip"},
		Children: []model.TemplateItem{{Title: "a"}, {Title: "b", Children: []model.TemplateItem{{Title: "c", Labels: []string{"docs"}}}}},
	}}
	if _, err := s.Update(store.Op{Name: "template", Author: testAuthor, SkipTransitions: true}, func(tx *store.Tx) error { return tx.PutTemplate(tpl) }); err != nil {
		t.Fatal(err)
	}
	fs, err := usecase.CheckSchema(s, mustOpen(t, s))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range fs {
		if f.Code != "template.bad_label" {
			t.Errorf("unexpected finding %+v", f)
			continue
		}
		got = append(got, f.Where+" | "+strings.Join(f.Args, ","))
	}
	want := []string{
		"templates/bugfix.md: children[1].children[0].labels | bugfix,children[1].children[0].labels,docs",
		"templates/bugfix.md: labels | bugfix,labels,wip",
	}
	if !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
}
