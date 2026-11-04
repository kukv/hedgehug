package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStoreSet(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("store", "init", "org", "--prefix", "ORG")

	if out := te.ok("store", "set", "org", "--color", "teal"); out != "Set org: color=teal hidden=false\n" {
		t.Errorf("color: %q", out)
	}
	// only what is given changes
	if out := te.ok("store", "set", "org", "--hidden"); out != "Set org: color=teal hidden=true\n" {
		t.Errorf("hidden: %q", out)
	}
	// "" gives the color back; the empty color is written as -
	if out := te.ok("store", "set", "org", "--color", "", "--hidden=false"); out != "Set org: color=- hidden=false\n" {
		t.Errorf("reset: %q", out)
	}
	te.goldenOK("set-json", "store", "set", "org", "--color", "sky", "--hidden=true", "--json")
	// store list --json carries hidden always, and color when it is set
	out := te.goldenOK("list-json", "store", "list", "--json")
	var list []struct {
		Name   string `json:"name"`
		Color  string `json:"color"`
		Hidden bool   `json:"hidden"`
	}
	if err := json.Unmarshal([]byte(out), &list); err != nil || len(list) != 2 {
		t.Fatalf("list = %s, %v", out, err)
	}
	if list[0].Color != "" || list[0].Hidden || list[1].Color != "sky" || !list[1].Hidden {
		t.Errorf("list = %+v", list)
	}
}

func TestStoreSetErrors(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("store", "init", "org", "--prefix", "ORG")
	for _, tc := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"neither flag", []string{"store", "set", "org"}, 2, "--color or --hidden"},
		{"no name", []string{"store", "set", "--color", "teal"}, 2, ""},
		{"bad color", []string{"store", "set", "org", "--color", "red"}, 1, "teal, orange, purple, sky, pink, olive, brown"},
		{"grey", []string{"store", "set", "org", "--color", "grey"}, 1, "not a store color"},
		{"bad hidden", []string{"store", "set", "org", "--hidden=maybe"}, 2, ""},
		{"unknown store", []string{"store", "set", "nope", "--color", "teal"}, 1, "me, org"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errs := te.run(tc.args...)
			if code != tc.code || !strings.Contains(errs, tc.want) {
				t.Errorf("exit %d, stderr %q; want exit %d with %q", code, errs, tc.code, tc.want)
			}
		})
	}
}
