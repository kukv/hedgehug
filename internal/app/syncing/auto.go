package syncing

import (
	"context"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// holdFor は、タブの「待つ条件にある」知らせが有効な時間(決定 C2-18)。タブは続くあいだ 10 秒ごとに知らせ直す。
const holdFor = 30 * time.Second

// Auto は、自動の同期(06「自動で同期する」)。契機は、間隔、アプリを開いたとき(作ったとき)、オンライン復帰
// (Network)、書き込み後(Wrote)。見えているタブが待つ条件にあるあいだ(Hold)と、オフラインのタブがある
// あいだ(Network)は始めない。結果は捨てる(自動の同期はトーストもアクティビティも出さない。決定 C2-24)。
type Auto struct {
	Service  *Service
	Settings port.SyncSettings

	mu      sync.Mutex
	all     bool                       // 「アプリを開いたとき」「オンライン復帰」が待っている
	wrote   map[port.StoreID]time.Time // Store ごとの最後の保存の時刻
	holds   map[string]time.Time       // 待つ条件にあるタブと、知らせの期限
	offline map[string]time.Time       // オフラインのタブと、知らせの期限
}

// NewAuto は、「アプリを開いたとき」の契機を持った Auto を作る。
func NewAuto(svc *Service, settings port.SyncSettings) *Auto {
	return &Auto{
		Service: svc, Settings: settings, all: true,
		wrote: map[port.StoreID]time.Time{}, holds: map[string]time.Time{}, offline: map[string]time.Time{},
	}
}

// Network は、タブ tab がネットワークにつながっているか(online)を知らせる(決定 C2-18b)。オフラインを
// 知らせていたタブがつながったら、「オンライン復帰」の契機にする。
func (a *Auto) Network(tab string, online bool) {
	now := a.Service.Clock.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if !online {
		a.offline[tab] = now.Add(holdFor)
		return
	}
	if _, was := a.offline[tab]; was {
		delete(a.offline, tab)
		a.all = true
	}
}

// Wrote は、Store に保存したことを知らせる。
func (a *Auto) Wrote(id port.StoreID) {
	now := a.Service.Clock.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.wrote[id] = now
}

// Hold は、見えているタブ tab が、待つ条件にあるか(hold)を知らせる。
func (a *Auto) Hold(tab string, hold bool) {
	now := a.Service.Clock.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if hold {
		a.holds[tab] = now.Add(holdFor)
		return
	}
	delete(a.holds, tab)
}

// Run は、ctx が終わるまで、1 秒ごとに Step を呼ぶ。Step の失敗は、次の Step でやり直す。
func (a *Auto) Run(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		_ = a.Step(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// Step は、いま同期する Store を選び、1 つずつ同期する。
func (a *Auto) Step(ctx context.Context) error {
	timing, err := a.Settings.Timing(ctx)
	if err != nil {
		return err
	}
	now := a.Service.Clock.Now()
	all, wrote, held := a.take(now, timing.Auto)
	if held || !timing.Auto {
		return nil
	}
	picked, err := a.pick(ctx, now, all, wrote, timing)
	if err != nil {
		return err
	}
	for _, t := range picked {
		if _, err := a.Service.once(ctx, t); err != nil {
			return err
		}
		a.synced(t.ID, now)
	}
	return nil
}

// take は、待っている契機を読む。待つ条件にあるタブかオフラインのタブがあれば held。そうでなければ「アプリを開いた・
// オンライン復帰」の契機をここで使い切る。自動同期がオフなら、契機をすべて捨てる。
func (a *Auto) take(now time.Time, auto bool) (all bool, wrote map[port.StoreID]time.Time, held bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !auto {
		a.all = false
		clear(a.wrote)
		return false, nil, false
	}
	if waiting(a.holds, now) || waiting(a.offline, now) {
		return false, nil, true
	}
	all, a.all = a.all, false
	wrote = make(map[port.StoreID]time.Time, len(a.wrote))
	for id, at := range a.wrote {
		wrote[id] = at
	}
	return all, wrote, false
}

func (a *Auto) pick(ctx context.Context, now time.Time, all bool, wrote map[port.StoreID]time.Time, timing port.SyncTiming) ([]port.SyncTarget, error) {
	targets, err := a.Service.Targets.List(ctx)
	if err != nil {
		return nil, err
	}
	records, err := a.Service.State.Records(ctx)
	if err != nil {
		return nil, err
	}
	byStore := map[port.StoreID]port.SyncRecord{}
	for _, r := range records {
		byStore[r.Store] = r
	}
	var picked []port.SyncTarget
	for _, t := range targets {
		if eligible(t) && (all || due(now, byStore[t.ID], wrote[t.ID], timing)) {
			picked = append(picked, t)
		}
	}
	return picked, nil
}

// waiting は、期限の切れていない知らせが 1 つでもあるか。切れた知らせは消す。
func waiting(m map[string]time.Time, now time.Time) bool {
	held := false
	for tab, until := range m {
		if now.Before(until) {
			held = true
			continue
		}
		delete(m, tab)
	}
	return held
}

// synced は、始めた時刻 started より前の保存を、同期で送ったものとして消す(同期のあいだの保存は残す)。
func (a *Auto) synced(id port.StoreID, started time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if at, ok := a.wrote[id]; ok && !at.After(started) {
		delete(a.wrote, id)
	}
}
