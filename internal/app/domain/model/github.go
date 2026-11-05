package model

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"
)

// The types of a link, and how a link syncs.
const (
	LinkPR    = "github_pr"
	LinkIssue = "github_issue"

	SyncMirror = "mirror"
	SyncPush   = "push"
	SyncLink   = "link"
	SyncOff    = "off" // unlinked: kept so that the automatic linking does not link it again
)

// Live reports whether the link is in use.
func (l Link) Live() bool { return l.Sync != SyncOff }

// GitHubRef is the ref of a link: <host>/<owner>/<repo>/pull/<n> or
// <host>/<owner>/<repo>/issues/<n>, in lower case: GitHub does not tell
// repositories apart by case, so a ref has one form.
type GitHubRef struct {
	Repo   string // <host>/<owner>/<repo>
	Kind   string // LinkPR or LinkIssue
	Number int
}

var githubRef = regexp.MustCompile(`^([^/]+/[^/]+/[^/]+)/(pull|issues)/([1-9][0-9]*)$`)

// ParseGitHubRef reads the ref of a link.
func ParseGitHubRef(s string) (GitHubRef, bool) {
	m := githubRef.FindStringSubmatch(strings.ToLower(s))
	if m == nil {
		return GitHubRef{}, false
	}
	n, err := strconv.Atoi(m[3])
	if err != nil {
		return GitHubRef{}, false
	}
	kind := LinkPR
	if m[2] == "issues" {
		kind = LinkIssue
	}
	return GitHubRef{Repo: m[1], Kind: kind, Number: n}, true
}

// CanonRef is s in the form String gives, or s itself when it is not a ref.
// Links written by hand may be in other letters; compare them through this.
func CanonRef(s string) string {
	if r, ok := ParseGitHubRef(s); ok {
		return r.String()
	}
	return s
}

// String is the ref as a link holds it.
func (r GitHubRef) String() string {
	part := "pull"
	if r.Kind == LinkIssue {
		part = "issues"
	}
	return fmt.Sprintf("%s/%s/%d", strings.ToLower(r.Repo), part, r.Number)
}

// Label is the ref as people write it: org-a/web#123.
func (r GitHubRef) Label() string {
	_, full, _ := strings.Cut(r.Repo, "/")
	return fmt.Sprintf("%s#%d", full, r.Number)
}

// PRState is what GitHub says of a pull request. The keys it does not know
// stay in the file: SetGitHub merges into the nodes that are there.
type PRState struct {
	Ref      string     `yaml:"ref"`
	State    string     `yaml:"state"`            // open, draft, merged, closed
	Review   string     `yaml:"review"`           // none, pending, approved, changes_requested
	Checks   string     `yaml:"checks,omitempty"` // success, failure, pending; empty with no checks
	MergedAt *time.Time `yaml:"merged_at"`
}

// IssueState is what GitHub says of an issue. Synced is the values as they
// were last made the same, which the issue sync of 5c reads and writes.
type IssueState struct {
	Ref         string     `yaml:"ref"`
	State       string     `yaml:"state"` // open, closed
	StateReason *string    `yaml:"state_reason"`
	Synced      *yaml.Node `yaml:"synced,omitempty"`
}

// GitHubState is the part of gh that Hedgehug writes. The other keys of gh
// stay in the file as they are.
type GitHubState struct {
	PRs    []PRState    `yaml:"prs,omitempty"`
	Issues []IssueState `yaml:"issues,omitempty"`
}

// GitHub reads gh. A task with none has an empty state; a gh that is not a
// mapping is an error, and SetGitHub would replace it.
func (t *Task) GitHub() (GitHubState, error) {
	var g GitHubState
	if t.GH == nil || t.GH.Tag == "!!null" {
		return g, nil
	}
	if t.GH.Kind != yaml.MappingNode {
		return g, &DecodeError{Field: "gh", Err: fmt.Errorf("want a mapping")}
	}
	if err := t.GH.Decode(&g); err != nil {
		return g, &DecodeError{Field: "gh", Err: err}
	}
	// The decoder reads a null into a pointer as a pointer to a zero value.
	for i := range g.PRs {
		if p := &g.PRs[i]; p.MergedAt != nil && p.MergedAt.IsZero() {
			p.MergedAt = nil
		}
	}
	for i := range g.Issues {
		if x := &g.Issues[i]; x.StateReason != nil && *x.StateReason == "" {
			x.StateReason = nil
		}
		// The decoder leaves a *yaml.Node empty: take the node from the file.
		g.Issues[i].Synced = nil
		if items := nodeAt(t.GH, "issues"); items != nil && i < len(items.Content) {
			g.Issues[i].Synced = nodeAt(items.Content[i], "synced")
		}
	}
	return g, nil
}

// nodeAt is the value of key in the mapping n, or nil.
func nodeAt(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// SetGitHub writes prs and issues of gh and keeps the rest as it is: the
// other keys, and in the items of prs and issues the keys and the parts that
// did not change, with their comments and look. With nothing to keep it
// leaves gh out.
func (t *Task) SetGitHub(g GitHubState) error {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if t.GH != nil && t.GH.Kind == yaml.MappingNode {
		c := *t.GH
		c.Content = append([]*yaml.Node(nil), t.GH.Content...)
		n = &c
	}
	if err := putGH(n, "prs", g.PRs, len(g.PRs) == 0); err != nil {
		return err
	}
	if err := putGH(n, "issues", g.Issues, len(g.Issues) == 0); err != nil {
		return err
	}
	if len(n.Content) == 0 {
		t.GH = nil
		if t.doc != nil {
			t.doc.drop("gh") // or Encode writes the key as null
		}
		return nil
	}
	t.GH = n
	return nil
}

// putGH sets key of the mapping n to v, or removes it when empty. The node
// that n holds for key is merged with v, not replaced.
func putGH(n *yaml.Node, key string, v any, empty bool) error {
	at := -1
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			at = i
		}
	}
	if empty {
		if at >= 0 {
			n.Content = append(n.Content[:at:at], n.Content[at+2:]...)
		}
		return nil
	}
	value := new(yaml.Node)
	if err := value.Encode(v); err != nil {
		return err
	}
	if at >= 0 {
		n.Content[at+1] = mergeByRef(n.Content[at+1], value, reflect.ValueOf(v))
		return nil
	}
	n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
	return nil
}

// mergeByRef merges the list nv (the encoding of v, a slice of structs with a
// ref) into old. An item takes the old item with the same ref, so that the
// keys it does not know stay with it, and an item with a new ref starts bare.
func mergeByRef(old, nv *yaml.Node, v reflect.Value) *yaml.Node {
	if old.Kind != yaml.SequenceNode || v.Len() != len(nv.Content) {
		return nv
	}
	refOf := func(n *yaml.Node) string {
		if n.Kind != yaml.MappingNode {
			return ""
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == "ref" {
				return n.Content[i+1].Value
			}
		}
		return ""
	}
	items := make([]*yaml.Node, len(nv.Content))
	for i, x := range nv.Content {
		items[i] = x
		for _, o := range old.Content {
			if r := refOf(o); r != "" && r == refOf(x) {
				items[i] = mergeNode(o, x, v.Index(i))
				break
			}
		}
	}
	out := *old
	out.Content = items
	return &out
}
