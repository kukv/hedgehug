package transfers

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// WorkspaceTargets は、ワークスペースを移せる Store の候補(B5c-15)。HiddenOnly は、候補が 0 件で、非表示の Store なら
// 移せるとき true。
type WorkspaceTargets struct {
	Stores     []port.StoreInfo
	HiddenOnly bool
}

// WorkspacePreview は、ワークスペースの移動の確認ダイアログの予告(B5c-20)。People は移動先に足す人。
type WorkspacePreview struct {
	Source, Destination                string
	Counts                             transfer.Counts
	People                             []member.Member
	RemoteNotice, DestinationHasRemote bool
}

// workspaceGathered は、ワークスペースを移すときに移動元から読むもの。set は削除済みを含む。logs はワークスペース・
// プロジェクト・アイテムのすべての行(人を数えるため)。
type workspaceGathered struct {
	ws       workspace.Workspace
	settings workspace.Settings
	set      transfer.WorkspaceSet
	wsLog    []change.Record
	logs     [][]change.Record
	members  []member.Member
}

// gatherWorkspace は、移動元からワークスペース id を読む。別の画面で削除されたものも読む(B5c-10)。ないときは port.ErrNotFound。
// 外との関連は、Store のすべてのアイテムから集める。
func gatherWorkspace(r port.Reader, id workspace.ID) (workspaceGathered, error) {
	ws, err := port.EditableWorkspace(r, id)
	if err != nil {
		return workspaceGathered{}, err
	}
	g := workspaceGathered{ws: ws}
	if g.settings, err = r.Workspaces().Settings(id); err != nil {
		return workspaceGathered{}, err
	}
	if g.wsLog, err = r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: id}); err != nil {
		return workspaceGathered{}, err
	}
	all, err := storeItems(r)
	if err != nil {
		return workspaceGathered{}, err
	}
	projects, err := r.Projects().List(id)
	if err != nil {
		return workspaceGathered{}, err
	}
	g.set = transfer.CollectWorkspace(id, projects, all)
	g.logs = [][]change.Record{g.wsLog}
	for _, p := range g.set.Projects {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindProject, ID: p.ID})
		if err != nil {
			return workspaceGathered{}, err
		}
		g.logs = append(g.logs, log)
	}
	for _, it := range g.set.Items {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID})
		if err != nil {
			return workspaceGathered{}, err
		}
		g.logs = append(g.logs, log)
	}
	g.members, err = r.Members().List()
	return g, err
}

// people は、移動先のメンバー toMembers に足す人と、人の綴りの対応(B5c-7)。items はすべてのアイテム、logs はすべての行、
// extra は設定とルールの人とプロジェクトのオーナー。移動の保存の fn も予告も、この 1 か所で決める。
func (g workspaceGathered) people(toMembers []member.Member) ([]member.Member, transfer.PeopleMap) {
	extra := transfer.SettingsPeople(g.settings, automation.FromLog(g.wsLog))
	for _, p := range g.set.Projects {
		extra = append(extra, p.Owner)
	}
	return transfer.People(g.settings, g.set.Items, g.logs, extra, g.members, toMembers)
}

// refuseWorkspaceMove は、ワークスペース w を、削除されていないワークスペース dst のある Store(名前 storeName)へ移せないとき
// の断り(B5c-5)。同じ接頭辞を先に、次に同じ名前を調べる。移せるなら nil。
func refuseWorkspaceMove(dst []workspace.Workspace, w workspace.Workspace, storeName string) error {
	switch {
	case workspace.PrefixTaken(w, dst):
		return refusal.New(storeName + ": この Store には同じ接頭辞のワークスペースがあります")
	case workspace.NameTaken(w, dst):
		return refusal.New(storeName + ": この Store には同じ名前のワークスペースがあります")
	}
	return nil
}

// WorkspaceTargets は、ワークスペース id を移せる Store の候補を返す(B5c-15)。候補は、表示中で閲覧のみでなく自分でない
// Store(Store の並び)。ワークスペースがないか完全に削除されていれば port.ErrNotFound。
func (s Service) WorkspaceTargets(ctx context.Context, store port.StoreID, id workspace.ID) (WorkspaceTargets, error) {
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		_, err := port.EditableWorkspace(r, id)
		return err
	})
	if err != nil {
		return WorkspaceTargets{}, err
	}
	infos, err := s.StoreList.List(ctx)
	if err != nil {
		return WorkspaceTargets{}, err
	}
	var out WorkspaceTargets
	for _, info := range infos {
		if !info.ReadOnly && info.ID != store {
			out.Stores = append(out.Stores, info)
		}
	}
	if len(out.Stores) > 0 {
		return out, nil
	}
	ts, err := s.SyncTargets.List(ctx)
	if err != nil {
		return WorkspaceTargets{}, err
	}
	for _, t := range ts {
		if t.Hidden && !t.ReadOnly && t.ID != store {
			out.HiddenOnly = true
		}
	}
	return out, nil
}

// WorkspacePreview は、ワークスペース id を Store to へ移すときの予告を返す。移動先に同じ接頭辞・名前のワークスペースが
// あれば refusal(B5c-5)。ワークスペースがないか完全に削除されている・移動先が閲覧のみか自分の Store なら
// port.ErrNotFound、表示中でない Store なら port.ErrStoreNotFound。移動元と移動先は 1 つずつ順に読む(読み取りを入れ子にしない)。
func (s Service) WorkspacePreview(ctx context.Context, store port.StoreID, id workspace.ID, to port.StoreID) (WorkspacePreview, error) {
	if err := s.checkDestination(ctx, to); err != nil {
		return WorkspacePreview{}, err
	}
	if to == store {
		return WorkspacePreview{}, port.ErrNotFound
	}
	var src workspaceGathered
	err := s.Stores.Read(ctx, store, func(r port.Reader) (err error) {
		src, err = gatherWorkspace(r, id)
		return err
	})
	if err != nil {
		return WorkspacePreview{}, err
	}
	var live []workspace.Workspace
	var toMembers []member.Member
	err = s.Stores.Read(ctx, to, func(r port.Reader) (err error) {
		if live, err = port.LiveWorkspaces(r); err != nil {
			return err
		}
		toMembers, err = r.Members().List()
		return err
	})
	if err != nil {
		return WorkspacePreview{}, err
	}
	p, err := s.workspacePreviewPlaces(ctx, store, to)
	if err != nil {
		return WorkspacePreview{}, err
	}
	if err := refuseWorkspaceMove(live, src.ws, p.Destination); err != nil {
		return WorkspacePreview{}, err
	}
	p.Counts = src.set.Counts()
	p.People, _ = src.people(toMembers)
	return p, nil
}

// workspacePreviewPlaces は、予告のうち、移動元・移動先の Store 名と remote の有無を埋める。
func (s Service) workspacePreviewPlaces(ctx context.Context, from, to port.StoreID) (WorkspacePreview, error) {
	fromInfo, err := s.storeInfo(ctx, from)
	if err != nil {
		return WorkspacePreview{}, err
	}
	toInfo, err := s.storeInfo(ctx, to)
	if err != nil {
		return WorkspacePreview{}, err
	}
	fromRemote, err := s.hasRemote(ctx, from)
	if err != nil {
		return WorkspacePreview{}, err
	}
	toRemote, err := s.hasRemote(ctx, to)
	if err != nil {
		return WorkspacePreview{}, err
	}
	return WorkspacePreview{
		Source: fromInfo.Name, Destination: toInfo.Name, RemoteNotice: fromRemote && !toRemote, DestinationHasRemote: toRemote,
	}, nil
}
