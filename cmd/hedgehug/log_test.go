package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestLog(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("First")
	b := te.addJSON("Second")
	te.ok("set", a.Short, "labels+=x")
	te.ok("comment", a.Short, "hello")

	lines := func(args ...string) []string {
		return strings.Split(strings.TrimRight(te.ok(args...), "\n"), "\n")
	}
	all := lines("log")
	if len(all) != 5 {
		t.Fatalf("log has %d lines, want 5:\n%s", len(all), strings.Join(all, "\n"))
	}
	re := regexp.MustCompile(`^[0-9a-f]{7}  \d{4}-\d{2}-\d{2} \d{2}:\d{2}  Alice  hedgehug: `)
	for _, l := range all {
		if !re.MatchString(l) {
			t.Errorf("log line %q does not match %s", l, re)
		}
	}
	if !strings.Contains(all[0], "comment") || !strings.Contains(all[4], "init") {
		t.Errorf("log is not newest first:\n%s", strings.Join(all, "\n"))
	}
	c := te.log("me")[0]
	want := c.Hash.String()[:7] + "  " + c.Author.When.In(time.Local).Format("2006-01-02 15:04") + "  Alice  "
	if !strings.HasPrefix(all[0], want) {
		t.Errorf("first line = %q, want prefix %q", all[0], want)
	}

	if got := lines("log", a.Short); len(got) != 3 {
		t.Errorf("log <id> has %d lines, want 3:\n%s", len(got), strings.Join(got, "\n"))
	}
	if got := lines("log", b.Short); len(got) != 1 {
		t.Errorf("log <other> has %d lines, want 1", len(got))
	}
	if got := lines("log", "-n", "1"); len(got) != 1 {
		t.Errorf("log -n 1 has %d lines, want 1", len(got))
	}
	if code, _, _ := te.run("log", "-n", "-1"); code != 2 {
		t.Errorf("log -n -1 exit = %d, want 2", code)
	}
}

func TestLogJSON(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("First")
	var got []struct {
		Hash   string
		Date   string
		Author struct{ Name, Email string }
		Op     *string
		Task   *string
		Subj   string `json:"subject"`
	}
	out := te.ok("log", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("log --json has %d entries, want 2", len(got))
	}
	e := got[0]
	if len(e.Hash) != 40 || e.Date == "" || e.Author.Name != "Alice" || e.Author.Email != "alice@example.com" {
		t.Errorf("entry = %+v", e)
	}
	if e.Op == nil || *e.Op != "add" || e.Task == nil || *e.Task != a.ID || !strings.HasPrefix(e.Subj, "hedgehug: add ") {
		t.Errorf("entry = %+v, want op add and task %s", e, a.ID)
	}
	if got[1].Op == nil || *got[1].Op != "init" || got[1].Task != nil {
		t.Errorf("init entry = %+v, want op init and no task", got[1])
	}
	if !strings.Contains(out, `"task": null`) {
		t.Errorf("task null is not printed:\n%s", out)
	}
}

func TestShowHistory(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("First")
	for range 6 {
		te.ok("comment", a.Short, "c")
	}
	out := te.ok("show", a.Short)
	_, hist, ok := strings.Cut(out, "\nHistory:\n")
	if !ok {
		t.Fatalf("show has no History:\n%s", out)
	}
	if n := len(strings.Split(strings.TrimRight(hist, "\n"), "\n")); n != 5 {
		t.Errorf("History has %d lines, want 5:\n%s", n, hist)
	}
	var j struct{ History []struct{ Hash string } }
	if err := json.Unmarshal([]byte(te.ok("show", a.Short, "--json")), &j); err != nil {
		t.Fatal(err)
	}
	if len(j.History) != 5 || len(j.History[0].Hash) != 40 {
		t.Errorf("history = %+v", j.History)
	}
}
