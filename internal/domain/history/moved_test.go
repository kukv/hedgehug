package history_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 移動元のワークスペース(WA)の設定。値の内部 ID は A01〜(ステータス A01 Todo・A02 InProgress)。
func settingsA() workspace.Settings {
	n := 0
	return workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("A%02d", n)) })
}

func taskIn(ws workspace.ID, pr project.ID) item.Item {
	return item.Item{ID: "T", Place: item.Place{Workspace: ws, Project: pr, Number: 2}, Kind: item.Task, Rank: "V", Title: "ログイン", Status: "A01"}
}

func ptrID(id item.ID) *item.ID { return &id }

func movedInFromMain(names map[change.ID]string) transfer.MovedInInfo {
	return transfer.MovedInInfo{
		Old: "HH-2", FromStore: "S1", FromStoreName: "main", FromWorkspace: "WA", FromWorkspaceName: "Web",
		FromProject: "PA", FromProjectTitle: "ログイン", At: at(3), Names: names,
	}
}

// 別の Store から移ってきた。moved_in の行が「{旧 ID} から移動(…)」になり、同じ操作の場所の変更は「親を変更」に出ない。
// 移る前の行の値は moved_in の names で、移動と同じ操作のステータスは from を names、to をいまの設定で引く。
func TestItemRowsMovedInFromAnotherStore(t *testing.T) {
	from, to := taskIn("WA", "PA"), taskIn("WB", "PB")
	info := movedInFromMain(map[change.ID]string{"A01": "Todo", "A02": "InProgress", "status": "ステータス"})
	log := logOf(
		step{"O1", at(0), taro, item.Create(from)},
		step{"O2", at(1), taro, change.NewSet("status", "A01", "A02")},
		step{"O3", at(2), taro, change.NewSet("status", "A02", "A99")}, // names にない値
		step{"O4", at(3), taro, transfer.NewMovedIn(info)},
		step{"O4", at(3), taro, change.NewSet("place", from.Place, to.Place)},
		step{"O4", at(3), taro, change.NewSet("status", "A99", "V01")},
	)
	rows := history.ItemRows(log, ctx())
	want := []string{
		"太郎: ステータスを (削除された値) → Todo",
		"太郎: HH-2 から移動(main / Web / ログイン)",
		"太郎: ステータスを InProgress → (削除された値)",
		"太郎: ステータスを Todo → InProgress",
		"太郎: 作成",
	}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
	if !rows[1].At.Equal(at(3)) {
		t.Errorf("移した日時 = %v", rows[1].At)
	}
}

// 移動と同じ操作の行の from は、同じ操作の moved_in の names で引く(moved_in はその行より前にある)。
// そのあとの行は、いまの設定で引く。
func TestItemRowsMovedInSameOpValueUsesItsNames(t *testing.T) {
	from, to := taskIn("WA", "PA"), taskIn("WB", "PB")
	log := logOf(
		step{"O1", at(0), taro, item.Create(from)},
		step{"O2", at(1), taro, change.NewSet("status", "A01", "A02")},
		step{"O3", at(3), taro, transfer.NewMovedIn(movedInFromMain(map[change.ID]string{"A01": "Todo", "A02": "InProgress"}))},
		step{"O3", at(3), taro, change.NewSet("place", from.Place, to.Place)},
		step{"O3", at(3), taro, change.NewSet("status", "A02", "V01")},
		step{"O4", at(4), taro, change.NewSet("status", "V01", "V03")},
	)
	want := []string{
		"太郎: ステータスを Todo → Done",
		"太郎: ステータスを InProgress → Todo",
		"太郎: HH-2 から移動(main / Web / ログイン)",
		"太郎: ステータスを Todo → InProgress",
		"太郎: 作成",
	}
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}

// 同じ Store の別のワークスペースへ。場所の変更の行と、転送記録(同じ op)から作り、移動元のワークスペースの値の名前で引く。
func TestItemRowsMovedWithinTheStore(t *testing.T) {
	from, to := taskIn("WA", "PA"), taskIn("WB", "PB")
	reparented := to.Place
	reparented.Parent = ptrID("E9")
	log := logOf(
		step{"O1", at(0), taro, item.Create(from)},
		step{"O2", at(1), taro, change.NewSet("status", "A01", "A02")},
		step{"O3", at(3), taro, change.NewSet("place", from.Place, to.Place)},
		step{"O3", at(3), taro, change.NewSet("status", "A02", "V01")},
		step{"O4", at(4), taro, change.NewSet("place", to.Place, reparented)},
	)
	c := ctx()
	c.Origin = history.Origin{
		Settings:      func(id workspace.ID) (workspace.Settings, bool) { return settingsA(), id == "WA" },
		WorkspaceName: func(id workspace.ID) (string, bool) { return "Web", id == "WA" },
		ProjectTitle:  func(id project.ID) (string, bool) { return "ログイン", id == "PA" },
		StoreName:     "main",
		Transfers:     []transfer.Record{{Op: "O3", Target: "T", Old: "HH-2", New: "MB-2"}},
	}
	got := texts(history.ItemRows(log, c))
	want := []string{
		"太郎: 親を変更", // 同じワークスペースの中の親の変更は、いままでどおり
		"太郎: ステータスを InProgress → Todo",
		"太郎: HH-2 から移動(main / Web / ログイン)",
		"太郎: ステータスを Todo → InProgress",
		"太郎: 作成",
	}
	if !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}

