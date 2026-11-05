# フェーズ 6a: 計画の土台 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** CLI と API だけでスプリントを作り、タスクを入れ、開始して閉じられるようにする。リリースと作業時間の操作と、`sprint:`・`release:` のクエリも足す（spec の 6a の完了条件）。

**Architecture:** `model`・`store`・マージは揃っている。index にスプリントとリリースを読み込み、クエリとビューはファイルを読まずに index を引く。操作は `usecase/planning`・`usecase/worklogs` に置き、cli と server はそこだけを通す。

**Tech Stack:** Go、cobra、huma v2、go-git。

**Spec:** `docs/superpowers/specs/2026-11-05-phase6-planning-design.md` の「6a 計画の土台」。

**PR:** spec、この plan、実装を1つの PR に入れる。

## Global Constraints

- `make` は `MAKEFLAGS=-j10`。順番が要るものは Makefile の依存で書く
- API を足したら `make generate` で生成物を更新し、差分をコミットに入れる
- 1操作1コミット。`hh sprint close`・`rm` はタスクの変更と同じコミットに入れる
- `sprint`・`releases` の検査は `Tx.sprintReleaseViolations` に任せる（作り直さない）
- 既存のテストの期待値は変えない。変わるのは `TaskView` の JSON に足す項目のゴールデンだけ

## この回で決めたこと

| # | 項目 | 決定 | 理由 |
|---|---|---|---|
| 1 | スプリントとリリースの読み込み | `Index.readWorkspaces` と同じく、開くときと `Refresh` で tree から読む。キャッシュには入れない | ファイルの数が少ない。`Entry` の `Sprint`・`Releases` はキャッシュに入るので `cacheFormat` を 11 にする |
| 2 | 使えない名前 | 1〜64 文字、制御文字・カンマ・前後の空白は不可。`current`・`next`・`none`・`backlog` は不可（大文字小文字を問わない） | クエリと `--to` の語と区別する |
| 3 | `hh set` の解決 | `ApplyAssign` は変えず、`usecase.ApplyTaskAssign` が `sprint`・`releases` を引いて残りを `ApplyAssign` に渡す | inbox の呼び出しに影響しない。値は名前か ULID |
| 4 | 閉じたスプリントへの割り当て | 拒否しない | 親設計に制約がない。レポートは終了後の追加を範囲に数えない |
| 5 | 名前の引数の解決 | Store の全ワークスペースから名前で探し、`--workspace`、`hh use` の順に絞る。残りが複数ならエラー | spec のとおり |
| 6 | `hh sprint ls` の既定 | `--workspace`、`hh use` のワークスペース、Store 全体の順 | spec のとおり |

## ファイル

| 操作 | パス | 役割 |
|---|---|---|
| 作成 | `internal/app/storage/index/plan.go` | スプリントとリリースの読み込みと引き当て |
| 変更 | `internal/app/storage/index/entry.go`・`build.go`・`index.go`・`workspace.go` | `Entry` の項目、読み込みの呼び出し、`cacheFormat` |
| 変更 | `internal/app/storage/query/eval.go` | `sprint:`・`release:` |
| 作成 | `internal/app/usecase/plan_view.go` | `SprintRef`・`ApplyTaskAssign` |
| 変更 | `internal/app/usecase/view.go`・`workspace_view.go` | `TaskView` と `WorkspaceView` の項目 |
| 変更 | `internal/app/usecase/tasks/task.go`・`patch.go` | `ApplyTaskAssign` の呼び出し |
| 作成 | `internal/app/usecase/planning/sprint.go`・`release.go`・`name.go` | 操作 |
| 作成 | `internal/app/usecase/worklogs/worklog.go` | 作業時間 |
| 作成 | `internal/app/cli/sprint.go`・`release.go`・`worklog.go` | コマンド |
| 変更 | `internal/app/cli/add.go`・`set.go`・`app.go`・`show.go` | フラグ、登録、合計の表示 |
| 作成 | `internal/app/server/planning.go` | API |
| 変更 | `internal/app/server/tasks_write.go`・`server.go` | `Fields`、登録 |
| 変更 | `docs/superpowers/specs/2026-11-04-codebase-restructure-design.md` | 領域の数 |

