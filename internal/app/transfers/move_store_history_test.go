package transfers_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
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
