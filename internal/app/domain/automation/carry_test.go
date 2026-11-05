package automation

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

func decodeFile(t *testing.T, src string) *File {
	t.Helper()
	f, err := Decode([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func ruleIDs(f *File) []string {
	out := make([]string, len(f.Rules))
	for i, r := range f.Rules {
		out[i] = r.ID
	}
	return out
}

func TestCarry(t *testing.T) {
	t.Parallel()
	p, other := model.NewULID(), model.NewULID()
	where := WherePrefix + p.String()
	src := decodeFile(t, `rules:
- {id: global, on: pr.opened, set: {status: todo}}
- {id: kept, where: '`+where+`', on: pr.merged, enabled: false, set: {status: done}}
- {id: by-category, where: '`+where+`', on: pr.merged, set: {category: done}}
- {id: no-status, where: '`+where+`', on: pr.opened, set: {status: review}}
- {id: taken, where: '`+where+`', on: pr.opened, set: {status: done}}
- {id: elsewhere, where: 'project:`+other.String()+`', on: pr.opened, set: {status: done}}
scoped:
- {where: '`+where+`', disable: [global, kept, no-status, base, gone]}
- {where: 'project:`+other.String()+`', disable: [global]}
`)
	dst := decodeFile(t, `# the rules of this store
rules:
- {id: base, on: pr.opened, set: {status: todo}}
- {id: taken, on: pr.opened, set: {status: todo}}
issue_fields: {default_sync: mirror}
`)
	next, dropped := dst.Carry(src, []model.ULID{p}, schema.Default())

	if want := []string{"base", "taken", "kept", "by-category"}; !slices.Equal(ruleIDs(next), want) {
		t.Errorf("rules = %v, want %v", ruleIDs(next), want)
	}
	if r, _ := next.Rule("kept"); r.Enabled || r.Where != where || r.On != PRMerged || r.Set.Status != "done" {
		t.Errorf("kept = %+v", r)
	}
	if r, _ := next.Rule("taken"); r.Where != "" {
		t.Errorf("the rule of the destination was replaced: %+v", r)
	}
	if len(next.Scoped) != 1 || next.Scoped[0].Where != where || !slices.Equal(next.Scoped[0].Disable, []string{"kept", "base"}) {
		t.Errorf("scoped = %+v", next.Scoped)
	}
	want := []string{"rule no-status", "rule taken", "disable global", "disable no-status", "disable gone"}
	if !slices.Equal(dropped, want) {
		t.Errorf("dropped = %v, want %v", dropped, want)
	}
	if next.IssueFields.DefaultSync != model.SyncMirror {
		t.Errorf("issue_fields = %+v", next.IssueFields)
	}
	if len(dst.Rules) != 2 || len(dst.Scoped) != 0 {
		t.Errorf("dst changed: %+v", dst)
	}
	b, err := next.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "# the rules of this store") {
		t.Errorf("the comment is gone:\n%s", b)
	}
	if ps := next.Check(schema.Default(), func(u model.ULID) bool { return u == p }); len(ps) != 0 {
		t.Errorf("Check = %v", ps)
	}
}

// A rule is held to what the project may use in the destination.
func TestCarryRespectsTheScopeOfTheProject(t *testing.T) {
	t.Parallel()
	p := model.NewULID()
	where := WherePrefix + p.String()
	sc := schema.Default()
	sc.Scoped = []schema.Scoped{{Where: where, Statuses: []string{"todo", "done"}}}
	src := decodeFile(t, "rules:\n- {id: hidden, where: '"+where+"', on: pr.opened, set: {status: in_progress}}\n")
	next, dropped := Default().Carry(src, []model.ULID{p}, sc)
	if len(next.Rules) != 0 || !slices.Equal(dropped, []string{"rule hidden"}) {
		t.Errorf("rules %v, dropped %v", next.Rules, dropped)
	}
}

// A category without a default status is one the rule could never set.
func TestCarryDropsACategoryWithoutADefault(t *testing.T) {
	t.Parallel()
	p := model.NewULID()
	sc := schema.Default()
	for i := range sc.Statuses {
		if sc.Statuses[i].Category == schema.CategoryDone {
			sc.Statuses[i].Default = false
		}
	}
	src := decodeFile(t, "rules:\n- {id: finish, where: '"+WherePrefix+p.String()+"', on: pr.merged, set: {category: done}}\n")
	next, dropped := Default().Carry(src, []model.ULID{p}, sc)
	if len(next.Rules) != 0 || !slices.Equal(dropped, []string{"rule finish"}) {
		t.Errorf("rules %v, dropped %v", next.Rules, dropped)
	}
}

// A where is read as Check reads it, whatever its case; a disable meets the
// item that the destination has for the project; a repeat is told once.
func TestCarryWhereAndDisable(t *testing.T) {
	t.Parallel()
	p, q := model.NewULID(), model.NewULID()
	lower := WherePrefix + strings.ToLower(p.String())
	src := decodeFile(t, "rules:\n- {id: a, where: '"+lower+"', on: pr.opened, set: {status: todo}}\n"+
		"scoped:\n- {where: '"+lower+"', disable: [a, b, nope]}\n- {where: '"+WherePrefix+q.String()+"', disable: [nope]}\n")
	dst := decodeFile(t, "rules:\n- {id: b, on: pr.opened, set: {status: todo}}\n"+
		"scoped:\n- {where: '"+WherePrefix+p.String()+"', disable: [b]}\n")
	next, dropped := dst.Carry(src, []model.ULID{p, q}, schema.Default())
	if len(next.Rules) != 2 || len(next.Scoped) != 1 || !slices.Equal(next.Scoped[0].Disable, []string{"b", "a"}) {
		t.Errorf("next = %+v", next)
	}
	if !slices.Equal(dropped, []string{"disable nope"}) {
		t.Errorf("dropped = %v", dropped)
	}
}

func TestWithout(t *testing.T) {
	t.Parallel()
	p, q := model.NewULID(), model.NewULID()
	src := "# keep me\nrules:\n" +
		"- {id: global, on: pr.opened, set: {status: todo}}\n" +
		"- {id: gone, where: '" + WherePrefix + strings.ToLower(p.String()) + "', on: pr.opened, set: {status: todo}}\n" +
		"- {id: stays, where: '" + WherePrefix + q.String() + "', on: pr.opened, set: {status: todo}}\n" +
		"scoped:\n- {where: '" + WherePrefix + p.String() + "', disable: [global]}\n" +
		"- {where: '" + WherePrefix + q.String() + "', disable: [global, gone]}\n"
	f := decodeFile(t, src)
	next := f.Without([]model.ULID{p})
	if want := []string{"global", "stays"}; !slices.Equal(ruleIDs(next), want) {
		t.Errorf("rules = %v", ruleIDs(next))
	}
	// no disable is left that names a rule that went
	if len(next.Scoped) != 1 || next.Scoped[0].Where != WherePrefix+q.String() || !slices.Equal(next.Scoped[0].Disable, []string{"global"}) {
		t.Errorf("scoped = %+v", next.Scoped)
	}
	if len(f.Rules) != 3 || len(f.Scoped) != 2 || len(f.Scoped[1].Disable) != 2 {
		t.Errorf("f changed: %+v", f)
	}
	b, err := next.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "# keep me") {
		t.Errorf("the comment is gone:\n%s", b)
	}
}

func TestProjects(t *testing.T) {
	t.Parallel()
	p, q := model.NewULID(), model.NewULID()
	f := decodeFile(t, "rules:\n- {id: a, on: pr.opened, set: {status: todo}}\n"+
		"- {id: b, where: '"+WherePrefix+q.String()+"', on: pr.opened, set: {status: todo}}\n"+
		"- {id: c, where: nonsense, on: pr.opened, set: {status: todo}}\n"+
		"scoped:\n- {where: '"+WherePrefix+p.String()+"', disable: [a]}\n"+
		"- {where: '"+WherePrefix+q.String()+"', disable: [a]}\n")
	if got, want := f.Projects(), []model.ULID{q, p}; !slices.Equal(got, want) {
		t.Errorf("Projects = %v, want %v", got, want)
	}
}
