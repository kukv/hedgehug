package planning

import (
	"strconv"
	"time"
)

// Today は、now をこのマシンの時刻帯で日付にしたもの(F1-9)。
func Today(now time.Time) string { return now.Local().Format(time.DateOnly) }

// AddDays は、日付 ymd の n 日後。読めない日付はそのまま返す。
func AddDays(ymd string, n int) string {
	t, err := time.Parse(time.DateOnly, ymd)
	if err != nil {
		return ymd
	}
	return t.AddDate(0, 0, n).Format(time.DateOnly)
}

// RemainingDays は、今日から終了日までの日数(終了日の当日は 1)。終了日を過ぎていたら overdue。
func RemainingDays(end, today string) (n int, overdue bool) {
	e, err1 := time.Parse(time.DateOnly, end)
	t, err2 := time.Parse(time.DateOnly, today)
	if err1 != nil || err2 != nil || e.Before(t) {
		return 0, true
	}
	return int(e.Sub(t).Hours()/24) + 1, false
}

// SprintDefaults は、作成の既定の名前・開始日・終了日(F1-7)。live は削除済みを除いたスプリント。
func SprintDefaults(live []Sprint, today string) (name, start, end string) {
	base, ok := baseSprint(live)
	start = today
	if ok {
		start = AddDays(base.End, 1)
	}
	end = AddDays(start, 13)
	name = "Sprint " + strconv.Itoa(len(live)+1)
	if next, found := nextNumber(base.Name); ok && found {
		name = next
	}
	for taken(name, live) {
		next, found := nextNumber(name)
		if !found {
			break
		}
		name = next
	}
	return name, start, end
}

// baseSprint は、開始日の基準にするスプリント。終了日が最も遅いもの、同じなら開始日が遅いもの、それも同じなら名前の昇順で後のもの。
func baseSprint(live []Sprint) (Sprint, bool) {
	var base Sprint
	found := false
	for _, s := range live {
		if s.End != "" && (!found || laterBase(s, base)) {
			base, found = s, true
		}
	}
	return base, found
}

func laterBase(a, b Sprint) bool {
	if a.End != b.End {
		return a.End > b.End
	}
	if a.Start != b.Start {
		return a.Start > b.Start
	}
	return a.Name > b.Name
}

func taken(name string, live []Sprint) bool {
	for _, s := range live {
		if NormalizeName(s.Name) == name {
			return true
		}
	}
	return false
}

// nextNumber は、名前の末尾の数字の並びに 1 を足した名前。桁は保つ(`Sprint 07` は `Sprint 08`、`S-099` は `S-100`)。
func nextNumber(name string) (string, bool) {
	i := len(name)
	for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
		i--
	}
	if i == len(name) {
		return "", false
	}
	digits := []byte(name[i:])
	for j := len(digits) - 1; j >= 0; j-- {
		if digits[j] < '9' {
			digits[j]++
			return name[:i] + string(digits), true
		}
		digits[j] = '0'
	}
	return name[:i] + "1" + string(digits), true
}
