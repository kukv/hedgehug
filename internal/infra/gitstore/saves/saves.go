// Package saves は、同期で受け取った保存のうち、通知の判定をまだしていないものを読む(決定 C4b1-2)。
// gitstore の同期が、判定の差し込み口に渡すために使う。
package saves

import (
	"cmp"
	"encoding/json"
	"errors"
	"path"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// OpTrailer は、保存のコミットのメッセージで操作の ID を示すトレーラー(gitstore の commit・同期もこれを使う)。
const OpTrailer = "\nHedgehug-Op: "

// KeyOf は、コミットの見分け方。作者はコミットの作者名、時刻は作者の時刻、Op はトレーラー(なければ空)。
func KeyOf(c *object.Commit) port.SaveKey {
	var op change.ID
	if _, rest, ok := strings.Cut(c.Message, OpTrailer); ok {
		line, _, _ := strings.Cut(rest, "\n")
		op = change.ID(strings.TrimSpace(line))
	}
	return port.SaveKey{Version: c.Hash.String(), Author: c.Author.Name, At: c.Author.When, Op: op}
}

// Unjudged は、from から辿れて判定済みでない保存(Hedgehug-Op を持つもの)を古い順に返す。親が先で、
// 同じ深さは作者の時刻、印の順。辿るのは SameVersion に当たったところで止め、SameSave は返さずに親へ
// 辿り続ける。合わせと最初の保存(Hedgehug-Op のないもの)は返さないが、その親は辿る。
func Unjudged(repo *git.Repository, from plumbing.Hash, known func(port.SaveKey) (port.Match, error)) ([]port.ReceivedSave, error) {
	nodes, err := walk(repo, from, known)
	if err != nil {
		return nil, err
	}
	var out []port.ReceivedSave
	for _, n := range ordered(nodes) {
		if n.match != port.NotJudged || n.key.Op == "" {
			continue
		}
		es, err := entities(n.c)
		if err != nil {
			return nil, err
		}
		out = append(out, port.ReceivedSave{Key: n.key, Entities: es})
	}
	return out, nil
}

// Reachable は、from から辿れて判定済みでない保存をすべて返す(合わせ・最初の保存を含む)。辿り方は Unjudged
// と同じ。SameSave の保存も返す(印を記録すれば、次からはそこで止まる)。
func Reachable(repo *git.Repository, from plumbing.Hash, known func(port.SaveKey) (port.Match, error)) ([]port.SaveKey, error) {
	nodes, err := walk(repo, from, known)
	if err != nil {
		return nil, err
	}
	out := make([]port.SaveKey, 0, len(nodes))
	for _, n := range ordered(nodes) {
		out = append(out, n.key)
	}
	return out, nil
}

// node は、辿った保存 1 件。
type node struct {
	c     *object.Commit
	key   port.SaveKey
	match port.Match
}

// walk は、from から親を辿り、SameVersion に当たったところで止める。辿った保存(NotJudged と SameSave)を返す。
func walk(repo *git.Repository, from plumbing.Hash, known func(port.SaveKey) (port.Match, error)) (map[plumbing.Hash]node, error) {
	nodes := map[plumbing.Hash]node{}
	if from.IsZero() {
		return nodes, nil
	}
	stopped := map[plumbing.Hash]bool{}
	stack := []plumbing.Hash{from}
	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := nodes[h]; ok || stopped[h] {
			continue
		}
		c, err := repo.CommitObject(h)
		if err != nil {
			return nil, err
		}
		k := KeyOf(c)
		m, err := known(k)
		if err != nil {
			return nil, err
		}
		if m == port.SameVersion {
			stopped[h] = true
			continue
		}
		nodes[h] = node{c: c, key: k, match: m}
		stack = append(stack, c.ParentHashes...)
	}
	return nodes, nil
}

// ordered は、nodes を古い順に並べる。深さ(nodes の中で最も長い親の連なり)、作者の時刻、印の順。
// 深さで並べるので、時計のずれたマシンの保存でも親が先になる。
func ordered(nodes map[plumbing.Hash]node) []node {
	depth := depths(nodes)
	out := make([]node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n)
	}
	slices.SortFunc(out, func(a, b node) int {
		return cmp.Or(
			cmp.Compare(depth[a.c.Hash], depth[b.c.Hash]),
			a.key.At.Compare(b.key.At),
			strings.Compare(a.key.Version, b.key.Version),
		)
	})
	return out
}

// depths は、nodes の保存ごとの深さ。nodes の中に親がなければ 0。長い履歴でも再帰しない。
func depths(nodes map[plumbing.Hash]node) map[plumbing.Hash]int {
	depth := make(map[plumbing.Hash]int, len(nodes))
	for h := range nodes {
		stack := []plumbing.Hash{h}
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			waiting, d := parentDepth(nodes, depth, top)
			if len(waiting) > 0 {
				stack = append(stack, waiting...)
				continue
			}
			depth[top] = d
			stack = stack[:len(stack)-1]
		}
	}
	return depth
}

