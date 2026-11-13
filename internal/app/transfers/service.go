// Package transfers は、アイテムの別のワークスペース・別の Store への移動(02「別のワークスペースと別の Store への移動」)
// を持つ。1 ファイル 1 操作。
package transfers

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Delivery は、移動先の Store に保存し、remote があれば同じロックの中で受け取って送る口(A5a-5)。
// 理由が空なら保存できた。syncing.Service をそのまま渡せる(配線は cmd/hedgehug)。
type Delivery interface {
	Deliver(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error)
}

// Service は、移動の操作が使う口。SyncTargets は、移動先・移動元の remote の有無と URL を知るため。
type Service struct {
	Stores      port.Stores
	StoreList   port.StoreList
	SyncTargets port.SyncTargets
	Delivery    Delivery
	Identity    port.Identity
	Clock       port.Clock
}

// source は、移動元で移すもの。set は削除済みを含む。
type source struct {
	set      transfer.Set
	ws       workspace.Workspace
	settings workspace.Settings
}

// items は、移すアイテムを、移すアイテム → 下のもの(深さ優先・兄弟の順)の順で返す。
func (s source) items() []item.Item {
	return append([]item.Item{s.set.Root}, s.set.Below...)
}

// readSource は、移すものを読む。移すアイテムがない・削除済み・Step なら port.ErrNotFound(Step は親と一緒に移る。02)。
func readSource(r port.Reader, id item.ID) (source, error) {
	it, err := port.LiveItem(r, id)
	if err != nil {
		return source{}, err
	}
	all, err := r.Items().List(it.Place.Project)
	if err != nil {
		return source{}, err
	}
	set, err := transfer.Collect(id, all)
	if errors.Is(err, transfer.ErrNotFound) || errors.Is(err, transfer.ErrStep) {
		return source{}, port.ErrNotFound
	}
	if err != nil {
		return source{}, err
	}
	ws, err := r.Workspaces().Get(it.Place.Workspace)
	if err != nil {
		return source{}, err
	}
	st, err := r.Workspaces().Settings(ws.ID)
	return source{set: set, ws: ws, settings: st}, err
}

// destination は、移動先のワークスペースとプロジェクト。next は移動先のワークスペースの次の番号。
type destination struct {
	ws       workspace.Workspace
	settings workspace.Settings
	project  project.Project
	next     int
}

// readDestination は、移動先を読む。ワークスペースがない・プロジェクトがないか削除済み・プロジェクトが
// そのワークスペースのものでないなら port.ErrNotFound。
func readDestination(r port.Reader, in MoveInput) (destination, error) {
	ws, err := r.Workspaces().Get(in.ToWorkspace)
	if err != nil {
		return destination{}, err
	}
	pr, err := port.LiveProject(r, in.ToProject)
	if err != nil {
		return destination{}, err
	}
	if pr.Place.Workspace != ws.ID {
		return destination{}, port.ErrNotFound
	}
	st, err := r.Workspaces().Settings(ws.ID)
	if err != nil {
		return destination{}, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws.ID})
	if err != nil {
		return destination{}, err
	}
	return destination{ws: ws, settings: st, project: pr, next: numbering.Next(log, numbering.Items)}, nil
}

// storeInfo は、表示中の Store id を返す。表示中でない(非表示・このマシンにない)なら port.ErrStoreNotFound。
func (s Service) storeInfo(ctx context.Context, id port.StoreID) (port.StoreInfo, error) {
	infos, err := s.StoreList.List(ctx)
	if err != nil {
		return port.StoreInfo{}, err
	}
	for _, info := range infos {
		if info.ID == id {
			return info, nil
		}
	}
	return port.StoreInfo{}, port.ErrStoreNotFound
}

// checkDestination は、移動先の Store を確かめる。表示中でないなら port.ErrStoreNotFound、閲覧のみなら
// port.ErrNotFound(候補に出さないものは選べない)。
func (s Service) checkDestination(ctx context.Context, id port.StoreID) error {
	info, err := s.storeInfo(ctx, id)
	if err == nil && info.ReadOnly {
		return port.ErrNotFound
	}
	return err
}

// hasRemote は、Store id に remote があるか。
func (s Service) hasRemote(ctx context.Context, id port.StoreID) (bool, error) {
	remote, err := s.remoteOf(ctx, id)
	return remote != "", err
}

// remoteOf は、Store id の remote の URL。なければ空。
func (s Service) remoteOf(ctx context.Context, id port.StoreID) (string, error) {
	ts, err := s.SyncTargets.List(ctx)
	if err != nil {
		return "", err
	}
	for _, t := range ts {
		if t.ID == id {
			return t.Remote, nil
		}
	}
	return "", nil
}

// displayID は、接頭辞と番号の画面の ID(HH-12)。書式は item.Item.DisplayID の 1 か所に持つ。
func displayID(prefix string, n int) string {
	return item.Item{Place: item.Place{Number: n}}.DisplayID(prefix)
}

// lastRank は、all のうち親が parent(nil ならプロジェクトの直下)のものの、最後の順位。なければ空
// (app/items の同名の関数の写し。app の機能どうしは import しない)。
func lastRank(all []item.Item, parent *item.ID) string {
	last := ""
	for _, it := range all {
		same := (parent == nil && it.Place.Parent == nil) ||
			(parent != nil && it.Place.Parent != nil && *it.Place.Parent == *parent)
		if same && it.Rank > last {
			last = it.Rank
		}
	}
	return last
}
