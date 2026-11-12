//go:build e2e

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/events"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
)

// e2e 用のビルドの Store の ID。どちらにもサンプルデータを入れる。readonlyStore は閲覧のみの Store。
const (
	e2eStore      = "01E2ESTORE0000000000000001"
	readonlyStore = "01E2ESTORE0000000000000002"
)

// wire は、Playwright の通しのテスト用に、口に apptest の偽物とサンプルデータを差し込む(設計書 6 節)。
// トークンか待ち受けが空なら起動しない(空のトークンでは誰でも API を使え、空の待ち受けは全アドレスで待つため)。
// 待ち受けがこのマシンの中(ループバック)でなければ起動しない(固定のトークンを外に見せないため)。
func wire(ctx context.Context) (httpapi.Deps, error) {
	if os.Getenv("HEDGEHUG_TOKEN") == "" || os.Getenv("HEDGEHUG_ADDR") == "" {
		return httpapi.Deps{}, errors.New("e2e のビルドには HEDGEHUG_TOKEN と HEDGEHUG_ADDR が要る")
	}
	if err := checkLoopback(os.Getenv("HEDGEHUG_ADDR")); err != nil {
		return httpapi.Deps{}, err
	}
	ids := apptest.NewIDs()
	stores := apptest.NewStores(apptest.NewClock(time.Now()), ids)
	for _, id := range []port.StoreID{e2eStore, readonlyStore} {
		stores.AddStore(id)
		if err := apptest.Sample(ctx, stores, id); err != nil {
			return httpapi.Deps{}, err
		}
	}
	hub := events.NewHub()
	context.AfterFunc(ctx, hub.Close) // 止めるとき SSE の流れを終える(決定 C2-27)
	syncSvc := &syncing.Service{
		Targets: &apptest.SyncTargets{Targets: []port.SyncTarget{
			{ID: e2eStore, Name: "e2e", Remote: "https://example.invalid/e2e.git", Account: "e2e-account"},
			{ID: readonlyStore, Name: "readonly", Remote: "https://example.invalid/readonly.git", Account: "e2e-account", ReadOnly: true},
		}},
		Tokens:     &apptest.SyncTokens{Tokens: map[port.StoreID]string{e2eStore: "e2e-gh-token", readonlyStore: "e2e-gh-token"}},
		Replicator: &apptest.Replicator{},
		State:      apptest.NewSyncState(),
		Events:     hub,
		Clock:      apptest.NewClock(time.Now()),
	}
	auto := syncing.NewAuto(syncSvc, &apptest.SyncSettings{Value: port.SyncTiming{Auto: false}})
	return httpapi.Deps{
		Stores:   stores,
		IDs:      ids,
		Identity: apptest.Identity{Login: "taro"},
		StoreList: apptest.StoreList{Stores: []port.StoreInfo{
			{ID: e2eStore, Name: "e2e", Color: "sky"},
			{ID: readonlyStore, Name: "readonly", Color: "orange", ReadOnly: true},
		}},
		Accounts:    apptest.NewAccounts(),
		Secrets:     &apptest.Secrets{Tokens: e2eTokens},
		GitHub:      e2eGitHub,
		DeviceFlow:  e2eDeviceFlow,
		Hosts:       apptest.Hosts{ID: "e2e-client"},
		StoreGitHub: &apptest.StoreGitHub{Login: "taro", Stores: []port.StoreGitHubInfo{{ID: e2eStore, Name: "e2e", Remote: true}, {ID: readonlyStore, Name: "readonly"}}},
		Clock:       apptest.NewClock(time.Now()),
		Sync:        syncSvc,
		Auto:        auto,
		Events:      hub,
	}, nil
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
