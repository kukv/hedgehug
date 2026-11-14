package bulkchange

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func TestParseInput(t *testing.T) {
	refused := func(msg string) func(error) bool {
		return func(err error) bool {
			var r *refusal.Refusal
			return errors.As(err, &r) && r.Message == msg
		}
	}
	invalid := func(err error) bool { return errors.Is(err, port.ErrInvalidOperation) }
	for _, c := range []struct {
		name string
		in   Input
		ok   func(error) bool
	}{
		{"negative points", Input{Field: FieldPoints, Value: "-1"}, refused("ポイントは 0 以上の数で入力してください")},
		{"estimate digits", Input{Field: FieldEstimate, Value: "1.234"}, refused("見積もり時間は小数第 2 位までで入力してください")},
		{"bad date", Input{Field: FieldStart, Value: "2026/11/14"}, func(err error) bool { return errors.Is(err, item.ErrBadInput) }},
		{"no systems", Input{Field: FieldSystemsAdd}, invalid},
		{"unknown field", Input{Field: "title"}, invalid},
		{"status both", Input{Field: FieldStatus, Value: "Done", Category: "done"}, invalid},
		{"status neither", Input{Field: FieldStatus}, invalid},
	} {
		if _, err := parseInput(c.in); !c.ok(err) {
			t.Errorf("%s: err = %v", c.name, err)
		}
	}
}

func TestParseInputValues(t *testing.T) {
	p, err := parseInput(Input{Field: FieldPoints, Value: ""})
	if err != nil || p.points != nil {
		t.Errorf("empty points = %+v, %v", p, err)
	}
	p, err = parseInput(Input{Field: FieldEstimate, Value: " 1.5 "})
	if err != nil || p.estimate == nil || *p.estimate != 1.5 {
		t.Errorf("estimate = %+v, %v", p, err)
	}
	p, err = parseInput(Input{Field: FieldEnd, Value: "2026-11-14"})
	if err != nil || p.date != "2026-11-14" {
		t.Errorf("date = %+v, %v", p, err)
	}
	p, err = parseInput(Input{Field: FieldSystemsRemove, Values: []string{" Web ", "", "iOS"}})
	if err != nil || len(p.names) != 2 || p.names[0] != "Web" || p.names[1] != "iOS" {
		t.Errorf("systems = %+v, %v", p, err)
	}
	p, err = parseInput(Input{Field: FieldStatus, Category: "done"})
	if err != nil || p.category != "done" || p.text != "" {
		t.Errorf("category = %+v, %v", p, err)
	}
	if _, err = parseInput(Input{Field: FieldStatus, Category: "wip"}); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("unknown category: %v", err)
	}
	p, err = parseInput(Input{Field: FieldSprint, Value: "  S-1 "})
	if err != nil || p.text != "S-1" {
		t.Errorf("sprint = %+v, %v", p, err)
	}
}

func TestFieldName(t *testing.T) {
	for _, c := range []struct {
		f        Field
		activity bool
		want     string
	}{
		{FieldStatus, false, "ステータス"},
		{FieldAssignee, true, "担当者"},
		{FieldPriority, false, "優先度"},
		{FieldSprint, false, "スプリント"},
		{FieldRelease, false, "リリース"},
		{FieldSystemsAdd, false, "関連システムを追加"},
		{FieldSystemsRemove, false, "関連システムを削除"},
		{FieldSystemsAdd, true, "関連システム"},
		{FieldSystemsRemove, true, "関連システム"},
		{FieldPoints, false, "ポイント"},
		{FieldEstimate, false, "見積もり時間"},
		{FieldStart, false, "開始日"},
		{FieldEnd, false, "終了日"},
	} {
		if got := fieldName(c.f, c.activity); got != c.want {
			t.Errorf("fieldName(%s, %v) = %q, want %q", c.f, c.activity, got, c.want)
		}
	}
}
