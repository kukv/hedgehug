package stores_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// automationsOf reads automations.yaml of the store name as it is now.
func automationsOf(t *testing.T, f *usecasetest.MoveFx, name string) *automation.File {
	t.Helper()
	s, err := f.TS.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Automations()
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func ruleIDs(a *automation.File) []string {
	out := make([]string, len(a.Rules))
	for i, r := range a.Rules {
		out[i] = r.ID
	}
	return out
}

// destinationHasAutomations reports whether org-b has automations.yaml.
func destinationHasAutomations(t *testing.T, f *usecasetest.MoveFx) bool {
	t.Helper()
	s, err := f.TS.ForStore("org-b").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(filepath.Join(s.Dir(), model.AutomationsPath))
	return err == nil
}

// automationsBytes is automations.yaml of the store name as it stands.
func automationsBytes(t *testing.T, f *usecasetest.MoveFx, name string) string {
	t.Helper()
	s, err := f.TS.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.Dir(), model.AutomationsPath))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// projectRules are a rule and a disable of the project moved, what fits
// org-b and what does not, and a rule and a disable that stay.
func projectRules(f *usecasetest.MoveFx) string {
	where := "'project:" + f.P.String() + "'"
	return `rules:
- {id: global, on: pr.opened, set: {status: in_progress}}
- {id: mine, where: ` + where + `, on: pr.merged, set: {status: done}}
- {id: mine-off, where: ` + where + `, on: pr.opened, enabled: false, set: {category: done}}
- {id: review, where: ` + where + `, on: pr.review_requested, set: {status: review}}
- {id: taken, where: ` + where + `, on: pr.opened, set: {status: done}}
- {id: other, where: 'project:` + f.First.String() + `', on: pr.opened, set: {status: done}}
scoped:
- {where: ` + where + `, disable: [mine-off, global]}
`
}

func TestMoveStoreCarriesAutomations(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetSchema(t, func(sc *schema.Schema) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
	})
	f.SetAutomations(t, "org-a", projectRules(f))
	f.SetAutomations(t, "org-b", "# ours\nrules:\n- {id: taken, on: pr.opened, set: {status: todo}}\n")

	in := f.In()
	in.Scoped = "carry"
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Carried {
		t.Error("not carried")
	}
	wantDropped := []string{"rule review", "rule taken", "disable global"}
	if res.Plan.Scoped == nil || !slices.Equal(res.Plan.Scoped.Dropped, wantDropped) {
		t.Errorf("plan = %+v", res.Plan.Scoped)
	}

	dst := automationsOf(t, f, "org-b")
	if want := []string{"taken", "mine", "mine-off"}; !slices.Equal(ruleIDs(dst), want) {
		t.Errorf("destination rules = %v, want %v", ruleIDs(dst), want)
	}
	if r, _ := dst.Rule("mine-off"); r.Enabled {
		t.Error("the rule that was off came on")
	}
	if len(dst.Scoped) != 1 || !slices.Equal(dst.Scoped[0].Disable, []string{"mine-off"}) {
		t.Errorf("destination scoped = %+v", dst.Scoped)
	}
	src := automationsOf(t, f, "org-a")
	if want := []string{"global", "other"}; !slices.Equal(ruleIDs(src), want) {
		t.Errorf("source rules = %v, want %v", ruleIDs(src), want)
	}
	if len(src.Scoped) != 0 {
		t.Errorf("source scoped = %+v", src.Scoped)
	}
	if op, _ := f.Log(t, "org-b")[0].Trailer("Hedgehug-Op"); op != "move-in" {
		t.Errorf("last op of the destination = %s", op)
	}
	if b := automationsBytes(t, f, "org-b"); !strings.Contains(b, "# ours") {
		t.Errorf("the comment of the destination is gone:\n%s", b)
	}
}

func TestMoveStoreDropsAutomations(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetAutomations(t, "org-a", projectRules(f))
	in := f.In()
	in.Scoped = "drop"
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Carried || len(res.Plan.Scoped.Dropped) != 0 {
		t.Errorf("res = %+v, scoped %+v", res, res.Plan.Scoped)
	}
	if destinationHasAutomations(t, f) {
		t.Error("the destination got automations.yaml")
	}
	src := automationsOf(t, f, "org-a")
	if want := []string{"global", "other"}; !slices.Equal(ruleIDs(src), want) || len(src.Scoped) != 0 {
		t.Errorf("source = %v %+v", ruleIDs(src), src.Scoped)
	}
}

