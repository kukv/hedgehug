package githublink

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/prlink"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	findStore port.StoreID = "S1"
	apiWS     workspace.ID = "01APIWS0000000000000000001"
	apiProj   project.ID   = "01APIPR0000000000000000001"
	apiItem   item.ID      = "01APIIT0000000000000000001"
)

// findStores は、ワークスペース HH(HH-1・HH-2。サンプル)と API(API-1)を持つ Store。
func findStores(t *testing.T) port.Stores {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore(findStore)
	if err := apptest.Sample(t.Context(), s, findStore); err != nil {
		t.Fatal(err)
	}
	ws := workspace.Workspace{ID: apiWS, Name: "API", Prefix: "API"}
	pr := project.Project{ID: apiProj, Place: project.Place{Workspace: ws.ID, Number: 1}, Rank: "V", Title: "api", Owner: "taro", Status: project.Todo}
	it := item.Item{ID: apiItem, Place: item.Place{Workspace: ws.ID, Project: pr.ID, Number: 1}, Kind: item.Epic, Rank: rank.First, Title: "api", Status: apptest.SampleStatusTodo, Priority: apptest.SamplePriorityMedium}
	n := 0
	write(t, s, func(tx port.Tx) error {
		d, err := workspace.CreateWithSettings(ws, workspace.DefaultSettings(func() change.ID {
			n++
			return change.ID(fmt.Sprintf("01APIVAL%018d", n))
		}))
		if err != nil {
			return err
		}
		if err := tx.WriteWorkspaces().Append(ws.ID, d, numbering.Draft(numbering.Projects, 1), numbering.Draft(numbering.Items, 1)); err != nil {
			return err
		}
		if err := tx.WriteProjects().Append(pr.ID, project.Create(pr)); err != nil {
			return err
		}
		return tx.WriteItems().Append(it.ID, item.Create(it))
	})
	return s
}

func write(t *testing.T, s port.Stores, fn func(tx port.Tx) error) {
	t.Helper()
	if err := s.Write(t.Context(), findStore, port.Operation{Actor: change.User("taro"), Summary: "テスト"}, fn); err != nil {
		t.Fatal(err)
	}
}

func targetsOf(t *testing.T, s port.Stores, p port.Pull) []item.ID {
	t.Helper()
	var got []item.ID
	err := s.Read(t.Context(), findStore, func(r port.Reader) error {
		si, err := readStoreItems(r)
		got = si.targets(p)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func pullOf(x prlink.Text) port.Pull { return port.Pull{Text: x} }

func TestStoreItems(t *testing.T) {
	s := findStores(t)
	check := func(name string, p port.Pull, want ...item.ID) {
		t.Helper()
		if got := targetsOf(t, s, p); !slices.Equal(got, want) {
			t.Errorf("%s: targets = %v, want %v", name, got, want)
		}
	}
	check("branch and title", pullOf(prlink.Text{Branch: "hh/HH-1", Title: "API-1"}), apptest.SampleEpic, apiItem)
	check("mark in body", pullOf(prlink.Text{Body: "hedgehug:" + string(apptest.SampleTask)}), apptest.SampleTask)
	check("unknown mark", pullOf(prlink.Text{Body: "hedgehug:01ZZZZZZZZZZZZZZZZZZZZZZZZ"}))
	check("unknown number", pullOf(prlink.Text{Title: "HH-9"}))
	check("same item by ID and mark", pullOf(prlink.Text{Title: "HH-1", Comments: []string{"hedgehug:" + string(apptest.SampleEpic)}}), apptest.SampleEpic)
}

func TestStoreItemsUseCurrentPrefixOnly(t *testing.T) {
	s := findStores(t)
	write(t, s, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("prefix", "HH", "HHX"))
	})
	if got := targetsOf(t, s, pullOf(prlink.Text{Title: "HH-1"})); len(got) != 0 {
		t.Errorf("old prefix: targets = %v, want none", got)
	}
	if got := targetsOf(t, s, pullOf(prlink.Text{Title: "HHX-1"})); !slices.Equal(got, []item.ID{apptest.SampleEpic}) {
		t.Errorf("new prefix: targets = %v", got)
	}
	// 印は接頭辞によらないので、接頭辞を変えたあとも引ける。
	if got := targetsOf(t, s, pullOf(prlink.Text{Body: "hh:" + string(apptest.SampleTask)})); !slices.Equal(got, []item.ID{apptest.SampleTask}) {
		t.Errorf("mark after prefix change: targets = %v", got)
	}
}

func TestStoreItemsSkipDeleted(t *testing.T) {
	s := findStores(t)
	if err := apptest.DeleteItems(t.Context(), s, findStore, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteProject(t.Context(), s, findStore, apiProj); err != nil {
		t.Fatal(err)
	}
	p := pullOf(prlink.Text{Title: "HH-2 API-1 HH-1", Comments: []string{"hedgehug:" + string(apptest.SampleTask) + " hedgehug:" + string(apiItem)}})
	if got := targetsOf(t, s, p); !slices.Equal(got, []item.ID{apptest.SampleEpic}) {
		t.Errorf("targets = %v, want only the live item", got)
	}
}
