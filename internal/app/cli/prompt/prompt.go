// Package prompt is how the CLI asks a person on the terminal: a yes or no
// (Confirm), and a pick from a list (Choose).
package prompt

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// UsageError is an answer that is not one of the answers there are. The CLI
// treats it as a mistake in the arguments: it exits 2.
type UsageError struct{ error }

func (e UsageError) Unwrap() error { return e.error }

func usagef(format string, args ...any) error {
	return UsageError{fmt.Errorf(format, args...)}
}

// Asker asks on the terminal. Out is stderr, ReadLine reads one line, and
// Terminal is whether stdin is a terminal.
type Asker struct {
	Out      io.Writer
	ReadLine func() (string, error)
	Terminal bool
}

// Interactive is whether a person can answer.
func (a Asker) Interactive() bool { return a.Terminal }

// YesNo is a question with a yes or a no for the answer.
type YesNo struct {
	Question string // shown as it is, with the "[Y/n] " in it
	Default  bool   // the empty answer
	Canceled string // the error when the input ends; "" gives false and no error
	Strict   bool   // an answer but y, yes, n and no is a usage error; otherwise it is no
	Refuse   string // the error for a no; "" gives false and no error
}

// Confirm asks q. The answer is lower-cased and trimmed: y and yes are yes, n
// and no are no.
func (a Asker) Confirm(q YesNo) (bool, error) {
	if _, err := fmt.Fprint(a.Out, q.Question); err != nil {
		return false, err
	}
	line, err := a.ReadLine()
	if err != nil && line == "" {
		if q.Canceled == "" {
			return false, nil // the end of the input, not an empty answer
		}
		return false, errors.New(q.Canceled)
	}
	yes := false
	switch ans := strings.ToLower(strings.TrimSpace(line)); ans {
	case "":
		yes = q.Default
	case "y", "yes":
		yes = true
	case "n", "no":
	default:
		if q.Strict {
			return false, usagef("%q is not y or n", strings.TrimSpace(line))
		}
	}
	if !yes && q.Refuse != "" {
		return false, errors.New(q.Refuse)
	}
	return yes, nil
}

// Choose is a question with one of Options for the answer.
type Choose struct {
	Question string   // the line above the list; for ByWord, the words before the options
	Options  []string // the values to pick from
	Labels   []string // for numbers: after each value; may be shorter than Options
	Prompt   string   // for numbers: the word before the answer ("Number" when empty)
	Default  string   // for numbers: the value the empty answer takes
	ByWord   bool     // pick by the word or its first letter: "[c]arry / [d]rop: "
	Canceled string   // the error when the input ends
}

// Choose asks c and returns the option picked.
func (a Asker) Choose(c Choose) (string, error) {
	if c.ByWord {
		return a.chooseByWord(c)
	}
	return a.chooseByNumber(c)
}

func (a Asker) chooseByWord(c Choose) (string, error) {
	hints := make([]string, len(c.Options))
	for i, o := range c.Options {
		if o == "" {
			return "", errors.New("prompt: an option is empty")
		}
		hints[i] = "[" + o[:1] + "]" + o[1:]
	}
	if _, err := fmt.Fprint(a.Out, c.Question+strings.Join(hints, " / ")+": "); err != nil {
		return "", err
	}
	line, err := a.ReadLine()
	if err != nil && line == "" {
		return "", errors.New(c.Canceled)
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	for _, o := range c.Options {
		if ans == o || (ans != "" && ans == o[:1]) {
			return o, nil
		}
	}
	return "", usagef("%q is not %s", strings.TrimSpace(line), strings.Join(c.Options, " or "))
}

func (a Asker) chooseByNumber(c Choose) (string, error) {
	var b strings.Builder
	if c.Question != "" {
		b.WriteString(c.Question + "\n")
	}
	def := 0
	for i, cand := range c.Options {
		fmt.Fprintf(&b, "  %d) %s", i+1, cand)
		if i < len(c.Labels) && c.Labels[i] != "" {
			b.WriteString(" " + c.Labels[i])
		}
		b.WriteString("\n")
		if c.Default != "" && cand == c.Default {
			def = i + 1
		}
	}
	b.WriteString(cmp.Or(c.Prompt, "Number"))
	if def > 0 {
		fmt.Fprintf(&b, " [%d]", def)
	}
	b.WriteString(": ")
	if _, err := fmt.Fprint(a.Out, b.String()); err != nil {
		return "", err
	}
	line, err := a.ReadLine()
	if err != nil && line == "" {
		return "", errors.New(c.Canceled)
	}
	text := strings.TrimSpace(line)
	if text == "" && def > 0 {
		return c.Options[def-1], nil
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 || n > len(c.Options) {
		return "", usagef("%q is not one of 1-%d", text, len(c.Options))
	}
	return c.Options[n-1], nil
}