## Task 1: index にスプリントとリリースを持たせる

- [ ] テストを書く: `internal/app/storage/index/plan_test.go`。`usecasetest` を使わず index のテストの手順でスプリントとリリースのファイルを commit し、`ix.Sprints(ws)` が `start`・ULID の順に返ること、`SprintNamed(ws, "s1")` が大文字小文字を問わないこと、`NextSprint(ws)` が `start` の最も早い `planned` を返すこと、`Refresh` で増減が反映されること、壊れたファイルを飛ばすことを確かめる
- [ ] 失敗を確かめる: `go test ./internal/app/storage/index/ -run Plan`
- [ ] `Entry` に `Sprint *model.ULID`・`Releases []model.ULID` を足し、`newEntry` で埋める。`cacheFormat` を 11 にする
- [ ] `plan.go` を書く

```go
// readPlans reads the sprints and the releases of tree. A file that does
// not decode is left to hh doctor.
func (ix *Index) readPlans(tree plumbing.Hash) error

func (ix *Index) Sprints(ws model.ULID) []*model.Sprint   // zero ws: all; by start, then ULID
func (ix *Index) Sprint(u model.ULID) (*model.Sprint, bool)
func (ix *Index) SprintNamed(ws model.ULID, name string) (*model.Sprint, bool)
func (ix *Index) NextSprint(ws model.ULID) (*model.Sprint, bool)
func (ix *Index) Releases(ws model.ULID) []*model.Release // zero ws: all; by date, then ULID
func (ix *Index) Release(u model.ULID) (*model.Release, bool)
func (ix *Index) ReleaseNamed(ws model.ULID, name string) (*model.Release, bool)
```

- [ ] `readWorkspaces` を呼ぶ場所で `readPlans` も呼ぶ。`start` のないスプリントは日付のあるものの後に置く
- [ ] 通す: `go test ./internal/app/storage/index/`
- [ ] コミット: `feat(index): スプリントとリリースを読み込む`

## Task 2: クエリ

- [ ] `internal/app/storage/query/plan_test.go` に表形式のテストを書く: `sprint:current`（active が2つ）、`sprint:next`、`sprint:none`、`sprint:"Sprint 12"`、`-sprint:none`、`sprint:s1,s2`、別のワークスペースの同名、`release:v1`、`release:none`、未知の名前は0件でエラーにしない
- [ ] 失敗を確かめる
- [ ] `eval.go` の `term` に足す

```go
case "sprint":
	return c.sprint(t)
case "release":
	return c.matchText(t, func(e *index.Entry) []string {
		var names []string
		for _, u := range e.Releases {
			if r, ok := c.ix.Release(u); ok {
				names = append(names, r.Name)
			}
		}
		return names
	}, nil)
```

`c.sprint` は `current`・`next` を先に取り除き、`matchText` の述語と OR でつなぐ。`current` は `e.Sprint` の状態が `active`、`next` は `e.Workspace` の `NextSprint` の ULID と一致するときに真。`Sprint` は引けるが ULID のファイルがないタスクは `none` にしない（`get` が空を返さないよう、ULID の文字列を返す）

- [ ] 通す: `go test ./internal/app/storage/query/`
- [ ] コミット: `feat(query): sprint: と release: を足す`

## Task 3: TaskView と hh set

- [ ] テストを書く: `internal/app/cli/plan_set_test.go`。`hh set <id> sprint=s1`、`sprint=`、`releases=v1`、`releases+=v2`、`releases-=v1`、別ワークスペースの名前は `sprint`・`releases` の違反、`hh add --sprint s1 --release v1`、`hh show --json` の `sprint`・`releases`、`hh set` のヘルプ
- [ ] `usecase/plan_view.go` を書く

```go
// PlanRef names a sprint or a release of a task.
type PlanRef struct {
	ULID  string `json:"ulid"`
	Name  string `json:"name,omitempty" doc:"Empty when the file is not in the store"`
	State string `json:"state,omitempty"`
}

// ApplyTaskAssign is ApplyAssign with the fields whose value is a name of
// the workspace of t: sprint and releases take a name or a ULID.
func ApplyTaskAssign(t *model.Task, a Assign, ix *index.Index, resolve func(string) (model.ULID, error)) error
```

