package transfers_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

const goneProject project.ID = "01TESTPR000000000000000030" // どの Store にもないプロジェクト

// putProjectRecord は、Store store にプロジェクトの転送記録を直接足す。
func putProjectRecord(t *testing.T, f fixture, store port.StoreID, target project.ID, old, newID string, to port.StoreID, toName string) {
	t.Helper()
	write(t, f.rec, store, func(tx port.Tx) error {
		return tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityProject, Target: target, Old: old, New: newID, ToStore: string(to), ToStoreName: toName})
	})
}

func locateProject(t *testing.T, f fixture, store, display string) transfers.LocatedProject {
	t.Helper()
	got, err := f.svc.LocateProject(t.Context(), store, display)
	if err != nil {
		t.Fatalf("LocateProject(%q, %q) = %v", store, display, err)
	}
	return got
}

// projectRecord は、Store store のプロジェクト id の転送記録のうち、i 番目(書いた順)。
func projectRecord(t *testing.T, f fixture, store port.StoreID, id project.ID, i int) transfer.Record {
	t.Helper()
	var out []transfer.Record
	for _, r := range recordsOf(t, f.rec, store) {
		if r.Entity == transfer.EntityProject && r.Target == id {
			out = append(out, r)
		}
	}
	if len(out) <= i {
		t.Fatalf("%s のプロジェクトの記録 %d 件目がない: %+v", store, i, out)
	}
	return out[i]
}

func setWebPrefix(t *testing.T, f fixture, from, to string) {
	t.Helper()
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("prefix", from, to))
	})
}

// A5b-17: いまの ID と旧い接頭辞の ID(HH → WEB)で開く。どちらも MovedFrom はない。
func TestLocateProjectCurrentAndOldPrefix(t *testing.T) {
	f := newFixture(t)
	if got := locateProject(t, f, "main", "HH-P1"); !got.Found || got.Store != store1 || got.Project != apptest.SampleProject || got.MovedFrom != nil || got.Gone != nil {
		t.Errorf("いまの ID: %+v", got)
	}
	if got := locateProject(t, f, "team", "TM-P1"); got.Store != store2 || got.Project != teamProject {
		t.Errorf("ほかの Store のいまの ID: %+v", got)
	}
	setWebPrefix(t, f, "HH", "WEB")
	for _, id := range []string{"HH-P1", "WEB-P1"} {
		if got := locateProject(t, f, "main", id); !got.Found || got.Project != apptest.SampleProject || got.MovedFrom != nil {
			t.Errorf("%s: %+v", id, got)
		}
	}
}

// A5b-17: 同じ Store で HH-P1 → MB-P2 と移したあと、HH-P1 は MB-P2 を開く。さらに別の Store(team)へ移すと、HH-P1 は
// team のプロジェクトを開き、Display は最初に一致した記録の新 ID(MB-P2)、Current はいまの ID。
func TestLocateProjectAfterMove(t *testing.T) {
	f := newFixture(t)
	moveProject(t, f, apptest.SampleProject, projectTo(store1, mobileWS))
	got := locateProject(t, f, "main", "HH-P1")
	want := transfers.MovedFrom{Old: "HH-P1", At: projectRecord(t, f, store1, apptest.SampleProject, 0).At, Display: "MB-P2", Current: "MB-P2"}
	if !got.Found || got.Store != store1 || got.Project != apptest.SampleProject || got.MovedFrom == nil || *got.MovedFrom != want {
		t.Errorf("同じ Store: %+v, %+v", got, got.MovedFrom)
	}
	moved := moveProject(t, f, apptest.SampleProject, toTeamWS())
	got = locateProject(t, f, "main", "HH-P1")
	want.Current = moved.Display
	if !got.Found || got.Store != store2 || got.Project != apptest.SampleProject || got.MovedFrom == nil || *got.MovedFrom != want {
		t.Errorf("別の Store(%s): %+v, %+v", moved.Display, got, got.MovedFrom)
	}
	second := projectRecord(t, f, store1, apptest.SampleProject, 1)
	if got := locateProject(t, f, "main", "MB-P2"); got.Store != store2 || got.MovedFrom == nil ||
		*got.MovedFrom != (transfers.MovedFrom{Old: "MB-P2", At: second.At, Display: moved.Display, Current: moved.Display}) {
		t.Errorf("MB-P2: %+v", got.MovedFrom)
	}
}

// 接頭辞を変えた(HH → WEB)あとに移したプロジェクト(記録の旧 ID は WEB-P1)は、変える前の ID(HH-P1)からも辿れる。
func TestLocateProjectFollowsARecordWrittenAfterThePrefixChanged(t *testing.T) {
	f := newFixture(t)
	setWebPrefix(t, f, "HH", "WEB")
	moveProject(t, f, apptest.SampleProject, projectTo(store1, mobileWS))
	for _, id := range []string{"HH-P1", "WEB-P1"} {
		got := locateProject(t, f, "main", id)
		if !got.Found || got.Project != apptest.SampleProject || got.MovedFrom == nil || got.MovedFrom.Old != "WEB-P1" || got.MovedFrom.Display != "MB-P2" {
			t.Errorf("%s: %+v", id, got)
		}
	}
}

