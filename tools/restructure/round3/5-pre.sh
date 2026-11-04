#!/bin/bash
# Before the split: the four places where a test reaches into a struct of
# the base get a method instead, since the tests become external.
set -euo pipefail
U=internal/app/usecase

# a test of stores drops the mounted stores, as a new run starts: production
perl -0pi -e 's#(// mountCache is the cache of ss, made on first use\.\n)#// ForgetMounts lets go of the stores the session mounted, as a new run of\n// the command starts without them.\nfunc (ss *Session) ForgetMounts() { ss.mounts = nil }\n\n$1#' -- $U/session.go
perl -pi -e 's#\tts\.mounts = nil // the stores are opened again, as a new run does#\tts.ForgetMounts() // the stores are opened again, as a new run does#' -- $U/use_test.go

# the base's own tests: export_test.go
cat > $U/export_test.go <<'EOF'
package usecase

// SharesTokensWith reports whether ss and o use one token cache.
func (ss *Session) SharesTokensWith(o *Session) bool {
	return ss.tokens != nil && ss.tokens == o.tokens
}

// WriterCached reports whether ss has read the writer and keeps it.
func (ss *Session) WriterCached() bool { return ss.writerDone }
EOF
perl -pi -e 's#sub\.tokens == nil \|\| sub\.tokens != src\.tokens#!sub.SharesTokensWith(src)#; s#if sub := ts\.ForStore\("me"\); sub\.writerDone \{#if sub := ts.ForStore("me"); sub.WriterCached() {#' -- $U/final_fixes_test.go

# the test of discover compares candidates without their key; it goes to
# stores with discover.go
cat > $U/discover_export_test.go <<'EOF'
package usecase

// ClearCandidateKey empties the key of c, which a test does not compare.
func ClearCandidateKey(c *Candidate) { c.key = "" }
EOF
perl -pi -e 's#^\t\tgot\.key = ""$#\t\tClearCandidateKey(&got)#' -- $U/discover_test.go
golangci-lint fmt ./...
go vet ./$U
