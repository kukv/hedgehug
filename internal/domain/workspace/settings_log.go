package workspace

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// valueLists は、値の一覧の項目名の頭。
var valueLists = []string{"statuses", "priorities", "systems"}

// CreateWithSettings は、設定を含めたワークスペースの作成の変更。検査は Create と同じ。
// 設定は項目ごと(statuses/<ID>/name など)に、作成の 1 行の to に書く(この計画の決定 2・4)。
func CreateWithSettings(w Workspace, s Settings) (change.Draft, error) {
	if _, err := Create(w); err != nil {
		return change.Draft{}, err
	}
	m := map[string]any{"id": w.ID, "name": w.Name, "prefix": w.Prefix}
	for _, p := range s.Properties {
		base := "properties/" + string(p.ID) + "/"
		m[base+"key"], m[base+"name"], m[base+"type"] = p.Key, p.Name, p.Type
		m[base+"builtin"], m[base+"kinds"], m[base+"hidden"], m[base+"rank"] = p.Builtin, p.Kinds, p.Hidden, p.Rank
	}
	for i, vs := range [][]Value{s.Statuses, s.Priorities, s.Systems} {
		for _, v := range vs {
			base := valueLists[i] + "/" + string(v.ID) + "/"
			m[base+"name"], m[base+"color"], m[base+"rank"] = v.Name, v.Color, v.Rank
			if i == 0 {
				m[base+"category"] = v.Category
			}
		}
	}
	m["initial_status"], m["initial_priority"] = s.InitialStatus, s.InitialPriority
	for c, id := range s.CategoryDefaults {
		m["category_default/"+string(c)] = id
	}
	return change.NewCreate(m), nil
}

// SettingsFromLog は、ワークスペースのログから設定を作る。作成の行がなければ false。
func SettingsFromLog(log []change.Record) (Settings, bool) {
	st := change.Resolve(log, opts)
	if st.Created == nil {
		return Settings{}, false
	}
	b := newBuilder()
	for f, raw := range st.Values {
		b.set(f, raw)
	}
	for f, list := range st.Lists {
		b.setKinds(f, list)
	}
	return b.settings(), true
}

// builder は、項目ごとの値を集めて Settings にする。
type builder struct {
	props map[change.ID]*Property
	vals  map[string]map[change.ID]*Value
	acc   Settings
}

func newBuilder() *builder {
	b := &builder{props: map[change.ID]*Property{}, vals: map[string]map[change.ID]*Value{}}
	for _, l := range valueLists {
		b.vals[l] = map[change.ID]*Value{}
	}
	b.acc.CategoryDefaults = map[Category]change.ID{}
	return b
}

func (b *builder) prop(id string) *Property {
	p, ok := b.props[change.ID(id)]
	if !ok {
		p = &Property{ID: change.ID(id)}
		b.props[change.ID(id)] = p
	}
	return p
}

func (b *builder) value(list, id string) *Value {
	v, ok := b.vals[list][change.ID(id)]
	if !ok {
		v = &Value{ID: change.ID(id)}
		b.vals[list][change.ID(id)] = v
	}
	return v
}

func (b *builder) set(f string, raw json.RawMessage) {
	parts := strings.Split(f, "/")
	switch {
	case len(parts) == 3 && parts[0] == "properties":
		setProperty(b.prop(parts[1]), parts[2], raw)
	case len(parts) == 3 && slices.Contains(valueLists, parts[0]):
		setValue(b.value(parts[0], parts[1]), parts[2], raw)
	case f == "initial_status":
		_ = json.Unmarshal(raw, &b.acc.InitialStatus)
	case f == "initial_priority":
		_ = json.Unmarshal(raw, &b.acc.InitialPriority)
	case len(parts) == 2 && parts[0] == "category_default":
		var id change.ID
		if json.Unmarshal(raw, &id) == nil {
			b.acc.CategoryDefaults[Category(parts[1])] = id
		}
	}
}

func (b *builder) setKinds(f string, list []json.RawMessage) {
	parts := strings.Split(f, "/")
	if len(parts) != 3 || parts[0] != "properties" || parts[2] != "kinds" {
		return
	}
	p := b.prop(parts[1])
	for _, k := range Kinds { // Kinds の並びにそろえる
		for _, raw := range list {
			var s string
			if json.Unmarshal(raw, &s) == nil && s == k {
				p.Kinds = append(p.Kinds, k)
			}
		}
	}
}

func setProperty(p *Property, field string, raw json.RawMessage) {
	switch field {
	case "key":
		_ = json.Unmarshal(raw, &p.Key)
	case "name":
		_ = json.Unmarshal(raw, &p.Name)
	case "type":
		_ = json.Unmarshal(raw, &p.Type)
	case "builtin":
		_ = json.Unmarshal(raw, &p.Builtin)
	case "hidden":
		_ = json.Unmarshal(raw, &p.Hidden)
	case "rank":
		_ = json.Unmarshal(raw, &p.Rank)
	}
}

func setValue(v *Value, field string, raw json.RawMessage) {
	switch field {
	case "name":
		_ = json.Unmarshal(raw, &v.Name)
	case "color":
		_ = json.Unmarshal(raw, &v.Color)
	case "category":
		_ = json.Unmarshal(raw, &v.Category)
	case "rank":
		_ = json.Unmarshal(raw, &v.Rank)
	}
}

func byRank[T any](m map[change.ID]*T, rank func(*T) (string, change.ID)) []T {
	out := make([]T, 0, len(m))
	for _, x := range m {
		out = append(out, *x)
	}
	slices.SortFunc(out, func(a, b T) int {
		ra, ia := rank(&a)
		rb, ib := rank(&b)
		if c := strings.Compare(ra, rb); c != 0 {
			return c
		}
		return strings.Compare(string(ia), string(ib))
	})
	return out
}

func (b *builder) settings() Settings {
	s := b.acc
	s.Properties = byRank(b.props, func(p *Property) (string, change.ID) { return p.Rank, p.ID })
	valueRank := func(v *Value) (string, change.ID) { return v.Rank, v.ID }
	s.Statuses = byRank(b.vals["statuses"], valueRank)
	s.Priorities = byRank(b.vals["priorities"], valueRank)
	s.Systems = byRank(b.vals["systems"], valueRank)
	return s
}
