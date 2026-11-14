package query

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/planning"
)

var relativeDate = regexp.MustCompile(`^([+-]?)([0-9]{1,6})[dD]$`)

// ParseDate は、日付の値(`2026-12-01`・`today`・`+3d`・`3d`・`-7d`)を YYYY-MM-DD にする。大文字と小文字は区別しない。
// today は今日の YYYY-MM-DD。読めなければ false。
func ParseDate(v, today string) (string, bool) {
	if _, err := time.Parse(time.DateOnly, today); err != nil {
		return "", false
	}
	if strings.EqualFold(v, "today") {
		return today, true
	}
	if m := relativeDate.FindStringSubmatch(v); m != nil {
		n, _ := strconv.Atoi(m[2])
		if m[1] == "-" {
			n = -n
		}
		return planning.AddDays(today, n), true
	}
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		return "", false
	}
	return t.Format(time.DateOnly), true
}

// localDate は、t をこのマシンの時刻帯の日付にする。零の時刻は ""(値なし)。
func localDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(time.DateOnly)
}
