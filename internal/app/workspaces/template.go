package workspaces

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// CreateTemplate は、テンプレートを作る(03「一覧と編集」の「テンプレートを作成」)。作ったものを返す。
func (s Service) CreateTemplate(ctx context.Context, store port.StoreID, ws workspace.ID, in workspace.Template) (workspace.Template, error) {
	return s.createTemplate(ctx, store, ws, in, nil)
}

// newTemplate は、作るテンプレート in に ID を振り、値を整える。
func (s Service) newTemplate(in workspace.Template, id change.ID) workspace.Template {
	in.ID = id
	in = s.withIDs(in)
	in.Values = mergeValues(nil, in.Values)
	for i := range in.Children {
		in.Children[i].Values = mergeValues(nil, in.Children[i].Values)
	}
	return in
}

// createTemplate は CreateTemplate と同じ。reread があれば、書く保存の中で最初に呼び、返したテンプレートで
// 作り直す(エラーなら何も保存しない)。
func (s Service) createTemplate(ctx context.Context, store port.StoreID, ws workspace.ID, in workspace.Template, reread func(tx port.Tx) (workspace.Template, error)) (workspace.Template, error) {
	in = s.newTemplate(in, s.IDs.New())
	var saved workspace.Template
	err := s.edit(ctx, store, ws,
		func(workspace.Settings) (string, error) {
			return fmt.Sprintf("テンプレート「%s」を作成", oneLine(workspace.NormalizeName(in.Name))), nil
		},
		func(tx port.Tx, st workspace.Settings) ([]change.Draft, error) {
			cur := in
			if reread != nil {
				fresh, err := reread(tx)
				if err != nil {
					return nil, err
				}
				cur = s.newTemplate(fresh, in.ID)
			}
			t, err := prepare(st, cur)
			if err != nil {
				return nil, err
			}
			saved = t
			return workspace.TemplateDrafts(workspace.Template{}, t), nil
		})
	if err != nil {
		return workspace.Template{}, err
	}
	return saved, nil
}

// UpdateTemplate は、テンプレートを直す(B3-9)。種類を変えたら、前の種類の既定から外す(B3-12)。
func (s Service) UpdateTemplate(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, in workspace.Template) error {
	in.ID = id
	in = s.withIDs(in)
	return s.edit(ctx, store, ws,
		func(st workspace.Settings) (string, error) {
			if _, ok := st.Template(id); !ok {
				return "", port.ErrNotFound
			}
			return fmt.Sprintf("テンプレート「%s」を編集", oneLine(workspace.NormalizeName(in.Name))), nil
		},
		func(tx port.Tx, st workspace.Settings) ([]change.Draft, error) {
			old, ok := st.Template(id)
			if !ok {
				return nil, port.ErrNotFound
			}
			t, err := prepare(st, merged(old, in))
			if err != nil {
				return nil, err
			}
			ds := workspace.TemplateDrafts(old, t)
			if old.Kind != t.Kind {
				ds = append(ds, workspace.DefaultClearDrafts(st, id, t.Kind)...)
				rds, err := projectTemplateDrafts(tx, ws, id)
				if err != nil {
					return nil, err
				}
				ds = append(ds, rds...)
			}
			return ds, nil
		})
}

// DeleteTemplate は、テンプレートを消す。種類ごとの既定からも外す。作ったアイテムは変えない。
func (s Service) DeleteTemplate(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.edit(ctx, store, ws,
		func(st workspace.Settings) (string, error) {
			t, ok := st.Template(id)
			if !ok {
				return "", port.ErrNotFound
			}
			return fmt.Sprintf("テンプレート「%s」を削除", oneLine(t.Name)), nil
		},
		func(tx port.Tx, st workspace.Settings) ([]change.Draft, error) {
			if _, ok := st.Template(id); !ok {
				return nil, port.ErrNotFound
			}
			rds, err := projectTemplateDrafts(tx, ws, id)
			if err != nil {
				return nil, err
			}
			return append(workspace.TemplateDeleteDrafts(st, id), rds...), nil
		})
}

