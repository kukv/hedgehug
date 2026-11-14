package noticing

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// overwritten は、上書きの通知を作った項目(ログのエンティティと、通知の Field)。1 つの項目に 1 行だけにする。
type overwritten map[change.Ref]map[string]bool

// add は、ref の項目 f を足す。もうあれば false。
func (o overwritten) add(ref change.Ref, f string) bool {
	if o[ref][f] {
		return false
	}
	if o[ref] == nil {
		o[ref] = map[string]bool{}
	}
	o[ref][f] = true
	return true
}

// overwrites は、判定する行に、このマシンの値が負けた項目ごとの「{項目名}が上書きされました」(決定 C4c1-2〜C4c1-11)。
// own は、op がこのマシンで作った保存のものか(nil は「どれも違う」)。own の失敗と、読み取りの失敗(ないもの以外)はエラー。
func overwrites(r port.Reader, b batch, store port.StoreID, me string, own func(change.ID) (bool, error)) ([]port.Notice, error) {
	m := &mineRows{me: me, own: own, ops: map[change.ID]bool{}}
	done := overwritten{}
	var out []port.Notice
	for _, ref := range b.refs {
		if b.movedIn(ref) {
			continue
		}
		ns, err := overwritesOf(r, b, ref, m, done)
		if err != nil {
			return nil, err
		}
		for _, n := range ns {
			n.Kind, n.Store = port.NoticeOverwrite, store
			out = append(out, n)
		}
	}
	return out, nil
}

// overwritesOf は、エンティティ ref の上書きの通知。項目ごとに、最初の(勝った行が最もあとの)負けだけを使う。
func overwritesOf(r port.Reader, b batch, ref change.Ref, m *mineRows, done overwritten) ([]port.Notice, error) {
	losses, err := lossesOf(b, ref, m)
	if err != nil {
		return nil, err
	}
	var out []port.Notice
	for _, l := range losses {
		field := fieldOf(l)
		if !done.add(ref, field) {
			continue
		}
		n, ok, err := overwriteNotice(r, b, ref, field, l)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, n)
		}
	}
	return out, nil
}

// mineRows は、このマシンの行か(決定 C4c1-4)を、op ごとに 1 回だけ own に聞いて決める。own の失敗は err に残す。
type mineRows struct {
	me  string
	own func(change.ID) (bool, error)
	ops map[change.ID]bool
	err error
}

// is は、行 r の by があなたで、実行者が人か自動化で、op がこのマシンの保存のものか。
func (m *mineRows) is(r change.Record) bool {
	if m.own == nil || m.err != nil || !same(r.By, m.me) || (r.Actor.Kind != change.ActorUser && r.Actor.Kind != change.ActorAutomation) {
		return false
	}
	ok, seen := m.ops[r.Op]
	if !seen {
		ok, m.err = m.own(r.Op)
		m.ops[r.Op] = ok
	}
	return ok && m.err == nil
}

// lossesOf は、ref の負けた項目のうち通知するもの(C4c1-5・C4c1-6)。並びは F の順で、同じ F の中は勝った行が
// あとのものから(複数の値を持つ項目は、値ごとの負けを、最もあとの勝ちの 1 行にまとめるため)。
func lossesOf(b batch, ref change.Ref, m *mineRows) ([]change.Loss, error) {
	ls := change.Lost(validRows(ref, b.after(ref)), func(r change.Record) bool { return b.judged[r.ID] }, m.is)
	if m.err != nil {
		return nil, m.err
	}
	ls = slices.DeleteFunc(ls, func(l change.Loss) bool { return !noticed(l) || movedRank(ls, l) })
	slices.SortStableFunc(ls, func(a, c change.Loss) int {
		if d := strings.Compare(a.F, c.F); d != 0 {
			return d
		}
		return laterFirst(a.Winner, c.Winner)
	})
	return ls, nil
}

// validRows は、ref のログのうち、いまの値を決める行(ワークスペースとメンバーの一覧は Resolve の手順 0 の検査を通す)。
func validRows(ref change.Ref, log []change.Record) []change.Record {
	var valid func(change.Record) bool
	switch ref.Kind {
	case change.KindWorkspace:
		valid = workspace.Valid
	case change.KindMembers:
		valid = member.Valid
	default:
		return log
	}
	return slices.DeleteFunc(slices.Clone(log), func(r change.Record) bool { return !valid(r) })
}

