package usecase

import (
	"regexp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/lib/github"
)

var (
	branchMark = regexp.MustCompile(`^(?:hh|hedgehug)/([A-Za-z][A-Za-z0-9]*-[0-9]+)(?:-|$)`)
	ulidMark   = regexp.MustCompile(`\b(?:hedgehug|hh):([0-9A-Za-z]{26})\b`)
	idMark     = regexp.MustCompile(`(\b[A-Z][A-Z0-9]*-[0-9]+\b)`)
	// ownMark is the last line of a body hh pr wrote.
	ownMark = regexp.MustCompile(`^hedgehug:([0-9A-Za-z]{26})$`)
)

// found returns the last group of the matches of re in text, leaving out a
// match next to a hyphen: in PRE-WEB-7 the id is PRE-WEB-7, not WEB-7, and
// WEB-1-2 is not WEB-1.
// \b does not tell a hyphen from a space, and RE2 has no look-behind.
func found(re *regexp.Regexp, text string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
		if m[0] > 0 && text[m[0]-1] == '-' || m[1] < len(text) && text[m[1]] == '-' {
			continue
		}
		out = append(out, text[m[len(m)-2]:m[len(m)-1]])
	}
	return out
}

// markOf is the ULID of the mark on the last line of body, "" when that line
// is something else.
func markOf(body string) string {
	body = strings.TrimSpace(body)
	line := strings.TrimSpace(body[strings.LastIndexByte(body, '\n')+1:])
	if m := ownMark.FindStringSubmatch(line); m != nil {
		return strings.ToUpper(m[1])
	}
	return ""
}

// mentioned returns the ids and ULIDs by which p points at a task: its
// branch, and in its title, body and comments a mark or an id with one of
// the prefixes. A body that ends in hedgehug:<ULID> names its task there: the
// rest of it and the title are the text of that task, which may speak of
// others, and are not read. Each is there once.
func mentioned(p github.Pull, prefixes []string) []string {
	var out []string
	add := func(s string) {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	if m := branchMark.FindStringSubmatch(p.Head); m != nil {
		add(m[1])
	}
	texts := append([]string{p.Title, p.Body}, p.Comments...)
	if u := markOf(p.Body); u != "" {
		add(u)
		texts = p.Comments
	}
	for _, text := range texts {
		for _, u := range found(ulidMark, text) {
			add(strings.ToUpper(u))
		}
		for _, id := range found(idMark, text) {
			if i := strings.LastIndex(id, "-"); slices.Contains(prefixes, id[:i]) {
				add(id)
			}
		}
	}
	return out
}

// prefixesOf returns the id prefixes of the store of ix: those of its
// workspaces, and those its tasks had, as ids and as former ids.
func prefixesOf(ix *index.Index) []string {
	var out []string
	add := func(p string) {
		if p != "" && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	for _, w := range ix.Workspaces() {
		add(w.IDPrefix)
	}
	for _, e := range ix.Tasks() {
		for _, k := range append([]string{e.ID}, e.FormerIDs...) {
			if p, _, ok := model.ParseTaskID(k); ok {
				add(p)
			}
		}
	}
	return out
}

// autoLinks finds the tasks of ix the pull requests of repo point at: by
// task, the refs to link. Ids of the past count, as ix.Resolve finds them.
func autoLinks(ix *index.Index, repo string, pulls []github.Pull) map[model.ULID][]string {
	repo = strings.ToLower(repo) // as hh link writes the ref
	prefixes := prefixesOf(ix)
	out := map[model.ULID][]string{}
	for _, p := range pulls {
		ref := model.GitHubRef{Repo: repo, Kind: model.LinkPR, Number: p.Number}.String()
		for _, name := range mentioned(p, prefixes) {
			u, err := ix.Resolve(name)
			if err != nil {
				continue // a task of no store here, or a number that only looks like an id
			}
			if e, ok := ix.Task(u); !ok || e.DeletedAt != nil || e.Broken != "" {
				continue
			}
			if !slices.Contains(out[u], ref) {
				out[u] = append(out[u], ref)
			}
		}
	}
	return out
}
