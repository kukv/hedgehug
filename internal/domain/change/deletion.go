package change

import (
	"encoding/json"
	"time"
)

// Restored は、log の restore の行を見て、delete の行 d が取り消されたかを答える関数を返す。
// 取り消されたのは、同じ f で to が d.Op の restore の行がある delete の行(C3a-7)。
func Restored(log []Record) func(d Record) bool {
	type key struct {
		f  string
		op ID
	}
	seen := map[key]bool{}
	for _, r := range log {
		var op ID
		if r.T == Restore && json.Unmarshal(r.To, &op) == nil {
			seen[key{r.F, op}] = true
		}
	}
	return func(d Record) bool { return d.T == Delete && seen[key{d.F, d.Op}] }
}

// Deletion は、削除済みのエンティティの、削除の印(保存形式「削除・復元・完全に削除」)。
type Deletion struct {
	Op   ID        // 削除した操作の ID。復元はこの op の行をまとめて戻す
	At   time.Time // 削除した日時。保持日数はここから数える
	Root bool      // 利用者が選んだものか(false は、親と一緒に削除されたもの)
}

// DeletionOf は、ログの削除の印を返す。t が delete で f が deleted の行がなければ nil。複数あれば、
// いまの値の決め方(Resolve)と同じ順で最もあとの行(at → by → id)を取るので、行の並びによらない。
// 同じ f で to が自分の op の restore がある行は飛ばす(C3a-7)。
func DeletionOf(log []Record) *Deletion { return DeletionOfField(log, DeletedField) }

// DeletionOfField は、DeletionOf と同じ決め方で、f への削除の印を返す。ワークスペースのログの
// sprints/<ID> のように、deleted でない f への削除に使う。
func DeletionOfField(log []Record, f string) *Deletion {
	var last *Record
	restored := Restored(log)
	for i := range log {
		r := &log[i]
		if r.T == Delete && r.F == f && !restored(*r) && (last == nil || later(*r, *last)) {
			last = r
		}
	}
	if last == nil {
		return nil
	}
	return &Deletion{Op: last.Op, At: last.At, Root: last.Root != nil && *last.Root}
}
