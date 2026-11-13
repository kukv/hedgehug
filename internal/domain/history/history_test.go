package history_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func at(h int) time.Time { return time.Date(2026, 11, 13, 9+h, 0, 0, 0, time.UTC) }

type step struct {
	op    change.ID
	at    time.Time
	actor change.Actor
	d     change.Draft
}

// logOf は、steps を順に 1 行ずつ書いたログ。
func logOf(steps ...step) []change.Record {
	var log []change.Record
	n := 0
	for _, s := range steps {
		log = append(log, change.Stamp(log, []change.Draft{s.d}, change.Meta{Op: s.op, At: s.at, Actor: s.actor}, func() change.ID {
			n++
			return change.ID(fmt.Sprintf("R%03d", n))
		})...)
	}
	return log
}

// settings は、既定の設定。値の内部 ID は、ステータス V01〜V05(Todo・InProgress・Done・Pending・Cancel)、
// 優先度 V06〜V10、関連システム V11〜V16。
func settings() workspace.Settings {
	n := 0
	return workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("V%02d", n)) })
}

func names(login string) string {
	if login == "taro" {
		return "太郎"
	}
	return "不明なユーザー"
}

func texts(rows []history.Row) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Who+": "+r.Text)
	}
	return out
}

func ctx() history.Context {
	return history.Context{Settings: settings(), Names: names, Today: at(0)}
}

var (
	taro   = change.User("taro")
	hanako = change.User("hanako")
)

func TestItemRowsNewestFirst(t *testing.T) {
	parent := item.ID("E1")
	other := item.ID("E2")
	base := item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Parent: &parent, Number: 2}, Kind: item.Task, Rank: "V", Title: "ログイン", Status: "V01"}
	moved := base.Place
	moved.Parent = &other
	renumbered := base.Place
	renumbered.Number = 9
	log := logOf(
		step{"O1", at(0), taro, item.Create(base)},
		step{"O2", at(1), taro, change.NewSet("status", "V01", "V02")},
		step{"O3", at(2), hanako, change.NewSet("title", "ログイン", "ログイン画面")},
		step{"O4", at(3), taro, change.NewDesc()},
		step{"O5", at(4), change.Actor{Kind: change.ActorAutomation, Login: "taro", Rule: "RULE1"}, change.NewSet("kind", "Task", "Bug")},
		step{"O6", at(5), change.Actor{Kind: change.ActorSync}, change.NewSet("place", base.Place, moved)},
		step{"O7", at(6), taro, change.NewSet("place", base.Place, renumbered)},
		step{"O8", at(7), taro, change.NewSet("rank", "V", "W")},
		step{"O9", at(8), taro, change.NewSet("points", nil, 5)},
		step{"O10", at(9), taro, change.NewSet("assignee", nil, "taro")},
		step{"O11", at(10), taro, change.NewSet("start", nil, "2026-11-20")},
		step{"O12", at(11), taro, change.NewSet("status", "V02", "V99")},
		step{"O13", at(12), taro, change.NewAdd("watchers", "hanako")},
		step{"O14", at(13), taro, change.Draft{T: change.Type("comment"), F: "C1", To: change.Value(map[string]any{"body": "確認"})}},
		step{"O15", at(14), change.Actor{Kind: change.ActorGitHub}, change.NewSet("links/pr:org/web#1/state", "open", "merged")},
		step{"O16", at(15), taro, change.NewSet("fields/01GONE0000000000000000001", nil, "x")},
	)
	rows := history.ItemRows(log, ctx())
	want := []string{
		"GitHub: org/web#1 の状態を Merged に変更",
		"太郎: ステータスを InProgress → (削除された値)",
		"太郎: 開始日を なし → 11/20",
		"太郎: 担当者を なし → 太郎",
		"太郎: ポイントを なし → 5",
		"同期: 親を変更",
		"自動化: 種類を Task から Bug に変更",
		"太郎: 説明を変更",
		"不明なユーザー: タイトルを「ログイン画面」に変更",
		"太郎: ステータスを Todo → InProgress",
		"太郎: 作成",
	}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
	if !rows[0].At.Equal(at(14)) || !rows[len(rows)-1].At.Equal(at(0)) {
		t.Errorf("At = %v ... %v", rows[0].At, rows[len(rows)-1].At)
	}
}

