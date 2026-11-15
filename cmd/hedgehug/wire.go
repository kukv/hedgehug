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
	appinbox "github.com/kukv/hedgehug/internal/app/inbox"
	"github.com/kukv/hedgehug/internal/app/noticing"
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
// Store の一覧は読まない(この計画の決定 6、C2 の決定 C2-2)。2 つ目の戻り値は、wrap に渡す判定の記録(履歴が読むのと同じ
// state.db。本物のビルドの wrap は使わない)。
func wire(ctx context.Context) (httpapi.Deps, port.JudgedSaves, error) {
	clock, ids := sysclock.Clock{}, sysclock.IDs{}
	stores := gitstore.New(clock, ids)
	cfg, err := configPath()
	if err != nil {
		return httpapi.Deps{}, nil, err
	}
	state, err := statePath()
	if err != nil {
		return httpapi.Deps{}, nil, err
	}
	db, err := statedb.Open(state)
	if err != nil {
		return httpapi.Deps{}, nil, err
	}
	file := machineconfig.Open(cfg)
	hub := events.NewHub()
	skew := &syncing.Skew{Records: db.ClockSkews(), Events: hub}
	gh := github.New(skew.Observe) // 時計のずれは syncing.Skew が受け取る(C5d-3)
	secrets, session := secret.New(), ghapp.NewSession()
	conn := ghapp.Connector{
		Session: session, Accounts: file.Accounts(), Secrets: secrets, GitHub: gh, Device: gh,
		Hosts: file.Hosts(), Stores: noStoreGitHub{}, Clock: clock,
	}
	rep := newReplicator(stores, db, clock, hub, skew)
	svc := &syncing.Service{
		Targets: noSyncTargets{}, Tokens: conn,
		Replicator: rep, State: db.SyncState(), Log: db.SyncLog(), Events: hub, Clock: clock,
	}
	auto := syncing.NewAuto(svc, file.SyncSettings())
	notified := syncing.NotifyWrites(stores, auto.Wrote)
	// 移動先への保存(Deliver)も、ほかの保存と同じく自動の同期と画面に知らせる
	svc.Stores = notified
	context.AfterFunc(ctx, hub.Close) // 止めるとき SSE の流れを終える(決定 C2-27)
	go auto.Run(ctx)
	go (&syncing.Watch{Versions: stores, Events: hub}).Run(ctx, 500*time.Millisecond)
	// state.db は閉じない。プロセスが終わるときに閉じる。サーバーを止める 5 秒のあいだも、動いている同期が記録できるようにするため。
	deps := httpapi.Deps{
		Stores:        notified,
		IDs:           ids,
		Identity:      noIdentity{},
		Inbox:         db.Inbox(clock),
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
		Delivery:      svc,
		SyncTargets:   svc.Targets,
	}
	// state.db に置くもの(レポートの選択・画面の状態・最近開いたもの・時計のずれ・保存したビュー・同期の行・同期で受け取った保存)と、コミットと版の印の読み口
	deps.Reports, deps.Screens, deps.Recent, deps.ClockSkew, deps.Views = db.ReportSelections(), db.ScreenStates(), db.Recent(), skew, db.Views()
	deps.SyncLog, deps.Commits, deps.Versions, deps.Received = db.SyncLog(), stores, stores, db
	deps.Links = linksOf(deps, conn) // トークンの入口は同期と同じ Connector(D4a-2)
	go (&githublink.Reader{Links: deps.Links, Settings: file.SyncSettings(), Waiting: auto.Waiting}).Run(ctx)
	return deps, db.JudgedSaves(), nil
}

// newReplicator は、合わせたあと、送る前に修復し(C3a の決定 C3a-3)、同期の最後に受け取った保存から通知を作る
// (決定 C4b1-2)同期。「あなた」と表示中の Store は G までないので、本物のビルドではまだ通知は出ない(C4b1-15)。
// git の通信の応答で測った時計のずれは skew に渡す(C5d-3)。
func newReplicator(stores *gitstore.Stores, db *statedb.DB, clock port.Clock, events port.Events, skew *syncing.Skew) *gitstore.Replicator {
	rep := gitstore.NewReplicator(stores)
	rep.Repair = repairing.Run
	rep.OnSkew = skew.Observe
	notices := noticing.Service{Inbox: appinbox.Notifier{Inbox: db.Inbox(clock), Events: events}, Identity: noIdentity{}, List: noStores{}}
	rep.Judge, rep.Judged = notices.Judge, db.JudgedSaves()
	return rep
}

// noStores は、表示する Store を返さない。
type noStores struct{}

func (noStores) List(context.Context) ([]port.StoreInfo, error) { return []port.StoreInfo{}, nil }

// noIdentity は、「あなた」を決めない。保存する操作は断る。
type noIdentity struct{}

func (noIdentity) Me(context.Context, port.StoreID) (string, error) {
	return "", errors.New("このマシンのログイン名が未設定です")
}

// tokenAndAddr は、起動ごとのトークンと、127.0.0.1 の空いた番号を返す。
func tokenAndAddr() (token, addr string) { return newToken(), "127.0.0.1:0" }

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read は失敗しない(Go 1.24 から)
	return hex.EncodeToString(b)
}

// wrap は、本物のビルドでは何も足さない(e2e のビルドだけが、テストの入口を足す)。
func wrap(_ string, _ httpapi.Deps, _ port.JudgedSaves, h http.Handler) http.Handler { return h }
