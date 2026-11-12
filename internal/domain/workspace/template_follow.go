package workspace

import (
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// listProperty は、一覧 l の値を持つプロパティの ID。テンプレートはステータスを持たないので false。
func listProperty(l List) (change.ID, bool) {
	switch l.Kind {
	case ListPriorities:
		return "priority", true
	case ListSystems:
		return "systems", true
	case ListChoices:
		return l.Property, true
	}
	return "", false
}

// TemplateValueDrafts は、一覧 l の値 id の削除で、テンプレートの値を置き換える・外す行(B3-15)。
func TemplateValueDrafts(s Settings, l List, id, repl change.ID) []change.Draft {
	prop, ok := listProperty(l)
	if !ok {
		return nil
	}
	single := s.Single(l)
	var ds []change.Draft
	for _, t := range s.Templates {
		ds = append(ds, followValue(TemplateField(t.ID, "values", string(prop)), t.Values[prop], id, repl, single)...)
		for _, c := range t.Children {
			f := TemplateField(t.ID, "children", string(c.ID), "values", string(prop))
			ds = append(ds, followValue(f, c.Values[prop], id, repl, single)...)
		}
	}
	return ds
}

func followValue(f string, cur json.RawMessage, id, repl change.ID, single bool) []change.Draft {
	if cur == nil {
		return nil
	}
	if single {
		var v change.ID
		if json.Unmarshal(cur, &v) != nil || v != id {
			return nil
		}
		return []change.Draft{change.NewSet(f, cur, orNil(repl))}
	}
	var vs []change.ID
	if json.Unmarshal(cur, &vs) != nil || !slices.Contains(vs, id) {
		return nil
	}
	rest := slices.DeleteFunc(slices.Clone(vs), func(v change.ID) bool { return v == id })
	if len(rest) == 0 {
		return []change.Draft{change.NewSet(f, cur, nil)}
	}
	return []change.Draft{change.NewSet(f, cur, rest)}
}

func orNil(id change.ID) any {
	if id == "" {
		return nil
	}
	return id
}

// TemplatePropertyDrafts は、追加プロパティ p の削除で、テンプレート(もとと子)の値を外す行(B3-15)。
func TemplatePropertyDrafts(s Settings, p change.ID) []change.Draft {
	var ds []change.Draft
	for _, t := range s.Templates {
		if v, ok := t.Values[p]; ok {
			ds = append(ds, change.NewSet(TemplateField(t.ID, "values", string(p)), v, nil))
		}
		for _, c := range t.Children {
			if v, ok := c.Values[p]; ok {
				ds = append(ds, change.NewSet(TemplateField(t.ID, "children", string(c.ID), "values", string(p)), v, nil))
			}
		}
	}
	return ds
}
