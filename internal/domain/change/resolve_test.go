package change_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func at(minute int) time.Time { return time.Date(2026, 11, 12, 10, minute, 0, 0, jst) }

func rec(id string, ts time.Time, by string, d change.Draft, prev ...change.ID) change.Record {
	return change.Record{
		ID: change.ID(id), Op: change.ID(id), At: ts, By: by, Actor: change.User(by),
		T: d.T, F: d.F, From: d.From, To: d.To, Prev: prev,
	}
}

func create(to any) change.Record { return rec("c", at(0), "taro", change.NewCreate(to)) }

var opts = change.Options{Multi: func(f string) bool { return f == "systems" }}

func value(t *testing.T, s change.State, f string) string {
	t.Helper()
	return string(s.Values[f])
}

func TestResolveCreateOnly(t *testing.T) {
	s := change.Resolve([]change.Record{create(map[string]any{"title": "a", "systems": []string{"Y1"}})}, opts)
	if s.Created == nil || value(t, s, "title") != `"a"` {
		t.Fatalf("state %+v", s)
	}
	if got := list(s, "systems"); got != `"Y1"` {
		t.Errorf("systems = %s", got)
	}
	if _, ok := s.Values["systems"]; ok {
		t.Error("multi field in Values")
	}
}

func TestResolveLaterSeenWins(t *testing.T) {
	// A と B が同時に書き(B の時計があと)、C が両方を見て書いた。C の時計が最も遅れていても C が勝つ。
	log := []change.Record{
		create(map[string]any{"title": "a"}),
		rec("A", at(10), "ann", change.NewSet("title", "a", "A"), "c"),
		rec("B", at(11), "bob", change.NewSet("title", "a", "B"), "c"),
		rec("C", at(1), "carl", change.NewSet("title", "B", "C"), "A", "B"),
	}
	if got := value(t, change.Resolve(log, opts), "title"); got != `"C"` {
		t.Errorf("title = %s, want C", got)
	}
}

func TestResolveConcurrentTieBreak(t *testing.T) {
	cases := []struct {
		name string
		a, b change.Record
		want string
	}{
		{"later at wins", rec("A", at(10), "zed", change.NewSet("title", "a", "A"), "c"), rec("B", at(11), "amy", change.NewSet("title", "a", "B"), "c"), `"B"`},
		{"same at: by lowercased", rec("A", at(10), "Bob", change.NewSet("title", "a", "A"), "c"), rec("B", at(10), "alice", change.NewSet("title", "a", "B"), "c"), `"A"`},
		{"same at and by: id", rec("A", at(10), "bob", change.NewSet("title", "a", "A"), "c"), rec("B", at(10), "BOB", change.NewSet("title", "a", "B"), "c"), `"B"`},
		{"at compared in UTC", rec("A", at(10).In(time.UTC), "a", change.NewSet("title", "a", "A"), "c"), rec("B", at(9), "a", change.NewSet("title", "a", "B"), "c"), `"A"`},
	}
	for _, c := range cases {
		log := []change.Record{create(map[string]any{"title": "a"}), c.a, c.b}
		if got := value(t, change.Resolve(log, opts), "title"); got != c.want {
			t.Errorf("%s: title = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestResolveInvalidRowIsIgnored(t *testing.T) {
	// 手順 0: 検査に合わない行は無いものとし、前の値に戻る。
	log := []change.Record{
		create(map[string]any{"prefix": "HH"}),
		rec("A", at(1), "taro", change.NewSet("prefix", "HH", "WEB"), "c"),
		rec("B", at(2), "taro", change.NewSet("prefix", "WEB", "web!"), "A"),
	}
	o := change.Options{Valid: func(r change.Record) bool { return r.F != "prefix" || string(r.To) != `"web!"` }}
	if got := value(t, change.Resolve(log, o), "prefix"); got != `"WEB"` {
		t.Errorf("prefix = %s, want WEB", got)
	}
}

func TestResolveAddRemovePerValue(t *testing.T) {
	// 一方が Y1 を取り除き、他方が同時に Y1 を足し直した。値ごとの後勝ちで、あとの add が勝つ。
	log := []change.Record{
		create(map[string]any{"systems": []string{"Y1", "Y2"}}),
		rec("A", at(1), "ann", change.NewRemove("systems", "Y1"), "c"),
		rec("B", at(2), "bob", change.NewAdd("systems", "Y1"), "c"),
		rec("C", at(3), "ann", change.NewRemove("systems", "Y2"), "c"),
		rec("D", at(3), "ann", change.NewAdd("systems", "Y3"), "c"),
	}
	if got := list(change.Resolve(log, opts), "systems"); got != `"Y1","Y3"` {
		t.Errorf("systems = %s", got)
	}
}

func TestStamp(t *testing.T) {
	log := []change.Record{create(map[string]any{"title": "a"})}
	n := 0
	newID := func() change.ID { n++; return change.ID(fmt.Sprintf("n%d", n)) }
	meta := change.Meta{Op: "op1", At: at(5), Actor: change.Actor{Kind: change.ActorSync}}
	got := change.Stamp(log, []change.Draft{
		change.NewSet("title", "a", "b"),
		change.NewSet("title", "b", "c"),
		change.NewAdd("systems", "Y1"),
	}, meta, newID)
	want := []struct{ id, prev string }{{"n1", "[c]"}, {"n2", "[n1]"}, {"n3", "[c]"}}
	for i, w := range want {
		r := got[i]
		if string(r.ID) != w.id || fmt.Sprint(r.Prev) != w.prev || r.Op != "op1" || !r.At.Equal(at(5)) || r.By != change.BotLogin || r.Actor.Kind != change.ActorSync {
			t.Errorf("row %d = %+v", i, r)
		}
	}
	created := change.Stamp(nil, []change.Draft{change.NewCreate(map[string]any{"id": "x"})}, meta, newID)
	if len(created[0].Prev) != 0 {
		t.Errorf("create prev = %v", created[0].Prev)
	}
}

func TestStampPrevListsAllTips(t *testing.T) {
	log := []change.Record{
		create(map[string]any{"title": "a"}),
		rec("A", at(1), "ann", change.NewSet("title", "a", "A"), "c"),
		rec("B", at(2), "bob", change.NewSet("title", "a", "B"), "c"),
	}
	got := change.Stamp(log, []change.Draft{change.NewSet("title", "B", "C")}, change.Meta{Op: "o", At: at(3), Actor: change.User("c")}, func() change.ID { return "n" })
	if fmt.Sprint(got[0].Prev) != "[A B]" {
		t.Errorf("prev = %v, want both tips", got[0].Prev)
	}
}

func list(s change.State, f string) string {
	var parts []string
	for _, v := range s.Lists[f] {
		parts = append(parts, string(v))
	}
	return strings.Join(parts, ",")
}
