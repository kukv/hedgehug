package query

import (
	"cmp"
	"math"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/mention"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// matchRest は、名前・ID 以外のキー(数・日付・スプリント・リリース・is・has・mentions・追加プロパティ)を判定する。
// 比較演算子は数と日付のキーだけが使い、ほかのキーは `:` として読む(04 の 233 行)。
func matchRest(t Term, f Facts, today string) bool {
	switch t.Key {
	case "points":
		return matchNumber(t, f.Item.Points)
	case "estimate":
		return matchNumber(t, f.Item.Estimate)
	case "start":
		return matchDate(t, f.Item.Start, today)
	case "end":
		return matchDate(t, f.Item.End, today)
	case "created":
		return matchDate(t, localDate(f.Created), today)
	case "updated":
		return matchDate(t, localDate(f.Updated), today)
	case "sprint":
		return anyValue(t, func(v string) bool { return sprint(f, v) })
	case "release":
		return anyValue(t, func(v string) bool { return isNoneOr(v, f.ReleaseName, strings.EqualFold(f.ReleaseName, v)) })
	case "is":
		return anyValue(t, func(v string) bool { return state(f, v, today) })
	case "has":
		return anyValue(t, func(v string) bool { return has(f, v) })
	case "mentions":
		return anyValue(t, func(v string) bool { return mentions(f, v) })
	}
	return matchCustom(t, f, today)
}

// isNoneOr は、v が none なら name が空か、そうでなければ eq。
func isNoneOr(v, name string, eq bool) bool {
	if isNone(v) {
		return name == ""
	}
	return eq
}

func compare(op Op, c int) bool {
	switch op {
	case OpLt:
		return c < 0
	case OpLe:
		return c <= 0
	case OpGt:
		return c > 0
	case OpGe:
		return c >= 0
	}
	return c == 0
}

// matchNumber は、数の値を比べる。値がない・読めない数は一致しない。`none` は値なし。
func matchNumber(t Term, have *float64) bool {
	return anyValue(t, func(v string) bool {
		if isNone(v) {
			return have == nil
		}
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || have == nil {
			return false
		}
		return compare(t.Op, cmp.Compare(*have, n))
	})
}

// matchDate は、日付(YYYY-MM-DD)を比べる。値がない・読めない日付は一致しない。`none` は値なし。
func matchDate(t Term, have, today string) bool {
	return anyValue(t, func(v string) bool {
		if isNone(v) {
			return have == ""
		}
		d, ok := ParseDate(v, today)
		if !ok || have == "" {
			return false
		}
		return compare(t.Op, strings.Compare(have, d))
	})
}

func sprint(f Facts, v string) bool {
	switch strings.ToLower(v) {
	case "current":
		return f.SprintCurrent
	case "next":
		return f.SprintNext
	}
	return isNoneOr(v, f.SprintName, strings.EqualFold(f.SprintName, v))
}

// state は、`is:` の値 1 つ。表にない値は何にも一致しない。
func state(f Facts, v, today string) bool {
	switch strings.ToLower(v) {
	case "blocked":
		return f.Item.Blocked
	case "overdue":
		return overdue(f, today)
	case "invalid":
		return f.Invalid
	case "deleted":
		return f.Item.Deleted != nil
	case "overwritten":
		return f.Overwritten
	}
	return false
}

// overdue は、未完了(分類が todo か doing)で終了日が今日より前(04 の集計の定義)。
func overdue(f Facts, today string) bool {
	if f.Item.End == "" || f.Item.End >= today {
		return false
	}
	for _, s := range f.Settings.Statuses {
		if s.ID == f.Item.Status {
			return s.Category == workspace.CategoryTodo || s.Category == workspace.CategoryDoing
		}
	}
	return false
}

// has は、`has:` の値 1 つ。組み込みと追加プロパティのキー、sprint・release・pr。表にない値は一致しない。
func has(f Facts, v string) bool {
	it := f.Item
	switch strings.ToLower(v) {
	case "sprint":
		return f.SprintName != ""
	case "release":
		return f.ReleaseName != ""
	case "pr":
		return f.HasPR
	case "status":
		return it.Status != ""
	case "priority":
		return it.Priority != ""
	case "assignee":
		return it.Assignee != ""
	case "start":
		return it.Start != ""
	case "end":
		return it.End != ""
	case "points":
		return it.Points != nil
	case "estimate":
		return it.Estimate != nil
	case "systems":
		return len(it.Systems) > 0
	}
	p, ok := customProperty(f, strings.ToLower(v))
	return ok && hasField(f, p)
}

// mentions は、説明と(削除されていない)コメントのメンションを判定する(E1-14)。none はどこにもない。
func mentions(f Facts, v string) bool {
	login := strings.TrimPrefix(v, "@")
	bodies := append([]string{f.Description}, f.Comments...)
	if isNone(login) {
		for _, b := range bodies {
			if mention.Any(b) {
				return false
			}
		}
		return true
	}
	if strings.EqualFold(login, "me") {
		login = f.Me
	}
	for _, b := range bodies {
		if mention.Has(b, login) {
			return true
		}
	}
	return false
}
