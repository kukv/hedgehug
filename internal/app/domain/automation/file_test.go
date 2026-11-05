package automation

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/lib/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }

func readFull(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/full.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustDecode(t *testing.T, src string) *File {
	t.Helper()
	f, err := Decode([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func mustEncode(t *testing.T, f *File) string {
	t.Helper()
	b, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	src := readFull(t)
	f, err := Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Rules) != 5 || len(f.Scoped) != 1 || f.Scoped[0].Disable[0] != "pr-review" {
		t.Fatalf("decoded = %+v", f)
	}
	if r := f.Rules[1]; r.Enabled || r.Set.Status != "review" {
		t.Errorf("pr-review = %+v", r)
	}
	if r := f.Rules[2]; !r.Enabled || r.When != AllLinkedPRsMerged || r.Set.Category != "done" {
		t.Errorf("pr-merged = %+v", r)
	}
	if r := f.Rules[4]; r.Where != "project:01JBZ7XXH2M4Q8R1T5V9W3Y6ZA" {
		t.Errorf("release-review = %+v", r)
	}
	if f.IssueFields.Props["milestone"] != "target" || len(f.Extra) != 1 || f.Extra[0].Key != "future" {
		t.Errorf("issue_fields = %+v, extra = %+v", f.IssueFields, f.Extra)
	}
	if out := mustEncode(t, f); out != string(src) {
		t.Errorf("round trip changed the file:\n%s", out)
	}
}

func TestDecodeDefaults(t *testing.T) {
	t.Parallel()
	for _, src := range []string{"", "# nothing\n", "rules:\n"} {
		f := mustDecode(t, src)
		if len(f.Rules) != 0 || len(f.Scoped) != 0 {
			t.Errorf("%q: %+v", src, f)
		}
		i := f.IssueFields
		if i.DefaultSync != "link" || !slices.Equal(i.Push, []string{"title", "body", "status", "assignees", "labels"}) ||
			!slices.Equal(i.Mirror, []string{"title", "status", "assignees"}) || len(i.Props) != 0 {
			t.Errorf("%q: issue_fields = %+v", src, i)
		}
	}
	f := mustDecode(t, "issue_fields:\n  push: []\n")
	if f.IssueFields.Push == nil || len(f.IssueFields.Push) != 0 || f.IssueFields.DefaultSync != "link" || len(f.IssueFields.Mirror) != 3 {
		t.Errorf("issue_fields = %+v", f.IssueFields)
	}
}

func TestDefaultWritesNothing(t *testing.T) {
	t.Parallel()
	if out := mustEncode(t, Default()); strings.Contains(out, "issue_fields") || strings.Contains(out, "rules") {
		t.Errorf("out = %q", out)
	}
	out := mustEncode(t, mustDecode(t, ""))
	if strings.Contains(out, "issue_fields") {
		t.Errorf("out = %q", out)
	}
}

func TestDevRules(t *testing.T) {
	t.Parallel()
	d := DevRules()
	ids := make([]string, len(d.Rules))
	for i, r := range d.Rules {
		ids[i] = r.ID
		if !r.Enabled {
			t.Errorf("%s is off", r.ID)
		}
	}
	if !slices.Equal(ids, []string{"pr-opened", "pr-review", "pr-merged", "issue-closed"}) {
		t.Errorf("ids = %v", ids)
	}
	out := mustEncode(t, d)
	back := mustDecode(t, out)
	if len(back.Rules) != 4 || back.Rules[1].Set.Status != "review" || back.Rules[2].When != AllLinkedPRsMerged || back.Rules[3].Set.Category != "done" {
		t.Errorf("rules = %+v\n%s", back.Rules, out)
	}
	if !strings.Contains(out, "set: {status: in_progress}") || strings.Contains(out, "enabled") {
		t.Errorf("out =\n%s", out)
	}
}

func TestAddRule(t *testing.T) {
	t.Parallel()
	f := mustDecode(t, string(readFull(t)))
	err := f.AddRule(Rule{ID: "pr-opened", On: PROpened, Enabled: true})
	if !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v", err)
	}
	if err := f.AddRule(Rule{ID: "extra", On: PRMerged, Set: Set{Status: "done"}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	out := mustEncode(t, f)
	if !strings.HasPrefix(out, "# Automation rules of the store.\n") || !strings.Contains(out, "enabled: false # off for now") || !strings.Contains(out, "future: {a: 1} # unknown key") {
		t.Errorf("comments lost:\n%s", out)
	}
	if r, ok := mustDecode(t, out).Rule("extra"); !ok || r.On != PRMerged || r.Set.Status != "done" || !r.Enabled {
		t.Errorf("extra = %+v, %v", r, ok)
	}
	if _, ok := f.Rule("none"); ok {
		t.Error("Rule found none")
	}
}

func TestRemoveRuleLeavesDisable(t *testing.T) {
	t.Parallel()
	f := mustDecode(t, string(readFull(t)))
	if err := f.RemoveRule("pr-review"); err != nil {
		t.Fatal(err)
	}
	if len(f.Scoped) != 0 {
		t.Errorf("scoped = %+v", f.Scoped)
	}
	back := mustDecode(t, mustEncode(t, f))
	if _, ok := back.Rule("pr-review"); ok || len(back.Rules) != 4 {
		t.Errorf("rules = %+v", back.Rules)
	}
	if err := f.RemoveRule("pr-review"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestRemoveRuleKeepsOtherDisables(t *testing.T) {
	t.Parallel()
	f := mustDecode(t, "rules:\n- {id: a, on: pr.opened, set: {status: x}}\n- {id: b, on: pr.opened, set: {status: x}}\nscoped:\n- {where: 'project:X', disable: [a, b]}\n")
	if err := f.RemoveRule("a"); err != nil {
		t.Fatal(err)
	}
	if len(f.Scoped) != 1 || !slices.Equal(f.Scoped[0].Disable, []string{"b"}) {
		t.Errorf("scoped = %+v", f.Scoped)
	}
}

func TestSetEnabledRoundTrip(t *testing.T) {
	t.Parallel()
	src := string(readFull(t))
	f := mustDecode(t, src)
	if err := f.SetEnabled("pr-review", true); err != nil {
		t.Fatal(err)
	}
	out := mustEncode(t, f)
	if strings.Contains(out, "enabled") {
		t.Errorf("enabled: true was written:\n%s", out)
	}
	if err := f.SetEnabled("pr-opened", false); err != nil {
		t.Fatal(err)
	}
	out = mustEncode(t, f)
	back := mustDecode(t, out)
	if r, _ := back.Rule("pr-opened"); r.Enabled {
		t.Errorf("pr-opened = %+v\n%s", r, out)
	}
	if r, _ := back.Rule("pr-review"); !r.Enabled {
		t.Errorf("pr-review = %+v", r)
	}
	if err := f.SetEnabled("pr-opened", true); err != nil {
		t.Fatal(err)
	}
	if err := f.SetEnabled("none", true); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v", err)
	}
	// Back where it started except for pr-review.
	want := strings.Replace(src, "  enabled: false # off for now\n", "", 1)
	if out := mustEncode(t, f); out != want {
		t.Errorf("out =\n%s\nwant\n%s", out, want)
	}
}

func TestSetDisabledIn(t *testing.T) {
	t.Parallel()
	const other = "project:01JBZ7XXH2M4Q8R1T5V9W3Y6ZB"
	f := mustDecode(t, string(readFull(t)))
	if err := f.SetDisabledIn(other, "pr-opened", true); err != nil {
		t.Fatal(err)
	}
	if err := f.SetDisabledIn(other, "pr-opened", true); err != nil {
		t.Fatal(err)
	}
	if len(f.Scoped) != 2 || !slices.Equal(f.Scoped[1].Disable, []string{"pr-opened"}) {
		t.Fatalf("scoped = %+v", f.Scoped)
	}
	if err := f.SetDisabledIn(other, "none", true); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v", err)
	}
	back := mustDecode(t, mustEncode(t, f))
	if len(back.Scoped) != 2 || back.Scoped[1].Where != other {
		t.Errorf("scoped = %+v", back.Scoped)
	}
	if err := f.SetDisabledIn(other, "pr-opened", false); err != nil {
		t.Fatal(err)
	}
	if err := f.SetDisabledIn(other, "pr-opened", false); err != nil {
		t.Fatal(err)
	}
	if len(f.Scoped) != 1 {
		t.Errorf("scoped = %+v", f.Scoped)
	}
	if err := f.SetDisabledIn("project:01JBZ7XXH2M4Q8R1T5V9W3Y6ZA", "pr-review", false); err != nil {
		t.Fatal(err)
	}
	if len(f.Scoped) != 0 {
		t.Errorf("an empty scoped stayed: %+v", f.Scoped)
	}
}

func TestSetIssueFields(t *testing.T) {
	t.Parallel()
	f := mustDecode(t, string(readFull(t)))
	push := []string{"title", "milestone"}
	mirror := []string{"title"}
	sync := "push"
	err := f.SetIssueFields(IssueFieldsChange{DefaultSync: &sync, Push: &push, Mirror: &mirror, Props: map[string]string{"milestone": "due"}})
	if err != nil {
		t.Fatal(err)
	}
	back := mustDecode(t, mustEncode(t, f))
	i := back.IssueFields
	if i.DefaultSync != "push" || !slices.Equal(i.Push, push) || !slices.Equal(i.Mirror, mirror) || i.Props["milestone"] != "due" {
		t.Errorf("issue_fields = %+v", i)
	}
	if out := mustEncode(t, f); !strings.Contains(out, "note: unknown key") {
		t.Errorf("unknown key lost:\n%s", out)
	}
	if err := f.SetIssueFields(IssueFieldsChange{Props: map[string]string{"milestone": ""}}); err != nil {
		t.Fatal(err)
	}
	if len(f.IssueFields.Props) != 0 {
		t.Errorf("props = %v", f.IssueFields.Props)
	}
	before := f.IssueFields
	bad := "sometimes"
	for _, ch := range []IssueFieldsChange{
		{DefaultSync: &bad},
		{Push: &[]string{"nope"}},
		{Mirror: &[]string{"nope"}},
		{Props: map[string]string{"owner": "x"}},
	} {
		if err := f.SetIssueFields(ch); err == nil {
			t.Errorf("%+v: no error", ch)
		}
	}
	if !slices.Equal(before.Push, f.IssueFields.Push) || before.DefaultSync != f.IssueFields.DefaultSync {
		t.Errorf("a refused change was kept: %+v", f.IssueFields)
	}
}

func TestRemoveRuleKeepsOtherComments(t *testing.T) {
	t.Parallel()
	f := mustDecode(t, string(readFull(t)))
	if err := f.RemoveRule("pr-opened"); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(string(readFull(t)), "- id: pr-opened\n  on: pr.opened\n  set: {status: in_progress}\n", "", 1)
	if out := mustEncode(t, f); out != want {
		t.Errorf("out =\n%s\nwant\n%s", out, want)
	}
}

func TestDecodeErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{"rules: [\n", "rules: 3\n", "- a\n", "scoped: x\n"} {
		if _, err := Decode([]byte(src)); err == nil {
			t.Errorf("%q: no error", src)
		}
	}
}
