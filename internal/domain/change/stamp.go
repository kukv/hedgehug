package change

import (
	"bytes"
	"encoding/json"
	"slices"
	"time"
)

// Meta は、1 回の操作で書くすべての行に共通する値。
type Meta struct {
	Op    ID
	At    time.Time
	Actor Actor
}

// Stamp は、drafts を log の続きに書く行にする。prev は、書く時点の同じ項目の先端すべてで、
// 先に並べた drafts も含めて数える。create の prev は空。
func Stamp(log []Record, drafts []Draft, meta Meta, newID func() ID) []Record {
	all := slices.Clone(log)
	out := make([]Record, 0, len(drafts))
	for _, d := range drafts {
		r := Record{
			ID: newID(), Op: meta.Op, At: meta.At, By: meta.Actor.By(), Actor: meta.Actor,
			T: d.T, F: d.F, From: d.From, To: d.To, Root: d.Root,
		}
		if r.T != Create {
			r.Prev = Tips(all, r)
		}
		all = append(all, r)
		out = append(out, r)
	}
	return out
}

// key は、行が属する項目。set は F ごと、add・remove は F と値ごと。
func key(r Record) string {
	switch r.T {
	case Set:
		return "set\x00" + r.F
	case Add:
		return "list\x00" + r.F + "\x00" + canonical(r.To)
	case Remove:
		return "list\x00" + r.F + "\x00" + canonical(r.From)
	}
	return string(r.T) + "\x00" + r.F
}

func canonical(v json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, v); err != nil {
		return string(v)
	}
	return b.String()
}

// group は、項目 k の行と、create の行。
func group(log []Record, k string) []Record {
	var g []Record
	for _, r := range log {
		if r.T == Create || key(r) == k {
			g = append(g, r)
		}
	}
	return g
}

func tipsOf(g []Record) []Record {
	seen := map[ID]bool{}
	for _, r := range g {
		for _, p := range r.Prev {
			seen[p] = true
		}
	}
	var tips []Record
	for _, r := range g {
		if !seen[r.ID] {
			tips = append(tips, r)
		}
	}
	return tips
}

// Tips は、r と同じ項目の、log の中の先端の ID を辞書順で返す。
func Tips(log []Record, r Record) []ID {
	var ids []ID
	for _, t := range tipsOf(group(log, key(r))) {
		ids = append(ids, t.ID)
	}
	slices.Sort(ids)
	return ids
}
