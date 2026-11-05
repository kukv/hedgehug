// Package worklogs is the operations on the time spent on a task.
package worklogs

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// WorklogView is one entry.
type WorklogView struct {
	ULID     string `json:"ulid"`
	Task     string `json:"task" doc:"The ULID of the task"`
	Author   string `json:"author"`
	Date     string `json:"date"`
	Duration string `json:"duration"`
	Body     string `json:"body,omitempty" doc:"The note"`
}

// WorklogWrite is what a write to the worklog answers.
type WorklogWrite struct {
	Worklog WorklogView `json:"worklog"`
	Commit  *string     `json:"commit"`
}

func viewOf(task model.ULID, w *model.Worklog) WorklogView {
	return WorklogView{
		ULID: w.ULID.String(), Task: task.String(), Author: w.Author,
		Date: w.Date.String(), Duration: w.Duration.String(), Body: w.Body,
	}
}

// open resolves arg to a live task, in the store it is in.
func open(ss *usecase.Session, arg string) (*usecase.Session, *store.Store, *index.Entry, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return nil, nil, nil, err
	}
	s, ix, err := ts.OpenForWrite()
	if err != nil {
		return nil, nil, nil, err
	}
	e, err := ts.Resolve(ix, arg)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return nil, nil, nil, err
	}
	return ts, s, e, nil
}

// Add records time spent on the task arg names. An empty date is today.
func Add(ss *usecase.Session, arg, duration, date, note string) (WorklogWrite, error) {
	d, err := model.ParseDuration(duration)
	if err != nil || time.Duration(d) <= 0 {
		return WorklogWrite{}, usecase.NewErr(usecase.KindUsage, "worklog.bad_duration", fmt.Errorf("%q is not a time spent (like 1h30m)", duration), "value", duration)
	}
	ts, s, e, err := open(ss, arg)
	if err != nil {
		return WorklogWrite{}, err
	}
	on, err := worklogDate(ts, date)
	if err != nil {
		return WorklogWrite{}, err
	}
	w := &model.Worklog{ULID: model.NewULID(), Date: on, Duration: d, Body: strings.TrimSpace(note)}
	if w.Author, err = ts.AccountID(); err != nil {
		return WorklogWrite{}, err
	}
	hash, err := ts.UpdateCommit(s, store.Op{Name: "worklog-add", Task: e.ULID}, func(tx *store.Tx) error {
		return tx.PutWorklog(e.ULID, w)
	})
	if err != nil {
		return WorklogWrite{}, err
	}
	return WorklogWrite{Worklog: viewOf(e.ULID, w), Commit: usecase.CommitPtr(usecase.CommitHex(hash))}, nil
}

func worklogDate(ss *usecase.Session, date string) (model.Date, error) {
	if date == "" {
		date = ss.Now().In(ss.Location()).Format(time.DateOnly)
	}
	d, err := model.ParseDate(date)
	if err != nil {
		return model.Date{}, usecase.NewErr(usecase.KindUsage, "worklog.bad_date", fmt.Errorf("%q is not a date (YYYY-MM-DD)", date), "value", date)
	}
	return d, nil
}

// List lists the entries of the task arg names, oldest first, without the
// deleted ones.
func List(ss *usecase.Session, arg string) ([]WorklogView, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return nil, err
	}
	s, ix, err := ts.Open()
	if err != nil {
		return nil, err
	}
	e, err := ts.Resolve(ix, arg)
	if err != nil {
		return nil, err
	}
	logs, err := live(s, e.ULID)
	if err != nil {
		return nil, err
	}
	out := make([]WorklogView, len(logs))
	for i, w := range logs {
		out[i] = viewOf(e.ULID, w)
	}
	return out, nil
}

func live(s *store.Store, task model.ULID) ([]*model.Worklog, error) {
	var out []*model.Worklog
	for w, err := range s.Worklogs(task) {
		if err != nil {
			return nil, err
		}
		if w.DeletedAt == nil {
			out = append(out, w)
		}
	}
	slices.SortFunc(out, func(a, b *model.Worklog) int {
		if c := a.Date.Time().Compare(b.Date.Time()); c != 0 {
			return c
		}
		return strings.Compare(a.ULID.String(), b.ULID.String())
	})
	return out, nil
}

// Remove deletes the entry log of the task arg names. An entry can be deleted
// only by who wrote it.
func Remove(ss *usecase.Session, arg, log string) (WorklogWrite, error) {
	ts, s, e, err := open(ss, arg)
	if err != nil {
		return WorklogWrite{}, err
	}
	logs, err := live(s, e.ULID)
	if err != nil {
		return WorklogWrite{}, err
	}
	i := slices.IndexFunc(logs, func(w *model.Worklog) bool { return strings.EqualFold(w.ULID.String(), log) })
	if i < 0 {
		return WorklogWrite{}, usecase.NewErr(usecase.KindNotFound, "worklog.not_found", fmt.Errorf("the task has no entry %s", log), "id", log)
	}
	w := logs[i]
	c, _, err := ts.Config()
	if err != nil {
		return WorklogWrite{}, err
	}
	if !slices.ContainsFunc(c.Me(), func(a string) bool { return config.SameAccount(a, w.Author) }) {
		return WorklogWrite{}, usecase.NewErr(usecase.KindConflict, "worklog.not_author", errors.New("the entry of "+w.Author+" cannot be removed"), "author", w.Author)
	}
	at := ts.Now().UTC().Truncate(time.Second)
	w.DeletedAt = &at
	hash, err := ts.UpdateCommit(s, store.Op{Name: "worklog-rm", Task: e.ULID}, func(tx *store.Tx) error {
		return tx.PutWorklog(e.ULID, w)
	})
	if err != nil {
		return WorklogWrite{}, err
	}
	return WorklogWrite{Worklog: viewOf(e.ULID, w), Commit: usecase.CommitPtr(usecase.CommitHex(hash))}, nil
}
