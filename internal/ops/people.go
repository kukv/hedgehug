package ops

import (
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/mount"
)

// People are the names the GUI offers after an @: the assignees, watchers and
// comment authors of the tasks, and the Hedgehug-Author of the commits. They
// are those of the store the session selects, or of every store; sorted, each
// once. A name that cannot be written as a mention (@login or @host/login,
// as index.Mentions reads it) is left out: the GUI writes the name after an @.
// Names are the same account when they differ only in case, as in
// index.MentionNames; the one that sorts first is kept.
func (ss *Session) People() ([]string, error) {
	set, err := ss.listSet()
	if err != nil {
		return nil, err
	}
	stores := set.Stores
	if ss.StoreName != "" {
		m, _ := set.Get(ss.StoreName)
		stores = []*mount.Mounted{m}
	}
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && slices.Equal(index.Mentions("@"+name), []string{name}) {
			seen[name] = true
		}
	}
	for _, m := range stores {
		ix, err := m.Index()
		if err != nil {
			return nil, classify(err)
		}
		for _, e := range ix.Tasks() {
			for _, n := range e.Assignees {
				add(n)
			}
			for _, n := range e.Watchers {
				add(n)
			}
			for _, c := range ix.Comments(e.ULID) {
				add(c.Author)
			}
		}
		var entries []gitrepo.LogEntry
		var logErr error
		if err := m.With(func(*index.Index) { entries, logErr = m.Store.Repo().Log(nil, 0) }); err != nil {
			return nil, classify(err)
		}
		if logErr != nil {
			return nil, logErr
		}
		for _, en := range entries {
			a, _ := en.Trailer("Hedgehug-Author")
			add(a)
		}
	}
	out := []string{}
	folded := map[string]bool{}
	for _, n := range slices.Sorted(maps.Keys(seen)) {
		if k := strings.ToLower(n); !folded[k] {
			folded[k] = true
			out = append(out, n)
		}
	}
	return out, nil
}
