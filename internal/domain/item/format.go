package item

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Names は、ログイン名から人の表示名を返す(メンバーにいなければ「不明なユーザー」)。
type Names func(login string) string

const deletedValue = "(削除された値)"

// Display は、アクティビティの {値} を作る(決定 15、01「画面の移り方と、共通の表示」)。
// v はログに書く形(Current・ParseInput の結果)。today は保存する日(日付の年を決める)。
func Display(p workspace.Property, v json.RawMessage, st workspace.Settings, names Names, today time.Time) string {
	if string(v) == "null" || len(v) == 0 {
		if p.Type == workspace.TypeCheck {
			return "オフ"
		}
		return "なし"
	}
	switch p.Type {
	case workspace.TypeStatus:
		return valueName(st.Statuses, change.ID(str(v)))
	case workspace.TypePriority:
		return valueName(st.Priorities, change.ID(str(v)))
	case workspace.TypeSingle:
		return valueName(choicesOf(st, p.ID), change.ID(str(v)))
	case workspace.TypeSystems:
		return joinNames(st.Systems, v)
	case workspace.TypeMulti:
		return joinNames(choicesOf(st, p.ID), v)
	case workspace.TypeUser:
		return names(str(v))
	case workspace.TypeDate:
		return dateText(str(v), today)
	case workspace.TypeCheck:
		var b bool
		_ = json.Unmarshal(v, &b)
		return map[bool]string{true: "オン", false: "オフ"}[b]
	case workspace.TypeDuration:
		if n := num(v); n != nil {
			return Hours(n) + " 時間"
		}
	case workspace.TypeNumber:
		if n := num(v); n != nil {
			return strconv.FormatFloat(*n, 'f', -1, 64)
		}
	}
	return str(v)
}

// Hours は、時間を小数第 2 位までで、末尾の 0 を省いて書く(02「時間の値」)。
// 100 倍すると無限大になるほど大きな値は、丸めずにそのまま書く(上限は設けない)。
func Hours(v *float64) string {
	r := math.Round(*v*100) / 100
	if math.IsInf(r, 0) {
		r = *v
	}
	return strconv.FormatFloat(r, 'f', -1, 64)
}

func valueName(vs []workspace.Value, id change.ID) string {
	for _, v := range vs {
		if v.ID == id {
			return v.Name
		}
	}
	return deletedValue
}

func joinNames(vs []workspace.Value, v json.RawMessage) string {
	var ids []change.ID
	_ = json.Unmarshal(v, &ids)
	if len(ids) == 0 {
		return "なし"
	}
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = valueName(vs, id)
	}
	return strings.Join(names, ", ")
}

// dateText は、01「日付と時刻」の表示(今年なら M/D、ほかの年は YYYY/MM/DD)。
func dateText(s string, today time.Time) string {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return s
	}
	if d.Year() == today.Year() {
		return strconv.Itoa(int(d.Month())) + "/" + strconv.Itoa(d.Day())
	}
	return d.Format("2006/01/02")
}
