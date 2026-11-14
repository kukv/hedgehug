package noticing

import (
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// descConflictText は、説明の同じ箇所が重なって両方を残したときの本文(画面のトーストと同じ文。決定 C4c1-12)。
const descConflictText = "説明の同じ箇所が同時に編集されたため、両方を残しました"

// descConflicts は、判定する合わせの、説明の同じ箇所が重なったエンティティ(ReceivedSave.Conflicts)のうち、どちらかの
// 側にこのマシンの行(C4c1-4)があるものごとの 1 行(決定 C4c1-12)。差出人は、もう一方の側の desc の行のうち at が
// 最後のものの人。もうないエンティティは飛ばす。own の失敗と、読み取りの失敗(ないもの以外)はエラー。
func descConflicts(r port.Reader, b batch, store port.StoreID, me string, own func(change.ID) (bool, error)) ([]port.Notice, error) {
	m := &mineRows{me: me, own: own, ops: map[change.ID]bool{}}
	var out []port.Notice
	for _, s := range b.saves {
		for _, c := range s.Conflicts {
			other, ok := otherSide(c, m)
			if m.err != nil {
				return nil, m.err
			}
			if !ok {
				continue
			}
			e, found, err := entityOf(r, b, c.Ref)
			if err != nil {
				return nil, err
			}
			if !found {
				continue
			}
			n := e.base
			n.Kind, n.Store, n.Text, n.Field, n.Sender = port.NoticeOverwrite, store, descConflictText, "desc", latestSender(other)
			n.Source = "overwrite:desc:" + string(c.Ref.ID) + ":" + s.Key.Version
			out = append(out, n)
		}
	}
	return out, nil
}

// otherSide は、c のうち、このマシンの行がない側の行。どちらの側にもこのマシンの行がなければ false。
func otherSide(c port.DescConflict, m *mineRows) ([]change.Record, bool) {
	for i, side := range c.Sides {
		for _, row := range side {
			if m.is(row) {
				return c.Sides[1-i], true
			}
		}
	}
	return nil, false
}

// latestSender は、rows のうち at が最後の行(同じ時刻は ID があとのもの)の差出人。行がなければ空。
func latestSender(rows []change.Record) string {
	var last change.Record
	for i, row := range rows {
		if i == 0 || laterFirst(row, last) < 0 {
			last = row
		}
	}
	return sender(last)
}
