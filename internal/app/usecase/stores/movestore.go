package stores

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

// MoveStoreInput is hedgehug mv <id> --store <dest>.
type MoveStoreInput struct {
	Arg       string // the task, looked up in every store
	To        string // the destination store
	Workspace string // in To; "" chooses as spec step 2 says
	Parent    string // in To; "" for the workspace's root
	Scoped    string // "carry", "drop" or "" (D7, D9)
	Yes       bool
	// Confirmed is the plan the user was shown and agreed to; MoveStore fails
	// with move.changed when the tasks are not what it lists any more. nil
	// when nothing was shown (--yes given up front).
	Confirmed *MovePlan
	// ConfirmedView is the plan the GUI showed and the user agreed to; it is
	// compared as Confirmed is, by the view of the plan made again under
	// the locks. nil when nothing was shown.
	ConfirmedView *MovePlanView
	// planOnly is set by PrepareMovePlan: a scoped choice that is not given
	// is no error, for the plan that asks for it.
	planOnly bool
}

// MovePlan is what a move will do, for the confirmation (spec step 5).
type MovePlan struct {
	From, To string
	Tasks    []*index.Entry // the root first
	Unlinked []store.Unlink
	// Labels name the tasks, sprints and releases of Unlinked and the
	// tasks of Tasks for a reader: the id as the source shows it, the name
	// of a sprint or a release, else the ULID.
	Labels map[model.ULID]string
	Hidden []string    // accounts that can read From and not To; nil when either list is unknown
	Scoped *ScopedPlan // nil when no scoped entry applies
	// Workspace and Parent are where the root goes in the destination
	// (step 2), for GUIView; Parent is nil unless one was given. On a rerun
	// after step 7, they are where the root is now.
	Workspace MoveWorkspaceView
	Parent    *MoveTaskView
	// ScopedChoice is MoveStoreInput.Scoped, for GUIView.
	ScopedChoice string
}

// Label is how the plan names ulid.
func (p MovePlan) Label(ulid model.ULID) string {
	if s, ok := p.Labels[ulid]; ok {
		return s
	}
	return ulid.String()
}

// UnlinkLines are the references the move cuts, one line each, as D8 words
// them: "unlink: API-12 blocked_by API-3".
func (p MovePlan) UnlinkLines() []string {
	out := make([]string, len(p.Unlinked))
	for i, u := range p.Unlinked {
		out[i] = fmt.Sprintf("unlink: %s %s %s", p.Label(u.Task), u.Field, p.Label(u.Target))
	}
	return out
}

// ScopedPlan is the scoped schema and the where rules of automations.yaml that
// the tasks moved bring with them.
type ScopedPlan struct {
	Projects []*index.Entry // the projects of the move that have a scoped entry or an automation in the source
	Dropped  []string       // D10, filled when carried: "status review", "template bug", "rule pr-opened"...
}

// Lines are the parts of the scoped schema and of the automations the move
// leaves out when it goes to the store to (D10), one line each.
func (sp *ScopedPlan) Lines(to string) []string {
	if sp == nil {
		return nil
	}
	out := make([]string, 0, len(sp.Dropped))
	for _, d := range sp.Dropped {
		what, name, _ := strings.Cut(d, " ")
		switch what {
		case "transitions":
			out = append(out, fmt.Sprintf("scoped: dropped transitions from %q (no target in %s)", name, to))
		case "required":
			out = append(out, fmt.Sprintf("scoped: dropped the required fields of %q (none of them in %s)", name, to))
		case "rule":
			out = append(out, fmt.Sprintf("scoped: dropped rule %q (does not fit %s)", name, to))
		case "disable":
			out = append(out, fmt.Sprintf("scoped: dropped the disable of rule %q (no such rule in %s)", name, to))
		default:
			out = append(out, fmt.Sprintf("scoped: dropped %s %q (not in %s)", what, name, to))
		}
	}
	return out
}

// MoveView is a MovePlan as a person reads it, and all that leaves usecase with
// the confirmation (Args "plan"): who moves where, the references cut, who
// loses sight of the tasks and what the scoped schema loses. It holds no
// content of the tasks (not their text, comments or worklogs).
type MoveView struct {
	From     string         `json:"from"`
	To       string         `json:"to"`
	Tasks    []MoveTaskView `json:"tasks"`
	Unlinked []string       `json:"unlinked,omitempty"` // "unlink: API-12 blocked_by API-3"
	Hidden   []string       `json:"hidden,omitempty"`   // accounts that can read From and not To
	Scoped   []string       `json:"scoped,omitempty"`   // "scoped: dropped status \"review\" (not in org-b)"
}

