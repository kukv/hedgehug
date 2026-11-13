package itemlink_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// opLog は、ops の各保存を 1 秒ずつずらした時刻で書いたログ(同じ保存の行は同じ op・時刻)。
func opLog(ops ...[]change.Draft) []change.Record {
	n := 0
	newID := func() change.ID { n++; return change.ID(fmt.Sprintf("R%03d", n)) }
	log := change.Stamp(nil, []change.Draft{change.NewCreate(map[string]any{})}, change.Meta{Op: "O0", At: base}, newID)
	for i, ds := range ops {
		meta := change.Meta{Op: change.ID(fmt.Sprintf("O%d", i+1)), At: base.Add(time.Duration(i+1) * time.Second)}
		log = append(log, change.Stamp(log, ds, meta, newID)...)
	}
	return log
}

func refsOf(s itemlink.Set) []itemlink.Ref {
	out := make([]itemlink.Ref, len(s.Links))
	for i, l := range s.Links {
		out[i] = l.Ref
	}
	return out
}

func TestFromLogKeepsTheOrderThroughRenames(t *testing.T) {
	web2 := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web2", Number: 41}
	web3 := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web3", Number: 41}
	s := itemlink.FromLog(opLog(
		[]change.Draft{change.NewAdd("links", pr41)},
		[]change.Draft{change.NewAdd("links", pr40)},
		[]change.Draft{change.NewRemove("links", pr41), change.NewAdd("links", web2)},
		[]change.Draft{change.NewRemove("links", web2), change.NewAdd("links", web3)},
	))
	if got := refsOf(s); !slices.Equal(got, []itemlink.Ref{web3, pr40}) {
		t.Errorf("Links = %+v, want the renamed link to keep its place before 40", got)
	}
}

func TestFromLogOrderDoesNotDependOnTheLineOrder(t *testing.T) {
	log := opLog(
		[]change.Draft{change.NewAdd("links", iss12)},
		[]change.Draft{change.NewAdd("links", pr41)},
		[]change.Draft{change.NewAdd("links", pr40)},
	)
	want := refsOf(itemlink.FromLog(log))
	shuffled := slices.Clone(log)
	slices.Reverse(shuffled[1:])
	if got := refsOf(itemlink.FromLog(shuffled)); !slices.Equal(got, want) || !slices.Equal(want, []itemlink.Ref{iss12, pr41, pr40}) {
		t.Errorf("shuffled = %+v, ordered = %+v, want issue, 41, 40", got, want)
	}
}

func TestFromLogIgnoresMalformedLinks(t *testing.T) {
	raw := func(v string) change.Draft {
		d := change.NewAdd("links", pr41)
		d.To = json.RawMessage(v)
		return d
	}
	s := itemlink.FromLog(logOf(
		raw(`{"kind":"mr","repo":"org/web","number":1}`),
		raw(`{"kind":"pr","repo":"../../user","number":2}`),
		raw(`{"kind":"pr","repo":" org/web","number":3}`),
		raw(`{"kind":"issue","repo":"org/web","number":0}`),
		change.NewAdd("links", pr41),
		change.NewAdd("links_off", itemlink.Ref{Kind: "mr", Repo: "org/web", Number: 5}),
	))
	if got := refsOf(s); !slices.Equal(got, []itemlink.Ref{pr41}) || len(s.Off) != 0 {
		t.Errorf("Links = %+v, Off = %+v, want only 41", got, s.Off)
	}
}

func TestFromLogTreatsUnknownValuesAsEmpty(t *testing.T) {
	s := itemlink.FromLog(logOf(
		change.NewAdd("links", pr41),
		change.NewSet(itemlink.ValueField(pr41, "title"), nil, "A"),
		change.NewSet(itemlink.ValueField(pr41, "state"), nil, "weird"),
		change.NewSet(itemlink.ValueField(pr41, "review"), nil, "odd"),
	))
	if got := s.Links[0].Values; got != (itemlink.Values{Title: "A", Review: itemlink.NoReview}) {
		t.Errorf("Values = %+v, want an empty state and no review", got)
	}
}
