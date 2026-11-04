from internal/app/usecase
recv Session
tests external
rename Session.resetWriter ForgetWriter
rename testSession.addTask AddTask
rename defaultAccountYAML DefaultAccountYAML
rename testAccount TestAccount
rename storeAccountRE StoreAccountRE
rename testSession.appendDefaultAccount AppendDefaultAccount
rename testSession.dropDefaultAccount DropDefaultAccount
rename testSession.nameAccountOfStores NameAccountOfStores
rename firstAccountName FirstAccountName
rename testSession.dropAccounts DropAccounts
rename testSession.hasDefaultAccount HasDefaultAccount
rename testSession.appendConfig AppendConfig
rename testSession Session
rename testSession.t T
rename testSession.env Env
rename testSession.warns Warns
rename newTestSession New
rename addBareRemote AddBareRemote
rename otherSession Other
rename cloneSession Clone
rename testSession.writeAndCommit WriteAndCommit
rename newTwoStoreSession NewTwoStore
rename newTwoStoreSessionWith NewTwoStoreWith
rename testSession.setConfig SetConfig
rename testSession.storeAccounts StoreAccounts
rename testSession.configBytes ConfigBytes
rename testSession.addItem AddItem
rename overwriteME1 OverwriteME1
rename testSession.viewFile ViewFile
rename testSession.headSubject HeadSubject
rename testSession.headTrailer HeadTrailer
rename authSession AuthSession
rename testSession.lockFile LockFile
rename testSession.accounts Accounts
rename heldElsewhere HeldElsewhere
rename testSession.commitRawFile CommitRawFile
rename testSession.editRawFile EditRawFile
rename testSession.head Head
rename testSession.doctorOne DoctorOne
rename testSession.putSprintRelease PutSprintRelease
rename testSession.workspaceULID WorkspaceULID
rename testSession.order Order
rename testSession.addedTask AddedTask
rename testSession.asAccount AsAccount
rename testSession.setMembers SetMembers
rename testSession.addProject AddProject
rename testSession.storeSchema StoreSchema
rename testSession.addDeleted AddDeleted
rename lastOp LastOp
rename testSession.addWorkspace AddWorkspace
rename testSession.deleteTask DeleteTask
rename testSession.useAccount UseAccount
rename wantErr WantErr
rename asError AsError
rename bobAccount BobAccount
rename testSession.asWriter AsWriter
rename moveFx MoveFx
rename moveFx.ts TS
rename moveFx.p P
rename moveFx.c C
rename moveFx.g G
rename moveFx.o O
rename moveFx.first First
rename moveFx.comment Comment
rename moveFx.worklog Worklog
rename moveFx.commentRel CommentRel
rename moveFx.worklogRel WorklogRel
rename newMoveFx NewMoveFx
rename moveFx.in In
rename moveFx.head Head
rename moveFx.log Log
rename moveFx.setSchema SetSchema
rename mustCode MustCode
rename aliceAccount AliceAccount
rename movedFixture MovedFixture
rename mustIndex MustIndex
rename headOf HeadOf
rename ulidOf UlidOf
rename wantCode WantCode
rename makeStoreReadOnly MakeStoreReadOnly
rename openSet OpenSet
rename testSchema TestSchema
rename mustAssigns MustAssigns
rename mustULID MustULID
rename useGoneWarning UseGoneWarning
rename addProjectIn AddProjectIn
rename loadContext LoadContext
rename fakeSession FakeSession
rename configText ConfigText
rename notExist NotExist
rename storeDirOf StoreDirOf
rename hookConfigWrite HookConfigWrite
rename logOf LogOf
rename cfgStore CfgStore
rename cfgStore.name Name
rename cfgStore.account Account
rename kr Keyring
rename kr.mu Mu
rename kr.m M
rename kr.onSet OnSet
rename kr.onDelete OnDelete
rename kr.failSet FailSet
rename kr.failDelete FailDelete
rename newKR NewKeyring
rename kr.has Has
rename movedAuthor MovedAuthor
rename makeReadOnly MakeReadOnly
rename moveFx.brokenForward BrokenForward
rename moveFx.oldForward OldForward
rename moveFx.addSprint AddSprint
rename cloneSchemaOf CloneSchemaOf
alias revertField RevertField
alias Session.githubRepoOf GithubRepoOf
alias inboxTaskRef InboxTaskRef
alias setLater SetLater
alias errNotRevertible ErrNotRevertible
alias duplicateContent DuplicateContent
alias detailWorkspace DetailWorkspace
alias withRebuildFailure WithRebuildFailure
alias undoScanner UndoScanner
alias undoScanner.next Next
alias propText PropText
alias projectOf ProjectOf
alias clearDir ClearDir
alias scopedGuard ScopedGuard
alias sameULID SameULID
alias sameTime SameTime
alias nameCandidates NameCandidates
alias Session.warnPrefixDuplicates WarnPrefixDuplicates
alias Session.crossStoreFindings CrossStoreFindings
alias logViewOf LogViewOf
alias inboxTarget InboxTarget
alias dupPlace DupPlace
rename duplicateFindings duplicateStoreFindings
rename taskName TaskName
rename Session.AuthStatus AuthStatuses
to internal/app/usecase/usecasetest usecasetest
file helpers_test.go session.go
to internal/app/usecase/tasks tasks
file task.go 
file addhere.go 
file edit.go 
file patch.go 
file move.go 
file delete.go 
file showat.go 
file mention.go 
file undo.go 
file add_sibling_test.go 
file addhere_fix_test.go 
file addhere_test.go 
file delete_test.go 
file edit_test.go 
file mention_test.go 
file move_sprint_test.go 
file move_test.go 
file patch_test.go 
file restore_forward_test.go 
file showat_test.go 
file task_test.go 
file undo_test.go 
to internal/app/usecase/comments comments
file comment.go 
file comment_prepare_test.go 
file comment_test.go 
to internal/app/usecase/browse browse
file read.go 
file people.go 
file query.go 
file list_stores_test.go 
file log_trailers_test.go 
file people_test.go 
file query_test.go 
file read_test.go 
to internal/app/usecase/views views
file views.go 
file views_test.go 
to internal/app/usecase/schemas schemas
file schemacheck.go 
file schemaview.go 
file scoped.go 
file schemaview_test.go 
file scoped_test.go 
to internal/app/usecase/workspaces workspaces
file workspace.go 
file workspace_test.go 
to internal/app/usecase/stores stores
file storeops.go 
file discover.go 
file movestore.go 
file use.go 
file discover_export_test.go export_test.go
file config_recheck_test.go 
file discover_test.go 
file movestore_test.go 
file movestore_unfinished_test.go 
file movestore_view_test.go 
file storeinit_remote_test.go 
file storeops_test.go 
file use_test.go 
to internal/app/usecase/replication replication
file sync.go 
file status_broken_access_test.go 
file sync_test.go 
to internal/app/usecase/notifications notifications
file inbox.go 
file inbox_detached_test.go 
file inbox_moved_test.go 
file inbox_test.go 
to internal/app/usecase/accounts accounts
file auth.go 
file auth_add_test.go 
file auth_test.go 
to internal/app/usecase/maintenance maintenance
file maintain.go 
file purge.go 
file rebuild.go 
file maintain_test.go 
file purge_broken_test.go 
file purge_test.go 
file rebuild_failure_test.go 
file rebuild_test.go 
file crossrefs_test.go 
to internal/app/usecase/stores stores
file movestore_view.go 
file movestore_gui_test.go 
file storeset_test.go 
to internal/app/usecase/accounts accounts
file auth_device_test.go 
external assign_test.go
external conn_insecure2_test.go
external conn_insecure_test.go
external conn_test.go
external destination_test.go
external destination_worktree_test.go
external duplicates_test.go
external errors_required_test.go
external errors_test.go
external final_fixes_test.go
external final_review_test.go
external github_errors_test.go
external mount_ops_test.go
external mount_test.go
external moved_test.go
external progress_test.go
external project_test.go
external qualified_test.go
external readonly_test.go
external refs_test.go
external resolve_moved_test.go
external resolve_test.go
external scope_test.go
external session_close_test.go
external session_more_test.go
external session_test.go
external testmain_test.go
external token_kind_test.go
external view_test.go
