package server

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/ops"
)

type syncStatus struct {
	ops.StoreStatus
	NextFetch    *time.Time `json:"next_fetch,omitempty" doc:"When the loop fetches next; absent before its first sync"`
	PushAt       *time.Time `json:"push_at,omitempty" doc:"When the background sync pushes the writes not pushed yet: sync.push_interval after pending_since, but not before sync.push_interval after the last sync and not after next_fetch, which syncs too; absent when nothing waits, before the first sync and while a sync runs"`
	Interval     float64    `json:"interval" doc:"sync.interval of config.yaml, in seconds (fractions allowed)"`
	PushInterval float64    `json:"push_interval" doc:"sync.push_interval of config.yaml, in seconds (fractions allowed)"`
}

// newSyncStatus is st with when the loop fetches next and when the writes
// waiting are pushed, as the loop schedules them with c's sync intervals.
func newSyncStatus(st ops.StoreStatus, l *Loop, c *config.Config) syncStatus {
	v := syncStatus{
		StoreStatus:  st,
		Interval:     c.Sync.Interval.Seconds(),
		PushInterval: c.Sync.PushInterval.Seconds(),
	}
	if t, ok := l.NextFetch(st.Store); ok {
		v.NextFetch = &t
	}
	if st.PendingSince != nil {
		if t, ok := l.PushAt(st.Store, *st.PendingSince, l.fetchInterval(c), c.Sync.PushInterval); ok {
			v.PushAt = &t
		}
	}
	return v
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
		ss := s.session("")
		defer ss.Close()
		c, _, err := ss.Config()
		if err != nil {
			return nil, problemOf(err)
		}
		sts, err := ss.SyncStatus()
		if err != nil {
			return nil, problemOf(err)
		}
		out := &struct{ Body []syncStatus }{[]syncStatus{}}
		for _, st := range sts {
			out.Body = append(out.Body, newSyncStatus(st, s.loop, c))
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "sync", Method: http.MethodPost, Path: "/api/sync",
		Summary: "Sync now: the store given, or every store with a remote, as hh sync", Tags: []string{"sync"},
	}, func(_ context.Context, in *StoreParam) (*struct{ Body []ops.SyncReport }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		reps, err := ss.Sync(false, true)
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
