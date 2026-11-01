package context

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/mount"
	"github.com/kukv/hedgehug/internal/store"
)

// ProjectKind is the kind of a task that hh use and the repo stage take.
const ProjectKind = "project"

// UseGoneWarning is the warning of a project hh use set that is gone (D12).
const UseGoneWarning = "the project of hedgehug use is not found (hedgehug use --clear removes it)"

// Input is what Resolve decides the destination from.
type Input struct {
	Dir                               string // the current directory; "" skips stages 3 and 4
	Store, Project, Parent, Workspace string // --store, --project, --parent, --workspace
	DefaultStore                      string // config's default_store
	Interactive                       bool   // stdin is a terminal: stages 3–5 may ask
	// The answers to what an earlier call asked; zero when not asked.
	BranchParent  Answer // stage 3
	PickProject   string // stage 4: the store-qualified id picked
	PickStore     string // stage 5
	PickWorkspace string // stage 5
}

// Answer is the answer to a question of stage 3.
type Answer int

const (
	Unasked Answer = iota
	Yes
	No
)

// Destination is where the new task goes.
type Destination struct {
	Store     *mount.Mounted
	Parent    *index.Entry // nil at a workspace's root; a copy as it was read
	Workspace *model.Workspace
	Stage     int    // 1–5, the stage that decided
	RepoID    string // the repo ID of the current repository, "" when none (stage 4's rule)
	Recent    bool   // stage 4 picked a project: recent[RepoID] is to be set to it
}

// Confirm is what stage 3 asks: whether to put the task under Parent.
type Confirm struct {
	Store  *mount.Mounted
	Parent *index.Entry
}

// Choice is what stages 4 and 5 ask: one of Options, Default ("" when none).
type Choice struct {
	What    string   // "project", "store" or "workspace"
	Options []string // store-qualified ids, store names or workspace names
	Labels  []string // what to show with each option (a project's title); nil when the options say enough
	Default string
}

// Kind is what an Error means; ops turns it into its own Kind.
type Kind int

const (
	KindUsage Kind = iota + 1
	KindInvalid
	KindAmbiguous
	KindConflict
)

// Error is a refusal of Resolve. Code and Args are those of the ops.Error it
// becomes.
type Error struct {
	Kind Kind
	Code string
	Err  error
	Args map[string]any
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

func newError(k Kind, code string, err error, args ...any) *Error {
	e := &Error{Kind: k, Code: code, Err: err}
	if len(args) > 0 {
		e.Args = map[string]any{}
		for i := 0; i+1 < len(args); i += 2 {
			e.Args[args[i].(string)] = args[i+1]
		}
	}
	return e
}

// LookupError is a task or a workspace the user named that was not found, or
// matched more than one: Err is what mount.Set.Resolve or the index said, for
// the caller to spell out. For a task, Input is what was given to Set.Resolve.
// For a workspace, Store is the store it was looked up in, or nil when it was
// looked up in every store.
type LookupError struct {
	What  string // "task" or "workspace"
	Input string
	Store *mount.Mounted
	Err   error
}

func (e *LookupError) Error() string { return e.Err.Error() }
func (e *LookupError) Unwrap() error { return e.Err }

// result is what a stage decided, asked or left alone.
type result struct {
	dest    *Destination
	confirm *Confirm
	choice  *Choice
}

func (r result) decided() bool { return r.dest != nil || r.confirm != nil || r.choice != nil }

type resolver struct {
	set  *mount.Set
	st   State
	in   Input
	warn func(string)
}

// Resolve decides the destination by the five stages of the spec. When a
// stage needs an answer it returns (nil, *Confirm, nil) or
// (nil, nil, *Choice) when Interactive; the caller asks and calls again with
// the answer. Not Interactive: stage 3 is skipped, stage 4 uses recent or is
// an error, stage 5 is an error, as the spec says.
func Resolve(set *mount.Set, st State, in Input, warn func(string)) (*Destination, *Confirm, *Choice, error) {
	r := &resolver{set: set, st: st, in: in, warn: warn}
	if warn == nil {
		r.warn = func(string) {}
	}
	var repo *git.Repository
	if in.Dir != "" {
		repo = openRepo(in.Dir)
	}
	ids := repoIDsOf(repo)
	stages := []func(*git.Repository, []string) (result, error){r.explicit, r.use, r.branch, r.repo, r.defaults}
	for _, stage := range stages {
		res, err := stage(repo, ids)
		if err != nil {
			return nil, nil, nil, err
		}
		if res.decided() {
			if res.dest != nil && len(ids) > 0 {
				res.dest.RepoID = ids[0]
			}
			return res.dest, res.confirm, res.choice, nil
		}
	}
	// stage 5 always decides or fails
	return nil, nil, nil, errors.New("no stage decided the destination")
}

// openRepo opens the git repository at dir or above it; nil when there is none.
func openRepo(dir string) *git.Repository {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil
	}
	return repo
}

