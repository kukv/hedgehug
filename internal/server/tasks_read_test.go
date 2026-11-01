package server

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
)

// add adds a task through ops, as the CLI would, and returns its ID.
func (ts *testServer) add(title, parent string) string {
	ts.t.Helper()
	res, err := ts.session().Add(ops.AddInput{Title: title, Parent: parent})
	if err != nil {
		ts.t.Fatal(err)
	}
	v, err := res.View()
	if err != nil {
		ts.t.Fatal(err)
	}
	return v.ID
}

type listBody struct {
	Tasks  []*ops.TaskView `json:"tasks"`
	Groups []struct {
		Key   string   `json:"key"`
		ULIDs []string `json:"ulids"`
	} `json:"groups"`
	Roots []*ops.TaskView `json:"roots"`
	Cycle []*ops.TaskView `json:"cycle"`
}

func TestListTasks(t *testing.T) {
	ts := newTestServer(t)
	ts.add("one", "")
	ts.add("two", "")
	var out listBody
	ts.call("GET", "/api/tasks", nil, 200, &out)
	if len(out.Tasks) != 2 || out.Roots != nil {
		t.Fatalf("tasks = %+v", out)
	}
	ts.call("GET", "/api/tasks?q="+url.QueryEscape("one"), nil, 200, &out)
	if len(out.Tasks) != 1 || out.Tasks[0].Title != "one" {
		t.Errorf("q=one: %+v", out.Tasks)
	}
}

func TestListTasksEmptyIsArray(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.call("GET", "/api/tasks", nil, 200, nil)
	b := resp.Body
	if string(b) != `{"tasks":[]}`+"\n" && string(b) != `{"tasks":[]}` {
		t.Errorf("body = %s, want an empty tasks array", b)
	}
}

func TestListTasksGroups(t *testing.T) {
	ts := newTestServer(t)
	ts.add("one", "")
	var out listBody
	ts.call("GET", "/api/tasks?q="+url.QueryEscape("group:status"), nil, 200, &out)
	if len(out.Groups) == 0 || len(out.Groups[0].ULIDs) != 1 {
		t.Errorf("groups = %+v", out.Groups)
	}
}

