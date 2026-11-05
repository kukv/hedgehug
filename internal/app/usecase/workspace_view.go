package usecase

import (
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
)

// WorkspaceView is a workspace as the CLI prints it with --json.
type WorkspaceView struct {
	ULID      string    `json:"ulid"`
	Name      string    `json:"name"`
	IDPrefix  string    `json:"id_prefix"`
	PrefixSet time.Time `json:"prefix_set"`
	Tasks     int       `json:"tasks"`   // not deleted
	Deleted   int       `json:"deleted"` // deleted and not purged yet
}

// WorkspaceViewOf is w as ix counts its tasks.
func WorkspaceViewOf(ix *index.Index, w *model.Workspace) WorkspaceView {
	var live, deleted int
	for _, e := range ix.Tasks() {
		switch {
		case e.Workspace != w.ULID:
		case e.DeletedAt == nil:
			live++
		default:
			deleted++
		}
	}
	return WorkspaceView{ULID: w.ULID.String(), Name: w.Name, IDPrefix: w.IDPrefix, PrefixSet: w.PrefixSet, Tasks: live, Deleted: deleted}
}
