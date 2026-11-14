package transfers_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// teamHistory は、team(S2)に移したアイテム id の履歴の行を「実行者: 文」で返す(いまの設定は Team)。
func teamHistory(t *testing.T, f fixture, id item.ID) []string {
	t.Helper()
	log := logOf(t, f.rec, store2, change.Ref{Kind: change.KindItem, ID: id})
	c := history.Context{Names: func(login string) string { return login }}
	if err := f.rec.Read(t.Context(), store2, func(r port.Reader) (err error) { c.Settings, err = r.Workspaces().Settings(teamWS); return }); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, row := range history.ItemRows(log, c) {
		got = append(got, row.Who+": "+row.Text)
	}
	return got
}

// 同じ Store の中で移した(Web → Mobile)あとに別の Store(team)へ移しても、前の移動の行「HH-2 から移動(main / Web / …)」と、
// Web にいたあいだの値の名前が、移動先の履歴に残る。
func TestMoveAcrossStoresKeepsTheHistoryOfAnEarlierMoveWithinTheStore(t *testing.T) {
	f := newFixture(t)
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusInProgress))
	})
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"hanako: MB-2 から移動(main / Mobile / アプリ)",
		"hanako: HH-2 から移動(main / Web / ログイン)",
		"taro: ステータスを Todo → InProgress",
		"taro: 作成",
	}
	if got := teamHistory(t, f, apptest.SampleTask); !slices.Equal(got, want) {
		t.Errorf("移動先の履歴 =\n%q\nwant\n%q", got, want)
	}
}

// 下のもの(Step HH-4)の moved_in の moves は、同じ操作の記録のうち、そのアイテム自身の記録の旧 ID を持つ(親の HH-2 ではない)。
func TestMoveAcrossStoresKeepsEachItemsOwnEarlierMove(t *testing.T) {
	f := newFixture(t)
	withSteps(t, f)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); err != nil {
		t.Fatal(err)
	}
	mi := transfer.MovedInOf(logOf(t, f.rec, store2, change.Ref{Kind: change.KindItem, ID: stepLiveID}))
	if len(mi) != 1 || len(mi[0].Moves) != 1 || mi[0].Moves[0].Old != "HH-4" {
		t.Errorf("Step の moved_in = %+v", mi)
	}
}

// 別の Store へ移したものを、さらに別の Store へ移せる(team → main へ戻す)。行に現れる前の Store のワークスペースは
// 移動先の Store にないので、names を引く設定から外す。
func TestMoveAcrossStoresTwiceAndBack(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); err != nil {
		t.Fatal(err)
	}
	back := transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject, ToParent: ptr(mobileEpic)}
	if got, err := f.svc.Move(t.Context(), store2, apptest.SampleTask, back); err != nil || got.Display != "MB-2" {
		t.Fatalf("戻す: %+v, %v", got, err)
	}
}

// 別の Store へ移したあとの移動先の履歴。作成のときにだけ設定した値(ステータス Todo・優先度 Medium)も moved_in の
// names に入るので、引き直しの行の from が「(削除された値)」にならず、名前が同じ引き直しは出ない。
func TestMoveAcrossStoresKeepsNamesOfValuesSetAtCreation(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); err != nil {
		t.Fatal(err)
	}
	log := logOf(t, f.rec, store2, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	c := history.Context{Names: func(login string) string { return login }}
	if err := f.rec.Read(t.Context(), store2, func(r port.Reader) (err error) { c.Settings, err = r.Workspaces().Settings(teamWS); return }); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, row := range history.ItemRows(log, c) {
		got = append(got, row.Who+": "+row.Text)
	}
	if want := []string{"hanako: HH-2 から移動(main / Web / ログイン)", "taro: 作成"}; !slices.Equal(got, want) {
		t.Errorf("移動先の履歴 = %q, want %q", got, want)
	}
}

// projectHistory は、Store store のプロジェクト id の履歴の行を「実行者: 文」で返す。ワークスペースの名前はその Store の
// 削除されていないワークスペースから引く(app/history.Project と同じ)。
func projectHistory(t *testing.T, f fixture, store port.StoreID, id project.ID) []string {
	t.Helper()
	log := logOf(t, f.rec, store, change.Ref{Kind: change.KindProject, ID: id})
	names := map[workspace.ID]string{}
	if err := f.rec.Read(t.Context(), store, func(r port.Reader) error {
		wss, err := port.LiveWorkspaces(r)
		for _, w := range wss {
			names[w.ID] = w.Name
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	c := history.ProjectContext{Names: func(login string) string { return login }, WorkspaceName: func(id workspace.ID) string { return names[id] }}
	var got []string
	for _, row := range history.ProjectRows(log, c) {
		got = append(got, row.Who+": "+row.Text)
	}
	return got
}

// A5b-21: MoveProject が書いた行で、同じ Store の中の移動(Web → Mobile)は「Web から移動」、別の Store(team)への移動は
// moved_in の「Mobile から移動」(同じ操作の場所の set は出さない)。team に写した main の中の移動の場所の set は、
// アイテムと同じく moved_in の Moves の名前で「Web から移動」。
func TestProjectHistoryOfMoves(t *testing.T) {
	f := newFixture(t)
	moveProject(t, f, apptest.SampleProject, projectTo(store1, mobileWS))
	if got, want := projectHistory(t, f, store1, apptest.SampleProject), []string{"hanako: Web から移動", "taro: 作成"}; !slices.Equal(got, want) {
		t.Errorf("同じ Store: %q, want %q", got, want)
	}
	moveProject(t, f, apptest.SampleProject, toTeamWS())
	// team のメンバーは Taro と綴るので、オーナーの置き換え(set owner)もほかの変更と同じく出る。
	want := []string{"hanako: オーナーを Taro に変更", "hanako: Mobile から移動", "hanako: Web から移動", "taro: 作成"}
	if got := projectHistory(t, f, store2, apptest.SampleProject); !slices.Equal(got, want) {
		t.Errorf("別の Store: %q, want %q", got, want)
	}
}
