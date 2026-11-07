// Package automation holds automations.yaml: its form, its checks, and the
// evaluation of its rules. It does no I/O.
package automation

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"go.yaml.in/yaml/v4"
)

// The events a rule may be on, the condition it may have, and the prefix of
// its where.
const (
	PROpened           = "pr.opened"
	PRReviewRequested  = "pr.review_requested"
	PRMerged           = "pr.merged"
	IssueClosed        = "issue.closed"
	AllLinkedPRsMerged = "all_linked_prs_merged"
	WherePrefix        = "project:"
)

// Events lists what a rule may be on, in the order they are handled.
var Events = []string{PROpened, PRReviewRequested, PRMerged, IssueClosed}

// IssueFieldNames are the fields of an issue that issue_fields may name.
var IssueFieldNames = []string{"title", "body", "status", "assignees", "labels", "milestone"}

// SyncModes are the values of issue_fields.default_sync.
var SyncModes = []string{model.SyncMirror, model.SyncPush, model.SyncLink}

// PropKeys are the keys of issue_fields.props.
var PropKeys = []string{"milestone"}

// ErrExists and ErrNotFound are what the edits of a File answer.
var (
	ErrExists   = errors.New("a rule with this id exists")
	ErrNotFound = errors.New("no such rule")
)

// Set says what a rule does: it sets a status, or the default status of a
// category.
type Set struct {
	Status   string `yaml:"status,omitempty"`
	Category string `yaml:"category,omitempty"`
}

// MarshalYAML writes Set in flow style: {status: review}.
func (s Set) MarshalYAML() (any, error) {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Style: yaml.FlowStyle}
	for _, kv := range [][2]string{{"status", s.Status}, {"category", s.Category}} {
		if kv[1] != "" {
			n.Content = append(n.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: kv[0]},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: kv[1]})
		}
	}
	return n, nil
}

// Rule is one item of rules.
type Rule struct {
	ID, On, When, Where string
	Set                 Set
	Enabled             bool // false only when the file says so
}

// ruleY is Rule as the file holds it.
type ruleY struct {
	ID      string `yaml:"id"`
	Where   string `yaml:"where,omitempty"`
	On      string `yaml:"on"`
	When    string `yaml:"when,omitempty"`
	Set     Set    `yaml:"set"`
	Enabled *bool  `yaml:"enabled,omitempty"`
}

func (r Rule) wire() ruleY {
	w := ruleY{ID: r.ID, Where: r.Where, On: r.On, When: r.When, Set: r.Set}
	if !r.Enabled {
		off := false
		w.Enabled = &off
	}
	return w
}

func (w ruleY) rule() Rule {
	return Rule{ID: w.ID, On: w.On, When: w.When, Where: w.Where, Set: w.Set, Enabled: w.Enabled == nil || *w.Enabled}
}

// Scoped turns rules off in one project and the projects under it.
type Scoped struct {
	Where   string   `yaml:"where"`
	Disable []string `yaml:"disable,omitempty"`
}

// IssueFields says which fields of an issue are synced, and how.
type IssueFields struct {
	DefaultSync string            `yaml:"default_sync"`
	Push        []string          `yaml:"push"`
	Mirror      []string          `yaml:"mirror"`
	Props       map[string]string `yaml:"props,omitempty"`
}

// issueFieldsY is IssueFields as the file holds it: a key the file lacks and
// whose value is the default is left out, so that an edit of one key does not
// write the others. A key that the file has stays, even when it is empty.
type issueFieldsY struct {
	DefaultSync *string            `yaml:"default_sync,omitempty"`
	Push        *[]string          `yaml:"push,omitempty"`
	Mirror      *[]string          `yaml:"mirror,omitempty"`
	Props       *map[string]string `yaml:"props,omitempty"`
}

