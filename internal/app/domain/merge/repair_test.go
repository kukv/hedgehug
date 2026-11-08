package merge

import (
	"bytes"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// memFiles is a set of files in memory.
type memFiles map[string][]byte

func (f memFiles) Paths(prefix string) []string {
	var out []string
	for p := range f {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

func (f memFiles) Read(path string) []byte { return f[path] }

// taskFile builds a task file in workspace ws. parent may be empty; extra
// lines are "key: value".
func taskFile(ulid, id, ws, parent, updated string, extra ...string) []byte {
	s := "---\nulid: " + ulid + "\nid: " + id + "\nworkspace: " + ws + "\n"
	if !slices.ContainsFunc(extra, func(l string) bool { return strings.HasPrefix(l, "kind:") }) {
		s += "kind: task\n"
	}
	if !slices.ContainsFunc(extra, func(l string) bool { return strings.HasPrefix(l, "title:") }) {
		s += "title: t\n"
	}
	if parent != "" {
		s += "parent: " + parent + "\n"
	}
	for _, l := range extra {
		s += l + "\n"
	}
	return []byte(s + "updated: " + updated + "\n---\n")
}

func pathOf(ulid string) string { return model.TaskPath(mustULID(ulid)) }

// uncatA is the ULID of the 未分類 project a repair makes in workspace wsA.
var uncatA = model.DerivedULID(time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC), "uncategorized "+wsA).String()

const wsA, wsB = "01JC0A1B2C3D4E5F6G7H8J9K0A", "01JC0A1B2C3D4E5F6G7H8J9K0B"

func wsFile(ulid, name, prefix string) []byte {
	return []byte("---\nulid: " + ulid + "\nname: " + name + "\nid_prefix: " + prefix + "\nprefix_set: 2026-10-30T00:00:00Z\ncreated: 2026-10-30T00:00:00Z\nupdated: 2026-10-30T00:00:00Z\n---\n")
}

func repair(t *testing.T, in RepairInput) (memFiles, []Repaired) {
	t.Helper()
	if in.Author == nil {
		in.Author = func(string) string { return "github.com/x" }
	}
	if in.OnOrigin == nil {
		in.OnOrigin = func(model.ULID, string) bool { return false }
	}
	if in.Changed == nil {
		in.Changed = func(string) bool { return true }
	}
	out, rs, _, err := Repair(in)
	if err != nil {
		t.Fatal(err)
	}
	after := maps.Clone(in.Files.(memFiles))
	for p, b := range out {
		if b == nil {
			delete(after, p)
		} else {
			after[p] = b
		}
	}
	// Repair must be deterministic.
	out2, rs2, _, _ := Repair(in)
	if !maps.EqualFunc(out, out2, bytes.Equal) || !slices.Equal(rs, rs2) {
		t.Fatal("not deterministic")
	}
	return after, rs
}

func TestRepairIDCollisionKeepsOrigin(t *testing.T) {
	x, y := "01JD0000000000000000000001", "01JD0000000000000000000002"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(x):                          taskFile(x, "WEB-43", wsA, "", "2026-10-31T10:00:00Z"),
		pathOf(y):                          taskFile(y, "WEB-43", wsA, "", "2026-10-31T10:00:00Z", "former_ids: [WEB-44]"),
		model.NumbersPath:                  []byte("WEB: 50\n"),
	}
	after, rs := repair(t, RepairInput{Files: f, OnOrigin: func(u model.ULID, id string) bool { return u.String() == y && id == "WEB-43" }})
	gx, gy := decoded(t, after[pathOf(x)]), decoded(t, after[pathOf(y)])
	if gy.ID != "WEB-43" || gx.ID != "WEB-51" || !slices.Contains(gx.FormerIDs, "WEB-43") {
		t.Errorf("x %s %v, y %s", gx.ID, gx.FormerIDs, gy.ID)
	}
	if len(rs) != 1 || rs[0].Kind != "id_collision" || rs[0].ULID.String() != x || rs[0].Value != `{"old":"WEB-43","new":"WEB-51"}` {
		t.Errorf("repaired = %+v", rs)
	}
}

func TestRepairIDCollisionNeitherOnOriginKeepsSmallerULID(t *testing.T) {
	x, y, z := "01JD0000000000000000000001", "01JD0000000000000000000002", "01JD0000000000000000000003"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(x):                          taskFile(x, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		pathOf(y):                          taskFile(y, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		pathOf(z):                          taskFile(z, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
	}
	after, _ := repair(t, RepairInput{Files: f})
	ids := []string{decoded(t, after[pathOf(x)]).ID, decoded(t, after[pathOf(y)]).ID, decoded(t, after[pathOf(z)]).ID}
	if !slices.Equal(ids, []string{"WEB-1", "WEB-2", "WEB-3"}) {
		t.Errorf("ids = %v", ids) // the others numbered in ULID order
	}
}

func TestRepairCountsBrokenFiles(t *testing.T) {
	x, y, bad := "01JD0000000000000000000001", "01JD0000000000000000000002", "01JD0000000000000000000009"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(x):                          taskFile(x, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		pathOf(y):                          taskFile(y, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		pathOf(bad):                        []byte("---\nid: WEB-77\n: [\n"),
	}
	after, _ := repair(t, RepairInput{Files: f})
	if got := decoded(t, after[pathOf(y)]).ID; got != "WEB-78" {
		t.Errorf("renumbered to %s, want WEB-78", got)
	}
}

// A cycle has one edge the kinds do not allow, a project under a task; that
// one is cut, whoever was updated last. The project goes back to its parent
// in the base, here the root.
func TestRepairParentCycle(t *testing.T) {
	a, b := "01JD0000000000000000000001", "01JD0000000000000000000002"
	base := memFiles{
		pathOf(a): taskFile(a, "WEB-1", wsA, "", "2026-10-31T10:00:00Z", "kind: project"),
		pathOf(b): taskFile(b, "WEB-2", wsA, a, "2026-10-31T10:00:00Z"),
	}
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(a):                          taskFile(a, "WEB-1", wsA, b, "2026-10-31T11:00:00Z", "kind: project"),
		pathOf(b):                          taskFile(b, "WEB-2", wsA, a, "2026-10-31T12:00:00Z"), // the later move
	}
	for name, in := range map[string]RepairInput{"base": {Files: f, Base: base}, "no base": {Files: f}} {
		after, rs := repair(t, in)
		if p := decoded(t, after[pathOf(a)]).Parent; p != nil {
			t.Errorf("%s: a.parent = %v, want the root", name, p)
		}
		if p := decoded(t, after[pathOf(b)]).Parent; p == nil || p.String() != a {
			t.Errorf("%s: b.parent changed: %v", name, p)
		}
		if len(rs) != 1 || rs[0].Kind != "parent_cycle" || rs[0].Value != `{"from":"`+b+`","to":null}` {
			t.Errorf("%s: repaired = %+v", name, rs)
		}
	}
}

// The task goes back to its parent in the base when that parent can hold it.
func TestRepairParentCycleGoesBackToTheBaseParent(t *testing.T) {
	pr, s, x := "01JD0000000000000000000001", "01JD0000000000000000000002", "01JD0000000000000000000003"
	base := memFiles{
		pathOf(pr): taskFile(pr, "WEB-1", wsA, "", "2026-10-31T10:00:00Z", "kind: project"),
		pathOf(s):  taskFile(s, "WEB-2", wsA, pr, "2026-10-31T10:00:00Z", "kind: story"),
		pathOf(x):  taskFile(x, "WEB-3", wsA, s, "2026-10-31T10:00:00Z", "kind: step"),
	}
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(pr):                         base[pathOf(pr)],
		pathOf(s):                          taskFile(s, "WEB-2", wsA, x, "2026-10-31T11:00:00Z", "kind: story"), // a story under its step
		pathOf(x):                          base[pathOf(x)],
	}
	after, rs := repair(t, RepairInput{Files: f, Base: base})
	if p := decoded(t, after[pathOf(s)]).Parent; p == nil || p.String() != pr {
		t.Errorf("story.parent = %v, want the project", p)
	}
	if len(rs) != 1 || rs[0].Value != `{"from":"`+x+`","to":"`+pr+`"}` {
		t.Errorf("repaired = %+v", rs)
	}
}

// A task with no ancestor to hold it goes under the 未分類 project of its
// workspace, made in the same repair; a step becomes a task there.
func TestRepairDeletedParent(t *testing.T) {
	p, c, st := "01JD0000000000000000000001", "01JD0000000000000000000002", "01JD0000000000000000000003"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(p):                          taskFile(p, "WEB-1", wsA, "", "2026-10-31T10:00:00Z", "kind: project", "deleted_at: 2026-10-31T10:00:00Z"),
		pathOf(c):                          taskFile(c, "WEB-2", wsA, p, "2026-10-31T10:00:00Z"),
		pathOf(st):                         taskFile(st, "WEB-3", wsA, p, "2026-10-31T10:00:00Z", "kind: step"),
	}
	after, rs := repair(t, RepairInput{Files: f})
	if got := decoded(t, after[pathOf(c)]); got.Parent == nil || got.Parent.String() != uncatA || got.Kind != "task" {
		t.Errorf("task = parent %v kind %s", got.Parent, got.Kind)
	}
	if got := decoded(t, after[pathOf(st)]); got.Parent == nil || got.Parent.String() != uncatA || got.Kind != "task" {
		t.Errorf("step = parent %v kind %s", got.Parent, got.Kind)
	}
	want := []Repaired{
		{ULID: mustULID(c), Kind: "parent_deleted", Value: `{"from":"` + p + `","to":"` + uncatA + `"}`, Author: "github.com/x"},
		{ULID: mustULID(st), Kind: "parent_deleted", Value: `{"from":"` + p + `","to":"` + uncatA + `","kind_from":"step","kind_to":"task"}`, Author: "github.com/x"},
	}
	if !slices.Equal(rs, want) {
		t.Errorf("repaired = %+v", rs)
	}
	u := decoded(t, after[pathOf(uncatA)])
	if u.Kind != "project" || u.Title != "未分類" || u.Parent != nil || u.DeletedAt != nil || u.Workspace != mustULID(wsA) || u.ID != "WEB-4" {
		t.Errorf("未分類 = %+v", u)
	}
}

// The 未分類 project that is there is used again, and the task goes after what
// is under it.
func TestRepairUncategorizedIsReused(t *testing.T) {
	u, k, p, c := "01JD0000000000000000000001", "01JD0000000000000000000002", "01JD0000000000000000000003", "01JD0000000000000000000004"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(u):                          taskFile(u, "WEB-1", wsA, "", "2026-10-31T10:00:00Z", "kind: project", "title: 未分類"),
		pathOf(k):                          taskFile(k, "WEB-2", wsA, u, "2026-10-31T10:00:00Z", "rank: a1"),
		pathOf(p):                          taskFile(p, "WEB-3", wsA, "", "2026-10-31T10:00:00Z", "kind: project", "deleted_at: 2026-10-31T10:00:00Z"),
		pathOf(c):                          taskFile(c, "WEB-4", wsA, p, "2026-10-31T10:00:00Z", "rank: a0"),
	}
	after, rs := repair(t, RepairInput{Files: f})
	if got := decoded(t, after[pathOf(c)]); got.Parent == nil || got.Parent.String() != u || got.Rank != "a2" {
		t.Errorf("task = parent %v rank %s", got.Parent, got.Rank)
	}
	if len(rs) != 1 || len(after) != len(f) {
		t.Errorf("repaired = %+v, files %d -> %d; want no new project", rs, len(f), len(after))
	}
}

