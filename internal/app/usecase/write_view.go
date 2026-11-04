package usecase

// CommitPtr is the commit of a write as JSON has it: nil when nothing was
// committed.
func CommitPtr(hex string) *string {
	if hex == "" {
		return nil
	}
	return &hex
}

// TaskWrite is what a write to one task answers, in the CLI's --json and in
// the server's response.
type TaskWrite struct {
	Task   *TaskView `json:"task"`
	Commit *string   `json:"commit" doc:"The commit made; null when nothing changed"`
	Note   string    `json:"note,omitempty" doc:"What restore at a commit has to say, as hh restore --at prints it"`
	// Detached is for a move to another workspace.
	Detached []Detached `json:"detached,omitempty" doc:"What a move to another workspace took off the tasks: the sprint and the releases that are not of the new workspace, as hh mv tells it"`
}

// AffectedWrite is what rm and restore answer: the task, then the tasks
// under it.
type AffectedWrite struct {
	Tasks  []*TaskView `json:"tasks,omitempty" doc:"The task and the tasks under it, as they are now"`
	Commit *string     `json:"commit"`
}

// IDChange is a task id that a change of the id prefix, or a move to another
// workspace, replaced.
type IDChange struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// Write is the answer to the write: the task as it is now, and the commit.
func (r TaskResult) Write() (TaskWrite, error) {
	v, err := r.View()
	if err != nil {
		return TaskWrite{}, err
	}
	return TaskWrite{Task: v, Commit: CommitPtr(r.Commit)}, nil
}

// Write is the answer to the move: the task as it is now, the commit and
// what the move took off the tasks. Each Detached has its DisplayID.
func (r MoveResult) Write() (TaskWrite, error) {
	w, err := r.TaskResult.Write()
	if err != nil {
		return TaskWrite{}, err
	}
	for _, d := range r.Detached {
		d.DisplayID = r.index.QualifyID(d.ID)
		w.Detached = append(w.Detached, d)
	}
	return w, nil
}

// Renumbered is the ids the tasks had and have now, as the index shows them
// (qualified when another store has the prefix). The index is the one Write
// refreshed, so call it after Write.
func (r MoveResult) Renumbered() []IDChange {
	out := make([]IDChange, len(r.Tasks))
	for i, t := range r.Tasks {
		out[i] = IDChange{Old: r.index.QualifyID(r.OldIDs[i]), New: r.index.QualifyID(t.ID)}
	}
	return out
}
