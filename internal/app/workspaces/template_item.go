package workspaces

import (
	"context"
	"encoding/json"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// FromItem は、「テンプレートとして保存…」のダイアログの入力(03「アイテムからテンプレートを作る」)。
type FromItem struct {
	Name      string
	Title     string // タイトルの書き出し
	Structure bool   // 子と Step を含める
}

// CreateTemplateFromItem は、アイテム id をもとに、そのワークスペースにテンプレートを作る。
// 種類・説明・値・(Structure なら)子(説明を含む)と Step を、並び順のまま写す(B3-16、A4a-5)。
// 保存と検査は CreateTemplate と同じ。
func (s Service) CreateTemplateFromItem(ctx context.Context, store port.StoreID, id item.ID, in FromItem) (workspace.Template, error) {
	var t workspace.Template
	var ws workspace.ID
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		ws, t, err = readTemplate(r, id, in)
		return err
	})
	if err != nil {
		return workspace.Template{}, err
	}
	// 読んだあと保存するまでのあいだに別の画面が削除していたら、写した値は古いので、保存の中で元のアイテムと
	// 生きている子を読み直して作り直す(決定 S8)
	return s.createTemplate(ctx, store, ws, t, func(tx port.Tx) (workspace.Template, error) {
		_, fresh, err := readTemplate(tx, id, in)
		return fresh, err
	})
}

// readTemplate は、生きているアイテム id から、写すテンプレートと、そのワークスペースを読む。
func readTemplate(r port.Reader, id item.ID, in FromItem) (workspace.ID, workspace.Template, error) {
	it, err := port.LiveItem(r, id)
	if err != nil {
		return "", workspace.Template{}, err
	}
	st, err := r.Workspaces().Settings(it.Place.Workspace)
	if err != nil {
		return "", workspace.Template{}, err
	}
	var family []item.Item
	if in.Structure {
		if family, err = port.LiveItems(r, it.Place.Project); err != nil {
			return "", workspace.Template{}, err
		}
	}
	texts, err := descriptionsOf(r, it, family)
	if err != nil {
		return "", workspace.Template{}, err
	}
	return it.Place.Workspace, templateOf(st, it, family, texts, in), nil
}

// descriptionsOf は、テンプレートに写す説明(アイテムと、含める子。Step は説明を持たない)。
func descriptionsOf(r port.Reader, it item.Item, family []item.Item) (map[item.ID]string, error) {
	ids := []item.ID{it.ID}
	if it.Kind == item.Epic {
		for _, c := range childrenOf(family, it.ID) {
			ids = append(ids, c.ID)
		}
	}
	out := make(map[item.ID]string, len(ids))
	for _, id := range ids {
		text, err := r.Items().Description(id)
		if err != nil {
			return nil, err
		}
		out[id] = text
	}
	return out, nil
}

func templateOf(st workspace.Settings, it item.Item, family []item.Item, texts map[item.ID]string, in FromItem) workspace.Template {
	t := workspace.Template{Name: in.Name, Kind: string(it.Kind), Title: in.Title, Description: texts[it.ID], Values: valuesOf(st, it)}
	for _, c := range childrenOf(family, it.ID) {
		switch {
		case it.Kind == item.Epic:
			t.Children = append(t.Children, workspace.TemplateChild{
				Kind: string(c.Kind), Title: c.Title, Description: texts[c.ID], Values: valuesOf(st, c), Steps: stepsOf(family, c),
			})
		case c.Kind == item.Step:
			t.Steps = append(t.Steps, stepOf(it, c))
		}
	}
	return t
}

// valuesOf は、テンプレートに持てるプロパティの、アイテムの値(B3-5・B3-6)。使わない・空の値は
// NormalizeTemplate が捨てる。
func valuesOf(st workspace.Settings, it item.Item) map[change.ID]json.RawMessage {
	out := map[change.ID]json.RawMessage{}
	for _, p := range st.Properties {
		if workspace.HoldsTemplateValue(p) {
			out[p.ID] = item.Current(it, p)
		}
	}
	return out
}

// childrenOf は、parent の直下のアイテムを並び順で。
func childrenOf(family []item.Item, parent item.ID) []item.Item {
	var out []item.Item
	for _, x := range family {
		if x.Place.Parent != nil && *x.Place.Parent == parent {
			out = append(out, x)
		}
	}
	item.SortSiblings(out)
	return out
}

func stepsOf(family []item.Item, parent item.Item) []workspace.TemplateStep {
	var out []workspace.TemplateStep
	for _, c := range childrenOf(family, parent.ID) {
		out = append(out, stepOf(parent, c))
	}
	return out
}

// stepOf は、Step の担当者を、すぐ上のアイテムと比べて決める(03「アイテムからテンプレートを作る」)。
func stepOf(parent, step item.Item) workspace.TemplateStep {
	a := step.Assignee
	switch a {
	case parent.Assignee:
		a = workspace.StepParent
	case "":
		a = workspace.StepNone
	}
	return workspace.TemplateStep{Title: step.Title, Assignee: a}
}