// A step under a deleted story goes under the epic above as a task, the
// nearest ancestor that can hold it; a story under a deleted epic goes under
// the project.
func TestRepairDeletedParentGoesToTheNearestAncestor(t *testing.T) {
	pr, e, s, x, s2 := "01JD0000000000000000000001", "01JD0000000000000000000002", "01JD0000000000000000000003", "01JD0000000000000000000004", "01JD0000000000000000000005"
	gone := "deleted_at: 2026-10-31T10:00:00Z"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(pr):                         taskFile(pr, "WEB-1", wsA, "", "2026-10-31T10:00:00Z", "kind: project"),
		pathOf(e):                          taskFile(e, "WEB-2", wsA, pr, "2026-10-31T10:00:00Z", "kind: epic", gone),
		pathOf(s):                          taskFile(s, "WEB-3", wsA, e, "2026-10-31T10:00:00Z", "kind: story", gone),
		pathOf(x):                          taskFile(x, "WEB-4", wsA, s, "2026-10-31T10:00:00Z", "kind: step"),
		pathOf(s2):                         taskFile(s2, "WEB-5", wsA, e, "2026-10-31T10:00:00Z", "kind: story"),
	}
	// The epic is deleted too, so both go under the project.
	after, rs := repair(t, RepairInput{Files: f})
	if got := decoded(t, after[pathOf(x)]); got.Parent == nil || got.Parent.String() != pr || got.Kind != "task" {
		t.Errorf("step = parent %v kind %s", got.Parent, got.Kind)
	}
	if got := decoded(t, after[pathOf(s2)]); got.Parent == nil || got.Parent.String() != pr || got.Kind != "story" {
		t.Errorf("story = parent %v kind %s", got.Parent, got.Kind)
	}
	if len(rs) != 2 || rs[0].Value != `{"from":"`+s+`","to":"`+pr+`","kind_from":"step","kind_to":"task"}` {
		t.Errorf("repaired = %+v", rs)
	}
	// With the epic alive, the step lands under it, as a task.
	f[pathOf(e)] = taskFile(e, "WEB-2", wsA, pr, "2026-10-31T10:00:00Z", "kind: epic")
	after, _ = repair(t, RepairInput{Files: f})
	if got := decoded(t, after[pathOf(x)]); got.Parent == nil || got.Parent.String() != e || got.Kind != "task" {
		t.Errorf("step under a live epic = parent %v kind %s", got.Parent, got.Kind)
	}
}

