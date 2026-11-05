// Package report computes the reports of a store from the versions of its
// tasks. It holds no I/O: what it reads is given as values.
package report

import (
	"math"
	"slices"
	"time"
)

// Unknown is the category of a status the schema no longer has.
const Unknown = "unknown"

// The categories the reports tell apart.
const (
	catDone     = "done"
	catDoing    = "doing"
	catCanceled = "canceled"
)

// Version is what a report reads of one version of a task file.
type Version struct {
	Commit    string
	When      time.Time
	Status    string
	Sprint    string // the ULID; "" for none
	Workspace string
	Created   time.Time
	Points    float64
	HasPoints bool
	Deleted   bool
	Gone      bool // the file is a forward, or was removed: the task left
	MovedIn   bool // the commit brought the task from another store
}

// Task is a task with its versions, oldest first.
type Task struct {
	ULID     string
	Versions []Version
}

// at is the version of t at the instant, leaving out the commit skip.
func (t Task) at(instant time.Time, skip string) (Version, bool) {
	var found Version
	ok := false
	for _, v := range t.Versions {
		if v.When.After(instant) {
			break
		}
		if skip != "" && v.Commit == skip {
			continue
		}
		found, ok = v, true
	}
	return found, ok
}

// Category gives the category of a status, Unknown for one the schema lacks.
type Category func(status string) string

// Day is one day of a report: At is the instant it is read at.
type Day struct {
	Date string
	At   time.Time
}

// Days lists the days from first to last, the dates in loc. The instant of a
// day is its end, or cutoff when that comes first.
func Days(first, last, cutoff time.Time, loc *time.Location) []Day {
	y, m, d := first.In(loc).Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, loc)
	end := last.In(loc)
	var out []Day
	for !day.After(end) {
		next := day.AddDate(0, 0, 1)
		at := next.Add(-time.Nanosecond)
		if at.After(cutoff) {
			at = cutoff
		}
		out = append(out, Day{Date: day.Format(time.DateOnly), At: at})
		day = next
	}
	return out
}

// Window is the time a sprint ran.
type Window struct {
	Sprint      string // the ULID
	Start       time.Time
	End         time.Time // zero while it runs
	CloseCommit string    // the commit that closed it; its versions are the move out
}

// BurndownDay is one day of a burndown.
type BurndownDay struct {
	Date      string  `json:"date"`
	Scope     float64 `json:"scope" doc:"What the sprint holds at the end of the day"`
	Done      float64 `json:"done"`
	Remaining float64 `json:"remaining"`
	Added     float64 `json:"added" doc:"What joined that day, after the sprint started"`
}

func value(v Version, byPoints bool) float64 {
	if !byPoints {
		return 1
	}
	return v.Points
}

// inScope reports whether the version counts for the sprint.
func inScope(v Version, sprint string, cat Category) bool {
	return v.Sprint == sprint && !v.Deleted && !v.Gone && cat(v.Status) != catCanceled
}

// Burndown is the sprint w day by day. byPoints sums the points; otherwise it
// counts tasks. A task in the sprint at w.Start is its initial scope, and one
// that joins later counts as added on the day it joined.
func Burndown(tasks []Task, w Window, days []Day, cat Category, byPoints bool) []BurndownDay {
	seen := map[string]bool{}
	for _, t := range tasks {
		if v, ok := t.at(w.Start, ""); ok && inScope(v, w.Sprint, cat) {
			seen[t.ULID] = true
		}
	}
	out := make([]BurndownDay, 0, len(days))
	for _, d := range days {
		row := BurndownDay{Date: d.Date}
		for _, t := range tasks {
			v, ok := t.at(d.At, w.CloseCommit)
			if !ok || !inScope(v, w.Sprint, cat) {
				continue
			}
			row.Scope += value(v, byPoints)
			if cat(v.Status) == catDone {
				row.Done += value(v, byPoints)
			}
			if !seen[t.ULID] && d.At.After(w.Start) {
				seen[t.ULID] = true
				row.Added += value(v, byPoints)
			}
		}
		row.Remaining = row.Scope - row.Done
		out = append(out, row)
	}
	return out
}

// HasPoints reports whether a task in the sprint has points at any version.
func HasPoints(tasks []Task, sprint string) bool {
	for _, t := range tasks {
		for _, v := range t.Versions {
			if v.Sprint == sprint && v.HasPoints {
				return true
			}
		}
	}
	return false
}

// Completed is what a sprint finished by its end.
type Completed struct {
	Points    float64 `json:"points"`
	Tasks     int     `json:"tasks"`
	Unpointed int     `json:"unpointed" doc:"Those of the tasks with no points, which count as 0"`
}

// Velocity adds up the tasks of the closed sprint w that were done at its end.
func Velocity(tasks []Task, w Window, cat Category) Completed {
	var c Completed
	for _, t := range tasks {
		v, ok := t.at(w.End, w.CloseCommit)
		if !ok || !inScope(v, w.Sprint, cat) || cat(v.Status) != catDone {
			continue
		}
		c.Tasks++
		c.Points += v.Points
		if !v.HasPoints {
			c.Unpointed++
		}
	}
	return c
}

// CFDDay is one day of a cumulative flow diagram.
type CFDDay struct {
	Date   string         `json:"date"`
	Counts map[string]int `json:"counts"`
}

// CFD counts the tasks of each key of key, day by day. An empty workspace is
// all of them.
func CFD(tasks []Task, days []Day, workspace string, key func(status string) string) []CFDDay {
	out := make([]CFDDay, 0, len(days))
	for _, d := range days {
		row := CFDDay{Date: d.Date, Counts: map[string]int{}}
		for _, t := range tasks {
			v, ok := t.at(d.At, "")
			if ok && !v.Deleted && !v.Gone && (workspace == "" || v.Workspace == workspace) {
				row.Counts[key(v.Status)]++
			}
		}
		out = append(out, row)
	}
	return out
}

// Times is how long a task took, once it was finished.
type Times struct {
	ULID  string
	Done  time.Time
	Cycle time.Duration // 0 with HasCycle false: it never was in doing
	Lead  time.Duration
	// HasCycle says the task went through the doing category.
	HasCycle bool
}

// Finished lists the tasks that are done now and entered done for the last
// time between since and until (inclusive), the workspace empty for all.
func Finished(tasks []Task, workspace string, since, until time.Time, cat Category) []Times {
	var out []Times
	for _, t := range tasks {
		n := len(t.Versions)
		if n == 0 {
			continue
		}
		last := t.Versions[n-1]
		if cat(last.Status) != catDone || last.Deleted || last.Gone || (workspace != "" && last.Workspace != workspace) {
			continue
		}
		i := n - 1
		for i > 0 && cat(t.Versions[i-1].Status) == catDone {
			i--
		}
		done := t.Versions[i].When
		if done.Before(since) || done.After(until) {
			continue
		}
		r := Times{ULID: t.ULID, Done: done, Lead: done.Sub(last.Created)}
		for _, v := range t.Versions[:i] {
			if cat(v.Status) == catDoing {
				r.Cycle, r.HasCycle = done.Sub(v.When), true
				break
			}
		}
		out = append(out, r)
	}
	return out
}

// Percentile is the value at the fraction p (nearest rank) of ds; 0 for none.
func Percentile(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := slices.Sorted(slices.Values(ds))
	i := int(math.Ceil(p*float64(len(s)))) - 1
	return s[max(i, 0)]
}
