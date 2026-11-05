package automations_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/automations"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func code(err error) string {
	if e := usecase.AsError(err); e != nil {
		return e.Code
	}
	return ""
}

func ids(vs []automations.RuleView) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.ID)
	}
	return out
}

func yamlOf(t *testing.T, ts *usecasetest.Session) string {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.Dir(), model.AutomationsPath)) //nolint:gosec // a file of the test store
	if err != nil {
		return ""
	}
	return string(b)
}

func TestAddWritesOneCommitOfOpAutomation(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	before := ts.Head()
	if err := automations.Add(ts.Session, automations.AddInput{ID: "opened", On: "pr.opened", Status: "in_progress"}); err != nil {
		t.Fatal(err)
	}
	if ts.Head() == before || usecasetest.LastOp(t, ts) != "automation" {
		t.Errorf("head %s -> %s, op %q", before, ts.Head(), usecasetest.LastOp(t, ts))
	}
	rs, err := automations.List(ts.Session, "")
	if err != nil || len(rs) != 1 {
		t.Fatalf("List = %+v, %v", rs, err)
	}
	if r := rs[0]; r.ID != "opened" || r.On != "pr.opened" || r.Status != "in_progress" || !r.Enabled || r.Project != nil || r.DisabledIn == nil {
		t.Errorf("rule = %+v", r)
	}
}

func TestAddRefusesWhatCheckFindsAndWritesNothing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if err := automations.Add(ts.Session, automations.AddInput{ID: "a", On: "pr.opened", Status: "in_progress"}); err != nil {
		t.Fatal(err)
	}
	head := ts.Head()
	for name, in := range map[string]automations.AddInput{
		"unknown status":   {ID: "b", On: "pr.opened", Status: "nope"},
		"unknown category": {ID: "b", On: "pr.opened", Category: "nope"},
		"unknown event":    {ID: "b", On: "pr.nope", Status: "done"},
		"unknown when":     {ID: "b", On: "pr.merged", When: "nope", Status: "done"},
		"both":             {ID: "b", On: "pr.opened", Status: "done", Category: "done"},
		"neither":          {ID: "b", On: "pr.opened"},
		"empty id":         {On: "pr.opened", Status: "done"},
		"space in id":      {ID: "a b", On: "pr.opened", Status: "done"},
	} {
		if err := automations.Add(ts.Session, in); code(err) != "automation.invalid" || usecase.KindOf(err) != usecase.KindInvalid {
			t.Errorf("%s: err = %v (%s)", name, err, code(err))
		}
	}
	err := automations.Add(ts.Session, automations.AddInput{ID: "a", On: "pr.opened", Status: "done"})
	if code(err) != "automation.exists" || usecase.KindOf(err) != usecase.KindConflict {
		t.Errorf("duplicate: err = %v (%s)", err, code(err))
	}
	if ts.Head() != head {
		t.Errorf("a refusal committed")
	}
}

