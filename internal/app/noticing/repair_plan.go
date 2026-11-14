package noticing

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

const sprintsPrefix = "sprints/"

// activeSprints は、i 番目の修復の保存の、実行中のスプリントを計画中に戻した修復 (6) の通知(06「同期の修復」)。
// ワークスペースのログの set sprints/<ID>/state(active から planned)の行ごとに 1 件。宛先は開始の行(修復の直前の
// その欄のいまの値の行)の by(C5a-9・C5a-10)。
func activeSprints(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	if !b.saves[i].Repair {
		return nil, nil
	}
	var out []port.Notice
	for _, e := range b.saves[i].Entities {
		if e.Ref.Kind != change.KindWorkspace {
			continue
		}
		for _, row := range e.Rows {
			if !stoppedRow(row) {
				continue
			}
			before, err := beforeRead(r, b, e.Ref, i)
			if err != nil {
				return nil, err
			}
			if n, ok := activeNotice(e.Ref, row, before, me); ok {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// stoppedRow は、row が set sprints/<ID>/state の active から planned の行か。
func stoppedRow(row change.Record) bool {
	var from, to planning.SprintState
	return row.T == change.Set && strings.HasPrefix(row.F, sprintsPrefix) && strings.HasSuffix(row.F, "/state") &&
		json.Unmarshal(row.From, &from) == nil && json.Unmarshal(row.To, &to) == nil &&
		from == planning.Active && to == planning.Planned
}

// activeNotice は、ワークスペース ws の修復の行 row の通知。開始の行は、修復の直前のログのうち人の行だけで求める
// (2 台が同じ修復をしたとき、先に届いた修復の行ではなく、同じ開始の行を鍵にする)。宛先があなたでなければ false。
func activeNotice(ws change.Ref, row change.Record, before []change.Record, me string) (port.Notice, bool) {
	people := slices.DeleteFunc(slices.Clone(before), func(c change.Record) bool { return c.Actor.Kind != change.ActorUser })
	start, ok := change.Current(people, change.Set, row.F, func(change.Record) bool { return true })
	if !ok || !same(start.By, me) {
		return port.Notice{}, false
	}
	id, _ := planning.PlanTarget(row)
	name := ""
	for _, s := range planning.FromLog(before).Sprints {
		if s.ID == id {
			name = s.Name
		}
	}
	return port.Notice{
		Workspace: ws.ID, Section: "sprints",
		Text:   "実行中のスプリントが 2 つになったため、「" + oneLine(name) + "」を計画中に戻しました",
		Source: "repair:active:" + string(id) + ":" + string(start.ID),
	}, true
}

// planRefs は、i 番目の修復の保存の、アイテムのスプリント・リリースを空にした修復 (7)・(8) の通知(06「同期の修復」)。
// set sprint・set release(to null)の行ごとに 1 件。同じ保存に place の行があれば修復 (5) の移動なので作らない。
// 宛先は修復の直前にその欄を最後に保存した人(C5a-9・C5a-10)。
func planRefs(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	s := b.saves[i]
	if !s.Repair || hasPlaceRow(s) {
		return nil, nil
	}
	var out []port.Notice
	for _, e := range s.Entities {
		for _, row := range e.Rows {
			target, ok := clearedTarget(e.Ref, row)
			if !ok {
				continue
			}
			n, ok, err := planRefNotice(r, b, i, e.Ref, row.F, target, me)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// clearedTarget は、row がアイテム ref の set sprint・set release(to null)の行なら、その前の値。
func clearedTarget(ref change.Ref, row change.Record) (change.ID, bool) {
	var target change.ID
	var to *change.ID
	cleared := len(row.To) == 0 || (json.Unmarshal(row.To, &to) == nil && to == nil)
	if ref.Kind != change.KindItem || row.T != change.Set || (row.F != item.SprintField && row.F != item.ReleaseField) ||
		!cleared || json.Unmarshal(row.From, &target) != nil {
		return "", false
	}
	return target, true
}

// hasPlaceRow は、保存 s に place の行があるか。
func hasPlaceRow(s port.ReceivedSave) bool {
	return slices.ContainsFunc(s.Entities, func(e port.EntityRows) bool {
		return slices.ContainsFunc(e.Rows, func(row change.Record) bool { return row.F == "place" })
	})
}

// planRefNotice は、アイテム ref の欄 field(修復の直前の値 target)を空にした修復の通知。宛先があなたでないか、
// アイテムを読めなければ false。
func planRefNotice(r port.Reader, b batch, i int, ref change.Ref, field string, target change.ID, me string) (port.Notice, bool, error) {
	before := b.beforeSave(ref, i)
	saved := lastSaved(before, field)
	it, ok := item.FromLog(before)
	if saved == nil || !ok || !same(saved.By, me) {
		return port.Notice{}, false, nil
	}
	self, ok, err := entityOf(r, b, ref)
	if !ok {
		return port.Notice{}, false, err
	}
	plan, err := planOf(r, b, change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
	if err != nil {
		return port.Notice{}, false, err
	}
	n := self.base
	if name, ok := closedSprint(plan, field, target); ok {
		n.Text = "スプリント「" + name + "」は完了しているため、" + withParticle(self.text, "を") + "バックログに戻しました"
		n.Source = "repair:closed:" + string(ref.ID) + ":" + string(saved.ID)
		return n, true, nil
	}
	if n.Text, err = danglingText(r, b, field, target, self.text); err != nil {
		return port.Notice{}, false, err
	}
	n.Source = "repair:planref:" + string(ref.ID) + ":" + field + ":" + string(saved.ID)
	return n, true, nil
}

// planOf は、ワークスペース ref のいまの計画(削除済みを含む)。ないものは空。
func planOf(r port.Reader, b batch, ref change.Ref) (planning.Plan, error) {
	log, err := logOf(r, b, ref)
	return planning.FromLog(log), err
}

// closedSprint は、field がスプリントで、target が plan の完了したスプリントなら、その名前。
func closedSprint(plan planning.Plan, field string, target change.ID) (string, bool) {
	if field != item.SprintField {
		return "", false
	}
	for _, s := range plan.Sprints {
		if s.ID == target && s.State == planning.Closed {
			return oneLine(s.Name), true
		}
	}
	return "", false
}

// danglingText は、存在しないスプリント・リリース target の修復 (8) の本文(C5a-6・C5a-10)。purged.jsonl にあれば
// 完全な削除でその display、どれかのワークスペースの計画にあればワークスペース違いでその名前、なければ完全な削除。
func danglingText(r port.Reader, b batch, field string, target change.ID, self string) (string, error) {
	kind, name, action := "スプリント", "削除されたスプリント", withParticle(self, "を")+"バックログに戻しました"
	if field == item.ReleaseField {
		kind, name, action = "リリース", "削除されたリリース", withParticle(self, "の")+"リリースを外しました"
	}
	cs, err := b.causes(r)
	if err != nil {
		return "", err
	}
	for _, p := range cs.purged {
		if p.Target == target && (p.Entity == "sprint" || p.Entity == "release") {
			return kind + "「" + oneLine(p.Display) + "」は完全に削除されたため、" + action, nil
		}
	}
	other, ok, err := planName(r, b, field, target)
	if err != nil {
		return "", err
	}
	if ok {
		return kind + "「" + other + "」は " + withParticle(self, "の") + "ワークスペースにないため、" + action, nil
	}
	return kind + "「" + name + "」は完全に削除されたため、" + action, nil
}

// planName は、どれかのワークスペースの計画(削除済みを含む)にある target の名前。なければ false。
func planName(r port.Reader, b batch, field string, target change.ID) (string, bool, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return "", false, notFoundIsSkip(err)
	}
	for _, ws := range wss {
		plan, err := planOf(r, b, change.Ref{Kind: change.KindWorkspace, ID: ws.ID})
		if err != nil {
			return "", false, err
		}
		if name, ok := nameIn(plan, field, target); ok {
			return name, true, nil
		}
	}
	return "", false, nil
}

// nameIn は、plan の、field がスプリントならスプリント、ほかはリリースのうち、target の名前。なければ false。
func nameIn(plan planning.Plan, field string, target change.ID) (string, bool) {
	if field == item.SprintField {
		i := slices.IndexFunc(plan.Sprints, func(s planning.Sprint) bool { return s.ID == target })
		if i < 0 {
			return "", false
		}
		return oneLine(plan.Sprints[i].Name), true
	}
	i := slices.IndexFunc(plan.Releases, func(rl planning.Release) bool { return rl.ID == target })
	if i < 0 {
		return "", false
	}
	return oneLine(plan.Releases[i].Name), true
}
