package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ErrParentElsewhere は、親が別のプロジェクトにあるとき。作成ダイアログの親の候補は同じプロジェクトだけ(02)。
var ErrParentElsewhere = errors.New("the parent is in another project")

// CreateInput は、作成ダイアログの入力。Values はプロパティの値(キーはプロパティの ID。組み込みはキー)。
// テンプレートは A3 の後で足す。
type CreateInput struct {
	Store   port.StoreID
	Project project.ID
	Parent  *item.ID
	Kind    item.Kind
	Title   string
	Values  map[change.ID]json.RawMessage
}

// Create は、アイテムを作る(02「作成したあと」)。
func (s Service) Create(ctx context.Context, in CreateInput) (item.Item, error) {
	return s.create(ctx, in, fmt.Sprintf("%s「%s」を作成", in.Kind, item.NormalizeTitle(in.Title)))
}

// create は、作成と Step の追加が共用する本体。summary はアクティビティの 1 行。
func (s Service) create(ctx context.Context, in CreateInput, summary string) (item.Item, error) {
	me, err := s.Identity.Me(ctx, in.Store)
	if err != nil {
		return item.Item{}, err
	}
	title := item.NormalizeTitle(in.Title)
	op := port.Operation{Actor: change.User(me), Summary: summary}
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
	// 表の順を守るため、タイトルとプロジェクトは親を読む前に確かめる。CheckCreate はこの 2 つを親より先に
	// 検査するので、親を渡さずに呼べば、この 2 つのどちらかで断る。
	if title == "" || in.Project == "" {
		return item.Item{}, item.CheckCreate(in.Kind, in.Title, in.Project != "", nil)
	}
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
	vals, st, err := s.checkValues(tx, p, in.Values)
	if err != nil {
		return item.Item{}, err
	}
	wsLog, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: p.Place.Workspace})
	if err != nil {
		return item.Item{}, err
	}
	created := item.Item{
		ID:       s.IDs.New(),
		Place:    item.Place{Workspace: p.Place.Workspace, Project: p.ID, Parent: in.Parent, Number: numbering.Next(wsLog, numbering.Items)},
		Kind:     in.Kind,
		Rank:     rank.After(lastRank(all, in.Parent)),
		Title:    title,
		Watchers: []string{me},
	}
	applyValues(&created, vals, st)
	return created, nil
}

// checkValues は、プロジェクトのワークスペースの設定で作成の値を検査して読む(親の検査のあと)。
func (s Service) checkValues(tx port.Tx, p project.Project, in map[change.ID]json.RawMessage) (map[change.ID]json.RawMessage, workspace.Settings, error) {
	st, err := tx.Workspaces().Settings(p.Place.Workspace)
	if err != nil {
		return nil, workspace.Settings{}, err
	}
	vals, err := item.CheckInputs(st, in)
	if errors.Is(err, item.ErrNoSuchValue) {
		return nil, st, port.ErrNotFound
	}
	return vals, st, err
}

// applyValues は、読んだ値をアイテムに入れ、ステータスと優先度がなければ初期値にする(決定 10)。
// 読み方は item の readValues と同じ(item の非公開関数は app から呼べないので、ここに写している)。
func applyValues(it *item.Item, vals map[change.ID]json.RawMessage, st workspace.Settings) {
	read := func(k string) string {
		var s string
		_ = json.Unmarshal(vals[change.ID(k)], &s)
		return s
	}
	it.Status = change.ID(read("status"))
	it.Priority = change.ID(read("priority"))
	it.Assignee, it.Start, it.End = read("assignee"), read("start"), read("end")
	it.Points, it.Estimate = number(vals["points"]), number(vals["estimate"])
	_ = json.Unmarshal(vals["systems"], &it.Systems)
	if it.Status == "" {
		it.Status = st.InitialStatus
	}
	if it.Priority == "" {
		it.Priority = st.InitialPriority
	}
	for id, v := range vals {
		if p, ok := st.Property(id); ok && !p.Builtin && string(v) != "null" {
			if it.Fields == nil {
				it.Fields = map[change.ID]json.RawMessage{}
			}
			it.Fields[id] = v
		}
	}
}

// number は、数の値。item の非公開の num と同じ(app から呼べないので写している)。
func number(v json.RawMessage) *float64 {
	var f float64
	if len(v) == 0 || string(v) == "null" || json.Unmarshal(v, &f) != nil {
		return nil
	}
	return &f
}

// kindOf は、親の種類を返す。親を選んでいなければ nil。
func kindOf(tx port.Tx, parent *item.ID, p project.ID) (*item.Kind, error) {
	if parent == nil {
		return nil, nil //nolint:nilnil // 親なしは正しい結果
	}
	it, err := tx.Items().Get(*parent)
	if err != nil {
		return nil, err
	}
	if it.Place.Project != p {
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
