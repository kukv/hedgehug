package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

type accountAddInput struct {
	Body struct {
		Name  string `json:"name" doc:"The name of the account in config.yaml"`
		Login string `json:"login" doc:"The GitHub login of the account"`
		Host  string `json:"host,omitempty" doc:"github.com without it"`
		Token string `json:"token,omitempty" doc:"Where the token comes from: gh or env:<VAR>; with none the account has no token. keychain and device are answered with 400 auth.token_source: a pasted token goes through PUT /api/accounts/{name}/token, and the device flow through POST /api/auth/device"`
	}
}

type accountAddOutput struct{ Body usecase.AuthAddResult }

type accountTokenInput struct {
	Name string `path:"name" doc:"The name of the account; a name that is no account's adds the account of the login the token is of"`
	Body struct {
		Token string `json:"token" doc:"The token to keep in the keychain, as hh auth login --with-token reads it. It is in no response, no event and no line of the log"`
		Host  string `json:"host,omitempty" doc:"The host of a new account (github.com without it); for an account that exists it must be its host"`
	}
}

// accountLoginBody is an AuthLoginResult as the API answers it. Uses is not
// in the CLI's JSON (AuthLoginResult.Uses is json:"-"), but a GUI that sent a
// token to an account of gh has to be told that nothing was kept.
type accountLoginBody struct {
	Account string `json:"account"`
	ID      string `json:"id"`
	Added   bool   `json:"added" doc:"The account was added to config.yaml: the name was no account's"`
	Uses    string `json:"uses,omitempty" doc:"gh or env:VAR: the account takes its token from there, so Hedgehug has nothing to log in and kept nothing"`
}

type accountLoginOutput struct{ Body accountLoginBody }

type accountParam struct {
	Name string `path:"name" doc:"The name of the account in config.yaml"`
}

func (s *Server) registerAccounts() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-accounts", Method: http.MethodGet, Path: "/api/accounts",
		Summary: "Check the token of each account against GitHub, as hh auth status --json; an account in error is in the array with its state", Tags: []string{"accounts"},
	}, func(_ context.Context, _ *struct{}) (*struct{ Body []usecase.AuthStatus }, error) {
		ss := s.session("")
		defer ss.Close()
		out, err := ss.AuthStatus()
		if err != nil && !errors.Is(err, usecase.ErrFindings) {
			return nil, problemOf(err)
		}
		if out == nil {
			out = []usecase.AuthStatus{}
		}
		return &struct{ Body []usecase.AuthStatus }{out}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "add-account", Method: http.MethodPost, Path: "/api/accounts", DefaultStatus: http.StatusCreated,
		Summary: "Add an account to config.yaml, as hh auth add", Tags: []string{"accounts"},
	}, func(_ context.Context, in *accountAddInput) (*accountAddOutput, error) {
		switch tok := strings.TrimSpace(in.Body.Token); tok {
		case "keychain", "device":
			p := newProblem(http.StatusBadRequest, "auth.token_source",
				fmt.Sprintf("token %q cannot be given here: keep a pasted token with PUT /api/accounts/{name}/token, or log in with POST /api/auth/device", tok))
			p.Args = map[string]any{"token": tok}
			return nil, p
		}
		ss := s.session("")
		defer ss.Close()
		res, err := ss.AuthAdd(usecase.AuthAddInput{Name: in.Body.Name, Login: in.Body.Login, Host: in.Body.Host, Token: in.Body.Token})
		if err != nil {
			return nil, problemOf(err)
		}
		s.pinnedStores(res.Pinned)
		return &accountAddOutput{Body: res}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "set-account-token", Method: http.MethodPut, Path: "/api/accounts/{name}/token",
		Summary: "Keep a token in the keychain after GitHub says it is of the account's login, as hh auth login <name> --with-token", Tags: []string{"accounts"},
	}, func(_ context.Context, in *accountTokenInput) (*accountLoginOutput, error) {
		ss := s.session("")
		defer ss.Close()
		res, err := ss.AuthLogin(usecase.AuthLoginInput{
			Name: in.Name, Host: in.Body.Host, HostGiven: in.Body.Host != "",
			WithToken: true, Token: in.Body.Token,
		})
		if err != nil {
			return nil, problemOf(err)
		}
		s.pinnedStores(res.Pinned)
		return &accountLoginOutput{Body: loginBody(res)}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-account-token", Method: http.MethodDelete, Path: "/api/accounts/{name}/token",
		Summary: "Remove the token of an account from the keychain, as hh auth logout; the account stays in config.yaml", Tags: []string{"accounts"},
	}, func(_ context.Context, in *accountParam) (*struct{ Body usecase.AuthLogoutResult }, error) {
		ss := s.session("")
		defer ss.Close()
		res, err := ss.AuthLogout(in.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body usecase.AuthLogoutResult }{res}, nil
	})
}

func loginBody(r usecase.AuthLoginResult) accountLoginBody {
	return accountLoginBody{Account: r.Account, ID: r.ID, Added: r.Added, Uses: r.Uses}
}

// pinnedStores tells the GUI when a write of an account also wrote the
// account of stores (the stores that had none were given the first account
// before a second one was added).
func (s *Server) pinnedStores(p usecase.Pinned) {
	if len(p.Stores) > 0 {
		s.storesWritten()
	}
}
