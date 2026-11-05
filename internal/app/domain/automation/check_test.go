package automation

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

func TestCheck(t *testing.T) {
	t.Parallel()
	proj := model.NewULID()
	gone := model.NewULID()
	isProject := func(u model.ULID) bool { return u == proj }
	sc := schema.Default()
	sc.Scoped = []schema.Scoped{{Where: "project:" + proj.String(), Statuses: []string{"todo", "done"}}}
	where := "project:" + proj.String()
	ok := func(id string) string {
		return fmt.Sprintf("- {id: %s, on: pr.opened, set: {status: in_progress}}\n", id)
	}
	for _, c := range []struct {
		name, src string
		want      []string // "path: part of the message"
	}{
		{"clean", "rules:\n" + ok("a") + "scoped:\n- {where: '" + where + "', disable: [a]}\n", nil},
		{"no file", "", nil},
		{"empty id", "rules:\n- {on: pr.opened, set: {status: todo}}\n", []string{"rules[0].id: "}},
		{"duplicate id", "rules:\n" + ok("a") + ok("a"), []string{"rules[1].id: "}},
		{"unknown on", "rules:\n- {id: a, on: pr.closed, set: {status: todo}}\n", []string{"rules[0].on: "}},
		{"no on", "rules:\n- {id: a, set: {status: todo}}\n", []string{"rules[0].on: "}},
		{"unknown when", "rules:\n- {id: a, on: pr.merged, when: always, set: {status: todo}}\n", []string{"rules[0].when: "}},
		{"set with both", "rules:\n- {id: a, on: pr.merged, set: {status: todo, category: done}}\n", []string{"rules[0].set: "}},
		{"set with neither", "rules:\n- {id: a, on: pr.merged}\n", []string{"rules[0].set: "}},
		{"unknown status", "rules:\n- {id: a, on: pr.merged, set: {status: nope}}\n", []string{"rules[0].set.status: "}},
		{"unknown category", "rules:\n- {id: a, on: pr.merged, set: {category: nope}}\n", []string{"rules[0].set.category: "}},
		{"status the project hides", "rules:\n- {id: a, where: '" + where + "', on: pr.merged, set: {status: in_progress}}\n", []string{"rules[0].set.status: "}},
		{"status the project has", "rules:\n- {id: a, where: '" + where + "', on: pr.merged, set: {status: done}}\n", nil},
		{"where without the prefix", "rules:\n- {id: a, where: " + proj.String() + ", on: pr.merged, set: {status: todo}}\n", []string{"rules[0].where: "}},
		{"where with a bad ULID", "rules:\n- {id: a, where: 'project:x', on: pr.merged, set: {status: todo}}\n", []string{"rules[0].where: "}},
		{"where of a missing project", "rules:\n- {id: a, where: 'project:" + gone.String() + "', on: pr.merged, set: {status: todo}}\n", []string{"rules[0].where: "}},
		{"scoped duplicate", "rules:\n" + ok("a") + "scoped:\n- {where: '" + where + "', disable: [a]}\n- {where: '" + where + "', disable: [a]}\n", []string{"scoped[1].where: "}},
		{"scoped of a missing project", "rules:\n" + ok("a") + "scoped:\n- {where: 'project:" + gone.String() + "', disable: [a]}\n", []string{"scoped[0].where: "}},
		{"scoped without the prefix", "rules:\n" + ok("a") + "scoped:\n- {where: x, disable: [a]}\n", []string{"scoped[0].where: "}},
		{"disable of an unknown id", "rules:\n" + ok("a") + "scoped:\n- {where: '" + where + "', disable: [a, b]}\n", []string{"scoped[0].disable[1]: "}},
		{"default_sync", "issue_fields: {default_sync: off}\n", []string{"issue_fields.default_sync: "}},
		{"push", "issue_fields: {push: [title, nope]}\n", []string{"issue_fields.push[1]: "}},
		{"mirror", "issue_fields: {mirror: [nope]}\n", []string{"issue_fields.mirror[0]: "}},
		{"milestone is allowed", "issue_fields: {push: [milestone], props: {milestone: target}}\n", nil},
		{"props key", "issue_fields: {props: {owner: x}}\n", []string{"issue_fields.props.owner: "}},
	} {
		f := mustDecode(t, c.src)
		var got []string
		for _, p := range f.Check(sc, isProject) {
			got = append(got, p.Path+": "+p.Msg)
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: problems = %q, want %q", c.name, got, c.want)
			continue
		}
		for i, w := range c.want {
			if !strings.HasPrefix(got[i], w) {
				t.Errorf("%s: problem %d = %q, want prefix %q", c.name, i, got[i], w)
			}
		}
	}
}

func TestCheckDevRules(t *testing.T) {
	t.Parallel()
	if ps := DevRules().Check(schema.Dev(), func(model.ULID) bool { return false }); len(ps) != 0 {
		t.Errorf("problems = %v", ps)
	}
	// The default schema has no review status.
	ps := DevRules().Check(schema.Default(), func(model.ULID) bool { return false })
	if len(ps) != 1 || !slices.Contains([]string{"rules[1].set.status"}, ps[0].Path) {
		t.Errorf("problems = %v", ps)
	}
}