// A5b-17: 見つからないときの理由は A5a-15 と同じ 4 つ。日時は、旧 ID が一致した記録(main の HH-P1 の記録)の日時。
func TestLocateProjectGone(t *testing.T) {
	nineAM := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	toMobileWS := func(t *testing.T, f fixture) { moveProject(t, f, apptest.SampleProject, projectTo(store1, mobileWS)) }
	cases := []struct {
		name  string
		setup func(t *testing.T, f fixture)
		want  transfers.Gone
	}{
		{"移したあと削除", func(t *testing.T, f fixture) {
			toMobileWS(t, f)
			if err := apptest.DeleteProject(t.Context(), f.rec, store1, apptest.SampleProject); err != nil {
				t.Fatal(err)
			}
		}, transfers.Gone{Reason: "deleted", Old: "HH-P1", At: nineAM, NewDisplay: "MB-P2"}},
		{"移したあとワークスペースを削除", func(t *testing.T, f fixture) {
			toMobileWS(t, f)
			if err := apptest.DeleteWorkspace(t.Context(), f.rec, store1, mobileWS); err != nil {
				t.Fatal(err)
			}
		}, transfers.Gone{Reason: "deleted", Old: "HH-P1", At: nineAM, NewDisplay: "MB-P2"}},
		{"完全に削除", func(t *testing.T, f fixture) {
			if err := apptest.DeleteProject(t.Context(), f.rec, store1, apptest.SampleProject); err != nil { // HH-P1 を空ける
				t.Fatal(err)
			}
			putProjectRecord(t, f, store1, goneProject, "HH-P1", "TM-P5", store2, "team")
			f.rec.AddPurged(store2, port.Purged{Entity: "project", Target: goneProject, Display: "TM-P5", At: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)})
		}, transfers.Gone{Reason: "purged", Old: "HH-P1", At: nineAM, NewDisplay: "TM-P5"}},
		{"移動先の Store が非表示", func(t *testing.T, f fixture) {
			moveProject(t, f, apptest.SampleProject, toTeamWS())
			f.list.Stores = []port.StoreInfo{f.list.Stores[0], f.list.Stores[2]}
		}, transfers.Gone{Reason: "hidden", Old: "HH-P1", At: nineAM.AddDate(0, 0, 1), NewDisplay: "TM-P2", StoreName: "team"}}, // 別の Store への移動の記録は Service の時計
		{"そのあと非表示の Store へ移した", func(t *testing.T, f fixture) {
			toMobileWS(t, f)
			moveProject(t, f, apptest.SampleProject, toTeamWS())
			f.list.Stores = []port.StoreInfo{f.list.Stores[0], f.list.Stores[2]}
		}, transfers.Gone{Reason: "elsewhere", Old: "HH-P1", At: nineAM, NewDisplay: "MB-P2", StoreName: "team"}},
	}
	for _, c := range cases {
		f := newFixture(t)
		c.setup(t, f)
		got, err := f.svc.LocateProject(t.Context(), "main", "HH-P1")
		at := projectRecord(t, f, store1, func() project.ID {
			if c.name == "完全に削除" {
				return goneProject
			}
			return apptest.SampleProject
		}(), 0).At
		checkGone(t, c.name, transfers.Located{Found: got.Found, Gone: got.Gone}, err, c.want, at)
	}
}

// 削除済みのプロジェクト・削除済みのワークスペースのプロジェクトは、いまの ID でも開かない。
func TestLocateProjectSkipsDeletedProjectsAndWorkspaces(t *testing.T) {
	f := newFixture(t)
	if err := apptest.DeleteWorkspace(t.Context(), f.rec, store1, mobileWS); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteProject(t.Context(), f.rec, store1, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"MB-P1", "HH-P1"} {
		if got, err := f.svc.LocateProject(t.Context(), "main", id); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %+v, %v", id, got, err)
		}
	}
}

// プロジェクトはプロジェクトの記録(entity: project)だけを見る。アイテムの記録の旧 ID に同じ文字列があっても使わない。
// ID の形が違うもの(アイテムの形の HH-12・プロジェクトの形を Locate に渡したもの)は見つからない。
func TestLocateProjectIgnoresItemRecords(t *testing.T) {
	f := newFixture(t)
	putRecord(t, f, store1, roamer, "HH-P9", "XX-P9", "S9", "elsewhere") // 使えば hidden になる
	putProjectRecord(t, f, store1, goneProject, "HH-9", "XX-9", "S9", "elsewhere")
	if got, err := f.svc.LocateProject(t.Context(), "main", "HH-P9"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("アイテムの記録: %+v, %v", got, err)
	}
	if got, err := f.svc.Locate(t.Context(), "main", "HH-9"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("プロジェクトの記録を Locate: %+v, %v", got, err)
	}
	for _, id := range []string{"HH-12", "HH-1", "HH-P", "HH-Px", "HH-P+1", "-P1", "P1", "HH-p1"} {
		if got, err := f.svc.LocateProject(t.Context(), "main", id); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("LocateProject(%q): %+v, %v", id, got, err)
		}
	}
	if got, err := f.svc.Locate(t.Context(), "main", "HH-P1"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Locate(HH-P1): %+v, %v", got, err)
	}
}
