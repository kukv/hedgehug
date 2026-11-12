package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/items"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/members"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/projects"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/stores"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/system"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/workspaces"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	appmembers "github.com/kukv/hedgehug/internal/app/members"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
	appstores "github.com/kukv/hedgehug/internal/app/stores"
	appworkspaces "github.com/kukv/hedgehug/internal/app/workspaces"
)

// Deps は、API が使う口。トラックが、自分の API に要る口を足す。
type Deps struct {
	Version  string
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity

	// トラック A2(Store の一覧。本物はトラック G)
	StoreList port.StoreList

	// トラック D(GitHub)
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	Clock    port.Clock
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
	stores.Register(api, appstores.Service{Stores: deps.StoreList})
	members.Register(api, appmembers.Service{Stores: deps.Stores, Identity: deps.Identity})
	projects.Register(api, appprojects.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity})
	items.Register(api, appitems.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity})
	workspaces.Register(api, appworkspaces.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity})
	ghapi.Register(api,
		ghapp.ListAccounts{Accounts: deps.Accounts},
		ghapp.AddAccount{Accounts: deps.Accounts, Secrets: deps.Secrets, GitHub: deps.GitHub, IDs: deps.IDs, Clock: deps.Clock})

	return api
}
