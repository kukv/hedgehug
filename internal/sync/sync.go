// Package sync syncs a store with its remotes: it fetches, merges field by
// field, repairs what holds across files and pushes.
package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/merge"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

var ErrDirty = errors.New("the store has uncommitted changes where the sync would write; check them with hedgehug doctor, then commit or restore them and run hedgehug sync")

// ErrUnrelated means a remote's main shares no commit with the store: it
// holds another store, or was made apart from this one.
var ErrUnrelated = errors.New("the remote holds a history of its own, not the store's")

type Options struct {
	Remotes    []string          // config.yaml stores[].remotes
	Author     gitrepo.Signature // for merge and repair commits
	Account    string            // the writer's account id; empty if none
	Me         []string          // all accounts, for the inbox
	Now        func() time.Time
	Others     []OtherStore // the other stores in config, for prefix duplicates across stores
	BeforePush func()       // a test hook: another clone pushes here
}

type OtherStore struct{ Name, Path string }

type Result struct {
	Merges      int
	Overwritten []merge.Overwritten
	Repaired    []merge.Repaired
	Invalid     []model.ULID // tasks left with problems (is:invalid)
	Pushed      []string     // remote names, of the last round
}

// rounds caps the fetch, merge and push rounds a push rejected by a remote
// starts again.
const rounds = 6

// Run syncs the store once. A store with no remote does nothing.
func Run(s *store.Store, opt Options) (Result, error) {
	var res Result
	if len(opt.Remotes) == 0 {
		return res, nil
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	r := s.Repo()
	before, err := r.Head()
	if err != nil {
		return res, err
	}
	ix, err := index.Open(s)
	if err != nil {
		return res, err
	}
	was := invalid(ix)
	if err := r.SetRemotes(opt.Remotes); err != nil {
		return res, err
	}
	if err := setup(s, opt); err != nil { // .gitattributes and the driver
		return res, err
	}
	wait := 500 * time.Millisecond
	for round := 0; ; round++ {
		res.Pushed = nil // a rejected mirror makes another round; report the last one
		for _, name := range r.Remotes() {
			if _, err := r.Fetch(name); err != nil {
				return res, err
			}
		}
		for _, name := range r.Remotes() {
			if err := mergeRemote(s, name, opt, &res); err != nil {
				return res, err
			}
		}
		if err := repairHead(s, opt, &res); err != nil {
			return res, err
		}
		if round == 0 {
			prefixDuplicates(s, opt, &res)
		}
		if opt.BeforePush != nil {
			opt.BeforePush()
		}
		err := r.Push("origin")
		if errors.Is(err, gitrepo.ErrRejected) && round < rounds-1 {
			time.Sleep(wait)
			wait *= 2
			continue
		}
		if errors.Is(err, gitrepo.ErrRejected) {
			return res, fmt.Errorf("push origin: %w", err)
		}
		if err != nil {
			return res, err
		}
		res.Pushed = append(res.Pushed, "origin")
		for _, name := range r.Remotes()[1:] {
			if err := r.Push(name); err == nil {
				res.Pushed = append(res.Pushed, name)
			} else if !errors.Is(err, gitrepo.ErrRejected) {
				return res, err
			}
			// A rejected mirror is merged in the next round; the rounds are capped.
		}
		if len(res.Pushed) == len(r.Remotes()) || round >= rounds-1 {
			break
		}
	}
	if err := findInvalid(s, ix, was, before, &res); err != nil {
		return res, err
	}
	if n, err := r.LooseObjects(); err == nil && n > 6700 {
		if err := r.Repack(); err != nil {
			return res, err
		}
	}
	return res, nil
}

// attributes are the lines of .gitattributes that send the files of the
// store to the hedgehug merge driver.
var attributes = []string{
	"tasks/**/*.md merge=hedgehug",
	"comments/**/*.md merge=hedgehug",
	"worklogs/**/*.md merge=hedgehug",
	"workspaces/*.md merge=hedgehug",
	"*.yaml merge=hedgehug",
}

const attributesPath = ".gitattributes"

// setup adds to .gitattributes the lines it lacks, in a commit of its own,
// and registers the merge driver. With nothing to add it commits nothing.
func setup(s *store.Store, opt Options) error {
	if _, missing, err := withAttributes(s); err != nil || missing {
		if err != nil {
			return err
		}
		op := store.Op{Name: "setup", Detail: "merge driver", Author: opt.Author, Account: opt.Account}
		if _, err := s.Update(op, func(tx *store.Tx) error {
			data, missing, err := withAttributes(s) // again, under the lock
			if err != nil || !missing {
				return err
			}
			return tx.PutFile(attributesPath, data)
		}); err != nil {
			return err
		}
	}
	return s.Repo().RegisterMergeDriver()
}

// withAttributes returns .gitattributes with the missing lines added at
// the end, and whether any were.
func withAttributes(s *store.Store) ([]byte, bool, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir(), attributesPath))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, false, err
	}
	have := map[string]bool{}
	for l := range strings.SplitSeq(string(data), "\n") {
		have[strings.TrimSpace(l)] = true
	}
	missing := false
	for _, l := range attributes {
		if have[l] {
			continue
		}
		if len(data) > 0 && data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		data = append(data, l+"\n"...)
		missing = true
	}
	return data, missing, nil
}

