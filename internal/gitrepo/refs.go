package gitrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"

	"github.com/kukv/hedgehug/internal/atomicfile"
)

// SetReference replaces the reference file in one rename, so that a reader
// sees the old reference or the new one and never an empty file. go-git
// truncates the file and then writes it; a reader that opens it in between
// finds it empty, which go-git reports as "reference not found" (and
// Repo.Head as "no commit yet").
func (s *cachedStorage) SetReference(ref *plumbing.Reference) error {
	// The same gate go-git's dotgit applies before it writes any reference.
	if err := validReferenceName(ref.Name()); err != nil {
		return err
	}
	var content string
	switch ref.Type() {
	case plumbing.SymbolicReference:
		content = fmt.Sprintf("ref: %s\n", ref.Target())
	case plumbing.HashReference:
		content = ref.Hash().String() + "\n"
	default:
		return fmt.Errorf("reference %s has an unknown type", ref.Name())
	}
	path := filepath.Join(s.gitDir, filepath.FromSlash(ref.Name().String()))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // #nosec G301 -- matches go-git (osfs), which git itself reads
		return err
	}
	// The temporary file is outside refs/, where go-git lists references.
	f, err := os.CreateTemp(s.gitDir, "hh-ref-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	// CreateTemp makes the file 0600; go-git writes references as 0666 minus
	// the umask, and other tools expect them readable.
	err = f.Chmod(0o644) // #nosec G302 -- a reference is not a secret
	if err == nil {
		_, err = f.WriteString(content)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = atomicfile.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// isPathSep reports whether r is a path separator in reference names.
//
// Same as isPathSep in go-git v5.19.3 storage/filesystem/dotgit (Apache-2.0,
// Copyright go-git authors). Review it when go-git is upgraded.
func isPathSep(r rune) bool { return r == '/' || r == '\\' }

// validReferenceName rejects reference names that cannot be safely turned
// into a path under the git directory: names outside refs/ that are not
// upper-case pseudo-references, control characters, and components that
// NTFS or HFS+ would fold to "." or "..".
//
// Same as validReferenceName in go-git v5.19.3 storage/filesystem/dotgit
// (Apache-2.0, Copyright go-git authors). Review it when go-git is upgraded.
func validReferenceName(name plumbing.ReferenceName) error {
	if !name.IsSafe() {
		return fmt.Errorf("%w: %q is not under refs/ nor a valid pseudo-ref", dotgit.ErrReferenceNameEscape, string(name))
	}

	s := string(name)
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return fmt.Errorf("%w: %q", dotgit.ErrReferenceNameEscape, s)
		}
	}
	for _, part := range strings.FieldsFunc(s, isPathSep) {
		// IsNTFSDot/IsHFSDot with a "." needle match ".." and its disguises
		// but not a bare ".", so reject that component explicitly too.
		if part == "." || isHFSDot(part, ".") || isNTFSDot(part, ".", "") {
			return fmt.Errorf("%w: %q", dotgit.ErrReferenceNameEscape, s)
		}
	}
	return nil
}

// hfsIgnoredCodepoints contains Unicode code points that HFS+ ignores during
// path normalization.
//
// Same as hfsIgnoredCodepoints in go-git v5.19.3 internal/pathutil
// (Apache-2.0, Copyright go-git authors). Review it when go-git is upgraded.
var hfsIgnoredCodepoints = map[rune]struct{}{
	0x200c: {}, // ZERO WIDTH NON-JOINER
	0x200d: {}, // ZERO WIDTH JOINER
	0x200e: {}, // LEFT-TO-RIGHT MARK
	0x200f: {}, // RIGHT-TO-LEFT MARK
	0x202a: {}, // LEFT-TO-RIGHT EMBEDDING
	0x202b: {}, // RIGHT-TO-LEFT EMBEDDING
	0x202c: {}, // POP DIRECTIONAL FORMATTING
	0x202d: {}, // LEFT-TO-RIGHT OVERRIDE
	0x202e: {}, // RIGHT-TO-LEFT OVERRIDE
	0x206a: {}, // INHIBIT SYMMETRIC SWAPPING
	0x206b: {}, // ACTIVATE SYMMETRIC SWAPPING
	0x206c: {}, // INHIBIT ARABIC FORM SHAPING
	0x206d: {}, // ACTIVATE ARABIC FORM SHAPING
	0x206e: {}, // NATIONAL DIGIT SHAPES
	0x206f: {}, // NOMINAL DIGIT SHAPES
	0xfeff: {}, // ZERO WIDTH NO-BREAK SPACE
}

