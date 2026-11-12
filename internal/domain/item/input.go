package item

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ErrNoSuchValue は、選択肢にない値を指したとき(決定 18。API は 404)。
var ErrNoSuchValue = errors.New("no such value")

// ErrBadInput は、型に合わない入力(日付の形でない、数値型の数でない値など)。仕様の文言はない(決定 16)。
var ErrBadInput = errors.New("bad input")

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// CheckDates は、開始日と終了日が両方あり、終了日が開始日より前なら断る(02「入力の方法と保存」)。
func CheckDates(start, end string) error {
	if start != "" && end != "" && end < start {
		return refusal.New("終了日は開始日以降にしてください")
	}
	return nil
}

// ParsePoints は、ポイントの入力を読む。空は nil。0 未満と数でない値は断る(決定 16)。
func ParsePoints(s string) (*float64, error) {
	v, ok, empty := parseNumber(s)
	if empty {
		return nil, nil //nolint:nilnil // 空は値なし
	}
	if !ok || v < 0 {
		return nil, refusal.New("ポイントは 0 以上の数で入力してください")
	}
	return &v, nil
}

// ParseEstimate は、見積もり時間の入力を読む。空は nil(02「入力の方法と保存」の検査の表)。
func ParseEstimate(s string) (*float64, error) {
	v, ok, empty := parseNumber(s)
	if empty {
		return nil, nil //nolint:nilnil // 空は値なし
	}
	if !ok || v < 0 {
		return nil, refusal.New("見積もり時間は 0 以上の数で入力してください")
	}
	if math.Abs(v*100-math.Round(v*100)) > 1e-9 {
		return nil, refusal.New("見積もり時間は小数第 2 位までで入力してください")
	}
	return &v, nil
}

func parseNumber(s string) (v float64, ok, empty bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false, true
	}
	v, err := strconv.ParseFloat(s, 64)
	if v == 0 {
		v = 0 // 「-0」は 0 として書く
	}
	return v, err == nil && !math.IsInf(v, 0) && !math.IsNaN(v), false
}

// ParseInput は、プロパティ p への入力 in を、ログに書く値にする(決定 8・12)。
// 関連システムと複数選択は入り切りの操作で扱うので、ここでは ErrBadInput。
// 文字列で受けるところに、null と文字列以外の JSON が来たときは、null は空、ほかは ErrBadInput。
func ParseInput(p workspace.Property, in json.RawMessage, st workspace.Settings) (json.RawMessage, error) {
	if p.Type == workspace.TypeCheck {
		var b bool
		if json.Unmarshal(in, &b) != nil {
			return nil, ErrBadInput
		}
		return change.Value(b), nil
	}
	s, err := readText(in)
	if err != nil {
		return nil, err
	}
	switch p.Type {
	case workspace.TypeStatus:
		return pickValue(s, st.Statuses, false)
	case workspace.TypePriority:
		return pickValue(s, st.Priorities, false)
	case workspace.TypeSingle:
		return pickValue(s, choicesOf(st, p.ID), true)
	case workspace.TypeDate:
		if s != "" && !validDate(s) {
			return nil, ErrBadInput
		}
		return orNull(s), nil
	case workspace.TypeNumber, workspace.TypeDuration:
		return parseNumberInput(p, s)
	case workspace.TypeUser, workspace.TypeText, workspace.TypeURL:
		return orNull(s), nil
	}
	return nil, ErrBadInput
}

func parseNumberInput(p workspace.Property, s string) (json.RawMessage, error) {
	var v *float64
	var err error
	switch {
	case p.Key == "estimate" && p.Builtin:
		v, err = ParseEstimate(s)
	case p.Key == "points" && p.Builtin:
		v, err = ParsePoints(s)
	default:
		n, ok, empty := parseNumber(s)
		if !empty && !ok {
			return nil, ErrBadInput
		}
		if !empty {
			v = &n
		}
	}
	if err != nil {
		return nil, err
	}
	return change.Value(v), nil
}

// pickValue は、s が vs のどれかの内部 ID なら、その ID の値を返す。nullable なら空も認める。
func pickValue(s string, vs []workspace.Value, nullable bool) (json.RawMessage, error) {
	id := change.ID(s)
	if id == "" && nullable {
		return json.RawMessage("null"), nil
	}
	for _, v := range vs {
		if v.ID == id {
			return change.Value(id), nil
		}
	}
	return nil, ErrNoSuchValue
}

func choicesOf(st workspace.Settings, p change.ID) []workspace.Value {
	if prop, ok := st.Property(p); ok {
		return prop.Choices
	}
	return nil
}

// readText は、文字列の入力の前後の空白を除いたもの。null は空、文字列でない JSON は ErrBadInput。
func readText(in json.RawMessage) (string, error) {
	var s *string
	if json.Unmarshal(in, &s) != nil {
		return "", ErrBadInput
	}
	if s == nil {
		return "", nil
	}
	return strings.TrimSpace(*s), nil
}

func orNull(s string) json.RawMessage {
	if s == "" {
		return json.RawMessage("null")
	}
	return change.Value(s)
}

func validDate(s string) bool {
	if !datePattern.MatchString(s) {
		return false
	}
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}
