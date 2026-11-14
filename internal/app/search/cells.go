package search

import (
	"slices"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	unknownUser  = "不明なユーザー"
	removedValue = "(削除された値)"
)

// Cell は、追加プロパティの列のセル。Type はアイテムのワークスペースのプロパティの型(列の型と違ってもよい。E2a-9)。
type Cell struct {
	Text string
	Type workspace.PropertyType
}

// Cells は、1 行の値の表示の材料。Unused は「—」にする列、Removed は「(削除された値)」にする列(どちらも列の鍵)、
// Custom は追加プロパティの列の鍵 → 値、Systems は関連システムの名前(定義順。一覧にない ID は「(削除された値)」)。
type Cells struct {
	Unused  []string
	Removed []string
	Custom  map[string]Cell
	Systems []string
}

// cellsOf は、1 行 f の、columns の各列の表示の材料(E2a-9〜E2a-11)。
func cellsOf(f query.Facts, columns []Column) Cells {
	cells := Cells{Custom: map[string]Cell{}}
	for _, c := range columns {
		switch {
		case c.Builtin != "":
			cells.builtin(f, c)
		case c.Key == "sprint" || c.Key == "release":
			if !slices.Contains(sprintKinds(c.Key), f.Item.Kind) {
				cells.Unused = append(cells.Unused, c.Key)
			}
		case strings.HasPrefix(c.Key, "p:"):
			cells.custom(f, c.Key)
		}
	}
	return cells
}

func sprintKinds(key string) []item.Kind {
	if key == "sprint" {
		return []item.Kind{item.Story, item.Task, item.Bug}
	}
	return []item.Kind{item.Epic, item.Story, item.Task, item.Bug}
}

// builtin は、組み込みのプロパティの列。ステータス以外は、使わない種類なら「—」。
func (cells *Cells) builtin(f query.Facts, c Column) {
	if c.Builtin != "status" && !usedBy(f.Settings, c.Builtin, f.Item.Kind) {
		cells.Unused = append(cells.Unused, c.Key)
		return
	}
	it, st := f.Item, f.Settings
	switch c.Builtin {
	case "status":
		cells.removedIf(c.Key, missing(st.Statuses, it.Status))
	case "priority":
		cells.removedIf(c.Key, missing(st.Priorities, it.Priority))
	case "systems":
		cells.Systems, _ = namesIn(st.Systems, it.Systems)
		cells.removedIf(c.Key, slices.Contains(cells.Systems, removedValue))
	}
}

func (cells *Cells) removedIf(key string, removed bool) {
	if removed {
		cells.Removed = append(cells.Removed, key)
	}
}

// custom は、追加プロパティの列。アイテムのワークスペースにそのキーのプロパティがないか、使わない種類なら「—」。
func (cells *Cells) custom(f query.Facts, key string) {
	p, ok := propertyOf(f.Settings, strings.TrimPrefix(key, "p:"))
	if !ok || !slices.Contains(p.Kinds, string(f.Item.Kind)) {
		cells.Unused = append(cells.Unused, key)
		return
	}
	text, removed := customText(f, p)
	cells.Custom[key] = Cell{Text: text, Type: p.Type}
	cells.removedIf(key, removed)
}

// usedBy は、組み込みのプロパティ key を、kind が使うか。
func usedBy(st workspace.Settings, key string, kind item.Kind) bool {
	for _, p := range st.Properties {
		if p.Builtin && p.Key == key {
			return slices.Contains(p.Kinds, string(kind))
		}
	}
	return false
}

func propertyOf(st workspace.Settings, lower string) (workspace.Property, bool) {
	for _, p := range st.Properties {
		if !p.Builtin && strings.ToLower(p.Key) == lower {
			return p, true
		}
	}
	return workspace.Property{}, false
}

// customText は、追加プロパティ p のセルの文字と、値が一覧にないか。値がなければ空。
func customText(f query.Facts, p workspace.Property) (string, bool) {
	switch v := item.FieldValue(f.Item, p).(type) {
	case change.ID:
		if v == "" {
			return "", false
		}
		names, removed := namesIn(p.Choices, []change.ID{v})
		return names[0], removed
	case []change.ID:
		names, removed := namesIn(p.Choices, v)
		return strings.Join(names, ", "), removed
	case *float64:
		if v == nil {
			return "", false
		}
		return strconv.FormatFloat(*v, 'f', -1, 64), false
	case bool:
		if v {
			return "true", false
		}
		return "", false
	case string:
		if p.Type == workspace.TypeUser && v != "" {
			if name, ok := f.Names[strings.ToLower(v)]; ok {
				return name, false
			}
			return unknownUser, false
		}
		return v, false
	}
	return "", false
}

// missing は、id があるのに vs にないか。
func missing(vs []workspace.Value, id change.ID) bool {
	return id != "" && !slices.ContainsFunc(vs, func(v workspace.Value) bool { return v.ID == id })
}

// namesIn は、ids の名前。vs の定義順に並べ、vs にない ID は最後に「(削除された値)」として並べる。
func namesIn(vs []workspace.Value, ids []change.ID) ([]string, bool) {
	var names []string
	for _, v := range vs {
		if slices.Contains(ids, v.ID) {
			names = append(names, v.Name)
		}
	}
	removed := false
	for _, id := range ids {
		if missing(vs, id) {
			names = append(names, removedValue)
			removed = true
		}
	}
	return names, removed
}
