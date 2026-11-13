package transfers

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// Located は、画面のアドレスの ID(HH-12)を解決した結果(A5a-15)。Found なら Store・Project・Item を開く。
// 転送記録で見つかったときは MovedFrom を添える。見つからず理由があれば Found は false で Gone を持つ。
type Located struct {
	Found     bool
	Store     port.StoreID
	Project   project.ID
	Item      item.ID
	MovedFrom *MovedFrom
	Gone      *Gone
}

// MovedFrom は、旧 ID から開いた印(A5a-16)。Old・At・Display は、旧 ID が一致した最初の記録(at が最も新しいもの)の
// 旧 ID・日時・新 ID。Current は開くアイテムのいまの ID。文は画面が作る。
type MovedFrom struct {
	Old     string
	At      time.Time
	Display string
	Current string
}

// Gone の理由。
const (
	GoneDeleted   = "deleted"   // 移した先で削除済み
	GonePurged    = "purged"    // 完全に削除
	GoneHidden    = "hidden"    // 移動先の Store が表示中でない
	GoneElsewhere = "elsewhere" // そのあと、表示中でない Store へ移した
)

// Gone は、旧 ID の記録はあるが開けないときの理由と材料。Old・At・NewDisplay は、旧 ID が一致した最初の記録のもの。
// StoreName は hidden・elsewhere のときの移動先の Store 名。文は画面が作る(02「見つからないとき」)。
type Gone struct {
	Reason     string
	Old        string
	At         time.Time
	NewDisplay string
	StoreName  string
}

// OldID は、旧 ID に当たるアイテム。
type OldID struct {
	Store port.StoreID
	Item  item.ID
}

// Locate は、画面のアドレスの ID display を解決する(A5a-15)。順は (a) いまの ID(Store 名 storeName の Store を
// 先に、残りはサイドバーの並び)、(b) 旧い接頭辞、(c) 表示中のすべての Store の転送記録。どれでも開けず、記録から
// 理由が分かれば Gone を返す。理由もない・ID が読めないなら port.ErrNotFound。
func (s Service) Locate(ctx context.Context, storeName, display string) (Located, error) {
	prefix, n, ok := parseDisplay(display)
	if !ok {
		return Located{}, port.ErrNotFound
	}
	snaps, err := s.snapshots(ctx)
	if err != nil {
		return Located{}, err
	}
	if h, ok := currentID(namedFirst(snaps, storeName), prefix, n); ok {
		return h.located(nil), nil
	}
	recs := matching(snaps, display)
	if hits := oldHits(snaps, prefix, n, recs); len(hits) > 0 {
		h := hits[0]
		if !h.byRecord {
			return h.located(nil), nil
		}
		return h.located(&MovedFrom{Old: recs[0].Old, At: recs[0].At, Display: recs[0].New, Current: h.current}), nil
	}
	if len(recs) == 0 {
		return Located{}, port.ErrNotFound
	}
	g, to, ok := whyGone(snaps, recs[0])
	if !ok {
		return Located{}, port.ErrNotFound
	}
	if to != nil {
		if g.StoreName, err = s.storeName(ctx, *to); err != nil {
			return Located{}, err
		}
	}
	return Located{Gone: &g}, nil
}

// OldIDs は、旧 ID display に当たる削除されていないアイテムを、Locate の (b)・(c) と同じ規則と順ですべて返す
// (いまの ID で見つかるものは含めない。Locate は、いまの ID でなければ、この先頭を開く)。
func (s Service) OldIDs(ctx context.Context, display string) ([]OldID, error) {
	prefix, n, ok := parseDisplay(display)
	if !ok {
		return nil, nil
	}
	snaps, err := s.snapshots(ctx)
	if err != nil {
		return nil, err
	}
	var out []OldID
	for _, h := range oldHits(snaps, prefix, n, matching(snaps, display)) {
		out = append(out, OldID{Store: h.store, Item: h.it.ID})
	}
	return out, nil
}

// parseDisplay は、「接頭辞-番号」を最後の - で分ける。接頭辞が空、番号が数字だけでないなら ok=false。
func parseDisplay(display string) (string, int, bool) {
	i := strings.LastIndex(display, "-")
	if i <= 0 {
		return "", 0, false
	}
	prefix, digits := display[:i], display[i+1:]
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return "", 0, false
	}
	n, err := strconv.Atoi(digits)
	return prefix, n, err == nil
}

// namedFirst は、名前が name の Store を先頭にした並び。
func namedFirst(snaps []snapshot, name string) []snapshot {
	i := slices.IndexFunc(snaps, func(s snapshot) bool { return s.info.Name == name })
	if i <= 0 {
		return snaps
	}
	out := append([]snapshot{snaps[i]}, snaps[:i]...)
	return append(out, snaps[i+1:]...)
}

// currentID は、(a) いまの接頭辞と番号が一致する、削除されていないアイテム。
func currentID(snaps []snapshot, prefix string, n int) (hit, bool) {
	for _, snap := range snaps {
		if h, ok := snap.numbered(n, func(w wsEntry) bool { return w.ws.Prefix == prefix }); ok {
			return h, true
		}
	}
	return hit{}, false
}

// oldHit は、(b)・(c) で見つかったもの。byRecord は (c) で見つかったこと。
type oldHit struct {
	hit
	byRecord bool
}

// oldHits は、(b) 旧い接頭辞で見つかるもの(Store の並び → ワークスペースの並び)、続いて (c) 記録 recs(新しい順)の
// 内部 ID の削除されていないもの。同じアイテムは 1 度だけ。
func oldHits(snaps []snapshot, prefix string, n int, recs []transfer.Record) []oldHit {
	var out []oldHit
	add := func(h hit, byRecord bool) {
		if !slices.ContainsFunc(out, func(o oldHit) bool { return o.store == h.store && o.it.ID == h.it.ID }) {
			out = append(out, oldHit{h, byRecord})
		}
	}
	for _, snap := range snaps {
		if h, ok := snap.numbered(n, func(w wsEntry) bool { return slices.Contains(w.old, prefix) }); ok {
			add(h, false)
		}
	}
	for _, r := range recs {
		for _, snap := range snaps {
			if h, ok := snap.live(r.Target); ok {
				add(h, true)
				break
			}
		}
	}
	return out
}

// matching は、表示中のすべての Store の転送記録のうち、アイテムの旧 ID が display の行を、at の新しい順で返す。
func matching(snaps []snapshot, display string) []transfer.Record {
	var out []transfer.Record
	for _, snap := range snaps {
		for _, r := range snap.records {
			if r.Entity == transfer.EntityItem && r.Old == display {
				out = append(out, r)
			}
		}
	}
	slices.SortStableFunc(out, func(a, b transfer.Record) int { return b.At.Compare(a.At) })
	return out
}
