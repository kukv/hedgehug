package change

import (
	"encoding/json"
	"slices"
	"strings"
)

// Options は、エンティティの種類ごとの決め方。
type Options struct {
	Multi func(f string) bool // 値を複数持つ項目か。nil なら、どれも 1 つ
	Valid func(r Record) bool // 手順 0 の検査。nil なら、どの行も有効
}

// State は、ログから決めたいまの値。Values は値を 1 つ持つ項目、Lists は複数持つ項目(値の JSON の辞書順)。
type State struct {
	Created *Record
	Values  map[string]json.RawMessage
	Lists   map[string][]json.RawMessage
}

// Resolve は、保存形式「いまの値の決め方」の手順 0〜3 で、項目ごとのいまの値を決める。
// 結果は、log の行の並びによらない。
func Resolve(log []Record, opt Options) State {
	valid := filterValid(log, opt.Valid)
	s := State{Values: map[string]json.RawMessage{}, Lists: map[string][]json.RawMessage{}}
	c := winner(creates(valid))
	if c == nil {
		return s
	}
	s.Created = c
	initial := map[string]json.RawMessage{}
	_ = json.Unmarshal(c.To, &initial) // 形の違う create は、初期値なしとして扱う
	multi := opt.Multi
	if multi == nil {
		multi = func(string) bool { return false }
	}
	g := groupAll(valid)
	for f := range fields(valid, initial) {
		if multi(f) {
			if vs := resolveList(valid, g, f, initial[f]); len(vs) > 0 {
				s.Lists[f] = vs
			}
		} else if v, ok := resolveValue(g, f, initial); ok {
			s.Values[f] = v
		}
	}
	return s
}

func filterValid(log []Record, valid func(Record) bool) []Record {
	var out []Record
	for _, r := range log {
		known := r.T == Create || r.T == Set || r.T == Add || r.T == Remove
		if known && (valid == nil || valid(r)) {
			out = append(out, r)
		}
	}
	return out
}

func creates(log []Record) []Record {
	var out []Record
	for _, r := range log {
		if r.T == Create {
			out = append(out, r)
		}
	}
	return out
}

func fields(log []Record, initial map[string]json.RawMessage) map[string]bool {
	fs := map[string]bool{}
	for f := range initial {
		fs[f] = true
	}
	for _, r := range log {
		if r.T != Create {
			fs[r.F] = true
		}
	}
	return fs
}

func resolveValue(g groups, f string, initial map[string]json.RawMessage) (json.RawMessage, bool) {
	w := winner(tipsOf(g.of(key(Record{T: Set, F: f}))))
	if w == nil {
		return nil, false // 先端がない(prev が輪になっている)項目は、値なし
	}
	if w.T == Create {
		v, ok := initial[f]
		return v, ok
	}
	return w.To, true
}

func resolveList(log []Record, g groups, f string, initial json.RawMessage) []json.RawMessage {
	var start []json.RawMessage
	_ = json.Unmarshal(initial, &start)
	inStart := map[string]bool{}
	cands := map[string]bool{}
	for _, v := range start {
		inStart[canonical(v)] = true
		cands[canonical(v)] = true
	}
	for _, r := range log {
		switch {
		case r.F == f && r.T == Add:
			cands[canonical(r.To)] = true
		case r.F == f && r.T == Remove:
			cands[canonical(r.From)] = true
		}
	}
	var out []json.RawMessage
	for c := range cands {
		w := winner(tipsOf(g.of("list\x00" + f + "\x00" + c)))
		if w != nil && (w.T == Add || (w.T == Create && inStart[c])) {
			out = append(out, json.RawMessage(c))
		}
	}
	slices.SortFunc(out, func(a, b json.RawMessage) int { return strings.Compare(string(a), string(b)) })
	return out
}

// winner は、先端のうち、手順 3 の比べ方で最もあとの行を返す。
func winner(tips []Record) *Record {
	var w *Record
	for i := range tips {
		if w == nil || later(tips[i], *w) {
			w = &tips[i]
		}
	}
	return w
}

// later は、a が b よりあとか。at(UTC)、by(小文字)、id の順に比べる。
func later(a, b Record) bool {
	if !a.At.Equal(b.At) {
		return a.At.After(b.At)
	}
	if la, lb := strings.ToLower(a.By), strings.ToLower(b.By); la != lb {
		return la > lb
	}
	return a.ID > b.ID
}
