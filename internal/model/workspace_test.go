package model

import (
	"errors"
	"strings"
	"testing"
)

func TestWorkspaceRoundTrip(t *testing.T) {
	in := "---\nid: 01JC0A1B2C3D4E5F6G7H8J9K0M\nname: web\nkey_prefix: WEB\nprefix_set: 2026-10-31T10:00:00+09:00\ncreated: 2026-10-31T10:00:00+09:00\nupdated: 2026-10-31T10:00:00+09:00\n---\nWeb のタスク。\n"
	w, err := DecodeWorkspace([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "web" || w.KeyPrefix != "WEB" || w.PrefixSet.IsZero() || w.Body != "Web のタスク。\n" {
		t.Errorf("%+v", w)
	}
	out, err := w.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != in {
		t.Errorf("Encode =\n%s", out)
	}
	if _, err := DecodeWorkspace([]byte("---\nname: x\n---\n")); !errors.As(err, new(*DecodeError)) {
		t.Errorf("no id: %v", err)
	}
	if WorkspacePath(w.ID) != "workspaces/01JC0A1B2C3D4E5F6G7H8J9K0M.md" {
		t.Error(WorkspacePath(w.ID))
	}
	if id, err := ParseWorkspacePath(WorkspacePath(w.ID)); err != nil || id != w.ID {
		t.Errorf("ParseWorkspacePath = %v %v", id, err)
	}
}

func TestValidName(t *testing.T) {
	for _, ok := range []string{"a", "web", "web-2", "my_ws", "0x"} {
		if err := ValidName("store", ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Web", "-a", "_a", "a b", "a/b", "é"} {
		err := ValidName("store", bad)
		if err == nil || !strings.Contains(err.Error(), "cannot name a store") {
			t.Errorf("%q: %v", bad, err)
		}
		if err := ValidWorkspaceName(bad); err == nil || !strings.Contains(err.Error(), "cannot name a workspace") {
			t.Errorf("workspace %q: %v", bad, err)
		}
	}
}
