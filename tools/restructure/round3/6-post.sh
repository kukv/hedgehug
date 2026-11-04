#!/bin/bash
# After the split: literals of another package's struct get their keys (vet),
# and every new package's tests get the TestMain of the others.
set -euo pipefail
U=internal/app/usecase
perl -pi -e 's/usecase\.CommentView\{c\.ULID\.String\(\), c\.Author, c\.Created, c\.Body\}/usecase.CommentView{ULID: c.ULID.String(), Author: c.Author, Created: c.Created, Body: c.Body}/' -- $U/browse/read.go
perl -0pi -e 's/usecasetest\.CfgStore\{\s*("[^"]*"|\w+),\s*("[^"]*"|\w+)\s*\}/usecasetest.CfgStore{Name: $1, Account: $2}/g' -- $U/accounts/auth_add_test.go $U/accounts/auth_device_test.go
for area in tasks comments browse views schemas workspaces stores replication notifications accounts; do
  cat > $U/$area/testmain_test.go <<EOF
package ${area}_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/lib/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }
EOF
done
golangci-lint fmt ./...
git add -A internal
set -euo pipefail
U=internal/app/usecase
doc() { # file comment...
  local f=$1; shift
  { printf '%s\n' "$@"; cat "$f"; } > "$f.new" && mv "$f.new" "$f"
}
doc $U/tasks/task.go "// Package tasks is the operations on tasks: add, set, edit, move, remove," "// restore and undo."
doc $U/comments/comment.go "// Package comments is the operations on the comments of a task."
doc $U/browse/read.go "// Package browse reads the stores: hh ls, tree, show, log and people."
doc $U/views/views.go "// Package views is the operations on the saved views of a store."
doc $U/schemas/schemaview.go "// Package schemas checks, migrates and shows the schema of a store, and the" "// scoped schemas of projects."
doc $U/workspaces/workspace.go "// Package workspaces is the operations on the workspaces of a store."
doc $U/stores/storeops.go "// Package stores sets the stores up: init, add, remove, discover, the move" "// of tasks to another store, and hh use."
doc $U/replication/sync.go "// Package replication syncs the stores with their remotes."
doc $U/notifications/inbox.go "// Package notifications is the inbox: what others did to the tasks one" "// watches, and how to read and revert it."
doc $U/accounts/auth.go "// Package accounts is the GitHub accounts and their tokens."
doc $U/usecasetest/session.go "// Package usecasetest makes sessions over test stores, for the tests of" "// usecase and its areas."

perl -0pi -e 's#(      - path: _test\\\.go\n        linters:\n          - gosec\n)#      - path: (_test\\.go|internal/app/usecase/usecasetest/)\n        linters:\n          - gosec\n#; s#\|internal/app/storage/history/historytest/\)#|internal/app/storage/history/historytest/|internal/app/usecase/usecasetest/)#' -- .golangci.yaml
git add -A internal .golangci.yaml
