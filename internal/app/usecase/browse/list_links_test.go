package browse_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestListShowsTheLinks(t *testing.T) {
	t.Parallel()
	const pr, off = "github.com/o/r/pull/1", "github.com/o/r/pull/2"
	ts := usecasetest.New(t)
	ts.AddTask("a", func(x *model.Task) {
		x.Links = []model.Link{
			{Type: model.LinkPR, Ref: pr, Sync: model.SyncMirror, Via: model.LinkViaBranch},
			{Type: model.LinkPR, Ref: off, Sync: model.SyncOff},
		}
		if err := x.SetGitHub(model.GitHubState{PRs: []model.PRState{{Ref: pr, Title: "the pull", Branch: "hh/ME-1", State: "open", Review: "none"}}}); err != nil {
			t.Fatal(err)
		}
	})
	l, err := browse.List(ts.Session, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Tasks) != 1 || len(l.Tasks[0].Links) != 1 {
		t.Fatalf("tasks = %+v", l.Tasks)
	}
	v := l.Tasks[0].Links[0]
	if v.Label != "o/r#1" || v.Title != "the pull" || v.Branch != "hh/ME-1" || v.State != "open" || v.Via != model.LinkViaBranch {
		t.Errorf("link = %+v", v)
	}
}
