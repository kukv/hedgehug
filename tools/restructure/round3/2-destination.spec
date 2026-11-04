from internal/app/usecase/current
to internal/app/usecase usecase
file resolve.go destination.go
file resolve_test.go destination_test.go
file resolve_worktree_test.go destination_worktree_test.go
recv Ref
rename Ref.Find FindProject
rename Resolve ResolveDestination
dup Error
dup Kind
dup KindAmbiguous
dup KindConflict
dup KindInvalid
dup KindUsage
dup idOrULID
