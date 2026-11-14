package transfers

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ProjectMoveInput は、プロジェクトの移動先と、移動先のワークスペースにないステータスの置き換え先。
type ProjectMoveInput struct {
	ToStore       port.StoreID
	ToWorkspace   workspace.ID
	StatusChoices map[change.ID]change.ID
}

// ProjectPreview は、プロジェクトの移動の確認ダイアログの予告(A5b-23)。Items・Deleted は削除されていない・削除済みの
// アイテムの数、Prefix は移動先の接頭辞、Source・Destination は {移動元}・{移動先} の文(A5b-10)。HasRules は
// プロジェクト別のルールか専用ルールがあるか。NotCarried は移らないルール(A5b-8)、Replacements はステータスの置き換え(A5b-5)。
type ProjectPreview struct {
	Items, Deleted                                           int
	Prefix, Source, Destination                              string
	CrossStore, RemoteNotice, DestinationHasRemote, HasRules bool
	NotCarried                                               []transfer.NotCarried
	Replacements                                             []StatusRow
}

// projectSource は、移動元で移すもの。set は削除済みを含む。rules はプロジェクト別のルール、own は専用ルール。
type projectSource struct {
	project  project.Project
	ws       workspace.Workspace
	settings workspace.Settings
	set      transfer.ProjectSet
	rules    projectrule.Rules
	own      []automation.Rule
}

// readProjectSource は、プロジェクト id を get で読んで、移すものを読む。外との関連は、ほかのプロジェクトのアイテムとの
// 関連なので、Store のすべてのアイテムから集める。
func readProjectSource(r port.Reader, id project.ID, get func(port.Reader, project.ID) (project.Project, error)) (projectSource, error) {
	p, err := get(r, id)
	if err != nil {
		return projectSource{}, err
	}
	all, err := storeItems(r)
	if err != nil {
		return projectSource{}, err
	}
	ws, err := r.Workspaces().Get(p.Place.Workspace)
	if err != nil {
		return projectSource{}, err
	}
	st, err := r.Workspaces().Settings(ws.ID)
	if err != nil {
		return projectSource{}, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws.ID})
	if err != nil {
		return projectSource{}, err
	}
	own := slices.DeleteFunc(automation.FromLog(log), func(rl automation.Rule) bool { return rl.Scope != id })
	return projectSource{
		project: p, ws: ws, settings: st, set: transfer.CollectProject(id, all),
		rules: projectrule.FromLog(log).Of(id), own: own,
	}, nil
}

// carry は、src のルールを移動先の設定 to へ引き継ぐ結果(transfer.CarryRules)。choice はステータスの置き換え先、
// people は別の Store のときの人の対応(同じ Store なら nil)、newID は専用ルールの新しい内部 ID。
func (src projectSource) carry(to workspace.Settings, choice map[change.ID]change.ID, people transfer.PeopleMap, newID func() change.ID) transfer.Carried {
	return transfer.CarryRules(transfer.CarryInput{
		Project: src.project.ID, From: src.settings, To: to, Rules: src.rules, Own: src.own,
		StatusChoice: choice, People: people, NewID: newID,
	})
}

// replacements は、移すアイテム(削除済みを含む)と専用ルールが使うステータスのうち、名前が移動先のワークスペースにない
// ものの行を、移動元のステータスの並びで返す(A5b-5)。選択肢は、一緒に移る使わないステータス(hidden)を除く。
func (src projectSource) replacements(to workspace.Settings, hidden []change.ID) []StatusRow {
	counts := map[change.ID]int{}
	for _, it := range src.set.Items() {
		counts[it.Status]++
	}
	ruled := transfer.RuleStatuses(src.own)
	var present []change.ID
	for _, v := range src.settings.Statuses {
		if counts[v.ID] > 0 || slices.Contains(ruled, v.ID) {
			present = append(present, v.ID)
		}
	}
	return statusRows(present, counts, src.settings, to, projectrule.Rules{}, projectrule.Rules{Hidden: hidden})
}

// projectDestination は、プロジェクトの移動先のワークスペース。
type projectDestination struct {
	ws       workspace.Workspace
	settings workspace.Settings
}

