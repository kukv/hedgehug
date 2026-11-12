package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/items"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/projects"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/system"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/workspaces"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
	appworkspaces "github.com/kukv/hedgehug/internal/app/workspaces"
)

// Deps は、API が使う口。トラックが、自分の API に要る口を足す。
type Deps struct {
	Version  string
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity

	// トラック D(GitHub)
	Accounts    port.Accounts
	Secrets     port.Secrets
	GitHub      port.GitHub
	DeviceFlow  port.DeviceFlow
	Hosts       port.Hosts
	StoreGitHub port.StoreGitHub
	Clock       port.Clock
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
	workspaces.Register(api, appworkspaces.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity})
	ghapi.Register(api, githubServices(deps))

	return api
}

// githubServices は、GitHub 連携の usecase を組む。Session はサーバーに 1 つ(アプリを開いているあいだの状態)。
// Connector を使う API を足すときも、同じ Session から作る。
func githubServices(d Deps) ghapi.Services {
	s := ghapp.NewSession()
	return ghapi.Services{
		List:   ghapp.ListAccounts{Accounts: d.Accounts, Session: s},
		Add:    ghapp.AddAccount{Accounts: d.Accounts, Secrets: d.Secrets, GitHub: d.GitHub, IDs: d.IDs, Clock: d.Clock},
		Edit:   ghapp.EditAccount{Session: s, Accounts: d.Accounts, Secrets: d.Secrets, GitHub: d.GitHub, Clock: d.Clock},
		Delete: ghapp.DeleteAccount{Session: s, Accounts: d.Accounts, Secrets: d.Secrets, Stores: d.StoreGitHub},
		Reauth: ghapp.Reauth{Session: s, Accounts: d.Accounts, Secrets: d.Secrets, GitHub: d.GitHub, Clock: d.Clock},
		Impact: ghapp.Impact{Accounts: d.Accounts, Stores: d.StoreGitHub},
		SignIn: ghapp.SignIn{
			Session: s, Accounts: d.Accounts, Secrets: d.Secrets, GitHub: d.GitHub, Device: d.DeviceFlow,
			Hosts: d.Hosts, IDs: d.IDs, Clock: d.Clock,
		},
	}
}
