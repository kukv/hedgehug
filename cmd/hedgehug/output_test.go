package main

import (
	"strings"
	"testing"
)

func TestWriteTableFullWidth(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	rows := [][]string{{"今週", "table"}, {"tree", "tree"}}
	if err := writeTable(&b, []string{"NAME", "LAYOUT"}, rows, nil); err != nil {
		t.Fatal(err)
	}
	want := "NAME  LAYOUT\n今週  table\ntree  tree\n"
	if b.String() != want {
		t.Errorf("writeTable = %q, want %q", b.String(), want)
	}
}