// isHFSDot reports whether part would be treated as ".<needle>" on an HFS+
// filesystem after stripping ignored code points and folding ASCII to lower
// case.
//
// Same as IsHFSDot in go-git v5.19.3 internal/pathutil (Apache-2.0,
// Copyright go-git authors). Review it when go-git is upgraded.
func isHFSDot(part, needle string) bool {
	runes := []rune(part)
	i := 0

	// skip ignored code points, then expect '.'
	for i < len(runes) {
		if _, ok := hfsIgnoredCodepoints[runes[i]]; !ok {
			break
		}
		i++
	}
	if i >= len(runes) || runes[i] != '.' {
		return false
	}
	i++

	// match needle case-insensitively, skipping ignored code points
	for _, expected := range needle {
		for i < len(runes) {
			if _, ok := hfsIgnoredCodepoints[runes[i]]; !ok {
				break
			}
			i++
		}
		if i >= len(runes) {
			return false
		}
		r := runes[i]
		if r > 127 {
			return false
		}
		if unicode.ToLower(r) != expected {
			return false
		}
		i++
	}

	// skip trailing ignored code points
	for i < len(runes) {
		if _, ok := hfsIgnoredCodepoints[runes[i]]; !ok {
			break
		}
		i++
	}

	// must be at end of component
	return i == len(runes)
}

// isNTFSDot detects path-component variants of a dotfile name that NTFS
// resolves to the canonical name (trailing spaces or periods, an Alternate
// Data Stream suffix, 8.3 short names).
//
// Same as IsNTFSDot in go-git v5.19.3 internal/pathutil (Apache-2.0,
// Copyright go-git authors). Review it when go-git is upgraded.
func isNTFSDot(name, dotgit, shortnamePrefix string) bool {
	onlySpacesAndPeriods := func(start int) bool {
		for i := start; i < len(name); i++ {
			c := name[i]
			if c == ':' {
				return true
			}
			if c != ' ' && c != '.' {
				return false
			}
		}
		return true
	}

	// Pattern 1: ".<dotgit>" prefix + trailing spaces / periods / ADS.
	if len(name) >= len(dotgit)+1 && name[0] == '.' &&
		strings.EqualFold(name[1:1+len(dotgit)], dotgit) {
		if onlySpacesAndPeriods(len(dotgit) + 1) {
			return true
		}
	}

	// Pattern 2: standard NTFS short name <dotgit[:6]>~[1-4].
	if len(dotgit) >= 6 && len(name) >= 8 &&
		strings.EqualFold(name[:6], dotgit[:6]) &&
		name[6] == '~' && name[7] >= '1' && name[7] <= '4' {
		if onlySpacesAndPeriods(8) {
			return true
		}
	}

	// Pattern 3: fall-back NTFS short name keyed by shortnamePrefix.
	if len(shortnamePrefix) < 6 || len(name) < 8 {
		return false
	}
	sawTilde := false
	i := 0
	for i < 8 {
		c := name[i]
		switch {
		case sawTilde:
			if c < '0' || c > '9' {
				return false
			}
		case c == '~':
			i++
			if i >= len(name) || name[i] < '1' || name[i] > '9' {
				return false
			}
			sawTilde = true
		case i >= 6:
			return false
		case c&0x80 != 0:
			return false
		default:
			if asciiToLower(c) != shortnamePrefix[i] {
				return false
			}
		}
		i++
	}
	return onlySpacesAndPeriods(8)
}

// Same as asciiToLower in go-git v5.19.3 internal/pathutil (Apache-2.0,
// Copyright go-git authors). Review it when go-git is upgraded.
func asciiToLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// CheckAndSetReference sets ref when the reference still is old. hh's own
// writers are serialized by the store lock. The check does not exclude an
// outside git process, and neither does go-git's flock.
func (s *cachedStorage) CheckAndSetReference(ref, old *plumbing.Reference) error {
	if old != nil {
		cur, err := s.Reference(old.Name())
		if err != nil {
			return err
		}
		if cur.Hash() != old.Hash() {
			return storage.ErrReferenceHasChanged
		}
	}
	return s.SetReference(ref)
}