// wire returns i as the file holds it; old is the issue_fields node that was
// read, or nil.
func (i IssueFields) wire(old *yaml.Node) issueFieldsY {
	def := defaultIssueFields()
	has := func(key string) bool {
		if old == nil || old.Kind != yaml.MappingNode {
			return false
		}
		for j := 0; j+1 < len(old.Content); j += 2 {
			if old.Content[j].Value == key {
				return true
			}
		}
		return false
	}
	var w issueFieldsY
	if has("default_sync") || i.DefaultSync != def.DefaultSync {
		w.DefaultSync = &i.DefaultSync
	}
	if has("push") || !slices.Equal(i.Push, def.Push) {
		w.Push = &i.Push
	}
	if has("mirror") || !slices.Equal(i.Mirror, def.Mirror) {
		w.Mirror = &i.Mirror
	}
	if has("props") || len(i.Props) > 0 {
		props := i.Props
		if props == nil {
			props = map[string]string{}
		}
		w.Props = &props
	}
	return w
}

// IssueFieldsChange is a change to issue_fields. A nil part is left as it is.
// A key of Props is set to its value, or taken off when the value is empty.
type IssueFieldsChange struct {
	DefaultSync *string
	Push        *[]string
	Mirror      *[]string
	Props       map[string]string
}

// File is automations.yaml. Extra holds the keys it does not know.
type File struct {
	Rules       []Rule
	Scoped      []Scoped
	IssueFields IssueFields
	Extra       model.Fields

	doc *model.Doc
}

var fileKeys = []string{"rules", "scoped", "issue_fields"}

func defaultIssueFields() IssueFields {
	return IssueFields{
		DefaultSync: model.SyncLink,
		Push:        []string{"title", "body", "status", "assignees", "labels"},
		Mirror:      []string{"title", "status", "assignees"},
	}
}

// Default returns the file of a store with no rules.
func Default() *File { return &File{IssueFields: defaultIssueFields()} }

// DevRules returns the rules of --template dev.
func DevRules() *File {
	f := Default()
	f.Rules = []Rule{
		{ID: "pr-opened", On: PROpened, Set: Set{Status: "in_progress"}, Enabled: true},
		{ID: "pr-review", On: PRReviewRequested, Set: Set{Status: "review"}, Enabled: true},
		{ID: "pr-merged", On: PRMerged, When: AllLinkedPRsMerged, Set: Set{Category: string(schema.CategoryDone)}, Enabled: true},
		{ID: "issue-closed", On: IssueClosed, Set: Set{Category: string(schema.CategoryDone)}, Enabled: true},
	}
	return f
}

// Decode reads automations.yaml. A key it lacks has its default. It does not
// check the file; see Check.
func Decode(b []byte) (*File, error) {
	d, err := model.ParseDoc(b, 0)
	if err != nil {
		return nil, err
	}
	f := Default()
	f.doc = d
	var rules []ruleY
	if err := d.Decode("rules", &rules); err != nil {
		return nil, err
	}
	for _, w := range rules {
		f.Rules = append(f.Rules, w.rule())
	}
	if err := d.Decode("scoped", &f.Scoped); err != nil {
		return nil, err
	}
	if err := d.Decode("issue_fields", &f.IssueFields); err != nil {
		return nil, err
	}
	f.Extra = d.Extra(fileKeys)
	return f, nil
}

// Encode writes automations.yaml in the order of its keys, with the unknown
// keys and the comments as they were read.
func (f *File) Encode() ([]byte, error) {
	if f.doc == nil {
		f.doc = model.NewDoc()
	}
	rules := make([]ruleY, len(f.Rules))
	for i, r := range f.Rules {
		rules[i] = r.wire()
	}
	e := f.doc.Encoder()
	e.Put("rules", rules, len(rules) == 0)
	e.Put("scoped", f.Scoped, len(f.Scoped) == 0)
	fields := f.IssueFields.wire(f.doc.Get("issue_fields"))
	e.Put("issue_fields", fields, reflect.DeepEqual(fields, issueFieldsY{}))
	return e.Bytes(f.Extra)
}

// Rule returns the rule with the id.
func (f *File) Rule(id string) (Rule, bool) {
	if i := f.ruleIndex(id); i >= 0 {
		return f.Rules[i], true
	}
	return Rule{}, false
}

func (f *File) ruleIndex(id string) int {
	return slices.IndexFunc(f.Rules, func(r Rule) bool { return r.ID == id })
}

