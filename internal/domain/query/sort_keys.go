package query

import (
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// kindOrder は、種類の並び(Epic、Story、Task、Bug、Step)。
var kindOrder = []item.Kind{item.Epic, item.Story, item.Task, item.Bug, item.Step}

func one(v sortVal) []sortVal { return []sortVal{v} }

func text(s string) []sortVal {
	if s == "" {
		return nil
	}
	return one(sortVal{s: s})
}

func number(n *float64) []sortVal {
	if n == nil {
		return nil
	}
	return one(sortVal{f: *n})
}

func moment(t time.Time) []sortVal {
	if t.IsZero() {
		return nil
	}
	return one(sortVal{i: t.UnixNano()})
}

// displayName は、ログイン名の表示名。名簿(Names)にないメンバーはログイン名をそのまま使う。
func displayName(f Facts, login string) string {
	if n, ok := f.Names[strings.ToLower(login)]; ok {
		return n
	}
	return login
}

// idVal は、HH-12 の形の ID を、接頭辞の文字列 → 番号の順に比べる値にする(HH-2 が HH-10 の前になる)。
func idVal(display string) []sortVal {
	if display == "" {
		return nil
	}
	i := strings.LastIndex(display, "-")
	if i < 0 {
		return one(sortVal{s: display})
	}
	n, err := strconv.Atoi(display[i+1:])
	if err != nil {
		return one(sortVal{s: display})
	}
	return one(sortVal{s: display[:i], f: float64(n)})
}

func refID(r *Ref) []sortVal {
	if r == nil {
		return nil
	}
	return idVal(r.Display)
}

func kindVal(f Facts) []sortVal {
	for i, k := range kindOrder {
		if k == f.Item.Kind {
			return one(sortVal{f: float64(i)})
		}
	}
	return nil
}

// builtinValue は、組み込みのキーの値を取り出す関数。表にないキーは nil。
// ranks は、選択肢の定義順を使うキー(status・priority・system)のときだけ呼ぶ。
func builtinValue(key string, ranks func() choiceRanks) func(Facts) []sortVal {
	simple := map[string]func(Facts) []sortVal{
		"id":       func(f Facts) []sortVal { return one(sortVal{f: float64(f.Item.Place.Number)}) },
		"title":    func(f Facts) []sortVal { return text(f.Item.Title) },
		"kind":     kindVal,
		"assignee": func(f Facts) []sortVal { return text(displayName(f, f.Item.Assignee)) },
		"project":  func(f Facts) []sortVal { return text(f.ProjectTitle) },
		"parent":   func(f Facts) []sortVal { return refID(f.Parent) },
		"sprint":   func(f Facts) []sortVal { return text(f.SprintName) },
		"release":  func(f Facts) []sortVal { return text(f.ReleaseName) },
		"start":    func(f Facts) []sortVal { return text(f.Item.Start) },
		"end":      func(f Facts) []sortVal { return text(f.Item.End) },
		"points":   func(f Facts) []sortVal { return number(f.Item.Points) },
		"estimate": func(f Facts) []sortVal { return number(f.Item.Estimate) },
		"created":  func(f Facts) []sortVal { return moment(f.Created) },
		"updated":  func(f Facts) []sortVal { return moment(f.Updated) },
	}
	if fn, ok := simple[key]; ok {
		return fn
	}
	switch key {
	case "status", "priority", "system":
		r := ranks()
		return func(f Facts) []sortVal {
			values := choiceValues(f, key)
			switch key {
			case "status":
				return rankedVals(idsOf(f.Item.Status), values, r)
			case "priority":
				return rankedVals(idsOf(f.Item.Priority), values, r)
			}
			return rankedVals(f.Item.Systems, values, r)
		}
	}
	return nil
}

// customValue は、追加プロパティ key(小文字)の値を型ごとに取り出す関数。持たないワークスペースの行は値なし。
func customValue(key string, ranks func() choiceRanks) func(Facts) []sortVal {
	r := ranks()
	return func(f Facts) []sortVal {
		p, ok := customProperty(f, key)
		if !ok {
			return nil
		}
		switch val := item.FieldValue(f.Item, p).(type) {
		case *float64:
			return number(val)
		case bool:
			if val {
				return one(sortVal{f: 1}) // チェックはオンだけが「値がある」(hasField と同じ)
			}
		case string:
			if p.Type == workspace.TypeUser {
				return text(displayName(f, val))
			}
			return text(val)
		}
		if p.Type == workspace.TypeSingle || p.Type == workspace.TypeMulti {
			return rankedVals(f.Item.Choices[p.ID], p.Choices, r)
		}
		return nil
	}
}

// idsOf は、単一の値 ID を ids の形にする。空は値なし。
func idsOf(id change.ID) []change.ID {
	if id == "" {
		return nil
	}
	return []change.ID{id}
}
