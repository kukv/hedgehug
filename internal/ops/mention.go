package ops

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// warnMentions warns of each account that after mentions and before did not,
// when the members of s are known and the account is none of them: it cannot
// read the store. It is called after a write. When the members cannot be
// read, nothing can be said.
func (ss *Session) warnMentions(s *store.Store, before, after string) {
	var fresh []string
	was := index.Mentions(before)
	for _, m := range index.Mentions(after) {
		if !slices.ContainsFunc(was, func(w string) bool { return index.MentionNames(m, w) }) {
			fresh = append(fresh, m)
		}
	}
	if len(fresh) == 0 {
		return
	}
	m, err := sync.ReadMembers(s)
	if err != nil || m.Collaborators == nil {
		return
	}
	name := s.Meta().Name
	if c, _, err := ss.Config(); err == nil {
		for _, e := range c.Stores {
			if e.Path == s.Dir() {
				name = e.Name
			}
		}
	}
	var warned []string
	for _, men := range fresh {
		if slices.ContainsFunc(m.Collaborators, func(c sync.Collaborator) bool { return mentionIsCollaborator(men, c.ID) }) {
			continue
		}
		login := men[strings.LastIndex(men, "/")+1:]
		if slices.ContainsFunc(warned, func(w string) bool { return strings.EqualFold(w, login) }) {
			continue
		}
		warned = append(warned, login)
		ss.warn(fmt.Sprintf("%s cannot read %s", login, name))
	}
}

// mentionIsCollaborator reports whether the mention names the collaborator
// with account ID id: @host/login that account, @login any with that login.
func mentionIsCollaborator(mention, id string) bool {
	if strings.Contains(mention, "/") {
		return config.SameAccount(mention, id)
	}
	return strings.EqualFold(mention, id[strings.LastIndex(id, "/")+1:])
}
