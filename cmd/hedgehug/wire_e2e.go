//go:build e2e

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/events"
	"github.com/kukv/hedgehug/internal/app/apptest"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/infra/statedb"
)

// e2e 用のビルドの Store の ID。どれにもサンプルデータを入れる。readonlyStore は閲覧のみの Store。teamStore は
// remote のない書き込める Store(別の Store への移動を通しのテストで流すため。A5a-20)。
const (
	e2eStore      = "01E2ESTORE0000000000000001"
	readonlyStore = "01E2ESTORE0000000000000002"
	teamStore     = "01E2ESTORE0000000000000003"
	// e2eAccount は、最初から入っているアカウント(login e2e-bot)。2 つの Store に割り当て、リンクの取り直しが GitHub に届くようにする。
	e2eAccount = "e2e-account"
)

// wire は、Playwright の通しのテスト用に、口に apptest の偽物とサンプルデータを差し込む(設計書 6 節)。
// トークンか待ち受けが空なら起動しない(空のトークンでは誰でも API を使え、空の待ち受けは全アドレスで待つため)。
// 待ち受けがこのマシンの中(ループバック)でなければ起動しない(固定のトークンを外に見せないため)。
// 自動の同期と同じく、GitHub の自動の読み取り(Reader)も動かさない。読み取りは入口 POST /e2e/github-read で行う(D4a-19)。
func wire(ctx context.Context) (httpapi.Deps, error) {
	if os.Getenv("HEDGEHUG_TOKEN") == "" || os.Getenv("HEDGEHUG_ADDR") == "" {
		return httpapi.Deps{}, errors.New("e2e のビルドには HEDGEHUG_TOKEN と HEDGEHUG_ADDR が要る")
	}
	if err := checkLoopback(os.Getenv("HEDGEHUG_ADDR")); err != nil {
		return httpapi.Deps{}, err
	}
	ids := apptest.NewIDs()
	stores, err := e2eStores(ctx, ids)
	if err != nil {
		return httpapi.Deps{}, err
	}
	accounts := apptest.NewAccounts()
	if err := accounts.Add(ctx, account.Account{ID: e2eAccount, Name: "e2e", Login: "e2e-bot", Method: account.EnvVar}); err != nil {
		return httpapi.Deps{}, err
	}
	state, err := e2eReads(ctx)
	if err != nil {
		return httpapi.Deps{}, err
	}
	hub := events.NewHub()
	context.AfterFunc(ctx, hub.Close) // 止めるとき SSE の流れを終える(決定 C2-27)
	syncSvc := e2eSync(hub, stores)
	go (&syncing.Watch{Versions: stores, Events: hub}).Run(ctx, 500*time.Millisecond)
	auto := syncing.NewAuto(syncSvc, &apptest.SyncSettings{Value: port.SyncTiming{Auto: false}})
	return e2eWithLinks(httpapi.Deps{
		Stores:   stores,
		IDs:      ids,
		Identity: apptest.Identity{Login: "taro"},
		Inbox:    &apptest.Inbox{},
		StoreList: apptest.StoreList{Stores: []port.StoreInfo{
			{ID: e2eStore, Name: "e2e", Color: "sky"},
			{ID: readonlyStore, Name: "readonly", Color: "orange", ReadOnly: true},
			{ID: teamStore, Name: "team", Color: "green"},
		}},
		Accounts:    accounts,
		Checks:      apptest.NewChecks(),
		GitHubReads: state.GitHubReads(),
		PRScans:     state.PRScans(),
		AppMissing:  state.AppMissing(),
		Secrets:     &apptest.Secrets{Tokens: e2eTokens},
		GitHub:      e2eGitHub,
		DeviceFlow:  e2eDeviceFlow,
		Hosts:       apptest.Hosts{ID: "e2e-client"},
		StoreGitHub: &apptest.StoreGitHub{Login: "taro", Stores: []port.StoreGitHubInfo{
			{ID: e2eStore, Name: "e2e", Remote: true, Account: e2eAccount},
			{ID: readonlyStore, Name: "readonly", Account: e2eAccount, ReadOnly: true},
		}},
		Clock:       apptest.NewClock(time.Now()),
		Sync:        syncSvc,
		Auto:        auto,
		Events:      hub,
		Delivery:    syncSvc,
		SyncTargets: syncSvc.Targets,
	}), nil
}

