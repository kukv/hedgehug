// Package reports puts the reports of a store together from its history.
package reports

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/report"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/reportcache"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

const (
	defaultDays = 30 // how far cfd and cycle look back
	defaultLast = 10 // how many sprints velocity lists
)

// The values of --by.
const (
	ByPoints   = "points"
	ByCount    = "count"
	ByCategory = "category"
	ByStatus   = "status"
)

// opened is a store with what a report reads of it.
type opened struct {
	ss *usecase.Session
	s  *store.Store
	ix *index.Index
	h  *history.History
}

func open(ss *usecase.Session) (*opened, error) {
	s, ix, err := ss.Open()
	if err != nil {
		return nil, err
	}
	_, e, err := ss.Entry()
	if err != nil {
		return nil, err
	}
	h, err := ss.OpenHistory(s, e.Name)
	if err != nil {
		return nil, err
	}
	return &opened{ss, s, ix, h}, nil
}

func categoryOf(sc *schema.Schema) report.Category {
	return func(status string) string {
		st, ok := sc.Status(status)
		if !ok {
			return report.Unknown
		}
		return string(st.Category)
	}
}

func (o *opened) tasks() ([]report.Task, error) {
	ts, err := reportcache.Read(o.s, o.h)
	if err != nil {
		return nil, usecase.Classify(err)
	}
	return ts, nil
}

// window reads from the history of the file of sp when it started and ended.
// A sprint whose state was written by hand has no such commit: it counts from
// the start of its first day to the end of its last.
func (o *opened) window(sp *model.Sprint) (report.Window, error) {
	w := report.Window{Sprint: sp.ULID.String()}
	p := model.SprintPath(sp.ULID)
	commits := o.h.Path(p)
	prev := ""
	for _, c := range slices.Backward(commits) {
		b, err := o.s.Repo().ReadAt(c.Hash, p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return report.Window{}, err
		}
		v, err := model.DecodeSprint(b)
		if err != nil {
			continue // a version that does not decode says nothing of the state
		}
		if v.State == model.SprintActive && w.Start.IsZero() {
			w.Start = c.When
		}
		if v.State == model.SprintClosed && prev != model.SprintClosed {
			w.End, w.CloseCommit = c.When, c.Hash.String()
		}
		if v.State != model.SprintClosed {
			w.End, w.CloseCommit = time.Time{}, ""
		}
		prev = v.State
	}
	loc := o.ss.Location()
	if w.Start.IsZero() {
		if sp.Start.IsZero() {
			return report.Window{}, usecase.NewErr(usecase.KindInvalid, "report.no_start", fmt.Errorf("sprint %s has no start", sp.Name))
		}
		w.Start = midnight(sp.Start, loc)
		w.End, w.CloseCommit = time.Time{}, ""
	}
	if sp.State == model.SprintClosed && w.End.IsZero() {
		if sp.End.IsZero() {
			return report.Window{}, usecase.NewErr(usecase.KindInvalid, "report.no_end", fmt.Errorf("sprint %s has no end", sp.Name))
		}
		w.End = midnight(sp.End, loc).AddDate(0, 0, 1)
	}
	return w, nil
}

func midnight(d model.Date, loc *time.Location) time.Time {
	y, m, day := d.Time().Date()
	return time.Date(y, m, day, 0, 0, 0, 0, loc)
}

// SprintRef names the sprint of a report.
type SprintRef struct {
	ULID  string `json:"ulid"`
	Name  string `json:"name"`
	State string `json:"state"`
	Start string `json:"start,omitempty" doc:"The first day"`
	End   string `json:"end,omitempty" doc:"The last day"`
}

func refOf(sp *model.Sprint) SprintRef {
	r := SprintRef{ULID: sp.ULID.String(), Name: sp.Name, State: sp.State}
	if !sp.Start.IsZero() {
		r.Start = sp.Start.String()
	}
	if !sp.End.IsZero() {
		r.End = sp.End.String()
	}
	return r
}

// BurndownView is hedgehug report burndown.
type BurndownView struct {
	Sprint SprintRef            `json:"sprint"`
	By     string               `json:"by" doc:"What the numbers are: points or count"`
	Note   string               `json:"note,omitempty"`
	Days   []report.BurndownDay `json:"days"`
}

