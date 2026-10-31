package main

import (
	"strings"
	"testing"
)

func TestStatusAndDone(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.ok("status", a.ID, "in_progress")
	if s := te.task(a.ULID).Status; s != "in_progress" {
		t.Errorf("status = %q, want in_progress", s)
	}
	te.ok("done", a.ID)
	if s := te.task(a.ULID).Status; s != "done" {
		t.Errorf("status = %q, want done", s)
	}
	log := te.log("me")
	if !strings.Contains(log[0].Message, "Hedgehug-Op: status") || !strings.Contains(log[1].Message, "Hedgehug-Op: status") {
		t.Errorf("ops = %q, %q", log[0].Message, log[1].Message)
	}
}

func TestStatusErrors(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	if code, _, stderr := te.run("status", a.ID, "doing"); code != 1 || !strings.Contains(stderr, "in_progress") {
		t.Errorf("a category name: exit %d, stderr %q (want the status names listed)", code, stderr)
	}
	if code, _, _ := te.run("status", a.ID); code != 2 {
		t.Errorf("status with no status: exit %d, want 2", code)
	}
}
