package transfers

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// LocatedProject は、画面のアドレスのプロジェクトの ID(HH-P1)を解決した結果(A5b-17)。Found なら Store・Project を開く。
// 転送記録で見つかったときは MovedFrom を添える(Current は開くプロジェクトのいまの ID)。見つからず理由があれば
// Found は false で Gone を持つ。
type LocatedProject struct {
	Found     bool
	Store     port.StoreID
	Project   project.ID
	MovedFrom *MovedFrom
	Gone      *Gone
}

// LocateProject は、画面のアドレスのプロジェクトの ID display(「接頭辞-P番号」)を、Locate と同じ順で解決する
// (A5b-17)。転送記録は entity が project の行だけを見る。削除済みのプロジェクト・削除済みのワークスペースのプロジェクトは
// 開かない。ID が読めない(アイテムの形の HH-12 を含む)・理由もないなら port.ErrNotFound。
func (s Service) LocateProject(ctx context.Context, storeName, display string) (LocatedProject, error) {
	prefix, n, ok := parseProjectDisplay(display)
	if !ok {
		return LocatedProject{}, port.ErrNotFound
	}
	f, err := s.find(ctx, projectKind, storeName, prefix, n, display)
	if err != nil || f.gone != nil {
		return LocatedProject{Gone: f.gone}, err
	}
	return LocatedProject{Found: true, Store: f.store, Project: f.id, MovedFrom: f.moved}, nil
}

// parseProjectDisplay は、「接頭辞-P番号」を最後の - で分ける。- のあとが P で始まらない、接頭辞が空、番号が数字だけで
// ないなら ok=false。
func parseProjectDisplay(display string) (string, int, bool) {
	i := strings.LastIndex(display, "-")
	if i < 0 || !strings.HasPrefix(display[i+1:], "P") {
		return "", 0, false
	}
	return parseDisplay(display[:i+1] + display[i+2:])
}

// LocatedWorkspace は、画面のアドレスのワークスペースの接頭辞を解決した結果(B5c-13)。Found なら Store・Workspace を開く。
// 転送記録で見つかったときは MovedFrom を添える(Old は画面の接頭辞(Gone の Old と同じ。接頭辞を変えてから移しても記録の
// old ではない)、Display は記録の移動先の Store 名、Current はいまの Store 名)。見つからず
// 理由があれば Found は false で Gone を持つ(Old は画面の接頭辞、NewDisplay は記録の移動先の Store 名)。
type LocatedWorkspace struct {
	Found     bool
	Store     port.StoreID
	Workspace workspace.ID
	MovedFrom *MovedFrom
	Gone      *Gone
}

// LocateWorkspace は、画面のアドレスのワークスペースの接頭辞 prefix を、Locate と同じ順で解決する(B5c-13)。転送記録は
// entity が workspace の行だけを見る。削除済みのワークスペースは開かない。理由もないなら port.ErrNotFound。
func (s Service) LocateWorkspace(ctx context.Context, storeName, prefix string) (LocatedWorkspace, error) {
	f, err := s.find(ctx, workspaceKind, storeName, prefix, 0, prefix)
	if err != nil || f.gone != nil {
		return LocatedWorkspace{Gone: f.gone}, err
	}
	if f.moved != nil {
		f.moved.Old = prefix
	}
	return LocatedWorkspace{Found: true, Store: f.store, Workspace: f.id, MovedFrom: f.moved}, nil
}

// withStoreName は、ワークスペースの記録で見つけたときの MovedFrom の Display を、記録の移動先の Store 名(このマシンの名前を
// 優先。storeName)にする(B5c-13)。
func (s Service) withStoreName(ctx context.Context, f found) (found, error) {
	if f.moved == nil || f.rec.Entity != transfer.EntityWorkspace {
		return f, nil
	}
	name, err := s.storeName(ctx, f.rec)
	f.moved.Display = name
	return f, err
}

// viaWorkspace は、scope のワークスペースの記録(新しい順)で移したワークスペースの中の、種類 k・番号 n の削除されていないもの
// (B5c-13。ワークスペースは表示中のすべての Store all から探す)。
func viaWorkspace(k kind, scope, all []snapshot, prefix string, n int) (found, bool) {
	for _, r := range matching(workspaceKind, scope, all, prefix, 0, prefix) {
		for _, snap := range all {
			if hs := snap.numbered(k, n, func(w wsEntry) bool { return w.ws.ID == r.Target }); len(hs) > 0 {
				return found{hit: hs[0]}, true
			}
		}
	}
	return found{}, false
}

// deletedIn は、表示中のどれかの Store のワークスペース ws に、種類 k・番号 n の削除済みのもの(それ自身かプロジェクトが削除済み)があるか。
func deletedIn(snaps []snapshot, k kind, ws workspace.ID, n int) bool {
	for _, snap := range snaps {
		for _, e := range snap.entries(k.entity) {
			if !e.live && e.ws == ws && e.number == n {
				return true
			}
		}
	}
	return false
}

// prefixesOf は、表示中のすべての Store snaps の中の、内部 ID id のワークスペース(削除済みを含む)のいまの接頭辞と旧い接頭辞。
func prefixesOf(snaps []snapshot, id change.ID) []string {
	for _, snap := range snaps {
		for _, w := range snap.workspaces {
			if w.ws.ID == id {
				return append([]string{w.ws.Prefix}, w.old...)
			}
		}
	}
	return nil
}
