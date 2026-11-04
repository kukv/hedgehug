#!/bin/bash
# Task 1 dry run: leftover detection -> storage/store. Run at the repo root.
set -euo pipefail
R=internal/app/remote/replicate/rebuild.go
S=internal/app/storage/store/leftover.go
start=$(grep -n '^// Leftover is what a rebuild left' $R | cut -d: -f1)
end=$(grep -n '^func (e \*LeftoverError) Error' $R | cut -d: -f1)
{
cat <<'EOF'
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// What a rebuild after a history rewrite (replicate.Rebuild) leaves beside
// the store at dir while it swaps: the rebuilt store, and the old one moved
// away.
const (
	RewriteNewSuffix    = ".rewrite-new"      // <dir>.rewrite-new
	BeforeRewritePrefix = ".before-rewrite-" // <dir>.before-rewrite-<when>
)

EOF
sed -n "${start},${end}p" $R | sed 's/base+newSuffix/base+RewriteNewSuffix/; s/base+beforePrefix/base+BeforeRewritePrefix/'
} > $S
sed -i "${start},$((end+1))d" $R
perl -0pi -e 's/\tnewSuffix    = ".rewrite-new"\n/\tnewSuffix    = store.RewriteNewSuffix\n/; s/\tbeforePrefix = ".before-rewrite-"\n/\tbeforePrefix = store.BeforeRewritePrefix\n/' $R

# callers: replicate.X -> store.X ; unqualified X in package replicate -> store.X
names='FindLeftover|ReadParent|LeftoverIn|LeftoverMessage|LeftoverError|Leftover'
grep -rlE "replicate\.($names)\b" --include='*.go' internal | xargs -r perl -pi -e "s/\breplicate\.($names)\b/store.\$1/g"
perl -pi -e "s/(?<![.\\w])($names)\\b(?=[({ ,)\\]]|\$)/store.\$1/g unless /^\\s*\\/\\// " internal/app/remote/replicate/*.go
