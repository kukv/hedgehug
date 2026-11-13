package history_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/project"
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
