package transfers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// WorkspaceMoved は、別の Store へ移したワークスペース。内部 ID と接頭辞は変わらない。
type WorkspaceMoved struct {
	ToStore     port.StoreID
	ToStoreName string
	Workspace   workspace.ID
	Prefix      string
}

// crossWorkspaceMove は、別の Store への 1 回のワークスペースの移動。src は prepare で移動元から読んだもの。added は、
// 移動先の保存の fn が呼ばれるたびに tx から作り直す(送信が重なると呼び直されるので、最後に呼ばれたときの値が保存された値)。
type crossWorkspaceMove struct {
	src          workspaceGathered
	from, to     port.StoreInfo
	remote       string // 移動先の remote(転送記録の to_remote)
	at           time.Time
	meFrom, meTo string
	// revive は、ワークスペースが別の画面で削除されていたときの、その削除の操作(取り消す)。削除されていなければ空。
	revive change.ID

	added []member.Member // 移動先のメンバーに足した人
}

// MoveWorkspace は、ワークスペース id を、プロジェクト・アイテム・削除済みのものごと Store to へ移す(03「別の Store へ移動」)。
// 移動元を読み、移動先を Delivery.Deliver で保存して送り、移動元を Stores.Write で保存する。片方の Store のロックを持ったまま、
// もう片方に触らない。送れなければ *SendError で、どちらの Store も変えない。移動元が閲覧のみなら refusal、移動先が閲覧のみか
// 移動元と同じなら port.ErrNotFound、表示中でなければ port.ErrStoreNotFound。移動先に同じ接頭辞・名前があれば refusal(B5c-5)。
// 移動元の保存が失敗したら移動先を戻す(戻せなければ *UndoError)。
func (s Service) MoveWorkspace(ctx context.Context, store port.StoreID, id workspace.ID, to port.StoreID) (WorkspaceMoved, error) {
	info, err := s.storeInfo(ctx, store)
	if err != nil {
		return WorkspaceMoved{}, err
	}
	if info.ReadOnly {
		return WorkspaceMoved{}, refusal.New("閲覧のみの Store です")
	}
	if err := s.checkDestination(ctx, to); err != nil {
		return WorkspaceMoved{}, err
	}
	if to == store {
		return WorkspaceMoved{}, port.ErrNotFound
	}
	m, err := s.prepareWorkspace(ctx, info, id, to)
	if err != nil {
		return WorkspaceMoved{}, err
	}
	op := port.Operation{Actor: change.User(m.meTo), Summary: fmt.Sprintf("ワークスペース「%s」を %s から移動", m.name(), m.from.Name)}
	reason, err := s.Delivery.Deliver(ctx, to, op, m.writeDestination)
	switch {
	case errors.Is(err, port.ErrSaveNotDiscarded):
		return WorkspaceMoved{}, s.rollbackWorkspace(ctx, m, err) // 保存されたかもしれない。移動元は保存しない
	case err != nil:
		return WorkspaceMoved{}, err
	case reason != "":
		return WorkspaceMoved{}, &SendError{Store: m.to.Name, Reason: reason}
	}
	op = port.Operation{Actor: change.User(m.meFrom), Summary: fmt.Sprintf("ワークスペース「%s」を %s へ移動", m.name(), m.to.Name)}
	if err := s.Stores.Write(ctx, store, op, m.writeSource); err != nil {
		return WorkspaceMoved{}, s.rollbackWorkspace(ctx, m, err)
	}
	if m.revive != "" {
		port.NoteRevived(ctx, port.Revival{Kind: port.RevivedName, Text: m.name()})
	}
	return WorkspaceMoved{ToStore: to, ToStoreName: m.to.Name, Workspace: id, Prefix: m.src.ws.Prefix}, nil
}

// prepareWorkspace は、移動元(Store from)を読み(ロックは読むあいだだけ)、続けて移動先を読んで断り(B5c-5)を調べ、実行者を決める。
// 移動先から読んだものは断りのほかに使わない(fn の中で tx から読み直す)。
func (s Service) prepareWorkspace(ctx context.Context, from port.StoreInfo, id workspace.ID, to port.StoreID) (*crossWorkspaceMove, error) {
	store := from.ID
	m := &crossWorkspaceMove{from: from, at: s.Clock.Now()}
	var err error
	if m.to, err = s.storeInfo(ctx, to); err != nil {
		return nil, err
	}
	if err := s.Stores.Read(ctx, store, func(r port.Reader) (err error) { m.src, err = gatherWorkspace(r, id); return }); err != nil {
		return nil, err
	}
	if d := m.src.ws.Deleted; d != nil {
		m.revive = d.Op
	}
	if err := s.Stores.Read(ctx, to, func(r port.Reader) error {
		live, err := port.LiveWorkspaces(r)
		if err != nil {
			return err
		}
		return refuseWorkspaceMove(live, m.src.ws, m.to.Name)
	}); err != nil {
		return nil, err
	}
	if m.remote, err = s.remoteOf(ctx, to); err != nil {
		return nil, err
	}
	if m.meTo, err = s.Identity.Me(ctx, to); err != nil {
		return nil, err
	}
	m.meFrom, err = s.Identity.Me(ctx, store)
	return m, err
}

// name は、ワークスペースの名前を 1 行にしたもの(アクティビティと、削除の取り消しの知らせに使う)。
func (m *crossWorkspaceMove) name() string { return strings.Join(strings.Fields(m.src.ws.Name), " ") }
