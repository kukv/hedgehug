package change

import (
	"bytes"
	"encoding/json"
	"slices"
)

// Absent は、値がないことを表すか(to が欠けているか null)。コメントと作業時間の削除の行がこれになる。
func Absent(v json.RawMessage) bool {
	s := bytes.TrimSpace(v)
	return len(s) == 0 || bytes.Equal(s, []byte("null"))
}

// rowsOf は、t と f が同じ行(1 つのコメントや作業時間の行の連なり)。create は含まない。
func rowsOf(log []Record, t Type, f string) []Record {
	var rows []Record
	for _, r := range log {
		if r.T == t && r.F == f {
			rows = append(rows, r)
		}
	}
	return rows
}

// Current は、t と f が同じ行から、いまの値を表す行を返す。先端(同じ項目の行の prev に挙がっていない行)のうち
// alive が真のものがあれば、その中で最もあとの行を選ぶ。一方が削除し他方が変更したときに変更を残すため
// (06「同時に編集したとき」)。alive が真のものがなければ、先端で最もあとの行(削除)を返す。
// 行がないか、prev が輪になって先端がなければ false。結果は log の行の並びによらない。
func Current(log []Record, t Type, f string, alive func(Record) bool) (Record, bool) {
	tips := tipsOf(rowsOf(log, t, f))
	var live []Record
	for _, r := range tips {
		if alive(r) {
			live = append(live, r)
		}
	}
	if w := winner(live); w != nil {
		return *w, true
	}
	if w := winner(tips); w != nil {
		return *w, true
	}
	return Record{}, false
}

// First は、t と f が同じ行のうち、最初に書いた行(投稿・記録)を返す。prev が同じ項目の行を指さない行のうち、
// at が最も古いもの(同じなら id が小さいもの)。prev が輪になっていれば、すべての行から選ぶ。
// 時計がずれて、あとの行の at が前になっていても、prev で決まる。
func First(log []Record, t Type, f string) (Record, bool) {
	rows := rowsOf(log, t, f)
	if len(rows) == 0 {
		return Record{}, false
	}
	ids := map[ID]bool{}
	for _, r := range rows {
		ids[r.ID] = true
	}
	var roots []Record
	for _, r := range rows {
		if !slices.ContainsFunc(r.Prev, func(p ID) bool { return ids[p] }) {
			roots = append(roots, r)
		}
	}
	if len(roots) == 0 {
		roots = rows
	}
	first := roots[0]
	for _, r := range roots[1:] {
		if r.At.Before(first.At) || (r.At.Equal(first.At) && r.ID < first.ID) {
			first = r
		}
	}
	return first, true
}
