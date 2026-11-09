package model

import (
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v4"
)

// IssueValues is what the fields of an issue hold, by the name of the field:
// assignees as they are, the others as one value. A field that is
// not there has no value yet.
type IssueValues map[string][]string

// IssueValueKeys are the fields of IssueValues, in the order they are written.
var IssueValueKeys = []string{"title", "body", "status", "assignees", "milestone"}

// IsIssueList reports whether the field of an issue holds a list.
func IsIssueList(field string) bool { return field == "assignees" }

// Values reads Synced.
func (x IssueState) Values() (IssueValues, error) {
	out := IssueValues{}
	if x.Synced == nil || x.Synced.Tag == "!!null" {
		return out, nil
	}
	n := x.Synced
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 { // as the decoder gives a node
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return nil, &DecodeError{Field: "gh.issues.synced", Err: fmt.Errorf("want a mapping")}
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i].Value, n.Content[i+1]
		switch v.Kind {
		case yaml.ScalarNode:
			out[k] = []string{v.Value}
		case yaml.SequenceNode:
			list := make([]string, 0, len(v.Content))
			for _, c := range v.Content {
				list = append(list, c.Value)
			}
			out[k] = list
		}
	}
	return out, nil
}

// SetValues writes v to Synced. With no values Synced goes.
func (x *IssueState) SetValues(v IssueValues) error {
	if len(v) == 0 {
		x.Synced = nil
		return nil
	}
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Style: yaml.FlowStyle}
	text := func(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
	for _, k := range IssueValueKeys {
		vals, ok := v[k]
		if !ok {
			continue
		}
		var node *yaml.Node
		if IsIssueList(k) {
			node = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
			for _, s := range vals {
				node.Content = append(node.Content, text(s))
			}
		} else {
			if len(vals) != 1 {
				return fmt.Errorf("issue field %s holds one value, not %d", k, len(vals))
			}
			node = text(vals[0])
		}
		n.Content = append(n.Content, text(k), node)
	}
	x.Synced = n
	return nil
}

// markLine is the line that ends a body Hedgehug wrote.
var markLine = regexp.MustCompile(`^hedgehug:[0-9A-Za-z]{26}$`)

// MarkedBody is body with the mark that points back at the task u on its last
// line.
func MarkedBody(u ULID, body string) string {
	mark := "hedgehug:" + u.String()
	if b := strings.TrimRight(body, " \t\r\n"); b != "" {
		return b + "\n\n" + mark
	}
	return mark
}

// NormalizeBody is body as it compares: in LF and with no white space at its
// end. GitHub's web editor writes CRLF.
func NormalizeBody(body string) string {
	return strings.TrimRight(strings.ReplaceAll(body, "\r\n", "\n"), " \t\n")
}

var commentMarkRE = regexp.MustCompile(`(?:^|\n)<!-- hedgehug:([0-9A-Za-z]{26}) -->$`)

// MarkedComment is body, as a push sends it as a comment: the mark that gives
// the comment u back, which GitHub does not show, ends it.
func MarkedComment(u ULID, body string) string {
	mark := "<!-- hedgehug:" + u.String() + " -->"
	if b := NormalizeBody(body); b != "" {
		return b + "\n\n" + mark
	}
	return mark
}

// SplitComment is the text of a comment of GitHub, in NormalizeBody form with
// the mark MarkedComment puts on its last line taken off, and the ULID the
// mark names. A comment with no mark has none.
func SplitComment(body string) (text string, u ULID, ok bool) {
	b := NormalizeBody(body)
	m := commentMarkRE.FindStringSubmatchIndex(b)
	if m == nil {
		return b, ULID{}, false
	}
	u, err := ParseULID(b[m[2]:m[3]])
	if err != nil {
		return b, ULID{}, false
	}
	return strings.TrimRight(b[:m[0]], " \t\n"), u, true
}

// StripMark is NormalizeBody with no mark on its last line.
func StripMark(body string) string {
	b := NormalizeBody(body)
	i := strings.LastIndexByte(b, '\n') + 1
	if markLine.MatchString(strings.TrimSpace(b[i:])) {
		b = strings.TrimRight(b[:i], " \t\n")
	}
	return b
}
