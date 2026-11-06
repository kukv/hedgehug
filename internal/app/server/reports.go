package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase/reports"
)

type reportBurndownInput struct {
	StoreParam
	Sprint    string `query:"sprint" doc:"The sprint; without it, the only active one"`
	Workspace string `query:"workspace"`
	By        string `query:"by" enum:"points,count," doc:"points (default) or count"`
}

type reportVelocityInput struct {
	StoreParam
	Workspace string `query:"workspace"`
	Last      int    `query:"last" minimum:"0" doc:"How many sprints; 0 for 10"`
}

type ReportSpanInput struct {
	StoreParam
	Workspace string `query:"workspace"`
	Since     string `query:"since" doc:"The first day; without it, 30 days ago"`
	Until     string `query:"until" doc:"The last day; without it, today"`
}

type reportCFDInput struct {
	ReportSpanInput
	By string `query:"by" enum:"category,status," doc:"category (default) or status"`
}

type reportEstimateInput struct {
	StoreParam
	Workspace string `query:"workspace"`
	Sprint    string `query:"sprint" doc:"Only the tasks of this sprint"`
}

func (s *Server) registerReports() {
	huma.Register(s.api, huma.Operation{
		OperationID: "report-burndown", Method: http.MethodGet, Path: "/api/reports/burndown",
		Summary: "A sprint day by day, as hh report burndown", Tags: []string{"reports"},
	}, func(_ context.Context, in *reportBurndownInput) (*struct{ Body reports.BurndownView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		v, err := reports.Burndown(ss, in.Sprint, in.Workspace, in.By)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body reports.BurndownView }{v}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "report-velocity", Method: http.MethodGet, Path: "/api/reports/velocity",
		Summary: "What the closed sprints finished, as hh report velocity", Tags: []string{"reports"},
	}, func(_ context.Context, in *reportVelocityInput) (*struct{ Body []reports.VelocityRow }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		v, err := reports.Velocity(ss, in.Workspace, in.Last)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body []reports.VelocityRow }{v}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "report-cfd", Method: http.MethodGet, Path: "/api/reports/cfd",
		Summary: "A cumulative flow diagram, as hh report cfd", Tags: []string{"reports"},
	}, func(_ context.Context, in *reportCFDInput) (*struct{ Body reports.CFDView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		v, err := reports.CFD(ss, in.Workspace, in.Since, in.Until, in.By)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body reports.CFDView }{v}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "report-cycle", Method: http.MethodGet, Path: "/api/reports/cycle",
		Summary: "Cycle and lead time, as hh report cycle", Tags: []string{"reports"},
	}, func(_ context.Context, in *ReportSpanInput) (*struct{ Body reports.CycleView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		v, err := reports.Cycle(ss, in.Workspace, in.Since, in.Until)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body reports.CycleView }{v}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "report-estimate", Method: http.MethodGet, Path: "/api/reports/estimate",
		Summary: "Estimates against the time logged, as hh report estimate", Tags: []string{"reports"},
	}, func(_ context.Context, in *reportEstimateInput) (*struct{ Body reports.EstimateView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		v, err := reports.Estimate(ss, in.Workspace, in.Sprint)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body reports.EstimateView }{v}, nil
	})
}