// readProjectDestination は、移動先のワークスペースを読む。ないか削除済みなら port.ErrNotFound。
func readProjectDestination(r port.Reader, ws workspace.ID) (projectDestination, error) {
	w, err := port.LiveWorkspace(r, ws)
	if err != nil {
		return projectDestination{}, err
	}
	st, err := r.Workspaces().Settings(w.ID)
	return projectDestination{ws: w, settings: st}, err
}

// placeName は、{移動元}・{移動先} の文(A5b-10): ワークスペース名、別の Store なら「{Store 名} の {ワークスペース名}」。
func placeName(info port.StoreInfo, ws workspace.Workspace, cross bool) string {
	if cross {
		return info.Name + " の " + ws.Name
	}
	return ws.Name
}

// ProjectPreview は、プロジェクト id を in の移動先へ移すときの予告を返す。in の StatusChoices は使わない。プロジェクトが
// ないか削除済み・移動先が選べない(自分のワークスペース・削除済みのワークスペース・閲覧のみの Store)なら port.ErrNotFound、
// 表示中でない Store なら port.ErrStoreNotFound。移動元と移動先は 1 つずつ順に読む(読み取りを入れ子にしない)。
func (s Service) ProjectPreview(ctx context.Context, store port.StoreID, id project.ID, in ProjectMoveInput) (ProjectPreview, error) {
	if err := s.checkDestination(ctx, in.ToStore); err != nil {
		return ProjectPreview{}, err
	}
	var src projectSource
	if err := s.Stores.Read(ctx, store, func(r port.Reader) (err error) { src, err = readProjectSource(r, id, port.LiveProject); return }); err != nil {
		return ProjectPreview{}, err
	}
	var dst projectDestination
	if err := s.Stores.Read(ctx, in.ToStore, func(r port.Reader) (err error) { dst, err = readProjectDestination(r, in.ToWorkspace); return }); err != nil {
		return ProjectPreview{}, err
	}
	cross := in.ToStore != store
	if !cross && dst.ws.ID == src.ws.ID {
		return ProjectPreview{}, port.ErrNotFound
	}
	p, err := s.projectPreviewPlaces(ctx, store, in.ToStore, src.ws, dst.ws)
	if err != nil {
		return ProjectPreview{}, err
	}
	// 予告は移らないもの(NotCarried)と一緒に移る使わないステータス(Hidden)だけを使い、専用ルールの担当者の人の対応も
	// 新しい内部 ID も出さないので、people は nil(同じ Store の扱い)、newID は空でよい。移動の保存ではこの呼び方を使わず、
	// 別の Store なら transfer.People の対応と本物の newID を渡すこと。
	carried := src.carry(dst.settings, nil, nil, func() change.ID { return "" })
	p.Prefix = dst.ws.Prefix
	p.HasRules = !src.rules.Empty() || len(src.own) > 0
	p.NotCarried = carried.NotCarried
	p.Replacements = src.replacements(dst.settings, carried.Hidden)
	for _, it := range src.set.Items() {
		if it.Deleted != nil {
			p.Deleted++
		} else {
			p.Items++
		}
	}
	return p, nil
}

// projectPreviewPlaces は、予告のうち、移動元・移動先の文と remote の有無を埋める。
func (s Service) projectPreviewPlaces(ctx context.Context, from, to port.StoreID, fromWS, toWS workspace.Workspace) (ProjectPreview, error) {
	fromInfo, err := s.storeInfo(ctx, from)
	if err != nil {
		return ProjectPreview{}, err
	}
	toInfo, err := s.storeInfo(ctx, to)
	if err != nil {
		return ProjectPreview{}, err
	}
	fromRemote, err := s.hasRemote(ctx, from)
	if err != nil {
		return ProjectPreview{}, err
	}
	toRemote, err := s.hasRemote(ctx, to)
	if err != nil {
		return ProjectPreview{}, err
	}
	cross := from != to
	return ProjectPreview{
		Source: placeName(fromInfo, fromWS, cross), Destination: placeName(toInfo, toWS, cross),
		CrossStore: cross, RemoteNotice: cross && fromRemote && !toRemote, DestinationHasRemote: toRemote,
	}, nil
}
