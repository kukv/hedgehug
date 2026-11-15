package noticing

import (
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// issueUnlinks は、i 番目の修復の保存が、重なった Issue のリンクを外した修復 (11) なら、その通知(06「同期の修復」)。
// 見分け方は決定 C5c2-8、宛先は C5c2-9、本文は C5c2-10、鍵は C5c2-11、開く先は C5c2-12。外した行ごとに 1 件。
// メンバーの統合の保存(メンバーの一覧のログの remove)は、アイテムの行ではないので通知を作らない(C5c2-13)。
func issueUnlinks(r port.Reader, b batch, i int, me string) ([]port.Notice, error) {
	s := b.saves[i]
	if !s.Repair || hasRestoreRow(s) {
		return nil, nil
	}
	var out []port.Notice
	for _, e := range s.Entities {
		if e.Ref.Kind != change.KindItem {
			continue
		}
		ns, err := itemUnlinks(r, b, e, me)
		if err != nil {
			return nil, err
		}
		out = append(out, ns...)
	}
	return out, nil
}

// itemUnlinks は、アイテムの行 e の、Issue のリンクを外した行ごとの通知。
func itemUnlinks(r port.Reader, b batch, e port.EntityRows, me string) ([]port.Notice, error) {
	var out []port.Notice
	for _, row := range e.Rows {
		link, ok := removedIssue(row)
		if !ok {
			continue
		}
		n, ok, err := unlinkNotice(r, b, e.Ref, row, link, me)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, n)
		}
	}
	return out, nil
}

// removedIssue は、行 row が Issue のリンクの remove なら、その Ref。
func removedIssue(row change.Record) (itemlink.Ref, bool) {
	if row.T != change.Remove || row.F != itemlink.LinksField {
		return itemlink.Ref{}, false
	}
	var ref itemlink.Ref
	if json.Unmarshal(row.From, &ref) != nil || ref.Kind != itemlink.Issue {
		return itemlink.Ref{}, false
	}
	return ref, true
}

// unlinkNotice は、アイテム ref の Issue のリンク link の remove の行 row の通知。リンクした人があなたでなければ、
// または本文を決められなければ false。
func unlinkNotice(r port.Reader, b batch, ref change.Ref, row change.Record, link itemlink.Ref, me string) (port.Notice, bool, error) {
	log, err := logOf(r, b, ref)
	if err != nil {
		return port.Notice{}, false, err
	}
	linked, ok := linkingRow(log, row)
	if !ok || linked.Actor.Kind != change.ActorUser || !same(linked.By, me) {
		return port.Notice{}, false, nil
	}
	self, ok, err := entityOf(r, b, ref)
	if !ok {
		return port.Notice{}, false, err
	}
	text, ok, err := unlinkText(r, b, ref, link, self.text, log)
	if !ok {
		return port.Notice{}, false, err
	}
	n := self.base
	n.Text = text
	n.Source = "repair:issue:" + string(ref.ID) + ":" + link.Key() + ":" + string(linked.ID)
	return n, true, nil
}

// linkingRow は、remove の行 row のリンクをした行(C5c2-9)。row の prev が指す add の行(複数なら、その行だけで
// change.ListRow の勝った行)から、改名を追った行をたどった先(C5c2-17)。prev が指す add の行がなければ false。
func linkingRow(log []change.Record, row change.Record) (change.Record, bool) {
	adds := rowsWhere(log, func(c change.Record) bool {
		return c.T == change.Add && c.F == itemlink.LinksField && slices.Contains(row.Prev, c.ID)
	})
	add, ok := change.ListRow(adds, itemlink.LinksField, row.From)
	if !ok {
		return change.Record{}, false
	}
	return itemlink.Origin(log, add), true
}

// unlinkText は、本文(C5c2-10)。同じ Issue を持つ別のアイテムがあれば (a)、外したアイテムにいま別の Issue があれば (b)。
// どちらもなければ false。self は外したアイテムの呼び方、log はそのログ。
func unlinkText(r port.Reader, b batch, ref change.Ref, link itemlink.Ref, self string, log []change.Record) (string, bool, error) {
	holder, ok, err := issueHolder(r, b, ref.ID, link)
	if err != nil {
		return "", false, err
	}
	if ok {
		other, ok, err := entityOf(r, b, change.Ref{Kind: change.KindItem, ID: holder})
		if !ok {
			return "", false, err
		}
		return "Issue " + link.Display() + " は " + other.text + " にリンクされているため、" + self + " からリンクを外しました", true, nil
	}
	left, ok := itemlink.FromLog(log).HasIssue()
	if !ok {
		return "", false, nil
	}
	return self + " には Issue " + left.Display() + " がリンクされているため、Issue " + link.Display() + " のリンクを外しました", true, nil
}

// issueHolder は、except 以外の削除されていないアイテムのうち、Issue link(Same)をリンクしているものの、
// 内部 ID が最も小さいもの。なければ false。
func issueHolder(r port.Reader, b batch, except item.ID, link itemlink.Ref) (item.ID, bool, error) {
	ids, err := liveItemIDs(r)
	if err != nil {
		return "", false, err
	}
	var best item.ID
	for _, id := range ids {
		if id == except || (best != "" && id > best) {
			continue
		}
		log, err := logOf(r, b, change.Ref{Kind: change.KindItem, ID: id})
		if err != nil {
			return "", false, err
		}
		if holdsIssue(log, link) {
			best = id
		}
	}
	return best, best != "", nil
}

// liveItemIDs は、Store の削除されていないアイテムの内部 ID。
func liveItemIDs(r port.Reader) ([]item.ID, error) {
	wss, err := port.LiveWorkspaces(r)
	if err != nil {
		return nil, notFoundIsSkip(err)
	}
	var ids []item.ID
	for _, w := range wss {
		ps, err := port.LiveProjects(r, w.ID)
		if err != nil {
			return nil, notFoundIsSkip(err)
		}
		for _, p := range ps {
			its, err := port.LiveItems(r, p.ID)
			if err != nil {
				return nil, notFoundIsSkip(err)
			}
			for _, it := range its {
				ids = append(ids, it.ID)
			}
		}
	}
	return ids, nil
}

// holdsIssue は、アイテムのログ log に、Issue link(Same)のリンクがあるか。
func holdsIssue(log []change.Record, link itemlink.Ref) bool {
	return slices.ContainsFunc(itemlink.FromLog(log).Links, func(l itemlink.Link) bool {
		return l.Kind == itemlink.Issue && l.Same(link)
	})
}
