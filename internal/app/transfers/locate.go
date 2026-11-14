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
// 旧 ID・日時・新 ID。Current は開くアイテム(LocateProject ではプロジェクト)のいまの ID。文は画面が作る。
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

// Locate は、画面のアドレスの ID display を解決する(A5a-15)。Store 名 storeName の Store を先に、残りの Store
// (サイドバーの並び)をそのあとに、それぞれ (a) いまの ID、(b) 旧い接頭辞と (c) 転送記録の順で探す。Store 名の Store の
// 転送記録で旧 ID が一致したのに開けず、理由が分かるときは、ほかの Store を探さずに Gone を返す。どこでも開けず、
// 表示中のすべての Store の記録から理由が分かれば Gone を返す。理由もない・ID が読めないなら port.ErrNotFound。
func (s Service) Locate(ctx context.Context, storeName, display string) (Located, error) {
	prefix, n, ok := parseDisplay(display)
	if !ok {
		return Located{}, port.ErrNotFound
	}
	f, err := s.find(ctx, itemKind, storeName, prefix, n, display)
	if err != nil || f.gone != nil {
		return Located{Gone: f.gone}, err
	}
	return Located{Found: true, Store: f.store, Project: f.project, Item: f.id, MovedFrom: f.moved}, nil
}

// found は、旧 ID の解決の結果。gone がなければ hit を開く(転送記録で見つかったら moved を添える)。
type found struct {
	hit
	moved *MovedFrom
	gone  *Gone
}

// find は、種類 k の、接頭辞 prefix・番号 n の画面の ID display を、Locate の順で解決する(A5a-15・A5b-17)。
func (s Service) find(ctx context.Context, k kind, storeName, prefix string, n int, display string) (found, error) {
	snaps, err := s.snapshots(ctx)
	if err != nil {
		return found{}, err
	}
	named, others := splitNamed(snaps, storeName)
	if f, ok := findIn(k, named, snaps, prefix, n, display); ok {
		return f, nil
	}
	if recs := matching(k, named, prefix, n, display); len(recs) > 0 {
		if f, ok, err := s.gone(ctx, snaps, recs[0]); ok || err != nil {
			return f, err
		}
	}
	if f, ok := findIn(k, others, snaps, prefix, n, display); ok {
		return f, nil
	}
	recs := matching(k, snaps, prefix, n, display)
	if len(recs) == 0 {
		return found{}, port.ErrNotFound
	}
	f, ok, err := s.gone(ctx, snaps, recs[0])
	if !ok && err == nil {
		err = port.ErrNotFound
	}
	return f, err
}

// findIn は、Store の集まり scope で (a) いまの ID、(b) 旧い接頭辞、(c) scope の転送記録の順に、種類 k のものを探す。
// 記録の内部 ID は、表示中のすべての Store all から探す。
func findIn(k kind, scope, all []snapshot, prefix string, n int, display string) (found, bool) {
	if h, ok := currentID(k, scope, prefix, n); ok {
		return found{hit: h}, true
	}
	recs := matching(k, scope, prefix, n, display)
	hits := oldHits(k, scope, all, prefix, n, recs)
	if len(hits) == 0 {
		return found{}, false
	}
	h := hits[0]
	if !h.byRecord {
		return found{hit: h.hit}, true
	}
	return found{hit: h.hit, moved: &MovedFrom{Old: recs[0].Old, At: recs[0].At, Display: recs[0].New, Current: h.current}}, true
}

// gone は、旧 ID が一致した最初の記録 rec から、開けない理由を found にする。理由がなければ ok=false。
func (s Service) gone(ctx context.Context, snaps []snapshot, rec transfer.Record) (found, bool, error) {
	g, to, ok := whyGone(snaps, rec)
	if !ok {
		return found{}, false, nil
	}
	if to != nil {
		var err error
		if g.StoreName, err = s.storeName(ctx, *to); err != nil {
			return found{}, false, err
		}
	}
	return found{gone: &g}, true, nil
}

