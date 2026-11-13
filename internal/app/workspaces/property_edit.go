package workspaces

import (
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// editProperty は、プロパティ id に当てる 1 回の変更。ない ID と、op が使えないプロパティ(allow が偽)は ErrNotFound。
// 別の画面で削除されたプロパティなら、削除を取り消して変更を残す(editAt)。
// text は読んだ設定のプロパティからアクティビティの文を作り、rows は書く Tx の中のプロパティから行を作る。
func (s Service) editProperty(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID,
	allow func(workspace.Property) bool,
	text func(workspace.Property) string,
	rows func(st workspace.Settings, p workspace.Property) ([]change.Draft, error),
) error {
	find := func(st workspace.Settings) (workspace.Property, error) {
		p, ok := st.Property(id)
		if !ok || !allow(p) {
			return workspace.Property{}, port.ErrNotFound
		}
		return p, nil
	}
	return s.editAt(ctx, store, ws, func([]change.Record) []string { return []string{"properties/" + string(id)} },
		func(st workspace.Settings) (string, error) {
			p, err := find(st)
			if err != nil {
				return "", err
			}
			return text(p), nil
		},
		func(_ port.Tx, st workspace.Settings) ([]change.Draft, error) {
			p, err := find(st)
			if err != nil {
				return nil, err
			}
			return rows(st, p)
		})
}

func anyProperty(workspace.Property) bool { return true }

// RenameProperty は、プロパティの名前を変える。同じ名前なら何もしない。
func (s Service) RenameProperty(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, name string) error {
	name = workspace.NormalizeName(name)
	return s.editProperty(ctx, store, ws, id, anyProperty,
		func(p workspace.Property) string {
			return fmt.Sprintf("プロパティ「%s」を「%s」に改名", oneLine(p.Name), oneLine(name))
		},
		func(st workspace.Settings, p workspace.Property) ([]change.Draft, error) {
			if err := workspace.CheckPropertyName(st, id, name); err != nil {
				return nil, err
			}
			if name == p.Name {
				return nil, nil
			}
			return []change.Draft{change.NewSet("properties/"+string(id)+"/name", p.Name, name)}, nil
		})
}

// SetPropertyKinds は、プロパティを使う種類を変える。差分だけを add・remove で書く。
func (s Service) SetPropertyKinds(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, kinds []string) error {
	kinds = workspace.NormalizeKinds(kinds)
	return s.editProperty(ctx, store, ws, id,
		func(p workspace.Property) bool { return p.Type != workspace.TypeStatus },
		func(p workspace.Property) string {
			return fmt.Sprintf("プロパティ「%s」を使う種類を変更", oneLine(p.Name))
		},
		func(_ workspace.Settings, p workspace.Property) ([]change.Draft, error) {
			if err := workspace.CheckKinds(kinds); err != nil {
				return nil, err
			}
			f := "properties/" + string(id) + "/kinds"
			var ds []change.Draft
			for _, k := range kinds {
				if !slices.Contains(p.Kinds, k) {
					ds = append(ds, change.NewAdd(f, k))
				}
			}
			for _, k := range p.Kinds {
				if !slices.Contains(kinds, k) {
					ds = append(ds, change.NewRemove(f, k))
				}
			}
			return ds, nil
		})
}

// SetPropertyHidden は、組み込みプロパティの表示・非表示を切り替える。同じ値なら何もしない。
func (s Service) SetPropertyHidden(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, hidden bool) error {
	return s.editProperty(ctx, store, ws, id,
		func(p workspace.Property) bool { return p.Builtin && p.Type != workspace.TypeStatus },
		func(p workspace.Property) string {
			if hidden {
				return fmt.Sprintf("プロパティ「%s」を非表示に設定", oneLine(p.Name))
			}
			return fmt.Sprintf("プロパティ「%s」を表示に設定", oneLine(p.Name))
		},
		func(_ workspace.Settings, p workspace.Property) ([]change.Draft, error) {
			if p.Hidden == hidden {
				return nil, nil
			}
			return []change.Draft{change.NewSet("properties/"+string(id)+"/hidden", p.Hidden, hidden)}, nil
		})
}

// MoveProperty は、プロパティを 1 つ上(up)または下へ動かす。端なら何もしない。隣と rank を入れ替える。
func (s Service) MoveProperty(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, up bool) error {
	return s.editProperty(ctx, store, ws, id, anyProperty,
		func(workspace.Property) string { return "プロパティの順番を変更" },
		func(st workspace.Settings, _ workspace.Property) ([]change.Draft, error) {
			i := slices.IndexFunc(st.Properties, func(p workspace.Property) bool { return p.ID == id })
			j := i + 1
			if up {
				j = i - 1
			}
			if j < 0 || j >= len(st.Properties) {
				return nil, nil
			}
			a, b := st.Properties[i], st.Properties[j]
			return workspace.SwapRanks("properties/"+string(a.ID)+"/rank", a.Rank, "properties/"+string(b.ID)+"/rank", b.Rank), nil
		})
}
