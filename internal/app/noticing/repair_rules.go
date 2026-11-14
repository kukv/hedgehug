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
// まとめ、P が保存のあとでこの Store になく、外したものを入れた人にあなたがいれば、P ごとに 1 件。
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
		in, err := inStore(r, b, p)
		if err != nil {
			return nil, err
		}
		if in { // そのワークスペースにある(C5b2 の形)か、同じ Store の別のワークスペースへ移った
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

// inStore は、プロジェクト id(削除済みを含む)をいまこの Store で読めるか。読めるなら、別の Store への古い転送記録が
// あっても原因に数えない(06「同期の修復」の (9): 同じ Store の中の移動は知らせない)。
func inStore(r port.Reader, b batch, id project.ID) (bool, error) {
	log, err := logOf(r, b, change.Ref{Kind: change.KindProject, ID: id})
	if err != nil {
		return false, err
	}
	_, ok := project.FromLog(log)
	return ok, nil
}

// putter は、修復の行 row が外したものを入れた人(人の行だけ。C5b1-8・C5b1-15)。log は修復の直前のログ。
// 移動の行と、修復 (1) が取り消しで書いた行(実行者が同期)では、移動した人を入れた人にしない。
//   - remove: その値を足した行を at の順にたどり、人の add なら入れた人をその人に、同期の add なら空に戻す(自動化などの
//     行は飛ばす)。
//   - null にした template_default: その項目を、row の from の値にした人の set のうち最後の行の人。
//   - 専用ルールの削除: そのルールの行(delete・restore を除く)を最後に保存した人。
//
// 決められなければ空。
func putter(log []change.Record, row change.Record) string {
	switch {
	case row.T == change.Remove:
		return lastAdder(rowsWhere(log, func(c change.Record) bool {
			return c.T == change.Add && c.F == row.F && sameValue(c.To, row.From)
		}))
	case row.T == change.Set && change.Absent(row.To) && strings.Contains(row.F, "/template_default/"):
		return lastSaver(rowsWhere(log, func(c change.Record) bool {
			return c.T == change.Set && c.F == row.F && sameValue(c.To, row.From)
		}), "")
	case row.T == change.Delete && strings.HasPrefix(row.F, "rules/"):
		return lastSaver(rowsWhere(log, func(c change.Record) bool {
			return c.T != change.Delete && c.T != change.Restore && (c.F == row.F || strings.HasPrefix(c.F, row.F+"/"))
		}), "")
	}
	return ""
}

// lastAdder は、add の行 adds を at の順(同じなら ID の順)にたどった、最後の人の add の人。そのあとに同期の add が
// あれば空。
func lastAdder(adds []change.Record) string {
	slices.SortStableFunc(adds, func(a, b change.Record) int {
		if c := a.At.Compare(b.At); c != 0 {
			return c
		}
		return strings.Compare(string(a.ID), string(b.ID))
	})
	who := ""
	for _, c := range adds {
		switch c.Actor.Kind {
		case change.ActorUser:
			who = c.By
		case change.ActorSync:
			who = ""
		}
	}
	return who
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

// droppedRules は、i 番目が合わせなら、合わせで取り除かれた、完全に削除したプロジェクトのルールの行のうち、共通の
// 祖先にない行(New)にあなたの行があるプロジェクトごとの通知(06「同期の修復」。C5b1-9・C5b1-10)。取り除く前の
// ログ(Log)だけにある行は数えない。{ID} は purged.jsonl の display で、空なら作らない。鍵は、合わせでプロジェクトが
// 消えたときと同じ repair:purged:{P}。
func droppedRules(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	s := b.saves[i]
	if !s.Merge {
		return nil, nil
	}
	var out []port.Notice
	for _, d := range s.DroppedRules {
		if !wroteBy(d.New, me) {
			continue
		}
		cs, err := b.causes(r)
		if err != nil {
			return nil, err
		}
		c, ok := cs.of(d.Project)
		if !ok || c.purged == nil || oneLine(c.purged.Display) == "" {
			continue
		}
		display := c.purged.Display
		out = append(out, port.Notice{
			Workspace: d.Workspace, Project: d.Project, Display: display,
			Text: purgedText(display), Source: "repair:purged:" + string(d.Project),
		})
	}
	return out, nil
}

// movedRules は、i 番目が合わせなら、もう片側の操作で別の Store へ移したプロジェクトのルールに、その側で書いた行に
// あなたの行がある、プロジェクトごとの移動の通知(06「プロジェクトがなくなったときのルール」。C5b1-17)。本文・鍵・
// Workspace・Project・Display は修復 (9) の通知(orphanNotice)と同じなので、同じプロジェクトで両方が出ても受信箱は
// 1 行。いまこの Store で読めるプロジェクトなら作らない。
func movedRules(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	s := b.saves[i]
	if !s.Merge {
		return nil, nil
	}
	var out []port.Notice
	for _, d := range s.MovedRules {
		if !wroteBy(d.New, me) {
			continue
		}
		in, err := inStore(r, b, d.Project)
		if err != nil {
			return nil, err
		}
		if in {
			continue
		}
		n, ok, err := orphanNotice(r, b, d.Workspace, d.Project)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, n)
		}
	}
	return out, nil
}
