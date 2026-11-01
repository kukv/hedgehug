package server

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"
)

// pingInterval keeps the connection open through proxies and browsers.
const pingInterval = 15 * time.Second

type helloEvent struct {
	Client string       `json:"client" doc:"This connection, for PUT /api/presence"`
	Stores []storeState `json:"stores" doc:"The HEAD and the sync state of each store now"`
	Unread int          `json:"unread"`
}

func (s *Server) registerEvents() {
	sse.Register(s.api, huma.Operation{
		OperationID: "events", Method: http.MethodGet, Path: "/api/events",
		Summary: "The changes, as server-sent events; hello comes first", Tags: []string{"events"},
	}, map[string]any{
		"hello":                helloEvent{},
		"tasks.changed":        tasksChanged{},
		"sync.state":           syncStatus{},
		"notifications.unread": unreadEvent{},
		"schema.changed":       schemaChanged{},
		"views.changed":        viewsChanged{},
	}, func(ctx context.Context, _ *struct{}, send sse.Sender) {
		events, stop := s.hub.Subscribe() // before hello: nothing after it is missed
		defer stop()
		if s.hub.isClosed() {
			return // after Shutdown
		}
		id := s.presence.Connect()
		defer s.presence.Disconnect(id)
		stores, unread := s.poller.Snapshot()
		if err := send.Data(helloEvent{Client: id, Stores: stores, Unread: unread}); err != nil {
			return
		}
		ping := time.NewTicker(pingInterval)
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-events:
				if !ok {
					return
				}
				if err := send.Data(e); err != nil {
					return
				}
			case <-ping.C:
				if err := send.Comment("ping"); err != nil {
					return
				}
			}
		}
	})
}
