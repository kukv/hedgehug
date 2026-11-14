package descriptions

import (
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// conflictText は、同じ箇所が同時に編集されて両方を残したときの本文(画面のトーストと同じ文。C4c1-12・C4c1-13)。
const conflictText = "説明の同じ箇所が同時に編集されたため、両方を残しました"

// conflictNotice は、保存の中で読んだ r から、重なりの通知を作る。差出人は、いまの説明の最後の desc の行の人
// (作成・複製のあと desc の行がなければ create の行の人)。
func conflictNotice(r port.Reader, store port.StoreID, sl slot) (port.Notice, error) {
	n, err := sl.target(r)
	if err != nil {
		return port.Notice{}, err
	}
	log, err := r.Changes().Of(sl.ref)
	if err != nil {
		return port.Notice{}, err
	}
	n.Kind, n.Store, n.Text, n.Field, n.Sender = port.NoticeOverwrite, store, conflictText, "desc", describer(log)
	return n, nil
}

// describer は、ログ log の最後の desc の行の人。なければ create の行の人。
func describer(log []change.Record) string {
	var desc, create string
	for _, r := range log {
		switch r.T {
		case change.Desc:
			desc = r.By
		case change.Create:
			create = r.By
		}
	}
	if desc != "" {
		return desc
	}
	return create
}