// MoveTaskView names a task of a move.
type MoveTaskView struct {
	ULID  model.ULID `json:"ulid"`
	ID    string     `json:"id"`
	Title string     `json:"title"`
}

// View is the plan as the confirmation shows it.
func (p MovePlan) View() MoveView {
	v := MoveView{From: p.From, To: p.To, Tasks: make([]MoveTaskView, len(p.Tasks)), Hidden: p.Hidden, Scoped: p.Scoped.Lines(p.To)}
	for i, e := range p.Tasks {
		v.Tasks[i] = MoveTaskView{ULID: e.ULID, ID: p.Label(e.ULID), Title: e.Title}
	}
	if len(p.Unlinked) > 0 {
		v.Unlinked = p.UnlinkLines()
	}
	return v
}

// MoveConfirmError is the confirm.required of a move. Its Args hold the
// MoveView only; the plan itself is for the caller that asks the user in
// person, and gives it back as MoveStoreInput.Confirmed.
type MoveConfirmError struct {
	Confirm *usecase.Error
	Plan    MovePlan
}

func (e *MoveConfirmError) Error() string { return e.Confirm.Error() }

// Unwrap gives the *Error, so that AsError finds its Kind, Code and Args.
func (e *MoveConfirmError) Unwrap() error { return e.Confirm }

// MovedTask is one task of a move with its id before and after.
type MovedTask struct {
	ULID  model.ULID `json:"ulid"`
	OldID string     `json:"old_id"`
	NewID string     `json:"new_id"`
}

// MoveStoreResult is what a move did.
type MoveStoreResult struct {
	Plan    MovePlan
	Moved   []MovedTask
	Carried bool
	Resumed bool // step 7 was done before; only step 8 ran (D12)
}

// PrepareMoveStore does steps 1 to 4 and 6's checks, and returns the plan to
// confirm as the confirmation shows it (the entry for the GUI). A task the destination schema refuses is KindInvalid, move.invalid,
// with one line per task and violation. A scoped entry that applies and no
// in.Scoped is choice.required.
func PrepareMoveStore(ss *usecase.Session, in MoveStoreInput) (MoveView, error) {
	j, err := openMove(ss, in)
	if err != nil {
		return MoveView{}, err
	}
	w, err := j.load()
	if err != nil {
		return MoveView{}, err
	}
	return w.plan.View(), nil
}

// PrepareMovePlan is PrepareMoveStore for the GUI: the plan as GUIView shows
// it. A scoped entry that applies and no in.Scoped is no error: the plan says
// scoped.required, and is made as if the entry were not carried.
func PrepareMovePlan(ss *usecase.Session, in MoveStoreInput) (MovePlanView, error) {
	in.planOnly = true
	j, err := openMove(ss, in)
	if err != nil {
		return MovePlanView{}, err
	}
	w, err := j.load()
	if err != nil {
		return MovePlanView{}, err
	}
	return w.plan.GUIView(), nil
}

