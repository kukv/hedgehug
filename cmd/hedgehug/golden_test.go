package main

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

var (
	ulidRE = regexp.MustCompile(`\b[0-9A-HJKMNP-TV-Z]{26}\b`)
	hashRE = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	timeRE = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`)
	// The directories under tasks/ are the first characters of the ULID.
	shardRE     = regexp.MustCompile(`tasks/[0-9A-Z]/[0-9A-Z]/`)
	columnGapRE = regexp.MustCompile(`(\S) {2,}`)
	tmpPathRE   = regexp.MustCompile(`<TMP>[^\s",:]*`)
	// The commit that undo names by its first seven characters.
	shortHashRE = regexp.MustCompile(`(Undid )[0-9a-f]{7}\b`)
	// The commit and the local time that a line of show's History or of log
	// starts with; the time is in the zone of the machine.
	logHashRE = regexp.MustCompile(`(?m)^[0-9a-f]{7}  \d{4}-\d{2}-\d{2} \d{2}:\d{2}`)
)

// normalize replaces what changes from run to run: the temporary directories
// of the test, ULIDs (numbered by first appearance, so which one is which
// stays visible), times and commit hashes. IDs, titles and messages are kept.
func (te *testEnv) normalize(s string) string {
	tmp := filepath.Dir(filepath.Dir(te.home))
	// JSON writes the \ of a Windows path as \\.
	escaped := strings.ReplaceAll(tmp, `\`, `\\`)
	if strings.Contains(s, tmp) || strings.Contains(s, escaped) {
		s = strings.ReplaceAll(strings.ReplaceAll(s, escaped, "<TMP>"), tmp, "<TMP>")
		// A path under it is written with / on every system.
		s = tmpPathRE.ReplaceAllStringFunc(s, func(p string) string {
			return strings.ReplaceAll(strings.ReplaceAll(p, `\\`, "/"), `\`, "/")
		})
		// The length of the directory changes the padding of a table.
		s = columnGapRE.ReplaceAllString(s, "$1  ")
	}
	s = shardRE.ReplaceAllString(s, "tasks/<SHARD>/")
	ulids := map[string]string{}
	s = ulidRE.ReplaceAllStringFunc(s, func(u string) string {
		if _, ok := ulids[u]; !ok {
			ulids[u] = "<ULID-" + strconv.Itoa(len(ulids)+1) + ">"
		}
		return ulids[u]
	})
	s = hashRE.ReplaceAllString(s, "<HASH>")
	s = shortHashRE.ReplaceAllString(s, "${1}<HASH>")
	s = logHashRE.ReplaceAllString(s, "<HASH>  <TIME>")
	return timeRE.ReplaceAllString(s, "<TIME>")
}

// golden compares got, normalized, with testdata/<test name>/<label>.golden
// exactly. go test -update rewrites the file.
func (te *testEnv) golden(label, got string) {
	te.t.Helper()
	got = te.normalize(got)
	path := filepath.Join("testdata", te.t.Name(), label+".golden")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			te.t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			te.t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // a golden file of the test
	if err != nil {
		te.t.Fatalf("%v (go test -update writes it)", err)
	}
	if got != string(want) {
		te.t.Errorf("%s differs from %s (go test -update rewrites it)\n got: %q\nwant: %q", label, path, got, want)
	}
}

// goldenOK runs the command, fails the test unless it exits 0, compares its
// stdout with the golden file label, and returns the stdout.
func (te *testEnv) goldenOK(label string, args ...string) string {
	te.t.Helper()
	out := te.ok(args...)
	te.golden(label, out)
	return out
}

// goldenRun runs the command, which has to exit with code, and compares its
// stdout and stderr with the golden files label.out and label.err.
func (te *testEnv) goldenRun(label string, code int, args ...string) {
	te.t.Helper()
	got, out, errs := te.run(args...)
	if got != code {
		te.t.Fatalf("hedgehug %s: exit %d, want %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), got, code, out, errs)
	}
	te.golden(label+".out", out)
	te.golden(label+".err", errs)
}
