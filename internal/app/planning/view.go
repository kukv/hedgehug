package planning

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Overview は、詳細パネルと作成ダイアログの選択肢、見出しの「実行中」、作成ダイアログの既定。
// Sprints は ScreenOrder、Releases は ReleaseOrder で、どちらも削除済みを除く。
type Overview struct {
	Sprints  []planning.Sprint
	Releases []planning.Release
	Active   *planning.Sprint // 実行中のスプリント。なければ nil
	Defaults SprintInput      // スプリントの作成の既定(F1-7)
	// RetentionDays は、その Store の削除済みの保持日数(スプリントの削除の確認の文。05)。
	RetentionDays int
}

// Row は、カードのアイテムの 1 行。
type Row struct {
	ID       item.ID
	Display  string
	Kind     item.Kind
	Title    string
	Points   *float64
	Status   change.ID
	Assignee string
}

// SprintCard は、スプリントのカード。Total・Done は中止を除く。Items は削除済みでない作業項目すべて(完了・中止を含む)で、ワークスペースの並び順。
type SprintCard struct {
	Sprint      planning.Sprint
	Total, Done planning.Totals
	Undone      int // 未着手・進行中の件数
	Canceled    int
	Items       []Row
}

// ReleaseCard は、リリースのカード。Target は Story・Task・Bug の中止でないもの。Items は Step 以外すべて(Epic・中止を含む)で、並び順。
type ReleaseCard struct {
	Release              planning.Release
	Target, Done, Undone int
	Items                []Row
}

// Overview は、ワークスペース ws のスプリントとリリースの選択肢と既定を読む。
func (s Service) Overview(ctx context.Context, store port.StoreID, ws workspace.ID) (Overview, error) {
	var ov Overview
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, err := planOf(r, ws)
		if err != nil {
			return err
		}
		live := p.LiveSprints()
		ov.Sprints = planning.ScreenOrder(live)
		ov.Releases = planning.ReleaseOrder(p.LiveReleases())
		for _, sp := range ov.Sprints {
			if sp.State == planning.Active {
				ov.Active = &sp
				break
			}
		}
		name, start, end := planning.SprintDefaults(live, planning.Today(s.Clock.Now()))
		ov.Defaults = SprintInput{Name: name, Start: start, End: end}
		ov.RetentionDays, err = r.RetentionDays()
		return err
	})
	return ov, err
}

// SprintCards は、使えるスプリントのカードを ScreenOrder で返す。使えないスプリントのアイテムはどのカードにも入らない(F1-5)。
func (s Service) SprintCards(ctx context.Context, store port.StoreID, ws workspace.ID) ([]SprintCard, error) {
	var out []SprintCard
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, its, st, prefix, err := cardSource(r, ws)
		if err != nil {
			return err
		}
		for _, sp := range planning.ScreenOrder(p.LiveSprints()) {
			out = append(out, sprintCard(sp, its, st, prefix))
		}
		return nil
	})
	return out, err
}

func sprintCard(sp planning.Sprint, its []item.Item, st workspace.Settings, prefix string) SprintCard {
	c := SprintCard{Sprint: sp}
	for _, it := range its {
		if it.Sprint != sp.ID || !planning.IsWork(it.Kind) {
			continue
		}
		c.Items = append(c.Items, row(it, prefix))
		switch planning.Category(st, it.Status) {
		case workspace.CategoryCanceled:
			c.Canceled++
			continue
		case workspace.CategoryDone:
			c.Done.Points += planning.Amount(it)
			c.Done.Count++
		default:
			c.Undone++
		}
		c.Total.Points += planning.Amount(it)
		c.Total.Count++
	}
	return c
}

// ReleaseCards は、使えるリリースのカードを ReleaseOrder で返す。使えないリリースのアイテムはどのカードにも入らない(F1-5)。
func (s Service) ReleaseCards(ctx context.Context, store port.StoreID, ws workspace.ID) ([]ReleaseCard, error) {
	var out []ReleaseCard
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, its, st, prefix, err := cardSource(r, ws)
		if err != nil {
			return err
		}
		for _, rel := range planning.ReleaseOrder(p.LiveReleases()) {
			out = append(out, releaseCard(rel, its, st, prefix))
		}
		return nil
	})
	return out, err
}

func releaseCard(rel planning.Release, its []item.Item, st workspace.Settings, prefix string) ReleaseCard {
	c := ReleaseCard{Release: rel}
	for _, it := range its {
		if it.Release != rel.ID || it.Kind == item.Step {
			continue
		}
		c.Items = append(c.Items, row(it, prefix))
		cat := planning.Category(st, it.Status)
		if !planning.IsWork(it.Kind) || cat == workspace.CategoryCanceled {
			continue
		}
		c.Target++
		if cat == workspace.CategoryDone {
			c.Done++
		}
	}
	c.Undone = c.Target - c.Done
	return c
}

// cardSource は、カードの元になる、計画・並び順の削除済みでないアイテム・設定・表示 ID の接頭辞。
func cardSource(r port.Reader, ws workspace.ID) (planning.Plan, []item.Item, workspace.Settings, string, error) {
	p, err := planOf(r, ws)
	if err != nil {
		return planning.Plan{}, nil, workspace.Settings{}, "", err
	}
	w, err := r.Workspaces().Get(ws)
	if err != nil {
		return planning.Plan{}, nil, workspace.Settings{}, "", err
	}
	st, err := r.Workspaces().Settings(ws)
	if err != nil {
		return planning.Plan{}, nil, workspace.Settings{}, "", err
	}
	all, err := items(r, ws)
	if err != nil {
		return planning.Plan{}, nil, workspace.Settings{}, "", err
	}
	var its []item.Item
	for _, it := range all {
		if it.Deleted == nil && it.Place.Workspace == ws {
			its = append(its, it)
		}
	}
	return p, planning.WorkOrder(its), st, w.Prefix, nil
}

func row(it item.Item, prefix string) Row {
	return Row{
		ID: it.ID, Display: it.DisplayID(prefix), Kind: it.Kind, Title: it.Title,
		Points: it.Points, Status: it.Status, Assignee: it.Assignee,
	}
}
