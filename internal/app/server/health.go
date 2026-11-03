package server

import (
	"context"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
)

type healthOutput struct {
	Body struct {
		Version string `json:"version" doc:"The version of hedgehug"`
		PID     int    `json:"pid" doc:"The process of the server"`
	}
}

func (s *Server) registerHealth() {
	huma.Register(s.api, huma.Operation{
		OperationID: "get-health", Method: http.MethodGet, Path: "/api/health",
		Summary: "Check that the server is up", Tags: []string{"server"},
	}, func(context.Context, *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Version, out.Body.PID = s.opt.Version, os.Getpid()
		return out, nil
	})
}
