// Package stores は、このマシンの Store の一覧の API を持つ。
package stores

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appstores "github.com/kukv/hedgehug/internal/app/stores"
)

// StoreInfo は、API が返す Store。
type StoreInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color" doc:"画面のトークン --t-<色> の名前"`
	ReadOnly bool   `json:"readOnly"`
}

type listOutput struct{ Body []StoreInfo }

// Register は、Store の一覧の API を登録する。
func Register(api huma.API, svc appstores.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-stores", Method: http.MethodGet, Path: "/api/stores", Summary: "表示中の Store をサイドバーの並びで返す"},
		func(ctx context.Context, _ *struct{}) (*listOutput, error) {
			list, err := svc.All(ctx)
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &listOutput{Body: []StoreInfo{}}
			for _, s := range list {
				out.Body = append(out.Body, StoreInfo{ID: string(s.ID), Name: s.Name, Color: s.Color, ReadOnly: s.ReadOnly})
			}
			return out, nil
		})
}