// parentDepth は、h の深さを、深さの決まった親から決める。深さの決まっていない親があれば waiting に返す。
func parentDepth(nodes map[plumbing.Hash]node, depth map[plumbing.Hash]int, h plumbing.Hash) (waiting []plumbing.Hash, d int) {
	for _, p := range nodes[h].c.ParentHashes {
		if _, in := nodes[p]; !in {
			continue
		}
		if pd, done := depth[p]; done {
			d = max(d, pd+1)
		} else {
			waiting = append(waiting, p)
		}
	}
	return waiting, d
}

// entities は、保存 c が、最初の親から変えたエンティティごとの、足した行と説明。Ref の順。
func entities(c *object.Commit) ([]port.EntityRows, error) {
	tree, err := c.Tree()
	if err != nil {
		return nil, err
	}
	var parent *object.Tree
	if c.NumParents() > 0 {
		p, err := c.Parent(0)
		if err != nil {
			return nil, err
		}
		if parent, err = p.Tree(); err != nil {
			return nil, err
		}
	}
	changes, err := object.DiffTree(parent, tree)
	if err != nil {
		return nil, err
	}
	byRef := map[change.Ref]*port.EntityRows{}
	for _, ch := range changes {
		if err := addChange(byRef, tree, ch); err != nil {
			return nil, err
		}
	}
	out := make([]port.EntityRows, 0, len(byRef))
	for _, e := range byRef {
		if len(e.Rows) > 0 || e.Description != nil {
			out = append(out, *e)
		}
	}
	slices.SortFunc(out, func(a, b port.EntityRows) int {
		return cmp.Or(strings.Compare(string(a.Ref.Kind), string(b.Ref.Kind)), strings.Compare(string(a.Ref.ID), string(b.Ref.ID)))
	})
	return out, nil
}

// addChange は、1 つのファイルの変更を byRef に足す。エンティティの log.jsonl と description.md だけを見る。
func addChange(byRef map[change.Ref]*port.EntityRows, tree *object.Tree, ch *object.Change) error {
	name := cmp.Or(ch.To.Name, ch.From.Name)
	dir, base := path.Split(name)
	ref, ok := RefOf(strings.TrimSuffix(dir, "/"))
	if !ok || !ref.Valid() || (base != "log.jsonl" && base != "description.md") {
		return nil
	}
	from, to, err := ch.Files()
	if err != nil {
		return err
	}
	e := byRef[ref]
	if e == nil {
		e = &port.EntityRows{Ref: ref}
		byRef[ref] = e
	}
	if base == "log.jsonl" {
		e.Rows, err = added(from, to)
		return err
	}
	if to == nil {
		// 説明を消した。エンティティごと消えた(完全に削除した)ときは説明の変更として扱わない。
		if _, err := tree.File(dir + "log.jsonl"); errors.Is(err, object.ErrFileNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		empty := ""
		e.Description = &empty
		return nil
	}
	text, err := to.Contents()
	e.Description = &text
	return err
}

// added は、to の log.jsonl の行のうち、from にない id の行。
func added(from, to *object.File) ([]change.Record, error) {
	if to == nil {
		return nil, nil
	}
	old := map[change.ID]bool{}
	if from != nil {
		b, err := from.Contents()
		if err != nil {
			return nil, err
		}
		for _, r := range parseLog(b) {
			old[r.ID] = true
		}
	}
	b, err := to.Contents()
	if err != nil {
		return nil, err
	}
	var out []change.Record
	for _, r := range parseLog(b) {
		if !old[r.ID] {
			out = append(out, r)
		}
	}
	return out, nil
}

// parseLog は、log.jsonl の中身を変更記録にする。形の違う行は読み飛ばす(gitstore の parseLog と同じ)。
func parseLog(s string) []change.Record {
	var out []change.Record
	for _, l := range strings.Split(s, "\n") {
		var r change.Record
		if strings.TrimSpace(l) != "" && json.Unmarshal([]byte(l), &r) == nil {
			out = append(out, r)
		}
	}
	return out
}

// Dirs は、種類ごとのエンティティのフォルダの名前(Store の根から。gitstore もこれを使う)。
var Dirs = map[change.Kind]string{
	change.KindMembers:   "members",
	change.KindWorkspace: "workspaces",
	change.KindProject:   "projects",
	change.KindItem:      "items",
}

// RefOf は、フォルダ(members、items/<ID> など)の種類と ID。members はそのフォルダちょうどのときだけ。
// ID が使えるものかは確かめない(ログを指せるかは Ref.Valid で確かめる)。
func RefOf(dir string) (change.Ref, bool) {
	top, id, sub := strings.Cut(dir, "/")
	for kind, d := range Dirs {
		if d == top && (kind != change.KindMembers || !sub) {
			return change.Ref{Kind: kind, ID: change.ID(id)}, true
		}
	}
	return change.Ref{}, false
}