// repoIDsOf is the repo IDs of the remotes of repo: origin alone when there
// is one, else every remote by name. The URLs are as .git/config has them,
// before url_rewrites. The first is the repo ID of the repository.
func repoIDsOf(repo *git.Repository) []string {
	if repo == nil {
		return nil
	}
	cfg, err := repo.Config()
	if err != nil || cfg.Raw == nil {
		return nil
	}
	urls := map[string][]string{}
	for _, sub := range cfg.Raw.Section("remote").Subsections {
		urls[sub.Name] = sub.Options.GetAll("url")
	}
	var names []string
	if _, ok := urls["origin"]; ok {
		names = []string{"origin"}
	} else {
		for n := range urls {
			names = append(names, n)
		}
		sort.Strings(names)
	}
	var ids []string
	for _, n := range names {
		for _, u := range urls[n] {
			if id, ok := RepoID(u); ok && !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// explicit is stage 1.
func (r *resolver) explicit(*git.Repository, []string) (result, error) {
	in := r.in
	if in.Store == "" && in.Project == "" && in.Parent == "" && in.Workspace == "" {
		return result{}, nil
	}
	if in.Parent != "" && in.Project != "" {
		return result{}, newError(KindUsage, "add.parent_and_project", errors.New("--parent and --project cannot be given together"))
	}
	if arg := cmp.Or(in.Parent, in.Project); arg != "" {
		m, e, err := r.findTask(arg)
		if err != nil {
			return result{}, err
		}
		if in.Project != "" && e.Kind != ProjectKind {
			label := m.Name + ":" + idOrULID(e)
			return result{}, newError(KindInvalid, "add.not_project",
				fmt.Errorf("%s is a %s, not a project", label, e.Kind), "id", label, "kind", e.Kind)
		}
		d, err := parentDest(m, e, 1)
		return result{dest: d}, err
	}
	if in.Store == "" {
		d, err := r.workspaceAnywhere(in.Workspace)
		return result{dest: d}, err
	}
	m, err := r.mounted(in.Store)
	if err != nil {
		return result{}, err
	}
	d, ch, err := r.root(m, in.Workspace, 1)
	return result{dest: d, choice: ch}, err
}

func idOrULID(e *index.Entry) string {
	if e.ID != "" {
		return e.ID
	}
	return e.ULID.String()
}

// mounted is the store name; one that is set up but did not open says why.
func (r *resolver) mounted(name string) (*mount.Mounted, error) {
	if m, ok := r.set.Get(name); ok {
		return m, nil
	}
	for _, sk := range r.set.Skipped {
		if sk.Name == name {
			return nil, fmt.Errorf("cannot open %s: %w", name, sk.Err)
		}
	}
	return nil, fmt.Errorf("no store is named %q", name)
}

// findTask finds the task arg names, in the store --store names, or in every
// store; a copy of its entry is returned.
func (r *resolver) findTask(arg string) (*mount.Mounted, *index.Entry, error) {
	q := arg
	if s := r.in.Store; s != "" {
		if name, rest, ok := strings.Cut(arg, ":"); ok && r.hasStore(name) {
			if name != s {
				return nil, nil, newError(KindUsage, "store.conflict",
					fmt.Errorf("%q is in %s, but --store is %s", arg, name, s), "store", s, "named", name)
			}
			q = rest
		}
		q = s + ":" + q
	}
	hit, err := r.set.Resolve(q)
	if err != nil {
		return nil, nil, &LookupError{What: "task", Input: q, Err: err}
	}
	var cp index.Entry
	var found bool
	if err := hit.Store.With(func(ix *index.Index) {
		if e, ok := ix.Task(hit.Entry.ULID); ok {
			cp, found = *e, true
		}
	}); err != nil {
		return nil, nil, err
	}
	if !found {
		cp = *hit.Entry
	}
	return hit.Store, &cp, nil
}

func (r *resolver) hasStore(name string) bool {
	_, ok := r.set.Get(name)
	return ok
}

// parentDest is the destination under the entry e of m.
func parentDest(m *mount.Mounted, e *index.Entry, stage int) (*Destination, error) {
	var w *model.Workspace
	if err := m.With(func(ix *index.Index) {
		if found, ok := ix.Workspace(e.Workspace); ok {
			cp := *found
			w = &cp
		}
	}); err != nil {
		return nil, err
	}
	if w == nil {
		// the parent sits in a workspace the store lacks: the write says so
		w = &model.Workspace{ULID: e.Workspace}
	}
	return &Destination{Store: m, Parent: e, Workspace: w, Stage: stage}, nil
}

// workspaceAnywhere is the root of the workspace arg names in whichever store
// has it; more than one store is an error.
func (r *resolver) workspaceAnywhere(arg string) (*Destination, error) {
	type found struct {
		m *mount.Mounted
		w model.Workspace
	}
	var hits []found
	for _, m := range r.set.Stores {
		var w *model.Workspace
		var rerr error
		if err := m.With(func(ix *index.Index) {
			var f *model.Workspace
			if f, rerr = ix.ResolveWorkspace(arg); rerr == nil {
				cp := *f
				w = &cp
			}
		}); err != nil {
			return nil, err
		}
		var ae *index.AmbiguousWorkspaceError
		switch {
		case w != nil:
			hits = append(hits, found{m, *w})
		case errors.As(rerr, &ae):
			return nil, &LookupError{What: "workspace", Input: arg, Store: m, Err: rerr}
		}
	}
	switch len(hits) {
	case 0:
		return nil, &LookupError{What: "workspace", Input: arg, Err: fmt.Errorf("workspace %q: %w", arg, store.ErrNotFound)}
	case 1:
		return &Destination{Store: hits[0].m, Workspace: &hits[0].w, Stage: 1}, nil
	}
	lines := make([]string, len(hits))
	for i, h := range hits {
		lines[i] = fmt.Sprintf("%s:%s (%s, %s)", h.m.Name, h.w.Name, h.w.IDPrefix, h.w.ULID)
	}
	return nil, newError(KindAmbiguous, "workspace.ambiguous",
		fmt.Errorf("%q matches %d workspaces:\n  %s", arg, len(hits), strings.Join(lines, "\n  ")), "input", arg)
}

// root is the root of a workspace of m: the one wsArg names, else the only
// one, else the one PickWorkspace names, else a Choice (or an error when
// not Interactive).
func (r *resolver) root(m *mount.Mounted, wsArg string, stage int) (*Destination, *Choice, error) {
	var w *model.Workspace
	var ch *Choice
	var err error
	if werr := m.With(func(ix *index.Index) {
		want := cmp.Or(wsArg, r.in.PickWorkspace)
		if want != "" {
			f, rerr := ix.ResolveWorkspace(want)
			if rerr != nil {
				err = &LookupError{What: "workspace", Input: want, Store: m, Err: rerr}
				return
			}
			cp := *f
			w = &cp
			return
		}
		wss := ix.Workspaces()
		switch len(wss) {
		case 0:
			err = newError(KindConflict, "workspace.none",
				errors.New("the store has no workspace; add one with hedgehug workspace add <name> --prefix <PREFIX>"))
		case 1:
			cp := *wss[0]
			w = &cp
		default:
			names := make([]string, len(wss))
			for i, f := range wss {
				names[i] = f.Name
			}
			if r.in.Interactive {
				ch = &Choice{What: "workspace", Options: names}
				return
			}
			err = newError(KindUsage, "choice.required",
				fmt.Errorf("workspace is needed; give one with --workspace: %s", strings.Join(names, ", ")),
				"what", "workspace", "candidates", names)
		}
	}); werr != nil {
		return nil, nil, werr
	}
	if err != nil || ch != nil {
		return nil, ch, err
	}
	return &Destination{Store: m, Workspace: w, Stage: stage}, nil, nil
}

// use is stage 2.
func (r *resolver) use(*git.Repository, []string) (result, error) {
	if r.st.Use == nil {
		return result{}, nil
	}
	m, e, _, ok, err := r.st.Use.Find(r.set)
	if err != nil {
		return result{}, err
	}
	if !ok {
		r.warn(UseGoneWarning)
		return result{}, nil
	}
	d, err := parentDest(m, &e, 2)
	return result{dest: d}, err
}

// Find is the project ref names, if it is there: its store's name must be
// mounted, and the task live and of kind project. id is the project's id as
// its store shows it. ok is false for one that is gone.
func (ref Ref) Find(set *mount.Set) (m *mount.Mounted, e index.Entry, id string, ok bool, err error) {
	for _, c := range set.Stores {
		if c.ULID == ref.Store {
			m = c
			break
		}
	}
	if m == nil {
		return nil, index.Entry{}, "", false, nil
	}
	err = m.With(func(ix *index.Index) {
		if t, found := ix.Task(ref.Project); found && t.DeletedAt == nil && t.Kind == ProjectKind {
			e, id, ok = *t, ix.DisplayID(t), true
		}
	})
	return m, e, id, ok, err
}

// branchPattern reads hh/<ID>-… and hedgehug/<ID>-…: <ID> is a prefix, "-" and
// digits, and the digits are followed by "-" or the end.
var branchPattern = regexp.MustCompile(`^(?:hh|hedgehug)/([A-Za-z][A-Za-z0-9]*-[0-9]+)(?:-|$)`)

// branch is stage 3.
func (r *resolver) branch(repo *git.Repository, _ []string) (result, error) {
	if repo == nil || !r.in.Interactive || r.in.BranchParent == No {
		return result{}, nil
	}
	head, err := repo.Storer.Reference(plumbing.HEAD)
	if err != nil || head.Type() != plumbing.SymbolicReference || !head.Target().IsBranch() {
		return result{}, nil
	}
	mm := branchPattern.FindStringSubmatch(strings.TrimPrefix(head.Target().String(), "refs/heads/"))
	if mm == nil {
		return result{}, nil
	}
	hit, err := r.set.Resolve(mm[1])
	if err != nil {
		return result{}, nil // not a task of ours: the stage is skipped
	}
	m, e, err := r.live(hit)
	if err != nil || e == nil {
		return result{}, err
	}
	if r.in.BranchParent == Yes {
		d, err := parentDest(m, e, 3)
		return result{dest: d}, err
	}
	return result{confirm: &Confirm{Store: m, Parent: e}}, nil
}

// live is the entry of hit as its store has it now, or nil when it is
// deleted.
func (r *resolver) live(hit mount.Hit) (*mount.Mounted, *index.Entry, error) {
	var cp *index.Entry
	if err := hit.Store.With(func(ix *index.Index) {
		if e, ok := ix.Task(hit.Entry.ULID); ok && e.DeletedAt == nil {
			c := *e
			cp = &c
		}
	}); err != nil {
		return nil, nil, err
	}
	return hit.Store, cp, nil
}

// candidate is a project that has the repository in repos.
type candidate struct {
	m     *mount.Mounted
	e     index.Entry
	ref   string // "<store>:<id>"
	title string
}

// repo is stage 4.
func (r *resolver) repo(_ *git.Repository, ids []string) (result, error) {
	if len(ids) == 0 {
		return result{}, nil
	}
	cands, err := r.projectsOf(ids)
	if err != nil || len(cands) == 0 {
		return result{}, err
	}
	pick := func(c candidate) (result, error) {
		d, err := parentDest(c.m, &c.e, 4)
		if d != nil {
			d.Recent = true
		}
		return result{dest: d}, err
	}
	if len(cands) == 1 {
		return pick(cands[0])
	}
	options := make([]string, len(cands))
	labels := make([]string, len(cands))
	def := ""
	for i, c := range cands {
		options[i], labels[i] = c.ref, c.title
		// recent is keyed by the repo ID of the repository: with no origin and
		// several remotes, that of the first remote by name (the spec's repo-link rule)
		if rc, ok := r.st.Recent[ids[0]]; ok && rc.Store == c.m.ULID && rc.Project == c.e.ULID {
			def = c.ref
		}
	}
	if r.in.PickProject != "" {
		for _, c := range cands {
			if strings.EqualFold(c.ref, r.in.PickProject) {
				return pick(c)
			}
		}
		return result{}, newError(KindUsage, "choice.invalid",
			fmt.Errorf("%q is not one of: %s", r.in.PickProject, strings.Join(options, ", ")), "what", "project", "candidates", options)
	}
	if r.in.Interactive {
		return result{choice: &Choice{What: "project", Options: options, Labels: labels, Default: def}}, nil
	}
	if def != "" {
		for _, c := range cands {
			if c.ref == def {
				return pick(c)
			}
		}
	}
	return result{}, newError(KindUsage, "choice.required",
		fmt.Errorf("project is needed; give one with --project: %s", strings.Join(options, ", ")),
		"what", "project", "candidates", options)
}

// projectsOf is the projects, not deleted, in every store that have one of
// ids in repos, by store and then by ULID.
func (r *resolver) projectsOf(ids []string) ([]candidate, error) {
	var out []candidate
	for _, m := range r.set.Stores {
		var found []candidate
		if err := m.With(func(ix *index.Index) {
			for _, e := range ix.Tasks() {
				if e.Kind != ProjectKind || e.DeletedAt != nil || e.Broken != "" {
					continue
				}
				t, err := m.Store.Task(e.ULID)
				if err != nil || !hasRepo(t.Repos, ids) {
					continue
				}
				found = append(found, candidate{m: m, e: *e, ref: m.Name + ":" + idOrULID(e), title: e.Title})
			}
		}); err != nil {
			return nil, err
		}
		sort.Slice(found, func(i, j int) bool { return found[i].e.ULID.String() < found[j].e.ULID.String() })
		out = append(out, found...)
	}
	return out, nil
}

func hasRepo(repos, ids []string) bool {
	for _, r := range repos {
		for _, id := range ids {
			if strings.EqualFold(r, id) {
				return true
			}
		}
	}
	return false
}

// defaults is stage 5.
func (r *resolver) defaults(*git.Repository, []string) (result, error) {
	var m *mount.Mounted
	var err error
	switch stores := r.set.Stores; {
	case r.in.DefaultStore != "":
		m, err = r.mounted(r.in.DefaultStore)
	case len(stores) == 0:
		err = config.ErrNoStores
	case len(stores) == 1:
		m = stores[0]
	case r.in.PickStore != "":
		m, err = r.mounted(r.in.PickStore)
	default:
		names := make([]string, len(stores))
		for i, s := range stores {
			names[i] = s.Name
		}
		if r.in.Interactive {
			return result{choice: &Choice{What: "store", Options: names}}, nil
		}
		err = newError(KindUsage, "choice.required",
			fmt.Errorf("store is needed; give one with --store or set one with hedgehug store default: %s", strings.Join(names, ", ")),
			"what", "store", "candidates", names)
	}
	if err != nil {
		return result{}, err
	}
	d, ch, err := r.root(m, "", 5)
	return result{dest: d, choice: ch}, err
}