// sprintFor finds the sprint of a burndown: the one name says, else the only
// active one of the workspace.
func (o *opened) sprintFor(name, workspace string) (*model.Sprint, error) {
	if name != "" {
		return usecase.FindSprint(o.ss, o.ix, name, workspace)
	}
	ws, err := usecase.PickWorkspace(o.ss, o.ix, workspace)
	if err != nil {
		return nil, err
	}
	active := slices.DeleteFunc(o.ix.Sprints(ws), func(sp *model.Sprint) bool { return sp.State != model.SprintActive })
	if len(active) != 1 {
		return nil, usecase.NewErr(usecase.KindUsage, "report.sprint_needed", fmt.Errorf("%d sprints are active; name one", len(active)), "active", len(active))
	}
	return active[0], nil
}

// Burndown is the sprint name day by day; with no name, the active one.
func Burndown(ss *usecase.Session, name, workspace, by string) (BurndownView, error) {
	if by == "" {
		by = ByPoints
	}
	if by != ByPoints && by != ByCount {
		return BurndownView{}, usecase.NewErr(usecase.KindUsage, "report.bad_by", fmt.Errorf("--by %q: want points or count", by), "value", by)
	}
	o, err := open(ss)
	if err != nil {
		return BurndownView{}, err
	}
	sp, err := o.sprintFor(name, workspace)
	if err != nil {
		return BurndownView{}, err
	}
	if sp.State == model.SprintPlanned {
		return BurndownView{}, usecase.NewErr(usecase.KindConflict, "report.not_started", fmt.Errorf("sprint %s has not started", sp.Name))
	}
	w, err := o.window(sp)
	if err != nil {
		return BurndownView{}, err
	}
	tasks, err := o.tasks()
	if err != nil {
		return BurndownView{}, err
	}
	v := BurndownView{Sprint: refOf(sp), By: by}
	if by == ByPoints && !report.HasPoints(tasks, w.Sprint) {
		v.By, v.Note = ByCount, "no task of the sprint has points, so tasks are counted"
	}
	loc := ss.Location()
	first, last := w.Start, ss.Now()
	if !sp.Start.IsZero() {
		if m := midnight(sp.Start, loc); m.Before(first) {
			first = m
		}
	}
	if !w.End.IsZero() {
		last = w.End
	}
	// the end of a sprint closed by hand is the midnight after its last day
	days := report.Days(first, last.Add(-time.Nanosecond), last, loc)
	v.Days = report.Burndown(tasks, w, days, categoryOf(o.ix.Schema()), v.By == ByPoints)
	return v, nil
}

// VelocityRow is one closed sprint.
type VelocityRow struct {
	Sprint SprintRef `json:"sprint"`
	report.Completed
}

// Velocity lists the closed sprints that ended last, oldest first; last is
// how many, 0 for the default.
func Velocity(ss *usecase.Session, workspace string, last int) ([]VelocityRow, error) {
	if last <= 0 {
		last = defaultLast
	}
	o, err := open(ss)
	if err != nil {
		return nil, err
	}
	ws, err := usecase.PickWorkspace(ss, o.ix, workspace)
	if err != nil {
		return nil, err
	}
	type closed struct {
		sp *model.Sprint
		w  report.Window
	}
	var cs []closed
	for _, sp := range o.ix.Sprints(ws) {
		if sp.State != model.SprintClosed {
			continue
		}
		w, err := o.window(sp)
		if err != nil {
			return nil, err
		}
		cs = append(cs, closed{sp, w})
	}
	slices.SortStableFunc(cs, func(a, b closed) int { return a.w.End.Compare(b.w.End) })
	if len(cs) > last {
		cs = cs[len(cs)-last:]
	}
	tasks, err := o.tasks()
	if err != nil {
		return nil, err
	}
	cat := categoryOf(o.ix.Schema())
	out := make([]VelocityRow, len(cs))
	for i, c := range cs {
		out[i] = VelocityRow{Sprint: refOf(c.sp), Completed: report.Velocity(tasks, c.w, cat)}
	}
	return out, nil
}

