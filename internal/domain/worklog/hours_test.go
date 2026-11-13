package worklog_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

const (
	msgEmpty = "時間を入力してください"
	msgScale = "時間は小数第 2 位までで入力してください"
)

func message(err error) string {
	if r, ok := err.(*refusal.Refusal); ok {
		return r.Message
	}
	return ""
}

func TestParseHours(t *testing.T) {
	ok := map[string]int{
		"1": 100, "1.5": 150, "0.25": 25, "0.1": 10, ".5": 50, "1.": 100, "1.250": 125, "0.07": 7,
		" 2 ": 200, "12": 1200, "00.5": 50, "999999999": 99999999900,
	}
	for in, want := range ok {
		if got, err := worklog.ParseHours(in); err != nil || got != want {
			t.Errorf("ParseHours(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", " ", "0", "0.0", "0.000", "-1", "-0.5", "abc", "1e2", "1,5", ".", "１", "1000000000"} {
		if _, err := worklog.ParseHours(in); message(err) != msgEmpty {
			t.Errorf("ParseHours(%q) = %v; want %q", in, err, msgEmpty)
		}
	}
	for _, in := range []string{"0.123", "1.005", "0.001", "2.999"} {
		if _, err := worklog.ParseHours(in); message(err) != msgScale {
			t.Errorf("ParseHours(%q) = %v; want %q", in, err, msgScale)
		}
	}
}

func TestFormat(t *testing.T) {
	want := map[int]string{0: "0", 7: "0.07", 10: "0.1", 25: "0.25", 30: "0.3", 100: "1", 110: "1.1", 125: "1.25", 200: "2", 1234: "12.34"}
	for h, s := range want {
		if got := worklog.Format(h); got != s {
			t.Errorf("Format(%d) = %q, want %q", h, got, s)
		}
	}
}
