package project_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/project"
)

// 01「日付と時刻」: 今年なら M/D、ほかの年は YYYY/MM/DD。空は「なし」。
func TestDateText(t *testing.T) {
	today := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	cases := []struct{ in, want string }{
		{"2026-11-20", "11/20"},
		{"2026-01-05", "1/5"},
		{"2027-01-05", "2027/01/05"},
		{"", "なし"},
	}
	for _, c := range cases {
		if got := project.DateText(c.in, today); got != c.want {
			t.Errorf("DateText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
