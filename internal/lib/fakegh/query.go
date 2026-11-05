package fakegh

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// shape is what a field may have: its arguments (without spaces) and its
// subfields. A field with no subfields is a leaf.
type shape struct {
	args string
	kids map[string]shape
}

// leaves makes the shapes of fields with no arguments and no subfields.
func leaves(names ...string) map[string]shape {
	m := make(map[string]shape, len(names))
	for _, n := range names {
		m[n] = shape{}
	}
	return m
}

func with(m map[string]shape, more map[string]shape) map[string]shape {
	for k, v := range more {
		m[k] = v
	}
	return m
}

// The fields the fake knows of GitHub's schema, which is what the queries of
// internal/lib/github must stay within: a typo or a missing field there is
// an error here, as GitHub would answer. They are written again from the
// client's queries on purpose.
var (
	rateLimitShape = shape{kids: leaves("limit", "remaining", "resetAt")}

	pullShape = shape{kids: with(
		leaves("number", "title", "body", "state", "isDraft", "mergedAt", "updatedAt", "headRefName", "reviewDecision"),
		map[string]shape{
			"reviewRequests":           {args: "first:1", kids: leaves("totalCount")},
			"latestOpinionatedReviews": {args: "first:100", kids: map[string]shape{"nodes": {kids: leaves("state")}}},
			"commits": {args: "last:1", kids: map[string]shape{"nodes": {kids: map[string]shape{
				"commit": {kids: map[string]shape{"statusCheckRollup": {kids: leaves("state")}}},
			}}}},
			"comments": {args: "first:100", kids: map[string]shape{"nodes": {kids: leaves("body")}}},
		})}
	issueShape = shape{kids: with(
		leaves("number", "title", "body", "state", "stateReason", "updatedAt"),
		map[string]shape{
			"assignees": {args: "first:100", kids: map[string]shape{"nodes": {kids: leaves("login")}}},
			"labels":    {args: "first:100", kids: map[string]shape{"nodes": {kids: leaves("name")}}},
			"milestone": {kids: leaves("title")},
		})}

	repositoryArgs = "owner:$owner,name:$name"

	openIssuesShape = map[string]shape{
		"rateLimit": rateLimitShape,
		"repository": {args: repositoryArgs, kids: map[string]shape{
			"issues": {
				args: "first:100,after:$after,states:[OPEN],labels:[$label]",
				kids: map[string]shape{
					"pageInfo": {kids: leaves("hasNextPage", "endCursor")},
					"nodes":    issueShape,
				},
			},
		}},
	}

	scanShape = map[string]shape{
		"rateLimit": rateLimitShape,
		"repository": {args: repositoryArgs, kids: map[string]shape{
			"pullRequests": {
				args: "first:100,after:$after,orderBy:{field:UPDATED_AT,direction:DESC}",
				kids: map[string]shape{
					"pageInfo": {kids: leaves("hasNextPage", "endCursor")},
					"nodes":    pullShape,
				},
			},
		}},
	}
)

// field is a field of a parsed query.
type field struct {
	alias, name, args string
	kids              []*field
}

// parseQuery parses the selection of a query: "query Name(...) { ... }". It
// returns the name of the query and its top-level fields.
func parseQuery(q string) (string, []*field, error) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(q), "query ")
	if !ok {
		return "", nil, errors.New("only a query is answered")
	}
	i := strings.IndexAny(rest, "({ \n")
	if i < 0 {
		return "", nil, errors.New("syntax error: no selection")
	}
	name := rest[:i]
	p := &parser{s: rest[i:]}
	p.skip()
	if p.peek() == '(' { // the variables
		if _, err := p.group(); err != nil {
			return "", nil, err
		}
		p.skip()
	}
	if p.peek() != '{' {
		return "", nil, errors.New("syntax error: want {")
	}
	p.i++
	kids, err := p.set()
	if err != nil {
		return "", nil, err
	}
	p.skip()
	if p.i != len(p.s) {
		return "", nil, errors.New("syntax error: text after the query")
	}
	return name, kids, nil
}

type parser struct {
	s string
	i int
}

