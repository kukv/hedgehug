package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/merge"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// mergeDriverCmd is the command .git/config names for the files
// .gitattributes sends to the hedgehug driver. git runs it in the work tree,
// in the middle of a merge, so it never starts a background sync.
func (a *app) mergeDriverCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "merge-driver <base> <ours> <theirs> <path>",
		Short:  "The git merge driver of the store's files",
		Hidden: true,
		Args:   exactArgs(4),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.mergeDriver(args[0], args[1], args[2], args[3])
		},
	}
}

// mergeDriver merges one file of a merge git runs: base, ours and theirs are
// the files git made of its versions, and the result goes to ours. The times
// and authors of the two sides come from the history of HEAD and of the
// commit being merged in (GITHEAD_*; see mergedIn); without one (a rebase, a
// cherry-pick) the sides carry none, and
// the merge goes by the files alone. The values that lose go to
// .hedgehug/local/overwritten/, for the next sync or hedgehug inbox. A merge
// of one file does not repair what holds across files; the next sync does.
func (a *app) mergeDriver(basePath, oursPath, theirsPath, rel string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	r, err := gitrepo.Open(dir)
	if err != nil {
		return err
	}
	var data [3][]byte
	for i, p := range []string{basePath, oursPath, theirsPath} {
		if data[i], err = os.ReadFile(p); err != nil { //nolint:gosec // the files git gives the driver
			return err
		}
	}
	head, err := r.Head()
	if err != nil {
		return err
	}
	theirs, hasMerge := mergedIn(os.Environ())
	sc, err := driverSchema(r, head)
	if err != nil {
		return err
	}
	var ours, other merge.Side
	// An empty base file is how git says the file has no common version
	// (both sides added it); the merge reads that as no base.
	baseData := data[0]
	if len(baseData) == 0 {
		baseData = nil
	}
	commit := ""
	if hasMerge {
		commit = theirs.String()
		base, err := r.MergeBase(head, theirs)
		if err != nil {
			return err
		}
		if sc, err = mergedSchema(r, base, head, theirs, sc, rel); err != nil {
			return err
		}
		if baseData, ours, other, err = sync.Sides(r, base, head, theirs, rel); err != nil {
			return err
		}
	}
	ours.Data, other.Data = data[1], data[2]
	m, err := merge.Merge(rel, sc, baseData, ours, other, merge.LWW)
	if err != nil {
		return fmt.Errorf("merge %s: %w", rel, err)
	}
	if m.Data == nil {
		return fmt.Errorf("merge %s: the merge removes the file; left to git", rel)
	}
	if err := os.WriteFile(oursPath, m.Data, 0o600); err != nil {
		return err
	}
	return a.recordLosses(dir, commit, m)
}

// driverSchema is the schema HEAD has, or the default one when it has none or
// it does not decode: a file that does not read is merged as a file.
func driverSchema(r *gitrepo.Repo, head plumbing.Hash) (*schema.Schema, error) {
	b, err := r.ReadAt(head, schema.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return schema.Default(), nil
	}
	if err != nil {
		return nil, err
	}
	sc, err := schema.Decode(b)
	if err != nil {
		return schema.Default(), nil //nolint:nilerr // see above
	}
	return sc, nil
}

// mergedSchema is the schema of the merge of head and theirs, for the merge
// of the file rel to read: head's when rel is schema.yaml itself, or the
// merge does not decode.
func mergedSchema(r *gitrepo.Repo, base, head, theirs plumbing.Hash, headSchema *schema.Schema, rel string) (*schema.Schema, error) {
	if rel == schema.Path {
		return headSchema, nil
	}
	b, o, t, err := sync.Sides(r, base, head, theirs, schema.Path)
	if err != nil {
		return nil, err
	}
	m, err := merge.Merge(schema.Path, headSchema, b, o, t, merge.LWW)
	if err != nil || m.Data == nil {
		return headSchema, nil //nolint:nilerr // a schema that does not merge is left to its own file
	}
	sc, err := schema.Decode(m.Data)
	if err != nil {
		return headSchema, nil //nolint:nilerr // see above
	}
	return sc, nil
}

// recordLosses writes what lost in m to .hedgehug/local/overwritten/, one
// file for each. commit is the one merged in, which the merge commit git
// makes will have as a parent: once the merge is aborted it is not in HEAD's
// history, and the inbox drops the record. It is empty when no commit is being merged in.
func (a *app) recordLosses(dir string, commit string, m merge.Result) error {
	now := a.now().UTC()
	var recs []inbox.Record
	for _, o := range m.Overwritten {
		recs = append(recs, inbox.Record{Path: o.Path, Field: o.Field, Value: o.Value, Author: o.Author, Commit: commit, Time: now})
	}
	for _, n := range m.Notices {
		recs = append(recs, inbox.Record{Kind: inbox.Kind(n.Kind), Path: n.Path, Author: n.Author, Commit: commit, Time: now})
	}
	for _, rec := range recs {
		b, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		if err := store.WriteLocalIn(dir, "overwritten/"+model.NewULID().String()+".json", append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// mergedIn finds the commit git merges into HEAD. git has not written
// MERGE_HEAD yet when it runs the driver; it names the commits being merged
// in the variables GITHEAD_<commit>. A merge of several commits, and any other
// command that runs the driver, has no one commit.
func mergedIn(environ []string) (plumbing.Hash, bool) {
	var found []plumbing.Hash
	for _, kv := range environ {
		k, _, _ := strings.Cut(kv, "=")
		if h, ok := strings.CutPrefix(k, "GITHEAD_"); ok && plumbing.IsHash(h) {
			found = append(found, plumbing.NewHash(h))
		}
	}
	if len(found) != 1 {
		return plumbing.ZeroHash, false
	}
	return found[0], true
}
