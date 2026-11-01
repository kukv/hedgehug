package server

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/ops"
)

type syncStatus struct {
	ops.StoreStatus
	NextFetch *time.Time `json:"next_fetch,omitempty" doc:"When the loop fetches next; absent before its first sync"`
}

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
	}, func(context.Context, *struct{}) (*struct{ Body []syncStatus }, error) {
		sts, err := s.session("").SyncStatus()
		if err != nil {
			return nil, problemOf(err)
		}
		out := &struct{ Body []syncStatus }{[]syncStatus{}}
		for _, st := range sts {
			v := syncStatus{StoreStatus: st}
			if t, ok := s.loop.NextFetch(st.Store); ok {
				v.NextFetch = &t
			}
			out.Body = append(out.Body, v)
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "sync", Method: http.MethodPost, Path: "/api/sync",
		Summary: "Sync now: the store given, or every store with a remote, as hh sync", Tags: []string{"sync"},
	}, func(_ context.Context, in *StoreParam) (*struct{ Body []ops.SyncReport }, error) {
		reps, err := s.session(in.Store).Sync(false, true)
		if len(reps) == 0 && err != nil {
			return nil, problemOf(err)
		}
		for i := range reps {
			if reps[i].Err != nil {
				reps[i].Error = detailOf(reps[i].Err)
			}
			s.loop.Synced(reps[i].Store)
			s.loop.onState(reps[i].Store)
		}
		if reps == nil {
			reps = []ops.SyncReport{}
		}
		return &struct{ Body []ops.SyncReport }{reps}, nil
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
