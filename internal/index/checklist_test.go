package index

import "testing"

func TestChecklist(t *testing.T) {
	for _, c := range []struct {
		name        string
		body        string
		done, total int
	}{
		{"none", "just text\n- a plain item\n", 0, 0},
		{"markers", "- [ ] a\n* [x] b\n+ [X] c\n1. [ ] d\n2) [x] e\n", 3, 5},
		{"nested", "- [ ] a\n  - [x] b\n    - [ ] c\n", 1, 3},
		{"needs a space after", "- [x]b\n-[x] c\n- [x]\n", 1, 1},
		{"not a checkbox", "- [y] a\n- [] b\n- [ x] c\n", 0, 0},
		{"fenced", "- [ ] a\n```\n- [x] in code\n```\n~~~md\n- [ ] in code\n~~~\n- [x] b\n", 1, 2},
		{"longer fence closes only on its length", "````\n```\n- [ ] in code\n````\n- [ ] a\n", 0, 1},
		{"unclosed fence hides the rest", "- [x] a\n```\n- [ ] b\n", 1, 1},
		{"crlf", "- [ ] a\r\n- [x] b\r\n", 1, 2},
	} {
		d, n := Checklist(c.body)
		if d != c.done || n != c.total {
			t.Errorf("%s: Checklist = %d/%d; want %d/%d", c.name, d, n, c.done, c.total)
		}
	}
}

func TestEntryChecklist(t *testing.T) {
	s, _ := newStore(t)
	a := newTask("a")
	a.Body = "- [x] a\n- [ ] b\n"
	put(t, s, a)
	for name, ix := range map[string]*Index{"built": mustOpen(t, s), "cache": mustOpen(t, s)} {
		e, _ := ix.Task(a.ULID)
		if e.ChecklistDone != 1 || e.ChecklistTotal != 2 {
			t.Errorf("%s: checklist = %d/%d; want 1/2", name, e.ChecklistDone, e.ChecklistTotal)
		}
	}
}
