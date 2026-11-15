package noticing

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// duplicates は、i 番目の修復の保存が、重なった名前・接頭辞・キーを直した修復 (11) なら、その通知(06「同期の修復」)。
// 見分け方は決定 C5c1-9、宛先は C5c1-10、鍵は C5c1-11、開く先は C5c1-12。直した行ごとに 1 件。
func duplicates(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	s := b.saves[i]
	if !s.Repair || hasRestoreRow(s) {
		return nil, nil
	}
	var out []port.Notice
	for _, e := range s.Entities {
		for _, row := range e.Rows {
			if row.T != change.Set || !namingField(e.Ref.Kind, row.F) {
				continue
			}
			n, ok, err := duplicateNotice(r, b, i, e.Ref, row, me)
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

// hasRestoreRow は、保存 s が restore の行を含むか(削除の取り消しの保存。決定 C5c1-9)。
func hasRestoreRow(s port.ReceivedSave) bool {
	return slices.ContainsFunc(s.Entities, func(e port.EntityRows) bool {
		return slices.ContainsFunc(e.Rows, func(row change.Record) bool { return row.T == change.Restore })
	})
}

// namedLists は、ワークスペースのログで、<一覧>/<ID>/name が名前の項目になる一覧。
var namedLists = []string{"properties", "statuses", "priorities", "systems", "templates", "rules", "sprints", "releases"}

// namingField は、kind のログの項目 f が、修復 (11) が直す項目(決定 C5c1-3)か。
func namingField(kind change.Kind, f string) bool {
	switch kind {
	case change.KindProject:
		return f == "title"
	case change.KindWorkspace:
	default:
		return false
	}
	parts := strings.Split(f, "/")
	switch len(parts) {
	case 1:
		return f == "name" || f == "prefix"
	case 3:
		return (parts[2] == "name" && slices.Contains(namedLists, parts[0])) || (parts[0] == "properties" && parts[2] == "key")
	case 4:
		return parts[0] == "choices" && parts[3] == "name"
	}
	return false
}

// duplicateNotice は、ref の修復の行 row の通知。名前を決めた行が人の行で、その人があなたでなければ false。
func duplicateNotice(r port.Reader, b batch, i int, ref change.Ref, row change.Record, me string) (port.Notice, bool, error) {
	log, err := logOf(r, b, ref)
	if err != nil {
		return port.Notice{}, false, err
	}
	naming, ok := namingRowOf(log, b.without(log, i), row)
	if !ok || naming.Actor.Kind != change.ActorUser || !same(naming.By, me) {
		return port.Notice{}, false, nil
	}
	n := port.Notice{Workspace: ref.ID, Section: section(row.F)}
	if ref.Kind == change.KindProject {
		e, ok, err := entityOf(r, b, ref)
		if !ok {
			return port.Notice{}, false, err
		}
		n = e.base
	} else if id, ok := strings.CutPrefix(row.F, "rules/"); ok {
		n.Rule = change.ID(strings.TrimSuffix(id, "/name"))
	}
	n.Text = "2 台で同時に「" + oneLine(idOf(row.From)) + "」が作られたため、あとから届いた方を「" + oneLine(idOf(row.To)) + "」に変更しました"
	n.Source = "repair:duplicate:" + string(ref.ID) + ":" + row.F + ":" + string(naming.ID)
	return n, true, nil
}

// namingRowOf は、修復の直前のログ before で、修復の行 row の項目の値を決めた行(決定 C5c1-3)。
// 同じ直しを書いた人でない set の行(2 台が同じ修復をしたときの、先に届いた修復の行)は飛ばして、いまのログ log の
// 行の prev をたどる(startRow と同じ)。set の行がなくなれば、before の作成の行。なければ false。
func namingRowOf(log, before []change.Record, row change.Record) (change.Record, bool) {
	creates := rowsWhere(before, func(c change.Record) bool { return c.T == change.Create })
	cur, ok := repair.NamingRow(before, row.F)
	seen := map[change.ID]bool{}
	for ok && cur.T == change.Set && cur.Actor.Kind != change.ActorUser && sameValue(cur.To, row.To) && !seen[cur.ID] {
		seen[cur.ID] = true
		prev := rowsWhere(log, func(c change.Record) bool { return slices.Contains(cur.Prev, c.ID) })
		cur, ok = repair.NamingRow(append(prev, creates...), row.F)
	}
	return cur, ok
}