// mergeRemote brings the main of the remote name into HEAD: nothing when
// HEAD has it, a fast-forward when HEAD is behind it, a merge commit
// otherwise. When HEAD moves meanwhile it looks again.
func mergeRemote(s *store.Store, name string, opt Options, res *Result) error {
	r := s.Repo()
	for {
		head, err := r.Head()
		if err != nil {
			return err
		}
		remote, err := r.RemoteHead(name)
		if err != nil {
			return err
		}
		if done, err := r.IsAncestor(remote, head); err != nil || done {
			return err
		}
		ff, err := r.IsAncestor(head, remote)
		if err != nil {
			return err
		}
		if ff {
			err = fastForward(s, head, remote)
		} else {
			err = mergeCommit(s, name, head, remote, opt, res)
		}
		if !errors.Is(err, store.ErrHeadMoved) {
			return err
		}
	}
}

func fastForward(s *store.Store, head, to plumbing.Hash) error {
	r := s.Repo()
	from, err := r.TreeOf(head)
	if err != nil {
		return err
	}
	tree, err := r.TreeOf(to)
	if err != nil {
		return err
	}
	cs, err := r.DiffTrees(from, tree, "")
	if err != nil {
		return err
	}
	paths := make([]string, len(cs))
	for i, c := range cs {
		paths[i] = c.Path
	}
	if err := checkDirty(r, paths); err != nil {
		return err
	}
	err = s.FastForward(head, to, paths)
	if errors.Is(err, gitrepo.ErrWorkTreeChanged) {
		return ErrDirty
	}
	return err
}