// A rule of a project that is not moved asks nothing, and no file is touched.
func TestMoveStoreLeavesOtherAutomations(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	src := "# as written\nrules:\n- {id: other, where: 'project:" + f.First.String() + "', on: pr.opened, set: {status: done}}\n"
	f.SetAutomations(t, "org-a", src)
	if _, err := stores.MoveStore(f.TS.Session, f.In()); err != nil {
		t.Fatal(err)
	}
	if destinationHasAutomations(t, f) {
		t.Error("the destination got automations.yaml")
	}
	s, _ := f.TS.ForStore("org-a").OpenStore()
	b, err := os.ReadFile(filepath.Join(s.Dir(), model.AutomationsPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != src {
		t.Errorf("the source file changed:\n%s", b)
	}
}

// A child of the project leaves the project behind: the rules stay where they
// are, nothing is asked and neither file changes.
func TestMoveStoreChildOfAProjectLeavesItsAutomations(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	src := "# theirs\n" + projectRules(f)
	dst := "# ours\nrules:\n- {id: taken, on: pr.opened, set: {status: todo}}\n"
	f.SetAutomations(t, "org-a", src)
	f.SetAutomations(t, "org-b", dst)
	in := f.In()
	in.Arg = "API-3"
	in.Parent = "WEB-1"
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Plan.Scoped != nil || res.Carried {
		t.Errorf("res = %+v, scoped %+v", res, res.Plan.Scoped)
	}
	if got := automationsBytes(t, f, "org-a"); got != src {
		t.Errorf("the source file changed:\n%s", got)
	}
	if got := automationsBytes(t, f, "org-b"); got != dst {
		t.Errorf("the destination file changed:\n%s", got)
	}
}

// automations.yaml alone asks the choice, and a project is named once when
// schema.yaml has it too.
func TestMoveStoreAutomationsNeedTheChoice(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetAutomations(t, "org-a", projectRules(f))
	_, err := stores.MoveStore(f.TS.Session, f.In())
	usecasetest.MustCode(t, err, usecase.KindUsage, "choice.required")
	if got := argsOf(err)["projects"]; !slices.Equal(got.([]string), []string{"API-2 P"}) {
		t.Errorf("projects = %v", got)
	}
	if got := argsOf(err)["subject"]; got != "automation rules" {
		t.Errorf("subject = %v", got)
	}

	v, err := stores.PrepareMovePlan(f.TS.Session, f.In())
	if err != nil {
		t.Fatal(err)
	}
	if v.Scoped == nil || !v.Scoped.Required || len(v.Scoped.Dropped) != 0 {
		t.Errorf("plan scoped = %+v", v.Scoped)
	}

	f.SetSchema(t, func(sc *schema.Schema) {
		sc.Scoped = []schema.Scoped{{Where: "project:" + f.P.String(), Statuses: []string{"backlog", "todo", "in_progress", "done", "canceled"}}}
	})
	_, err = stores.MoveStore(f.TS.Session, f.In())
	usecasetest.MustCode(t, err, usecase.KindUsage, "choice.required")
	if got := argsOf(err)["projects"]; !slices.Equal(got.([]string), []string{"API-2 P"}) {
		t.Errorf("projects = %v", got)
	}
	if got := argsOf(err)["subject"]; got != "scoped schema and automation rules" {
		t.Errorf("subject = %v", got)
	}
}

// A carried rule is held to the scoped entry that comes with it.
func TestMoveStoreCarriesRulesIntoTheCarriedScope(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetSchema(t, func(sc *schema.Schema) {
		sc.Scoped = []schema.Scoped{{Where: "project:" + f.P.String(), Statuses: []string{"backlog", "todo", "in_progress", "done", "canceled"}}}
	})
	f.SetAutomations(t, "org-a", "rules:\n- {id: hidden, where: 'project:"+f.P.String()+"', on: pr.opened, set: {status: waiting}}\n")
	in := f.In()
	in.Scoped = "carry"
	v, err := stores.PrepareMovePlan(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(v.Scoped.Dropped, "rule hidden") {
		t.Errorf("dropped = %v", v.Scoped.Dropped)
	}
}

// Everything left out is still a carried move, and writes nothing.
func TestMoveStoreCarriesNothingOfTheAutomations(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetSchema(t, func(sc *schema.Schema) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
	})
	f.SetAutomations(t, "org-a", "rules:\n- {id: review, where: 'project:"+f.P.String()+"', on: pr.opened, set: {status: review}}\n")
	in := f.In()
	in.Scoped = "carry"
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Carried || !slices.Equal(res.Plan.Scoped.Dropped, []string{"rule review"}) {
		t.Errorf("res = %+v, scoped %+v", res, res.Plan.Scoped)
	}
	if destinationHasAutomations(t, f) {
		t.Error("the destination got automations.yaml")
	}
	if a := automationsOf(t, f, "org-a"); len(a.Rules) != 0 {
		t.Errorf("source rules = %v", ruleIDs(a))
	}
}

// A project with a deleted task under it moves with the task, and its rules go
// too. (A project cannot be deleted under another one any more: only a project
// sits at the root, and a deleted root is refused.)
func TestMoveStoreCarriesTheRulesOfAProjectWithADeletedTask(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	deletedTaskWithARule(t, f)
	in := f.In()
	in.Scoped = "carry"
	if _, err := stores.MoveStore(f.TS.Session, in); err != nil {
		t.Fatal(err)
	}
	if got := ruleIDs(automationsOf(t, f, "org-b")); !slices.Equal(got, []string{"sub"}) {
		t.Errorf("destination rules = %v", got)
	}
}

// A move with a task that was deleted before it stops between the stores: the
// destination has the task deleted, as the source does, and the rerun ends it.
func TestMoveStoreRerunWithADeletedTask(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	deletedTaskWithARule(t, f)
	in := f.In()
	in.Scoped = "carry"
	stopInTheSource(t, f, in)
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed || !res.Carried {
		t.Errorf("res = %+v", res)
	}
	if got := ruleIDs(automationsOf(t, f, "org-a")); len(got) != 0 {
		t.Errorf("source rules = %v", got)
	}
}

// A task that is alive in the source and deleted in the destination is not
// what a move left there.
func TestMoveStoreRefusesATaskDeletedInTheDestination(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	stopInTheSource(t, f, f.In())
	if _, err := tasks.Remove(f.TS.ForStore("org-b"), f.G.String()); err != nil {
		t.Fatal(err)
	}
	_, err := stores.MoveStore(f.TS.Session, f.In())
	usecasetest.MustCode(t, err, usecase.KindConflict, "move.exists")
}

// deletedTaskWithARule gives org-a a task under API-2 and deletes it, and a rule
// for API-2.
func deletedTaskWithARule(t *testing.T, f *usecasetest.MoveFx) {
	t.Helper()
	a := f.TS.ForStore("org-a")
	r, err := tasks.Add(a, tasks.AddInput{Title: "Sub", Parent: "API-2", Kind: "task"})
	if err != nil {
		t.Fatal(err)
	}
	f.SetAutomations(t, "org-a", "rules:\n- {id: sub, where: 'project:"+f.P.String()+"', on: pr.opened, set: {status: done}}\n")
	if _, err := tasks.Remove(a, r.ULID.String()); err != nil {
		t.Fatal(err)
	}
}

// A rule with a defect the destination would report stops a carry, as a
// scoped entry does; dropping it goes through.
func TestMoveStoreAutomationsThatCannotBeCarried(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetAutomations(t, "org-a", "rules:\n- {id: bad, where: 'project:"+f.P.String()+"', on: pr.closed, set: {status: done}}\n")
	hb := f.Head(t, "org-b")
	in := f.In()
	in.Scoped = "carry"
	_, err := stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindInvalid, "move.invalid")
	if !strings.Contains(err.Error(), "--scoped drop") || !strings.Contains(err.Error(), "pr.closed") {
		t.Errorf("err = %v", err)
	}
	if f.Head(t, "org-b") != hb {
		t.Error("the destination was written")
	}
	in.Scoped = "drop"
	if _, err := stores.MoveStore(f.TS.Session, in); err != nil {
		t.Fatal(err)
	}
}

// A defect the destination has already is not the move's.
func TestMoveStoreCarriesPastADefectOfTheDestination(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetAutomations(t, "org-a", "rules:\n- {id: mine, where: 'project:"+f.P.String()+"', on: pr.opened, set: {status: done}}\n")
	f.SetAutomations(t, "org-b", "rules:\n- {id: old, on: pr.closed, set: {status: done}}\n")
	in := f.In()
	in.Scoped = "carry"
	if _, err := stores.MoveStore(f.TS.Session, in); err != nil {
		t.Fatal(err)
	}
	if got := ruleIDs(automationsOf(t, f, "org-b")); !slices.Equal(got, []string{"old", "mine"}) {
		t.Errorf("destination rules = %v", got)
	}
}

// A source that cannot be read might hold rules that the tasks need.
func TestMoveStoreStopsOnASourceAutomationsThatDoesNotDecode(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetAutomations(t, "org-a", "rules: [\n")
	ha, hb := f.Head(t, "org-a"), f.Head(t, "org-b")
	_, err := stores.MoveStore(f.TS.Session, f.In())
	usecasetest.MustCode(t, err, usecase.KindInvalid, "automation.invalid")
	if argsOf(err)["store"] != "org-a" || !strings.Contains(err.Error(), "org-a") {
		t.Errorf("err = %v, args %v", err, argsOf(err))
	}
	if f.Head(t, "org-a") != ha || f.Head(t, "org-b") != hb {
		t.Error("a store was written")
	}
}

// A destination that cannot be read stops what would write it, and nothing
// else: what is not carried does not need the file.
func TestMoveStoreDestinationAutomationsThatDoesNotDecode(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetAutomations(t, "org-a", projectRules(f))
	f.SetAutomations(t, "org-b", "rules: [\n")
	hb := f.Head(t, "org-b")
	in := f.In()
	in.Scoped = "carry"
	_, err := stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindInvalid, "automation.invalid")
	if argsOf(err)["store"] != "org-b" {
		t.Errorf("args = %v", argsOf(err))
	}
	if f.Head(t, "org-b") != hb {
		t.Error("the destination was written")
	}
	in.Scoped = "drop"
	if _, err := stores.MoveStore(f.TS.Session, in); err != nil {
		t.Fatal(err)
	}
}

