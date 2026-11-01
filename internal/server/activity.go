package server

import (
	"context"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/ops"
)

type undoKept struct {
	Path   string `json:"path"`
	Task   string `json:"task,omitempty" doc:"The ID of the task; empty for other files"`
	Field  string `json:"field"`
	Author string `json:"author"`
}

type undoWrite struct {
	Undone ops.LogView `json:"undone" doc:"The commit taken back"`
	Commit *string     `json:"commit"`
	Kept   []undoKept  `json:"kept" doc:"What others changed after it, which undo kept"`
}

type undoOutput struct{ Body undoWrite }

type activityInput struct {
	StoreParam
	Limit  int    `query:"limit" default:"100" minimum:"1" maximum:"1000"`
	Author string `query:"author" doc:"Only the commits of this account (its ID)"`
	Sync   string `query:"sync" enum:"only,exclude" doc:"only: the merges and repairs of syncs; exclude: all but them"`
}

type activityOutput struct{ Body []ops.LogView }

type notificationsInput struct {
	All bool `query:"all" doc:"Include the items read"`
}

type notification struct {
	ops.InboxItemView
	Read bool `json:"read"`
}

type notificationsOutput struct{ Body []notification }

// NotificationParam is the item a request is on. It is exported because huma
// skips an embedded struct whose type is not.
type NotificationParam struct {
	ID string `path:"id" doc:"The ULID of the item"`
}

type notificationPatchInput struct {
	NotificationParam
	Body struct {
		Read bool `json:"read" doc:"true marks it read; items cannot be marked unread"`
	}
}

// syncOps are the ops of the commits a sync makes.
var syncOps = []string{"merge", "repair"}

func (s *Server) registerActivity() {
	huma.Register(s.api, huma.Operation{
		OperationID: "undo", Method: http.MethodPost, Path: "/api/undo",
		Summary: "Take back my last operation, as hh undo", Tags: []string{"tasks"},
	}, func(_ context.Context, in *StoreParam) (*undoOutput, error) {
		ss := s.session(in.Store)
		r, err := ss.Undo()
		if err != nil {
			return nil, problemOf(err)
		}
		_, ix, err := ss.Open()
		if err != nil {
			return nil, problemOf(err)
		}
		out := &undoOutput{}
		out.Body.Undone = ops.LogViewsOf(ix, []gitrepo.LogEntry{r.Commit})[0]
		out.Body.Commit = commitPtr(r.NewCommit)
		out.Body.Kept = []undoKept{}
		for _, k := range r.Kept {
			out.Body.Kept = append(out.Body.Kept, undoKept{k.Path, k.Task, k.Field, k.Author})
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "list-activity", Method: http.MethodGet, Path: "/api/activity",
		Summary: "List the commits of the store, newest first, as hh log", Tags: []string{"activity"},
	}, func(_ context.Context, in *activityInput) (*activityOutput, error) {
		ix, entries, err := s.session(in.Store).Log(nil, 0)
		if err != nil {
			return nil, problemOf(err)
		}
		kept := []gitrepo.LogEntry{}
		for _, e := range entries {
			op, _ := e.Trailer("Hedgehug-Op")
			author, _ := e.Trailer("Hedgehug-Author")
			isSync := slices.Contains(syncOps, op)
			switch {
			case in.Sync == "only" && !isSync, in.Sync == "exclude" && isSync:
				continue
			case in.Author != "" && author != in.Author:
				continue
			}
			kept = append(kept, e)
			if len(kept) == in.Limit {
				break
			}
		}
		return &activityOutput{Body: ops.LogViewsOf(ix, kept)}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "list-notifications", Method: http.MethodGet, Path: "/api/notifications",
		Summary: "List the inbox of every store, newest first, as hh inbox", Tags: []string{"notifications"},
	}, func(_ context.Context, in *notificationsInput) (*notificationsOutput, error) {
		ss := s.session("")
		_, unread, err := ss.InboxList(false)
		if err != nil {
			return nil, problemOf(err)
		}
		items := unread
		if in.All {
			if _, items, err = ss.InboxList(true); err != nil {
				return nil, problemOf(err)
			}
		}
		isUnread := map[string]bool{}
		for _, it := range unread {
			isUnread[it.ULID.String()] = true
		}
		out := &notificationsOutput{Body: make([]notification, len(items))}
		for i, it := range items {
			out.Body[i] = notification{InboxItemView: it, Read: !isUnread[it.ULID.String()]}
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-notification", Method: http.MethodPatch, Path: "/api/notifications/{id}",
		Summary: "Mark an item read, as hh inbox read", Tags: []string{"notifications"},
	}, func(_ context.Context, in *notificationPatchInput) (*struct{}, error) {
		if !in.Body.Read {
			return nil, newProblem(http.StatusBadRequest, "notification.unread", "an item cannot be marked unread")
		}
		if _, err := s.session("").InboxRead([]string{in.ID}, false); err != nil {
			return nil, problemOf(err)
		}
		return nil, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "revert-notification", Method: http.MethodPost, Path: "/api/notifications/{id}/revert",
		Summary: "Write my overwritten value back, as hh inbox revert", Tags: []string{"notifications"},
	}, func(_ context.Context, in *NotificationParam) (*taskWriteOutput, error) {
		r, err := s.session("").InboxRevert(in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		// The item may be of any store; r.Index is the index of the store
		// InboxRevert wrote to.
		v, err := r.View()
		if err != nil {
			return nil, problemOf(err)
		}
		return &taskWriteOutput{Body: taskWrite{Task: v, Commit: commitPtr(r.Commit)}}, nil
	})
}