func (p *parser) peek() byte {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

func (p *parser) skip() {
	for p.i < len(p.s) && strings.IndexByte(" \t\r\n,", p.s[p.i]) >= 0 {
		p.i++
	}
}

func (p *parser) word() string {
	j := p.i
	for j < len(p.s) && (p.s[j] == '_' || p.s[j] >= '0' && p.s[j] <= '9' || p.s[j] >= 'a' && p.s[j] <= 'z' || p.s[j] >= 'A' && p.s[j] <= 'Z') {
		j++
	}
	w := p.s[p.i:j]
	p.i = j
	return w
}

// group reads (...) and returns what is inside, without spaces.
func (p *parser) group() (string, error) {
	depth, start := 0, p.i+1
	for ; p.i < len(p.s); p.i++ {
		switch p.s[p.i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				in := p.s[start:p.i]
				p.i++
				return strings.Join(strings.Fields(in), ""), nil
			}
		}
	}
	return "", errors.New("syntax error: ( is not closed")
}

// set reads fields up to the closing }.
func (p *parser) set() ([]*field, error) {
	var out []*field
	for {
		p.skip()
		if p.peek() == '}' {
			p.i++
			return out, nil
		}
		f := &field{name: p.word()}
		if f.name == "" {
			return nil, fmt.Errorf("syntax error at %q", p.s[p.i:min(p.i+20, len(p.s))])
		}
		p.skip()
		if p.peek() == ':' {
			p.i++
			p.skip()
			f.alias, f.name = f.name, p.word()
			p.skip()
		}
		if p.peek() == '(' {
			var err error
			if f.args, err = p.group(); err != nil {
				return nil, err
			}
			p.skip()
		}
		if p.peek() == '{' {
			p.i++
			var err error
			if f.kids, err = p.set(); err != nil {
				return nil, err
			}
		}
		out = append(out, f)
		if p.i >= len(p.s) {
			return nil, errors.New("syntax error: } is missing")
		}
	}
}

// check compares fields with what the schema allows and returns the errors
// GitHub would give: fields that do not exist, arguments that differ, and
// fields the query lacks (which the client reads).
func check(fields []*field, want map[string]shape, on string) []string {
	var errs []string
	seen := map[string]bool{}
	for _, f := range fields {
		w, ok := want[f.name]
		if !ok {
			errs = append(errs, fmt.Sprintf("Field '%s' doesn't exist on type '%s'", f.name, on))
			continue
		}
		seen[f.name] = true
		if f.args != w.args {
			errs = append(errs, fmt.Sprintf("Field '%s' on type '%s' is called with the arguments (%s), want (%s)", f.name, on, f.args, w.args))
		}
		if w.kids == nil && len(f.kids) > 0 {
			errs = append(errs, fmt.Sprintf("Field '%s' must not have a selection", f.name))
		}
		if w.kids != nil {
			errs = append(errs, check(f.kids, w.kids, f.name)...)
		}
	}
	for _, name := range sortedKeys(want) {
		if !seen[name] {
			errs = append(errs, fmt.Sprintf("Field '%s' is missing on type '%s'", name, on))
		}
	}
	return errs
}

func sortedKeys(m map[string]shape) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

var linkedAliasName = regexp.MustCompile(`^([pi])(\d+)$`)

// checkLinked checks the query of LinkedItems: the repository holds aliased
// pullRequest(number: n) and issue(number: n), whose alias is p<n> and i<n>.
func checkLinked(top []*field) []string {
	var errs []string
	for _, f := range top {
		switch f.name {
		case "rateLimit":
			errs = append(errs, check([]*field{f}, map[string]shape{"rateLimit": rateLimitShape}, "Query")...)
		case "repository":
			if f.args != repositoryArgs {
				errs = append(errs, "Field 'repository' is called with the arguments ("+f.args+")")
			}
			for _, a := range f.kids {
				errs = append(errs, checkAlias(a)...)
			}
		default:
			errs = append(errs, fmt.Sprintf("Field '%s' doesn't exist on type 'Query'", f.name))
		}
	}
	return errs
}

func checkAlias(a *field) []string {
	m := linkedAliasName.FindStringSubmatch(a.alias)
	if m == nil {
		return []string{fmt.Sprintf("The alias '%s' is not p<number> or i<number>", a.alias)}
	}
	kind, name, sh := "pullRequest", "PullRequest", pullShape
	if m[1] == "i" {
		kind, name, sh = "issue", "Issue", issueShape
	}
	if a.name != kind {
		return []string{fmt.Sprintf("The alias '%s' must be of %s, not %s", a.alias, kind, a.name)}
	}
	if a.args != "number:"+m[2] {
		return []string{fmt.Sprintf("The alias '%s' is called with the arguments (%s), want (number:%s)", a.alias, a.args, m[2])}
	}
	return check(a.kids, sh.kids, name)
}
