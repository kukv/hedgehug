package transfers_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	reviewStatus  change.ID = "VALMBREVIEW"
	blockedStatus change.ID = "VALMBBLOCKED"
)

// withReviewAndBlocked は、Mobile のステータスの最後に Blocked(未着手)・Review(進行中)の順で足す。main の Web にはない。
func withReviewAndBlocked(st *workspace.Settings) {
	last := st.Statuses[len(st.Statuses)-1].Rank
	st.Statuses = append(st.Statuses,
		workspace.Value{ID: blockedStatus, Name: "Blocked", Color: "gray", Category: workspace.CategoryTodo, Rank: last + "V"},
		workspace.Value{ID: reviewStatus, Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: last + "W"})
}

// mobileItem は、Mobile のプロジェクト「アプリ」の Epic「iOS」の下に、ステータス status の Task(番号 n)を足す。
func mobileItem(t *testing.T, f fixture, id item.ID, n int, status change.ID) item.Item {
	t.Helper()
	return addItem(t, f.rec, store1, item.Item{ID: id, Place: item.Place{Workspace: mobileWS, Project: mobileProject, Parent: ptr(mobileEpic), Number: n}, Kind: item.Task, Rank: "V", Title: "t", Status: status})
}

// addOwnRule は、Mobile のプロジェクト「アプリ」の専用ルール(PR がマージされたらステータスを status にする)を足す。
func addOwnRule(t *testing.T, f fixture, status change.ID) {
	t.Helper()
	r := automation.Rule{ID: "01TESTRULE0000000000000001", Name: "マージで変える", Trigger: automation.TriggerPRMerged, Action: automation.ActionStatus, Value: string(status), Enabled: true, Scope: mobileProject}
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(mobileWS, automation.Drafts(automation.Rule{}, r)...)
	})
}

func projectTo(store port.StoreID, ws workspace.ID) transfers.ProjectMoveInput {
	return transfers.ProjectMoveInput{ToStore: store, ToWorkspace: ws}
}

// 削除されていないアイテム 3 件(Epic と Task 2 件)・削除済み 1 件。接頭辞は移動先のもの。同じ Store ならワークスペース名、
// 別の Store なら「{Store 名} の {ワークスペース名}」。
func TestProjectPreviewCounts(t *testing.T) {
	f := newFixture(t)
	mobileItem(t, f, "01TESTIT000000000000000010", 2, "")
	mobileItem(t, f, "01TESTIT000000000000000011", 3, "")
	gone := mobileItem(t, f, "01TESTIT000000000000000012", 4, "")
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, gone.ID); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.ProjectPreview(t.Context(), store1, mobileProject, projectTo(store1, apptest.SampleWorkspace))
	if err != nil {
		t.Fatal(err)
	}
	if got.Items != 3 || got.Deleted != 1 || got.Prefix != "HH" || got.Source != "Mobile" || got.Destination != "Web" || got.CrossStore {
		t.Errorf("same store = %+v", got)
	}
	got, err = f.svc.ProjectPreview(t.Context(), store1, mobileProject, projectTo(store2, teamWS))
	if err != nil {
		t.Fatal(err)
	}
	if got.Items != 3 || got.Deleted != 1 || got.Prefix != "TM" || got.Source != "main の Mobile" || got.Destination != "team の Team" || !got.CrossStore {
		t.Errorf("cross store = %+v", got)
	}
}

// 自分のワークスペース・削除済みのワークスペース・閲覧のみの Store・ないワークスペースは移動先に選べない。
func TestProjectPreviewRefusesUnselectableDestinations(t *testing.T) {
	f := newFixture(t)
	if err := apptest.DeleteWorkspace(t.Context(), f.rec, store2, teamWS); err != nil {
		t.Fatal(err)
	}
	for name, in := range map[string]transfers.ProjectMoveInput{
		"自分":    projectTo(store1, mobileWS),
		"削除済み":  projectTo(store2, teamWS),
		"閲覧のみ":  projectTo(store3, archiveWS),
		"ないワーク": projectTo(store1, "01TESTWS000000000000000099"),
	} {
		if _, err := f.svc.ProjectPreview(t.Context(), store1, mobileProject, in); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// アイテムが Review(2 件、うち 1 件は削除済み)と Todo、専用ルールが Blocked を使い、移動先(Web)に Review・Blocked がない
// → 行は移動元の並びで Blocked(0)・Review(2)(ルールだけが使うものも同じ並びに入る)。選択肢は、プロジェクトが
// 使わないことにしていて一緒に移る Pending を除いた移動先のステータス。初めの値は同じ分類の移動先の既定。
func TestProjectPreviewReplacements(t *testing.T) {
	f := newFixtureWith(t, withReviewAndBlocked)
	mobileItem(t, f, "01TESTIT000000000000000010", 2, reviewStatus)
	gone := mobileItem(t, f, "01TESTIT000000000000000011", 3, reviewStatus)
	mobileItem(t, f, "01TESTIT000000000000000012", 4, valueID(t, f.rec, "status", "Todo"))
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, gone.ID); err != nil {
		t.Fatal(err)
	}
	addOwnRule(t, f, blockedStatus)
	hideStatus(t, f.rec, store1, mobileWS, mobileProject, "Pending")

	got, err := f.svc.ProjectPreview(t.Context(), store1, mobileProject, projectTo(store1, apptest.SampleWorkspace))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Replacements) != 2 {
		t.Fatalf("rows = %+v", got.Replacements)
	}
	blocked, review := got.Replacements[0], got.Replacements[1]
	if review.Status != reviewStatus || review.Name != "Review" || review.Count != 2 || review.Default != apptest.SampleStatusInProgress {
		t.Errorf("Review = %+v", review)
	}
	if blocked.Status != blockedStatus || blocked.Name != "Blocked" || blocked.Count != 0 || blocked.Default != apptest.SampleStatusTodo {
		t.Errorf("Blocked = %+v", blocked)
	}
	want := []change.ID{apptest.SampleStatusTodo, apptest.SampleStatusInProgress, apptest.SampleStatusDone, apptest.SampleStatusCancel}
	for _, row := range got.Replacements {
		var ids []change.ID
		for _, v := range row.Choices {
			ids = append(ids, v.ID)
		}
		if !slices.Equal(ids, want) {
			t.Errorf("%s の選択肢 = %v, want %v", row.Name, ids, want)
		}
	}
}

