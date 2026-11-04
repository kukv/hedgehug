package server

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/workspaces"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// add adds a task through usecase, as the CLI would, and returns its ID.
func (ts *testServer) add(title, parent string) string {
	ts.t.Helper()
	res, err := tasks.Add(ts.session(), tasks.AddInput{Title: title, Parent: parent})
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
	Tasks  []*usecase.TaskView `json:"tasks"`
	Groups []struct {
		Key   string   `json:"key"`
		ULIDs []string `json:"ulids"`
	} `json:"groups"`
	Roots []*usecase.TaskView `json:"roots"`
	Cycle []*usecase.TaskView `json:"cycle"`
}

func TestListTasks(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	ts := newTestServer(t)
	resp := ts.call("GET", "/api/tasks", nil, 200, nil)
	b := resp.Body
	if string(b) != `{"tasks":[]}`+"\n" && string(b) != `{"tasks":[]}` {
		t.Errorf("body = %s, want an empty tasks array", b)
	}
}

func TestListTasksGroups(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.add("one", "")
	var out listBody
	ts.call("GET", "/api/tasks?q="+url.QueryEscape("group:status"), nil, 200, &out)
	if len(out.Groups) == 0 || len(out.Groups[0].ULIDs) != 1 {
		t.Errorf("groups = %+v", out.Groups)
	}
}

func TestTreeHasProgress(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	res, err := tasks.Add(ts.session(), tasks.AddInput{Title: "p", Kind: "project"})
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
	t.Parallel()
	ts := newTestServer(t)
	res, err := tasks.Add(ts.session(), tasks.AddInput{Title: "p", Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	pv, _ := res.View()
	id := ts.add("c", pv.ID)
	body := "- [x] a\n- [ ] b\n"
	if _, err := tasks.Patch(ts.session(), id, tasks.PatchInput{Body: &body}); err != nil {
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
			if v.Checklist == nil || *v.Checklist != (usecase.Count{Done: 1, Total: 2}) {
				t.Errorf("checklist = %+v", v.Checklist)
			}
		}
	}
}

func TestShowTask(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	var out usecase.ShowView
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
	t.Parallel()
	ts := newTestServer(t)
	p := ts.problem("GET", "/api/tasks/ME-99", nil, 404)
	if p.Status != 404 || p.Type == "" || p.Detail == "" {
		t.Errorf("problem = %+v", p)
	}
}

func TestUnknownStoreIsProblem(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	p := ts.problem("GET", "/api/tasks?store=nope", nil, 404)
	if p.Type != "store.not_found" || p.Args["name"] != "nope" {
		t.Errorf("problem = %+v, want store.not_found for nope", p)
	}
}

func TestBadQueryIsProblem(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	p := ts.problem("GET", "/api/tasks?q="+url.QueryEscape("sort:"), nil, 400)
	if p.Type == "" {
		t.Errorf("problem = %+v", p)
	}
}

func TestTaskHistory(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	title := "two"
	if _, err := tasks.Patch(ts.session(), id, tasks.PatchInput{Assigns: []usecase.Assign{{Key: "title", Op: "=", Value: title}}}); err != nil {
		t.Fatal(err)
	}
	var out []usecase.LogView
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

// TestTaskHistoryAcrossAMerge has two clones change different fields of one
// task and sync, so that the merge of the fields is a commit of its own; the
// history of the task lists it beside the commits of both sides.
func TestTaskHistoryAcrossAMerge(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	remote := ts.addRemote()
	id := ts.add("one", "")
	if _, err := replication.Sync(ts.session(), false, true); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{}
	for k, v := range ts.env {
		env[k] = v
	}
	hh := filepath.Join(t.TempDir(), "hh2")
	if err := os.MkdirAll(hh, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hh, "config.yaml"), []byte(defaultAccountYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	env["HEDGEHUG_HOME"] = hh
	other := &usecase.Session{Getenv: func(k string) string { return env[k] }, Now: time.Now, Warn: func(string) {}}
	if _, err := stores.StoreAdd(other, "me", []string{remote}, "", ""); err != nil {
		t.Fatal(err)
	}

	if _, err := tasks.Patch(ts.session(), id, tasks.PatchInput{Assigns: []usecase.Assign{{Key: "title", Op: "=", Value: "from me"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Patch(other, id, tasks.PatchInput{Assigns: []usecase.Assign{{Key: "status", Op: "=", Value: "in_progress"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := replication.Sync(other, false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := replication.Sync(ts.session(), false, true); err != nil {
		t.Fatal(err)
	}

	var out []usecase.LogView
	ts.call("GET", "/api/tasks/"+id+"/history", nil, 200, &out)
	var got []string
	for _, v := range out {
		if v.Op != nil {
			got = append(got, *v.Op)
		}
	}
	if len(got) != 4 || got[0] != "merge" {
		t.Errorf("history ops = %v; want the merge first, then both sides and the add", got)
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
	author := config.CommitAuthor(testAccount)
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		ts.t.Fatal(err)
	}
}

func TestEmptyTreeIsArray(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	resp := ts.call("GET", "/api/tasks?tree=true", nil, 200, nil)
	if b := string(resp.Body); b != `{"roots":[]}`+"\n" && b != `{"roots":[]}` {
		t.Errorf("body = %s, want an empty roots array", b)
	}
}

func TestTreeParentCycle(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	var out listBody
	ts.call("GET", "/api/tasks?store=me", nil, 200, &out)
	if len(out.Tasks) != 1 {
		t.Errorf("list: %+v", out.Tasks)
	}
	var sv usecase.ShowView
	ts.call("GET", "/api/tasks/"+id+"?store=me", nil, 200, &sv)
	if sv.ID != id {
		t.Errorf("show: %s", sv.ID)
	}
	ts.call("GET", "/api/tasks/"+id+"/history?store=me", nil, 200, nil)
}

func TestShowByFormerID(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	old := ts.add("one", "")
	if _, err := workspaces.WorkspacePrefix(ts.session(), "me", "XX"); err != nil {
		t.Fatal(err)
	}
	var out usecase.ShowView
	ts.call("GET", "/api/tasks/"+old, nil, 200, &out)
	if out.ID == old || out.TaskView == nil || out.Title != "one" {
		t.Errorf("show by former ID %s: %+v", old, out.TaskView)
	}
}
