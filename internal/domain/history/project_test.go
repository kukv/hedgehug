package history_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestProjectRows(t *testing.T) {
	base := project.Project{ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Rank: "V", Title: "ログイン", Owner: "", Status: project.Todo}
	log := logOf(
		step{"O1", at(0), taro, project.Create(base)},
		step{"O2", at(1), taro, change.NewSet("title", "ログイン", "認証")},
		step{"O3", at(2), taro, change.NewSet("status", "Todo", "InProgress")},
		step{"O4", at(3), hanako, change.NewSet("owner", "", "taro")},
		step{"O5", at(4), taro, change.NewSet("owner", "taro", "")},
		step{"O6", at(5), taro, change.NewSet("start", "", "2026-12-01")},
		step{"O7", at(6), taro, change.NewSet("end", "2026-12-01", "")},
		step{"O8", at(7), taro, change.NewDesc()},
		step{"O9", at(8), taro, change.NewSet("place", base.Place, project.Place{Workspace: "W2", Number: 3})},
		step{"O10", at(9), taro, change.NewSet("rank", "V", "W")},
		step{"O11", at(10), taro, change.Draft{T: change.Type("comment"), F: "C1", To: change.Value(map[string]any{"body": "確認"})}},
		step{"O12", at(11), taro, change.NewAdd("repos", "org/web")},
	)
	rows := history.ProjectRows(log, history.ProjectContext{Names: names, Today: at(0)})
	want := []string{
		"太郎: (削除された値) から移動", // ワークスペースの名前を引けない(WorkspaceName がない)
		"太郎: 説明を変更",
		"太郎: 終了日を なし に変更",
		"太郎: 開始日を 12/1 に変更",
		"太郎: オーナーを なし に変更",
		"不明なユーザー: オーナーを 太郎 に変更",
		"太郎: ステータスを InProgress に変更",
		"太郎: タイトルを 認証 に変更",
		"太郎: 作成",
	}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}

// A5b-21: 同じ Store の中でワークスペースが変わる場所の set と、別の Store からの moved_in は「{移動元のワークスペース名} から移動」。
// moved_in と同じ操作の場所の set、番号だけが変わる場所の set は出さない。名前が引けない(Store にない)ワークスペースと、
// moved_in の名前が空のときは「(削除された値)」。
func TestProjectRowsMoved(t *testing.T) {
	base := project.Project{ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Rank: "V", Title: "ログイン", Status: project.Todo}
	at2 := project.Place{Workspace: "W2", Number: 3}
	log := logOf(
		step{"O1", at(0), taro, project.Create(base)},
		step{"O2", at(1), taro, change.NewSet("place", base.Place, at2)},
		step{"O3", at(2), taro, change.NewSet("place", at2, project.Place{Workspace: "W2", Number: 4})},
		step{"O4", at(3), taro, transfer.NewMovedIn(transfer.MovedInInfo{Old: "MB-P4", FromStore: "S1", FromStoreName: "main", FromWorkspace: "W2", FromWorkspaceName: "Web", FromProject: "P", FromProjectTitle: "ログイン", At: at(3)})},
		step{"O4", at(3), taro, change.NewSet("place", project.Place{Workspace: "W2", Number: 4}, project.Place{Workspace: "W3", Number: 2})},
		step{"O5", at(4), taro, change.NewSet("place", project.Place{Workspace: "W3", Number: 2}, project.Place{Workspace: "W9", Number: 5})},
		step{"O6", at(5), taro, change.NewSet("place", project.Place{Workspace: "W9", Number: 5}, project.Place{Workspace: "W3", Number: 6})},
		step{"O7", at(6), taro, transfer.NewMovedIn(transfer.MovedInInfo{Old: "TM-P6", FromStore: "S2", FromStoreName: "team", FromWorkspace: "W3", At: at(6)})},
	)
	wsNames := map[workspace.ID]string{"W": "Web", "W3": "Team"} // W9 は Store にない
	rows := history.ProjectRows(log, history.ProjectContext{Names: names, Today: at(0), WorkspaceName: func(id workspace.ID) string { return wsNames[id] }})
	want := []string{
		"太郎: (削除された値) から移動", // O7: moved_in の名前が空
		"太郎: (削除された値) から移動", // O6: W9 から
		"太郎: Team から移動",     // O5: W3 から
		"太郎: Web から移動",      // O4: moved_in の FromWorkspaceName。同じ操作の場所の set は出さない
		"太郎: Web から移動",      // O2: W から。O3 は番号だけなので出さない
		"太郎: 作成",
	}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}

// 移動元の Store の中で別のワークスペースへ移した場所の set(写した行)は、アイテムと同じく moved_in の Moves の
// ワークスペース名で「{ワークスペース名} から移動」(A5a-2。この Store の WorkspaceName では引けない)。Moves の名前が
// 空なら「(削除された値)」。
func TestProjectRowsMovedWithinThePreviousStore(t *testing.T) {
	base := project.Project{ID: "P", Place: project.Place{Workspace: "WA", Number: 1}, Rank: "V", Title: "ログイン", Status: project.Todo}
	log := logOf(
		step{"O1", at(0), taro, project.Create(base)},
		step{"O2", at(1), taro, change.NewSet("place", base.Place, project.Place{Workspace: "WB", Number: 2})},
		step{"O3", at(2), taro, change.NewSet("place", project.Place{Workspace: "WB", Number: 2}, project.Place{Workspace: "WC", Number: 3})},
		step{"O4", at(3), taro, transfer.NewMovedIn(transfer.MovedInInfo{
			Old: "MX-P3", FromStore: "S1", FromStoreName: "main", FromWorkspace: "WC", FromWorkspaceName: "Mobile", FromProject: "P", FromProjectTitle: "ログイン", At: at(3),
			Moves: []transfer.Move{{Op: "O2", Old: "HH-P1", FromWorkspaceName: "Web"}, {Op: "O3", Old: "MB-P2"}},
		})},
		step{"O4", at(3), taro, change.NewSet("place", project.Place{Workspace: "WC", Number: 3}, project.Place{Workspace: "WT", Number: 2})},
	)
	rows := history.ProjectRows(log, history.ProjectContext{Names: names, Today: at(0), WorkspaceName: func(workspace.ID) string { return "" }})
	want := []string{"太郎: Mobile から移動", "太郎: (削除された値) から移動", "太郎: Web から移動", "太郎: 作成"}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}
