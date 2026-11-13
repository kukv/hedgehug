package transfers_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
)

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
