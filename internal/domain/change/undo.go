package change

import "slices"

// Undo は、log の中の操作 op の行を取り消す下書きを、log の行の順に返す(C3a-8)。
// delete は restore、set・add・remove は、その行が項目の先端(Tips に入る)のときだけ逆の変更にする。
// すでに restore された delete と、ほかの種類の行は戻さない。
func Undo(log []Record, op ID) []Draft {
	restored := Restored(log)
	var drafts []Draft
	for _, r := range log {
		if r.Op != op {
			continue
		}
		if d, ok := undoRow(log, r, restored); ok {
			drafts = append(drafts, d)
		}
	}
	return drafts
}

// undoRow は、行 r を取り消す下書き。取り消さない行なら false。
func undoRow(log []Record, r Record, restored func(Record) bool) (Draft, bool) {
	if r.T == Delete {
		return NewRestore(r.F, r.Op), !restored(r)
	}
	if !isTip(log, r) {
		return Draft{}, false
	}
	switch r.T {
	case Set:
		return Draft{T: Set, F: r.F, From: r.To, To: r.From}, true
	case Add:
		return Draft{T: Remove, F: r.F, From: r.To}, true
	case Remove:
		return Draft{T: Add, F: r.F, To: r.From}, true
	}
	return Draft{}, false
}

// isTip は、行 r が、同じ項目の先端かを答える。
func isTip(log []Record, r Record) bool {
	return slices.Contains(Tips(log, r), r.ID)
}
