package itemlink_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// stamped は、作成の行の後ろに、操作ごとの下書きを change.Stamp で書き足したログ。
type stamped struct {
	log []change.Record
	n   int
}

func newStamped() *stamped {
	c := change.Record{ID: "c0", Op: "c0", At: base, By: "taro", Actor: change.User("taro"), T: change.Create, To: []byte(`{}`)}
	return &stamped{log: []change.Record{c}}
}

func (s *stamped) write(actor change.Actor, ds []change.Draft) {
	s.n++
	meta := change.Meta{Op: change.ID(fmt.Sprintf("op%d", s.n)), At: base.Add(time.Duration(s.n) * time.Hour), Actor: actor}
	s.log = append(s.log, change.Stamp(s.log, ds, meta, func() change.ID {
		s.n++
		return change.ID(fmt.Sprintf("r%02d", s.n))
	})...)
}

func (s *stamped) addOf(r itemlink.Ref) change.Record {
	for _, x := range s.log {
		if x.T == change.Add && x.F == "links" && string(x.To) == refJSON(r) {
			return x
		}
	}
	return change.Record{}
}

func TestOriginFollowsRenames(t *testing.T) {
	v := itemlink.Values{Title: "t", State: itemlink.Open}
	for _, kind := range []itemlink.Kind{itemlink.Issue, itemlink.PR} {
		r0 := itemlink.Ref{Kind: kind, Repo: "Kukv/Web", Number: 5}
		r1 := itemlink.Ref{Kind: kind, Repo: "kukv/web2", Number: 5}
		r2 := itemlink.Ref{Kind: kind, Repo: "kukv/web3", Number: 5}
		s := newStamped()
		s.write(change.User("taro"), itemlink.FromLog(s.log).Attach(r0, v))
		for _, to := range []itemlink.Ref{r1, r2} {
			set := itemlink.FromLog(s.log)
			from := r0
			if to == r2 {
				from = r1
			}
			l, ok := set.Find(from)
			if !ok {
				t.Fatalf("%s: %v is not linked", kind, from)
			}
			s.write(change.Actor{Kind: change.ActorGitHub}, set.Rename(l, to.Repo, v))
		}
		first, last := s.addOf(r0), s.addOf(r2)
		if got := itemlink.Origin(s.log, last); got.ID != first.ID || got.By != "taro" {
			t.Errorf("%s: Origin = %v by %q; want %v by taro", kind, got.ID, got.By, first.ID)
		}
		if got := itemlink.Origin(s.log, first); got.ID != first.ID {
			t.Errorf("%s: a plain add must be returned as is, got %v", kind, got.ID)
		}
	}
}

func TestOriginStopsOnACycle(t *testing.T) {
	a := itemlink.Ref{Kind: itemlink.Issue, Repo: "o/a", Number: 1}
	b := itemlink.Ref{Kind: itemlink.Issue, Repo: "o/b", Number: 1}
	mk := func(id, op string, d change.Draft, prev ...change.ID) change.Record {
		return change.Record{ID: change.ID(id), Op: change.ID(op), At: base, T: d.T, F: d.F, From: d.From, To: d.To, Prev: prev}
	}
	// a1 は a→b の改名、b1 は b→a の改名で、互いの remove の prev が相手の add を指す。
	log := []change.Record{
		mk("ra", "o1", change.NewRemove("links", a), "b1"), mk("a1", "o1", change.NewAdd("links", b), "ra"),
		mk("rb", "o2", change.NewRemove("links", b), "a1"), mk("b1", "o2", change.NewAdd("links", a), "rb"),
	}
	if got := itemlink.Origin(log, log[1]); got.ID != "a1" && got.ID != "b1" {
		t.Errorf("Origin = %v; want a row of the cycle", got.ID)
	}
}

func TestOriginKeepsTheRowWhenTheOldAddIsMissing(t *testing.T) {
	r1 := itemlink.Ref{Kind: itemlink.Issue, Repo: "kukv/web2", Number: 5}
	r0 := itemlink.Ref{Kind: itemlink.Issue, Repo: "kukv/web", Number: 5}
	s := newStamped()
	set := itemlink.FromLog(s.log)
	s.write(change.Actor{Kind: change.ActorGitHub}, set.Rename(itemlink.Link{Ref: r0}, r1.Repo, itemlink.Values{}))
	add := s.addOf(r1)
	if got := itemlink.Origin(s.log, add); got.ID != add.ID {
		t.Errorf("Origin = %v; want %v", got.ID, add.ID)
	}
}