func TestRepairWorkspaceMismatchFollowsParent(t *testing.T) {
	p, c := "01JD0000000000000000000001", "01JD0000000000000000000002"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		model.WorkspacePath(mustULID(wsB)): wsFile(wsB, "api", "API"),
		pathOf(p):                          taskFile(p, "API-1", wsB, "", "2026-10-31T10:00:00Z", "former_ids: [WEB-1]"),
		pathOf(c):                          taskFile(c, "WEB-2", wsA, p, "2026-10-31T10:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: f})
	got := decoded(t, after[pathOf(c)])
	if got.Workspace.String() != wsB || got.ID != "API-2" || !slices.Contains(got.FormerIDs, "WEB-2") {
		t.Errorf("child = %s %s %v", got.Workspace, got.ID, got.FormerIDs)
	}
	if len(rs) != 1 || rs[0].Kind != "workspace_mismatch" {
		t.Errorf("repaired = %+v", rs) // id_renumbered of the same task is not told
	}
}

func TestRepairRevivesRemovedWorkspace(t *testing.T) {
	x := "01JD0000000000000000000001"
	base := memFiles{model.WorkspacePath(mustULID(wsB)): wsFile(wsB, "api", "API")}
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(x):                          taskFile(x, "API-1", wsB, "", "2026-10-31T10:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: f, Base: base})
	if after[model.WorkspacePath(mustULID(wsB))] == nil || len(rs) != 1 || rs[0].Kind != "workspace_revived" {
		t.Errorf("workspace not revived: %+v", rs)
	}
}

func TestRepairInvalidPrefixBackToBase(t *testing.T) {
	base := memFiles{model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB")}
	f := memFiles{model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "web!")}
	after, rs := repair(t, RepairInput{Files: f, Base: base})
	w, _ := model.DecodeWorkspace(after[model.WorkspacePath(mustULID(wsA))])
	if w.IDPrefix != "WEB" || len(rs) != 1 || rs[0].Kind != "id_prefix" {
		t.Errorf("prefix = %s, %+v", w.IDPrefix, rs)
	}
}

func TestRepairPrefixAndNameDuplicatesOnlyTell(t *testing.T) {
	later := strings.Replace(string(wsFile(wsB, "web", "WEB")), "prefix_set: 2026-10-30", "prefix_set: 2026-10-31", 1)
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		model.WorkspacePath(mustULID(wsB)): []byte(strings.Replace(later, "created: 2026-10-30", "created: 2026-10-31", 1)),
	}
	out, rs, _, err := Repair(RepairInput{Files: f, Author: func(string) string { return "x" }, OnOrigin: func(model.ULID, string) bool { return false }, Changed: func(string) bool { return true }})
	if err != nil || len(out) != 0 {
		t.Fatalf("out = %v, err = %v; duplicates must not change data", out, err)
	}
	kinds := []string{}
	for _, r := range rs {
		if r.ULID.String() != wsB {
			t.Errorf("told the earlier workspace: %+v", r)
		}
		kinds = append(kinds, r.Kind)
	}
	if !slices.Equal(kinds, []string{"prefix_duplicate", "name_duplicate"}) {
		t.Errorf("kinds = %v", kinds)
	}
}

func TestRepairIDPrefixMismatchRenumbersSilently(t *testing.T) {
	x := "01JD0000000000000000000001"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "FE"),
		pathOf(x):                          taskFile(x, "WEB-3", wsA, "", "2026-10-31T10:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: f})
	if got := decoded(t, after[pathOf(x)]); got.ID != "FE-1" || !slices.Contains(got.FormerIDs, "WEB-3") {
		t.Errorf("task = %s %v", got.ID, got.FormerIDs)
	}
	if len(rs) != 1 || rs[0].Kind != "id_renumbered" || rs[0].Author != "" {
		t.Errorf("repaired = %+v", rs)
	}
}

// threeCycle is a -> b -> c -> a, where only a and c have another parent
// than in base3 and b was updated last.
var (
	cyA, cyB, cyC = "01JD0000000000000000000001", "01JD0000000000000000000002", "01JD0000000000000000000003"
	threeCycle    = memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(cyA):                        taskFile(cyA, "WEB-1", wsA, cyB, "2026-10-31T11:00:00Z"),
		pathOf(cyB):                        taskFile(cyB, "WEB-2", wsA, cyC, "2026-10-31T12:00:00Z"),
		pathOf(cyC):                        taskFile(cyC, "WEB-3", wsA, cyA, "2026-10-31T10:00:00Z"),
	}
	base3 = memFiles{
		pathOf(cyA): taskFile(cyA, "WEB-1", wsA, "", "2026-10-31T09:00:00Z"),
		pathOf(cyB): taskFile(cyB, "WEB-2", wsA, cyC, "2026-10-31T09:00:00Z"),
		pathOf(cyC): taskFile(cyC, "WEB-3", wsA, "", "2026-10-31T09:00:00Z"),
	}
)

