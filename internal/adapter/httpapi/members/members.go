// Package members は、Store のメンバーと「あなた」の API を持つ。
package members

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appmembers "github.com/kukv/hedgehug/internal/app/members"
	"github.com/kukv/hedgehug/internal/app/port"
)

// Member は、API が返すメンバー。
type Member struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

// Me は、「あなた」のログイン名。
type Me struct {
	Login string `json:"login"`
}

type storePath struct {
	Store string `path:"store"`
}

type listOutput struct{ Body []Member }

type meOutput struct{ Body Me }

// Register は、メンバーの API を登録する。
func Register(api huma.API, svc appmembers.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-members", Method: http.MethodGet, Path: "/api/stores/{store}/members", Summary: "Store のメンバーを返す"},
		func(ctx context.Context, in *storePath) (*listOutput, error) {
			list, err := svc.List(ctx, port.StoreID(in.Store))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &listOutput{Body: []Member{}}
			for _, m := range list {
				out.Body = append(out.Body, Member{Login: m.Login, Name: m.Name})
			}
			return out, nil
		})
	// 「あなた」が決まらないときは、エラーをそのまま返し 500 になる。
	huma.Register(api, huma.Operation{OperationID: "get-me", Method: http.MethodGet, Path: "/api/stores/{store}/me", Summary: "Store での「あなた」のログイン名を返す"},
		func(ctx context.Context, in *storePath) (*meOutput, error) {
			login, err := svc.Me(ctx, port.StoreID(in.Store))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &meOutput{Body: Me{Login: login}}, nil
		})
}
