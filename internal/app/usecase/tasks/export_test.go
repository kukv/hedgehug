package tasks

import "github.com/kukv/hedgehug/internal/lib/gitrepo"

func (s *undoScanner) Next(a0 gitrepo.LogEntry, a1 string) (gitrepo.LogEntry, bool) {
	return s.next(a0, a1)
}

var SameTime = sameTime

var SameULID = sameULID

type UndoScanner = undoScanner

// StageOf is the stage of the spec that decided where AddHere put the task.
func StageOf(r AddHereResult) int { return r.stage }
