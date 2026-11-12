package items

import (
	"context"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// ErrParentElsewhere は、親が別のプロジェクトにあるとき。作成ダイアログの親の候補は同じプロジェクトだけ(02)。
var ErrParentElsewhere = errors.New("the parent is in another project")

// CreateInput は、作成ダイアログの入力のうち A1 で扱うもの。プロパティとテンプレートは A3 で足す。
type CreateInput struct {
	Store   port.StoreID
	Project project.ID
	Parent  *item.ID
	Kind    item.Kind
	Title   string
}

// Create は、アイテムを作る(02「作成したあと」)。
func (s Service) Create(ctx context.Context, in CreateInput) (item.Item, error) {
	me, err := s.Identity.Me(ctx, in.Store)
	if err != nil {
		return item.Item{}, err
	}
	title := item.NormalizeTitle(in.Title)
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%s「%s」を作成", in.Kind, title)}
	var id item.ID
	err = s.Stores.Write(ctx, in.Store, op, func(tx port.Tx) error {
		created, err := s.build(tx, in, title, me)
		if err != nil {
			return err
		}
		id = created.ID
		return errors.Join(
			tx.WriteWorkspaces().Append(created.Place.Workspace, numbering.Draft(numbering.Items, created.Place.Number)),
			tx.WriteItems().Append(created.ID, item.Create(created)),
		)
	})
	if err != nil {
		return item.Item{}, err
	}
	return s.Get(ctx, in.Store, id)
}

// build は、検査をして、作るアイテムを決める。
func (s Service) build(tx port.Tx, in CreateInput, title, me string) (item.Item, error) {
	parentKind, err := kindOf(tx, in.Parent, in.Project)
	if err != nil {
		return item.Item{}, err
	}
	if err := item.CheckCreate(in.Kind, in.Title, in.Project != "", parentKind); err != nil {
		return item.Item{}, err
	}
	p, err := tx.Projects().Get(in.Project)
	if err != nil {
		return item.Item{}, err
	}
	all, err := tx.Items().List(in.Project)
	if err != nil {
		return item.Item{}, err
	}
	wsLog, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: p.Place.Workspace})
	if err != nil {
		return item.Item{}, err
	}
	return item.Item{
		ID:       s.IDs.New(),
		Place:    item.Place{Workspace: p.Place.Workspace, Project: p.ID, Parent: in.Parent, Number: numbering.Next(wsLog, numbering.Items)},
		Kind:     in.Kind,
		Rank:     rank.After(lastRank(all, in.Parent)),
		Title:    title,
		Watchers: []string{me},
	}, nil
}

// kindOf は、親の種類を返す。親を選んでいなければ nil。プロジェクトを選んでいなければ、
// 親のプロジェクトとは比べない(CheckCreate が「プロジェクトを選んでください」で断る)。
func kindOf(tx port.Tx, parent *item.ID, p project.ID) (*item.Kind, error) {
	if parent == nil {
		return nil, nil //nolint:nilnil // 親なしは正しい結果
	}
	it, err := tx.Items().Get(*parent)
	if err != nil {
		return nil, err
	}
	if p != "" && it.Place.Project != p {
		return nil, ErrParentElsewhere
	}
	return &it.Kind, nil
}

// lastRank は、同じ親の子のうち、最後の並び順の値。親が nil なら、プロジェクトの直下(Epic)。
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
