package planning

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// Where the unfinished tasks of a closed sprint go, besides a sprint's name.
const (
	ToNext    = "next"
	ToBacklog = "backlog"
)

// SprintView is a sprint with what its tasks add up to.
type SprintView struct {
	ULID      string               `json:"ulid"`
	Workspace usecase.WorkspaceRef `json:"workspace"`
	Name      string               `json:"name"`
	Start     string               `json:"start,omitempty"`
	End       string               `json:"end,omitempty"`
	State     string               `json:"state"`
	Goal      string               `json:"goal,omitempty"`
	Tasks     int                  `json:"tasks" doc:"The tasks in the sprint, not deleted"`
	Done      int                  `json:"done" doc:"Those of them in the done category"`
	Points    float64              `json:"points" doc:"The sum of the points of the tasks"`
}

// SprintWrite is what a write to a sprint answers, in the CLI's --json and in
// the server's response.
type SprintWrite struct {
	Sprint SprintView `json:"sprint"`
	Moved  int        `json:"moved,omitempty" doc:"Closing: the tasks moved out of the sprint"`
	To     string     `json:"to,omitempty" doc:"Closing: where they went: backlog, or the name of a sprint"`
	Commit *string    `json:"commit"`
}

// isOpen reports whether the status is neither in the done nor in the
// canceled category. A status the schema lacks counts as open.
func isOpen(sc *schema.Schema, status string) bool {
	st, ok := sc.Status(status)
	return !ok || (st.Category != schema.CategoryDone && st.Category != schema.CategoryCanceled)
}

func sprintViewOf(ix *index.Index, sp *model.Sprint) SprintView {
	v := SprintView{
		ULID: sp.ULID.String(), Workspace: workspaceRef(ix, sp.Workspace), Name: sp.Name,
		Start: dateText(sp.Start), End: dateText(sp.End), State: sp.State, Goal: sp.Goal,
	}
	sc := ix.Schema()
	for _, e := range ix.Tasks() {
		if e.Sprint == nil || *e.Sprint != sp.ULID || e.DeletedAt != nil {
			continue
		}
		v.Tasks++
		if st, ok := sc.Status(e.Status); ok && st.Category == schema.CategoryDone {
			v.Done++
		}
		v.Points += pointsOf(e)
	}
	return v
}

// pointsOf is the points of e; 0 when it has none.
func pointsOf(e *index.Entry) float64 {
	vals := e.Prop("points")
	if len(vals) != 1 {
		return 0
	}
	f, err := strconv.ParseFloat(vals[0], 64)
	if err != nil {
		return 0
	}
	return f
}

// written is the answer to a write to sp. It reads the index after the write;
// the commit is made, so a failure to read leaves the counts as they were.
func written(ix *index.Index, sp *model.Sprint, hash string) SprintWrite {
	_ = ix.Refresh()
	return SprintWrite{Sprint: sprintViewOf(ix, sp), Commit: usecase.CommitPtr(hash)}
}

func badDates(start, end model.Date) error {
	if !start.IsZero() && !end.IsZero() && end.Time().Before(start.Time()) {
		return usecase.NewErr(usecase.KindInvalid, "sprint.bad_dates", fmt.Errorf("the sprint ends (%s) before it starts (%s)", end, start))
	}
	return nil
}

// checkSprintName refuses a name another sprint of the workspace has.
func checkSprintName(ix *index.Index, ws, self model.ULID, name string) error {
	if sp, ok := ix.SprintNamed(ws, name); ok && sp.ULID != self {
		return usecase.NewErr(usecase.KindConflict, "sprint.name_taken", fmt.Errorf("the workspace has a sprint called %s", sp.Name), "name", name)
	}
	return nil
}

// SprintAddInput is what hedgehug sprint add takes.
type SprintAddInput struct {
	Name, Start, End, Goal, Workspace string
}

// SprintAdd adds a planned sprint.
func SprintAdd(ss *usecase.Session, in SprintAddInput) (SprintWrite, error) {
	if err := badName("sprint", in.Name); err != nil {
		return SprintWrite{}, err
	}
	start, err := parseDate("start", in.Start)
	if err != nil {
		return SprintWrite{}, err
	}
	end, err := parseDate("end", in.End)
	if err != nil {
		return SprintWrite{}, err
	}
	if err := badDates(start, end); err != nil {
		return SprintWrite{}, err
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return SprintWrite{}, err
	}
	ws, err := writeWorkspace(ss, ix, in.Workspace)
	if err != nil {
		return SprintWrite{}, err
	}
	if err := checkSprintName(ix, ws, model.ULID{}, in.Name); err != nil {
		return SprintWrite{}, err
	}
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: in.Name, Start: start, End: end, State: model.SprintPlanned, Goal: in.Goal}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "sprint-add", Workspace: ws, Detail: sp.Name}, func(tx *store.Tx) error {
		return tx.PutSprint(sp)
	})
	if err != nil {
		return SprintWrite{}, err
	}
	return written(ix, sp, usecase.CommitHex(hash)), nil
}

