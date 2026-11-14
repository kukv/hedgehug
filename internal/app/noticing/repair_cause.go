package noticing

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// cause は、消えたものの原因(C4b2-12)。at は記録を持つもの、purged か moved のどちらか一方を持つ。text は本文の
// {ID}・{旧 ID}(ワークスペースなら「ワークスペース「{名前}」」)。
type cause struct {
	at     change.Ref
	purged *port.Purged
	moved  *transfer.Record
	text   string
}

// records は、1 回の判定で読んだ causes(1 回だけ読む)。
type records struct {
	loaded bool
	cs     causes
}

// causes は、Store の記録。2 回目からは 1 回目に読んだものを返す。
func (b batch) causes(r port.Reader) (causes, error) {
	if b.records.loaded {
		return b.records.cs, nil
	}
	cs, err := readCauses(r, b.store)
	if err != nil {
		return causes{}, err
	}
	b.records.loaded, b.records.cs = true, cs
	return cs, nil
}

// storeNames は、合わせで消えたものか、ワークスペースの行を持つ修復の保存(修復 (9) のことがある)があるときだけ
// 読む、表示中の Store の名前(C4b2-12・C5b1-7 の移動先の Store 名)。閲覧のみの同期では通知を作らないので読まない。
func (s Service) storeNames(ctx context.Context, in port.Received) (map[port.StoreID]string, error) {
	need := false
	for _, sv := range in.Saves {
		need = need || (sv.Merge && len(sv.Removed) > 0) ||
			(sv.Repair && slices.ContainsFunc(sv.Entities, func(e port.EntityRows) bool { return e.Ref.Kind == change.KindWorkspace }))
	}
	if !need || in.ReadOnly {
		return nil, nil
	}
	infos, err := s.List.List(ctx)
	if err != nil {
		return nil, err
	}
	names := map[port.StoreID]string{}
	for _, info := range infos {
		names[info.ID] = info.Name
	}
	return names, nil
}

// storeName は、移動先の Store 名。to_store が表示中の Store ならこのマシンでの名前、なければ to_store_name、
// それも空なら to_store(保存形式 :257)。
func (b batch) storeName(rec transfer.Record) string {
	if name, ok := b.storeNames[port.StoreID(rec.ToStore)]; ok {
		return name
	}
	if rec.ToStoreName != "" {
		return rec.ToStoreName
	}
	return rec.ToStore
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
	cs, err := b.causes(r)
	if err != nil {
		return cause{}, false, err
	}
	seen := map[change.Ref]bool{}
	for !seen[ref] {
		seen[ref] = true
		log, err := removedOrCurrent(r, b, removed, ref)
		if err != nil {
			return cause{}, false, err
		}
		if c, ok := cs.of(ref.ID); ok {
			c.at = ref
			c.text = causeText(c, log)
			return c, true, nil
		}
		next, ok := parentOf(ref, log)
		if !ok {
			return cause{}, false, nil
		}
		ref = next
	}
	return cause{}, false, nil
}

// causeText は、記録を持つもの(消える前のログ log)の本文での呼び方。ワークスペースは「ワークスペース「{名前}」」
// (名前はログ、なければ記録の display か old)、ほかは記録の display か old。
func causeText(c cause, log []change.Record) string {
	var text string
	if c.purged != nil {
		text = c.purged.Display
	} else if c.moved != nil {
		text = c.moved.Old
	}
	if c.at.Kind != change.KindWorkspace {
		return text
	}
	if w, ok := workspace.FromLog(log); ok {
		text = w.Name
	}
	return "ワークスペース「" + oneLine(text) + "」"
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
