package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/lib/github"
)

const repo = "github.com/org-a/web"

func prRef(n int) string {
	return model.GitHubRef{Repo: repo, Kind: model.LinkPR, Number: n}.String()
}

func TestApplyWritesWhatChanged(t *testing.T) {
	t.Parallel()
	x := &model.Task{Links: []model.Link{{Type: model.LinkPR, Ref: prRef(1), Sync: model.SyncMirror}}}
	got := fetched{pulls: map[string]github.Pull{prRef(1): {State: "open", Review: "pending", Checks: "success"}}}
	a, err := apply(x, got, nil)
	if err != nil || !a.Changed {
		t.Fatalf("apply = %v, %v", a.Changed, err)
	}
	g, _ := x.GitHub()
	if len(g.PRs) != 1 || g.PRs[0].Ref != prRef(1) || g.PRs[0].Review != "pending" {
		t.Fatalf("prs = %+v", g.PRs)
	}
	if a, err := apply(x, got, nil); err != nil || a.Changed {
		t.Errorf("the same values: %v, %v", a.Changed, err)
	}
}

func TestApplyKeepsWhatWasNotRead(t *testing.T) {
	t.Parallel()
	x := &model.Task{Links: []model.Link{{Type: model.LinkPR, Ref: prRef(1), Sync: model.SyncMirror}}}
	if err := x.SetGitHub(model.GitHubState{PRs: []model.PRState{{Ref: prRef(1), State: "open", Review: "none"}}}); err != nil {
		t.Fatal(err)
	}
	if a, err := apply(x, fetched{}, nil); err != nil || a.Changed {
		t.Fatalf("apply = %v, %v", a.Changed, err)
	}
}

func TestApplyDropsWhatIsNoLongerLinked(t *testing.T) {
	t.Parallel()
	x := &model.Task{Links: []model.Link{{Type: model.LinkPR, Ref: prRef(1), Sync: model.SyncOff}}}
	if err := x.SetGitHub(model.GitHubState{PRs: []model.PRState{{Ref: prRef(1), State: "open", Review: "none"}}}); err != nil {
		t.Fatal(err)
	}
	a, err := apply(x, fetched{}, nil)
	g, _ := x.GitHub()
	if err != nil || !a.Changed || len(g.PRs) != 0 {
		t.Fatalf("apply = %v, %v, %+v", a.Changed, err, g)
	}
}

func TestApplyLinksAutomatically(t *testing.T) {
	t.Parallel()
	x := &model.Task{Links: []model.Link{{Type: model.LinkPR, Ref: prRef(2), Sync: model.SyncOff}}}
	got := fetched{pulls: map[string]github.Pull{prRef(1): {State: "open", Review: "none"}, prRef(2): {State: "open", Review: "none"}}}
	if _, err := apply(x, got, []string{prRef(1), prRef(2)}); err != nil {
		t.Fatal(err)
	}
	if len(x.Links) != 2 || x.Links[1].Ref != prRef(1) || x.Links[1].Sync != model.SyncMirror || x.Links[0].Sync != model.SyncOff {
		t.Fatalf("links = %+v", x.Links)
	}
}

func TestApplyIssueKeepsSynced(t *testing.T) {
	t.Parallel()
	ref := model.GitHubRef{Repo: repo, Kind: model.LinkIssue, Number: 5}.String()
	d, err := model.DecodeTask([]byte("---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nlinks:\n  - {type: github_issue, ref: " + ref + ", sync: link}\ngh:\n  issues:\n    - ref: " + ref + "\n      state: open\n      state_reason: null\n      synced: {title: a}\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := fetched{issues: map[string]github.Issue{ref: {State: "closed", StateReason: "completed", UpdatedAt: time.Now()}}}
	if a, err := apply(d, got, nil); err != nil || !a.Changed {
		t.Fatalf("apply = %v, %v", a.Changed, err)
	}
	g, _ := d.GitHub()
	if g.Issues[0].State != "closed" || *g.Issues[0].StateReason != "completed" || g.Issues[0].Synced == nil {
		t.Fatalf("issues = %+v", g.Issues)
	}
}

func TestTasksOfPull(t *testing.T) {
	t.Parallel()
	const u = "01JBZ8K3QW7M2X9V4T6R5N8P0A"
	for name, c := range map[string]struct {
		p    github.Pull
		want []string
	}{
		"branch":     {github.Pull{Head: "hh/WEB-42-login"}, []string{"WEB-42"}},
		"long":       {github.Pull{Head: "hedgehug/WEB-42"}, []string{"WEB-42"}},
		"not ours":   {github.Pull{Head: "feature/WEB-42"}, nil},
		"mark":       {github.Pull{Body: "hedgehug:" + u}, []string{u}},
		"short mark": {github.Pull{Title: "x hh:" + u}, []string{u}},
		"id":         {github.Pull{Comments: []string{"fixes WEB-7 and OPS-1"}}, []string{"WEB-7"}},
		"lower":      {github.Pull{Body: "web-7"}, nil},
		"inside":     {github.Pull{Body: "xhh:" + u + " my_hedgehug:" + u + " " + u}, nil},
		"too long":   {github.Pull{Body: "hh:" + u + "X"}, nil},
		"id inside":  {github.Pull{Body: "PRE-WEB-7 WEB-7x XWEB-7 WEB-77"}, []string{"WEB-77"}},
		"id twice":   {github.Pull{Title: "WEB-7", Body: "WEB-7, (WEB-8)"}, []string{"WEB-7", "WEB-8"}},
		"prefix":     {github.Pull{Head: "hh/WEB-42x"}, nil},
		"id range":   {github.Pull{Body: "WEB-1-2 WEB-3"}, []string{"WEB-3"}},
	} {
		got := mentioned(c.p, []string{"WEB"})
		if len(got) != len(c.want) {
			t.Errorf("%s = %v", name, got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s = %v", name, got)
			}
		}
	}
}

func TestApplyMatchesRefsInAnyCase(t *testing.T) {
	t.Parallel()
	x := &model.Task{Links: []model.Link{{Type: model.LinkPR, Ref: prRef(1), Sync: model.SyncMirror}}}
	got := fetched{pulls: map[string]github.Pull{prRef(1): {State: "open", Review: "none"}}}
	if _, err := apply(x, got, []string{strings.ToUpper(prRef(1))}); err != nil {
		t.Fatal(err)
	}
	if len(x.Links) != 1 {
		t.Fatalf("links = %+v", x.Links)
	}
	if err := x.SetGitHub(model.GitHubState{PRs: []model.PRState{{Ref: strings.ToUpper(prRef(1)), State: "open", Review: "none"}}}); err != nil {
		t.Fatal(err)
	}
	if a, err := apply(x, got, nil); err != nil || a.Changed {
		t.Errorf("a ref in another case: %v, %v", a.Changed, err)
	}
}