// SprintList lists the sprints of the workspace arg names, of the context's,
// or of the store when there is neither; state narrows them when not empty.
func SprintList(ss *usecase.Session, workspace, state string) ([]SprintView, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return nil, err
	}
	ws, err := pickWorkspace(ss, ix, workspace)
	if err != nil {
		return nil, err
	}
	out := []SprintView{}
	for _, sp := range ix.Sprints(ws) {
		if state == "" || sp.State == state {
			out = append(out, sprintViewOf(ix, sp))
		}
	}
	return out, nil
}

// SprintSet changes the name, the dates or the goal of a sprint.
func SprintSet(ss *usecase.Session, name, workspace string, assigns []usecase.Assign) (SprintWrite, error) {
	if len(assigns) == 0 {
		return SprintWrite{}, usecase.NewErr(usecase.KindUsage, "sprint.nothing_to_set", errors.New("nothing to change"))
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return SprintWrite{}, err
	}
	found, err := findSprint(ss, ix, name, workspace)
	if err != nil {
		return SprintWrite{}, err
	}
	var sp *model.Sprint
	hash, err := ss.UpdateCommit(s, store.Op{Name: "sprint-set", Workspace: found.Workspace, Detail: found.Name}, func(tx *store.Tx) error {
		var err error
		if sp, err = tx.Sprint(found.ULID); err != nil {
			return err
		}
		for _, a := range assigns {
			if err := applySprint(sp, a); err != nil {
				return err
			}
		}
		if err := badDates(sp.Start, sp.End); err != nil {
			return err
		}
		if err := checkSprintName(ix, sp.Workspace, sp.ULID, sp.Name); err != nil {
			return err
		}
		return tx.PutSprint(sp)
	})
	if err != nil {
		return SprintWrite{}, err
	}
	return written(ix, sp, usecase.CommitHex(hash)), nil
}

func applySprint(sp *model.Sprint, a usecase.Assign) error {
	if a.Op != "=" {
		return usecase.NewErr(usecase.KindInvalid, "assign.single", fmt.Errorf("%s holds one value; use %s=<value>", a.Key, a.Key), "key", a.Key)
	}
	var err error
	switch a.Key {
	case "name":
		if err = badName("sprint", a.Value); err == nil {
			sp.Name = a.Value
		}
	case "start":
		sp.Start, err = parseDate(a.Key, a.Value)
	case "end":
		sp.End, err = parseDate(a.Key, a.Value)
	case "goal":
		sp.Goal = a.Value
	case "state":
		err = usecase.NewErr(usecase.KindInvalid, "assign.readonly", errors.New("state cannot be set; use hedgehug sprint start or close"), "key", a.Key)
	default:
		err = usecase.NewErr(usecase.KindInvalid, "assign.unknown_key", fmt.Errorf("%s is not name, start, end or goal", a.Key), "key", a.Key)
	}
	return err
}

// SprintStart makes a planned sprint active.
func SprintStart(ss *usecase.Session, name, workspace string) (SprintWrite, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return SprintWrite{}, err
	}
	found, err := findSprint(ss, ix, name, workspace)
	if err != nil {
		return SprintWrite{}, err
	}
	var sp *model.Sprint
	hash, err := ss.UpdateCommit(s, store.Op{Name: "sprint-start", Workspace: found.Workspace, Detail: found.Name}, func(tx *store.Tx) error {
		var err error
		if sp, err = tx.Sprint(found.ULID); err != nil {
			return err
		}
		if sp.State != model.SprintPlanned {
			return usecase.NewErr(usecase.KindConflict, "sprint.not_planned", fmt.Errorf("sprint %s is %s, not planned", sp.Name, sp.State), "state", sp.State)
		}
		if sp.Start.IsZero() || sp.End.IsZero() {
			return usecase.NewErr(usecase.KindInvalid, "sprint.no_dates", fmt.Errorf("sprint %s needs a start and an end; set them with hedgehug sprint set", sp.Name))
		}
		sp.State = model.SprintActive
		return tx.PutSprint(sp)
	})
	if err != nil {
		return SprintWrite{}, err
	}
	return written(ix, sp, usecase.CommitHex(hash)), nil
}

// openTasks lists the live tasks of the sprint that are not finished.
func openTasks(ix *index.Index, sprint model.ULID) []*index.Entry {
	sc := ix.Schema()
	var out []*index.Entry
	for _, e := range ix.Tasks() {
		if e.Sprint != nil && *e.Sprint == sprint && e.DeletedAt == nil && isOpen(sc, e.Status) {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b *index.Entry) int { return strings.Compare(a.ULID.String(), b.ULID.String()) })
	return out
}

