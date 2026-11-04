#!/bin/bash
# Task 3: take mount apart. Run at the repo root. $1 hoist, $2 scratchpad.
set -euo pipefail
HOIST=$1 SP=$2

# 1. the whole package to storage/mount
git mv internal/app/usecase/mount internal/app/storage/mount
mapfile -t f < <(grep -rlF '"github.com/kukv/hedgehug/internal/app/usecase/mount"' --include='*.go' .)
perl -pi -e 's#"github.com/kukv/hedgehug/internal/app/usecase/mount"#"github.com/kukv/hedgehug/internal/app/storage/mount"#' -- "${f[@]}"

D=internal/app/storage/mount
# 2. the store helpers of its tests to mounttest, which the tests that move
#    to usecase use too; openSet stays (usecase has its own)
mkdir -p $D/mounttest
cp "$SP/3-mounttest.go.txt" $D/mounttest/mounttest.go
perl -pi -e 's/\bnewStoreAt\(/mounttest.NewStoreAt(/g; s/\bputTasks\(/mounttest.PutTasks(/g; s/\btestAuthor\b/mounttest.Author/g' -- $D/*_test.go
for f in $D/*_test.go; do
  if grep -q 'mounttest\.' "$f"; then
    perl -0pi -e 's#^import \(\n#import (\n\t"github.com/kukv/hedgehug/internal/app/storage/mount/mounttest"\n#m' -- "$f"
  fi
done
cat > $D/helpers_test.go <<'EOF'
package mount

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/config"
)

// openSet opens the stores of es.
func openSet(t *testing.T, es ...config.StoreEntry) *Set {
	t.Helper()
	set := Open(&config.Config{Stores: es}, Options{})
	return set
}
EOF

# 3. what the moving code and tests used inside Set and Mounted
perl -pi -e 's/(set2?\.Stores\[\d\])\.ix != nil/$1.IndexOpened()/g; s/(set2?\.Stores\[\d\])\.ix == nil/!$1.IndexOpened()/g' -- $D/query_test.go
perl -0pi -e 's#(// reopen opens the store and its index again)#// IndexOpened reports whether Index has opened the index and keeps it.\nfunc (m *Mounted) IndexOpened() bool {\n\tm.mu.Lock()\n\tdefer m.mu.Unlock()\n\treturn m.ix != nil\n}\n\n$1#' -- $D/mount.go
perl -0pi -e 's/\t\t\tif s\.unopened != nil \{\n\t\t\t\ts\.unopened\(Skipped\{Name: m\.Name, Err: err\}\)\n\t\t\t\}\n/\t\t\ts.Unopened(Skipped{Name: m.Name, Err: err})\n/' -- $D/resolve.go
perl -0pi -e 's#(// Open opens every store of c\.Stores in order\.)#// Unopened tells Options.Unopened of a store whose index did not open.\nfunc (s *Set) Unopened(sk Skipped) {\n\tif s.unopened != nil {\n\t\ts.unopened(sk)\n\t}\n}\n\n$1#' -- $D/mount.go
golangci-lint fmt ./...
go vet ./$D/...

# 4. Resolve, Query and CrossRefs to usecase
"$HOIST" "$SP/3-mount.spec"
golangci-lint fmt ./...
git add -A internal
