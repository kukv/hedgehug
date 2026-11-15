package noticing

import (
	"encoding/json"
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
			n, ok, err := duplicateNotice(r, b, e.Ref, row, me)
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
func duplicateNotice(r port.Reader, b batch, ref change.Ref, row change.Record, me string) (port.Notice, bool, error) {
	log, err := logOf(r, b, ref)
	if err != nil {
		return port.Notice{}, false, err
	}
	naming, ok := namingRowOf(log, row)
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
	n.Text = "2 台で同時に「" + oneLine(valueName(row.From)) + "」が作られたため、あとから届いた方を「" + oneLine(valueName(row.To)) + "」に変更しました"
	n.Source = "repair:duplicate:" + string(ref.ID) + ":" + row.F + ":" + string(naming.ID)
	return n, true, nil
}

// namingRowOf は、修復の行 row の項目の値を決めた行(決定 C5c1-3・C5c1-10)。row の prev が指す行(修復を書いたときの
// 項目の先端)から決め、prev がなければいまのログ log の作成の行。2 台が同じ修復をした間に名前が変わっても、
// どちらの修復の行からも同じ行になる。なければ false。
func namingRowOf(log []change.Record, row change.Record) (change.Record, bool) {
	return repair.NamingRow(rowsWhere(log, func(c change.Record) bool {
		return slices.Contains(row.Prev, c.ID) || c.T == change.Create
	}), row.F)
}

// valueName は、行の値 raw(JSON の文字列)の名前・接頭辞・キー。文字列でなければ空。
func valueName(raw json.RawMessage) string {
	var name string
	_ = json.Unmarshal(raw, &name)
	return name
}
