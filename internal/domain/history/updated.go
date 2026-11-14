package history

import (
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Updated は、クエリの updated に使う、アイテムが最後に変わった時刻。履歴に出す行(ItemRows と同じ規則)のうち、
// 実行者が人か自動化のもののいちばん新しい時刻(作成を含む)で、GitHub・同期の行と、履歴に出ない行(順位の変更など)は使わない。
// 該当する行がなければゼロ値。
func Updated(log []change.Record, c Context) time.Time {
	var last time.Time
	for _, e := range emitted(log, c) {
		if k := e.rec.Actor.Kind; (k == change.ActorUser || k == change.ActorAutomation) && e.rec.At.After(last) {
			last = e.rec.At
		}
	}
	return last
}
