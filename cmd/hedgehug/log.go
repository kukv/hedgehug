package main

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) logCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "log [id]",
		Short: "Show the commits of the store, or of one task",
		Args:  asUsage(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			if limit < 0 {
				return usageErrorf("-n must not be negative")
			}
			return a.log(args, limit)
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "show at most this many commits (0 for all)")
	return cmd
}

// logJSON is a commit as --json prints it.
type logJSON struct {
	Hash   string    `json:"hash"`
	Date   time.Time `json:"date"`
	Author struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
	Op      *string `json:"op"`
	Task    *string `json:"task"`
	Key     *string `json:"key"` // the current key of the task
	Subject string  `json:"subject"`
}

func toLogJSON(ix *index.Index, e gitrepo.LogEntry) logJSON {
	j := logJSON{Hash: e.Hash.String(), Date: e.When, Subject: e.Subject}
	j.Author.Name, j.Author.Email = e.Author.Name, e.Author.Email
	if v, ok := e.Trailer("Hedgehug-Op"); ok {
		j.Op = &v
	}
	if v, ok := e.Trailer("Hedgehug-Task"); ok {
		j.Task = &v
		if id, err := model.ParseULID(v); err == nil {
			if t, ok := ix.Task(id); ok && t.Key != "" {
				j.Key = &t.Key
			}
		}
	}
	return j
}

func logJSONs(ix *index.Index, entries []gitrepo.LogEntry) []logJSON {
	out := make([]logJSON, len(entries))
	for i, e := range entries {
		out[i] = toLogJSON(ix, e)
	}
	return out
}

// subjectNow gives the subject of a Hedgehug commit with the task's current
// key and ULID in place of the label it was written with: the subject is
// "hedgehug: <op> <label> <title>". Other subjects, and tasks the index does
// not have, keep the subject as it is.
func subjectNow(ix *index.Index, le gitrepo.LogEntry) string {
	op, ok1 := le.Trailer("Hedgehug-Op")
	tid, ok2 := le.Trailer("Hedgehug-Task")
	head := "hedgehug: " + op + " "
	rest, ok3 := strings.CutPrefix(le.Subject, head)
	if !ok1 || !ok2 || !ok3 {
		return le.Subject
	}
	id, err := model.ParseULID(tid)
	if err != nil {
		return le.Subject
	}
	e, ok := ix.Task(id)
	if !ok {
		return le.Subject
	}
	_, title, _ := strings.Cut(rest, " ")
	return strings.TrimRight(head+taskLabel(e)+" "+title, " ")
}

func logText(ix *index.Index, entries []gitrepo.LogEntry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s  %s  %s  %s\n", e.Hash.String()[:7], e.When.In(time.Local).Format("2006-01-02 15:04"), e.Author.Name, subjectNow(ix, e))
	}
	return b.String()
}

func (a *app) log(args []string, limit int) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	var entries []gitrepo.LogEntry
	if len(args) == 0 {
		entries, err = s.Repo().Log(nil, limit)
	} else {
		e, rerr := a.resolve(ix, args[0])
		if rerr != nil {
			return rerr
		}
		entries, err = a.taskLog(s, e.ULID, limit)
	}
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(logJSONs(ix, entries))
	}
	_, err = fmt.Fprint(a.stdout, logText(ix, entries))
	return err
}

// taskLog returns the commits that touched the task, its comments or its
// work logs.
func (a *app) taskLog(s *store.Store, id model.ULID, limit int) ([]gitrepo.LogEntry, error) {
	return s.Repo().Log([]string{
		model.TaskPath(id),
		path.Dir(model.CommentPath(id, model.ULID{})) + "/",
		path.Dir(model.WorklogPath(id, model.ULID{})) + "/",
	}, limit)
}