// プロジェクトが使わないことにしている Pending をアイテムが持っていても、移動先(Web)に Pending があれば置き換えない
// (ルールと一緒に移るので、移動の前と同じ状態になる)。
func TestProjectPreviewKeepsHiddenStatusThatExists(t *testing.T) {
	f := newFixture(t)
	pending := hideStatus(t, f.rec, store1, mobileWS, mobileProject, "Pending")
	mobileItem(t, f, "01TESTIT000000000000000010", 2, pending)
	got, err := f.svc.ProjectPreview(t.Context(), store1, mobileProject, projectTo(store1, apptest.SampleWorkspace))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Replacements) != 0 {
		t.Errorf("rows = %+v, want none", got.Replacements)
	}
}

// ルールがなければ HasRules は偽で、移らないものはない。プロジェクト別のルールがあれば HasRules は真で、移らないものは
// CarryRules のもの(Blocked は Web にない)。専用ルールだけでも HasRules は真。
func TestProjectPreviewRules(t *testing.T) {
	f := newFixtureWith(t, withReviewAndBlocked)
	in := projectTo(store1, apptest.SampleWorkspace)
	got, err := f.svc.ProjectPreview(t.Context(), store1, mobileProject, in)
	if err != nil || got.HasRules || len(got.NotCarried) != 0 {
		t.Fatalf("no rules = %+v, %v", got, err)
	}
	addOwnRule(t, f, valueID(t, f.rec, "status", "Done"))
	if got, err = f.svc.ProjectPreview(t.Context(), store1, mobileProject, in); err != nil || !got.HasRules || len(got.NotCarried) != 0 {
		t.Fatalf("own rule only = %+v, %v", got, err)
	}
	hideStatus(t, f.rec, store1, mobileWS, mobileProject, "Blocked")
	got, err = f.svc.ProjectPreview(t.Context(), store1, mobileProject, in)
	if err != nil {
		t.Fatal(err)
	}
	if want := []transfer.NotCarried{{Kind: transfer.NotCarriedHidden, Name: "Blocked"}}; !got.HasRules || !slices.EqualFunc(got.NotCarried, want, notCarriedEqual) {
		t.Errorf("hidden Blocked = %+v", got)
	}
}

func notCarriedEqual(a, b transfer.NotCarried) bool {
	return a.Kind == b.Kind && a.Name == b.Name && slices.Equal(a.Missing, b.Missing) && a.Count == b.Count
}

// remote の注意: 同じ Store(main は remote あり)は注意なし・移動先に remote あり。main から team(remote なし)へは注意あり・
// 移動先に remote なし。team から main へは注意なし・移動先に remote あり。
func TestProjectPreviewRemote(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name                  string
		from                  port.StoreID
		in                    transfers.ProjectMoveInput
		notice, destHasRemote bool
	}{
		{"同じ Store", store1, projectTo(store1, apptest.SampleWorkspace), false, true},
		{"main から team", store1, projectTo(store2, teamWS), true, false},
		{"team から main", store2, projectTo(store1, mobileWS), false, true},
	}
	for _, c := range cases {
		p := mobileProject
		if c.from == store2 {
			p = teamProject
		}
		got, err := f.svc.ProjectPreview(t.Context(), c.from, p, c.in)
		if err != nil || got.RemoteNotice != c.notice || got.DestinationHasRemote != c.destHasRemote {
			t.Errorf("%s = %+v, %v", c.name, got, err)
		}
	}
}
