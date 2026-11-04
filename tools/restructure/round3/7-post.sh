#!/bin/bash
# After the findings went down: the package comment of maintenance was on
# finding.go; it goes to doctor.go, saying what the package is now.
set -euo pipefail
U=internal/app/usecase
perl -0pi -e 's#\A// Package maintenance finds what is wrong with a store: a schema that does not\n// fit its tasks, files that do not decode, changes that are not committed\.\n##' -- $U/finding.go
{ printf '%s\n' \
  "// Package maintenance is hh doctor, hh purge and the rebuild of a store after" \
  "// a history rewrite. Doctor finds what is wrong with a store: a schema that" \
  "// does not fit its tasks, files that do not decode, changes that are not" \
  "// committed."
  cat $U/maintenance/doctor.go; } > $U/maintenance/doctor.go.new
mv $U/maintenance/doctor.go.new $U/maintenance/doctor.go
golangci-lint fmt ./...
git add -A internal
