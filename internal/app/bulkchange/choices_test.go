package bulkchange

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	s1 port.StoreID = "S1"
	s2 port.StoreID = "S2"
	s3 port.StoreID = "S3"
)

// setup は、S1(Web と Mobile)・S2(Web)・S3(Web。閲覧のみ)の 3 つの Store を持つ Service を返す。
func setup(t *testing.T) (Service, *apptest.Stores) {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	for _, id := range []port.StoreID{s1, s2, s3} {
		s.AddStore(id)
		if err := apptest.Sample(t.Context(), s, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := apptest.SampleMobile(t.Context(), s, s1); err != nil {
		t.Fatal(err)
	}
	list := &apptest.StoreList{Stores: []port.StoreInfo{{ID: s1, Name: "main"}, {ID: s2, Name: "team"}, {ID: s3, Name: "readonly", ReadOnly: true}}}
	return Service{Stores: s, StoreList: list, Identity: apptest.Identity{Login: "hanako"}, Clock: clock}, s
}

// writeWs は、Store st のワークスペース ws のログに行 ds を書く。
func writeWs(t *testing.T, s port.Stores, st port.StoreID, ws workspace.ID, ds ...change.Draft) {
	t.Helper()
	op := port.Operation{Actor: change.User("hanako"), Summary: "テストの行"}
	err := s.Write(t.Context(), st, op, func(tx port.Tx) error { return tx.WriteWorkspaces().Append(ws, ds...) })
	if err != nil {
		t.Fatal(err)
	}
}

func settingsOf(t *testing.T, s port.Stores, st port.StoreID, ws workspace.ID) workspace.Settings {
	t.Helper()
	var out workspace.Settings
	err := s.Read(t.Context(), st, func(r port.Reader) (err error) {
		out, err = r.Workspaces().Settings(ws)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestChoicesMerge(t *testing.T) {
	svc, s := setup(t)
	// Mobile の既定のステータスは Web と同じ名前なので、1 つを Mobile にしかない名前にする。
	l := workspace.List{Kind: workspace.ListStatuses}
	mobileDone := settingsOf(t, s, s1, apptest.SampleMobileWorkspace).Statuses[2]
	writeWs(t, s, s1, apptest.SampleMobileWorkspace, change.NewSet(l.Field(mobileDone.ID, "name"), mobileDone.Name, "Released"))
	got, err := svc.Choices(t.Context(), []Target{{s1, apptest.SampleTask}, {s1, apptest.SampleMobileEpic}})
	if err != nil {
		t.Fatal(err)
	}
	web := settingsOf(t, s, s1, apptest.SampleWorkspace)
	mobile := settingsOf(t, s, s1, apptest.SampleMobileWorkspace)
	var want []string
	for _, v := range append(slices.Clone(web.Statuses), mobile.Statuses...) {
		if !slices.ContainsFunc(want, func(n string) bool { return sameName(n, v.Name) }) {
			want = append(want, v.Name)
		}
	}
	if !slices.Equal(got.Statuses, want) || len(want) <= len(web.Statuses) {
		t.Errorf("Statuses = %q, want %q (Web only has %d)", got.Statuses, want, len(web.Statuses))
	}
	if !slices.Equal(got.Kinds, []string{"Epic", "Task"}) {
		t.Errorf("Kinds = %q", got.Kinds)
	}
	// メンバーの一覧の順はログイン名の小文字の順(member.FromLog)。
	if !slices.Equal(got.Members, []Member{{"hanako", "hanako"}, {"taro", "taro"}}) {
		t.Errorf("Members = %+v", got.Members)
	}
}

func TestChoicesIgnoreCase(t *testing.T) {
	svc, s := setup(t)
	l := workspace.List{Kind: workspace.ListStatuses}
	writeWs(t, s, s2, apptest.SampleWorkspace, change.NewSet(l.Field(apptest.SampleStatusDone, "name"), "Done", "done"))
	got, err := svc.Choices(t.Context(), []Target{{s1, apptest.SampleTask}, {s2, apptest.SampleTask}})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, name := range got.Statuses {
		if sameName(name, "Done") {
			n++
			if name != "Done" {
				t.Errorf("name = %q, want the first seen spelling", name)
			}
		}
	}
	if n != 1 {
		t.Errorf("Done appears %d times in %q", n, got.Statuses)
	}
}

func TestChoicesSkipReadOnlyAndUnused(t *testing.T) {
	svc, s := setup(t)
	got, err := svc.Choices(t.Context(), []Target{{s3, apptest.SampleTask}})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(got.Kinds) + len(got.Statuses) + len(got.Priorities) + len(got.Sprints) + len(got.Releases) + len(got.Systems) + len(got.Members); n != 0 {
		t.Errorf("read-only item gave choices: %+v", got)
	}
	writeWs(t, s, s1, apptest.SampleWorkspace, change.NewAdd(projectrule.HiddenField(apptest.SampleProject), string(apptest.SampleStatusPending)))
	got, err = svc.Choices(t.Context(), []Target{{s1, apptest.SampleTask}})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(got.Statuses, "Pending") || !slices.Contains(got.Statuses, "Done") {
		t.Errorf("Statuses = %q", got.Statuses)
	}
}

func TestChoicesSprints(t *testing.T) {
	svc, s := setup(t)
	var ds []change.Draft
	ds = append(ds, planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: "01SPRINT0000000000000000A", Name: "S-1", State: planning.Planned})...)
	ds = append(ds, planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: "01SPRINT0000000000000000B", Name: "S-0", State: planning.Closed})...)
	ds = append(ds, planning.ReleaseDrafts(planning.Release{}, planning.Release{ID: "01RELEASE000000000000000A", Name: "R-0", State: planning.Archived})...)
	ds = append(ds, planning.ReleaseDrafts(planning.Release{}, planning.Release{ID: "01RELEASE000000000000000B", Name: "R-1", State: planning.Unreleased})...)
	writeWs(t, s, s1, apptest.SampleWorkspace, ds...)
	got, err := svc.Choices(t.Context(), []Target{{s1, apptest.SampleTask}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Sprints, []string{"S-1"}) || !slices.Equal(got.Releases, []string{"R-1"}) {
		t.Errorf("Sprints = %q, Releases = %q", got.Sprints, got.Releases)
	}
}

func TestChoicesUnknownStore(t *testing.T) {
	svc, _ := setup(t)
	_, err := svc.Choices(t.Context(), []Target{{s1, apptest.SampleTask}, {"S9", apptest.SampleTask}})
	if !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestGroupsFirstSeenOrder(t *testing.T) {
	svc, _ := setup(t)
	got, err := svc.groups(t.Context(), []Target{{s2, apptest.SampleTask}, {s3, apptest.SampleTask}, {s2, apptest.SampleEpic}, {s2, apptest.SampleTask}})
	if err != nil || len(got) != 2 {
		t.Fatalf("groups = %+v, %v", got, err)
	}
	if got[0].id != s2 || got[0].name != "team" || got[0].readOnly || len(got[0].targets) != 2 {
		t.Errorf("group 0 = %+v", got[0])
	}
	if got[1].id != s3 || !got[1].readOnly {
		t.Errorf("group 1 = %+v", got[1])
	}
}

func TestChoicesSkipDeletedWorkspace(t *testing.T) {
	svc, s := setup(t)
	writeWs(t, s, s1, apptest.SampleMobileWorkspace, change.NewDeleteEntity(true))
	got, err := svc.Choices(t.Context(), []Target{{s1, apptest.SampleMobileEpic}, {s1, apptest.SampleTask}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Kinds, []string{"Task"}) || len(got.Statuses) == 0 {
		t.Errorf("Kinds = %q, Statuses = %q", got.Kinds, got.Statuses)
	}
}
