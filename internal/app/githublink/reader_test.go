package githublink_test

import (
	"context"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/githublink"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// countingInfos は、List を呼んだ回数を数える(ReadOnce は 1 回ごとに 1 度だけ List を呼ぶ)。
type countingInfos struct {
	port.StoreGitHub
	lists int
}

func (c *countingInfos) List(ctx context.Context) ([]port.StoreGitHubInfo, error) {
	c.lists++
	return c.StoreGitHub.List(ctx)
}

type readerFixture struct {
	*readFixture
	reader   *githublink.Reader
	settings *apptest.SyncSettings
	infos    *countingInfos
	waiting  bool
}

// newReader は、自動の同期がオンで、GitHub を読む間隔が every の Reader。HH-2 に org/web#1(Open)をリンクしておく。
func newReader(t *testing.T) *readerFixture {
	t.Helper()
	f := &readerFixture{readFixture: newRead(t)}
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Open))
	f.infos = &countingInfos{StoreGitHub: f.links.StoreGH}
	f.links.StoreGH = f.infos
	f.settings = &apptest.SyncSettings{Value: port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second, GitHubEvery: every}}
	f.reader = &githublink.Reader{Links: f.links, Settings: f.settings, Waiting: func() bool { return f.waiting }}
	return f
}

func (f *readerFixture) step(t *testing.T) {
	t.Helper()
	if err := f.reader.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// 「裏で自動的に同期する」がオフなら、読まず、確かめ直しもしない(07「状態」111 行、「GitHub を読む間隔」)。
func TestReaderStopsWhenAutoOff(t *testing.T) {
	f := newReader(t)
	f.settings.Value.Auto = false
	f.step(t)
	f.advance(time.Hour)
	f.step(t)
	if f.infos.lists != 0 || f.tokens.rechecks != 0 || f.gh.ReadManyCalls("org/web") != 0 {
		t.Errorf("lists %d, rechecks %d, ReadMany %d", f.infos.lists, f.tokens.rechecks, f.gh.ReadManyCalls("org/web"))
	}
}

// 自動の同期をオンに戻したら、最初の Step で確かめ直しと読み取りを 1 回ずつ行う。
func TestReaderReadsWhenAutoTurnsOn(t *testing.T) {
	f := newReader(t)
	f.settings.Value.Auto = false
	for range 3600 {
		f.step(t)
		f.advance(time.Second)
	}
	f.settings.Value.Auto = true
	f.step(t)
	if f.infos.lists != 1 || f.tokens.rechecks != 1 || f.gh.ReadManyCalls("org/web") != 1 {
		t.Errorf("lists %d, rechecks %d, ReadMany %d", f.infos.lists, f.tokens.rechecks, f.gh.ReadManyCalls("org/web"))
	}
}

// 待つ条件のあいだは始めず、解けた次の Step で読む(D4a-8)。
func TestReaderWaits(t *testing.T) {
	f := newReader(t)
	f.waiting = true
	f.step(t)
	f.advance(time.Second)
	f.step(t)
	if f.infos.lists != 0 || f.tokens.rechecks != 0 {
		t.Fatalf("read while waiting: lists %d, rechecks %d", f.infos.lists, f.tokens.rechecks)
	}
	f.waiting = false
	f.advance(time.Second)
	f.step(t)
	if f.infos.lists != 1 || f.tokens.rechecks != 1 || f.gh.ReadManyCalls("org/web") != 1 {
		t.Errorf("lists %d, rechecks %d, ReadMany %d", f.infos.lists, f.tokens.rechecks, f.gh.ReadManyCalls("org/web"))
	}
}

// 確かめ直しは、起動の直後と、そのあと GitHub を読む間隔ごとに 1 回。戻ったら EventGitHub を流す(D4a-18)。
func TestReaderRecheckEveryInterval(t *testing.T) {
	f := newReader(t)
	f.step(t)
	if f.tokens.rechecks != 1 {
		t.Fatalf("rechecks after the first step = %d", f.tokens.rechecks)
	}
	f.advance(4 * time.Minute)
	f.step(t)
	if f.tokens.rechecks != 1 {
		t.Errorf("rechecked before the interval: %d", f.tokens.rechecks)
	}
	if hasGitHubEvent(f.events.List(), "") {
		t.Error("EventGitHub without a recovered account")
	}
	f.tokens.recovered = true
	f.advance(time.Minute)
	f.step(t)
	if f.tokens.rechecks != 2 {
		t.Errorf("rechecks at the interval = %d", f.tokens.rechecks)
	}
	if !hasGitHubEvent(f.events.List(), "") {
		t.Errorf("no EventGitHub after a recovery: %+v", f.events.List())
	}
}

func hasGitHubEvent(evs []port.Event, store port.StoreID) bool {
	for _, e := range evs {
		if e == (port.Event{Kind: port.EventGitHub, Store: store}) {
			return true
		}
	}
	return false
}

// ReadOnce には、設定の GitHub を読む間隔を渡す(D4a-7)。
func TestReaderUsesGitHubEvery(t *testing.T) {
	f := newReader(t)
	f.settings.Value.GitHubEvery = 30 * time.Minute
	f.step(t)
	f.advance(5 * time.Minute)
	f.step(t)
	if got := f.gh.ReadManyCalls("org/web"); got != 1 {
		t.Errorf("read before 30 minutes: %d", got)
	}
	f.advance(25 * time.Minute)
	f.step(t)
	if got := f.gh.ReadManyCalls("org/web"); got != 2 {
		t.Errorf("not read at 30 minutes: %d", got)
	}
}

// ReadOnce はすべての Store を読むので、1 秒ごとには呼ばず、1 分に 1 回までにする。最初の Step ではすぐ読む。
func TestReaderReadsAtMostOncePerMinute(t *testing.T) {
	f := newReader(t)
	f.step(t)
	f.advance(time.Second)
	f.step(t)
	if f.infos.lists != 1 {
		t.Fatalf("ReadOnce calls within a minute = %d", f.infos.lists)
	}
	f.advance(59 * time.Second)
	f.step(t)
	if f.infos.lists != 2 {
		t.Errorf("ReadOnce calls after a minute = %d", f.infos.lists)
	}
}

// 待つ条件で見送った読み取りは、次の 1 分を待たず、解けた最初の Step で読む(「解けたら続けて読む」)。
func TestReaderReadsRightAfterWaitingClears(t *testing.T) {
	f := newReader(t)
	f.step(t)
	f.advance(30 * time.Second)
	f.waiting = true
	f.advance(40 * time.Second)
	f.step(t)
	if f.infos.lists != 1 {
		t.Fatalf("read while waiting: %d", f.infos.lists)
	}
	f.waiting = false
	f.advance(time.Second)
	f.step(t)
	if f.infos.lists != 2 {
		t.Errorf("not read right after waiting cleared: %d", f.infos.lists)
	}
	f.advance(time.Second)
	f.step(t)
	if f.infos.lists != 2 {
		t.Errorf("read again within a minute: %d", f.infos.lists)
	}
}
