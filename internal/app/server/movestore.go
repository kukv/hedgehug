package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

// MoveStoreFields are what a move to another store takes, as hh mv --store.
// It is exported so that the body of a move can embed it.
type MoveStoreFields struct {
	To        string `json:"to" doc:"The store to move to, by name"`
	Workspace string `json:"workspace,omitempty" doc:"The workspace of that store, by name or prefix; needed when it has more than one (otherwise 400 choice.required)"`
	Parent    string `json:"parent,omitempty" doc:"A task of that store to put the task under: an ID or a ULID"`
	Scoped    string `json:"scoped,omitempty" enum:"carry,drop" doc:"Whether the scoped schema of the projects moved goes along; without it the plan says scoped.required, and the move is 400 choice.required"`
}

func (f MoveStoreFields) input(arg string) usecase.MoveStoreInput {
	return usecase.MoveStoreInput{Arg: arg, To: f.To, Workspace: f.Workspace, Parent: f.Parent, Scoped: f.Scoped}
}

type moveStorePlanInput struct {
	TaskParam
	Body MoveStoreFields
}

type moveStorePlanOutput struct{ Body usecase.MovePlanView }

type moveStoreInput struct {
	TaskParam
	Body struct {
		MoveStoreFields
		Confirmed *usecase.MovePlanView `json:"confirmed,omitempty" doc:"The plan POST /move-store/plan showed and the user agreed to; without it the answer is 400 confirm.required, and if the tasks are no longer what it lists, 409 move.changed"`
	}
}

type moveStoreOutput struct{ Body usecase.MoveStoreResultView }

func (s *Server) registerMoveStore() {
	huma.Register(s.api, huma.Operation{
		OperationID: "plan-move-store", Method: http.MethodPost, Path: "/api/tasks/{id}/move-store/plan",
		Summary: "Show what moving the task and the tasks under it to another store would do; writes nothing", Tags: []string{"tasks"},
	}, func(_ context.Context, in *moveStorePlanInput) (*moveStorePlanOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		v, err := ss.PrepareMovePlan(in.Body.input(in.ID))
		if err != nil {
			return nil, problemOf(err)
		}
		return &moveStorePlanOutput{Body: v}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "move-store", Method: http.MethodPost, Path: "/api/tasks/{id}/move-store",
		Summary: "Move the task and the tasks under it to another store, as hh mv --store after its question", Tags: []string{"tasks"},
	}, func(_ context.Context, in *moveStoreInput) (*moveStoreOutput, error) {
		mi := in.Body.input(in.ID)
		// Without a confirmed plan Yes is false, and MoveStore answers
		// confirm.required, as the CLI without --yes.
		mi.Yes, mi.ConfirmedView = in.Body.Confirmed != nil, in.Body.Confirmed
		ss := s.session(in.Store)
		defer ss.Close()
		res, err := ss.MoveStore(mi)
		// A move that stopped after the destination was written
		// (move.unfinished) changed it too, so the poller looks on any error.
		s.poller.Kick()
		if err != nil {
			return nil, problemOf(err)
		}
		return &moveStoreOutput{Body: res.View()}, nil
	})
}