// AddRule appends r. It is ErrExists when the id is taken.
func (f *File) AddRule(r Rule) error {
	if f.ruleIndex(r.ID) >= 0 {
		return fmt.Errorf("%q: %w", r.ID, ErrExists)
	}
	f.Rules = append(f.Rules, r)
	return nil
}

// RemoveRule removes the rule, and its id from every disable.
func (f *File) RemoveRule(id string) error {
	i := f.ruleIndex(id)
	if i < 0 {
		return fmt.Errorf("%q: %w", id, ErrNotFound)
	}
	f.Rules = slices.Delete(f.Rules, i, i+1)
	for j := range f.Scoped {
		f.Scoped[j].Disable = slices.DeleteFunc(slices.Clone(f.Scoped[j].Disable), func(s string) bool { return s == id })
	}
	f.dropEmptyScoped()
	return nil
}

// dropEmptyScoped removes the scoped items that disable nothing.
func (f *File) dropEmptyScoped() {
	f.Scoped = slices.DeleteFunc(f.Scoped, func(s Scoped) bool { return len(s.Disable) == 0 })
}

// SetEnabled turns the rule on or off. Turning it on takes the key out.
func (f *File) SetEnabled(id string, on bool) error {
	i := f.ruleIndex(id)
	if i < 0 {
		return fmt.Errorf("%q: %w", id, ErrNotFound)
	}
	f.Rules[i].Enabled = on
	return nil
}

// SetDisabledIn turns the rule off in the project where names, or back on.
// A scoped item left with nothing to disable is dropped.
func (f *File) SetDisabledIn(where, id string, off bool) error {
	if off && f.ruleIndex(id) < 0 {
		return fmt.Errorf("%q: %w", id, ErrNotFound)
	}
	i := slices.IndexFunc(f.Scoped, func(s Scoped) bool { return s.Where == where })
	if off {
		if i < 0 {
			f.Scoped = append(f.Scoped, Scoped{Where: where})
			i = len(f.Scoped) - 1
		}
		if !slices.Contains(f.Scoped[i].Disable, id) {
			f.Scoped[i].Disable = append(slices.Clone(f.Scoped[i].Disable), id)
		}
		return nil
	}
	if i >= 0 {
		f.Scoped[i].Disable = slices.DeleteFunc(slices.Clone(f.Scoped[i].Disable), func(s string) bool { return s == id })
		f.dropEmptyScoped()
	}
	return nil
}

// SetIssueFields applies ch. It is an error, and f is not changed, when the
// result has a value that Check would report.
func (f *File) SetIssueFields(ch IssueFieldsChange) error {
	next := f.IssueFields
	if ch.DefaultSync != nil {
		next.DefaultSync = *ch.DefaultSync
	}
	if ch.Push != nil {
		next.Push = slices.Clone(*ch.Push)
	}
	if ch.Mirror != nil {
		next.Mirror = slices.Clone(*ch.Mirror)
	}
	if len(ch.Props) > 0 {
		props := maps.Clone(next.Props)
		if props == nil {
			props = map[string]string{}
		}
		for k, v := range ch.Props {
			if v == "" {
				delete(props, k)
			} else {
				props[k] = v
			}
		}
		next.Props = props
		if len(props) == 0 {
			next.Props = nil
		}
	}
	if ps := next.problems("issue_fields"); len(ps) > 0 {
		return errors.New(ps[0].String())
	}
	f.IssueFields = next
	return nil
}

// problems lists the defects of i, with their paths under base.
func (i IssueFields) problems(base string) []schema.Problem {
	var ps []schema.Problem
	oneOf := func(path, v string, want []string) {
		if !slices.Contains(want, v) {
			ps = append(ps, schema.NewProblem(path, "automation.not_one_of", "%q is not one of %s", v, strings.Join(want, ", ")))
		}
	}
	oneOf(base+".default_sync", i.DefaultSync, SyncModes)
	for _, l := range []struct {
		key    string
		fields []string
	}{{"push", i.Push}, {"mirror", i.Mirror}} {
		for j, name := range l.fields {
			oneOf(fmt.Sprintf("%s.%s[%d]", base, l.key, j), name, IssueFieldNames)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(i.Props)) {
		oneOf(base+".props."+k, k, PropKeys)
	}
	return ps
}