// mergeCommit merges remote into head as one merge commit: the paths only
// the remote changed are taken from it, those both sides changed go
// through merge.Merge (schema.yaml first, for the others to use), and the
// repair runs over the result.
func mergeCommit(s *store.Store, name string, head, remote plumbing.Hash, opt Options, res *Result) error {
	r := s.Repo()
	base, err := r.MergeBase(head, remote)
	if err != nil {
		return err
	}
	if base.IsZero() {
		return fmt.Errorf("merge %s: %w", name, ErrUnrelated)
	}
	var trees [3]plumbing.Hash
	for i, c := range []plumbing.Hash{base, head, remote} {
		if trees[i], err = r.TreeOf(c); err != nil {
			return err
		}
	}
	bt, ht, rt := trees[0], trees[1], trees[2]
	ours, err := r.DiffTrees(bt, ht, "")
	if err != nil {
		return err
	}
	theirs, err := r.DiffTrees(bt, rt, "")
	if err != nil {
		return err
	}
	inOurs := map[string]bool{}
	for _, c := range ours {
		inOurs[c.Path] = true
	}
	files := map[string][]byte{} // nil removes
	var both []string
	for _, c := range theirs {
		switch {
		case inOurs[c.Path]:
			both = append(both, c.Path)
		case c.Deleted:
			files[c.Path] = nil
		default:
			if files[c.Path], err = r.ReadFile(rt, c.Path); err != nil {
				return err
			}
		}
	}
	var lost []merge.Overwritten
	var notices []merge.Notice
	sc := s.Schema()
	mergeFile := func(p string) error {
		b, o, t, err := sides(r, base, head, remote, p)
		if err != nil {
			return err
		}
		m, err := merge.Merge(p, sc, b, o, t, merge.LWW)
		if err != nil {
			return fmt.Errorf("merge %s: %w", p, err)
		}
		files[p] = m.Data
		lost = append(lost, m.Overwritten...)
		notices = append(notices, m.Notices...)
		return nil
	}
	// schema.yaml first: the merge of the other files reads the merged
	// schema, or HEAD's when it does not decode.
	if slices.Contains(both, schema.Path) {
		if err := mergeFile(schema.Path); err != nil {
			return err
		}
	}
	if data := files[schema.Path]; data != nil {
		if merged, err := schema.Decode(data); err == nil {
			sc = merged
		}
	}
	for _, p := range both {
		if p != schema.Path {
			if err := mergeFile(p); err != nil {
				return err
			}
		}
	}
	merged := maps.Clone(files)
	snap, err := newSnapshot(r, ht, files)
	if err != nil {
		return err
	}
	baseSnap, err := newSnapshot(r, bt, nil)
	if err != nil {
		return err
	}
	onOrigin, err := originIDs(r)
	if err != nil {
		return err
	}
	fixed, repaired, err := merge.Repair(merge.RepairInput{
		Files:    snap,
		Base:     baseSnap,
		OnOrigin: onOrigin,
		Changed:  func(p string) bool { _, ok := merged[p]; return ok },
		Author:   mergeAuthor(r, base, head, remote),
		Now:      opt.Now().Truncate(time.Second),
	})
	if err != nil {
		return err
	}
	maps.Copy(files, fixed)
	if err := checkDirty(r, slices.Collect(maps.Keys(files))); err != nil {
		return err
	}
	var trailers []gitrepo.Trailer
	for _, o := range lost {
		trailers = append(trailers, gitrepo.Trailer{Key: "Hedgehug-Overwritten", Value: strings.Join([]string{o.Path, o.Field, o.Value, orDash(o.Author)}, " ")})
	}
	for _, n := range notices {
		trailers = append(trailers, gitrepo.Trailer{Key: "Hedgehug-Repaired", Value: strings.Join([]string{fileULID(n.Path), string(n.Kind), "-", orDash(n.Author)}, " ")})
	}
	trailers = append(trailers, repairedTrailers(repaired)...)
	op := store.Op{Name: "merge", Detail: name, Author: opt.Author, Account: opt.Account}
	if _, err := s.CommitMerge(head, []plumbing.Hash{head, remote}, files, op, trailers); err != nil {
		return err
	}
	res.Merges++
	res.Overwritten = append(res.Overwritten, lost...)
	res.Repaired = append(res.Repaired, repaired...)
	return nil
}

func repairedTrailers(rs []merge.Repaired) []gitrepo.Trailer {
	out := make([]gitrepo.Trailer, len(rs))
	for i, x := range rs {
		out[i] = gitrepo.Trailer{Key: "Hedgehug-Repaired", Value: strings.Join([]string{x.ULID.String(), x.Kind, x.Value, orDash(x.Author)}, " ")}
	}
	return out
}

// orDash keeps an empty author a word of its own in a trailer.
func orDash(author string) string {
	if author == "" {
		return "-"
	}
	return author
}

// fileULID is the ULID a file of the store is named by. A file not named by
// one, such as schema.yaml or a template, is given by its path, which has no
// space either.
func fileULID(p string) string {
	if s, ok := strings.CutSuffix(path.Base(p), ".md"); ok {
		if u, err := model.ParseULID(s); err == nil {
			return u.String()
		}
	}
	return p
}

