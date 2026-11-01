package model

import "testing"

func mustULID(t *testing.T, s string) ULID {
	t.Helper()
	ulid, err := ParseULID(s)
	if err != nil {
		t.Fatal(err)
	}
	return ulid
}

func TestPaths(t *testing.T) {
	task := mustULID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0A")
	c := mustULID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")
	for _, tc := range []struct{ got, want string }{
		{TaskPath(task), "tasks/A/0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md"},
		{CommentPath(task, c), "comments/A/0/01JBZ8K3QW7M2X9V4T6R5N8P0A/01JBZ8K3QW7M2X9V4T6R5N8P0B.md"},
		{WorklogPath(task, c), "worklogs/A/0/01JBZ8K3QW7M2X9V4T6R5N8P0A/01JBZ8K3QW7M2X9V4T6R5N8P0B.md"},
		{SprintPath(task), "sprints/01JBZ8K3QW7M2X9V4T6R5N8P0A.md"},
		{ReleasePath(task), "releases/01JBZ8K3QW7M2X9V4T6R5N8P0A.md"},
		{TemplatePath("bug-report"), "templates/bug-report.md"},
		{StoreMetaPath, ".hedgehug/store.yaml"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

func TestValidTemplateName(t *testing.T) {
	for _, ok := range []string{"bug-report", "release-checklist", "リリース", "v2.4"} {
		if err := ValidTemplateName(ok); err != nil {
			t.Errorf("ValidTemplateName(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "a/b", `a\b`, "..", "a..b", "../x", ".hidden", "a\x00b", "a\nb", "a\x7fb"} {
		if err := ValidTemplateName(bad); err == nil {
			t.Errorf("ValidTemplateName(%q) succeeded", bad)
		}
	}
}

func TestParseTaskPath(t *testing.T) {
	want := mustULID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0A")
	got, err := ParseTaskPath(TaskPath(want))
	if err != nil || got != want {
		t.Errorf("ParseTaskPath = %v, %v; want %v", got, err, want)
	}
	for _, bad := range []string{
		"tasks/01JBZ8K3QW7M2X9V4T6R5N8P0A.md",        // flat
		"tasks/B/0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md",    // wrong shard
		"tasks/A/0/01JBZ8K3QW7M2X9V4T6R5N8P0A.txt",   // wrong extension
		"comments/A/0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md", // wrong root
		"tasks/A/0/nope.md",
	} {
		if _, err := ParseTaskPath(bad); err == nil {
			t.Errorf("ParseTaskPath(%q) succeeded", bad)
		}
	}
}

func TestParseCommentAndWorklogPath(t *testing.T) {
	task := mustULID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0A")
	c := mustULID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")
	gt, gc, err := ParseCommentPath(CommentPath(task, c))
	if err != nil || gt != task || gc != c {
		t.Errorf("ParseCommentPath = %v, %v, %v", gt, gc, err)
	}
	gt, gc, err = ParseWorklogPath(WorklogPath(task, c))
	if err != nil || gt != task || gc != c {
		t.Errorf("ParseWorklogPath = %v, %v, %v", gt, gc, err)
	}
	if _, _, err := ParseCommentPath(WorklogPath(task, c)); err == nil {
		t.Error("ParseCommentPath accepted a worklog path")
	}
	if _, _, err := ParseCommentPath("comments/B/0/01JBZ8K3QW7M2X9V4T6R5N8P0A/01JBZ8K3QW7M2X9V4T6R5N8P0B.md"); err == nil {
		t.Error("ParseCommentPath accepted a wrong shard")
	}
}

func TestViewPath(t *testing.T) {
	if got := ViewPath("今週"); got != "views/今週.yaml" {
		t.Errorf("ViewPath = %q", got)
	}
	for _, c := range []struct {
		p    string
		name string
		ok   bool
	}{
		{"views/a.yaml", "a", true},
		{"views/今週 の作業.yaml", "今週 の作業", true},
		{"views/a.yml", "", false},
		{"views/sub/a.yaml", "", false},
		{"schema.yaml", "", false},
		{"views/.yaml", "", true},
	} {
		name, ok := IsViewPath(c.p)
		if name != c.name || ok != c.ok {
			t.Errorf("IsViewPath(%q) = %q, %v; want %q, %v", c.p, name, ok, c.name, c.ok)
		}
	}
}
