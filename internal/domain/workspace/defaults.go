package workspace

import (
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

type builtin struct {
	key, name string
	typ       PropertyType
}

// builtins は 03「組み込みの 8 種」の表の並び。
var builtins = []builtin{
	{"status", "ステータス", TypeStatus},
	{"assignee", "担当者", TypeUser},
	{"priority", "優先度", TypePriority},
	{"start", "開始日", TypeDate},
	{"end", "終了日", TypeDate},
	{"points", "ポイント", TypeNumber},
	{"estimate", "見積もり時間", TypeDuration},
	{"systems", "関連システム", TypeSystems},
}

// rankAt は、初期値の並びの i 番目の順位(rank.First から rank.After を i 回)。辞書順で並ぶ。
func rankAt(i int) string {
	r := rank.First
	for range i {
		r = rank.After(r)
	}
	return r
}

func values(newID func() change.ID, vs []Value) []Value {
	for i := range vs {
		vs[i].ID = newID()
		vs[i].Rank = rankAt(i)
	}
	return vs
}

// DefaultSettings は、作ったワークスペースの初期値(03「ワークスペースの作成」)。値の内部 ID は newID で振る。
// 振る順は、ステータス 5、優先度 5、関連システム 6。
func DefaultSettings(newID func() change.ID) Settings {
	s := Settings{}
	for i, b := range builtins {
		s.Properties = append(s.Properties, Property{
			ID: change.ID(b.key), Key: b.key, Name: b.name, Type: b.typ, Builtin: true,
			Kinds: slices.Clone(Kinds), Rank: rankAt(i),
		})
	}
	s.Statuses = values(newID, []Value{
		{Name: "Todo", Color: "gray", Category: CategoryTodo},
		{Name: "InProgress", Color: "blue", Category: CategoryDoing},
		{Name: "Done", Color: "green", Category: CategoryDone},
		{Name: "Pending", Color: "yellow", Category: CategoryTodo},
		{Name: "Cancel", Color: "gray", Category: CategoryCanceled},
	})
	s.Priorities = values(newID, []Value{
		{Name: "Highest", Color: "red"},
		{Name: "High", Color: "orange"},
		{Name: "Medium", Color: "yellow"},
		{Name: "Low", Color: "blue"},
		{Name: "Lowest", Color: "gray"},
	})
	s.Systems = values(newID, []Value{
		{Name: "Web", Color: "blue"},
		{Name: "API", Color: "teal"},
		{Name: "iOS", Color: "gray"},
		{Name: "Android", Color: "green"},
		{Name: "DB", Color: "purple"},
		{Name: "CI", Color: "yellow"},
	})
	s.InitialStatus = s.Statuses[0].ID
	s.InitialPriority = s.Priorities[2].ID
	s.CategoryDefaults = map[Category]change.ID{
		CategoryTodo:     s.Statuses[0].ID,
		CategoryDoing:    s.Statuses[1].ID,
		CategoryDone:     s.Statuses[2].ID,
		CategoryCanceled: s.Statuses[4].ID,
	}
	return s
}
