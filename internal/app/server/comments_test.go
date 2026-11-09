package server

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

type commentBody struct {
	Comment struct {
		ULID string `json:"ulid"`
		Body string `json:"body"`
	} `json:"comment"`
	Commit *string `json:"commit"`
}

func TestComments(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	var out commentBody
	ts.call("POST", "/api/tasks/"+id+"/comments", map[string]any{"body": "first"}, 201, &out)
	if out.Comment.ULID == "" || out.Comment.Body != "first\n" || out.Commit == nil {
		t.Fatalf("added = %+v", out)
	}
	cid := out.Comment.ULID
	ts.call("PATCH", "/api/tasks/"+id+"/comments/"+cid, map[string]any{"body": "edited"}, 200, &out)
	if out.Comment.Body != "edited\n" {
		t.Errorf("edited = %+v", out)
	}
	var show usecase.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if len(show.Comments) != 1 || show.Comments[0].Body != "edited\n" {
		t.Errorf("comments = %+v", show.Comments)
	}
	// A comment of another task is not found under this one.
	other := ts.add("two", "")
	if p := ts.problem("PATCH", "/api/tasks/"+other+"/comments/"+cid, map[string]any{"body": "x"}, 404); p.Type != "comment.not_found" {
		t.Errorf("other task: %+v", p)
	}
	ts.problem("DELETE", "/api/tasks/"+other+"/comments/"+cid, nil, 404)
	ts.call("DELETE", "/api/tasks/"+id+"/comments/"+cid, nil, 200, &out)
	if out.Comment.Body != "edited\n" || out.Commit == nil {
		t.Errorf("removed = %+v", out)
	}
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if len(show.Comments) != 0 {
		t.Errorf("comments after delete = %+v", show.Comments)
	}
	ts.problem("POST", "/api/tasks/"+id+"/comments", map[string]any{"body": ""}, 400)
	ts.problem("POST", "/api/tasks/ME-99/comments", map[string]any{"body": "x"}, 404)
}

func TestCommentFromGitHub(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	var show usecase.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	task, err := model.ParseULID(show.ULID)
	if err != nil {
		t.Fatal(err)
	}
	var out commentBody
	ts.call("POST", "/api/tasks/"+id+"/comments", map[string]any{"body": "mine"}, 201, &out)
	own := out.Comment.ULID
	mirrored, deleted := model.NewULID(), model.NewULID()
	ts.uc.MustEdit("me", "comment", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		now := time.Now()
		if err := tx.PutComment(task, &model.Comment{ULID: mirrored, Author: "github:octocat", Body: "from GitHub\n"}); err != nil {
			return err
		}
		return tx.PutComment(task, &model.Comment{ULID: deleted, Author: "github:octocat", DeletedAt: &now, Body: "gone\n"})
	})
	cid := mirrored.String()
	if p := ts.problem("PATCH", "/api/tasks/"+id+"/comments/"+cid, map[string]any{"body": "x"}, 422); p.Type != "comment.mirrored" {
		t.Errorf("edit: %+v", p)
	}
	if p := ts.problem("DELETE", "/api/tasks/"+id+"/comments/"+cid, nil, 422); p.Type != "comment.mirrored" {
		t.Errorf("remove: %+v", p)
	}
	// Deleted comes before mirrored.
	if p := ts.problem("PATCH", "/api/tasks/"+id+"/comments/"+deleted.String(), map[string]any{"body": "x"}, 409); p.Type != "comment.deleted" {
		t.Errorf("edit of a deleted one: %+v", p)
	}
	if p := ts.problem("DELETE", "/api/tasks/"+id+"/comments/"+deleted.String(), nil, 409); p.Type != "comment.deleted" {
		t.Errorf("remove of a deleted one: %+v", p)
	}
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	flags := map[string]bool{}
	for _, c := range show.Comments {
		flags[c.ULID] = c.GitHub
	}
	if len(flags) != 2 || !flags[cid] || flags[own] {
		t.Errorf("github flags = %v; want true for %s and false for %s", flags, cid, own)
	}
	ts.call("PATCH", "/api/tasks/"+id+"/comments/"+own, map[string]any{"body": "edited"}, 200, &out)
}

func TestWatchMe(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	var out taskWriteBody
	ts.call("PUT", "/api/tasks/"+id+"/watchers/me", nil, 200, &out)
	var show usecase.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if len(show.Watchers) != 1 {
		t.Fatalf("watchers = %v", show.Watchers)
	}
	ts.call("DELETE", "/api/tasks/"+id+"/watchers/me", nil, 200, &out)
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if len(show.Watchers) != 0 {
		t.Errorf("watchers = %v", show.Watchers)
	}
}

func TestWatchMeWithoutAccountIsConflict(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	ts.uc.DropDefaultAccount()
	if p := ts.problem("PUT", "/api/tasks/"+id+"/watchers/me", nil, 409); p.Type != "account.none" {
		t.Errorf("problem = %+v", p)
	}
}