// mergeAuthor tells who last changed a file for a merge of head and remote
// over base: the side that changed it, the later one when both did (then
// the larger author, as the merge decides), or, when neither did, whoever
// changed it last before.
func mergeAuthor(r *gitrepo.Repo, base, head, remote plumbing.Hash) func(string) string {
	return func(p string) string {
		o, okO, err1 := r.LastChange(head, base, p)
		t, okT, err2 := r.LastChange(remote, base, p)
		if err1 != nil || err2 != nil {
			return ""
		}
		switch {
		case okO && okT:
			if t.When.After(o.When) || t.When.Equal(o.When) && author(t) > author(o) {
				return author(t)
			}
			return author(o)
		case okO:
			return author(o)
		case okT:
			return author(t)
		}
		return headAuthor(r, head)(p)
	}
}

// headAuthor tells who last changed a file in the history of head.
func headAuthor(r *gitrepo.Repo, head plumbing.Hash) func(string) string {
	return func(p string) string {
		e, ok, err := r.LastChange(head, plumbing.ZeroHash, p)
		if err != nil || !ok {
			return ""
		}
		return author(e)
	}
}

// originIDs answers whether the main of origin, as last fetched, has the
// task ulid with the id.
func originIDs(r *gitrepo.Repo) (func(model.ULID, string) bool, error) {
	h, err := r.RemoteHead("origin")
	if err != nil {
		return nil, err
	}
	tree, err := r.TreeOf(h)
	if err != nil {
		return nil, err
	}
	return func(u model.ULID, id string) bool {
		if tree.IsZero() {
			return false
		}
		b, err := r.ReadFile(tree, model.TaskPath(u))
		if err != nil {
			return false
		}
		t, err := model.DecodeTask(b)
		return err == nil && t.ID == id
	}, nil
}

// checkDirty returns ErrDirty when one of paths has changes git has not
// committed.
func checkDirty(r *gitrepo.Repo, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	st, err := r.Status()
	if err != nil {
		return err
	}
	dirty := map[string]bool{}
	for _, f := range st {
		dirty[f.Path] = true
	}
	for _, p := range paths {
		if dirty[p] {
			return ErrDirty
		}
	}
	return nil
}

// repairHead repairs the files of HEAD, for what a merge made by hand or a
// file edited by hand brought in, and commits what it fixed. What it only
// tells of, like a prefix two workspaces share, goes to res without a
// commit, once per run.
func repairHead(s *store.Store, opt Options, res *Result) error {
	r := s.Repo()
	for {
		head, err := r.Head()
		if err != nil {
			return err
		}
		tree, err := r.TreeOf(head)
		if err != nil {
			return err
		}
		snap, err := newSnapshot(r, tree, nil)
		if err != nil {
			return err
		}
		onOrigin, err := originIDs(r)
		if err != nil {
			return err
		}
		fixed, repaired, err := merge.Repair(merge.RepairInput{
			Files:    snap,
			OnOrigin: onOrigin,
			Changed:  func(string) bool { return true },
			Author:   headAuthor(r, head),
			Now:      opt.Now().Truncate(time.Second),
		})
		if err != nil {
			return err
		}
		if len(fixed) == 0 {
			for _, x := range repaired {
				addNotice(res, x)
			}
			return nil
		}
		if err := checkDirty(r, slices.Collect(maps.Keys(fixed))); err != nil {
			return err
		}
		op := store.Op{Name: "repair", Detail: fmt.Sprintf("%d issues", len(repaired)), Author: opt.Author, Account: opt.Account}
		_, err = s.CommitMerge(head, []plumbing.Hash{head}, fixed, op, repairedTrailers(repaired))
		if errors.Is(err, store.ErrHeadMoved) {
			continue
		}
		if err != nil {
			return err
		}
		res.Repaired = append(res.Repaired, repaired...)
		return nil
	}
}