// parents returns the parents of a, b and c of threeCycle in after.
func parents(t *testing.T, after memFiles) []string {
	t.Helper()
	var out []string
	for _, u := range []string{cyA, cyB, cyC} {
		p := ""
		if x := decoded(t, after[pathOf(u)]).Parent; x != nil {
			p = x.String()
		}
		out = append(out, p)
	}
	return out
}

func TestRepairThreeCycleWithBaseRevertsAChangedParent(t *testing.T) {
	after, rs := repair(t, RepairInput{Files: threeCycle, Base: base3})
	if got := parents(t, after); !slices.Equal(got, []string{uncatA, cyC, cyA}) {
		t.Errorf("parents = %v", got) // a, changed and later than c, leaves for 未分類 (its base root cannot hold a task); b, updated last, keeps c
	}
	if len(rs) != 1 || rs[0].Kind != "parent_cycle" || rs[0].ULID.String() != cyA || rs[0].Value != `{"from":"`+cyB+`","to":"`+uncatA+`"}` {
		t.Errorf("repaired = %+v", rs)
	}
}

func TestRepairThreeCycleWithoutBaseMovesTheLatestToUncategorized(t *testing.T) {
	after, rs := repair(t, RepairInput{Files: threeCycle})
	if got := parents(t, after); !slices.Equal(got, []string{cyB, uncatA, cyA}) {
		t.Errorf("parents = %v", got)
	}
	if len(rs) != 1 || rs[0].ULID.String() != cyB {
		t.Errorf("repaired = %+v", rs)
	}
}

