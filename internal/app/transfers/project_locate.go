package transfers

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/project"
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