// addNotice adds x to res unless res already tells the same of the same
// workspace or task.
func addNotice(res *Result, x merge.Repaired) {
	if !slices.ContainsFunc(res.Repaired, func(y merge.Repaired) bool {
		return y.ULID == x.ULID && y.Kind == x.Kind && y.Value == x.Value
	}) {
		res.Repaired = append(res.Repaired, x)
	}
}

// prefixDuplicates tells of each workspace of the store whose id prefix a
// workspace of another store has too, when this store's one set it later
// (the same time: the larger ULID). The other store tells of its own when
// it syncs. A store that does not open is skipped.
func prefixDuplicates(s *store.Store, opt Options, res *Result) {
	var mine []*model.Workspace
	for w, err := range s.Workspaces() {
		if err == nil && w.IDPrefix != "" {
			mine = append(mine, w)
		}
	}
	head, err := s.Repo().Head()
	if len(mine) == 0 || err != nil {
		return
	}
	who := headAuthor(s.Repo(), head)
	for _, o := range opt.Others {
		other, err := store.Open(o.Path)
		if err != nil {
			continue
		}
		for w, err := range other.Workspaces() {
			if err != nil {
				continue
			}
			for _, m := range mine {
				if !strings.EqualFold(m.IDPrefix, w.IDPrefix) {
					continue
				}
				if c := m.PrefixSet.Compare(w.PrefixSet); c < 0 || c == 0 && m.ULID.String() < w.ULID.String() {
					continue
				}
				v, _ := json.Marshal(struct {
					Store     string `json:"store"`
					Workspace string `json:"workspace"`
				}{o.Name, w.Name}) // strings always encode
				addNotice(res, merge.Repaired{ULID: m.ULID, Kind: "prefix_duplicate", Value: string(v), Author: who(model.WorkspacePath(m.ULID))})
			}
		}
	}
}

// invalid returns the tasks that have problems, as is:invalid finds them.
func invalid(ix *index.Index) map[model.ULID]bool {
	out := map[model.ULID]bool{}
	for _, e := range ix.Tasks() {
		if len(ix.Problems(e.ULID)) > 0 {
			out[e.ULID] = true
		}
	}
	return out
}

// findInvalid puts in res, in ULID order, the tasks that have problems now
// and had none before the sync, which was at the commit before.
func findInvalid(s *store.Store, ix *index.Index, was map[model.ULID]bool, before plumbing.Hash, res *Result) error {
	after, err := s.Repo().Head()
	if err != nil || after == before {
		return err
	}
	if err := ix.Refresh(); err != nil {
		return err
	}
	for u := range invalid(ix) {
		if !was[u] {
			res.Invalid = append(res.Invalid, u)
		}
	}
	slices.SortFunc(res.Invalid, compareULIDs)
	return nil
}

func compareULIDs(a, b model.ULID) int { return strings.Compare(a.String(), b.String()) }

// snapshot is the files of a tree with some laid over it (nil removes).
type snapshot struct {
	r     *gitrepo.Repo
	tree  plumbing.Hash
	over  map[string][]byte
	paths []string // sorted
}

func newSnapshot(r *gitrepo.Repo, tree plumbing.Hash, over map[string][]byte) (*snapshot, error) {
	s := &snapshot{r: r, tree: tree, over: over}
	for p, err := range r.Files(tree, "") {
		if err != nil {
			return nil, err
		}
		if _, ok := over[p]; !ok {
			s.paths = append(s.paths, p)
		}
	}
	for p, d := range over {
		if d != nil {
			s.paths = append(s.paths, p)
		}
	}
	slices.Sort(s.paths)
	return s, nil
}

func (s *snapshot) Paths(prefix string) []string {
	var out []string
	for _, p := range s.paths {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	return out
}

func (s *snapshot) Read(p string) []byte {
	if d, ok := s.over[p]; ok {
		return d
	}
	b, err := s.r.ReadFile(s.tree, p)
	if err != nil {
		return nil
	}
	return b
}