// span reads since and until: the default is the last defaultDays days, and
// the end of until is the instant read, or now when that is sooner.
func span(ss *usecase.Session, since, until string) (from, to time.Time, err error) {
	loc := ss.Location()
	now := ss.Now().In(loc)
	to = now
	if until != "" {
		d, err := model.ParseDate(until)
		if err != nil {
			return from, to, usecase.NewErr(usecase.KindUsage, "report.bad_date", fmt.Errorf("--until %q is not a date (YYYY-MM-DD)", until), "value", until)
		}
		if end := midnight(d, loc).AddDate(0, 0, 1).Add(-time.Nanosecond); end.Before(now) {
			to = end
		}
	}
	from = to.AddDate(0, 0, -defaultDays)
	if since != "" {
		d, err := model.ParseDate(since)
		if err != nil {
			return from, to, usecase.NewErr(usecase.KindUsage, "report.bad_date", fmt.Errorf("--since %q is not a date (YYYY-MM-DD)", since), "value", since)
		}
		from = midnight(d, loc)
	}
	if from.After(to) {
		return from, to, usecase.NewErr(usecase.KindUsage, "report.bad_span", errors.New("--since is after --until"))
	}
	return from, to, nil
}

func wsText(ws model.ULID) string {
	if ws.IsZero() {
		return ""
	}
	return ws.String()
}

// CFDView is hedgehug report cfd.
type CFDView struct {
	By      string          `json:"by"`
	Columns []string        `json:"columns" doc:"The keys of counts in the order of the schema"`
	Days    []report.CFDDay `json:"days"`
}

// CFD counts the tasks day by day, by category or by status.
func CFD(ss *usecase.Session, workspace, since, until, by string) (CFDView, error) {
	if by == "" {
		by = ByCategory
	}
	if by != ByCategory && by != ByStatus {
		return CFDView{}, usecase.NewErr(usecase.KindUsage, "report.bad_by", fmt.Errorf("--by %q: want category or status", by), "value", by)
	}
	from, to, err := span(ss, since, until)
	if err != nil {
		return CFDView{}, err
	}
	o, err := open(ss)
	if err != nil {
		return CFDView{}, err
	}
	ws, err := usecase.PickWorkspace(ss, o.ix, workspace)
	if err != nil {
		return CFDView{}, err
	}
	tasks, err := o.tasks()
	if err != nil {
		return CFDView{}, err
	}
	sc := o.ix.Schema()
	key := categoryOf(sc)
	var cols []string
	if by == ByCategory {
		for _, c := range schema.Categories {
			cols = append(cols, string(c))
		}
	} else {
		for _, st := range sc.Statuses {
			cols = append(cols, st.Name)
		}
		key = func(status string) string {
			if _, ok := sc.Status(status); ok {
				return status
			}
			return report.Unknown
		}
	}
	v := CFDView{By: by, Days: report.CFD(tasks, report.Days(from, to, to, ss.Location()), wsText(ws), key)}
	v.Columns = cols
	for _, d := range v.Days {
		if d.Counts[report.Unknown] > 0 {
			v.Columns = append(v.Columns, report.Unknown)
			break
		}
	}
	return v, nil
}

// TimeStats sums up durations.
type TimeStats struct {
	Count  int    `json:"count"`
	Median string `json:"median" doc:"Such as 1h30m"`
	P85    string `json:"p85"`
}

func statsOf(ds []time.Duration) TimeStats {
	return TimeStats{
		Count:  len(ds),
		Median: model.Duration(report.Percentile(ds, 0.5)).String(),
		P85:    model.Duration(report.Percentile(ds, 0.85)).String(),
	}
}

// CycleView is hedgehug report cycle.
type CycleView struct {
	Since    string    `json:"since"`
	Until    string    `json:"until"`
	Cycle    TimeStats `json:"cycle" doc:"From first entering doing to the last entering done"`
	Lead     TimeStats `json:"lead" doc:"From the creation to the last entering done"`
	Warnings []string  `json:"warnings,omitempty"`
}

// withOrigin puts before the versions of a task that came from another store
// the versions it had there, when that store is mounted.
func (o *opened) withOrigin(t report.Task) (report.Task, string, error) {
	if len(t.Versions) == 0 || !t.Versions[0].MovedIn {
		return t, "", nil
	}
	ulid, err := model.ParseULID(t.ULID)
	if err != nil {
		return t, "", nil //nolint:nilerr // the ULID comes from a path that parsed
	}
	set, err := o.ss.Mount()
	if err != nil {
		return t, "", err
	}
	for _, m := range set.Stores {
		if m.Store == o.s || m.Store.Dir() == o.s.Dir() {
			continue
		}
		ix, err := m.Index()
		if err != nil {
			continue // a store that does not open is told of by the mount
		}
		if _, ok := ix.Forward(ulid); !ok {
			continue
		}
		h, err := o.ss.OpenHistory(m.Store, m.Name)
		if err != nil {
			return t, "", err
		}
		from, err := reportcache.Versions(m.Store, h, ulid)
		if err != nil {
			return t, "", err
		}
		lived := slices.DeleteFunc(from.Versions, func(v report.Version) bool { return v.Gone })
		t.Versions = append(lived, t.Versions...)
		return t, "", nil
	}
	return t, fmt.Sprintf("task %s came from a store that is not mounted; its times start at the move", ulid), nil
}

