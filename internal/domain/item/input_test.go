package item_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func seqIDs() func() change.ID {
	n := 0
	return func() change.ID { n++; return change.ID(fmt.Sprintf("01SEQ%021d", n)) }
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func refusedWith(t *testing.T, err error, want string) {
	t.Helper()
	var r *refusal.Refusal
	if !errors.As(err, &r) || r.Message != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestParseEstimate(t *testing.T) {
	for _, c := range [][2]string{{"1.5", "1.5"}, {"0", "0"}, {"2.25", "2.25"}, {"", "null"}, {" 3 ", "3"}} {
		v, err := item.ParseEstimate(c[0])
		if err != nil || string(mustJSON(v)) != c[1] {
			t.Errorf("ParseEstimate(%q) = %v, %v", c[0], v, err)
		}
	}
	_, err := item.ParseEstimate("-1")
	refusedWith(t, err, "見積もり時間は 0 以上の数で入力してください")
	_, err = item.ParseEstimate("abc")
	refusedWith(t, err, "見積もり時間は 0 以上の数で入力してください")
	_, err = item.ParseEstimate("0.123")
	refusedWith(t, err, "見積もり時間は小数第 2 位までで入力してください")
}

func TestParsePoints(t *testing.T) {
	if v, err := item.ParsePoints("2.5"); err != nil || *v != 2.5 {
		t.Errorf("ParsePoints = %v, %v", v, err)
	}
	for _, in := range []string{"-0.5", "x"} {
		_, err := item.ParsePoints(in)
		refusedWith(t, err, "ポイントは 0 以上の数で入力してください")
	}
}

func TestCheckDates(t *testing.T) {
	if item.CheckDates("2026-11-11", "2026-11-11") != nil || item.CheckDates("", "2026-11-01") != nil {
		t.Error("same day and one-sided must pass")
	}
	refusedWith(t, item.CheckDates("2026-11-11", "2026-11-10"), "終了日は開始日以降にしてください")
}

// byKey は、設定の組み込みのプロパティ(キー k)。
func byKey(t *testing.T, st workspace.Settings, k string) workspace.Property {
	t.Helper()
	for _, p := range st.Properties {
		if p.Key == k {
			return p
		}
	}
	t.Fatalf("no %s", k)
	return workspace.Property{}
}

func TestParseInputNormalizes(t *testing.T) {
	st := workspace.DefaultSettings(seqIDs())
	prop := func(k string) workspace.Property { return byKey(t, st, k) }
	cases := []struct {
		p    workspace.Property
		in   string
		want string
	}{
		{prop("status"), `"` + string(st.Statuses[1].ID) + `"`, `"` + string(st.Statuses[1].ID) + `"`},
		{prop("assignee"), `""`, `null`},
		{prop("assignee"), `"taro"`, `"taro"`},
		{prop("start"), `"2026-11-11"`, `"2026-11-11"`},
		{prop("start"), `null`, `null`},
		{prop("points"), `"3"`, `3`},
		{prop("estimate"), `"1.50"`, `1.5`},
		// 「-0」は 0 として書く
		{prop("points"), `"-0"`, `0`},
		{prop("estimate"), `"-0"`, `0`},
		{workspace.Property{ID: fNote, Type: workspace.TypeNumber}, `"-0"`, `0`},
		{workspace.Property{ID: fNote, Type: workspace.TypeText}, `"  memo  "`, `"memo"`},
		{workspace.Property{ID: fNote, Type: workspace.TypeURL}, `"   "`, `null`},
		{workspace.Property{ID: fDone, Type: workspace.TypeCheck}, `true`, `true`},
	}
	for _, c := range cases {
		got, err := item.ParseInput(c.p, json.RawMessage(c.in), st)
		if err != nil || string(got) != c.want {
			t.Errorf("ParseInput(%s, %s) = %s, %v; want %s", c.p.Key, c.in, got, err, c.want)
		}
	}
}

func TestParseInputRejectsUnknownValues(t *testing.T) {
	st := workspace.DefaultSettings(seqIDs())
	status := st.Properties[0]
	if _, err := item.ParseInput(status, json.RawMessage(`"01NOPE00000000000000000000"`), st); !errors.Is(err, item.ErrNoSuchValue) {
		t.Errorf("err = %v", err)
	}
	if _, err := item.ParseInput(status, json.RawMessage(`null`), st); !errors.Is(err, item.ErrNoSuchValue) {
		t.Errorf("status cannot be empty: %v", err)
	}
	date := workspace.Property{Key: "start", Builtin: true, Type: workspace.TypeDate}
	if _, err := item.ParseInput(date, json.RawMessage(`"11/11"`), st); !errors.Is(err, item.ErrBadInput) {
		t.Errorf("err = %v", err)
	}
}

func TestParseInputRejectsNonStringJSON(t *testing.T) {
	st := workspace.DefaultSettings(seqIDs())
	prop := func(k string) workspace.Property { return byKey(t, st, k) }
	cases := []struct {
		p  workspace.Property
		in string
	}{
		{prop("start"), `true`},
		{prop("points"), `3`},
		{prop("estimate"), `[]`},
		{prop("assignee"), `{}`},
		{prop("status"), `1`},
		{workspace.Property{ID: fNote, Type: workspace.TypeText}, `false`},
		{workspace.Property{ID: fNote, Type: workspace.TypeURL}, `7`},
		{workspace.Property{ID: fCount, Type: workspace.TypeNumber}, `2`},
	}
	for _, c := range cases {
		if _, err := item.ParseInput(c.p, json.RawMessage(c.in), st); !errors.Is(err, item.ErrBadInput) {
			t.Errorf("ParseInput(%v, %s) err = %v, want ErrBadInput", c.p.Type, c.in, err)
		}
	}
}
