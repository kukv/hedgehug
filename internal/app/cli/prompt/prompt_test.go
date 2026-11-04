package prompt_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/cli/prompt"
)

type pcase struct {
	name, in string
	want     string // the value returned; "true"/"false" for a yes or no
	err      string
	usage    bool
	out      string
}

func asker(in string, term bool) (prompt.Asker, *strings.Builder) {
	var out strings.Builder
	r := strings.NewReader(in)
	return prompt.Asker{Out: &out, Terminal: term, ReadLine: func() (string, error) {
		var b strings.Builder
		for {
			c, err := r.ReadByte()
			if err != nil {
				return b.String(), err
			}
			b.WriteByte(c)
			if c == '\n' {
				return b.String(), nil
			}
		}
	}}, &out
}

func check(t *testing.T, c pcase, got string, err error, out string) {
	t.Helper()
	if c.err != "" {
		if err == nil || err.Error() != c.err {
			t.Fatalf("err = %v, want %q", err, c.err)
		}
		var ue prompt.UsageError
		if errors.As(err, &ue) != c.usage {
			t.Fatalf("usage = %v, want %v", !c.usage, c.usage)
		}
	} else if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != c.want {
		t.Fatalf("got %q, want %q", got, c.want)
	}
	if out != c.out {
		t.Fatalf("out = %q, want %q", out, c.out)
	}
}

func yn(b bool) string { return map[bool]string{true: "true", false: "false"}[b] }

// askBranchParent in add.go.
func TestConfirmBranch(t *testing.T) {
	const q = "Add as a child of HH-1 Parent? [Y/n] "
	for _, c := range []pcase{
		{"empty", "\n", "true", "", false, q},
		{"y", "y\n", "true", "", false, q},
		{"YES", "YES\n", "true", "", false, q},
		{"no line end", "y", "true", "", false, q},
		{"n", "n\n", "false", "", false, q},
		{"No", "No\n", "false", "", false, q},
		{"x", "x\n", "", `"x" is not y or n`, true, q},
		{"eof", "", "", "canceled", false, q},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, out := asker(c.in, true)
			yes, err := a.Confirm(prompt.YesNo{Question: q, Default: true, Canceled: "canceled", Strict: true})
			got := ""
			if err == nil {
				got = yn(yes)
			}
			check(t, c, got, err, out.String())
		})
	}
}

// askRetry in editor.go. "yes" is yes now (decision 7 of the plan).
func TestConfirmRetry(t *testing.T) {
	const q = "Open the editor again? [Y/n] "
	for _, c := range []pcase{
		{"empty", "\n", "true", "", false, q},
		{"y", "y\n", "true", "", false, q},
		{"Y", "Y\n", "true", "", false, q},
		{"yes", "yes\n", "true", "", false, q},
		{"YES", "YES\n", "true", "", false, q},
		{"n", "n\n", "false", "", false, q},
		{"x", "x\n", "false", "", false, q},
		{"eof", "", "false", "", false, q},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, out := asker(c.in, true)
			yes, err := a.Confirm(prompt.YesNo{Question: q, Default: true})
			check(t, c, yn(yes), err, out.String())
		})
	}
	t.Run("not a terminal", func(t *testing.T) {
		a, out := asker("y\n", false)
		if a.Interactive() || out.Len() != 0 {
			t.Fatalf("interactive = %v, out = %q", a.Interactive(), out.String())
		}
	})
}

// confirmMove and confirmPurge.
func TestConfirmRefuse(t *testing.T) {
	const q = "Move 0 tasks to ? [y/N] "
	for _, c := range []pcase{
		{"empty", "\n", "", "Canceled", false, q},
		{"n", "n\n", "", "Canceled", false, q},
		{"x", "x\n", "", "Canceled", false, q},
		{"y", "y\n", "ok", "", false, q},
		{"YES", "YES\n", "ok", "", false, q},
		{"eof", "", "", "Canceled", false, q},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, out := asker(c.in, true)
			_, err := a.Confirm(prompt.YesNo{Question: q, Canceled: "Canceled", Refuse: "Canceled"})
			got := ""
			if err == nil {
				got = "ok"
			}
			check(t, c, got, err, out.String())
		})
	}
}

// askScoped in mv.go.
func TestChooseByWord(t *testing.T) {
	const q = "The scoped schema of A, B — [c]arry / [d]rop: "
	for _, c := range []pcase{
		{"c", "c\n", "carry", "", false, q},
		{"Carry", "Carry\n", "carry", "", false, q},
		{"d", "d\n", "drop", "", false, q},
		{"drop", "drop\n", "drop", "", false, q},
		{"empty", "\n", "", `"" is not carry or drop`, true, q},
		{"x", "x\n", "", `"x" is not carry or drop`, true, q},
		{"eof", "", "", "Canceled", false, q},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, out := asker(c.in, true)
			got, err := a.Choose(prompt.Choose{Question: "The scoped schema of A, B — ", Options: []string{"carry", "drop"}, ByWord: true, Canceled: "Canceled"})
			check(t, c, got, err, out.String())
		})
	}
}

// An empty option cannot be told by its first letter: it is an error, not a
// panic.
func TestChooseByWordRejectsAnEmptyOption(t *testing.T) {
	a, _ := asker("c\n", true)
	if _, err := a.Choose(prompt.Choose{Question: "Q ", Options: []string{"carry", ""}, ByWord: true, Canceled: "Canceled"}); err == nil {
		t.Fatal("err = nil; want an error for the empty option")
	}
}

// askChoice in store_init.go and askPick in add.go.
func TestChooseByNumber(t *testing.T) {
	opts := []string{"x", "y", "z"}
	plain := "Which?\n  1) x\n  2) y\n  3) z\nNumber: "
	labeled := "  1) x (one)\n  2) y\n  3) z\nProject [2]: "
	for _, c := range []pcase{
		{"2", "2\n", "y", "", false, plain},
		{"empty no default", "\n", "", `"" is not one of 1-3`, true, plain},
		{"0", "0\n", "", `"0" is not one of 1-3`, true, plain},
		{"4", "4\n", "", `"4" is not one of 1-3`, true, plain},
		{"word", "q\n", "", `"q" is not one of 1-3`, true, plain},
		{"eof", "", "", "canceled", false, plain},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, out := asker(c.in, true)
			got, err := a.Choose(prompt.Choose{Question: "Which?", Options: opts, Canceled: "canceled"})
			check(t, c, got, err, out.String())
		})
	}
	for _, c := range []pcase{
		{"empty takes default", "\n", "y", "", false, labeled},
		{"3", "3\n", "z", "", false, labeled},
	} {
		t.Run("labeled "+c.name, func(t *testing.T) {
			a, out := asker(c.in, true)
			got, err := a.Choose(prompt.Choose{Options: opts, Labels: []string{"(one)"}, Prompt: "Project", Default: "y", Canceled: "canceled"})
			check(t, c, got, err, out.String())
		})
	}
}