// Cycle sums up the cycle and the lead time of the tasks finished in the span.
func Cycle(ss *usecase.Session, workspace, since, until string) (CycleView, error) {
	from, to, err := span(ss, since, until)
	if err != nil {
		return CycleView{}, err
	}
	o, err := open(ss)
	if err != nil {
		return CycleView{}, err
	}
	ws, err := usecase.PickWorkspace(ss, o.ix, workspace)
	if err != nil {
		return CycleView{}, err
	}
	tasks, err := o.tasks()
	if err != nil {
		return CycleView{}, err
	}
	v := CycleView{Since: from.Format(time.DateOnly), Until: to.Format(time.DateOnly)}
	for i, t := range tasks {
		merged, warning, err := o.withOrigin(t)
		if err != nil {
			return CycleView{}, usecase.Classify(err)
		}
		tasks[i] = merged
		if warning != "" {
			v.Warnings = append(v.Warnings, warning)
		}
	}
	var cycle, lead []time.Duration
	for _, r := range report.Finished(tasks, wsText(ws), from, to, categoryOf(o.ix.Schema())) {
		lead = append(lead, r.Lead)
		if r.HasCycle {
			cycle = append(cycle, r.Cycle)
		}
	}
	v.Cycle, v.Lead = statsOf(cycle), statsOf(lead)
	return v, nil
}

// EstimateRow is the estimate of a task and the time logged on it.
type EstimateRow struct {
	ULID     string `json:"ulid"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Estimate string `json:"estimate,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

// EstimateView is hedgehug report estimate.
type EstimateView struct {
	Tasks    []EstimateRow `json:"tasks"`
	Estimate string        `json:"estimate" doc:"The sum of the estimates"`
	Actual   string        `json:"actual" doc:"The sum of the time logged"`
}

// Estimate compares the estimates of the live tasks with the time logged on
// them, those with neither left out.
func Estimate(ss *usecase.Session, workspace, sprint string) (EstimateView, error) {
	o, err := open(ss)
	if err != nil {
		return EstimateView{}, err
	}
	ws, err := usecase.PickWorkspace(ss, o.ix, workspace)
	if err != nil {
		return EstimateView{}, err
	}
	var only *model.ULID
	if sprint != "" {
		sp, err := usecase.FindSprint(ss, o.ix, sprint, workspace)
		if err != nil {
			return EstimateView{}, err
		}
		only = &sp.ULID
	}
	entries := o.ix.Tasks()
	slices.SortFunc(entries, func(a, b *index.Entry) int { return strings.Compare(a.ULID.String(), b.ULID.String()) })
	v := EstimateView{Tasks: []EstimateRow{}}
	var totalEstimate, totalActual model.Duration
	for _, e := range entries {
		if e.DeletedAt != nil || (!ws.IsZero() && e.Workspace != ws) || (only != nil && (e.Sprint == nil || *e.Sprint != *only)) {
			continue
		}
		r := EstimateRow{ULID: e.ULID.String(), ID: o.ix.DisplayID(e), Title: e.Title}
		if vals := e.Prop("estimate"); len(vals) == 1 {
			if d, err := model.ParseDuration(vals[0]); err == nil {
				r.Estimate = d.String()
				totalEstimate += d
			}
		}
		var actual model.Duration
		logged := false
		for w, err := range o.s.Worklogs(e.ULID) {
			if err != nil {
				return EstimateView{}, usecase.Classify(err)
			}
			if w.DeletedAt == nil {
				actual += w.Duration
				logged = true
			}
		}
		if logged {
			r.Actual = actual.String()
			totalActual += actual
		}
		if r.Estimate != "" || r.Actual != "" {
			v.Tasks = append(v.Tasks, r)
		}
	}
	v.Estimate, v.Actual = totalEstimate.String(), totalActual.String()
	return v, nil
}

// ParseLast reads the value of --last.
func ParseLast(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, usecase.NewErr(usecase.KindUsage, "report.bad_last", fmt.Errorf("--last %q: want a positive number", s), "value", s)
	}
	return n, nil
}
