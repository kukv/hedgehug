// Package system は、サーバーそのものについての API を持つ。
//
// 機能の API はこの形をまねる。本文の型には機能の名前を付ける(OpenAPI のスキーマ名になる)。
package system

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// SystemInfo は、サーバーについての情報。
type SystemInfo struct {
	Version string `json:"version" doc:"hedgehug の版"`
}

type output struct {
	Body SystemInfo
}

// Register は GET /api/system を登録する。
func Register(api huma.API, version string) {
	huma.Register(api, huma.Operation{
		OperationID: "get-system",
		Method:      http.MethodGet,
		Path:        "/api/system",
		Summary:     "サーバーの版を返す",
	}, func(context.Context, *struct{}) (*output, error) {
		return &output{Body: SystemInfo{Version: version}}, nil
	})
}
