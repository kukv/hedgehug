package workspace_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestValidPrefix(t *testing.T) {
	for s, want := range map[string]bool{
		"HH": true, "WEB2": true, "A": true, "ABCDEFGHIJ": true,
		"": false, "hh": false, "2WEB": false, "ABCDEFGHIJK": false, "W-B": false,
	} {
		if workspace.ValidPrefix(s) != want {
			t.Errorf("ValidPrefix(%q) != %v", s, want)
		}
	}
}

func TestCreateAndFromLog(t *testing.T) {
	if _, err := workspace.Create(workspace.Workspace{ID: "W", Name: "Web", Prefix: "hh"}); err == nil {
		t.Error("bad prefix accepted")
	}
	if _, err := workspace.Create(workspace.Workspace{ID: "W", Name: "", Prefix: "HH"}); err == nil {
		t.Error("empty name accepted")
	}
	d, err := workspace.Create(workspace.Workspace{ID: "W", Name: "Web", Prefix: "HH"})
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	meta := change.Meta{Op: "o", At: ts, Actor: change.User("taro")}
	log := change.Stamp(nil, []change.Draft{d}, meta, func() change.ID { return "c" })
	// GitHub 上で手で書き換えた不正な接頭辞の行は無いものとし、前の値に戻る(保存形式の手順 0)。
	meta.At = ts.Add(time.Second)
	log = append(log, change.Stamp(log, []change.Draft{change.NewSet("prefix", "HH", "h h")}, meta, func() change.ID { return "s" })...)
	w, ok := workspace.FromLog(log)
	if !ok || w != (workspace.Workspace{ID: "W", Name: "Web", Prefix: "HH"}) {
		t.Errorf("FromLog = %+v, %v", w, ok)
	}
	if _, ok := workspace.FromLog(nil); ok {
		t.Error("empty log gave a workspace")
	}
}

// 手で書き換えた不正な接頭辞の作成の行は無いものとし、ワークスペースは無い。
func TestFromLogIgnoresInvalidCreate(t *testing.T) {
	d := change.NewCreate(map[string]any{"id": "W", "name": "Web", "prefix": "h h"})
	meta := change.Meta{Op: "o", At: time.Unix(0, 0), Actor: change.User("taro")}
	log := change.Stamp(nil, []change.Draft{d}, meta, func() change.ID { return "c" })
	if w, ok := workspace.FromLog(log); ok {
		t.Errorf("FromLog = %+v, want none", w)
	}
}

func TestFromLogMarksDeleted(t *testing.T) {
	d, err := workspace.Create(workspace.Workspace{ID: "W", Name: "Web", Prefix: "HH"})
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	stamp := func(log []change.Record, op change.ID, at time.Time, d change.Draft) []change.Record {
		meta := change.Meta{Op: op, At: at, Actor: change.User("taro")}
		return append(log, change.Stamp(log, []change.Draft{d}, meta, func() change.ID { return "c" + op })...)
	}
	log := stamp(nil, "o0", ts, d)
	if w, ok := workspace.FromLog(log); !ok || w.Deleted != nil {
		t.Fatalf("a live workspace has Deleted = %+v", w.Deleted)
	}
	log = stamp(log, "o1", ts.Add(time.Second), change.NewDeleteEntity(true))
	w, ok := workspace.FromLog(log)
	if !ok || w.Deleted == nil || w.Deleted.Op != "o1" || !w.Deleted.Root {
		t.Fatalf("FromLog after delete = %+v, %v", w.Deleted, ok)
	}
	log = stamp(log, "o2", ts.Add(2*time.Second), change.NewRestore(change.DeletedField, "o1"))
	if w, ok := workspace.FromLog(log); !ok || w.Deleted != nil {
		t.Errorf("FromLog after restore has Deleted = %+v", w.Deleted)
	}
}

func TestLiveSkipsDeleted(t *testing.T) {
	ws := []workspace.Workspace{{ID: "A"}, {ID: "B", Deleted: &change.Deletion{Op: "o1", Root: true}}, {ID: "C"}}
	got := workspace.Live(ws)
	if len(got) != 2 || got[0].ID != "A" || got[1].ID != "C" {
		t.Errorf("Live = %+v", got)
	}
}
