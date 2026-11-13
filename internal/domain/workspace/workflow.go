package workspace

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// 遷移の制約(03「遷移の制約」)。From のステータスごとの項目 workflow/<From> に、
// 許可する To を add・remove で持つ(B3-1)。

const workflowPrefix = "workflow/"

// WorkflowField は、From の行の項目名。
func WorkflowField(from change.ID) string { return workflowPrefix + string(from) }

func (b *builder) setWorkflow(f string, list []json.RawMessage) {
	from, ok := strings.CutPrefix(f, workflowPrefix)
	if !ok {
		return
	}
	for _, raw := range list {
		var to change.ID
		if json.Unmarshal(raw, &to) != nil {
			continue
		}
		if b.acc.Workflow == nil {
			b.acc.Workflow = map[change.ID][]change.ID{}
		}
		b.acc.Workflow[change.ID(from)] = append(b.acc.Workflow[change.ID(from)], to)
	}
}

// Allowed は、from から変更できる To を、ステータスの並びで返す。いまあるステータスだけを数え、
// 1 つもなければ制限なし(false)。
func (s Settings) Allowed(from change.ID) ([]change.ID, bool) {
	set := s.Workflow[from]
	var to []change.ID
	for _, v := range s.Statuses {
		if v.ID != from && slices.Contains(set, v.ID) {
			to = append(to, v.ID)
		}
	}
	return to, len(to) > 0
}

// CheckTransition は、from から to への変更を遷移の制約で検査する(B3-2)。同じステータス、
// from が空、from がいまの一覧にない(削除された値のまま。02「ステータスの変更」)、制限のない行は通す。
func CheckTransition(s Settings, from, to change.ID) error {
	if from == "" || from == to || !hasValue(s.Statuses, from) {
		return nil
	}
	allowed, restricted := s.Allowed(from)
	if !restricted || slices.Contains(allowed, to) {
		return nil
	}
	names := make([]string, 0, len(allowed))
	for _, id := range allowed {
		names = append(names, StatusName(s, id))
	}
	return refusal.New(fmt.Sprintf("ワークフローの制約: %s から %s には変更できません(%s のみ)",
		StatusName(s, from), StatusName(s, to), strings.Join(names, "・")))
}

// WorkflowDeleteDrafts は、ステータス id を表から外す行(B3-15)。id の行の許可先と、ほかの行の id。
func WorkflowDeleteDrafts(s Settings, id change.ID) []change.Draft {
	var ds []change.Draft
	for _, to := range s.Workflow[id] {
		ds = append(ds, change.NewRemove(WorkflowField(id), to))
	}
	froms := make([]change.ID, 0, len(s.Workflow))
	for from := range s.Workflow {
		froms = append(froms, from)
	}
	slices.Sort(froms)
	for _, from := range froms {
		if from != id && slices.Contains(s.Workflow[from], id) {
			ds = append(ds, change.NewRemove(WorkflowField(from), id))
		}
	}
	return ds
}