func TestTreeHasProgress(t *testing.T) {
	ts := newTestServer(t)
	res, err := ts.session().Add(ops.AddInput{Title: "p", Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	pv, _ := res.View()
	ts.add("child", pv.ID)
	var out listBody
	ts.call("GET", "/api/tasks?tree=true", nil, 200, &out)
	if len(out.Roots) != 1 || len(out.Roots[0].Children) != 1 || out.Tasks != nil {
		t.Fatalf("tree = %+v", out)
	}
	if p := out.Roots[0].Progress; p == nil || p.Total != 1 {
		t.Errorf("progress = %+v", p)
	}
}

func TestListHasChecklistAndProgress(t *testing.T) {
	ts := newTestServer(t)
	res, err := ts.session().Add(ops.AddInput{Title: "p", Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	pv, _ := res.View()
	id := ts.add("c", pv.ID)
	body := "- [x] a\n- [ ] b\n"
	if _, err := ts.session().Patch(id, ops.PatchInput{Body: &body}); err != nil {
		t.Fatal(err)
	}
	var out listBody
	ts.call("GET", "/api/tasks", nil, 200, &out)
	for _, v := range out.Tasks {
		switch v.ID {
		case pv.ID:
			if v.Progress == nil || v.Progress.Total != 1 {
				t.Errorf("project progress = %+v", v.Progress)
			}
		case id:
			if v.Checklist == nil || *v.Checklist != (ops.Count{Done: 1, Total: 2}) {
				t.Errorf("checklist = %+v", v.Checklist)
			}
		}
	}
}

func TestShowTask(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	var out ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &out)
	if out.TaskView == nil || out.ID != id || out.Comments == nil || len(out.History) == 0 {
		t.Errorf("show = %+v", out)
	}
	ulid := out.ULID
	ts.call("GET", "/api/tasks/"+ulid, nil, 200, &out)
	if out.ID != id {
		t.Errorf("by ULID: %s", out.ID)
	}
}

func TestShowTaskNotFound(t *testing.T) {
	ts := newTestServer(t)
	p := ts.problem("GET", "/api/tasks/ME-99", nil, 404)
	if p.Status != 404 || p.Type == "" || p.Detail == "" {
		t.Errorf("problem = %+v", p)
	}
}

func TestUnknownStoreIsProblem(t *testing.T) {
	ts := newTestServer(t)
	p := ts.problem("GET", "/api/tasks?store=nope", nil, 404)
	if p.Type != "store.not_found" || p.Args["name"] != "nope" {
		t.Errorf("problem = %+v, want store.not_found for nope", p)
	}
}

func TestBadQueryIsProblem(t *testing.T) {
	ts := newTestServer(t)
	p := ts.problem("GET", "/api/tasks?q="+url.QueryEscape("sort:"), nil, 400)
	if p.Type == "" {
		t.Errorf("problem = %+v", p)
	}
}

func TestTaskHistory(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	title := "two"
	if _, err := ts.session().Patch(id, ops.PatchInput{Assigns: []ops.Assign{{Key: "title", Op: "=", Value: title}}}); err != nil {
		t.Fatal(err)
	}
	var out []ops.LogView
	ts.call("GET", "/api/tasks/"+id+"/history", nil, 200, &out)
	if len(out) != 2 || out[0].Op == nil || *out[0].Op != "set" {
		t.Fatalf("history = %+v", out)
	}
	ts.call("GET", "/api/tasks/"+id+"/history?limit=1", nil, 200, &out)
	if len(out) != 1 {
		t.Errorf("limit=1: %d entries", len(out))
	}
	if p := ts.problem("GET", "/api/tasks/"+id+"/history?limit=0", nil, 400); p.Type != "request.invalid" {
		t.Errorf("limit=0: problem = %+v", p)
	}
}

// rawTask commits a task file past the store's checks, as a sync or a hand
// edit could.
func (ts *testServer) rawTask(id string, ulid model.ULID, front string) {
	ts.t.Helper()
	ss := ts.session()
	_, e, err := ss.Entry()
	if err != nil {
		ts.t.Fatal(err)
	}
	s, err := ss.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	_, ix, err := ss.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	rel := model.TaskPath(ulid)
	p := filepath.Join(e.Path, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		ts.t.Fatal(err)
	}
	body := "---\nulid: " + ulid.String() + "\nid: " + id + "\nworkspace: " + ix.Workspaces()[0].ULID.String() + "\nkind: task\nstatus: todo\n" + front + "---\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	author, err := config.CommitAuthor(config.Account{}, false, ts.getenv)
	if err != nil {
		ts.t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		ts.t.Fatal(err)
	}
}

func TestEmptyTreeIsArray(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.call("GET", "/api/tasks?tree=true", nil, 200, nil)
	if b := string(resp.Body); b != `{"roots":[]}`+"\n" && b != `{"roots":[]}` {
		t.Errorf("body = %s, want an empty roots array", b)
	}
}

func TestTreeParentCycle(t *testing.T) {
	ts := newTestServer(t)
	x, y := model.NewULID(), model.NewULID()
	ts.rawTask("ME-1", x, "title: X\nparent: "+y.String()+"\n")
	ts.rawTask("ME-2", y, "title: Y\nparent: "+x.String()+"\n")
	var out listBody
	ts.call("GET", "/api/tasks?tree=true", nil, 200, &out)
	if len(out.Roots) != 0 || len(out.Cycle) != 2 || !out.Cycle[0].InCycle {
		t.Errorf("roots = %d, cycle = %+v", len(out.Roots), out.Cycle)
	}
}

func TestStoreParamNamesTheStore(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	var out listBody
	ts.call("GET", "/api/tasks?store=me", nil, 200, &out)
	if len(out.Tasks) != 1 {
		t.Errorf("list: %+v", out.Tasks)
	}
	var sv ops.ShowView
	ts.call("GET", "/api/tasks/"+id+"?store=me", nil, 200, &sv)
	if sv.ID != id {
		t.Errorf("show: %s", sv.ID)
	}
	ts.call("GET", "/api/tasks/"+id+"/history?store=me", nil, 200, nil)
}

func TestShowByFormerID(t *testing.T) {
	ts := newTestServer(t)
	old := ts.add("one", "")
	if _, err := ts.session().WorkspacePrefix("me", "XX"); err != nil {
		t.Fatal(err)
	}
	var out ops.ShowView
	ts.call("GET", "/api/tasks/"+old, nil, 200, &out)
	if out.ID == old || out.TaskView == nil || out.Title != "one" {
		t.Errorf("show by former ID %s: %+v", old, out.TaskView)
	}
}