func TestAddInAProject(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("Web")
	other := ts.AddTask("Login", nil)

	if err := automations.Add(ts.Session, automations.AddInput{ID: "in-web", On: "pr.opened", Status: "in_progress", Project: p.String()}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(yamlOf(t, ts), "project:"+p.String()) {
		t.Errorf("automations.yaml = %s", yamlOf(t, ts))
	}
	rs, _ := automations.List(ts.Session, "")
	if len(rs) != 1 || rs[0].Project == nil || rs[0].Project.ULID != p.String() || rs[0].Project.Title != "Web" {
		t.Errorf("List = %+v", rs)
	}
	err := automations.Add(ts.Session, automations.AddInput{ID: "x", On: "pr.opened", Status: "in_progress", Project: other.ULID.String()})
	if code(err) != "automation.not_project" || usecase.KindOf(err) != usecase.KindInvalid {
		t.Errorf("a task that is not a project: %v (%s)", err, code(err))
	}
	err = automations.Add(ts.Session, automations.AddInput{ID: "x", On: "pr.opened", Status: "in_progress", Project: "NOPE-9"})
	if usecase.KindOf(err) != usecase.KindNotFound {
		t.Errorf("unknown project: %v", err)
	}
}

func TestRemoveAndNotFound(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("Web")
	for _, id := range []string{"a", "b"} {
		if err := automations.Add(ts.Session, automations.AddInput{ID: id, On: "pr.opened", Status: "in_progress"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := automations.SetEnabled(ts.Session, "a", p.String(), false); err != nil {
		t.Fatal(err)
	}
	if err := automations.Remove(ts.Session, "a"); err != nil {
		t.Fatal(err)
	}
	if got := yamlOf(t, ts); strings.Contains(got, "id: a") || strings.Contains(got, "scoped: [") && strings.Contains(got, "disable") {
		t.Errorf("automations.yaml = %s", got)
	}
	rs, _ := automations.List(ts.Session, "")
	if !slices.Equal(ids(rs), []string{"b"}) {
		t.Errorf("List = %v", ids(rs))
	}
	for _, err := range []error{
		automations.Remove(ts.Session, "a"),
		automations.SetEnabled(ts.Session, "a", "", false),
		automations.SetEnabled(ts.Session, "a", p.String(), true),
	} {
		if code(err) != "automation.not_found" || usecase.KindOf(err) != usecase.KindNotFound {
			t.Errorf("err = %v (%s)", err, code(err))
		}
	}
}

func TestAStoreWithoutRulesListsNone(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	rs, err := automations.List(ts.Session, "")
	if err != nil {
		t.Fatal(err)
	}
	if rs == nil || len(rs) != 0 {
		t.Fatalf("rules = %#v, want an empty list", rs)
	}
}

func TestEnableDisableAndTheListInAProject(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	web := ts.AddProject("Web")
	api := ts.AddProject("API")
	for _, in := range []automations.AddInput{
		{ID: "all", On: "pr.opened", Status: "in_progress"},
		{ID: "web-only", On: "pr.merged", Category: "done", Project: web.String()},
		{ID: "api-only", On: "pr.merged", Category: "done", Project: api.String()},
	} {
		if err := automations.Add(ts.Session, in); err != nil {
			t.Fatal(err)
		}
	}

	// in Web: the rule of the store, then that of Web
	rs, err := automations.List(ts.Session, web.String())
	if err != nil || !slices.Equal(ids(rs), []string{"all", "web-only"}) {
		t.Fatalf("List in Web = %v, %v", ids(rs), err)
	}

	// disable in Web: the rule stays on in the file, off in Web
	if err := automations.SetEnabled(ts.Session, "all", web.String(), false); err != nil {
		t.Fatal(err)
	}
	all, _ := automations.List(ts.Session, "")
	if len(all) != 3 || !all[0].Enabled || len(all[0].DisabledIn) != 1 || all[0].DisabledIn[0].ULID != web.String() {
		t.Errorf("List = %+v", all)
	}
	rs, _ = automations.List(ts.Session, web.String())
	if rs[0].ID != "all" || rs[0].Enabled || !rs[1].Enabled {
		t.Errorf("in Web = %+v", rs)
	}
	rs, _ = automations.List(ts.Session, api.String())
	if !slices.Equal(ids(rs), []string{"all", "api-only"}) || !rs[0].Enabled {
		t.Errorf("in API = %+v", rs)
	}

	// enabled again: the scoped item goes
	if err := automations.SetEnabled(ts.Session, "all", web.String(), true); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(yamlOf(t, ts), "disable") {
		t.Errorf("automations.yaml = %s", yamlOf(t, ts))
	}

	// the rule's own switch
	if err := automations.SetEnabled(ts.Session, "all", "", false); err != nil {
		t.Fatal(err)
	}
	rs, _ = automations.List(ts.Session, "")
	if rs[0].Enabled {
		t.Errorf("all is still on: %+v", rs[0])
	}
	// the same again writes nothing
	head := ts.Head()
	if err := automations.SetEnabled(ts.Session, "all", "", false); err != nil || ts.Head() != head {
		t.Errorf("same change: %v, committed %v", err, ts.Head() != head)
	}
	// a task that is not a project
	plain := ts.AddTask("Plain", nil)
	if err := automations.SetEnabled(ts.Session, "all", plain.ULID.String(), false); code(err) != "automation.not_project" {
		t.Errorf("err = %v (%s)", err, code(err))
	}
}

func TestFields(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	v, err := automations.Fields(ts.Session)
	if err != nil || v.DefaultSync != "link" || len(v.Push) != 5 || len(v.Mirror) != 3 || v.Props == nil || len(v.Props) != 0 {
		t.Fatalf("Fields = %+v, %v", v, err)
	}
	head := ts.Head()
	if err := automations.SetFields(ts.Session, automations.FieldsInput{}); err != nil || ts.Head() != head {
		t.Errorf("no change: %v, committed %v", err, ts.Head() != head)
	}

	mode, push := "mirror", []string{"title", "status"}
	if err := automations.SetFields(ts.Session, automations.FieldsInput{DefaultSync: &mode, Push: &push, Props: map[string]string{"milestone": "target"}}); err != nil {
		t.Fatal(err)
	}
	if usecasetest.LastOp(t, ts) != "automation" {
		t.Errorf("op = %q", usecasetest.LastOp(t, ts))
	}
	v, _ = automations.Fields(ts.Session)
	if v.DefaultSync != "mirror" || !slices.Equal(v.Push, push) || v.Props["milestone"] != "target" || len(v.Mirror) != 3 {
		t.Errorf("Fields = %+v", v)
	}
	// an empty value takes a prop off
	if err := automations.SetFields(ts.Session, automations.FieldsInput{Props: map[string]string{"milestone": ""}}); err != nil {
		t.Fatal(err)
	}
	if v, _ = automations.Fields(ts.Session); len(v.Props) != 0 {
		t.Errorf("Props = %v", v.Props)
	}

	head = ts.Head()
	bad := "nope"
	for name, ch := range map[string]automations.FieldsInput{
		"mode":  {DefaultSync: &bad},
		"push":  {Push: &[]string{"nope"}},
		"prop":  {Props: map[string]string{"nope": "x"}},
		"empty": {Mirror: &[]string{"title", "nope"}},
	} {
		if err := automations.SetFields(ts.Session, ch); code(err) != "automation.invalid" {
			t.Errorf("%s: err = %v (%s)", name, err, code(err))
		}
	}
	if ts.Head() != head {
		t.Errorf("a refusal committed")
	}
}

func TestAFileThatDoesNotDecodeIsInvalid(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.CommitRawFile(model.AutomationsPath, "rules: [")
	if _, err := automations.List(ts.Session, ""); code(err) != "automation.invalid" {
		t.Errorf("List: %v (%s)", err, code(err))
	}
	if _, err := automations.Fields(ts.Session); code(err) != "automation.invalid" {
		t.Errorf("Fields: %v (%s)", err, code(err))
	}
	err := automations.Add(ts.Session, automations.AddInput{ID: "a", On: "pr.opened", Status: "done"})
	if code(err) != "automation.invalid" {
		t.Errorf("Add: %v (%s)", err, code(err))
	}
}

func TestAProblemThatWasThereBeforeDoesNotStopAnEdit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.CommitRawFile(model.AutomationsPath, "rules:\n  - id: old\n    on: pr.opened\n    set: {status: gone}\n")
	if err := automations.Add(ts.Session, automations.AddInput{ID: "new", On: "pr.opened", Status: "done"}); err != nil {
		t.Fatalf("Add = %v", err)
	}
}

func TestRemoveInFrontOfABrokenRuleIsNotRefused(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.CommitRawFile(model.AutomationsPath, "rules:\n  - {id: a, on: pr.opened, set: {status: done}}\n  - {id: b, on: pr.opened, set: {status: gone}}\n")
	if err := automations.Remove(ts.Session, "a"); err != nil {
		t.Fatalf("Remove = %v", err)
	}
	// a new problem of the same text is still refused
	err := automations.Add(ts.Session, automations.AddInput{ID: "c", On: "pr.opened", Status: "gone"})
	if code(err) != "automation.invalid" {
		t.Errorf("Add of a second broken rule: %v (%s)", err, code(err))
	}
}

func TestEnableInFrontOfABrokenScopedIsNotRefused(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("Web")
	q := ts.AddProject("API")
	ts.CommitRawFile(model.AutomationsPath, "rules:\n  - {id: a, on: pr.opened, set: {status: done}}\n"+
		"scoped:\n  - {where: 'project:"+p.String()+"', disable: [a]}\n  - {where: 'project:"+q.String()+"', disable: [nope]}\n")
	if err := automations.SetEnabled(ts.Session, "a", p.String(), true); err != nil {
		t.Fatalf("SetEnabled = %v", err)
	}
	if strings.Contains(yamlOf(t, ts), p.String()) {
		t.Errorf("automations.yaml = %s", yamlOf(t, ts))
	}
}

func TestProjectTakesAStoreQualifiedID(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddProject("Web")
	if err := automations.Add(ts.Session, automations.AddInput{ID: "a", On: "pr.opened", Status: "done", Project: "me:ME-1"}); err != nil {
		t.Fatalf("a store-qualified id: %v", err)
	}
	if rs, err := automations.List(ts.Session, "me:ME-1"); err != nil || len(rs) != 1 {
		t.Errorf("List = %+v, %v", rs, err)
	}
}

func TestProjectInAnotherStoreIsRefusedWithItsName(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	a := ts.ForStore("org-a")
	for _, arg := range []string{"WEB-1", "org-b:WEB-1"} {
		err := automations.Add(a, automations.AddInput{ID: "x", On: "pr.opened", Status: "done", Project: arg})
		usecasetest.WantCode(t, arg, err, usecase.KindInvalid, "ref.cross_store")
		if err != nil && !strings.Contains(err.Error(), "org-b") {
			t.Errorf("%s: %v does not name the store", arg, err)
		}
	}
}
