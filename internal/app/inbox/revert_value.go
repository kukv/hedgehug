package inbox

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	deletedValueText = "(削除された値)"
	unknownUserText  = "不明なユーザー"
)

// sameValue は、2 つの値が同じか(C4c2-10)。JSON を読んで比べる。
// null・""・false・空の配列は同じ「空」、配列は集合、数は値で比べる。
func sameValue(a, b json.RawMessage) bool {
	return canonical(a) == canonical(b)
}

func canonical(raw json.RawMessage) string {
	var v any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &v)
	}
	out, _ := json.Marshal(normalize(v))
	return string(out)
}

func normalize(v any) any {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil
		}
	case bool:
		if !x {
			return nil
		}
	case []any:
		if len(x) == 0 {
			return nil
		}
		parts := make([]string, len(x))
		for i, e := range x {
			b, _ := json.Marshal(normalize(e))
			parts[i] = string(b)
		}
		slices.Sort(parts)
		return slices.Compact(parts)
	}
	return v
}

// showProperty は、プロパティの値 v の表示(C4c2-5)。削除済みの値はログの名前で出し、
// 複数の値は設定の並び(削除済みの値は末尾、ID 順)に並べ直す。
func showProperty(p workspace.Property, v json.RawMessage, st workspace.Settings, log []change.Record, names item.Names, now time.Time) string {
	vals, prefix, set := valueList(p, &st)
	if set == nil {
		return item.Display(p, v, st, names, now)
	}
	ids, multi := idsOf(v)
	var extra []change.ID
	for _, id := range ids {
		if !slices.ContainsFunc(vals, func(x workspace.Value) bool { return x.ID == id }) {
			extra = append(extra, id)
		}
	}
	slices.Sort(extra)
	extra = slices.Compact(extra)
	var ordered []change.ID
	for _, x := range vals {
		if slices.Contains(ids, x.ID) {
			ordered = append(ordered, x.ID)
		}
	}
	ordered = append(ordered, extra...)
	withDeleted := slices.Clone(vals)
	for _, id := range extra {
		if name := workspace.NameOfDeleted(log, prefix+string(id)); name != "" {
			withDeleted = append(withDeleted, workspace.Value{ID: id, Name: name})
		}
	}
	set(withDeleted)
	if multi {
		v, _ = json.Marshal(ordered)
	}
	return item.Display(p, v, st, names, now)
}

// valueList は、p の値の一覧と、削除済みの値の名前を引く接頭辞、st の一覧を差し替える関数を返す。
// 値の ID を持たない種類は set が nil。
func valueList(p workspace.Property, st *workspace.Settings) ([]workspace.Value, string, func([]workspace.Value)) {
	switch p.Type {
	case workspace.TypeStatus:
		return st.Statuses, "statuses/", func(vs []workspace.Value) { st.Statuses = vs }
	case workspace.TypePriority:
		return st.Priorities, "priorities/", func(vs []workspace.Value) { st.Priorities = vs }
	case workspace.TypeSystems:
		return st.Systems, "systems/", func(vs []workspace.Value) { st.Systems = vs }
	case workspace.TypeSingle, workspace.TypeMulti:
		cur, _ := st.Property(p.ID)
		return cur.Choices, "choices/" + string(p.ID) + "/", func(vs []workspace.Value) {
			props := slices.Clone(st.Properties)
			for i := range props {
				if props[i].ID == p.ID {
					props[i].Choices = vs
				}
			}
			st.Properties = props
		}
	}
	return nil, "", nil
}

// idsOf は、値の ID を読む。配列なら multi。
func idsOf(v json.RawMessage) (ids []change.ID, multi bool) {
	if json.Unmarshal(v, &ids) == nil && string(v) != "null" {
		return ids, true
	}
	var one change.ID
	if json.Unmarshal(v, &one) == nil && one != "" {
		return []change.ID{one}, false
	}
	return nil, false
}

// showPlan は、スプリント(sprints/)・リリース(releases/)の値 v の表示。空は「なし」。
func showPlan(v json.RawMessage, log []change.Record, prefix string) string {
	var id string
	_ = json.Unmarshal(v, &id)
	if id == "" {
		return "なし"
	}
	if name := workspace.NameOfDeleted(log, prefix+id); name != "" {
		return oneLine(name)
	}
	return deletedValueText
}

// showProject は、プロジェクトの項目 f の値 v の表示。
func showProject(f string, v json.RawMessage, names item.Names, now time.Time) string {
	var s string
	_ = json.Unmarshal(v, &s)
	switch f {
	case "owner":
		if s == "" {
			return "なし"
		}
		return names(s)
	case "start", "end":
		return project.DateText(s, now)
	}
	if s == "" {
		return "なし"
	}
	return s
}

// memberNames は、ログイン名(大文字と小文字を区別しない)から表示名を引く。いなければ「不明なユーザー」。
func memberNames(ms []member.Member) item.Names {
	return func(login string) string {
		for _, m := range ms {
			if strings.EqualFold(m.Login, login) {
				return m.Name
			}
		}
		return unknownUserText
	}
}
