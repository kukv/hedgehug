package model

import "testing"

func mustID(t *testing.T, s string) ID {
	t.Helper()
	id, err := ParseID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPaths(t *testing.T) {
	task := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0A")
	c := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")
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
	want := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0A")
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
	task := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0A")
	c := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")
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