func TestRepairCycleWithoutChangedMemberPicksAmongAll(t *testing.T) {
	after, rs := repair(t, RepairInput{Files: threeCycle, Changed: func(string) bool { return false }})
	if got := parents(t, after); !slices.Equal(got, []string{cyB, uncatA, cyA}) || len(rs) != 1 {
		t.Errorf("parents = %v, repaired = %+v", got, rs)
	}
}

func TestRepairCycleBaseParentStillCyclesGoesToUncategorized(t *testing.T) {
	// Only c has another parent than in the base, and its base parent b
	// still leads back to c.
	base := memFiles{
		pathOf(cyA): taskFile(cyA, "WEB-1", wsA, cyB, "2026-10-31T09:00:00Z"),
		pathOf(cyB): taskFile(cyB, "WEB-2", wsA, cyC, "2026-10-31T09:00:00Z"),
		pathOf(cyC): taskFile(cyC, "WEB-3", wsA, cyB, "2026-10-31T09:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: threeCycle, Base: base})
	if got := parents(t, after); !slices.Equal(got, []string{cyB, cyC, uncatA}) || len(rs) != 1 {
		t.Errorf("parents = %v, repaired = %+v", got, rs)
	}
}

func TestRepairRevivesUndecodableWorkspaceAsItWas(t *testing.T) {
	x := "01JD0000000000000000000001"
	raw := []byte("---\n: [\n")
	base := memFiles{model.WorkspacePath(mustULID(wsB)): raw}
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(x):                          taskFile(x, "API-1", wsB, "", "2026-10-31T10:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: f, Base: base})
	if !bytes.Equal(after[model.WorkspacePath(mustULID(wsB))], raw) || len(rs) != 1 || rs[0].Kind != "workspace_revived" {
		t.Errorf("not revived as it was: %+v", rs)
	}
}

func TestRepairIDCollisionBothOnOriginKeepsSmallerULID(t *testing.T) {
	x, y := "01JD0000000000000000000001", "01JD0000000000000000000002"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(x):                          taskFile(x, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		pathOf(y):                          taskFile(y, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: f, OnOrigin: func(model.ULID, string) bool { return true }})
	if gx, gy := decoded(t, after[pathOf(x)]).ID, decoded(t, after[pathOf(y)]).ID; gx != "WEB-1" || gy != "WEB-2" {
		t.Errorf("ids = %s %s", gx, gy)
	}
	if len(rs) != 1 || rs[0].ULID.String() != y {
		t.Errorf("repaired = %+v", rs)
	}
}

func TestRepairUnnumberableTaskDoesNotTakePartInACollision(t *testing.T) {
	x, y := "01JD0000000000000000000001", "01JD0000000000000000000002"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		model.WorkspacePath(mustULID(wsB)): wsFile(wsB, "api", "api!"),
		pathOf(x):                          taskFile(x, "WEB-1", wsB, "", "2026-10-31T10:00:00Z"), // malformed prefix
		pathOf(y):                          taskFile(y, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: f})
	if got := decoded(t, after[pathOf(y)]).ID; got != "WEB-1" || len(rs) != 0 {
		t.Errorf("y = %s, repaired = %+v", got, rs)
	}
	delete(f, model.WorkspacePath(mustULID(wsB))) // no workspace
	after, rs = repair(t, RepairInput{Files: f})
	if got := decoded(t, after[pathOf(y)]).ID; got != "WEB-1" || len(rs) != 0 {
		t.Errorf("y = %s, repaired = %+v", got, rs)
	}
}

func TestRepairNumbersCollisionGroupsInOneULIDOrder(t *testing.T) {
	u1, u2, u3, u4 := "01JD0000000000000000000001", "01JD0000000000000000000002", "01JD0000000000000000000003", "01JD0000000000000000000004"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(u1):                         taskFile(u1, "WEB-2", wsA, "", "2026-10-31T10:00:00Z"),
		pathOf(u2):                         taskFile(u2, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		pathOf(u3):                         taskFile(u3, "WEB-2", wsA, "", "2026-10-31T10:00:00Z"),
		pathOf(u4):                         taskFile(u4, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
	}
	after, _ := repair(t, RepairInput{Files: f})
	if g3, g4 := decoded(t, after[pathOf(u3)]).ID, decoded(t, after[pathOf(u4)]).ID; g3 != "WEB-3" || g4 != "WEB-4" {
		t.Errorf("ids = %s %s, want WEB-3 WEB-4", g3, g4)
	}
}

func TestRepairKeepsCommentsAndUnknownKeys(t *testing.T) {
	p, c := "01JD0000000000000000000001", "01JD0000000000000000000002"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(p):                          taskFile(p, "WEB-1", wsA, "", "2026-10-31T10:00:00Z", "deleted_at: 2026-10-31T10:00:00Z"),
		pathOf(c):                          taskFile(c, "WEB-2", wsA, p, "2026-10-31T10:00:00Z", "# a note\nmine: 1"),
	}
	after, _ := repair(t, RepairInput{Files: f})
	if s := string(after[pathOf(c)]); !strings.Contains(s, "# a note\n") || !strings.Contains(s, "mine: 1\n") {
		t.Errorf("lost a comment or a key:\n%s", s)
	}
}

// A child of a deleted parent goes after the live tasks under the parent it
// gets, in its workspace only.
func TestRepairMovedTaskGoesAfterTheLiveSiblings(t *testing.T) {
	pr, other, gone, e, c1, c2 := "01JD0000000000000000000001", "01JD0000000000000000000002", "01JD0000000000000000000003",
		"01JD0000000000000000000004", "01JD0000000000000000000005", "01JD0000000000000000000006"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(pr):                         taskFile(pr, "WEB-1", wsA, "", "2026-10-31T10:00:00Z", "kind: project"),
		pathOf(other):                      taskFile(other, "WEB-2", wsA, pr, "2026-10-31T10:00:00Z", "rank: a1"),
		pathOf(gone):                       taskFile(gone, "WEB-3", wsA, pr, "2026-10-31T10:00:00Z", "rank: a7", "deleted_at: 2026-10-31T10:00:00Z"),
		pathOf(e):                          taskFile(e, "WEB-4", wsA, pr, "2026-10-31T10:00:00Z", "kind: epic", "rank: a0", "deleted_at: 2026-10-31T10:00:00Z"),
		pathOf(c1):                         taskFile(c1, "WEB-5", wsA, e, "2026-10-31T10:00:00Z", "rank: a0"),
		pathOf(c2):                         taskFile(c2, "WEB-6", wsA, e, "2026-10-31T10:00:00Z", "rank: a1"),
	}
	after, _ := repair(t, RepairInput{Files: f})
	if g1, g2 := decoded(t, after[pathOf(c1)]).Rank, decoded(t, after[pathOf(c2)]).Rank; g1 != "a2" || g2 != "a3" {
		t.Errorf("ranks = %s %s, want a2 a3", g1, g2)
	}
}

func TestRepairBrokenParentIsNotDeleted(t *testing.T) {
	p, c := "01JD0000000000000000000001", "01JD0000000000000000000002"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(p):                          []byte("---\n: [\n"),
		pathOf(c):                          taskFile(c, "WEB-2", wsA, p, "2026-10-31T10:00:00Z"),
	}
	if after, rs := repair(t, RepairInput{Files: f}); len(rs) != 0 || !bytes.Equal(after[pathOf(c)], f[pathOf(c)]) {
		t.Errorf("repaired = %+v", rs)
	}
}

