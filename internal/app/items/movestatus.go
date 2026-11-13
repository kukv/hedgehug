package items

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// MovePreview は、移動の確認ダイアログに出す件数(02「同じワークスペースの中での移動」、B5a-19)。
type MovePreview struct {
	Below    int           // 下のもの(削除済みを除く)の件数
	Statuses []StatusCount // 移動先が使わないステータス。ワークスペースのステータスの順
}

// StatusCount は、移動先が使わないステータスと、それを持つ移すアイテム(削除済みと自分自身を含む)の件数。
// Default は、置き換えの初めの値(同じ分類の、移動先の既定のステータス)。
type StatusCount struct {
	Status  change.ID
	Count   int
	Default change.ID
}

// statusChange は、移すアイテムのステータスの置き換え 1 件。
type statusChange struct {
	id       item.ID
	from, to change.ID
}

// MovePreview は、Move の確認ダイアログに出す予告を読む。保存はしない。プロジェクトが変わらない移動(何もしない
// 移動を含む)は空を返す。Move と同じ検査で断る。
func (s Service) MovePreview(ctx context.Context, store port.StoreID, id item.ID, in MoveInput) (MovePreview, error) {
	if (in.Parent == nil) == (in.Project == nil) {
		return MovePreview{}, port.ErrNotFound
	}
	var out MovePreview
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := port.LiveItem(r, id)
		if err != nil {
			return err
		}
		to, move, err := destinationOf(r, it, in)
		if err != nil || !move || to.project.ID == it.Place.Project {
			return err
		}
		all, err := r.Items().List(it.Place.Project)
		if err != nil {
			return err
		}
		below := item.Descendants(it.ID, all)
		out.Below = len(item.Live(below))
		st, rules, err := rulesOf(r, it.Place.Workspace, to.project.ID)
		if err != nil {
			return err
		}
		out.Statuses = unusedCounts(st, rules, append([]item.Item{it}, below...))
		return nil
	})
	return out, err
}

// unusedCounts は、moved のうち、プロジェクトが使わないステータスごとの件数(ワークスペースのステータスの順)。
func unusedCounts(st workspace.Settings, rules projectrule.Rules, moved []item.Item) []StatusCount {
	counts := map[change.ID]int{}
	for _, m := range moved {
		if !projectrule.Uses(st, rules, m.Status) {
			counts[m.Status]++
		}
	}
	var out []StatusCount
	for _, v := range st.Statuses {
		if n := counts[v.ID]; n > 0 {
			out = append(out, StatusCount{Status: v.ID, Count: n, Default: st.CategoryDefaults[v.Category]})
		}
	}
	return out
}

// rulesOf は、ワークスペースの設定と、プロジェクトのルール。
func rulesOf(r port.Reader, ws workspace.ID, p project.ID) (workspace.Settings, projectrule.Rules, error) {
	st, err := r.Workspaces().Settings(ws)
	if err != nil {
		return st, projectrule.Rules{}, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return st, projectrule.Rules{}, err
	}
	return st, projectrule.FromLog(log).Of(p), nil
}

// statusReplacements は、moved(移すアイテム。下のものと削除済みを含む)のうち、移動先 dest が使わないステータスの
// ものの置き換えを決める。置き換え先は picks、なければ同じ分類の移動先の既定。置き換え先が移動先で使わない
// ステータス(ないものを含む)なら port.ErrNotFound。移すものがすでに dest にあるなら何もしない。
func statusReplacements(r port.Reader, moved []item.Item, dest project.ID, picks map[change.ID]change.ID) ([]statusChange, error) {
	if len(moved) == 0 || moved[0].Place.Project == dest {
		return nil, nil
	}
	st, rules, err := rulesOf(r, moved[0].Place.Workspace, dest)
	if err != nil {
		return nil, err
	}
	for _, to := range picks {
		if !hasStatus(st, to) || !projectrule.Uses(st, rules, to) {
			return nil, port.ErrNotFound
		}
	}
	var out []statusChange
	for _, m := range moved {
		if projectrule.Uses(st, rules, m.Status) {
			continue
		}
		to, ok := picks[m.Status]
		if !ok {
			to = st.CategoryDefaults[statusCategory(st, m.Status)]
		}
		out = append(out, statusChange{id: m.ID, from: m.Status, to: to})
	}
	return out, nil
}

func hasStatus(st workspace.Settings, id change.ID) bool {
	return slices.ContainsFunc(st.Statuses, func(v workspace.Value) bool { return v.ID == id })
}

func statusCategory(st workspace.Settings, id change.ID) workspace.Category {
	for _, v := range st.Statuses {
		if v.ID == id {
			return v.Category
		}
	}
	return ""
}
