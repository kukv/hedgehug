package server

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

type undoOutput struct{ Body tasks.UndoWrite }

type activityInput struct {
	Store  string `query:"store" doc:"The store; without it, the commits of every store"`
	Limit  int    `query:"limit" default:"100" minimum:"1" maximum:"1000"`
	Author string `query:"author" doc:"Only the commits of this account (its ID)"`
	Sync   string `query:"sync" enum:"only,exclude" doc:"only: the merges and repairs of syncs; exclude: all but them"`
	// Automation with Sync=exclude leaves what people did.
	Automation string `query:"automation" enum:"only,exclude" doc:"only: what GitHub and the rules wrote, and the fetches that note a refused rule; exclude: all but them"`
	// Workspace is a comma-separated string, as confirm is.
	Workspace string `query:"workspace" doc:"Only the commits of these workspaces (ULID, name or id prefix), comma-separated, as hh log --workspace"`
}

type activityOutput struct{ Body []usecase.LogView }

type notificationsInput struct {
	StoreParam
	All bool `query:"all" doc:"Include the items read"`
}

type notificationsOutput struct{ Body []notifications.Notification }

// NotificationParam is the item a request is on. It is exported because huma
// skips an embedded struct whose type is not.
type NotificationParam struct {
	StoreParam
	ID string `path:"id" doc:"The ULID of the item"`
}

type notificationPatchInput struct {
	NotificationParam
	Body struct {
		Read bool `json:"read" doc:"true marks it read; items cannot be marked unread"`
	}
}

type notificationReadInput struct {
	StoreParam
	Body struct {
		All bool     `json:"all,omitempty" doc:"true marks every unread item read; not with ids"`
		IDs []string `json:"ids,omitempty" nullable:"false" doc:"The ULIDs of the items to mark read; not with all"`
	}
}

type notificationReadOutput struct{ Body notifications.ReadResult }

// syncOps are the ops of the commits a sync makes.
var syncOps = []string{"merge", "repair"}

func (s *Server) registerActivity() {
	huma.Register(s.api, huma.Operation{
		OperationID: "undo", Method: http.MethodPost, Path: "/api/undo",
		Summary: "Take back my last operation, as hh undo", Tags: []string{"tasks"},
	}, func(_ context.Context, in *StoreParam) (*undoOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := tasks.Undo(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		return &undoOutput{Body: r.Write()}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "list-activity", Method: http.MethodGet, Path: "/api/activity",
		Summary: "List the commits of the store, newest first, as hh log", Tags: []string{"activity"},
	}, func(_ context.Context, in *activityInput) (*activityOutput, error) {
		// Without a filter the first Limit commits are the answer.
		limit := 0
		if in.Author == "" && in.Sync == "" && in.Automation == "" {
			limit = in.Limit
		}
		var workspaces []string
		if in.Workspace != "" {
			workspaces = strings.Split(in.Workspace, ",")
		}
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := browse.LogIn(ss, nil, workspaces, limit)
		if err != nil {
			return nil, problemOf(err)
		}
		kept := []usecase.LogView{}
		for _, l := range r.Logs {
			op, _ := l.Trailer("Hedgehug-Op")
			author, _ := l.Trailer("Hedgehug-Author")
			isSync := slices.Contains(syncOps, op)
			isAutomated := usecase.AutomatedCommit(l.Trailer)
			switch {
			case in.Sync == "only" && !isSync, in.Sync == "exclude" && isSync:
				continue
			case in.Automation == "only" && !isAutomated, in.Automation == "exclude" && isAutomated:
				continue
			case in.Author != "" && author != in.Author:
				continue
			}
			kept = append(kept, l)
			if len(kept) == in.Limit {
				break
			}
		}
		return &activityOutput{Body: kept}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "list-notifications", Method: http.MethodGet, Path: "/api/notifications",
		Summary: "List the inbox of every store (of ?store= only), newest first, as hh inbox", Tags: []string{"notifications"},
	}, func(_ context.Context, in *notificationsInput) (*notificationsOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		items, err := notifications.InboxNotifications(ss, in.All)
		if err != nil {
			return nil, problemOf(err)
		}
		return &notificationsOutput{Body: items}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-notification", Method: http.MethodPatch, Path: "/api/notifications/{id}",
		Summary: "Mark an item read, as hh inbox read", Tags: []string{"notifications"},
	}, func(_ context.Context, in *notificationPatchInput) (*struct{}, error) {
		if !in.Body.Read {
			return nil, newProblem(http.StatusBadRequest, "notification.unread", "an item cannot be marked unread")
		}
		ss := s.session(in.Store)
		defer ss.Close()
		if _, err := notifications.InboxRead(ss, []string{in.ID}, false); err != nil {
			return nil, problemOf(err)
		}
		s.poller.Kick()
		return nil, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "read-notifications", Method: http.MethodPost, Path: "/api/notifications/read",
		Summary: "Mark items read, all of them or the ids given, as hh inbox read", Tags: []string{"notifications"},
	}, func(_ context.Context, in *notificationReadInput) (*notificationReadOutput, error) {
		if in.Body.All == (len(in.Body.IDs) > 0) {
			return nil, newProblem(http.StatusBadRequest, "notification.read_args", "give either all: true or ids, not both and not neither")
		}
		ss := s.session(in.Store)
		defer ss.Close()
		ulids, err := notifications.ParseInboxItems(in.Body.IDs)
		if err != nil {
			return nil, problemOf(err)
		}
		stores, err := notifications.InboxStores(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		read, err := notifications.InboxReadOf(ss, stores, ulids, in.Body.All)
		if err != nil {
			return nil, problemOf(err)
		}
		s.poller.Kick()
		return &notificationReadOutput{Body: notifications.ReadResult{Read: read}}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "revert-notification", Method: http.MethodPost, Path: "/api/notifications/{id}/revert",
		Summary: "Write my overwritten value back, as hh inbox revert", Tags: []string{"notifications"},
	}, func(_ context.Context, in *NotificationParam) (*taskWriteOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := notifications.InboxRevert(ss, in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		// The item may be of any store; the result is read from the store
		// InboxRevert wrote to.
		return writeOut(r)
	})
}
