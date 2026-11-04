#!/bin/bash
# The end of round 3: the areas import no other area, storage/mount does not
# use replicate, and no test function was lost. Run at the repo root; $1 is
# the commit to compare the test functions with (default: where the branch
# left origin/main).
set -euo pipefail
base=${1:-$(git merge-base HEAD origin/main)}
bad=0
for p in tasks comments browse views schemas workspaces stores replication notifications accounts maintenance; do
  if go list -deps ./internal/app/usecase/$p | grep "internal/app/usecase/" | grep -v "usecase/$p\$"; then
    echo "^ imported by usecase/$p" >&2
    bad=1
  fi
done
if go list -deps ./internal/app/usecase | grep "internal/app/usecase/"; then
  echo "^ imported by the base of usecase" >&2
  bad=1
fi
if go list -deps ./internal/app/storage/mount | grep "remote/replicate"; then
  echo "^ imported by storage/mount" >&2
  bad=1
fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
pat=(-e '^func Test' -e '^func Fuzz' -e '^func Bench')
git grep -h "${pat[@]}" "$base" -- '*_test.go' | sed 's/(.*//' | sort > "$tmp/before"
git grep -h "${pat[@]}" -- '*_test.go' | sed 's/(.*//' | sort > "$tmp/after"
if diff "$tmp/before" "$tmp/after" | grep '^[<>]' | grep -v '^> func TestMain$'; then
  echo "^ test functions that differ" >&2
  bad=1
fi
echo "TestMain added: $(diff "$tmp/before" "$tmp/after" | grep -c '^> func TestMain$' || true)"
exit $bad
