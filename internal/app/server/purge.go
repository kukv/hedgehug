package server

import (
	"cmp"
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
)

type purgeListInput struct {
	StoreParam
	OlderThan string `query:"older_than" default:"90d" doc:"Only what is older than this many days, such as 90d (0d for everything), as hh purge --older-than"`
}

type purgePlanOutput struct{ Body maintenance.PurgePlan }

type purgeInput struct {
	StoreParam
	Body struct {
		OlderThan string                `json:"older_than,omitempty" doc:"As GET /api/purge; 90d without it"`
		Confirmed maintenance.PurgePlan `json:"confirmed" doc:"The plan GET /api/purge showed and the user agreed to; if what is to be purged is no longer that, the answer is 409 purge.changed"`
	}
}

type purgeOutput struct{ Body maintenance.PurgeResult }

func (s *Server) registerPurge() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-purge", Method: http.MethodGet, Path: "/api/purge",
		Summary: "List the deleted tasks and the forward files hh purge would remove", Tags: []string{"maintenance"},
	}, func(_ context.Context, in *purgeListInput) (*purgePlanOutput, error) {
		days, err := maintenance.ParseOlderThan(in.OlderThan)
		if err != nil {
			return nil, problemOf(err)
		}
		ss := s.session(in.Store)
		defer ss.Close()
		plan, err := maintenance.PurgeList(ss, days)
		if err != nil {
			return nil, problemOf(err)
		}
		return &purgePlanOutput{Body: plan}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "purge", Method: http.MethodPost, Path: "/api/purge",
		Summary: "Remove, for good, what the confirmed plan lists, as hh purge after its question", Tags: []string{"maintenance"},
	}, func(_ context.Context, in *purgeInput) (*purgeOutput, error) {
		days, err := maintenance.ParseOlderThan(cmp.Or(in.Body.OlderThan, fmt.Sprintf("%dd", maintenance.DefaultPurgeDays)))
		if err != nil {
			return nil, problemOf(err)
		}
		ss := s.session(in.Store)
		defer ss.Close()
		res, err := maintenance.PurgeConfirmed(ss, days, in.Body.Confirmed)
		if err != nil {
			return nil, problemOf(err)
		}
		s.poller.Kick()
		return &purgeOutput{Body: res}, nil
	})
}
