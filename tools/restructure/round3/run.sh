#!/bin/bash
# Round 3 of the restructure (usecase), one step at a time. Run at the repo
# root: bash tools/restructure/round3/run.sh <step>, steps 1 to 7 in order,
# each on the commit of the step before. Every step ends with go build and
# go vet passing, and its changes staged.
set -euo pipefail
R=tools/restructure/round3
BIN=$(mktemp -d)
trap 'rm -rf "$BIN"' EXIT
go -C tools/restructure build -o "$BIN/" ./hoist ./carve
H=$BIN/hoist C=$BIN/carve
fmt() { golangci-lint fmt ./...; }
check() { go build ./... && go vet ./internal/... ./tools/... && git add -A; }

case "${1:?step 1-7}" in
1) # what a rebuild left beside a store: storage/store
  bash $R/1-leftover.sh; fmt ;;
2) # current: lib/giturl, usecase (destination), storage/current
  bash $R/2-current.sh "$H" "$R" ;;
3) # mount: storage/mount, usecase (ResolveIn, Query, CrossRefs)
  bash $R/3-mount.sh "$H" "$R" ;;
4) # what two areas use goes down to the base, and is exported
  "$C" $R/4-base.carve >/dev/null; fmt
  "$H" $R/4-export.spec; fmt ;;
5) # the test helpers two areas use into helpers_test.go; tests that
   # reach into a struct get a method
  "$C" $R/5-helpers.carve >/dev/null; fmt
  bash $R/5-pre.sh ;;
6) # the areas, usecasetest, and every test external
  "$H" $R/6-split.spec; fmt
  bash $R/6-post.sh; fmt ;;
7) # the findings and the schema checks of the old doctor: the base
  "$C" $R/7-findings.carve >/dev/null; fmt
  "$H" $R/7-findings.spec; fmt
  bash $R/7-post.sh ;;
*) echo "no step $1" >&2; exit 2 ;;
esac
check
echo "step $1 done"