// MoveStore does the move. Without in.Yes it is confirm.required with the
// MoveView in Args "plan" (the error is a *MoveConfirmError, which has the
// plan itself); a needed --scoped is choice.required.
func MoveStore(ss *usecase.Session, in MoveStoreInput) (MoveStoreResult, error) {
	j, err := openMove(ss, in)
	if err != nil {
		return MoveStoreResult{}, err
	}
	w, err := j.load()
	if err != nil {
		return MoveStoreResult{}, err
	}
	if !in.Yes {
		ce := usecase.ConfirmRequired(fmt.Sprintf("move %s to %s", plural.Count(len(w.tasks), "task"), j.toName))
		ce.Args["plan"] = w.plan.View()
		return MoveStoreResult{}, &MoveConfirmError{Confirm: ce, Plan: w.plan}
	}

	// Both stores are locked, smallest ULID first, while the move is worked
	// out again from what they hold now and written.
	l, err := store.LockInOrder(j.src, j.dst)
	if err != nil {
		return MoveStoreResult{}, usecase.Classify(err)
	}
	defer l.Release()
	if err := errors.Join(j.srcIx.Refresh(), j.dstIx.Refresh()); err != nil {
		return MoveStoreResult{}, err
	}
	if w, err = j.load(); err != nil {
		return MoveStoreResult{}, err
	}
	if (in.Confirmed != nil && !samePlan(*in.Confirmed, w.plan)) ||
		(in.ConfirmedView != nil && !samePlanView(*in.ConfirmedView, w.plan.GUIView())) {
		return MoveStoreResult{}, usecase.NewErr(usecase.KindConflict, "move.changed",
			errors.New("the tasks changed after the confirmation; run the command again"))
	}

	root := w.tasks[0]
	if !w.resumed {
		_, err := j.to.UpdateCommit(j.dst, store.Op{Name: "move-in", Task: root.ULID, Locked: l}, func(tx *store.Tx) error {
			return tx.MoveIn(store.MoveIn{
				From: j.src.Meta().ULID, Tasks: w.tasks, Workspace: w.workspace, Parent: w.parent,
				Notes: w.notes, Unlinked: w.plan.Unlinked, Scoped: w.scopedIn, Automations: w.automationsIn,
			})
		})
		if err != nil {
			return MoveStoreResult{}, j.moveInError(err, w)
		}
	}
	ulids := make([]model.ULID, len(w.tasks))
	for i, t := range w.tasks {
		ulids[i] = t.ULID
	}
	_, err = j.from.UpdateCommit(j.src, store.Op{Name: "move-out", Detail: root.ID + " " + root.Title, Locked: l}, func(tx *store.Tx) error {
		return tx.MoveOut(store.MoveOut{To: j.dst.Meta().ULID, Tasks: ulids, Unlinked: w.plan.Unlinked, Scoped: w.scopedOut, Automations: w.automationsOut})
	})
	if err != nil {
		if w.resumed {
			return MoveStoreResult{}, err
		}
		// Half a move is a state the rerun settles, whatever the I/O error
		// that stopped it: a conflict, not an unknown failure.
		kind := usecase.KindOf(err)
		if kind == usecase.KindUnknown {
			kind = usecase.KindConflict
		}
		return MoveStoreResult{}, &usecase.Error{
			Kind: kind, Code: "move.unfinished",
			Args: map[string]any{"store": j.toName},
			Err:  fmt.Errorf("moved into %s; run the same command again to finish the source: %w", j.toName, err),
		}
	}

	res := MoveStoreResult{Plan: w.plan, Carried: w.carried || w.carriedBefore, Resumed: w.resumed}
	for _, t := range w.tasks {
		nt, err := j.dst.Task(t.ULID)
		if err != nil {
			return MoveStoreResult{}, err
		}
		res.Moved = append(res.Moved, MovedTask{ULID: t.ULID, OldID: t.ID, NewID: nt.ID})
	}
	return res, nil
}

// samePlan reports whether b moves what a listed, and cuts what a listed.
func samePlan(a, b MovePlan) bool {
	ulids := func(p MovePlan) []model.ULID {
		out := make([]model.ULID, len(p.Tasks))
		for i, e := range p.Tasks {
			out[i] = e.ULID
		}
		return out
	}
	dropped := func(p MovePlan) []string {
		if p.Scoped == nil {
			return nil
		}
		return p.Scoped.Dropped
	}
	return slices.Equal(ulids(a), ulids(b)) && slices.Equal(a.Unlinked, b.Unlinked) && slices.Equal(dropped(a), dropped(b))
}

// moveJob is a move with its two stores opened.
type moveJob struct {
	in               MoveStoreInput
	from, to         *usecase.Session
	arg              string // in.Arg without the "<store>:" that named the source
	fromName, toName string
	src, dst         *store.Store
	srcIx, dstIx     *index.Index
	warned           map[string]bool // the files load has warned of
}

// moveWork is a move worked out from the two indexes: steps 1 to 4 and 6.
type moveWork struct {
	plan      MovePlan
	tasks     []*model.Task // the root first, every parent before its children
	workspace model.ULID    // in the destination
	parent    *model.ULID   // of the root, in the destination
	notes     map[string][]byte
	scopedIn  []schema.Scoped // carried to the destination
	scopedOut []string        // the wheres to take off the source
	// automationsIn and automationsOut are the new automations.yaml of the
	// destination and of the source; nil leaves the file.
	automationsIn, automationsOut []byte
	carried                       bool // the choice was to carry, and there was something to choose
	resumed                       bool // the destination has the tasks already (D12)
	// carriedBefore: on a rerun, the destination holds a scoped entry or an
	// automation of the projects, so the first run carried (its Dropped list
	// is not known).
	carriedBefore bool
}

