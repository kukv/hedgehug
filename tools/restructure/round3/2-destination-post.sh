#!/bin/bash
# Task 2, after resolve.go moved into usecase as destination.go: its Kind,
# Error and newError were the same as usecase's own, and idOrULID the same
# as mount_ops.go's. They go; the refusals are usecase *Error from the start.
set -euo pipefail
F=internal/app/usecase/destination.go
perl -0pi -e 's#// Kind is what an Error means; usecase turns it into its own Kind\.\ntype Kind int\n\nconst \(\n\tKindUsage Kind = iota \+ 1\n\tKindInvalid\n\tKindAmbiguous\n\tKindConflict\n\)\n\n##' -- $F
perl -0pi -e 's#// Error is a refusal of Resolve\. Code and Args are those of the usecase\.Error it\n// becomes\.\ntype Error struct \{\n.*?\n\}\n\nfunc \(e \*Error\) Error\(\) string \{ return e\.Err\.Error\(\) \}\nfunc \(e \*Error\) Unwrap\(\) error \{ return e\.Err \}\n\nfunc newError\(.*?\n\}\n\n##s' -- $F
perl -0pi -e 's#func idOrULID\(e \*index\.Entry\) string \{\n\tif e\.ID != "" \{\n\t\treturn e\.ID\n\t\}\n\treturn e\.ULID\.String\(\)\n\}\n\n##' -- $F
perl -pi -e 's/\bnewError\(/newErr(/g' -- $F
# contextError no longer turns a refusal of Resolve into an *Error
perl -0pi -e 's#\tvar ce \*Error\n##; s#\tcase errors\.As\(err, &ce\):\n\t\tvar k Kind\n.*?\t\te\.Args = ce\.Args\n\t\treturn e\n##s' -- internal/app/usecase/addhere.go
# with one case left, the switch is an if
perl -0pi -e 's#// contextError gives the error of current\.Resolve its Kind and Code\.\n#// contextError spells out what ResolveDestination could not find (LookupError).\n#; s#\tvar le \*LookupError\n\tswitch \{\n\tcase errors\.As\(err, &le\):\n(.*?)\t\treturn newErr\(KindNotFound, "workspace\.not_found", le\.Err, "input", le\.Input\)\n\t\}\n\treturn err\n#"\tvar le *LookupError\n\tif !errors.As(err, &le) {\n\t\treturn err\n\t}\n" . ($1 =~ s/^\t//gmr) . "\treturn newErr(KindNotFound, \"workspace.not_found\", le.Err, \"input\", le.Input)\n"#se' -- internal/app/usecase/addhere.go
