package noticing

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// orphanRules は、i 番目の修復の保存が、ワークスペースにないプロジェクトのルールを外した修復 (9) なら、その通知
// (06「同期の修復」。C5b1-7・C5b1-8・C5b1-10・C5b1-11)。ワークスペースのログの行を持ち主のプロジェクト P ごとに
// まとめ、P が保存のあとでそのワークスペースになく、外したものを入れた人にあなたがいれば、P ごとに 1 件。
func orphanRules(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	if !b.saves[i].Repair {
		return nil, nil
	}
	var out []port.Notice
	for _, e := range b.saves[i].Entities {
		if e.Ref.Kind != change.KindWorkspace {
			continue
		}
		ns, err := orphansIn(r, b, i, e, me)
		if err != nil {
			return nil, err
		}
		out = append(out, ns...)
	}
	return out, nil
}

// orphansIn は、i 番目の保存のワークスペース e の行のうち、修復 (9) の行の通知。
func orphansIn(r port.Reader, b batch, i int, e port.EntityRows, me string) ([]port.Notice, error) {
	var out []port.Notice
	for _, p := range ownersFor(b.beforeSave(e.Ref, i), e.Rows, me) {
		in, err := inWorkspace(r, b, e.Ref.ID, p)
		if err != nil {
			return nil, err
		}
		if in {
			continue
		}
		n, ok, err := orphanNotice(r, b, e.Ref.ID, p)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, n)
		}
	}
	return out, nil
}

// ownersFor は、rows にルールの行があるプロジェクトのうち、外したものを入れた人に me がいるもの(最初に出た順)。
// 持ち主と入れた人は、修復の直前のログ before で決める。
func ownersFor(before, rows []change.Record, me string) []project.ID {
	scopes := repair.RuleScopes(before)
	var out []project.ID
	for _, row := range rows {
		p, ok := repair.RuleOwner(row, scopes)
		if ok && !slices.Contains(out, p) && same(putter(before, row), me) {
			out = append(out, p)
		}
	}
	return out
}

// inWorkspace は、プロジェクト id(削除済みを含む)がいまワークスペース ws にあるか。
func inWorkspace(r port.Reader, b batch, ws workspace.ID, id project.ID) (bool, error) {
	log, err := logOf(r, b, change.Ref{Kind: change.KindProject, ID: id})
	if err != nil {
		return false, err
	}
	p, ok := project.FromLog(log)
	return ok && p.Place.Workspace == ws, nil
}

// putter は、修復の行 row が外したものを入れた人(人の行だけ。C5b1-8)。log は修復の直前のログ。
// remove はその値を最後に足した人、null にした template_default はその項目を最後に保存した人、専用ルールの削除は
// そのルールの行を最後に保存した人。決められなければ空。
func putter(log []change.Record, row change.Record) string {
	switch {
	case row.T == change.Remove:
		return lastSaver(rowsWhere(log, func(c change.Record) bool {
			return c.T == change.Add && c.F == row.F && sameValue(c.To, row.From)
		}), "")
	case row.T == change.Set && change.Absent(row.To) && strings.Contains(row.F, "/template_default/"):
		return lastSaver(log, row.F)
	case row.T == change.Delete && strings.HasPrefix(row.F, "rules/"):
		return lastSaver(rowsWhere(log, func(c change.Record) bool {
			return c.F == row.F || strings.HasPrefix(c.F, row.F+"/")
		}), "")
	}
	return ""
}

// rowsWhere は、log のうち keep が真の行。
func rowsWhere(log []change.Record, keep func(change.Record) bool) []change.Record {
	var out []change.Record
	for _, c := range log {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

// sameValue は、JSON の値 a と b が同じか(空白や並びの違いは問わない)。
func sameValue(a, b json.RawMessage) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

// orphanNotice は、ワークスペース ws にあった、ワークスペースにないプロジェクト p のルールを外した通知。原因が
// 完全な削除なら鍵は repair:purged:{P}、別の Store への移動なら repair:moved:{P}:{転送記録の ID}(合わせで p が
// 消えたときの通知と同じ鍵)。原因がないか、記録の ID が空なら false。
func orphanNotice(r port.Reader, b batch, ws workspace.ID, p project.ID) (port.Notice, bool, error) {
	cs, err := b.causes(r)
	if err != nil {
		return port.Notice{}, false, err
	}
	c, ok := cs.of(p)
	if !ok {
		return port.Notice{}, false, nil
	}
	c.at = change.Ref{Kind: change.KindProject, ID: p}
	if c.text = causeText(c, nil); oneLine(c.text) == "" {
		return port.Notice{}, false, nil
	}
	n := port.Notice{Workspace: ws, Project: p, Display: c.text}
	if c.purged != nil {
		n.Text, n.Source = purgedText(c.text), "repair:purged:"+string(p)
		return n, true, nil
	}
	who, err := nameOf(r, c.moved.By)
	if err != nil {
		return port.Notice{}, false, err
	}
	n.Text, n.Source = movedText(b, c, who), "repair:moved:"+string(p)+":"+string(c.moved.ID)
	return n, true, nil
}