// A forward is not a task, and not a broken file either: it is left as it is.
func TestRepairSkipsForwards(t *testing.T) {
	t.Parallel()
	x, fwd := "01JD0000000000000000000001", "01JD0000000000000000000002"
	forward := []byte("---\nulid: " + fwd + "\nmoved_to: 01JC0M3R5W7M2X9V4T6R5N8P1B\nmoved_at: 2026-10-31T10:00:00Z\n---\n")
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(x):                          taskFile(x, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		pathOf(fwd):                        forward,
	}
	after, rs := repair(t, RepairInput{Files: f})
	if len(rs) != 0 || !bytes.Equal(after[pathOf(fwd)], forward) || !bytes.Equal(after[pathOf(x)], f[pathOf(x)]) {
		t.Errorf("repaired = %+v", rs)
	}
}

func TestRepairMismatchedIDDoesNotKeepACollision(t *testing.T) {
	x, y := "01JD0000000000000000000001", "01JD0000000000000000000002"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		model.WorkspacePath(mustULID(wsB)): wsFile(wsB, "api", "API"),
		pathOf(x):                          taskFile(x, "WEB-1", wsB, "", "2026-10-31T10:00:00Z"), // renumbered for its prefix
		pathOf(y):                          taskFile(y, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: f})
	if gx, gy := decoded(t, after[pathOf(x)]).ID, decoded(t, after[pathOf(y)]).ID; gx != "API-1" || gy != "WEB-1" {
		t.Errorf("ids = %s %s", gx, gy)
	}
	if len(rs) != 1 || rs[0].Kind != "id_renumbered" || rs[0].Value != `"WEB-1"` {
		t.Errorf("repaired = %+v", rs)
	}
}

