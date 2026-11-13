//go:build !e2e

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/events"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/githublink"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/repairing"
	"github.com/kukv/hedgehug/internal/app/syncing"
	"github.com/kukv/hedgehug/internal/infra/github"
	"github.com/kukv/hedgehug/internal/infra/gitstore"
	"github.com/kukv/hedgehug/internal/infra/machineconfig"
	"github.com/kukv/hedgehug/internal/infra/secret"
	"github.com/kukv/hedgehug/internal/infra/statedb"
	"github.com/kukv/hedgehug/internal/infra/sysclock"
)

// wire は、本物の部品を口に差し込み、自動の同期と GitHub の自動の読み取りと Store の見張りを ctx が終わるまで動かす。
// Store の一覧は読まない(この計画の決定 6、C2 の決定 C2-2)。
func wire(ctx context.Context) (httpapi.Deps, error) {
	clock, ids := sysclock.Clock{}, sysclock.IDs{}
	stores := gitstore.New(clock, ids)
	cfg, err := configPath()
	if err != nil {
		return httpapi.Deps{}, err
	}
	state, err := statePath()
	if err != nil {
		return httpapi.Deps{}, err
	}
	db, err := statedb.Open(state)
	if err != nil {
		return httpapi.Deps{}, err
	}
	file := machineconfig.Open(cfg)
	gh := github.New(nil) // 時計のずれは受け取らない(トラック D の計画の決定 3)
	secrets := secret.New()
	session := ghapp.NewSession()
	hub := events.NewHub()
	conn := ghapp.Connector{
		Session: session, Accounts: file.Accounts(), Secrets: secrets, GitHub: gh, Device: gh,
		Hosts: file.Hosts(), Stores: noStoreGitHub{}, Clock: clock,
	}
	rep := gitstore.NewReplicator(stores)
	rep.Repair = repairing.Run // 合わせたあと、送る前の修復(C3a の決定 C3a-3)
	svc := &syncing.Service{
		Targets: noSyncTargets{}, Tokens: conn,
		Replicator: rep, State: db.SyncState(), Events: hub, Clock: clock,
	}
	auto := syncing.NewAuto(svc, file.SyncSettings())
	context.AfterFunc(ctx, hub.Close) // 止めるとき SSE の流れを終える(決定 C2-27)
	go auto.Run(ctx)
	go (&syncing.Watch{Versions: stores, Events: hub}).Run(ctx, 500*time.Millisecond)
	// state.db は閉じない。プロセスが終わるときに閉じる。サーバーを止める 5 秒のあいだも、動いている同期が記録できるようにするため。
	deps := httpapi.Deps{
		Stores:        syncing.NotifyWrites(stores, auto.Wrote),
		IDs:           ids,
		Identity:      noIdentity{},
		Inbox:         noInbox{},
		StoreList:     noStores{},
		Accounts:      file.Accounts(),
		Secrets:       secrets,
		GitHub:        gh,
		DeviceFlow:    gh,
		Hosts:         file.Hosts(),
		StoreGitHub:   noStoreGitHub{},
		Checks:        db.Checks(),
		GitHubReads:   db.GitHubReads(),
		PRScans:       db.PRScans(),
		AppMissing:    db.AppMissing(),
		Clock:         clock,
		Sync:          svc,
		Auto:          auto,
		Events:        hub,
		GitHubSession: session,
	}
	deps.Links = linksOf(deps, conn) // トークンの入口は同期と同じ Connector(D4a-2)
	go (&githublink.Reader{Links: deps.Links, Settings: file.SyncSettings(), Waiting: auto.Waiting}).Run(ctx)
	return deps, nil
}

// noStores は、表示する Store を返さない。
type noStores struct{}

func (noStores) List(context.Context) ([]port.StoreInfo, error) { return []port.StoreInfo{}, nil }

// noIdentity は、「あなた」を決めない。保存する操作は断る。
type noIdentity struct{}

func (noIdentity) Me(context.Context, port.StoreID) (string, error) {
	return "", errors.New("このマシンのログイン名が未設定です")
}

// noInbox は、通知を捨てる(B4-12)。
type noInbox struct{}

func (noInbox) Add(context.Context, port.Notice) error { return nil }

// tokenAndAddr は、起動ごとのトークンと、127.0.0.1 の空いた番号を返す。
func tokenAndAddr() (token, addr string) { return newToken(), "127.0.0.1:0" }

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read は失敗しない(Go 1.24 から)
	return hex.EncodeToString(b)
}

// wrap は、本物のビルドでは何も足さない(e2e のビルドだけが、テストの入口を足す)。
func wrap(_ string, _ httpapi.Deps, h http.Handler) http.Handler { return h }
