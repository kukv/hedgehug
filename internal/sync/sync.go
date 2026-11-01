// Package sync syncs a store with its remotes: it fetches, merges field by
// field, repairs what holds across files and pushes.
package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/merge"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// ErrDirty means a file the sync would write has changes git has not
// committed. The sync changes nothing then.
var ErrDirty = errors.New("the store has uncommitted changes where the sync would write; check them with hedgehug doctor, then commit or restore them and run hedgehug sync")

// ErrUnrelated means a remote's main shares no commit with the store: it
// holds another store, or was made apart from this one.
var ErrUnrelated = errors.New("the remote holds a history of its own, not the store's")

// Options is what one sync needs besides the store.
type Options struct {
	Remotes    []string          // config.yaml stores[].remotes
	Author     gitrepo.Signature // for merge and repair commits
	Account    string            // the writer's account id; empty if none
	Me         []string          // all accounts, for the inbox
	Now        func() time.Time
	Others     []OtherStore // the other stores in config, for prefix duplicates across stores
	Conn       gitrepo.Conn // how the remotes are reached: url_rewrites and the token
	BeforePush func()       // a test hook: another clone pushes here
	Name       string       // the store's name in config, for the read-only error
	GitHub     *GitHubRepo  // origin on GitHub; nil when it is not
	Explicit   bool         // hh sync, not the background one: a push_denied is tried again

	// origin, when set, is the commit the repair takes as the main of origin
	// (the replay after a history rewrite, where it is not the one tracked).
	origin plumbing.Hash
	// skip, when set, names the paths mergeTrees leaves out of both sides
	// (the files of the purged tasks in the replay).
	skip func(path string) bool
}

// OtherStore is another store of the config, by its name.
type OtherStore = store.NamedStore

// Result is what one sync did.
type Result struct {
	Merges      int
	Overwritten []merge.Overwritten
	Repaired    []merge.Repaired
	Invalid     []model.ULID // tasks left with problems (is:invalid)
	Pushed      []string     // remote names, of the last round
	Notices     []inbox.Item // what the sync added to the inbox
	// Warnings are what failed after the push, which the sync did not undo:
	// other stores' prefixes, the inbox, finding the invalid tasks,
	// repacking.
	Warnings []string
	// Rewrite is the history rewrite of origin that is recorded and not yet
	// taken in; the sync stopped before changing the store.
	Rewrite *model.Rewrite
	// Rebuild is the clone made again for Rewrite, ready to be swapped in;
	// Run does not swap it (the caller releases the locks first). The store
	// is as it was.
	Rebuild *Rebuild
	// Swapped is set by the caller that put Rebuild in the place of the
	// store. Reopen is set by a caller that closed the Store handle the sync
	// was given (it swapped, or tried to): the caller of that opens it again.
	Swapped, Reopen bool
	// Unpushable counts the commits not pushed because the store is read-only.
	Unpushable int
}

const (
	// rounds caps the fetch, merge and push rounds that a push rejected by
	// a remote starts again.
	rounds = 6
	// tries caps how often one merge or repair is computed again because
	// HEAD moved under it.
	tries = 6
)

// Replaced by tests.
var (
	retryWait    = 500 * time.Millisecond // the first wait after a rejected push; it doubles
	repackAt     = 6700                   // repack when there are more loose objects
	beforeCommit = func() {}              // runs before each fast-forward, merge or repair commit
	onFullRepair = func() {}              // runs when the repair reads every file of HEAD
)