// stopInTheSource runs a move that stops after the destination was written,
// and puts back what stopped it.
func stopInTheSource(t *testing.T, f *usecasetest.MoveFx, in stores.MoveStoreInput) {
	t.Helper()
	as, _ := f.TS.ForStore("org-a").OpenStore()
	gPath := filepath.Join(as.Dir(), filepath.FromSlash(model.TaskPath(f.G)))
	gBytes, err := os.ReadFile(gPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.TS.BeforeUpdate = func() {
		if calls++; calls == 2 { // the write into the source
			_ = os.Remove(gPath)
		}
	}
	_, err = stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindNotFound, "move.unfinished")
	f.TS.BeforeUpdate = nil
	if err := os.WriteFile(gPath, gBytes, 0o600); err != nil {
		t.Fatal(err)
	}
}

// A carry that stopped between the stores: the destination has the rules and
// the source still has them. The rerun writes the destination no more, and the
// source lets them go, with no --scoped asked of it.
func TestMoveStoreRerunDoesNotRepeatAutomations(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetAutomations(t, "org-a", "rules:\n- {id: global, on: pr.opened, set: {status: todo}}\n"+
		"- {id: mine, where: 'project:"+f.P.String()+"', on: pr.opened, set: {status: done}}\n")
	in := f.In()
	in.Scoped = "carry"
	stopInTheSource(t, f, in)
	if got := ruleIDs(automationsOf(t, f, "org-b")); !slices.Equal(got, []string{"mine"}) {
		t.Fatalf("destination rules after the first run = %v", got)
	}
	if got := ruleIDs(automationsOf(t, f, "org-a")); !slices.Equal(got, []string{"global", "mine"}) {
		t.Fatalf("source rules after the first run = %v", got)
	}
	hb := f.Head(t, "org-b")

	in.Scoped = ""
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed || !res.Carried {
		t.Errorf("res = %+v", res)
	}
	if f.Head(t, "org-b") != hb {
		t.Error("the destination was written again")
	}
	if got := ruleIDs(automationsOf(t, f, "org-b")); !slices.Equal(got, []string{"mine"}) {
		t.Errorf("destination rules = %v", got)
	}
	if got := ruleIDs(automationsOf(t, f, "org-a")); !slices.Equal(got, []string{"global"}) {
		t.Errorf("source rules = %v", got)
	}
}

