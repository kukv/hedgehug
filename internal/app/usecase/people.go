package usecase

import (
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/mount"
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
			return nil, Classify(err)
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
		var authors []string
		var histErr error
		if err := m.With(func(*index.Index) {
			var h *history.History
			if h, histErr = ss.OpenHistory(m.Store, m.Name); histErr == nil {
				authors = h.Authors()
			}
		}); err != nil {
			return nil, Classify(err)
		}
		if histErr != nil {
			return nil, histErr
		}
		for _, a := range authors {
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
