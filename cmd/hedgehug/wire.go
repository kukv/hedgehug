//go:build !e2e

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/events"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/githublink"
	appinbox "github.com/kukv/hedgehug/internal/app/inbox"
	"github.com/kukv/hedgehug/internal/app/members"
	"github.com/kukv/hedgehug/internal/app/noticing"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/repairing"
	appstores "github.com/kukv/hedgehug/internal/app/stores"
	"github.com/kukv/hedgehug/internal/app/syncing"
	"github.com/kukv/hedgehug/internal/infra/github"
	"github.com/kukv/hedgehug/internal/infra/gitstore"
	"github.com/kukv/hedgehug/internal/infra/machineconfig"
	"github.com/kukv/hedgehug/internal/infra/secret"
	"github.com/kukv/hedgehug/internal/infra/statedb"
	"github.com/kukv/hedgehug/internal/infra/sysclock"
)

// wire は、config.yaml の Store を開いて本物の部品を口に差し込み、自動の同期と GitHub の自動の読み取りと Store の見張りを
// ctx が終わるまで動かす。2 つ目の戻り値は、wrap に渡す判定の記録(履歴が読むのと同じ state.db。本物のビルドの wrap は使わない)。
func wire(ctx context.Context) (httpapi.Deps, port.JudgedSaves, error) {
	return wireRunning(ctx, &sync.WaitGroup{})
}

