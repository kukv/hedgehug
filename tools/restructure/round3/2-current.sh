#!/bin/bash
# Task 2: take current apart. Run at the repo root. $1 hoist, $2 scratchpad.
set -euo pipefail
HOIST=$1 SP=$2

# 1. remote URLs to lib/giturl
"$HOIST" "$SP/2-giturl.spec"
golangci-lint fmt ./...
perl -0pi -e 's#\A// Package current decides where a new task goes when the command does not\n// say: it reads remote URLs, and keeps context\.json \(the project hh use set,\n// and the project last used for each repository\)\.\n#// Package giturl reads the remote URLs of git repositories: their host,\n// owner and repository, and the repo ID Hedgehug knows a repository by.\n#' -- internal/lib/giturl/giturl.go
perl -0pi -e 's#\Apackage current\n#// Package current keeps context.json: the project hh use set, and the\n// project last used for each repository.\npackage current\n#' -- internal/app/usecase/current/state.go

# 2. the decision of the destination to usecase
"$HOIST" "$SP/2-destination.spec"
golangci-lint fmt ./...
bash "$SP/2-destination-post.sh"

git add -A internal
# 3. what is left (context.json) to storage/current
git mv internal/app/usecase/current internal/app/storage/current
mapfile -t f < <(grep -rlF '"github.com/kukv/hedgehug/internal/app/usecase/current"' --include='*.go' .)
perl -pi -e 's#"github.com/kukv/hedgehug/internal/app/usecase/current"#"github.com/kukv/hedgehug/internal/app/storage/current"#' -- "${f[@]}"
golangci-lint fmt ./...