// moveSource is the session of the store the task in.Arg is in, and arg as
// that session reads it. A task that a move into in.To began has copies in
// both stores, so an input that names it in the two is the one in the source.
func moveSource(ss *usecase.Session, in MoveStoreInput) (*usecase.Session, string, error) {
	from, arg, err := ss.ForTask(in.Arg)
	if err == nil {
		return from, arg, nil
	}
	var me *usecase.AmbiguousError
	if !errors.As(err, &me) {
		return nil, "", err
	}
	var elsewhere []usecase.Hit
	for _, h := range me.Candidates {
		if h.Store.Name != in.To {
			elsewhere = append(elsewhere, h)
		}
	}
	if len(elsewhere) != 1 {
		return nil, "", err
	}
	return ss.ForStore(elsewhere[0].Store.Name), in.Arg, nil
}

// openMove opens the two stores and their indexes, refusing a move within one
// store and a read-only store before any lock is taken.
func openMove(ss *usecase.Session, in MoveStoreInput) (*moveJob, error) {
	if in.To == "" {
		return nil, usecase.NewErr(usecase.KindUsage, "move.store_required", errors.New("the store to move to is needed (--store)"))
	}
	switch in.Scoped {
	case "", "carry", "drop":
	default:
		return nil, usecase.NewErr(usecase.KindUsage, "usage", fmt.Errorf("--scoped is carry or drop, not %q", in.Scoped))
	}
	from, arg, err := moveSource(ss, in)
	if err != nil {
		return nil, err
	}
	_, fe, err := from.Entry()
	if err != nil {
		return nil, err
	}
	to := ss.ForStore(in.To)
	_, te, err := to.Entry()
	if err != nil {
		return nil, err
	}
	if fe.Name == te.Name {
		return nil, usecase.NewErr(usecase.KindUsage, "move.same_store",
			fmt.Errorf("the task is in %s already; use hedgehug mv --workspace to move it to another workspace", fe.Name), "store", fe.Name)
	}
	j := &moveJob{warned: map[string]bool{}, in: in, from: from, to: to, arg: arg, fromName: fe.Name, toName: te.Name}
	if j.src, j.srcIx, err = from.OpenForWrite(); err != nil {
		return nil, err
	}
	if j.dst, j.dstIx, err = to.OpenForWrite(); err != nil {
		return nil, err
	}
	return j, nil
}

