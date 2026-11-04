package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
)

type presenceInput struct {
	Body struct {
		Client  string `json:"client" minLength:"1" doc:"The client that hello gave"`
		Visible bool   `json:"visible"`
	}
}

func (s *Server) registerSync() {
	huma.Register(s.api, huma.Operation{
		OperationID: "get-sync", Method: http.MethodGet, Path: "/api/sync",
		Summary: "How the syncs went, as hh sync --status", Tags: []string{"sync"},
	}, func(context.Context, *struct{}) (*struct{ Body []usecase.SyncStatusView }, error) {
		ss := s.session("")
		defer ss.Close()
		iv, err := ss.SyncIntervals()
		if err != nil {
			return nil, problemOf(err)
		}
		sts, err := usecase.SyncStatus(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		out := &struct{ Body []usecase.SyncStatusView }{[]usecase.SyncStatusView{}}
		for _, st := range sts {
			out.Body = append(out.Body, usecase.NewSyncStatusView(st, s.loop, iv))
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "sync", Method: http.MethodPost, Path: "/api/sync",
		Summary: "Sync now: the store given, or every store with a remote, as hh sync", Tags: []string{"sync"},
	}, func(_ context.Context, in *StoreParam) (*struct{ Body []usecase.SyncReport }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		reps, err := replication.Sync(ss, false, true)
		if len(reps) == 0 && err != nil {
			return nil, problemOf(err)
		}
		for i := range reps {
			if reps[i].Err != nil {
				reps[i].Error = usecase.Describe(reps[i].Err)
			}
			s.loop.Synced(reps[i].Store)
			s.loop.onState(reps[i].Store)
		}
		if reps == nil {
			reps = []usecase.SyncReport{}
		}
		return &struct{ Body []usecase.SyncReport }{reps}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "set-presence", Method: http.MethodPut, Path: "/api/presence",
		Summary: "Tell whether a tab shows the GUI", Tags: []string{"sync"},
	}, func(_ context.Context, in *presenceInput) (*struct{}, error) {
		if !s.presence.Set(in.Body.Client, in.Body.Visible) {
			return nil, newProblem(http.StatusNotFound, "presence.unknown_client", "no events connection is "+in.Body.Client)
		}
		return nil, nil
	})
}
