package main

import (
	"encoding/json"
)

// style colors text when the output is a terminal.
type style struct{ on bool } //nolint:unused // used by the commands that follow in phase 1d-1

func (a *app) style() style { //nolint:unused // used by the commands that follow in phase 1d-1
	return style{on: !a.noColor && a.getenv("NO_COLOR") == "" && a.isTerminal(a.stdout)}
}

func (s style) wrap(code, t string) string { //nolint:unused // used by the commands that follow in phase 1d-1
	if !s.on {
		return t
	}
	return "\x1b[" + code + "m" + t + "\x1b[0m"
}

// id colors a short ID.
func (s style) id(t string) string { return s.wrap("33", t) } //nolint:unused // used by the commands that follow in phase 1d-1

// dim marks text that is context, not a result. Without color it is put in
// parentheses.
func (s style) dim(t string) string { //nolint:unused // used by the commands that follow in phase 1d-1
	if !s.on {
		return "(" + t + ")"
	}
	return s.wrap("2", t)
}

// printJSON writes v as indented JSON.
func (a *app) printJSON(v any) error {
	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