// projectTemplateDrafts は、テンプレート id の削除・種類の変更に、プロジェクト別のルールを追従させる行(B5a-16)。
func projectTemplateDrafts(tx port.Tx, ws workspace.ID, id change.ID) ([]change.Draft, error) {
	log, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return nil, err
	}
	return projectrule.TemplateDropDrafts(projectrule.FromLog(log), id), nil
}

// SetTemplateDefault は、種類 kind の既定のテンプレートを変える(03「種類ごとの既定」)。id が空なら「なし」。
// 候補はその種類のテンプレートだけ。ほかは ErrNotFound(B2-12)。
func (s Service) SetTemplateDefault(ctx context.Context, store port.StoreID, ws workspace.ID, kind string, id change.ID) error {
	if !slices.Contains(workspace.Kinds, kind) {
		return port.ErrNotFound
	}
	return s.edit(ctx, store, ws,
		func(workspace.Settings) (string, error) {
			return fmt.Sprintf("%s の既定のテンプレートを変更", kind), nil
		},
		func(_ port.Tx, st workspace.Settings) ([]change.Draft, error) {
			if t, ok := st.Template(id); id != "" && (!ok || t.Kind != kind) {
				return nil, port.ErrNotFound
			}
			cur := st.TemplateDefaults[kind]
			if cur == id {
				return nil, nil
			}
			return []change.Draft{change.NewSet("template_default/"+kind, orNull(cur), orNull(id))}, nil
		})
}

func orNull(id change.ID) any {
	if id == "" {
		return nil
	}
	return id
}

// prepare は、検査して保存する形にする。ない種類・値の参照は ErrNotFound、名前は断り。
func prepare(st workspace.Settings, in workspace.Template) (workspace.Template, error) {
	if !workspace.ValidTemplate(st, in) {
		return workspace.Template{}, port.ErrNotFound
	}
	t := workspace.NormalizeTemplate(st, in).Ranked()
	if err := workspace.CheckTemplate(st, t); err != nil {
		return workspace.Template{}, err
	}
	return t, nil
}

// merged は、編集の入力の値を、いまの値に重ねる(B3-9)。子は同じ ID のものに重ねる。
func merged(old, in workspace.Template) workspace.Template {
	in.Values = mergeValues(old.Values, in.Values)
	for i, c := range in.Children {
		var prev map[change.ID]json.RawMessage
		if j := slices.IndexFunc(old.Children, func(o workspace.TemplateChild) bool { return o.ID == c.ID }); j >= 0 {
			prev = old.Children[j].Values
		}
		in.Children[i].Values = mergeValues(prev, c.Values)
	}
	return in
}

// mergeValues は、old に in を重ねる。in の null は外す。
func mergeValues(old, in map[change.ID]json.RawMessage) map[change.ID]json.RawMessage {
	out := maps.Clone(old)
	for k, v := range in {
		if string(v) == "null" {
			delete(out, k)
			continue
		}
		if out == nil {
			out = map[change.ID]json.RawMessage{}
		}
		out[k] = v
	}
	return out
}

// withIDs は、ID のない子と Step に新しい ID を振る(入力の配列は書き換えない)。
func (s Service) withIDs(t workspace.Template) workspace.Template {
	t.Children = slices.Clone(t.Children)
	for i := range t.Children {
		if t.Children[i].ID == "" {
			t.Children[i].ID = s.IDs.New()
		}
		t.Children[i].Steps = s.stepIDs(t.Children[i].Steps)
	}
	t.Steps = s.stepIDs(t.Steps)
	return t
}

func (s Service) stepIDs(ss []workspace.TemplateStep) []workspace.TemplateStep {
	ss = slices.Clone(ss)
	for i := range ss {
		if ss[i].ID == "" {
			ss[i].ID = s.IDs.New()
		}
	}
	return ss
}
