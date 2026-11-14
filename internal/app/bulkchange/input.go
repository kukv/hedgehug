package bulkchange

import (
	"cmp"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Field は、一括で変える項目。
type Field string

// 一括で変える項目(F2b2-1。この順がダイアログの項目のセレクトの順)。
const (
	FieldStatus        Field = "status"
	FieldAssignee      Field = "assignee"
	FieldPriority      Field = "priority"
	FieldSprint        Field = "sprint"
	FieldRelease       Field = "release"
	FieldSystemsAdd    Field = "systemsAdd"
	FieldSystemsRemove Field = "systemsRemove"
	FieldPoints        Field = "points"
	FieldEstimate      Field = "estimate"
	FieldStart         Field = "start"
	FieldEnd           Field = "end"
)

// categories は、分類の一覧。
var categories = []workspace.Category{
	workspace.CategoryTodo, workspace.CategoryDoing, workspace.CategoryDone, workspace.CategoryCanceled,
}

// Input は、変える項目と値。値は名前で送る(担当者はログイン名。F2b2-2)。Category は、ステータスを分類で
// 変えるときの分類(Value とどちらか一方)。
type Input struct {
	Field    Field
	Value    string
	Values   []string
	Category workspace.Category
}

// fieldName は、項目の表示名(F2b2-1)。activity が true なら、関連システムの 2 つは「関連システム」
// (アクティビティと理由)。
func fieldName(f Field, activity bool) string {
	switch f {
	case FieldStatus:
		return "ステータス"
	case FieldAssignee:
		return "担当者"
	case FieldPriority:
		return "優先度"
	case FieldSprint:
		return "スプリント"
	case FieldRelease:
		return "リリース"
	case FieldSystemsAdd:
		if activity {
			return "関連システム"
		}
		return "関連システムを追加"
	case FieldSystemsRemove:
		if activity {
			return "関連システム"
		}
		return "関連システムを削除"
	case FieldPoints:
		return "ポイント"
	case FieldEstimate:
		return "見積もり時間"
	case FieldStart:
		return "開始日"
	case FieldEnd:
		return "終了日"
	}
	return string(f)
}

// parsed は、検査した入力。text は前後の空白を除いた名前(空は「なし」)。
type parsed struct {
	field            Field
	text             string
	points, estimate *float64
	date             string
	names            []string
	category         workspace.Category
}

// parseInput は、入力を検査する(F2b2-11)。ポイント・見積もり時間・日付の不正は断りか item.ErrBadInput、
// 項目が 11 個にない・関連システムの値が 0 個・ステータスの名前と分類が両方か両方なしは
// port.ErrInvalidOperation。
func parseInput(in Input) (parsed, error) {
	p := parsed{field: in.Field, text: strings.TrimSpace(in.Value)}
	var err error
	switch in.Field {
	case FieldStatus:
		if (p.text == "") == (in.Category == "") || (in.Category != "" && !slices.Contains(categories, in.Category)) {
			return parsed{}, port.ErrInvalidOperation
		}
		p.category = in.Category
	case FieldAssignee, FieldPriority, FieldSprint, FieldRelease:
	case FieldSystemsAdd, FieldSystemsRemove:
		for _, v := range in.Values {
			if v = strings.TrimSpace(v); v != "" {
				p.names = append(p.names, v)
			}
		}
		if len(p.names) == 0 {
			return parsed{}, port.ErrInvalidOperation
		}
	case FieldPoints:
		p.points, err = item.ParsePoints(in.Value)
	case FieldEstimate:
		p.estimate, err = item.ParseEstimate(in.Value)
	case FieldStart, FieldEnd:
		p.date, err = parseDate(p.text)
	default:
		return parsed{}, port.ErrInvalidOperation
	}
	if err != nil {
		return parsed{}, err
	}
	return p, nil
}

// parseDate は、日付の入力を、1 件の編集と同じ形の検査で読む。空は空。
func parseDate(s string) (string, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	if _, err := item.ParseInput(workspace.Property{Type: workspace.TypeDate}, raw, workspace.Settings{}); err != nil {
		return "", err
	}
	return s, nil
}

// valueText は、確認の本文の {値}(F2b2-16)。名前は送った名前、担当者は first のメンバーのうち最初に一致した
// ものの表示名、空は「なし」(スプリントは「バックログ」)。分類で変えるときは空。now は日付の表示の年を決める。
func valueText(in parsed, first itemContext, now time.Time) string {
	switch in.field {
	case FieldStatus:
		if in.category != "" {
			return ""
		}
	case FieldAssignee:
		i := slices.IndexFunc(first.members, func(m member.Member) bool { return sameName(m.Login, in.text) })
		if in.text != "" && i >= 0 {
			return first.members[i].Name
		}
	case FieldSprint:
		return cmp.Or(in.text, "バックログ")
	case FieldSystemsAdd, FieldSystemsRemove:
		return strings.Join(in.names, ", ")
	case FieldPoints:
		if in.points != nil {
			return strconv.FormatFloat(*in.points, 'f', -1, 64)
		}
	case FieldEstimate:
		if in.estimate != nil {
			return item.Hours(in.estimate) + " 時間"
		}
	case FieldStart, FieldEnd:
		if in.date != "" {
			raw, _ := json.Marshal(in.date)
			return item.Display(workspace.Property{Type: workspace.TypeDate}, raw, workspace.Settings{}, nil, now)
		}
	}
	return cmp.Or(in.text, "なし")
}
