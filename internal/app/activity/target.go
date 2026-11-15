package activity

import (
	"errors"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ruleHome は、自動化のルールの名前と、ルールを持つワークスペース。
type ruleHome struct {
	name      string
	workspace workspace.ID
}

// view は、Store を 1 回読んで引く材料: メンバーの表示名、ワークスペース(削除済みを含む)とその設定、ルール、
// 転送記録、完全に削除の記録。アイテムとプロジェクトは、行ごとに Reader から引く。
type view struct {
	r          port.Reader
	names      item.Names
	workspaces map[workspace.ID]workspace.Workspace
	settings   map[workspace.ID]workspace.Settings
	rules      map[change.ID]ruleHome
	transfers  []transfer.Record
	purged     []port.Purged
}

func readView(r port.Reader) (*view, error) {
	ms, err := r.Members().List()
	if err != nil {
		return nil, err
	}
	byLogin := make(map[string]string, len(ms))
	for _, m := range ms {
		byLogin[strings.ToLower(m.Login)] = m.Name
	}
	v := &view{
		r: r,
		names: func(login string) string {
			if n, ok := byLogin[strings.ToLower(login)]; ok {
				return n
			}
			return unknownUser
		},
		workspaces: map[workspace.ID]workspace.Workspace{},
		settings:   map[workspace.ID]workspace.Settings{},
		rules:      map[change.ID]ruleHome{},
	}
	if err := v.readWorkspaces(); err != nil {
		return nil, err
	}
	if v.transfers, err = r.Transfers().List(); err != nil {
		return nil, err
	}
	v.purged, err = r.Purged().List()
	return v, err
}

func (v *view) readWorkspaces() error {
	wss, err := v.r.Workspaces().List()
	if err != nil {
		return err
	}
	for _, w := range wss {
		v.workspaces[w.ID] = w
		if v.settings[w.ID], err = v.r.Workspaces().Settings(w.ID); err != nil {
			return err
		}
		log, err := v.r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
		if err != nil {
			return err
		}
		for id, name := range automation.Names(log) {
			v.rules[id] = ruleHome{name: name, workspace: w.ID}
		}
	}
	return nil
}

// item は、Store にあるアイテム(削除済みを含む)。なければ false。
func (v *view) item(id item.ID) (item.Item, bool, error) {
	it, err := v.r.Items().Get(id)
	if errors.Is(err, port.ErrNotFound) {
		return item.Item{}, false, nil
	}
	return it, err == nil, err
}

// project は、Store にあるプロジェクト(削除済みを含む)。なければ false。
func (v *view) project(id project.ID) (project.Project, bool, error) {
	p, err := v.r.Projects().Get(id)
	if errors.Is(err, port.ErrNotFound) {
		return project.Project{}, false, nil
	}
	return p, err == nil, err
}

// place は、行の対象(C4d-7・C4d-8)とワークスペース(C4d-9)を refs から決める。storewide は、ワークスペースが
// 1 つも決まらないときに「Store 全体」の印を持つか。
func (v *view) place(row *Row, refs []change.Ref, storewide bool) error {
	wss, err := v.workspacesOf(refs)
	if err != nil {
		return err
	}
	row.Workspaces = wss
	row.Storewide = len(wss) == 0 && storewide
	if len(wss) == 1 {
		row.Workspace = v.workspaces[wss[0]].Name
	}
	row.Item, row.Project, err = v.targets(refs)
	return err
}

// targets は、対象のアイテムとプロジェクト(C4d-7)。プロジェクトは、行を持つものが 1 つならそれ、なければ対象の
// アイテムのいまのプロジェクト。
func (v *view) targets(refs []change.Ref) (*Target, *Target, error) {
	items, projects := split(refs)
	root, err := v.root(items)
	if err != nil || root == "" && len(projects) != 1 {
		return nil, nil, err
	}
	var it *Target
	if root != "" {
		if it, err = v.itemTarget(root); err != nil {
			return nil, nil, err
		}
		if len(projects) == 0 {
			got, ok, err := v.item(root)
			if err != nil || !ok {
				return it, nil, err
			}
			projects = []project.ID{got.Place.Project}
		}
	}
	if len(projects) != 1 {
		return it, nil, nil
	}
	p, err := v.projectTarget(projects[0])
	return it, p, err
}

// split は、refs のアイテムとプロジェクト。
func split(refs []change.Ref) ([]item.ID, []project.ID) {
	var items []item.ID
	var projects []project.ID
	for _, ref := range refs {
		switch ref.Kind {
		case change.KindItem:
			items = append(items, ref.ID)
		case change.KindProject:
			projects = append(projects, ref.ID)
		}
	}
	return items, projects
}

// workspacesOf は、refs のいまのワークスペースの集合(C4d-9)。
func (v *view) workspacesOf(refs []change.Ref) ([]workspace.ID, error) {
	var out []workspace.ID
	for _, ref := range refs {
		id, ok, err := v.workspaceOf(ref)
		if err != nil {
			return nil, err
		}
		if ok && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out, nil
}

// workspaceOf は、エンティティのいまのワークスペース。ワークスペースはそれ自身、プロジェクト・アイテムは Store に
// あるもの(削除済みを含む)の Workspace。members と、Store にないものは false。
func (v *view) workspaceOf(ref change.Ref) (workspace.ID, bool, error) {
	switch ref.Kind {
	case change.KindWorkspace:
		_, ok := v.workspaces[ref.ID]
		return ref.ID, ok, nil
	case change.KindProject:
		p, ok, err := v.project(ref.ID)
		return p.Place.Workspace, ok, err
	case change.KindItem:
		it, ok, err := v.item(ref.ID)
		return it.Place.Workspace, ok, err
	}
	return "", false, nil
}

// root は、対象のアイテム(C4d-7): 1 つならそれ。2 つ以上なら、いまの親がその中にないものがちょうど 1 つのときだけ
// それ。Store にないアイテム(別の Store へ移した・完全に削除した)は、いまの親がないので根に数える。なければ ""。
func (v *view) root(ids []item.ID) (item.ID, error) {
	if len(ids) <= 1 {
		if len(ids) == 0 {
			return "", nil
		}
		return ids[0], nil
	}
	var roots []item.ID
	for _, id := range ids {
		it, ok, err := v.item(id)
		if err != nil {
			return "", err
		}
		if ok && it.Place.Parent != nil && slices.Contains(ids, *it.Place.Parent) {
			continue
		}
		roots = append(roots, id)
	}
	if len(roots) != 1 {
		return "", nil
	}
	return roots[0], nil
}

// itemTarget は、アイテムの表示(C4d-8)。
func (v *view) itemTarget(id item.ID) (*Target, error) {
	it, ok, err := v.item(id)
	if err != nil || !ok {
		return v.gone(transfer.EntityItem, id), err
	}
	return &Target{Display: it.DisplayID(v.workspaces[it.Place.Workspace].Prefix), Link: linkOf(it.Deleted)}, nil
}

// projectTarget は、プロジェクトの表示(C4d-8)。タイトルは Store にあるとき(削除済みを含む)だけ。
func (v *view) projectTarget(id project.ID) (*Target, error) {
	p, ok, err := v.project(id)
	if err != nil || !ok {
		return v.gone(transfer.EntityProject, id), err
	}
	return &Target{Display: p.DisplayID(v.workspaces[p.Place.Workspace].Prefix), Title: p.Title, Link: linkOf(p.Deleted)}, nil
}

func linkOf(deleted *change.Deletion) string {
	if deleted != nil {
		return ""
	}
	return LinkCurrent
}

// gone は、Store にないものの表示: 転送記録の最後の行の旧 ID(リンク)、なければ完全に削除の記録の ID(文字だけ)。
// どちらにもなければ nil。
func (v *view) gone(entity string, id change.ID) *Target {
	for _, rec := range slices.Backward(v.transfers) {
		if rec.Entity == entity && rec.Target == id {
			return &Target{Display: rec.Old, Link: LinkMoved}
		}
	}
	for _, p := range slices.Backward(v.purged) {
		if p.Entity == entity && p.Target == id {
			return &Target{Display: p.Display}
		}
	}
	return nil
}
