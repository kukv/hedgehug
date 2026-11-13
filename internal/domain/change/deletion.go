package change

import "time"

// Deletion は、削除済みのエンティティの、削除の印(保存形式「削除・復元・完全に削除」)。
type Deletion struct {
	Op   ID        // 削除した操作の ID。復元はこの op の行をまとめて戻す
	At   time.Time // 削除した日時。保持日数はここから数える
	Root bool      // 利用者が選んだものか(false は、親と一緒に削除されたもの)
}

// DeletionOf は、ログの削除の印を返す。t が delete で f が deleted の行がなければ nil。複数あれば、
// いまの値の決め方(Resolve)と同じ順で最もあとの行(at → by → id)を取るので、行の並びによらない。
// restore の行は見ない。
func DeletionOf(log []Record) *Deletion {
	var last *Record
	for i := range log {
		r := &log[i]
		if r.T == Delete && r.F == DeletedField && (last == nil || later(*r, *last)) {
			last = r
		}
	}
	if last == nil {
		return nil
	}
	return &Deletion{Op: last.Op, At: last.At, Root: last.Root != nil && *last.Root}
}