// 関連システムの追加・取り除きは「旧 → 新」で出す。旧と新は、その行の直前までと、その行までのログから決める。
// 同時の変更(prev が同じ)は、後勝ちの規則で決まった一覧になる。
func TestItemRowsOfASystemsListIncludingAConcurrentChange(t *testing.T) {
	st := settings()
	web, api := st.Systems[0], st.Systems[1]
	base := item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Task, Rank: "V", Title: "画面", Systems: []change.ID{web.ID}}
	log := logOf(
		step{"O1", at(0), taro, item.Create(base)},
		step{"O2", at(1), taro, change.NewAdd("systems", api.ID)},                                   // R002。prev は作成の R001
		step{"O9", at(5), taro, change.NewAdd("relations", item.Relation{Type: "blocks", To: "X"})}, // 出さない
	)
	// hanako が、taro の追加を見ずに(prev が作成だけ)api を取り除く。時刻はあと。
	log = append(log, change.Record{
		ID: "R100", Op: "O3", At: at(2), By: "hanako", Actor: hanako, T: change.Remove, F: "systems",
		From: change.Value(api.ID), Prev: []change.ID{"R001"},
	})
	got := texts(history.ItemRows(log, ctx()))
	want := []string{
		// 同時の取り除きが後勝ちで、api は外れる。旧は、見えていた [web, api]。
		fmt.Sprintf("不明なユーザー: 関連システムを %s, %s → %s", web.Name, api.Name, web.Name),
		fmt.Sprintf("太郎: 関連システムを %s → %s, %s", web.Name, web.Name, api.Name),
		"太郎: 作成",
	}
	if !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
	// 同じ一覧になる行(すでにある値の追加)は出さない。
	log = append(log, change.Stamp(log, []change.Draft{change.NewAdd("systems", web.ID)}, change.Meta{Op: "O4", At: at(3), Actor: taro}, func() change.ID { return "R200" })...)
	if got := texts(history.ItemRows(log, ctx())); len(got) != len(want) {
		t.Errorf("結果が変わらない追加は出さない: %q", got)
	}
}

func TestItemRowsOfOneSaveShowLaterRowsFirst(t *testing.T) {
	base := item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Epic, Rank: "V", Title: "認証", Status: "V01"}
	log := logOf(
		step{"O1", at(0), taro, item.Create(base)},
		step{"O2", at(1), taro, change.NewSet("title", "認証", "認可")},
		step{"O2", at(1), taro, change.NewSet("status", "V01", "V02")},
	)
	want := []string{"太郎: ステータスを Todo → InProgress", "太郎: タイトルを「認可」に変更", "太郎: 作成"}
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

// 値やプロパティの削除と同じ保存で書いたアイテムの行は出さない(A4a-16。B2-9)。
func TestItemRowsHideTheRowsOfAValueDeletion(t *testing.T) {
	base := item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Task, Rank: "V", Title: "画面", Status: "V05"}
	log := logOf(
		step{"O1", at(0), taro, item.Create(base)},
		step{"O2", at(1), taro, change.NewSet("status", "V05", "V01")},
		step{"O3", at(2), taro, change.NewSet("title", "画面", "画面 2")},
	)
	wsLog := logOf(
		step{"W1", at(0), taro, change.NewCreate(map[string]any{"id": "W", "name": "Web", "prefix": "HH"})},
		step{"O2", at(1), taro, change.NewDelete("statuses/V05")},
	)
	hidden := history.HiddenOps(wsLog)
	if !hidden["O2"] || hidden["W1"] || len(hidden) != 1 {
		t.Fatalf("HiddenOps = %v", hidden)
	}
	c := ctx()
	c.Hidden = hidden
	want := []string{"太郎: タイトルを「画面 2」に変更", "太郎: 作成"}
	if got := texts(history.ItemRows(log, c)); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

func TestWho(t *testing.T) {
	cases := []struct {
		actor change.Actor
		want  string
	}{
		{taro, "太郎"},
		{hanako, "不明なユーザー"},
		{change.Actor{Kind: change.ActorAutomation, Login: "taro", Rule: "R"}, "自動化"},
		{change.Actor{Kind: change.ActorGitHub}, "GitHub"},
		{change.Actor{Kind: change.ActorSync}, "同期"},
		{change.Actor{Kind: change.ActorNone}, "不明なユーザー"},
	}
	for _, c := range cases {
		if got := history.Who(c.actor, names); got != c.want {
			t.Errorf("Who(%+v) = %q, want %q", c.actor, got, c.want)
		}
	}
}

func TestNamesOfIgnoresCaseAndFallsBack(t *testing.T) {
	n := history.NamesOf([]member.Member{{Login: "Taro", Name: "太郎"}})
	if n("taro") != "太郎" || n("TARO") != "太郎" || n("jiro") != "不明なユーザー" {
		t.Errorf("NamesOf: %q %q %q", n("taro"), n("TARO"), n("jiro"))
	}
}

