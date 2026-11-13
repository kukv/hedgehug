package history

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ItemRows は、アイテムの履歴を新しい順で返す(02「履歴」の表)。出すのは、作成・説明の変更・タイトル・種類・親・
// プロパティの変更(関連システムと複数選択の追加・取り除きを含む)と、実行者「GitHub」のリンクの値の変更・改名・自動リンク(D3-12)と、
// 別のワークスペースからの移動(A5a-17)だけで、コメント・作業時間・関連・ウォッチ・削除と、知らない種類の行は出さない。
// 値の名前は、行を書いたときにアイテムがいたワークスペースの定義で引く。
func ItemRows(log []change.Record, c Context) []Row {
	log = byPrev(log)
	t := newTrail(log)
	var rows []Row
	for i, r := range log {
		if c.Hidden[r.Op] {
			continue
		}
		if text, ok := itemText(log, i, c, t); ok {
			rows = append(rows, Row{Who: Who(r.Actor, c.Names), Text: text, At: r.At})
		}
	}
	return newestFirst(rows)
}

func itemText(log []change.Record, i int, c Context, t trail) (string, bool) {
	r := log[i]
	if r.Actor.Kind == change.ActorGitHub && (r.F == itemlink.LinksField || strings.HasPrefix(r.F, itemlink.LinksField+"/")) {
		return linkText(log, i)
	}
	switch r.T {
	case change.Create:
		return "作成", true
	case change.Desc:
		return "説明を変更", true
	case change.MovedIn:
		return movedInText(r)
	case change.Set:
		if changesWorkspace(r) {
			return movedText(log, r, c) // 「親を変更」には出さない
		}
		if r.F == "place" && sameOpSetsKind(log, r.Op) {
			return "", false // Step から昇格した行は「種類を … に変更」の 1 行だけにする
		}
		from, to := t.vocabs(i, r, c)
		return setText(r, c, from, to)
	case change.Add, change.Remove:
		from, to := t.vocabs(i, r, c)
		return listText(log, i, c, t, from, to)
	}
	return "", false
}

// sameOpSetsKind は、op に種類を変える行があるか。
func sameOpSetsKind(log []change.Record, op change.ID) bool {
	return slices.ContainsFunc(log, func(r change.Record) bool { return r.Op == op && r.T == change.Set && r.F == "kind" })
}

// listText は、関連システムと複数選択の追加・取り除きを「{プロパティ名}を {旧} → {新}」にする。旧は log[:i]、
// 新は log[:i+1] から決めた一覧なので、同時の変更も、prev のグラフの規則で決まった結果になる。
// プロパティでない項目(関連・ウォッチなど)と、一覧が変わらない行は出さない。旧は from、新は to の定義で引く。
// ワークスペースが変わる操作(移動の引き直し)では、取り除き・追加を 1 行にまとめ、その操作の最後の行に、
// 操作の前と後の一覧で出す(名前が同じなら出さない。set の行と同じ)。
func listText(log []change.Record, i int, c Context, t trail, fromV, toV vocab) (string, bool) {
	r := log[i]
	pf, p, ok := propertyIn(r.F, fromV, toV)
	if !ok || (p.Type != workspace.TypeSystems && p.Type != workspace.TypeMulti) {
		return "", false
	}
	start, moving := i, t.moving(r.Op)
	if moving {
		if slices.ContainsFunc(log[i+1:], func(x change.Record) bool {
			return x.Op == r.Op && x.F == r.F && (x.T == change.Add || x.T == change.Remove)
		}) {
			return "", false
		}
		start = slices.IndexFunc(log, func(x change.Record) bool { return x.Op == r.Op })
	}
	before, okBefore := item.FromLog(log[:start])
	after, okAfter := item.FromLog(log[:i+1])
	if !okBefore || !okAfter {
		return "", false
	}
	from, to := item.Current(before, p), item.Current(after, p)
	fromText, toText := fromV.display(pf, from, c), toV.display(p, to, c)
	if bytes.Equal(from, to) || (moving && fromText == toText) {
		return "", false
	}
	return fmt.Sprintf("%sを %s → %s", p.Name, fromText, toText), true
}

// propertyIn は、項目名 f が指すプロパティの、from と to の定義でのもの。片方にしかなければ、両方ともそれにする。
func propertyIn(f string, fromV, toV vocab) (workspace.Property, workspace.Property, bool) {
	pf, okFrom := fromV.property(f)
	pt, okTo := toV.property(f)
	switch {
	case okFrom && okTo:
		return pf, pt, true
	case okTo:
		return pt, pt, true
	}
	return pf, pf, okFrom
}

func setText(r change.Record, c Context, fromV, toV vocab) (string, bool) {
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
	pf, p, ok := propertyIn(r.F, fromV, toV)
	if !ok {
		return "", false
	}
	from := fromV.display(pf, r.From, c)
	to := toV.display(p, r.To, c)
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