// SprintOpenTasks counts the tasks that closing the sprint would move.
func SprintOpenTasks(ss *usecase.Session, name, workspace string) (int, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return 0, err
	}
	sp, err := findSprint(ss, ix, name, workspace)
	if err != nil {
		return 0, err
	}
	return len(openTasks(ix, sp.ULID)), nil
}

// closeTarget is the sprint to says, nil for the backlog. A sprint left with
// no open tasks needs none, so to may be empty then.
func closeTarget(ix *index.Index, sp *model.Sprint, to string, open int) (*model.Sprint, error) {
	switch to {
	case "":
		if open > 0 {
			return nil, usecase.NewErr(usecase.KindUsage, "sprint.close_to",
				fmt.Errorf("sprint %s has %d unfinished tasks; say where they go with --to next, backlog or the name of a sprint", sp.Name, open), "tasks", open)
		}
		return nil, nil
	case ToBacklog:
		return nil, nil
	case ToNext:
		next, ok := ix.NextSprint(sp.Workspace)
		if !ok {
			if open == 0 {
				return nil, nil
			}
			return nil, usecase.NewErr(usecase.KindConflict, "sprint.no_next", errors.New("the workspace has no planned sprint; add one, or use --to backlog"))
		}
		return next, nil
	}
	next, ok := ix.SprintNamed(sp.Workspace, to)
	if !ok {
		return nil, usecase.NewErr(usecase.KindNotFound, "sprint.not_found", fmt.Errorf("sprint %q is not in the workspace of %s", to, sp.Name), "name", to)
	}
	if next.ULID == sp.ULID || next.State == model.SprintClosed {
		return nil, usecase.NewErr(usecase.KindInvalid, "sprint.bad_target", fmt.Errorf("the tasks cannot go to sprint %s, which is %s", next.Name, closedWord(next, sp)))
	}
	return next, nil
}

func closedWord(next, sp *model.Sprint) string {
	if next.ULID == sp.ULID {
		return "the one being closed"
	}
	return "closed"
}

// SprintClose closes an active sprint and moves its unfinished tasks as to
// says: ToNext, ToBacklog or the name of a sprint of the workspace; all in
// one commit. to may be empty when no task is left to move.
func SprintClose(ss *usecase.Session, name, workspace, to string) (SprintWrite, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return SprintWrite{}, err
	}
	found, err := findSprint(ss, ix, name, workspace)
	if err != nil {
		return SprintWrite{}, err
	}
	open := openTasks(ix, found.ULID)
	target, err := closeTarget(ix, found, to, len(open))
	if err != nil {
		return SprintWrite{}, err
	}
	var sp *model.Sprint
	hash, err := ss.UpdateCommit(s, store.Op{Name: "sprint-close", Workspace: found.Workspace, Detail: found.Name}, func(tx *store.Tx) error {
		var err error
		if sp, err = tx.Sprint(found.ULID); err != nil {
			return err
		}
		if sp.State != model.SprintActive {
			return usecase.NewErr(usecase.KindConflict, "sprint.not_active", fmt.Errorf("sprint %s is %s, not active", sp.Name, sp.State), "state", sp.State)
		}
		sp.State = model.SprintClosed
		if err := tx.PutSprint(sp); err != nil {
			return err
		}
		for _, e := range open {
			t, err := tx.Task(e.ULID)
			if err != nil {
				return err
			}
			t.Sprint = nil
			if target != nil {
				t.Sprint = &target.ULID
			}
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return SprintWrite{}, err
	}
	w := written(ix, sp, usecase.CommitHex(hash))
	w.Moved = len(open)
	if w.Moved > 0 {
		w.To = ToBacklog
		if target != nil {
			w.To = target.Name
		}
	}
	return w, nil
}

// SprintRemove takes the sprint off its tasks and deletes it, in one commit.
// A deleted task is taken off too, so that nothing points at the sprint.
func SprintRemove(ss *usecase.Session, name, workspace string) (SprintWrite, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return SprintWrite{}, err
	}
	sp, err := findSprint(ss, ix, name, workspace)
	if err != nil {
		return SprintWrite{}, err
	}
	view := sprintViewOf(ix, sp)
	hash, err := ss.UpdateCommit(s, store.Op{Name: "sprint-rm", Workspace: sp.Workspace, Detail: sp.Name}, func(tx *store.Tx) error {
		for _, e := range ix.Tasks() {
			if e.Sprint == nil || *e.Sprint != sp.ULID {
				continue
			}
			t, err := tx.Task(e.ULID)
			if err != nil {
				return err
			}
			t.Sprint = nil
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return tx.RemoveFile(model.SprintPath(sp.ULID))
	})
	if err != nil {
		return SprintWrite{}, err
	}
	return SprintWrite{Sprint: view, Commit: usecase.CommitPtr(usecase.CommitHex(hash))}, nil
}