// A disable alone is what the first run carried.
func TestMoveStoreRerunSeesACarriedDisable(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetAutomations(t, "org-a", "rules:\n- {id: base, on: pr.opened, set: {status: todo}}\n"+
		"scoped:\n- {where: 'project:"+f.P.String()+"', disable: [base]}\n")
	f.SetAutomations(t, "org-b", "rules:\n- {id: base, on: pr.opened, set: {status: todo}}\n")
	in := f.In()
	in.Scoped = "carry"
	stopInTheSource(t, f, in)
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Carried {
		t.Errorf("res = %+v", res)
	}
	if a := automationsOf(t, f, "org-a"); len(a.Scoped) != 0 {
		t.Errorf("source scoped = %+v", a.Scoped)
	}
}

// A first run that dropped: the rerun does not call it carried, and the rules
// still leave the source.
func TestMoveStoreRerunAfterADrop(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetAutomations(t, "org-a", projectRules(f))
	in := f.In()
	in.Scoped = "drop"
	stopInTheSource(t, f, in)
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed || res.Carried || len(res.Plan.Scoped.Dropped) != 0 {
		t.Errorf("res = %+v, scoped %+v", res, res.Plan.Scoped)
	}
	if a := automationsOf(t, f, "org-a"); len(a.Scoped) != 0 || !slices.Equal(ruleIDs(a), []string{"global", "other"}) {
		t.Errorf("source = %v %+v", ruleIDs(a), a.Scoped)
	}
}