// 「親を変更」は親が変わったときだけ出す。親が両方ない(Epic)なら、プロジェクトが変わったときに出す。
// プロジェクトが変わる移動で一緒に移る下のもの(親は同じ)の行は出さない(02「同じワークスペースの中での移動」)。
func TestPlaceRowsShowOnlyAParentChange(t *testing.T) {
	parent, other := item.ID("E1"), item.ID("E2")
	cases := []struct {
		name     string
		from, to item.Place
		want     []string
	}{
		{"親が変わる", item.Place{Project: "P", Parent: &parent}, item.Place{Project: "P", Parent: &other}, []string{"太郎: 親を変更"}},
		{"親とプロジェクトが変わる", item.Place{Project: "P", Parent: &parent}, item.Place{Project: "Q", Parent: &other}, []string{"太郎: 親を変更"}},
		{"Epic のプロジェクトが変わる", item.Place{Project: "P"}, item.Place{Project: "Q"}, []string{"太郎: 親を変更"}},
		{"親がなくなる", item.Place{Project: "P", Parent: &parent}, item.Place{Project: "P"}, []string{"太郎: 親を変更"}},
		{"親が同じでプロジェクトだけ変わる(下のもの)", item.Place{Project: "P", Parent: &parent}, item.Place{Project: "Q", Parent: &parent}, nil},
	}
	for _, c := range cases {
		log := logOf(step{"O1", at(0), taro, change.NewSet("place", c.from, c.to)})
		if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, c.want) {
			t.Errorf("%s: rows = %q, want %q", c.name, got, c.want)
		}
	}
}

// Step から昇格した行(同じ op に set kind と set place)は「種類を … に変更」の 1 行だけにする。
// 種類を変えない別の op の set place は、これまでどおり「親を変更」と出る。
func TestPlaceRowIsHiddenWhenTheSameOpChangesTheKind(t *testing.T) {
	parent, epic := item.ID("T1"), item.ID("E1")
	from, to := item.Place{Project: "P", Parent: &parent}, item.Place{Project: "P", Parent: &epic}
	n := 0
	newID := func() change.ID { n++; return change.ID(fmt.Sprintf("R%03d", n)) }
	log := change.Stamp(nil, []change.Draft{
		change.NewSet("kind", "Step", "Story"), change.NewSet("place", from, to), change.NewSet("rank", "V", "W"),
	}, change.Meta{Op: "O1", At: at(0), Actor: taro}, newID)
	log = append(log, change.Stamp(log, []change.Draft{change.NewSet("place", to, from)},
		change.Meta{Op: "O2", At: at(1), Actor: taro}, newID)...)
	want := []string{"太郎: 親を変更", "太郎: 種類を Step から Story に変更"}
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

func TestItemRowsShowGitHubLinkRows(t *testing.T) {
	gh := change.Actor{Kind: change.ActorGitHub}
	pr := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 41}
	moved := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web2", Number: 41}
	log := logOf(
		step{"O1", at(0), taro, item.Create(item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Task, Rank: "V", Title: "画面"})},
		step{"O2", at(1), taro, change.NewAdd("links", pr)},                                   // 手でのリンク: 出さない
		step{"O2", at(1), taro, change.NewSet(itemlink.ValueField(pr, "state"), nil, "open")}, // 初めの値: 出さない
		step{"O3", at(2), gh, change.NewSet(itemlink.ValueField(pr, "review"), "none", "approved")},
		step{"O4", at(3), gh, change.NewSet(itemlink.ValueField(pr, "title"), "A", "B")},
		step{"O5", at(4), gh, change.NewRemove("links", pr)},
		step{"O5", at(4), gh, change.NewAdd("links", moved)},
		step{"O5", at(4), gh, change.NewSet(itemlink.ValueField(moved, "title"), "B", "B")},
		step{"O5", at(4), gh, change.NewSet(itemlink.ValueField(moved, "state"), "open", "closed")},
		step{"O6", at(5), gh, change.NewAdd("links", itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 50})},
		step{"O7", at(6), taro, change.NewRemove("links", moved)}, // 手での解除: 出さない
		step{"O8", at(7), gh, change.NewSet(itemlink.ValueField(moved, "state"), "open", "open")},
	)
	want := []string{
		"GitHub: org/web#50 を自動でリンク",
		"GitHub: org/web2#41 の状態を Closed に変更",
		"GitHub: リンクを org/web#41 から org/web2#41 に更新",
		"GitHub: org/web#41 のタイトルを「B」に変更",
		"GitHub: org/web#41 のレビューを 承認済み に変更",
		"太郎: 作成",
	}
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}

func TestItemRowsGitHubLinkTitleIsOneLineAndOnlyLinksFields(t *testing.T) {
	gh := change.Actor{Kind: change.ActorGitHub}
	pr := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 41}
	log := logOf(
		step{"O1", at(0), taro, item.Create(item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Task, Rank: "V", Title: "画面"})},
		step{"O2", at(1), gh, change.NewSet(itemlink.ValueField(pr, "title"), "A", "B\n  C")},
		step{"O3", at(2), gh, change.NewSet("linksX/pr:org/web#41/state", "open", "merged")}, // links の項目ではない
	)
	want := []string{"GitHub: org/web#41 のタイトルを「B C」に変更", "太郎: 作成"}
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}