// OldIDs は、旧 ID display に当たる削除されていないアイテムを、Locate の (b)・(c) と同じ規則で、表示中のすべての
// Store について(旧い接頭辞は Store の並び → ワークスペースの並び、続いて記録の新しい順)すべて返す(いまの ID で
// 見つかるものは含めない。Locate は Store 名の Store を先に探すので、Locate が開くものが先頭とは限らない)。display が
// 「接頭辞-番号」として読めなければ、当たるものはないので nil, nil を返す(検索は、ID でない語でも呼ぶ)。
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
	for _, h := range oldHits(itemKind, snaps, snaps, prefix, n, matching(itemKind, snaps, prefix, n, display)) {
		out = append(out, OldID{Store: h.store, Item: h.id})
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

// splitNamed は、名前が name の Store(なければ空)と、残りの Store(並びのまま)に分ける。
func splitNamed(snaps []snapshot, name string) (named, others []snapshot) {
	for _, snap := range snaps {
		if named == nil && snap.info.Name == name {
			named = []snapshot{snap}
		} else {
			others = append(others, snap)
		}
	}
	return named, others
}

// currentID は、(a) いまの接頭辞と番号が一致する、削除されていない種類 k のもの。
func currentID(k kind, snaps []snapshot, prefix string, n int) (hit, bool) {
	for _, snap := range snaps {
		if hs := snap.numbered(k, n, func(w wsEntry) bool { return w.ws.Prefix == prefix }); len(hs) > 0 {
			return hs[0], true
		}
	}
	return hit{}, false
}

// oldHit は、(b)・(c) で見つかったもの。byRecord は (c) で見つかったこと。
type oldHit struct {
	hit
	byRecord bool
}

// oldHits は、種類 k の、(b) scope で旧い接頭辞で見つかるもの(Store の並び → ワークスペースの並び)、続いて (c) 記録
// recs(新しい順)の内部 ID の削除されていないもの(表示中のすべての Store all から探す)。同じものは 1 度だけ。
func oldHits(k kind, scope, all []snapshot, prefix string, n int, recs []transfer.Record) []oldHit {
	var out []oldHit
	add := func(h hit, byRecord bool) {
		if !slices.ContainsFunc(out, func(o oldHit) bool { return o.store == h.store && o.id == h.id }) {
			out = append(out, oldHit{h, byRecord})
		}
	}
	for _, snap := range scope {
		for _, h := range snap.numbered(k, n, func(w wsEntry) bool { return slices.Contains(w.old, prefix) }) {
			add(h, false)
		}
	}
	for _, r := range recs {
		for _, snap := range all {
			if h, ok := snap.live(k, r.Target); ok {
				add(h, true)
				break
			}
		}
	}
	return out
}

// matching は、snaps の転送記録のうち、種類 k(Entity)の旧 ID が display の行を、at の新しい順で返す。Store の中に、display の
// 接頭辞を旧い接頭辞に持つワークスペースがあれば、その Store の記録は、そのワークスペースのいまの接頭辞・ほかの旧い
// 接頭辞で書いた同じ番号の ID(HH-2 なら WEB-2)でも一致させる(接頭辞を変えたあとに移したもの)。
func matching(k kind, snaps []snapshot, prefix string, n int, display string) []transfer.Record {
	var out []transfer.Record
	for _, snap := range snaps {
		ids := snap.aliases(k, prefix, n, display)
		for _, r := range snap.records {
			if r.Entity == k.entity && slices.Contains(ids, r.Old) {
				out = append(out, r)
			}
		}
	}
	slices.SortStableFunc(out, func(a, b transfer.Record) int { return b.At.Compare(a.At) })
	return out
}

// aliases は、display と、接頭辞 prefix を旧い接頭辞に持つワークスペースの、いまの接頭辞・旧い接頭辞で書いた種類 k・番号 n の ID。
func (snap snapshot) aliases(k kind, prefix string, n int, display string) []string {
	ids := []string{display}
	for _, w := range snap.workspaces {
		if !slices.Contains(w.old, prefix) {
			continue
		}
		for _, p := range append([]string{w.ws.Prefix}, w.old...) {
			ids = append(ids, k.display(p, n))
		}
	}
	return ids
}
