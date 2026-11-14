package change

import (
	"encoding/json"
	"slices"
	"strings"
)

// Loss は、このマシンの行 Mine が、同時の別の行 Winner に負けたこと。T は項目の種類で、
// add・remove の項目は Add、ほかは行の種類(Set・Comment・Worklog)。
type Loss struct {
	T      Type
	F      string
	Mine   Record
	Winner Record
}

// Lost は、fresh が真の行(今回の同期で入った行)のせいで、mine が真の行が負けた項目を返す(決定 C4c1-3)。
// 項目ごとに、fresh の行を除いたログの勝ち M と、いまのログの勝ち W を決め、M がこのマシンの行で、
// M がいまも先端で、W が fresh の行で、W と M の値が違うときに負けとする。並びは F、Mine の ID の順。
func Lost(log []Record, fresh func(Record) bool, mine func(Record) bool) []Loss {
	var old []Record
	for _, r := range log {
		if !fresh(r) {
			old = append(old, r)
		}
	}
	gOld, gNew := groupAll(old), groupAll(log)
	seen := map[string]bool{}
	var out []Loss
	for _, r := range log {
		if k := key(r); r.T != Create && lostType(r.T) && !seen[k] {
			seen[k] = true
			if l, ok := lostIn(r, gOld, gNew, fresh, mine); ok {
				out = append(out, l)
			}
		}
	}
	slices.SortFunc(out, func(a, b Loss) int {
		if c := strings.Compare(a.F, b.F); c != 0 {
			return c
		}
		return strings.Compare(string(a.Mine.ID), string(b.Mine.ID))
	})
	return out
}

func lostType(t Type) bool {
	return t == Set || t == Add || t == Remove || t == Comment || t == Worklog
}

// lostIn は、r と同じ項目で、負けた行があるか。
func lostIn(r Record, gOld, gNew groups, fresh, mine func(Record) bool) (Loss, bool) {
	k := key(r)
	m := pickWinner(r.T, tipsOf(itemRows(r.T, gOld, k)))
	tips := tipsOf(itemRows(r.T, gNew, k))
	w := pickWinner(r.T, tips)
	if m == nil || w == nil || m.T == Create || w.T == Create || m.ID == w.ID || !mine(*m) || !fresh(*w) {
		return Loss{}, false
	}
	if !slices.ContainsFunc(tips, func(t Record) bool { return t.ID == m.ID }) || sameValue(*m, *w) {
		return Loss{}, false
	}
	if (r.T == Comment || r.T == Worklog) && Absent(m.To) {
		return Loss{}, false // 削除と変更の重なりは、取り消しの通知の仕事
	}
	t := r.T
	if t == Remove {
		t = Add
	}
	return Loss{T: t, F: r.F, Mine: *m, Winner: *w}, true
}

// itemRows は、項目 k の行。set・add・remove は create の行も含む(Resolve と同じ)。
func itemRows(t Type, g groups, k string) []Record {
	if t == Comment || t == Worklog {
		return g.rows[k]
	}
	return g.of(k)
}

// pickWinner は、先端から勝ちの行を選ぶ。コメントと作業時間は、Current と同じく生きている先端を先にする。
func pickWinner(t Type, tips []Record) *Record {
	if t == Comment || t == Worklog {
		var live []Record
		for _, r := range tips {
			if !Absent(r.To) {
				live = append(live, r)
			}
		}
		if w := winner(live); w != nil {
			return w
		}
	}
	return winner(tips)
}

// sameValue は、2 行の値が同じか。add・remove の項目は、同じ種類なら同じ。
func sameValue(a, b Record) bool {
	if a.T == Add || a.T == Remove {
		return a.T == b.T
	}
	return canonical(a.To) == canonical(b.To)
}

// ListOf は、複数の値を持つ項目 f のいまの値。Resolve の Lists[f] と同じ値と並び(作成の値を含む)。
func ListOf(log []Record, f string) []json.RawMessage {
	valid := filterValid(log, nil)
	c := winner(creates(valid))
	if c == nil {
		return nil
	}
	initial := map[string]json.RawMessage{}
	_ = json.Unmarshal(c.To, &initial)
	return resolveList(valid, groupAll(valid), f, initial[f])
}
