package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/merge"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) doctorCmd() *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Find what is wrong with the files, the work tree and git of the store, and with --fix repair what a sync repairs",
		Args:  exactArgs(0),
		RunE:  func(*cobra.Command, []string) error { return a.doctor(fix) },
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "repair parent cycles, children of deleted tasks, tasks in another workspace than their parent and ids that clash, in one commit; then tell what is left")
	return cmd
}

func (a *app) doctor(fix bool) error {
	c, _, err := a.ss.Config()
	if err != nil {
		return err
	}
	e, err := c.Select(a.ss.StoreName)
	if err != nil {
		return err
	}
	var fixed []fixedJSON
	var openErr error
	if fix {
		var s *store.Store
		// A store that does not open is told of by the findings, which say why.
		if s, openErr = store.Open(e.Path); openErr == nil {
			if fixed, err = a.fixStore(c, e, s); err != nil {
				return err
			}
		}
	}
	fs, err := doctor.Run(e.Path)
	if err != nil {
		return err
	}
	if openErr != nil && len(fs) == 0 {
		return openErr
	}
	if !fix {
		return a.printFindings(fs)
	}
	if a.jsonOut {
		if fixed == nil {
			fixed = []fixedJSON{}
		}
		if fs == nil {
			fs = []doctor.Finding{}
		}
		if err := a.printJSON(struct {
			Repaired []fixedJSON      `json:"repaired"`
			Findings []doctor.Finding `json:"findings"`
		}{fixed, fs}); err != nil {
			return err
		}
		if len(fs) > 0 {
			return errFindings
		}
		return nil
	}
	if len(fixed) == 0 {
		if _, err := fmt.Fprintln(a.stdout, "Nothing to repair"); err != nil {
			return err
		}
	}
	for _, x := range fixed {
		if _, err := fmt.Fprintln(a.stdout, x.line); err != nil {
			return err
		}
	}
	return a.printFindings(fs)
}

// fixedJSON is something doctor --fix repaired. The text line is made when
// the repair is read, with the names the store has after it.
type fixedJSON struct {
	ULID   string          `json:"ulid"`
	ID     string          `json:"id,omitempty"`
	Title  string          `json:"title"`
	Kind   string          `json:"kind"`
	Value  json.RawMessage `json:"value"`
	Author string          `json:"author,omitempty"`
	line   string
}

// fixStore repairs the files of HEAD of the store e, opened as s, as a sync does, under
// the lock of a sync, and describes what it fixed.
func (a *app) fixStore(c *config.Config, e config.StoreEntry, s *store.Store) ([]fixedJSON, error) {
	opt, err := a.ss.SyncOptions(c, e)
	if err != nil {
		return nil, err
	}
	var repaired []merge.Repaired
	err = s.WithLocalLock("sync.lock", syncLockWait, func() error {
		var err error
		repaired, err = doctor.Fix(s, opt)
		return err
	})
	if err != nil || len(repaired) == 0 {
		return nil, err
	}
	a.afterWrite(s)
	ix, err := index.Open(s)
	if err != nil {
		ix = nil // the ULIDs name what was repaired
	}
	out := make([]fixedJSON, 0, len(repaired))
	for _, x := range repaired {
		f := fixedJSON{ULID: x.ULID.String(), Kind: x.Kind, Value: jsonValue(x.Value), Author: x.Author}
		name := x.ULID.String()
		if ix != nil {
			if t, ok := ix.Task(x.ULID); ok {
				f.ID, f.Title, name = t.ID, t.Title, ops.RefName(ix, x.ULID)
			} else if w, ok := ix.Workspace(x.ULID); ok {
				f.Title, name = w.Name, w.Name+" ("+x.ULID.String()+")"
			}
		}
		f.line = fmt.Sprintf("repaired: %s: %s", name, fixedContent(ix, x))
		out = append(out, f)
	}
	return out, nil
}

// fixedContent says what a repair did: the id a clash was given, or the
// kind with what it took from, as the inbox tells of it.
func fixedContent(ix *index.Index, x merge.Repaired) string {
	if x.Kind == "id_collision" {
		var v struct{ Old, New string }
		if json.Unmarshal([]byte(x.Value), &v) == nil && v.Old != "" && v.New != "" {
			return fmt.Sprintf("id_collision %s → %s", v.Old, v.New)
		}
	}
	detail, _ := json.Marshal(struct {
		Kind  string          `json:"kind"`
		Value json.RawMessage `json:"value"`
	}{x.Kind, jsonValue(x.Value)}) // a string and JSON always encode
	return repairedContent(ix, inbox.Item{Detail: detail})
}