- [ ] `TaskView` に `Sprint *PlanRef `json:"sprint,omitempty"`` と `Releases []PlanRef `json:"releases,omitempty"`` を足し、`TaskViewOf` で埋める
- [ ] `tasks/task.go` の2か所と `tasks/patch.go` を `ApplyTaskAssign` に替える。`hh add` は `--sprint`・`--release` を Assign にして渡す。`set` のヘルプの Fields に `sprint, releases` を足す
- [ ] `hh show` に Sprint と Releases の行を足す
- [ ] 通す: `go test ./internal/app/...`。ゴールデンは `TaskView` の項目が増える分だけ更新する
- [ ] コミット: `feat: タスクにスプリントとリリースを割り当てる`

## Task 4: スプリントの操作

- [ ] テストを書く: `internal/app/usecase/planning/sprint_test.go`（`usecasetest.Session`）。追加、名前の重複（大文字小文字違い）、予約語、`end < start`、開始（日付なしはエラー、`active` への再開始はエラー）、閉じる（`next`・`backlog`・名前、次がないときは何も変えない、done・canceled は動かない、1コミット）、`set` で `state` を拒否、`rm` が参照を外して1コミット
- [ ] 失敗を確かめる
- [ ] `planning/name.go` を書く

```go
func BadPlanName(what, name string) error // 決定 2
func pickWorkspace(ss *usecase.Session, ix *index.Index, arg string) (model.ULID, error) // zero: none given
func findSprint(ss *usecase.Session, ix *index.Index, name, ws string) (*model.Sprint, error) // 決定 5
func findRelease(ss *usecase.Session, ix *index.Index, name, ws string) (*model.Release, error)
```

- [ ] `planning/sprint.go` を書く

```go
type SprintView struct {
	ULID, Workspace, WorkspaceName, Name, Start, End, State, Goal string // json タグ付き
	Tasks, Done, Points int
}
type SprintWrite struct {
	Sprint SprintView `json:"sprint"`
	Moved  int        `json:"moved,omitempty"`
	To     string     `json:"to,omitempty"`
	Commit *string    `json:"commit"`
}

type SprintAddInput struct{ Name, Start, End, Goal, Workspace string }
func SprintAdd(ss *usecase.Session, in SprintAddInput) (SprintWrite, error)
func SprintList(ss *usecase.Session, workspace, state string) ([]SprintView, error)
func SprintSet(ss *usecase.Session, name, workspace string, assigns []usecase.Assign) (SprintWrite, error)
func SprintStart(ss *usecase.Session, name, workspace string) (SprintWrite, error)
func SprintOpenTasks(ss *usecase.Session, name, workspace string) (int, error)
func SprintClose(ss *usecase.Session, name, workspace, to string) (SprintWrite, error)
func SprintRemove(ss *usecase.Session, name, workspace string) (SprintWrite, error)
```

`SprintClose` は Tx の中でタスクを読み直し、`sprint` が対象で done・canceled カテゴリでないものを `tx.PutTask` する。`store.Op{Name: "sprint-close", Workspace: sp.Workspace}`、`tx.SetDetail("close <name>")`。

- [ ] `store.Tx` に `RemoveSprint`・`RemoveRelease` がなければ `RemoveFile` を使い、`tx.sprints` に入れないことを確かめる
- [ ] 通す
- [ ] コミット: `feat(planning): スプリントの操作を足す`

## Task 5: リリースの操作

- [ ] テストを書く: `planning/release_test.go`。追加、重複、`set` の `state` の全方向と不正な値、一覧の件数、`notes` が done のタスクを種類ごとに ULID 順でなく rank 順にまとめること、種類の見出しがスキーマの表示名であること、`rm`
- [ ] `planning/release.go` を書く

```go
func ReleaseAdd(ss *usecase.Session, name, date, workspace string) (ReleaseWrite, error)
func ReleaseList(ss *usecase.Session, workspace, state string) ([]ReleaseView, error)
func ReleaseSet(ss *usecase.Session, name, workspace string, assigns []usecase.Assign) (ReleaseWrite, error)
func ReleaseNotes(ss *usecase.Session, name, workspace string) (string, error)
func ReleaseRemove(ss *usecase.Session, name, workspace string) (ReleaseWrite, error)
```

