package model

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestTaskRoundTrip(t *testing.T) {
	src := readFile(t, "testdata/task/full.md")
	task, err := DecodeTask(src)
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "WEB-42" || task.Title != "ログイン画面の改修" || task.Rank != "a0V" ||
		task.Parent == nil || task.Sprint == nil || len(task.BlockedBy) != 1 ||
		len(task.Links) != 1 || task.Links[0].Sync != "mirror" ||
		task.Props.Get("points").Value != "5" || task.GH == nil || task.DeletedAt != nil ||
		task.Extra.Get("custom").Value != "kept" || !strings.HasPrefix(task.Body, "メモ本文") {
		t.Errorf("decoded task = %+v", task)
	}
	if want := time.Date(2026, 10, 29, 3, 30, 0, 0, time.UTC); !task.Updated.Equal(want) {
		t.Errorf("Updated = %v, want %v", task.Updated, want)
	}
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(src) {
		t.Errorf("round trip changed the file:\n%s", out)
	}
}

func TestTaskLinksKeepUnknownKeys(t *testing.T) {
	src := readFile(t, "testdata/task/links.md")
	task, err := DecodeTask(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := task.Encode()
	if err != nil || string(out) != string(src) {
		t.Fatalf("round trip changed the file (%v):\n%s", err, out)
	}
	task.Links = append(task.Links, Link{Type: "github_pr", Ref: "github.com/org-a/web/pull/124"})
	out, err = task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"links:\n# 最初のリンク\n- type: github_pr # PR\n",
		"  reviewed: true # 未知のキー\n",
		"- {type: github_issue, ref: github.com/org-a/web/issues/7, note: 調査中} # flow\n",
		"- type: github_pr\n  ref: github.com/org-a/web/pull/124\n---\n",
	} {
		if !contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestTaskEncodeChanges(t *testing.T) {
	task, err := DecodeTask(readFile(t, "testdata/task/full.md"))
	if err != nil {
		t.Fatal(err)
	}
	task.ID = "WEB-43"
	task.Labels = append(task.Labels, "api")
	task.Props.Delete("points")
	task.Extra.Delete("custom")
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"id: WEB-43 # 人が読める ID\n", "labels: [frontend, api]\n", "  priority: high # 優先度\n"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, s)
		}
	}
	for _, gone := range []string{"points:", "custom:"} {
		if strings.Contains(s, gone) {
			t.Errorf("output still has %q", gone)
		}
	}
}

func TestTaskReordersKnownKeys(t *testing.T) {
	src := "---\ntitle: A\ncustom: x\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n---\n"
	task, err := DecodeTask([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\ntitle: A\ncustom: x\n---\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTaskCRLF(t *testing.T) {
	src := "---\r\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\r\ntitle: A\r\n---\r\nline\r\n"
	task, err := DecodeTask([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := task.Encode()
	if got, want := string(out), "---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\ntitle: A\n---\nline\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNewTaskEncode(t *testing.T) {
	ulid, _ := ParseULID("01JBZ8K3QW7M2X9V4T6R5N8P0A")
	created := time.Date(2026, 10, 29, 10, 0, 0, 0, time.FixedZone("", 9*3600))
	task := &Task{
		ULID: ulid, Kind: "task", Title: "新しいタスク", Rank: "a0", Status: "todo",
		Labels: []string{"x"}, Created: created, CreatedBy: "github.com/me", Updated: created, Body: "本文\n",
	}
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nkind: task\ntitle: 新しいタスク\nrank: a0\nstatus: todo\nlabels: [x]\n" +
		"created: 2026-10-29T10:00:00+09:00\ncreated_by: github.com/me\nupdated: 2026-10-29T10:00:00+09:00\n---\n本文\n"
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestDecodeTaskRefusesForward(t *testing.T) {
	// A file with moved_to is a forward, not a task: DecodeForward reads it.
	_, err := DecodeTask([]byte("---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nmoved_to: 01JC0M3R5W7M2X9V4T6R5N8P1B\n---\n"))
	if !errors.Is(err, ErrForward) {
		t.Errorf("DecodeTask err = %v, want ErrForward", err)
	}
}

func TestDecodeTaskErrors(t *testing.T) {
	for _, src := range []string{
		"---\ntitle: no ulid\n---\n",
		"---\nulid: nope\n---\n",
		"---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nparent: [1]\n---\n",
		"no frontmatter",
	} {
		if _, err := DecodeTask([]byte(src)); err == nil {
			t.Errorf("DecodeTask(%q) succeeded", src)
		}
	}
}

func TestTaskEncodeKeepsValidYAMLWithKeyComment(t *testing.T) {
	// "props: # c" holds a null; the comment sits on the key. Encoding an
	// empty mapping in its place must still give a file we can read.
	src := "---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nprops: #c\n---\n"
	task, err := DecodeTask([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeTask(out); err != nil {
		t.Fatalf("own output %q: %v", out, err)
	}
}

func TestTaskKeysIsACopy(t *testing.T) {
	k := TaskKeys()
	k[0] = "changed"
	if TaskKeys()[0] != "ulid" {
		t.Error("TaskKeys returned the shared slice")
	}
}
