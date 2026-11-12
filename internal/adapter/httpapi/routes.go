package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/items"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/projects"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/system"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
)

// Deps は、API が使う口。トラックが、自分の API に要る口を足す。
type Deps struct {
	Version  string
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
}

// NewAPI は、mux に API を登録する。機能の API は httpapi/<機能> の Register を 1 行ずつ足す。
// 本文の型には機能の名前を付ける(例: system.SystemInfo。OpenAPI のスキーマ名になる)。
func NewAPI(mux *http.ServeMux, deps Deps) huma.API {
	cfg := huma.DefaultConfig("hedgehug", deps.Version)
	cfg.OpenAPIPath = "" // OpenAPI は make generate で api/openapi.yaml に書く。サーバーでは出さない
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	// 既定の CreateHooks は、本文に $schema(出さないパスへのリンク)を足す。本文は {"version"} のような約束どおりにする
	cfg.CreateHooks = nil
	api := humago.New(mux, cfg)

	system.Register(api, deps.Version)
	projects.Register(api, appprojects.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity})
	items.Register(api, appitems.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity})

	return api
}
