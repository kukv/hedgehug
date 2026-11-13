package project

import (
	"strconv"
	"time"
)

// DateText は、アクティビティの {値} に書く日付(01「日付と時刻」)。空は「なし」、今年なら M/D、
// ほかの年は YYYY/MM/DD。item の同じ規則(item は project を import するので、ここに置く)。
func DateText(s string, today time.Time) string {
	if s == "" {
		return "なし"
	}
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return s
	}
	if d.Year() == today.Year() {
		return strconv.Itoa(int(d.Month())) + "/" + strconv.Itoa(d.Day())
	}
	return d.Format("2006/01/02")
}
