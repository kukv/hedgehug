package saves

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// movedMerge は、片側(taro)がプロジェクト P1 を別の Store へ移し(P1 のフォルダを消し、専用ルール RX を削除し、
// 転送記録を足す)、もう片側(hanako)が row を足した合わせの Pending の結果。共通の祖先は、専用ルール RX(scope が
// P1)と P1 の使わないステータスを持つ。toOther が偽なら、P1 は同じ Store の中で移り、フォルダは残る。
func movedMerge(t *testing.T, row change.Record, toOther bool) (port.ReceivedSave, bool) {
	t.Helper()
	const pLog = "projects/P1/log.jsonl"
	const moved = `{"id":"TR1","op":"OP1","at":"2026-11-12T09:01:00+09:00","by":"taro","entity":"project","target":"P1","old":"HH-P1","new":"WEB-P2","to_store":"S2","to_store_name":"team-b"}` + "\n"
	create := rec("R0", "OP0", t0, change.Create, "")
	create.To = []byte(`{}`)
	scope := rec("R1", "OP0", t0, change.Set, "rules/RX/scope")
	scope.To = []byte(`"P1"`)
	hidden := rec("R2", "OP0", t0, change.Add, "project_rules/P1/hidden")
	pCreate := rec("RP", "OP0", t0, change.Create, "")
	pCreate.To = []byte(`{}`)
	del := rec("R4", "OP1", t0.Add(time.Minute), change.Delete, "rules/RX")
	r := newRepo(t)
	j, known := judged(t)
	c0 := r.save(map[string]string{wsLog: logOf(t, create, scope, hidden), pLog: logOf(t, pCreate)}, "taro", t0, "OP0")
	moveFiles := map[string]string{wsLog: logOf(t, create, scope, hidden, del), "transfers.jsonl": moved}
	if toOther {
		moveFiles[pLog] = ""
	}
	move := r.save(moveFiles, "taro", t0.Add(time.Minute), "OP1")
	r.reset(c0)
	row.By, row.Actor = "hanako", change.User("hanako")
	other := r.save(map[string]string{wsLog: logOf(t, create, scope, hidden, row)}, "hanako", t0.Add(2*time.Minute), "OP2")
	moveFiles[wsLog] = logOf(t, create, scope, hidden, del, row)
	m := r.save(moveFiles, BotName, t0.Add(3*time.Minute), "", move, other)
	r.mark(j, c0)
	s, ok := pending(t, r, m, m, known)[m.String()]
	return s, ok
}

// 片側で P1 を別の Store へ移し、もう片側で P1 のルールを変えた合わせは、MovedRules に {W1, P1}(New はその側の
// 共通の祖先にない 1 行)を持つ。移した側の行(RX の削除)は数えない。
func TestMovedRulesInMerge(t *testing.T) {
	for name, row := range map[string]change.Record{
		"使わないステータスに足した": rec("R3", "OP2", t0.Add(2*time.Minute), change.Add, "project_rules/P1/hidden"),
		"専用ルールの名前を変えた":  rec("R3", "OP2", t0.Add(2*time.Minute), change.Set, "rules/RX/name"),
	} {
		t.Run(name, func(t *testing.T) {
			s, ok := movedMerge(t, row, true)
			if !ok || !s.Merge || len(s.MovedRules) != 1 {
				t.Fatalf("merge = %+v, want one MovedRules", s)
			}
			d := s.MovedRules[0]
			if d.Workspace != "01WS000000000000000000001" || d.Project != "P1" || len(d.New) != 1 || d.New[0].ID != "R3" {
				t.Errorf("MovedRules = %+v", d)
			}
		})
	}
}

// P1 のルールを変えていないか、P1 が同じ Store の中で移った(フォルダが残る)なら、合わせを渡さない。
func TestMovedRulesNone(t *testing.T) {
	for name, c := range map[string]struct {
		row     change.Record
		toOther bool
	}{
		"ルールを変えていない":     {rec("R3", "OP2", t0.Add(2*time.Minute), change.Set, "title"), true},
		"同じ Store の中の移動": {rec("R3", "OP2", t0.Add(2*time.Minute), change.Set, "rules/RX/name"), false},
	} {
		t.Run(name, func(t *testing.T) {
			if s, ok := movedMerge(t, c.row, c.toOther); ok {
				t.Errorf("merge = %+v, want none", s)
			}
		})
	}
}
