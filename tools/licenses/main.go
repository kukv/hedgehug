// Command licenses writes to stdout the license texts of the third-party code
// in the released binary: the Go standard library, the Go modules linked into
// ./cmd/hedgehug on every release platform, and then each file named in the
// arguments as is (the web bundle's, which web/vite.config.ts writes). It
// fails when a module has no license file. The release archives ship its
// output as THIRD_PARTY_LICENSES (make licenses).
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// platforms are the GOOS/GOARCH pairs in .goreleaser.yaml. The modules linked
// differ between them (go-keyring uses dbus on Linux only).
var platforms = [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}}

// licenseFile matches the files at a module's root that carry its license or
// a notice that must go with it.
var licenseFile = regexp.MustCompile(`(?i)^(licen[cs]e|copying|notice|patents)([.\-_].*)?$`)

type module struct {
	Path, Version, Dir string
	Main               bool
	Replace            *module
}

func main() {
	if err := run(os.Stdout, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "licenses:", err)
		os.Exit(1)
	}
}

func run(w io.Writer, extra []string) error {
	mods := map[string]module{}
	for _, p := range platforms {
		out, err := goCmd(p, "list", "-deps", "-json=Module", "github.com/kukv/hedgehug/cmd/hedgehug")
		if err != nil {
			return err
		}
		dec := json.NewDecoder(bytes.NewReader(out))
		for dec.More() {
			var pkg struct{ Module *module }
			if err := dec.Decode(&pkg); err != nil {
				return err
			}
			if m := pkg.Module; m != nil && !m.Main {
				if m.Replace != nil {
					m = m.Replace
				}
				mods[m.Path] = *m
			}
		}
	}
	goroot, err := goCmd(platforms[0], "env", "GOROOT", "GOVERSION")
	if err != nil {
		return err
	}
	root, version, _ := strings.Cut(strings.TrimSpace(string(goroot)), "\n")

	var b strings.Builder
	b.WriteString("Hedgehug includes the following third-party software. Each is under the license below it.\n")
	if err := section(&b, "Go "+strings.TrimPrefix(version, "go")+" (standard library)", root); err != nil {
		return err
	}
	for _, path := range slices.Sorted(maps.Keys(mods)) {
		m := mods[path]
		if err := section(&b, m.Path+" "+m.Version, m.Dir); err != nil {
			return err
		}
	}
	for _, f := range extra {
		text, err := osfs.ReadFile(f)
		if err != nil {
			return err
		}
		b.WriteString("\n" + string(text))
	}
	_, err = io.WriteString(w, b.String())
	return err
}

// section writes the license files at the root of dir under a heading.
func section(b *strings.Builder, title, dir string) error {
	entries, err := osfs.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && licenseFile.MatchString(e.Name()) {
			files = append(files, e.Name())
		}
	}
	if !slices.ContainsFunc(files, func(n string) bool { return !strings.HasPrefix(strings.ToLower(n), "notice") }) {
		return errors.New("no license file for " + title + " in " + dir)
	}
	b.WriteString("\n==== " + title + " ====\n")
	for _, f := range files {
		text, err := osfs.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return err
		}
		b.WriteString("\n---- " + f + " ----\n\n" + strings.TrimRight(string(text), "\n") + "\n")
	}
	return nil
}

func goCmd(p [2]string, args ...string) ([]byte, error) {
	cmd := osfs.Command("go", args...)
	cmd.Env = append(os.Environ(), "GOOS="+p[0], "GOARCH="+p[1], "CGO_ENABLED=0")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go %s (%s/%s): %w", strings.Join(args, " "), p[0], p[1], err)
	}
	return out, nil
}
