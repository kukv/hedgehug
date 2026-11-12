package archtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type goPackage struct {
	ImportPath   string
	Dir          string
	GoFiles      []string
	TestGoFiles  []string
	XTestGoFiles []string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

func listPackages(t *testing.T) []goPackage {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var pkgs []goPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p goPackage
		err := dec.Decode(&p)
		if errors.Is(err, io.EOF) {
			return pkgs
		}
		if err != nil {
			t.Fatalf("decode go list: %v", err)
		}
		pkgs = append(pkgs, p)
	}
}

// TestImports は、すべてのパッケージの import が設計書 2 節の表に合うことを確かめる。
func TestImports(t *testing.T) {
	for _, p := range listPackages(t) {
		for _, imp := range p.Imports {
			if !Allowed(p.ImportPath, imp, false) {
				t.Errorf("%s imports %s (設計書 2 節)", p.ImportPath, imp)
			}
		}
		for _, imp := range append(p.TestImports, p.XTestImports...) {
			if !Allowed(p.ImportPath, imp, true) {
				t.Errorf("tests of %s import %s (設計書 2 節)", p.ImportPath, imp)
			}
		}
	}
}

// TestSizes は、設計書 4 節のファイルとパッケージの大きさの上限を確かめる。
func TestSizes(t *testing.T) {
	for _, p := range listPackages(t) {
		if len(p.GoFiles) > maxPackageFiles {
			t.Errorf("%s has %d non-test files (max %d): split the package", p.ImportPath, len(p.GoFiles), maxPackageFiles)
		}
		checkLines(t, p.Dir, p.GoFiles, maxFileLines)
		checkLines(t, p.Dir, append(p.TestGoFiles, p.XTestGoFiles...), maxTestFileLines)
	}
}

func checkLines(t *testing.T, dir string, files []string, limit int) {
	t.Helper()
	for _, f := range files {
		src, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if n := lineCount(src); n > limit && !isGenerated(src) {
			t.Errorf("%s has %d lines (max %d): split the file", filepath.Join(dir, f), n, limit)
		}
	}
}
