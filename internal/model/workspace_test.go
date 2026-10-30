package model

import (
	"errors"
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
