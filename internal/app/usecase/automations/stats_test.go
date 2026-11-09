package automations_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/automations"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func fetch(t *testing.T, ts *usecasetest.Session) {
	t.Helper()
	c, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := usecase.FetchGitHub(ts.Session, c, e); err != nil {
		t.Fatal(err)
	}
}

func TestStatsCountTheLastSevenDays(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	sc, f := schema.Dev(), automation.DevRules()
	sc.Workflow.Required = schema.Lists{{Key: "in_progress", Values: []string{"estimate"}}}
	b, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		if err := tx.PutSchema(sc); err != nil {
			return err
		}
		return tx.PutFile(model.AutomationsPath, b)
	})
	const web = "github.com/org-a/web"
	refused := ts.AddTask("refused", func(x *model.Task) { x.Repos = []string{web} })
	ts.AddTask("applied", func(x *model.Task) {
		x.Repos = []string{web}
		d, _ := model.ParseDuration("2h")
		x.Estimate = &d
	})
	for _, branch := range []string{"hh/ME-1-a", "hh/ME-3-a"} {
		if _, err := fake.OpenPull("org-a/web", branch, "A", "", false); err != nil {
			t.Fatal(err)
		}
	}
	fetch(t, ts)
	stats, err := automations.Stats(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	x := stats[0]
	if x.Rule != "pr-opened" || x.Applied != 1 || x.Rejected != 1 {
		t.Errorf("stat = %+v", x)
	}
	if len(x.RejectedTasks) != 1 || x.RejectedTasks[0].ULID != refused.ULID.String() || !strings.Contains(x.LastReason, "estimate") {
		t.Errorf("stat = %+v", x)
	}
	week := ts.Now().Add(8 * 24 * time.Hour)
	ts.Now = func() time.Time { return week }
	if stats, err := automations.Stats(ts.Session); err != nil || len(stats) != 0 {
		t.Errorf("a week later = %+v, %v", stats, err)
	}
}