func TestRepairWithoutBaseLeavesWhatItCannotRestore(t *testing.T) {
	x := "01JD0000000000000000000001"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "web!"),
		pathOf(x):                          taskFile(x, "WEB-1", wsB, "", "2026-10-31T10:00:00Z"),
	}
	if _, rs := repair(t, RepairInput{Files: f}); len(rs) != 0 {
		t.Errorf("repaired = %+v", rs)
	}
}

func TestRepairUnreadableNumbers(t *testing.T) {
	x, y := "01JD0000000000000000000001", "01JD0000000000000000000002"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		pathOf(x):                          taskFile(x, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		pathOf(y):                          taskFile(y, "WEB-2", wsA, "", "2026-10-31T10:00:00Z"),
		model.NumbersPath:                  []byte("[\n"),
	}
	in := RepairInput{Files: f, Author: func(string) string { return "x" }, OnOrigin: func(model.ULID, string) bool { return false }, Changed: func(string) bool { return true }}
	if _, _, _, err := Repair(in); err != nil {
		t.Errorf("nothing to number, but err = %v", err)
	}
	f[pathOf(y)] = taskFile(y, "WEB-1", wsA, "", "2026-10-31T10:00:00Z")
	if _, _, _, err := Repair(in); err == nil {
		t.Error("numbered past a numbers.yaml it cannot read")
	}
}