// e2eStores は、e2e と readonly の Store にサンプル(e2e にはワークスペース Mobile も)、team の Store に Team の
// サンプルを入れる。
func e2eStores(ctx context.Context, ids port.IDs) (*apptest.Stores, error) {
	stores := apptest.NewStores(apptest.NewClock(time.Now()), ids)
	for _, id := range []port.StoreID{e2eStore, readonlyStore, teamStore} {
		stores.AddStore(id)
	}
	for _, f := range []func() error{
		func() error { return apptest.Sample(ctx, stores, e2eStore) },
		func() error { return apptest.SampleMobile(ctx, stores, e2eStore) },
		func() error { return apptest.Sample(ctx, stores, readonlyStore) },
		func() error { return apptest.SampleTeam(ctx, stores, teamStore) },
	} {
		if err := f(); err != nil {
			return nil, err
		}
	}
	return stores, nil
}

// e2eSync は、e2e と readonly の Store を偽の remote と同期する(手での同期の確かめに使う)。team の Store は remote が
// ない。移動先への保存(Deliver)は stores に書く(remote があれば偽の Replicator が stores に書く)。stores は、ほかの機能と
// 同じく NotifyWrites で包まない(e2e のビルドは自動の同期を動かさず、画面への知らせは Watch が出す)。
func e2eSync(hub *events.Hub, stores *apptest.Stores) *syncing.Service {
	return &syncing.Service{
		Targets: &apptest.SyncTargets{Targets: []port.SyncTarget{
			{ID: e2eStore, Name: "e2e", Remote: "https://example.invalid/e2e.git", Account: e2eAccount},
			{ID: readonlyStore, Name: "readonly", Remote: "https://example.invalid/readonly.git", Account: e2eAccount, ReadOnly: true},
			{ID: teamStore, Name: "team"},
		}},
		Tokens:     &apptest.SyncTokens{Tokens: map[port.StoreID]string{e2eStore: "e2e-gh-token", readonlyStore: "e2e-gh-token"}},
		Replicator: &apptest.Replicator{Stores: stores},
		Stores:     stores,
		State:      apptest.NewSyncState(),
		Events:     hub,
		Clock:      apptest.NewClock(time.Now()),
	}
}

// e2eWithLinks は、d に、API と共用する Session と、その Session のトークンの入口で組んだリンクの操作を足す。
func e2eWithLinks(d httpapi.Deps) httpapi.Deps {
	d.GitHubSession = ghapp.NewSession()
	d.Links = linksOf(d, ghapp.Connector{
		Session: d.GitHubSession, Accounts: d.Accounts, Secrets: d.Secrets, GitHub: d.GitHub, Device: d.DeviceFlow,
		Hosts: d.Hosts, Stores: d.StoreGitHub, Clock: d.Clock,
	})
	return d
}

// checkLoopback は、addr の host が 127.0.0.1・::1・localhost でなければエラーを返す。
func checkLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("HEDGEHUG_ADDR %q: %w", addr, err)
	}
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return nil
	}
	return fmt.Errorf("HEDGEHUG_ADDR %q: 待ち受けは 127.0.0.1・[::1]・localhost に限る", addr)
}

// tokenAndAddr は、テストが決めたトークン(HEDGEHUG_TOKEN)と待ち受け(HEDGEHUG_ADDR)を返す。
func tokenAndAddr() (token, addr string) {
	return os.Getenv("HEDGEHUG_TOKEN"), os.Getenv("HEDGEHUG_ADDR")
}

// e2eReads は、GitHub の記録を持つ state.db を、使い捨ての場所に開く。止めるときに閉じて消す。
func e2eReads(ctx context.Context) (*statedb.DB, error) {
	dir, err := os.MkdirTemp("", "hedgehug-e2e-state-")
	if err != nil {
		return nil, err
	}
	db, err := statedb.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	context.AfterFunc(ctx, func() {
		_ = db.Close()
		_ = os.RemoveAll(dir)
	})
	return db, nil
}