// noticed は、負けを通知するか。勝った行が GitHub の操作か同期の修復、項目がウォッチ中の人か PR のリンクの状態なら
// 作らない(C4c1-5)。
func noticed(l change.Loss) bool {
	k := l.Winner.Actor.Kind
	return k != change.ActorGitHub && k != change.ActorSync && l.F != "watchers" && (l.T != change.Set || !strings.HasPrefix(l.F, "links/"))
}

// movedRank は、l が順位の負けで、同じ op の行に場所も負けているか(移動は場所の 1 行だけにする。C4c1-6)。
func movedRank(ls []change.Loss, l change.Loss) bool {
	return l.T == change.Set && l.F == "rank" && slices.ContainsFunc(ls, func(o change.Loss) bool {
		return o.T == change.Set && o.F == "place" && o.Winner.Op == l.Winner.Op
	})
}

// laterFirst は、a が c よりあとの行なら負(時刻、ID の順に比べる)。
func laterFirst(a, c change.Record) int {
	if !a.At.Equal(c.At) {
		return c.At.Compare(a.At)
	}
	return strings.Compare(string(c.ID), string(a.ID))
}

// fieldOf は、通知の Field(ログの f。コメント・作業時間は comment:<ID>・worklog:<ID>。C4c1-10)。
func fieldOf(l change.Loss) string {
	switch l.T {
	case change.Comment:
		return commentField + l.F
	case change.Worklog:
		return worklogField + l.F
	}
	return l.F
}

// overwriteNotice は、ref の項目 field の負け l の通知。項目名を決められない項目と、ないエンティティは false。
func overwriteNotice(r port.Reader, b batch, ref change.Ref, field string, l change.Loss) (port.Notice, bool, error) {
	name, ok, err := fieldName(r, ref, field)
	if err != nil || !ok {
		return port.Notice{}, false, err
	}
	n, ok, err := overwriteTarget(r, b, ref, field)
	if err != nil || !ok {
		return port.Notice{}, false, err
	}
	n.Text, n.Sender, n.Field = name+"が上書きされました", sender(l.Winner), field
	n.Mine, n.Kept = l.Mine.To, l.Winner.To
	if l.T == change.Add {
		n.Mine, n.Kept = listOf(validRows(ref, b.before(ref)), l.F), listOf(validRows(ref, b.after(ref)), l.F)
	}
	n.Source = "overwrite:" + string(ref.ID) + ":" + field + ":" + string(l.Winner.ID)
	return n, true, nil
}

// listOf は、複数の値を持つ項目 f の値の、ログに書く形の配列(値の JSON の辞書順。空は [])。
func listOf(log []change.Record, f string) json.RawMessage {
	vs := change.ListOf(log, f)
	if vs == nil {
		vs = []json.RawMessage{}
	}
	out, _ := json.Marshal(vs) // 値はログから読んだ JSON なので失敗しない
	return out
}

// overwriteTarget は、通知の下地(C4c1-8)。アイテム・プロジェクトはそれを指し、ワークスペースは開く区画を指す
// (プロジェクト別のルールはプロジェクト)。メンバーの一覧はどれも指さない。
func overwriteTarget(r port.Reader, b batch, ref change.Ref, field string) (port.Notice, bool, error) {
	switch ref.Kind {
	case change.KindItem, change.KindProject:
		e, ok, err := entityOf(r, b, ref)
		return e.base, ok, err
	case change.KindWorkspace:
		if id, ok := projectRuleOf(field); ok {
			e, found, err := entityOf(r, b, change.Ref{Kind: change.KindProject, ID: id})
			if !found {
				e.base = port.Notice{Workspace: ref.ID, Project: id} // 引けないプロジェクトも内部 ID で指す
			}
			return e.base, err == nil, err
		}
		return port.Notice{Workspace: ref.ID, Section: section(field)}, true, nil
	}
	return port.Notice{}, true, nil
}
