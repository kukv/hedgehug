package noticing

import (
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// cause は、消えたものの原因(C4b2-12)。at は記録を持つもの、purged か moved のどちらか一方を持つ。
type cause struct {
	at     change.Ref
	purged *port.Purged
	moved  *transfer.Record
}

// causes は、Store の完全な削除の記録と、別の Store への転送記録。
type causes struct {
	purged []port.Purged
	moved  []transfer.Record
}

// readCauses は、Store store の purged.jsonl と、transfers.jsonl のうち別の Store への行を読む。
func readCauses(r port.Reader, store port.StoreID) (causes, error) {
	ps, err := r.Purged().List()
	if err = notFoundIsSkip(err); err != nil {
		return causes{}, err
	}
	ts, err := r.Transfers().List()
	if err = notFoundIsSkip(err); err != nil {
		return causes{}, err
	}
	cs := causes{purged: ps}
	for _, t := range ts {
		if t.ToStore != string(store) {
			cs.moved = append(cs.moved, t)
		}
	}
	return cs, nil
}

// of は、id の記録。完全な削除を先に見て、別の Store への転送記録は最後の行を使う。
func (cs causes) of(id change.ID) (cause, bool) {
	for j := range cs.purged {
		if cs.purged[j].Target == id {
			return cause{purged: &cs.purged[j]}, true
		}
	}
	for j := len(cs.moved) - 1; j >= 0; j-- {
		if cs.moved[j].Target == id {
			return cause{moved: &cs.moved[j]}, true
		}
	}
	return cause{}, false
}

// findCause は、ref から、親・プロジェクト・ワークスペースの順に記録を探す(C4b2-12)。上のもののログは、同じ
// 保存か合わせで消えたものならその消える前のログ、ほかはいまのログ。なければ false。
func findCause(r port.Reader, b batch, removed []port.RemovedEntity, ref change.Ref) (cause, bool, error) {
	cs, err := readCauses(r, b.store)
	if err != nil {
		return cause{}, false, err
	}
	seen := map[change.Ref]bool{}
	for !seen[ref] {
		seen[ref] = true
		if c, ok := cs.of(ref.ID); ok {
			c.at = ref
			return c, true, nil
		}
		log, err := removedOrCurrent(r, b, removed, ref)
		if err != nil {
			return cause{}, false, err
		}
		next, ok := parentOf(ref, log)
		if !ok {
			return cause{}, false, nil
		}
		ref = next
	}
	return cause{}, false, nil
}

// removedOrCurrent は、removed にあればその消える前のログ、なければいまのログ(ないものは nil)。
func removedOrCurrent(r port.Reader, b batch, removed []port.RemovedEntity, ref change.Ref) ([]change.Record, error) {
	for _, e := range removed {
		if e.Ref == ref {
			return e.Log, nil
		}
	}
	return logOf(r, b, ref)
}
