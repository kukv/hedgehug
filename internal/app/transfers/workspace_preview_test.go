package transfers_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const otherWS workspace.ID = "01TESTWS000000000000000010"

func storeNames(infos []port.StoreInfo) []string {
	var out []string
	for _, i := range infos {
		out = append(out, i.Name)
	}
	return out
}

// 候補は、表示中で閲覧のみでなく自分でない Store。0 件のとき、SyncTargets だけに非表示の Store があれば HiddenOnly。
func TestWorkspaceTargets(t *testing.T) {
	f := newFixture(t)
	hid := port.SyncTarget{ID: "S4", Name: "hid", Hidden: true}
	f.svc.SyncTargets = &apptest.SyncTargets{Targets: []port.SyncTarget{
		{ID: store1, Name: "main"}, {ID: store2, Name: "team"}, {ID: store3, Name: "readonly", ReadOnly: true}, hid,
	}}
	got, err := f.svc.WorkspaceTargets(t.Context(), store1, mobileWS)
	if err != nil || !slices.Equal(storeNames(got.Stores), []string{"team"}) || got.HiddenOnly {
		t.Errorf("targets = %+v, %v", got, err)
	}
	f.list.Stores = []port.StoreInfo{{ID: store1, Name: "main"}, {ID: store3, Name: "readonly", ReadOnly: true}}
	got, err = f.svc.WorkspaceTargets(t.Context(), store1, mobileWS)
	if err != nil || len(got.Stores) != 0 || !got.HiddenOnly {
		t.Errorf("hidden only = %+v, %v", got, err)
	}
	hid.ReadOnly = true
	f.svc.SyncTargets = &apptest.SyncTargets{Targets: []port.SyncTarget{{ID: store1, Name: "main"}, hid}}
	got, err = f.svc.WorkspaceTargets(t.Context(), store1, mobileWS)
	if err != nil || got.HiddenOnly {
		t.Errorf("hidden read-only = %+v, %v", got, err)
	}
	if _, err := f.svc.WorkspaceTargets(t.Context(), store1, "01TESTWS999999999999999999"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("missing workspace = %v", err)
	}
}

// 件数は B5c-4、足す人は移動先にいない人(オーナーなど)。綴りだけ違う人(TARO)は足さない。
func TestWorkspacePreview(t *testing.T) {
	f := newFixture(t)
	gone := mobileItem(t, f, "01TESTIT000000000000000010", 2, "")
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, gone.ID); err != nil {
		t.Fatal(err)
	}
	write(t, f.rec, store1, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(mobileProject, change.NewSet("owner", "taro", "goro")),
			tx.WriteItems().Append(mobileEpic, change.NewSet("assignee", nil, "TARO")),
		)
	})
	got, err := f.svc.WorkspacePreview(t.Context(), store1, mobileWS, store2)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "main" || got.Destination != "team" || got.Counts != (transfer.Counts{Projects: 1, Items: 1, Deleted: 1}) {
		t.Errorf("preview = %+v", got)
	}
	if !got.RemoteNotice || got.DestinationHasRemote {
		t.Errorf("remote = %+v", got)
	}
	if want := []member.Member{{Login: "goro", Name: "goro"}}; !slices.Equal(got.People, want) {
		t.Errorf("people = %+v, want %+v", got.People, want)
	}
}

func TestWorkspacePreviewRefuses(t *testing.T) {
	cases := []struct {
		name string
		ws   workspace.Workspace
		want string
	}{
		{"prefix", workspace.Workspace{ID: otherWS, Name: "Other", Prefix: "MB"}, "team: この Store には同じ接頭辞のワークスペースがあります"},
		{"name", workspace.Workspace{ID: otherWS, Name: "Mobile", Prefix: "XX"}, "team: この Store には同じ名前のワークスペースがあります"},
		{"both", workspace.Workspace{ID: otherWS, Name: "Mobile", Prefix: "MB"}, "team: この Store には同じ接頭辞のワークスペースがあります"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			addWorkspace(t, f.rec, store2, c.ws, "", "", nil)
			_, err := f.svc.WorkspacePreview(t.Context(), store1, mobileWS, store2)
			var r *refusal.Refusal
			if !errors.As(err, &r) || r.Message != c.want {
				t.Errorf("err = %v, want %q", err, c.want)
			}
		})
	}
	t.Run("deleted is ignored", func(t *testing.T) {
		f := newFixture(t)
		addWorkspace(t, f.rec, store2, workspace.Workspace{ID: otherWS, Name: "Mobile", Prefix: "MB"}, "", "", nil)
		if err := apptest.DeleteWorkspace(t.Context(), f.rec, store2, otherWS); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.WorkspacePreview(t.Context(), store1, mobileWS, store2); err != nil {
			t.Errorf("err = %v", err)
		}
	})
}

func TestWorkspacePreviewNotFound(t *testing.T) {
	f := newFixture(t)
	for name, c := range map[string]struct {
		ws workspace.ID
		to port.StoreID
	}{
		"missing":   {"01TESTWS999999999999999999", store2},
		"read only": {mobileWS, store3},
		"self":      {mobileWS, store1},
	} {
		if _, err := f.svc.WorkspacePreview(t.Context(), store1, c.ws, c.to); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := f.svc.WorkspacePreview(t.Context(), store1, mobileWS, "S9"); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("hidden destination = %v", err)
	}
	if err := apptest.DeleteWorkspace(t.Context(), f.rec, store1, mobileWS); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.WorkspacePreview(t.Context(), store1, mobileWS, store2); err != nil {
		t.Errorf("deleted elsewhere: err = %v", err)
	}
}