// wireRunning は wire の本体。動かした goroutine を running に積む(テストが、止めたあと Store と state.db に
// 触れなくなるまで待つため)。
func wireRunning(ctx context.Context, running *sync.WaitGroup) (httpapi.Deps, port.JudgedSaves, error) {
	clock, ids := sysclock.Clock{}, sysclock.IDs{}
	stores := gitstore.New(clock, ids)
	file, view, err := openStores(stores)
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
	identity, hub := appstores.Identity{GitHub: view.StoreGitHub(), Accounts: file.Accounts()}, events.NewHub()
	skew := &syncing.Skew{Records: db.ClockSkews(), Events: hub}
	gh := github.New(skew.Observe) // 時計のずれは syncing.Skew が受け取る(C5d-3)
	secrets, session := secret.New(), ghapp.NewSession()
	conn := ghapp.Connector{
		Session: session, Accounts: file.Accounts(), Secrets: secrets, GitHub: gh, Device: gh,
		Hosts: file.Hosts(), Stores: view.StoreGitHub(), Clock: clock,
	}
	rep := newReplicator(stores, db, clock, hub, skew, noticing.Service{Identity: identity, List: view.StoreList()})
	svc := &syncing.Service{
		Targets: view.SyncTargets(), Tokens: conn,
		Replicator: rep, State: db.SyncState(), Log: db.SyncLog(), Events: hub, Clock: clock,
	}
	auto := syncing.NewAuto(svc, file.SyncSettings())
	// 人の書き込みでは「あなた」をメンバーに足す(G1-6)。移動先への保存(Deliver)も、ほかの保存と同じく自動の同期と画面に知らせる
	notified := syncing.NotifyWrites(members.JoinOnWrite(stores, view.StoreGitHub()), auto.Wrote)
	svc.Stores = notified
	context.AfterFunc(ctx, hub.Close) // 止めるとき SSE の流れを終える(決定 C2-27)
	running.Go(func() { auto.Run(ctx) })
	running.Go(func() { (&syncing.Watch{Versions: stores, Events: hub}).Run(ctx, 500*time.Millisecond) })
	// state.db は閉じない。プロセスが終わるときに閉じる。サーバーを止める 5 秒のあいだも、動いている同期が記録できるようにするため。
	deps := httpapi.Deps{
		Stores:        notified,
		IDs:           ids,
		Identity:      identity,
		Inbox:         db.Inbox(clock),
		StoreList:     view.StoreList(),
		Accounts:      file.Accounts(),
		Secrets:       secrets,
		GitHub:        gh,
		DeviceFlow:    gh,
		Hosts:         file.Hosts(),
		StoreGitHub:   view.StoreGitHub(),
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
	// state.db に置くもの(レポートの選択・画面の状態・最近開いたもの・時計のずれ・同期の行・同期で受け取った保存)と、コミットと版の印の読み口
	deps.Reports, deps.Screens, deps.Recent, deps.ClockSkew = db.ReportSelections(), db.ScreenStates(), db.Recent(), skew
	deps.SyncLog, deps.Commits, deps.Versions, deps.Received = db.SyncLog(), stores, stores, db
	deps.Links = linksOf(deps, conn) // トークンの入口は同期と同じ Connector(D4a-2)
	reader := &githublink.Reader{Links: deps.Links, Settings: file.SyncSettings(), Waiting: auto.Waiting}
	running.Go(func() { reader.Run(ctx) })
	return deps, db.JudgedSaves(), nil
}

// newReplicator は、合わせたあと、送る前に修復し(C3a の決定 C3a-3)、同期の最後に受け取った保存から通知を作る
// (決定 C4b1-2)同期。notices は「あなた」と表示中の Store を決める判定で、受信箱はここで足す。
// git の通信の応答で測った時計のずれは skew に渡す(C5d-3)。
func newReplicator(stores *gitstore.Stores, db *statedb.DB, clock port.Clock, events port.Events, skew *syncing.Skew, notices noticing.Service) *gitstore.Replicator {
	rep := gitstore.NewReplicator(stores)
	rep.Repair = repairing.Run
	rep.OnSkew = skew.Observe
	notices.Inbox = appinbox.Notifier{Inbox: db.Inbox(clock), Events: events}
	rep.Judge, rep.Judged = notices.Judge, db.JudgedSaves()
	return rep
}

// openStores は、config.yaml の stores の行の Store を開き、config.yaml と、開けた Store だけを見せる StoresView を返す。id のない行・
// 同じ id の行がほかにもある行・path を開けない行・store.yaml の Store ID が id と違う行は、警告を出して開かない(G1-2)。
// 開く前に別の gitstore.Stores で ID を確かめる(違う ID で開くと、その ID の Store のフォルダを差し替えてしまう)。
// config.yaml を読めないときも、警告を出して Store を 1 つも開かずに起動を続ける(G1-2「読めない行で起動を止めない」)。
func openStores(stores *gitstore.Stores) (*machineconfig.File, machineconfig.StoresView, error) {
	cfg, err := configPath()
	if err != nil {
		return nil, machineconfig.StoresView{}, err
	}
	file := machineconfig.Open(cfg)
	entries, err := file.StoreEntries()
	if err != nil {
		slog.Warn("config.yaml の stores を読めないので、Store を開きません", "path", cfg, "error", err)
		return file, file.Stores(func(port.StoreID) bool { return false }), nil
	}
	rows := map[port.StoreID]int{}
	for _, e := range entries {
		rows[e.ID]++
	}
	probe, opened := gitstore.New(sysclock.Clock{}, sysclock.IDs{}), map[port.StoreID]bool{}
	for _, e := range entries {
		if reason := unopenable(probe, e, rows[e.ID]); reason != "" {
			slog.Warn("config.yaml の Store を開きません: "+reason, "id", e.ID, "path", e.Path)
			continue
		}
		if _, err := stores.Attach(e.Path); err != nil {
			slog.Warn("config.yaml の Store を開けません", "id", e.ID, "path", e.Path, "error", err)
			continue
		}
		opened[e.ID] = true
	}
	return file, file.Stores(func(id port.StoreID) bool { return opened[id] }), nil
}

// unopenable は、e を開かない理由。開けるなら空。rows は e と同じ id の行の数。
func unopenable(probe *gitstore.Stores, e machineconfig.StoreEntry, rows int) string {
	switch {
	case e.ID == "":
		return "id がありません(前回の版の形)"
	case rows > 1:
		return "同じ id の行がほかにもあります"
	case e.Path == "":
		return "path がありません"
	}
	id, err := probe.Attach(e.Path)
	switch {
	case err != nil:
		return "path を開けません(" + err.Error() + ")"
	case id != e.ID:
		return "store.yaml の Store ID が " + string(id) + " です"
	}
	return ""
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