// load works the move out from the indexes as they are now. MoveStore runs it
// twice, before and under the locks: the first run fails early (choice,
// confirmation, schema) without taking a lock; the second is the one written.
func (j *moveJob) load() (*moveWork, error) {
	w := &moveWork{}
	root, err := j.from.Resolve(j.srcIx, j.arg)
	if err != nil {
		return nil, err
	}
	if err := usecase.NotDeleted(j.srcIx, root); err != nil {
		return nil, err
	}
	// Step 1: the task and everything under it, deleted tasks too. Descendants
	// walks depth first from the root, so every parent comes before its
	// children whatever their ULIDs are (MoveIn needs that).
	entries := append([]*index.Entry{root}, usecase.Descendants(j.srcIx, root.ULID, usecase.Every)...)
	inM := make(map[model.ULID]bool, len(entries))
	for _, x := range entries {
		if x.Broken != "" {
			return nil, usecase.NewErr(usecase.KindInvalid, "move.invalid", fmt.Errorf("%s does not decode, so it cannot be moved: %s", x.ULID, x.Broken))
		}
		inM[x.ULID] = true
		t, err := j.src.Task(x.ULID)
		if err != nil {
			return nil, err
		}
		w.tasks = append(w.tasks, t)
	}
	var rest []*model.Task
	for t, err := range j.src.Tasks() {
		var fe *store.FileError
		if errors.As(err, &fe) {
			// A file that does not decode holds no reference we could cut.
			// load runs twice; each file is told of once.
			if !j.warned[fe.Path] {
				j.warned[fe.Path] = true
				j.from.AddWarning(fmt.Sprintf("%s does not decode; references in it are not cut: %v", fe.Path, fe.Err))
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		if !inM[t.ULID] {
			rest = append(rest, t)
		}
	}

	// Step 4: the references to cut.
	srcSchema := j.srcIx.Schema()
	unlinked := store.Unlinks(srcSchema, w.tasks, rest)
	w.plan = MovePlan{From: j.fromName, To: j.toName, Tasks: entries, Unlinked: unlinked}
	w.plan.Labels = j.labels(entries, unlinked)

	// The scoped entries of the projects moved.
	var scoped []schema.Scoped
	var projects []*index.Entry
	for _, sp := range srcSchema.Scoped {
		where, ok := strings.CutPrefix(sp.Where, "project:")
		if !ok {
			continue
		}
		u, err := model.ParseULID(where)
		if err != nil || !inM[u] {
			continue
		}
		scoped = append(scoped, sp)
		projects = append(projects, mustEntry(j.srcIx, u))
		w.scopedOut = append(w.scopedOut, sp.Where)
	}
	// What automations.yaml has for them. The source loses it on a rerun too,
	// so it is worked out before the rerun is told apart.
	auto, err := readAutomations(j.src, j.fromName)
	if err != nil {
		return nil, err
	}
	var autoProjects []model.ULID
	for _, u := range auto.Projects() {
		if !inM[u] {
			continue
		}
		autoProjects = append(autoProjects, u)
		if !slices.ContainsFunc(projects, func(e *index.Entry) bool { return e.ULID == u }) {
			projects = append(projects, mustEntry(j.srcIx, u))
		}
	}
	if len(autoProjects) > 0 {
		if w.automationsOut, err = auto.Without(autoProjects).Encode(); err != nil {
			return nil, err
		}
	}
	if len(projects) > 0 {
		w.plan.Scoped = &ScopedPlan{Projects: projects}
		w.plan.ScopedChoice = j.in.Scoped
	}

	// A rerun after step 7: the tasks are in the destination already.
	resumed, err := j.resumeCheck(w.tasks)
	if err != nil {
		return nil, err
	}
	if resumed {
		w.resumed = true
		// What the first run carried is what the destination has now.
		for _, sp := range j.dstIx.Schema().Scoped {
			if slices.Contains(w.scopedOut, sp.Where) {
				w.carriedBefore = true
			}
		}
		if len(autoProjects) > 0 {
			dst, err := readAutomations(j.dst, j.toName)
			if err != nil {
				return nil, err
			}
			w.carriedBefore = w.carriedBefore || slices.ContainsFunc(dst.Projects(), func(u model.ULID) bool {
				return slices.Contains(autoProjects, u)
			})
		}
		if e, ok := j.dstIx.Task(w.tasks[0].ULID); ok {
			w.plan.Workspace, w.plan.Parent = j.placement(e.Workspace, e.Parent)
		}
		return w, nil
	}

	// Step 2: where they go.
	if err := j.place(w); err != nil {
		return nil, err
	}
	w.plan.Workspace, w.plan.Parent = j.placement(w.workspace, w.parent)
	// Step 6: the scoped entries and the automations.
	dest, err := CloneSchemaOf(j.dstIx.Schema())
	if err != nil {
		return nil, err
	}
	if len(projects) > 0 {
		switch j.in.Scoped {
		case "":
			if j.in.planOnly {
				break // asked for in the plan: scoped.required
			}
			ce := usecase.ChoiceRequired("--scoped", []string{"carry", "drop"})
			names := make([]string, len(projects))
			for i, p := range projects {
				names[i] = j.nameOf(p)
			}
			ce.Args["projects"] = names
			ce.Args["subject"] = scopedSubject(len(scoped) > 0, len(autoProjects) > 0)
			return nil, ce
		case "carry":
			w.carried = true
			if w.scopedIn, w.plan.Scoped.Dropped, err = j.carry(dest, scoped); err != nil {
				return nil, err
			}
			for _, sp := range w.scopedIn {
				dest.Scoped = slices.DeleteFunc(dest.Scoped, func(s schema.Scoped) bool { return s.Where == sp.Where })
				dest.Scoped = append(dest.Scoped, sp)
			}
			if len(autoProjects) > 0 {
				if err := j.carryAutomations(w, auto, autoProjects, dest); err != nil {
					return nil, err
				}
			}
		}
	}
	// Step 3: the schema of the destination, with what is carried.
	if err := j.validate(w, dest); err != nil {
		return nil, err
	}
	// The files of the tasks.
	w.notes = map[string][]byte{}
	for _, t := range w.tasks {
		notes, err := j.src.NoteFiles(t.ULID)
		if err != nil {
			return nil, err
		}
		for rel, b := range notes {
			w.notes[rel] = b
		}
	}
	w.plan.Hidden = j.hidden()
	return w, nil
}

// placement names the workspace and the parent of the root in the destination.
func (j *moveJob) placement(ws model.ULID, parent *model.ULID) (MoveWorkspaceView, *MoveTaskView) {
	wv := MoveWorkspaceView{ULID: ws}
	if w, ok := j.dstIx.Workspace(ws); ok {
		wv.Name, wv.Prefix = w.Name, w.IDPrefix
	}
	if parent == nil {
		return wv, nil
	}
	pv := &MoveTaskView{ULID: *parent}
	if e, ok := j.dstIx.Task(*parent); ok {
		pv.ID, pv.Title = j.dstIx.DisplayID(e), e.Title
	}
	return wv, pv
}

func mustEntry(ix *index.Index, ulid model.ULID) *index.Entry {
	e, _ := ix.Task(ulid)
	return e
}

// labels names the tasks of entries, and what unlinked cuts, for the plan.
func (j *moveJob) labels(entries []*index.Entry, unlinked []store.Unlink) map[model.ULID]string {
	out := map[model.ULID]string{}
	task := func(u model.ULID) {
		if e, ok := j.srcIx.Task(u); ok && e.ID != "" {
			out[u] = j.srcIx.DisplayID(e)
		}
	}
	for _, e := range entries {
		task(e.ULID)
	}
	for _, u := range unlinked {
		task(u.Task)
		switch u.Field {
		case "sprint":
			if sp, err := j.src.Sprint(u.Target); err == nil && sp.Name != "" {
				out[u.Target] = sp.Name
			}
		case "releases":
			if r, err := j.src.Release(u.Target); err == nil && r.Name != "" {
				out[u.Target] = r.Name
			}
		default:
			task(u.Target)
		}
	}
	return out
}

// resumeCheck tells whether tasks are in the destination because a move from
// this source began (D12): true when all are there, alive, and a move-in
// commit of this source names exactly them; an error when only some are, or
// when none of the history says why they are there.
func (j *moveJob) resumeCheck(tasks []*model.Task) (bool, error) {
	var present, alive int
	var first model.ULID
	for _, t := range tasks {
		e, ok := j.dstIx.Task(t.ULID)
		if !ok {
			continue
		}
		if present == 0 {
			first = t.ULID
		}
		present++
		if e.DeletedAt == nil {
			alive++
		}
	}
	if present == 0 {
		return false, nil
	}
	if alive == len(tasks) {
		done, err := j.moveInRecorded(tasks)
		if err != nil {
			return false, err
		}
		if done {
			return true, nil
		}
	}
	return false, usecase.NewErr(usecase.KindConflict, "move.exists", fmt.Errorf("%s is already in %s", first, j.toName), "store", j.toName, "ulid", first.String())
}

// moveInRecorded reports whether the history of the destination has the
// move-in commit of this source that moved exactly tasks.
func (j *moveJob) moveInRecorded(tasks []*model.Task) (bool, error) {
	want := make([]string, len(tasks))
	for i, t := range tasks {
		want[i] = t.ULID.String()
	}
	slices.Sort(want)
	h, err := j.to.OpenHistory(j.dst, j.toName)
	if err != nil {
		return false, err
	}
	var moveIns []history.Commit
	for _, c := range h.All() {
		if c.Op == "move-in" {
			moveIns = append(moveIns, c)
		}
	}
	log, err := h.Entries(moveIns, 0)
	if err != nil {
		return false, err
	}
	from := j.src.Meta().ULID.String()
	for _, c := range log {
		if f, _ := c.Trailer("Hedgehug-Moved-From"); f != from {
			continue
		}
		got := slices.Clone(c.TrailerValues("Hedgehug-Task"))
		slices.Sort(got)
		if slices.Equal(got, want) {
			return true, nil
		}
	}
	return false, nil
}

// place works out step 2: the workspace and the parent of the root in the
// destination.
func (j *moveJob) place(w *moveWork) error {
	var ws *model.Workspace
	if j.in.Workspace != "" {
		var err error
		if ws, err = usecase.ResolveWorkspace(j.dstIx, j.in.Workspace); err != nil {
			return err
		}
	}
	if j.in.Parent != "" {
		p, err := j.to.ResolveRef(j.dstIx, j.in.Parent)
		if err != nil {
			return err
		}
		if err := usecase.LiveParent(j.dstIx, p); err != nil {
			return err
		}
		if ws != nil && ws.ULID != p.Workspace {
			pw, _ := j.dstIx.Workspace(p.Workspace)
			name := p.Workspace.String()
			if pw != nil {
				name = pw.Name
			}
			return usecase.NewErr(usecase.KindInvalid, "move.invalid", fmt.Errorf("the parent is in workspace %s", name))
		}
		w.parent, w.workspace = &p.ULID, p.Workspace
		return nil
	}
	if ws == nil {
		all := j.dstIx.Workspaces()
		switch len(all) {
		case 0:
			return usecase.NewErr(usecase.KindInvalid, "move.invalid", fmt.Errorf("%s has no workspace", j.toName))
		case 1:
			ws = all[0]
		default:
			names := make([]string, len(all))
			for i, x := range all {
				names[i] = x.Name
			}
			return usecase.ChoiceRequired("--workspace", names)
		}
	}
	w.workspace = ws.ULID
	return nil
}

// carry cuts the scoped entries down to what the destination knows (step 6)
// and checks the schema they make.
func (j *moveJob) carry(dest *schema.Schema, scoped []schema.Scoped) (carried []schema.Scoped, dropped []string, err error) {
	hasTemplate := func(name string) bool {
		_, err := j.dst.Template(name)
		return err == nil
	}
	had := map[string]bool{}
	for _, p := range dest.Check() {
		had[p.String()] = true
	}
	next, err := CloneSchemaOf(dest)
	if err != nil {
		return nil, nil, err
	}
	for _, sp := range scoped {
		c, d := dest.Carry(sp, hasTemplate)
		carried = append(carried, c)
		for _, x := range d {
			if !slices.Contains(dropped, x) {
				dropped = append(dropped, x)
			}
		}
		next.Scoped = slices.DeleteFunc(next.Scoped, func(s schema.Scoped) bool { return s.Where == c.Where })
		next.Scoped = append(next.Scoped, c)
	}
	var lines []string
	for _, p := range next.Check() {
		if !had[p.String()] {
			lines = append(lines, "  "+p.String())
		}
	}
	if len(lines) > 0 {
		return nil, nil, usecase.NewErr(usecase.KindInvalid, "move.invalid",
			fmt.Errorf("the scoped schema does not fit %s even after cutting it down; use --scoped drop:\n%s", j.toName, strings.Join(lines, "\n")),
			"store", j.toName, "violations", lines)
	}
	return carried, dropped, nil
}

// carryAutomations cuts the automations of the projects down to what the
// destination knows, with dest as its schema, and sets w.automationsIn. Only
// the problems the move brings refuse it.
func (j *moveJob) carryAutomations(w *moveWork, src *automation.File, projects []model.ULID, dest *schema.Schema) error {
	cur, err := readAutomations(j.dst, j.toName)
	if err != nil {
		return err
	}
	next, dropped := cur.Carry(src, projects, dest)
	w.plan.Scoped.Dropped = append(w.plan.Scoped.Dropped, dropped...)
	// A project deleted goes with its parent and can be restored there.
	isProject := func(u model.ULID) bool {
		if slices.ContainsFunc(w.tasks, func(t *model.Task) bool { return t.ULID == u }) {
			e, ok := j.srcIx.Task(u)
			return ok && e.Kind == usecase.ProjectKind
		}
		e, ok := j.dstIx.Task(u)
		return ok && e.DeletedAt == nil && e.Kind == usecase.ProjectKind
	}
	var lines []string
	for _, p := range automation.Fresh(cur.Check(dest, isProject), next.Check(dest, isProject)) {
		lines = append(lines, "  "+p.String())
	}
	if len(lines) > 0 {
		return usecase.NewErr(usecase.KindInvalid, "move.invalid",
			fmt.Errorf("the automations do not fit %s even after cutting them down; use --scoped drop:\n%s", j.toName, strings.Join(lines, "\n")),
			"store", j.toName, "violations", lines)
	}
	before, err := cur.Encode()
	if err != nil {
		return err
	}
	after, err := next.Encode()
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		w.automationsIn = after
	}
	return nil
}

// readAutomations reads automations.yaml of the store name. A move cannot tell
// without it what the file holds for the tasks, so a file that cannot be read
// stops the move.
func readAutomations(s *store.Store, name string) (*automation.File, error) {
	f, err := s.Automations()
	if err != nil {
		return nil, usecase.NewErr(usecase.KindInvalid, "automation.invalid",
			fmt.Errorf("the move stops: %s of %s cannot be read: %w", model.AutomationsPath, name, err), "store", name)
	}
	return f, nil
}

// scopedSubject names what the --scoped choice is about.
func scopedSubject(scoped, automations bool) string {
	switch {
	case scoped && automations:
		return "scoped schema and automation rules"
	case automations:
		return "automation rules"
	}
	return "scoped schema"
}

// CloneSchemaOf copies sc deeply, through its file form.
func CloneSchemaOf(sc *schema.Schema) (*schema.Schema, error) {
	b, err := sc.Encode()
	if err != nil {
		return nil, err
	}
	return schema.Decode(b)
}

// validate runs the tasks through the checks of a write into the schema dest
// (step 3, without the transitions) and reports every violation, one line per
// task and violation.
func (j *moveJob) validate(w *moveWork, dest *schema.Schema) error {
	moved := make(map[model.ULID]*model.Task, len(w.tasks))
	for i, src := range w.tasks {
		b, err := src.Encode()
		if err != nil {
			return err
		}
		t, err := model.DecodeTask(b)
		if err != nil {
			return err
		}
		t.Workspace = w.workspace
		if i == 0 {
			t.Parent = w.parent
		}
		moved[t.ULID] = t
	}
	read := func(u model.ULID) (*model.Task, error) {
		if t, ok := moved[u]; ok {
			return t, nil
		}
		if e, ok := j.dstIx.Task(u); ok && e.Broken == "" {
			return &model.Task{ULID: e.ULID, Kind: e.Kind, Parent: e.Parent}, nil
		}
		return nil, store.ErrNotFound
	}
	var lines []string
	for i, src := range w.tasks {
		t := moved[src.ULID]
		parentKind, ancestors, err := store.Lineage(t, dest, read)
		if err != nil {
			return err
		}
		for _, v := range schema.ValidateWrite(nil, "", nil, t, parentKind, dest.For(ancestors), false) {
			lines = append(lines, fmt.Sprintf("  %s: %s", j.nameOf(w.plan.Tasks[i]), v))
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return invalidMove(j.toName, lines)
}

// nameOf is a task of the source as a line about it names it: its id and title.
func (j *moveJob) nameOf(e *index.Entry) string {
	id := e.ULID.String()
	if e.ID != "" {
		id = j.srcIx.DisplayID(e)
	}
	return id + " " + e.Title
}

// invalidMove is the error of a move the schema of the destination refuses.
func invalidMove(to string, lines []string) error {
	return usecase.NewErr(usecase.KindInvalid, "move.invalid",
		fmt.Errorf("%s refuses the tasks:\n%s", to, strings.Join(lines, "\n")), "store", to, "violations", lines)
}

// moveInError gives the error of the write into the destination the words of
// the move.
func (j *moveJob) moveInError(err error, w *moveWork) error {
	var ve *store.ValidationError
	if !errors.As(err, &ve) {
		return err
	}
	label := ve.Task.String()
	for _, e := range w.plan.Tasks {
		if e.ULID == ve.Task {
			label = j.nameOf(e)
		}
	}
	lines := make([]string, len(ve.Violations))
	for i, v := range ve.Violations {
		lines[i] = fmt.Sprintf("  %s: %s", label, v)
	}
	return invalidMove(j.toName, lines)
}

// hidden are the accounts that can read the source and not the destination;
// nil when either list of collaborators is unknown.
func (j *moveJob) hidden() []string {
	sm, err := replicate.ReadMembers(j.src)
	if err != nil || sm.Collaborators == nil {
		return nil
	}
	dm, err := replicate.ReadMembers(j.dst)
	if err != nil || dm.Collaborators == nil {
		return nil
	}
	var out []string
	for _, c := range sm.Collaborators {
		if !slices.ContainsFunc(dm.Collaborators, func(d replicate.Collaborator) bool { return config.SameAccount(c.ID, d.ID) }) {
			out = append(out, c.ID)
		}
	}
	slices.Sort(out)
	return out
}
