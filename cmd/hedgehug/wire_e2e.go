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
	"github.com/kukv/hedgehug/internal/app/apptest"
)

// e2eStore は、e2e 用のビルドのサンプルデータを入れた Store の ID。
const e2eStore = "01E2ESTORE0000000000000001"

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
	stores.AddStore(e2eStore)
	if err := apptest.Sample(ctx, stores, e2eStore); err != nil {
		return httpapi.Deps{}, err
	}
	return httpapi.Deps{
		Stores:   stores,
		IDs:      ids,
		Identity: apptest.Identity{Login: "taro"},
		Accounts: apptest.NewAccounts(),
		Secrets:  &apptest.Secrets{Tokens: map[string]string{"taro": "e2e-gh-token"}},
		GitHub:   &apptest.GitHub{Users: map[string]string{"e2e-gh-token": "taro"}},
		Clock:    apptest.NewClock(time.Now()),
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