- [ ] 通す
- [ ] コミット: `feat(planning): リリースの操作を足す`

## Task 6: hh sprint と hh release

- [ ] ゴールデンテストを書く: `internal/app/cli/sprint_test.go`・`release_test.go`。表、`--json`、`close` の `--to` なし（端末でなければエラー、端末なら聞く、未完了なしなら聞かない）
- [ ] `cli/sprint.go`・`cli/release.go` を `workspaceCmd` と同じ形で書く。`--workspace` は親の `PersistentFlags`
- [ ] `close` の端末の質問は `a.ask()` の `prompt` を使い、`next` / `backlog` を選ばせる
- [ ] `app.go` に登録する
- [ ] 通す
- [ ] コミット: `feat(cli): hh sprint と hh release を足す`

## Task 7: 作業時間

- [ ] テストを書く: `internal/app/cli/worklog_test.go`。`hh worklog <id> 1h30m メモ`、`--date`、不正な時間、`ls`、`rm`（他人の記録は拒否）、`hh show` の合計、`hh worklog foo` が `ls` などのサブコマンドと衝突しないこと
- [ ] `usecase/worklogs/worklog.go` を書く

```go
type WorklogView struct{ ULID, Task, Author, Date, Duration, Body string } // json タグ付き
type WorklogWrite struct { Worklog WorklogView `json:"worklog"`; Commit *string `json:"commit"` }
func Add(ss *usecase.Session, arg, duration, date, note string) (WorklogWrite, error)
func List(ss *usecase.Session, arg string) ([]WorklogView, error)
func Remove(ss *usecase.Session, arg, log string) (WorklogWrite, error)
```

Op は `worklog-add`・`worklog-rm`、`Task` を入れる。`Remove` は `comments.CheckCommentEditable` と同じ判定（`config.SameAccount`）

- [ ] `cli/worklog.go` を書く。親コマンドは `Args: minArgs(2)` の `RunE` で追加し、`ls`・`rm` をサブコマンドにする
- [ ] `ShowView` に `Worklog *WorklogTotal`（`total`・`count`）を足し、`estimate` と並べて出す
- [ ] 通す
- [ ] コミット: `feat: hh worklog を足す`

## Task 8: API

- [ ] テストを書く: `internal/app/server/planning_test.go`。spec の表の全エンドポイントの正常系、409（`close` の次のスプリントなし）、404、`PATCH /api/tasks/{id}` の `sprint`・`releases`
- [ ] `server/planning.go` を `registerAdmin` と同じ形で書き、`server.go` で登録する
- [ ] `tasks_write.go` の `Fields` に `Sprint *string`・`Releases *[]string` を足し、`assigns()` で Assign にする
- [ ] `WorkspaceView` に `Sprints`・`Releases` の件数を足す
- [ ] `make generate` を流し、生成物の差分を確かめる
- [ ] 通す
- [ ] コミット: `feat(server): 計画の API を足す`

## Task 9: 通しのテストと仕上げ

- [ ] `internal/app/cli/undo_plan_test.go`: `sprint-add`・`sprint-close`・`sprint-rm`・`release-set`・`worklog-add` を `hh undo` で戻す。`undoSkip`・`undoBlocked` に足すものがあれば足す
- [ ] `internal/app/cli/scenario` に、作成、割り当て、開始、完了、閉じて持ち越しまでの通しのテストを足す。git のない CI ジョブでも通ることを確かめる
- [ ] 同期のテスト: 2つのクローンで同じ名前のスプリントを作ると `hh doctor` が出すことを確かめる
- [ ] `docs/superpowers/specs/2026-11-04-codebase-restructure-design.md` の領域の数を直す
- [ ] `docs/superpowers/plans/2026-11-05-phase6a-handoff.md` を書く
- [ ] `make check`
- [ ] 手で確かめる: 検証用の Store で、スプリントの開始から終了までを実際のバイナリで流す
- [ ] コミット: `docs(plan): 6a の引き継ぎを書く`
