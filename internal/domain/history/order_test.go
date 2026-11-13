package history_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// 時計がずれて、A を見て書いた C(prev が A)が、at の順では A より先に並ぶログ。
// 旧と新は、並びでなく prev に沿って決めるので、どちらの行も出る。
func TestItemRowsFollowPrevWhenTheClockIsSkewed(t *testing.T) {
	st := settings()
	web, api := st.Systems[0], st.Systems[1]
	base := item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Task, Rank: "V", Title: "画面", Systems: []change.ID{web.ID}}
	log := logOf(step{"O1", at(0), taro, item.Create(base)}) // R001
	// ファイルの並び(at の順)は、作成・C・A。
	log = append(log,
		change.Record{ID: "R100", Op: "O3", At: at(1), By: "hanako", Actor: hanako, T: change.Remove, F: "systems", From: change.Value(api.ID), Prev: []change.ID{"R200"}},
		change.Record{ID: "R200", Op: "O2", At: at(2), By: "taro", Actor: taro, T: change.Add, F: "systems", To: change.Value(api.ID), Prev: []change.ID{"R001"}},
	)
	want := []string{
		fmt.Sprintf("太郎: 関連システムを %s → %s, %s", web.Name, web.Name, api.Name),
		fmt.Sprintf("不明なユーザー: 関連システムを %s, %s → %s", web.Name, api.Name, web.Name),
		"太郎: 作成",
	}
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}

// 消えた値どうしの変更など、表示が同じになる set は出さない。
func TestItemRowsSkipASetWhoseDisplayDoesNotChange(t *testing.T) {
	base := item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Task, Rank: "V", Title: "画面", Status: "V98"}
	log := logOf(
		step{"O1", at(0), taro, item.Create(base)},
		step{"O2", at(1), taro, change.NewSet("status", "V98", "V99")},
	)
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, []string{"太郎: 作成"}) {
		t.Errorf("rows = %q", got)
	}
}