// Run syncs the store once. A store with no remote does nothing. Whether
// it succeeds or not, it then fills the inbox from what HEAD gained.
func Run(s *store.Store, opt Options) (res Result, err error) {
	if len(opt.Remotes) == 0 {
		return res, nil
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	r := s.Repo()
	r.SetConn(opt.Conn)
	acc, brokenWarning, err := readAccessOrReset(s)
	if err != nil {
		return res, err
	}
	if brokenWarning != "" {
		res.Warnings = append(res.Warnings, brokenWarning)
	}
	if opt.Explicit && acc.Reason == ReasonPushDenied {
		acc = Access{}
		if err := WriteAccess(s, acc); err != nil {
			return res, err
		}
	}
	// A person who runs hh sync gets a read-only store judged again at once,
	// whatever infoInterval says: the permission may have come back. A check
	// that fails leaves the access as it was, with its warning.
	if opt.Explicit && acc.ReadOnly {
		a, warns, err := CheckAccess(s, opt, opt.Now())
		res.Warnings = append(res.Warnings, warns...)
		if err != nil {
			res.Warnings = append(res.Warnings, "check the access: "+err.Error())
		} else {
			acc = a
		}
	}
	before, err := r.Head()
	if err != nil {
		return res, err
	}
	// The inbox catches up from here, even when a Catch runs meanwhile.
	if err := inbox.Open(s).Start(before); err != nil {
		return res, err
	}
	// A failed sync may have moved HEAD too: a fast-forward, a merge.
	defer func() {
		if err := notify(s, before, opt, &res); err != nil {
			res.Warnings = append(res.Warnings, "fill the inbox: "+err.Error())
		}
	}()
	// setup commits after the merges; refuse now what it would refuse then.
	if _, err := attributesToAdd(s, before, true); err != nil {
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
	if err := r.RegisterMergeDriver(); err != nil {
		return res, err
	}
	wait := retryWait
	// A mirror that cannot be reached is left out, and told of as a
	// warning; origin is the remote the sync is for.
	down := map[string]error{}
	diverged := map[string]bool{}
	for round := 0; ; round++ {
		res.Pushed = nil // a rejected mirror makes another round; report the last one
		var up []string
		var tracked plumbing.Hash // origin before the fetch
		for i, name := range r.Remotes() {
			if i == 0 {
				if tracked, err = r.RemoteHead(name); err != nil {
					return res, err
				}
			}
			if _, err := r.Fetch(name); err != nil && i == 0 {
				return res, err
			} else if err != nil {
				down[name] = err
				continue
			}
			if i == 0 {
				if err := checkOrigin(s, tracked, opt, &res); err != nil {
					return res, err
				}
				if res.Rewrite != nil {
					return res, rebuild(s, tracked, opt, &res)
				}
			}
			delete(down, name) // a warning tells of how the last round went
			up = append(up, name)
		}
		// A mirror that still has the history before a rewrite is not merged
		// (it would bring back what the rewrite removed); it is force pushed
		// to below. Origin comes first, so HEAD has its rewrites.yaml.
		stale := map[string]plumbing.Hash{}
		left := map[string]bool{}
		for i, name := range up {
			if i > 0 {
				mirror, err := r.RemoteHead(name)
				if err != nil {
					return res, err
				}
				head, err := r.Head()
				if err != nil {
					return res, err
				}
				state, err := staleMirror(r, head, mirror)
				if err != nil {
					return res, err
				}
				if state == mirrorStale {
					stale[name] = mirror
					continue
				}
				if state == mirrorDiverged {
					left[name] = true
					diverged[name] = true
					continue
				}
			}
			if err := mergeRemote(s, name, opt, &res); err != nil {
				return res, err
			}
		}
		if err := repairHead(s, ix, opt, &res); err != nil {
			return res, err
		}
		if err := setup(s, opt); err != nil {
			return res, err
		}
		if opt.BeforePush != nil {
			opt.BeforePush()
		}
		if acc.ReadOnly {
			n, err := Unpushed(r)
			if err != nil {
				return res, err
			}
			res.Unpushable = n
			break // no push, not to the mirrors either
		}
		err := r.Push("origin")
		if errors.Is(err, gitrepo.ErrPushDenied) {
			denied := Access{ReadOnly: true, Reason: ReasonPushDenied, Checked: opt.Now()}
			return res, &readOnlyError{msg: opt.storeName() + " is read-only (" + denied.Why() + ")", errs: []error{err, WriteAccess(s, denied)}}
		}
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
		rejected := false
		for _, name := range up[1:] {
			if left[name] {
				continue
			}
			var err error
			if mirror, old := stale[name]; old {
				err = r.ForcePush(name, mirror)
			} else {
				err = r.Push(name)
			}
			switch {
			case err == nil:
				res.Pushed = append(res.Pushed, name)
			case errors.Is(err, gitrepo.ErrRejected):
				rejected = true // merged in the next round; the rounds are capped
			default:
				down[name] = err
			}
		}
		if !rejected || round >= rounds-1 {
			break
		}
	}
	// The push is done: what fails from here is a warning.
	warn := func(what string, err error) {
		if err != nil {
			res.Warnings = append(res.Warnings, what+": "+err.Error())
		}
	}
	for _, name := range r.Remotes()[1:] {
		warn("sync mirror "+name, down[name])
		if diverged[name] {
			res.Warnings = append(res.Warnings, name+" has commits that are in neither history; it is not pushed")
		}
	}
	refreshInfo(s, opt, &res)
	warn("check the other stores' id prefixes", prefixDuplicates(s, opt, &res))
	warn("find the invalid tasks", findInvalid(s, ix, was, before, &res))
	n, err := r.LooseObjects()
	warn("count the loose objects", err)
	if err == nil && n > repackAt {
		warn("repack", r.Repack())
	}
	return res, nil
}

// setup adds to .gitattributes the lines it lacks, in a commit of its own.
// With nothing to add it commits nothing.
func setup(s *store.Store, opt Options) error {
	head, err := s.Repo().Head()
	if err != nil {
		return err
	}
	if data, err := attributesToAdd(s, head, false); err != nil || data == nil {
		return err
	}
	op := store.Op{Name: "setup", Detail: "merge driver", Author: opt.Author, Account: opt.Account}
	_, err = s.Update(op, func(tx *store.Tx) error {
		head, err := s.Repo().Head() // again, under the lock
		if err != nil {
			return err
		}
		data, err := attributesToAdd(s, head, true)
		if err != nil || data == nil {
			return err
		}
		return tx.PutFile(store.AttributesPath, data)
	})
	return err
}

// NeedsSetup reports whether HEAD's .gitattributes lacks a line of the merge
// driver, which a sync adds.
func NeedsSetup(s *store.Store) (bool, error) {
	head, err := s.Repo().Head()
	if err != nil {
		return false, err
	}
	data, err := attributesToAdd(s, head, false)
	return data != nil, err
}

// attributesToAdd returns .gitattributes as commit has it with the missing
// lines added at the end, or nil when none is missing. A line is there when
// a line of the file gives its pattern merge=hedgehug. With checkDirty, a
// file that would change but has changes git has not committed is ErrDirty.
func attributesToAdd(s *store.Store, commit plumbing.Hash, checkDirty bool) ([]byte, error) {
	data, err := s.Repo().ReadAt(commit, store.AttributesPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	have := map[string]bool{}
	for l := range strings.SplitSeq(string(data), "\n") {
		f := strings.Fields(l)
		if len(f) > 1 && !strings.HasPrefix(f[0], "#") && slices.Contains(f[1:], "merge=hedgehug") {
			have[f[0]] = true
		}
	}
	missing := false
	for _, l := range store.Attributes {
		if pattern, _, _ := strings.Cut(l, " "); have[pattern] {
			continue
		}
		if len(data) > 0 && data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		data = append(data, l+"\n"...)
		missing = true
	}
	if !missing {
		return nil, nil
	}
	if checkDirty {
		if err := dirty(s.Repo(), []string{store.AttributesPath}); err != nil {
			return nil, err
		}
	}
	return data, nil
}

// mergeRemote brings the main of the remote name into HEAD: nothing when
// HEAD has it, a fast-forward when HEAD is behind it, a merge commit
// otherwise. When HEAD moves meanwhile it looks again, up to tries times.
func mergeRemote(s *store.Store, name string, opt Options, res *Result) error {
	r := s.Repo()
	for try := 1; ; try++ {
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
		if !errors.Is(err, store.ErrHeadMoved) || try == tries {
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
	if err := dirty(r, paths); err != nil {
		return err
	}
	beforeCommit()
	err = s.FastForward(head, to, paths)
	if errors.Is(err, gitrepo.ErrWorkTreeChanged) {
		return ErrDirty
	}
	return err
}

// mergeCommit merges remote into head as one merge commit (mergeTrees over
// the merge base).
func mergeCommit(s *store.Store, name string, head, remote plumbing.Hash, opt Options, res *Result) error {
	r := s.Repo()
	base, err := r.MergeBase(head, remote)
	if err != nil {
		return err
	}
	if base.IsZero() {
		return fmt.Errorf("merge %s: %w", name, ErrUnrelated)
	}
	m, err := mergeTrees(r, base, head, remote, opt)
	if err != nil {
		return err
	}
	if err := dirty(r, slices.Collect(maps.Keys(m.files))); err != nil {
		return err
	}
	op := store.Op{Name: "merge", Detail: name, Author: opt.Author, Account: opt.Account}
	beforeCommit()
	_, err = s.CommitMerge(head, []plumbing.Hash{head, remote}, m.files, op, m.trailers())
	if errors.Is(err, gitrepo.ErrWorkTreeChanged) {
		return ErrDirty
	}
	if err != nil {
		return err
	}
	res.Merges++
	res.Overwritten = append(res.Overwritten, m.lost...)
	res.Repaired = append(res.Repaired, m.repaired...)
	return nil
}

// merged is what mergeTrees made: the files that differ from ours (nil
// removes), what lost, and what the repair did.
type merged struct {
	files    map[string][]byte
	lost     []merge.Overwritten
	notices  []merge.Notice
	repaired []merge.Repaired
}

// trailers are the trailers of the commit that holds the merge.
func (m *merged) trailers() []gitrepo.Trailer {
	var ts []gitrepo.Trailer
	ts = append(ts, overwrittenTrailers(m.lost)...)
	for _, n := range m.notices {
		ts = append(ts, gitrepo.Trailer{Key: "Hedgehug-Repaired", Value: strings.Join([]string{url.PathEscape(fileULID(n.Path)), string(n.Kind), "-", orDash(n.Author)}, " ")})
	}
	return append(ts, repairedTrailers(m.repaired)...)
}

// mergeTrees merges the commit theirs into ours over base: the paths only
// theirs changed are taken from it, those both sides changed go through
// merge.Merge (schema.yaml first, for the others to use), and the repair
// runs over the result. The files are the changes to make to ours. base need
// not be an ancestor of either side (the replay after a history rewrite).
func mergeTrees(r *gitrepo.Repo, base, ours, theirs plumbing.Hash, opt Options) (*merged, error) {
	var trees [3]plumbing.Hash
	for i, c := range []plumbing.Hash{base, ours, theirs} {
		var err error
		if trees[i], err = r.TreeOf(c); err != nil {
			return nil, err
		}
	}
	bt, ht, rt := trees[0], trees[1], trees[2]
	oursDiff, err := r.DiffTrees(bt, ht, "")
	if err != nil {
		return nil, err
	}
	theirsDiff, err := r.DiffTrees(bt, rt, "")
	if err != nil {
		return nil, err
	}
	if opt.skip != nil {
		leave := func(c gitrepo.Change) bool { return opt.skip(c.Path) }
		oursDiff = slices.DeleteFunc(oursDiff, leave)
		theirsDiff = slices.DeleteFunc(theirsDiff, leave)
	}
	changed := map[string]bool{} // by either side
	for _, c := range oursDiff {
		changed[c.Path] = true
	}
	files := map[string][]byte{} // nil removes
	var both []string
	for _, c := range theirsDiff {
		switch {
		case changed[c.Path]:
			both = append(both, c.Path)
		case c.Deleted:
			files[c.Path] = nil
		default:
			if files[c.Path], err = r.ReadFile(rt, c.Path); err != nil {
				return nil, err
			}
		}
		changed[c.Path] = true
	}
	var lost []merge.Overwritten
	var notices []merge.Notice
	sc, err := schemaAt(r, ht)
	if err != nil {
		return nil, err
	}
	mergeFile := func(p string) error {
		b, o, t, err := Sides(r, base, ours, theirs, p)
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
	// schema, or ours when it does not decode.
	if slices.Contains(both, schema.Path) {
		if err := mergeFile(schema.Path); err != nil {
			return nil, err
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
				return nil, err
			}
		}
	}
	// Both sides removed a file: ours already lacks it, so there is nothing
	// to remove.
	if err := forgetAbsent(r, ht, files); err != nil {
		return nil, err
	}
	snap, err := newSnapshot(r, ht, "", files)
	if err != nil {
		return nil, err
	}
	baseSnap, err := newSnapshot(r, bt, "", nil)
	if err != nil {
		return nil, err
	}
	originHead := opt.origin
	if originHead.IsZero() {
		if originHead, err = r.RemoteHead("origin"); err != nil {
			return nil, err
		}
	}
	onOrigin, err := originIDsAt(r, originHead)
	if err != nil {
		return nil, err
	}
	var authorErr error
	fixed, repaired, dropped, err := merge.Repair(merge.RepairInput{
		Files:    snap,
		Base:     baseSnap,
		OnOrigin: onOrigin,
		Changed:  func(p string) bool { return changed[p] },
		Author:   keepError(&authorErr, mergeAuthor(r, base, ours, theirs)),
		Now:      opt.Now().Truncate(time.Second),
	})
	if err = errors.Join(err, authorErr); err != nil {
		return nil, err
	}
	maps.Copy(files, fixed)
	lost = append(lost, dropped...)
	// A file the repair removed was not revived for good.
	notices = slices.DeleteFunc(notices, func(n merge.Notice) bool {
		return slices.ContainsFunc(dropped, func(d merge.Overwritten) bool { return d.Path == n.Path })
	})
	// A file the repair removes may not be in ours: it came from theirs.
	if err := forgetAbsent(r, ht, files); err != nil {
		return nil, err
	}
	return &merged{files: files, lost: lost, notices: notices, repaired: repaired}, nil
}

// schemaAt is the schema of tree, or the default one when the tree has
// none or it does not decode (the store reports that on its next write).
func schemaAt(r *gitrepo.Repo, tree plumbing.Hash) (*schema.Schema, error) {
	b, err := r.ReadFile(tree, schema.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return schema.Default(), nil
	}
	if err != nil {
		return nil, err
	}
	sc, err := schema.Decode(b)
	if err != nil {
		return schema.Default(), nil //nolint:nilerr // a broken schema.yaml is merged as a file; the merge goes on
	}
	return sc, nil
}

// forgetAbsent drops from files the removals of files that tree does not
// have: there is nothing to remove.
func forgetAbsent(r *gitrepo.Repo, tree plumbing.Hash, files map[string][]byte) error {
	for p, data := range files {
		if data != nil {
			continue
		}
		if _, err := r.ReadFile(tree, p); errors.Is(err, fs.ErrNotExist) {
			delete(files, p)
		} else if err != nil {
			return err
		}
	}
	return nil
}

func overwrittenTrailers(os []merge.Overwritten) []gitrepo.Trailer {
	out := make([]gitrepo.Trailer, len(os))
	for i, o := range os {
		out[i] = gitrepo.Trailer{Key: "Hedgehug-Overwritten", Value: strings.Join([]string{url.PathEscape(o.Path), fieldEscaper.Replace(o.Field), o.Value, orDash(o.Author)}, " ")}
	}
	return out
}

func repairedTrailers(rs []merge.Repaired) []gitrepo.Trailer {
	out := make([]gitrepo.Trailer, len(rs))
	for i, x := range rs {
		out[i] = gitrepo.Trailer{Key: "Hedgehug-Repaired", Value: strings.Join([]string{x.ULID.String(), x.Kind, x.Value, orDash(x.Author)}, " ")}
	}
	return out
}

// fieldEscaper keeps a field one word of a trailer: a yaml key or a prop name
// can hold spaces (statuses.In Progress.color). Readers url.PathUnescape it.
var fieldEscaper = strings.NewReplacer("%", "%25", " ", "%20")

// orDash keeps an empty author a word of its own in a trailer.
func orDash(author string) string {
	if author == "" {
		return "-"
	}
	return author
}

// fileULID is the ULID a file of the store is named by. A file not named by
// one, such as schema.yaml or a template, is given by its path.
func fileULID(p string) string {
	if s, ok := strings.CutSuffix(path.Base(p), ".md"); ok {
		if u, err := model.ParseULID(s); err == nil {
			return u.String()
		}
	}
	return p
}

// keepError adapts a lookup that can fail to the Author of merge.Repair,
// keeping the first error in *errp.
func keepError(errp *error, f func(string) (string, error)) func(string) string {
	return func(p string) string {
		a, err := f(p)
		if err != nil && *errp == nil {
			*errp = err
		}
		return a
	}
}

// mergeAuthor tells who last changed a file for a merge of head and remote
// over base: the side that changed it, the later one when both did (then
// the larger author, as the merge decides), or, when neither did, whoever
// changed it last before.
func mergeAuthor(r *gitrepo.Repo, base, head, remote plumbing.Hash) func(string) (string, error) {
	return func(p string) (string, error) {
		o, okO, err := r.LastChange(head, base, p)
		if err != nil {
			return "", err
		}
		t, okT, err := r.LastChange(remote, base, p)
		if err != nil {
			return "", err
		}
		switch {
		case okO && okT:
			if t.When.After(o.When) || t.When.Equal(o.When) && t.Writer() > o.Writer() {
				return t.Writer(), nil
			}
			return o.Writer(), nil
		case okO:
			return o.Writer(), nil
		case okT:
			return t.Writer(), nil
		}
		return headAuthor(r, head)(p)
	}
}

// headAuthor tells who last changed a file in the history of head.
func headAuthor(r *gitrepo.Repo, head plumbing.Hash) func(string) (string, error) {
	return func(p string) (string, error) {
		e, ok, err := r.LastChange(head, plumbing.ZeroHash, p)
		if err != nil || !ok {
			return "", err
		}
		return e.Writer(), nil
	}
}

// originIDs answers whether the main of origin, as last fetched, has the
// task ulid with the id.
func originIDs(r *gitrepo.Repo) (func(model.ULID, string) bool, error) {
	h, err := r.RemoteHead("origin")
	if err != nil {
		return nil, err
	}
	return originIDsAt(r, h)
}

// originIDsAt is originIDs for the commit h taken as the main of origin.
func originIDsAt(r *gitrepo.Repo, h plumbing.Hash) (func(model.ULID, string) bool, error) {
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

// dirty returns ErrDirty when one of paths has changes git has not
// committed.
func dirty(r *gitrepo.Repo, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	st, err := r.Status()
	if err != nil {
		return err
	}
	changed := map[string]bool{}
	for _, f := range st {
		changed[f.Path] = true
	}
	for _, p := range paths {
		if changed[p] {
			return ErrDirty
		}
	}
	return nil
}

// RepairHead repairs the files of HEAD as a sync does, without fetching or
// pushing: what a merge made by hand or a file edited by hand left. It
// returns what it fixed and what it only tells of. A store with no remote
// has no task on origin, so a duplicate id is renumbered by the order of
// the ULIDs.
func RepairHead(s *store.Store, opt Options) ([]merge.Repaired, error) {
	ix, err := index.Open(s)
	if err != nil {
		return nil, err
	}
	var res Result
	if err := repairHead(s, ix, opt, &res); err != nil {
		return nil, err
	}
	return res.Repaired, nil
}

// repairHead repairs the files of HEAD, for what a merge made by hand or a
// file edited by hand brought in, and commits what it fixed. It reads every
// file only when the index has a problem the repair fixes; otherwise it
// reads the workspaces alone, for the prefixes and names two of them share.
// What it only tells of goes to res without a commit, once per run; the
// inbox drops what it told before.
func repairHead(s *store.Store, ix *index.Index, opt Options, res *Result) error {
	r := s.Repo()
	for try := 1; ; try++ {
		head, err := r.Head()
		if err != nil {
			return err
		}
		tree, err := r.TreeOf(head)
		if err != nil {
			return err
		}
		full, err := needsRepair(ix, r, tree)
		if err != nil {
			return err
		}
		prefix := "workspaces/"
		if full {
			onFullRepair()
			prefix = ""
		}
		snap, err := newSnapshot(r, tree, prefix, nil)
		if err != nil {
			return err
		}
		onOrigin, err := originIDs(r)
		if err != nil {
			return err
		}
		var authorErr error
		fixed, repaired, dropped, err := merge.Repair(merge.RepairInput{
			Files:    snap,
			OnOrigin: onOrigin,
			Changed:  func(string) bool { return true },
			Author:   keepError(&authorErr, headAuthor(r, head)),
			Now:      opt.Now().Truncate(time.Second),
		})
		if err = errors.Join(err, authorErr); err != nil {
			return err
		}
		if len(fixed) == 0 {
			for _, x := range repaired {
				addNotice(res, x)
			}
			return nil
		}
		if err := dirty(r, slices.Collect(maps.Keys(fixed))); err != nil {
			return err
		}
		op := store.Op{Name: "repair", Detail: fmt.Sprintf("%d issues", len(repaired)), Author: opt.Author, Account: opt.Account}
		beforeCommit()
		_, err = s.CommitMerge(head, []plumbing.Hash{head}, fixed, op, append(overwrittenTrailers(dropped), repairedTrailers(repaired)...))
		if errors.Is(err, store.ErrHeadMoved) && try < tries {
			continue
		}
		if errors.Is(err, gitrepo.ErrWorkTreeChanged) {
			return ErrDirty
		}
		if err != nil {
			return err
		}
		res.Repaired = append(res.Repaired, repaired...)
		res.Overwritten = append(res.Overwritten, dropped...)
		return nil
	}
}

// repairCodes are the problems of the index that the repair of HEAD fixes:
// ids two tasks share, ids missing or not of the workspace's prefix, and
// children in another workspace than their parent. Cycles and missing
// tasks count only as parents.
var repairCodes = []string{index.CodeDuplicateID, index.CodeNoID, index.CodeIDPrefix, store.CodeWorkspaceMismatch}

// needsRepair reports whether the tasks of HEAD may need the repair: the
// index has a problem it fixes, or a forward has comments or worklogs under
// it. tree is HEAD's.
func needsRepair(ix *index.Index, r *gitrepo.Repo, tree plumbing.Hash) (bool, error) {
	if err := ix.Refresh(); err != nil {
		return false, err
	}
	if len(ix.Forwards()) > 0 {
		under, err := filesUnderForwards(ix, r, tree)
		if err != nil || under {
			return under, err
		}
	}
	for _, e := range ix.Tasks() {
		for _, p := range ix.Problems(e.ULID) {
			parent := p.Field == "parent" && (p.Code == schema.CodeCycle || p.Code == schema.CodeMissingRef || p.Code == index.CodeParentDeleted)
			moved := p.Code == schema.CodeMissingRef && pointsAtForward(ix, e, p.Field)
			if parent || moved || slices.Contains(repairCodes, p.Code) {
				return true, nil
			}
		}
	}
	return false, nil
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
func prefixDuplicates(s *store.Store, opt Options, res *Result) error {
	head, err := s.Repo().Head()
	if err != nil {
		return err
	}
	who := headAuthor(s.Repo(), head)
	var mine []*model.Workspace
	var prefixes []string
	for m, err := range s.Workspaces() {
		if err == nil && m.IDPrefix != "" {
			mine = append(mine, m)
			prefixes = append(prefixes, m.IDPrefix)
		}
	}
	if len(mine) == 0 {
		return nil
	}
	uses := store.PrefixUses(opt.Others, prefixes, nil) // each other store opens once
	for _, m := range mine {
		for _, u := range uses {
			w := u.Workspace
			if !strings.EqualFold(m.IDPrefix, w.IDPrefix) {
				continue
			}
			if c := m.PrefixSet.Compare(w.PrefixSet); c < 0 || c == 0 && m.ULID.String() < w.ULID.String() {
				continue
			}
			a, err := who(model.WorkspacePath(m.ULID))
			if err != nil {
				return err
			}
			v, _ := json.Marshal(struct {
				Store     string `json:"store"`
				Workspace string `json:"workspace"`
			}{u.Store, w.Name}) // strings always encode
			addNotice(res, merge.Repaired{ULID: m.ULID, Kind: "prefix_duplicate", Value: string(v), Author: a})
		}
	}
	return nil
}

// notify adds to the inbox what reached HEAD for opt.Me since the inbox
// last caught up (since before, the first time), the merge driver's
// records, and the duplicate prefixes and names the sync told of without a
// commit (the inbox drops those it has). What the inbox skipped goes to the
// warnings.
func notify(s *store.Store, before plumbing.Hash, opt Options, res *Result) error {
	box := inbox.Open(s)
	defer func() { res.Warnings = append(res.Warnings, box.Warnings()...) }()
	added, err := box.Catch(opt.Me, before)
	res.Notices = append(res.Notices, added...)
	if err != nil {
		return err
	}
	head, err := s.Repo().Head()
	if err != nil {
		return err
	}
	var items []inbox.Item
	now := opt.Now()
	for _, x := range res.Repaired {
		if x.Kind == "prefix_duplicate" || x.Kind == "name_duplicate" {
			// Account IDs are compared without case (spec: 共通の決まり, 大文字小文字); sync does not import config, so strings.EqualFold, like config.SameAccount.
			if it, ok := inbox.Notice(x.ULID, x.Kind, x.Value, x.Author, head.String(), now); ok && slices.ContainsFunc(opt.Me, func(m string) bool { return strings.EqualFold(m, x.Author) }) {
				items = append(items, it)
			}
		}
	}
	added, err = box.Add(items)
	res.Notices = append(res.Notices, added...)
	return err
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

// snapshot is the files under a prefix of a tree, with some laid over it
// (nil removes).
type snapshot struct {
	r     *gitrepo.Repo
	tree  plumbing.Hash
	over  map[string][]byte
	paths []string // sorted
}

func newSnapshot(r *gitrepo.Repo, tree plumbing.Hash, prefix string, over map[string][]byte) (*snapshot, error) {
	s := &snapshot{r: r, tree: tree, over: over}
	for p, err := range r.Files(tree, prefix) {
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

// Unpushed counts the commits of HEAD that origin did not have as of the last
// fetch or push. A store that never pushed has all of them. The commands that
// show it call this too.
func Unpushed(r *gitrepo.Repo) (int, error) {
	head, err := r.Head()
	if err != nil {
		return 0, err
	}
	remote, err := r.RemoteHead("origin")
	if err != nil {
		return 0, err
	}
	es, err := r.Between(head, remote)
	return len(es), err
}

// readOnlyError says a store is read-only. It holds the push error and the
// failure to write access.json, if any, for errors.Is, without repeating
// them in its text.
type readOnlyError struct {
	msg  string
	errs []error
}

func (e *readOnlyError) Error() string   { return e.msg }
func (e *readOnlyError) Unwrap() []error { return e.errs }

// storeName is the name for messages: the store's, or "the store" when none
// is given.
func (o Options) storeName() string {
	if o.Name == "" {
		return "the store"
	}
	return o.Name
}

// pointsAtForward reports whether the reference of e through field (parent,
// blocked_by or a task_ref property) names a forward.
func pointsAtForward(ix *index.Index, e *index.Entry, field string) bool {
	targets := e.Targets(field)
	if field == "parent" && e.Parent != nil {
		targets = []model.ULID{*e.Parent}
	}
	return slices.ContainsFunc(targets, func(u model.ULID) bool { _, ok := ix.Forward(u); return ok })
}

// filesUnderForwards reports whether tree has a comment or worklog file of a
// task that is a forward.
func filesUnderForwards(ix *index.Index, r *gitrepo.Repo, tree plumbing.Hash) (bool, error) {
	parse := map[string]func(string) (model.ULID, model.ULID, error){
		"comments/": model.ParseCommentPath,
		"worklogs/": model.ParseWorklogPath,
	}
	for prefix, parseFn := range parse {
		for p, err := range r.Files(tree, prefix) {
			if err != nil {
				return false, err
			}
			task, _, perr := parseFn(p)
			if _, ok := ix.Forward(task); perr == nil && ok {
				return true, nil
			}
		}
	}
	return false, nil
}
