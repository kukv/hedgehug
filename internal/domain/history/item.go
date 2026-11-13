package history

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ItemRows は、アイテムの履歴を新しい順で返す(02「履歴」の表)。出すのは、作成・説明の変更・タイトル・種類・親・
// プロパティの変更(関連システムと複数選択の追加・取り除きを含む)だけで、コメント・作業時間・関連・ウォッチ・削除と、知らない種類の行は出さない。
func ItemRows(log []change.Record, c Context) []Row {
	log = byPrev(log)
	var rows []Row
	for i, r := range log {
		if c.Hidden[r.Op] {
			continue
		}
		if text, ok := itemText(log, i, c); ok {
			rows = append(rows, Row{Who: Who(r.Actor, c.Names), Text: text, At: r.At})
		}
	}
	return newestFirst(rows)
}

func itemText(log []change.Record, i int, c Context) (string, bool) {
	r := log[i]
	switch r.T {
	case change.Create:
		return "作成", true
	case change.Desc:
		return "説明を変更", true
	case change.Set:
		return setText(r, c)
	case change.Add, change.Remove:
		return listText(log, i, c)
	}
	return "", false
}

// listText は、関連システムと複数選択の追加・取り除きを「{プロパティ名}を {旧} → {新}」にする。旧は log[:i]、
// 新は log[:i+1] から決めた一覧なので、同時の変更も、prev のグラフの規則で決まった結果になる。
// プロパティでない項目(関連・ウォッチなど)と、一覧が変わらない行は出さない。
func listText(log []change.Record, i int, c Context) (string, bool) {
	p, ok := propertyOf(log[i].F, c.Settings)
	if !ok || (p.Type != workspace.TypeSystems && p.Type != workspace.TypeMulti) {
		return "", false
	}
	before, okBefore := item.FromLog(log[:i])
	after, okAfter := item.FromLog(log[:i+1])
	if !okBefore || !okAfter {
		return "", false
	}
	from, to := item.Current(before, p), item.Current(after, p)
	if bytes.Equal(from, to) {
		return "", false
	}
	return fmt.Sprintf("%sを %s → %s", p.Name, item.Display(p, from, c.Settings, c.Names, c.Today), item.Display(p, to, c.Settings, c.Names, c.Today)), true
}

func setText(r change.Record, c Context) (string, bool) {
	switch r.F {
	case "title":
		var to string
		if json.Unmarshal(r.To, &to) != nil {
			return "", false
		}
		return fmt.Sprintf("タイトルを「%s」に変更", to), true
	case "kind":
		var from, to string
		if json.Unmarshal(r.From, &from) != nil || json.Unmarshal(r.To, &to) != nil {
			return "", false
		}
		return fmt.Sprintf("種類を %s から %s に変更", from, to), true
	case "place":
		return placeText(r)
	case "rank":
		return "", false
	}
	p, ok := propertyOf(r.F, c.Settings)
	if !ok {
		return "", false
	}
	from := item.Display(p, r.From, c.Settings, c.Names, c.Today)
	to := item.Display(p, r.To, c.Settings, c.Names, c.Today)
	if from == to {
		return "", false
	}
	return fmt.Sprintf("%sを %s → %s", p.Name, from, to), true
}

// byPrev は、各行を、その prev に挙がった行よりあとに置いた並びにする。それ以外は、ファイルの並びを保つ。
// 同期のマージは at と id の順に並べ直すので、時計がずれていると、prev の行より先に並ぶ行ができる。
func byPrev(log []change.Record) []change.Record {
	index := make(map[change.ID]int, len(log))
	for i, r := range log {
		index[r.ID] = i
	}
	out := make([]change.Record, 0, len(log))
	state := make([]byte, len(log)) // 0: 未, 1: 途中(循環の検出), 2: 済
	var visit func(i int)
	visit = func(i int) {
		if state[i] != 0 {
			return
		}
		state[i] = 1
		for _, p := range log[i].Prev {
			if j, ok := index[p]; ok {
				visit(j)
			}
		}
		state[i] = 2
		out = append(out, log[i])
	}
	for i := range log {
		visit(i)
	}
	return out
}

// placeText は、親が変わった場所の変更を「親を変更」にする。親が両方ない(Epic)なら、プロジェクトが変わったとき。
// 親が同じでプロジェクトだけが変わる行(プロジェクトが変わる移動で一緒に移る下のもの)と、番号だけが変わる行
// (同期の修復)は出さない。
func placeText(r change.Record) (string, bool) {
	var from, to item.Place
	if json.Unmarshal(r.From, &from) != nil || json.Unmarshal(r.To, &to) != nil {
		return "", false
	}
	changed := from.Project != to.Project
	if from.Parent != nil || to.Parent != nil {
		changed = from.Parent == nil || to.Parent == nil || *from.Parent != *to.Parent
	}
	if !changed {
		return "", false
	}
	return "親を変更", true
}

// propertyOf は、項目名 f が指すプロパティ。組み込みは項目名がキー(= 内部 ID)、追加は fields/<内部 ID>。
// 設定にない(消えた)プロパティは、名前が分からないので ok=false。
func propertyOf(f string, st workspace.Settings) (workspace.Property, bool) {
	id := change.ID(strings.TrimPrefix(f, item.FieldKey("")))
	return st.Property(id)
}