// 転送記録がない(手で書き換えた)ワークスペースの変更は、文を作らない。
func TestItemRowsWorkspaceChangeWithoutARecordIsNotShown(t *testing.T) {
	from, to := taskIn("WA", "PA"), taskIn("WB", "PB")
	log := logOf(step{"O1", at(0), taro, item.Create(from)}, step{"O2", at(1), taro, change.NewSet("place", from.Place, to.Place)})
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, []string{"太郎: 作成"}) {
		t.Errorf("rows = %q", got)
	}
}

// 移動の引き直しの関連システム(取り除き・追加)は、操作の前と後の一覧の 1 行にまとめ、名前が同じなら出さない。
func TestItemRowsMovedInSystemsAreOneRowPerProperty(t *testing.T) {
	a := settingsA().Systems
	names := map[change.ID]string{a[0].ID: a[0].Name, a[1].ID: a[1].Name}
	cur := settings().Systems
	from := taskIn("WA", "PA")
	from.Systems = []change.ID{a[0].ID, a[1].ID}
	to := taskIn("WB", "PB").Place
	for _, c := range []struct {
		added []change.ID
		want  []string
	}{
		{[]change.ID{cur[0].ID, cur[1].ID}, []string{"太郎: HH-2 から移動(main / Web / ログイン)", "太郎: 作成"}},
		{[]change.ID{cur[0].ID}, []string{
			fmt.Sprintf("太郎: 関連システムを %s, %s → %s", a[0].Name, a[1].Name, cur[0].Name),
			"太郎: HH-2 から移動(main / Web / ログイン)", "太郎: 作成",
		}},
	} {
		steps := []step{
			{"O1", at(0), taro, item.Create(from)},
			{"O2", at(1), taro, transfer.NewMovedIn(movedInFromMain(names))},
			{"O2", at(1), taro, change.NewSet("place", from.Place, to)},
			{"O2", at(1), taro, change.NewRemove("systems", a[0].ID)},
			{"O2", at(1), taro, change.NewRemove("systems", a[1].ID)},
		}
		for _, id := range c.added {
			steps = append(steps, step{"O2", at(1), taro, change.NewAdd("systems", id)})
		}
		if got := texts(history.ItemRows(logOf(steps...), ctx())); !slices.Equal(got, c.want) {
			t.Errorf("added %v: rows =\n%q\nwant\n%q", c.added, got, c.want)
		}
	}
}

// A → B → C と 2 回、別の Store へ移した。B にいたあいだの値は、B から C への moved_in の names で引き、A から B への
// 引き直しの行の to(B の値)も同じ names で引く(名前が同じなので出ない)。
func TestItemRowsMovedInTwiceAcrossStores(t *testing.T) {
	a, b, c := taskIn("WA", "PA"), taskIn("WB", "PB"), taskIn("WC", "PC")
	fromB := transfer.MovedInInfo{
		Old: "TM-2", FromStore: "S2", FromStoreName: "team", FromWorkspace: "WB", FromWorkspaceName: "Team",
		FromProject: "PB", FromProjectTitle: "アプリ", At: at(3), Names: map[change.ID]string{"B01": "Todo", "B02": "InProgress"},
	}
	log := logOf(
		step{"O1", at(0), taro, item.Create(a)},
		step{"O2", at(1), taro, transfer.NewMovedIn(movedInFromMain(map[change.ID]string{"A01": "Todo"}))},
		step{"O2", at(1), taro, change.NewSet("place", a.Place, b.Place)},
		step{"O2", at(1), taro, change.NewSet("status", "A01", "B01")},
		step{"O3", at(2), taro, change.NewSet("status", "B01", "B02")},
		step{"O4", at(3), taro, transfer.NewMovedIn(fromB)},
		step{"O4", at(3), taro, change.NewSet("place", b.Place, c.Place)},
		step{"O4", at(3), taro, change.NewSet("status", "B02", "V02")},
	)
	want := []string{
		"太郎: TM-2 から移動(team / Team / アプリ)",
		"太郎: ステータスを Todo → InProgress",
		"太郎: HH-2 から移動(main / Web / ログイン)",
		"太郎: 作成",
	}
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}

// 同じ Store の中の移動で、移動元のワークスペース・プロジェクトの名前が引けない(完全に削除した)ときは「(削除された値)」。
func TestItemRowsMovedWithinTheStoreFromAPurgedWorkspace(t *testing.T) {
	from, to := taskIn("WA", "PA"), taskIn("WB", "PB")
	log := logOf(step{"O1", at(0), taro, item.Create(from)}, step{"O2", at(1), taro, change.NewSet("place", from.Place, to.Place)})
	c := ctx()
	c.Origin = history.Origin{
		WorkspaceName: func(workspace.ID) (string, bool) { return "", false },
		ProjectTitle:  func(project.ID) (string, bool) { return "", false },
		StoreName:     "main",
		Transfers:     []transfer.Record{{Op: "O2", Target: "T", Old: "HH-2", New: "MB-2"}},
	}
	want := []string{"太郎: HH-2 から移動(main / (削除された値) / (削除された値))", "太郎: 作成"}
	if got := texts(history.ItemRows(log, c)); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}