// The rerun writes nothing into the destination, so a file there that cannot
// be read does not stop it: it is told of once, and what it would say is left
// unsaid.
func TestMoveStoreRerunPastADestinationThatDoesNotDecode(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetAutomations(t, "org-a", projectRules(f))
	in := f.In()
	in.Scoped = "carry"
	stopInTheSource(t, f, in)
	f.SetAutomations(t, "org-b", "rules: [\n")
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed || len(res.Plan.Scoped.Dropped) != 0 {
		t.Errorf("res = %+v, scoped %+v", res, res.Plan.Scoped)
	}
	if a := automationsOf(t, f, "org-a"); !slices.Equal(ruleIDs(a), []string{"global", "other"}) || len(a.Scoped) != 0 {
		t.Errorf("source = %v %+v", ruleIDs(a), a.Scoped)
	}
	n := 0
	for _, w := range f.TS.Warns {
		if strings.Contains(w, model.AutomationsPath) && strings.Contains(w, "org-b") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d warnings; want 1 (all: %q)", n, f.TS.Warns)
	}
}

// The rerun says what the first run left out: the source still has it and the
// destination does not.
func TestMoveStoreRerunTellsWhatWasLeftOut(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetSchema(t, func(sc *schema.Schema) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
	})
	f.SetAutomations(t, "org-a", projectRules(f))
	f.SetAutomations(t, "org-b", "rules:\n- {id: taken, on: pr.opened, set: {status: todo}}\n")
	in := f.In()
	in.Scoped = "carry"
	stopInTheSource(t, f, in)
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"rule review", "rule taken", "disable global"}
	if !res.Resumed || !slices.Equal(res.Plan.Scoped.Dropped, want) {
		t.Errorf("res = %+v, dropped %v, want %v", res, res.Plan.Scoped.Dropped, want)
	}
}

// A carry that left out everything has written nothing: the rerun of the same
// command finishes the source and still says what was left out.
func TestMoveStoreRerunOfACarryOfNothing(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetSchema(t, func(sc *schema.Schema) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
	})
	f.SetAutomations(t, "org-a", "rules:\n- {id: review, where: 'project:"+f.P.String()+"', on: pr.opened, set: {status: review}}\n")
	in := f.In()
	in.Scoped = "carry"
	stopInTheSource(t, f, in)
	hb := f.Head(t, "org-b")
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed || !slices.Equal(res.Plan.Scoped.Dropped, []string{"rule review"}) {
		t.Errorf("res = %+v, scoped %+v", res, res.Plan.Scoped)
	}
	if f.Head(t, "org-b") != hb || destinationHasAutomations(t, f) {
		t.Error("the destination was written")
	}
	if a := automationsOf(t, f, "org-a"); len(a.Rules) != 0 {
		t.Errorf("source rules = %v", ruleIDs(a))
	}
}

// What the rerun takes off the source is worked out from the file, so a file
// that cannot be read stops it too, and the tasks stay for one more rerun.
func TestMoveStoreRerunStopsOnASourceThatDoesNotDecode(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetAutomations(t, "org-a", projectRules(f))
	in := f.In()
	in.Scoped = "drop"
	stopInTheSource(t, f, in)
	f.SetAutomations(t, "org-a", "rules: [\n")
	_, err := stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindInvalid, "automation.invalid")
}

// The words for what the rules lose go with the plan the GUI shows.
func TestMovePlanShowsWhatTheAutomationsLose(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetSchema(t, func(sc *schema.Schema) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
	})
	f.SetAutomations(t, "org-a", projectRules(f))
	in := f.In()
	in.Scoped = "carry"
	v, err := stores.PrepareMovePlan(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if v.Scoped == nil || v.Scoped.Required || !slices.Equal(v.Scoped.Dropped, []string{"rule review", "disable global"}) {
		t.Errorf("scoped = %+v", v.Scoped)
	}
	in.ConfirmedView = &v
	if _, err := stores.MoveStore(f.TS.Session, in); err != nil {
		t.Fatalf("the same plan: %v", err)
	}
}
