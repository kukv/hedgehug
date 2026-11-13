package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/automations"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/events"
	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/items"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/members"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/projects"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/stores"
	syncapi "github.com/kukv/hedgehug/internal/adapter/httpapi/syncing"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/system"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/workspaces"
	appautomations "github.com/kukv/hedgehug/internal/app/automations"
	appcomments "github.com/kukv/hedgehug/internal/app/comments"
	appdescriptions "github.com/kukv/hedgehug/internal/app/descriptions"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	appmembers "github.com/kukv/hedgehug/internal/app/members"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
	appstores "github.com/kukv/hedgehug/internal/app/stores"
	appsyncing "github.com/kukv/hedgehug/internal/app/syncing"
	appworklogs "github.com/kukv/hedgehug/internal/app/worklogs"
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

	// トラック B(自動化・通知)
	Inbox port.Inbox // 受信箱(本物は C4)

	// トラック D(GitHub)
	Accounts    port.Accounts
	Secrets     port.Secrets
	GitHub      port.GitHub
	DeviceFlow  port.DeviceFlow
	Hosts       port.Hosts
	StoreGitHub port.StoreGitHub
	Clock       port.Clock

	// トラック C(同期)。Sync と Auto はサーバーに 1 つで、cmd が作る(自動の同期を動かすため)。
	Sync   *appsyncing.Service
	Auto   *appsyncing.Auto
	Events *events.Hub // nil なら NewAPI が作る(知らせる者がいない)
	// GitHubSession は、GitHub の API と同期のトークンの入口で共用する Session。nil なら githubServices が作る。
	GitHubSession *ghapp.Session
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
	items.Register(api, appitems.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity, Clock: deps.Clock, Inbox: deps.Inbox})
	workspaces.Register(api, appworkspaces.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity, Inbox: deps.Inbox})
	notes.Register(api, notes.Services{
		Descriptions: appdescriptions.Service{Stores: deps.Stores, Identity: deps.Identity},
		Comments:     appcomments.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity},
		Worklogs:     appworklogs.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity, Clock: deps.Clock},
	})
	automations.Register(api, appautomations.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity, Clock: deps.Clock})
	ghapi.Register(api, githubServices(deps))
	hub := deps.Events
	if hub == nil {
		hub = events.NewHub()
	}
	events.Register(api, hub)
	syncapi.Register(api, syncapi.Services{Sync: deps.Sync, Auto: deps.Auto})

	return api
}

// githubServices は、GitHub 連携の usecase を組む。Session はサーバーに 1 つ(アプリを開いているあいだの状態)。
// D3 で Connector を使う API を足すときは、同じ s から ghapp.Connector{Session: s, ...} を作る(Session を 2 つ作らない)。
func githubServices(d Deps) ghapi.Services {
	s := d.GitHubSession
	if s == nil {
		s = ghapp.NewSession()
	}
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
