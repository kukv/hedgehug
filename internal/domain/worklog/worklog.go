package worklog

import (
	"cmp"
	"encoding/json"
	"math"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Entry は、いまの作業時間の記録 1 件。By は記録した人(最初の行の by)。Date は記録した日(YYYY-MM-DD)。
type Entry struct {
	ID         change.ID
	By         string
	Date       string
	Hundredths int
	Memo       string
}

type value struct {
	Hours float64 `json:"hours"`
	Memo  string  `json:"memo"`
	Date  string  `json:"date"`
}

// Value は、作業時間の値(行の from・to に書く形)。時間は JSON の数で書く。
func Value(hundredths int, memo, date string) any {
	return value{Hours: float64(hundredths) / 100, Memo: memo, Date: date}
}

func alive(r change.Record) bool { return !change.Absent(r.To) }

func read(raw json.RawMessage) (value, bool) {
	var v value
	if json.Unmarshal(raw, &v) != nil || math.Round(v.Hours*100) <= 0 {
		return value{}, false
	}
	return v, true
}

// FromLog は、ログのいまの作業時間を、記録の古い順(同じなら ID の順)で返す。削除されたものと、
// 読めないものは除く。いまの値は change.Current が決める。
func FromLog(log []change.Record) []Entry {
	ids := map[change.ID]bool{}
	for _, r := range log {
		if r.T == change.Worklog {
			ids[change.ID(r.F)] = true
		}
	}
	type dated struct {
		Entry
		recorded change.Record
	}
	var found []dated
	for id := range ids {
		if e, first, ok := find(log, id); ok {
			found = append(found, dated{Entry: e, recorded: first})
		}
	}
	slices.SortFunc(found, func(a, b dated) int {
		return cmp.Or(a.recorded.At.Compare(b.recorded.At), strings.Compare(string(a.ID), string(b.ID)))
	})
	out := make([]Entry, 0, len(found))
	for _, d := range found {
		out = append(out, d.Entry)
	}
	return out
}

// Find は、いまの記録を 1 件返す。ないか、削除されているか、読めなければ false。
func Find(log []change.Record, id change.ID) (Entry, bool) {
	e, _, ok := find(log, id)
	return e, ok
}

func find(log []change.Record, id change.ID) (Entry, change.Record, bool) {
	f := string(id)
	cur, ok := change.Current(log, change.Worklog, f, alive)
	if !ok || change.Absent(cur.To) {
		return Entry{}, change.Record{}, false
	}
	v, ok := read(cur.To)
	if !ok {
		return Entry{}, change.Record{}, false
	}
	first, _ := change.First(log, change.Worklog, f)
	return Entry{ID: id, By: first.By, Date: v.Date, Hundredths: int(math.Round(v.Hours * 100)), Memo: v.Memo}, first, true
}
