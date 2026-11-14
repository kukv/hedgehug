package transfers

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// whyGone は、旧 ID が一致した最初の記録 rec の内部 ID が、表示中のどの Store でも開けないときの理由(A5a-15)。
// 順は (1) どこかで削除済み、(2) どこかの purged.jsonl にある、(3) rec の移動先の Store が表示中でない、
// (4) rec より新しい同じ内部 ID の最新の記録 later があり、その移動先が rec と違い表示中でない(B から C へ移した、
// その C が見えない)。hidden・elsewhere のときは、Store 名を決める記録(rec か later)も返す。どれでもなければ ok=false。
func whyGone(snaps []snapshot, rec transfer.Record) (Gone, *transfer.Record, bool) {
	g := Gone{Old: rec.Old, At: rec.At, NewDisplay: rec.New}
	switch {
	case slices.ContainsFunc(snaps, func(s snapshot) bool { return s.hasDeleted(rec.Entity, rec.Target) }):
		g.Reason = GoneDeleted
	case slices.ContainsFunc(snaps, func(s snapshot) bool { return s.hasPurged(rec.Target) }):
		g.Reason = GonePurged
	case !shown(snaps, rec.ToStore):
		g.Reason = GoneHidden
		return g, &rec, true
	default:
		later, ok := latestAfter(snaps, rec)
		if !ok || later.ToStore == rec.ToStore || shown(snaps, later.ToStore) {
			return Gone{}, nil, false
		}
		g.Reason = GoneElsewhere
		return g, &later, true
	}
	return g, nil, true
}

// shown は、Store id が表示中か。
func shown(snaps []snapshot, id string) bool {
	return slices.ContainsFunc(snaps, func(s snapshot) bool { return string(s.info.ID) == id })
}

// latestAfter は、表示中のすべての Store の記録のうち、rec と同じ内部 ID で、at が rec より後の最新のもの。
func latestAfter(snaps []snapshot, rec transfer.Record) (transfer.Record, bool) {
	var latest transfer.Record
	found := false
	for _, s := range snaps {
		for _, r := range s.records {
			if r.Entity == rec.Entity && r.Target == rec.Target && r.At.After(rec.At) && (!found || r.At.After(latest.At)) {
				latest, found = r, true
			}
		}
	}
	return latest, found
}

// storeName は、記録 r の移動先の Store 名。このマシンにある Store(非表示を含む。SyncTargets)ならそのいまの名前、
// なければ記録の ToStoreName。
func (s Service) storeName(ctx context.Context, r transfer.Record) (string, error) {
	ts, err := s.SyncTargets.List(ctx)
	if err != nil {
		return "", err
	}
	for _, t := range ts {
		if string(t.ID) == r.ToStore {
			return t.Name, nil
		}
	}
	return r.ToStoreName, nil
}
