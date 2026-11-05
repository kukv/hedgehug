# フェーズ 6b: テンプレートと一括更新 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 子タスクを含むテンプレートを CLI と API で展開でき、一致するタスクを1コミットで更新できるようにする（spec の 6b の完了条件）。

**Architecture:** 展開は `tasks.Add` の中で行い、親子を1つの `Update` で書く。テンプレートの一覧と編集は `usecase/templates` に置く。一括更新は `Op.CheckOnly` で全 Store を先に検査してから書く。領域どうしは import できないので、クエリから対象への変換は `browse` に置き、cli と server が `tasks.Bulk` に渡す。

**Tech Stack:** Go、cobra、huma v2、go-git。

**Spec:** `docs/superpowers/specs/2026-11-05-phase6-planning-design.md` の「6b テンプレートと一括更新」。

**PR:** この plan と実装を1つの PR に入れる。

## Global Constraints

- 検査の完了は `make check > log 2>&1; echo exit=$?` の exit 0 と `make: ***` の行がないことで判断する
- API を足したら `go generate ./...` と `pnpm gen` の生成物をコミットに入れる
- 既存のテストの期待値は変えない
- `kinds.template` は今の既定値が空なので、テンプレートのない Store の `hh add` は変わらない

## この回で決めたこと

| # | 項目 | 決定 | 理由 |
|---|---|---|---|
| 1 | 全件の検査 | `store.Op.CheckOnly` を足す。検査だけして書かず、違反を全タスク分 `ValidationErrors` で返す | 既存の `validate` を使い回せる。Store をまたいでも書く前に止められる |
| 2 | クエリの解決 | `browse.Targets` がクエリを `[]usecase.TaskTarget` にし、cli と server が `tasks.Bulk` に渡す | `tasks` から `browse` を import できない（決定 17） |
| 3 | 検査の結果 | `DryRun` は違反を結果に入れてエラーにしない。実行は `bulk.invalid` のエラーで違反を全部書く | 画面の事前確認で違反を出せる |
| 4 | API の本文 | `POST /api/tasks/bulk` は `ulids` か `query`、`PATCH` と同じ項目、`dry_run` | `Fields.assigns` を使い回す。spec の `assigns` を直す |
| 5 | 既定のテンプレートが無い | `hh add` は警告を出して普通に作る。`--template` で名前を指定したときはエラー | `hh doctor` が既に知らせる不整合で、タスクの追加を止めない |
| 6 | 展開の優先 | 引数と `-p` が最も強く、次にテンプレート、最後にスキーマの既定 | `hh add --kind` や `-p` の意味を変えない |
| 7 | 子のテンプレート | 子には種類の既定テンプレートを適用しない | 無限の展開を防ぐ |
| 8 | テンプレートの検査 | 保存のときに `kind` と `status` がスキーマにあることを、子まで確かめる | `blocked_by` などタスクを指す値は展開のときに検査される |
| 9 | 新しい名前で閉じた編集 | `hh template edit` で雛形を変えずに閉じたら作らない | `hh edit` の「変更なし」と同じ |

## ファイル

| 操作 | パス | 役割 |
|---|---|---|
| 変更 | `internal/app/storage/store/tx.go`・`validate.go` | `Op.CheckOnly`、`ValidationErrors` |
| 作成 | `internal/app/usecase/tasks/template.go` | テンプレートの選択と展開 |
| 変更 | `internal/app/usecase/tasks/task.go`・`internal/app/usecase/task_shared.go`・`write_view.go` | `AddInput`、`TaskResult` |
| 作成 | `internal/app/usecase/templates/template.go` | 一覧、読み書き、削除 |
| 作成 | `internal/app/usecase/tasks/bulk.go` | 一括更新 |
| 作成 | `internal/app/usecase/browse/targets.go` | クエリと ULID から対象へ |
| 変更 | `internal/app/domain/schema/scoped.go` | `ScopedChange` の `TemplateOnly`・`TemplateDefault` |
| 変更 | `internal/app/usecase/schemas/scoped.go`・`internal/app/cli/schema_scope.go`・`internal/app/server/scoped.go` | scoped の `templates` |
| 作成 | `internal/app/cli/template.go` | `hh template` |
| 変更 | `internal/app/cli/add.go`・`set.go`・`app.go` | フラグと登録 |
| 作成 | `internal/app/server/templates.go` | API |
| 変更 | `internal/app/server/tasks_write.go` | `template`、`POST /api/tasks/bulk` |

## Task 1: Op.CheckOnly

- [x] テストを書く（`store/tx_test.go`）: 違反のある2タスクで `CheckOnly` が `*ValidationErrors` に2件返す。ファイルも HEAD も変わらない。違反がなければ nil でコミットしない
- [x] `validate` で `CheckOnly` のときは最初の違反で返さず集める。`UpdateCommit` は `validate` の後で返す
- [x] `usecase.ViolationsOf` は `ValidationError` のままにする（`Bulk` が `errors.As` で読む）
- [x] コミット: `feat(store): 書かずに全件を検査する Op.CheckOnly を足す`

## Task 2: テンプレートの展開

- [x] テストを書く（`tasks/template_test.go`）: `--template` の項目と子の孫までの作成が1コミット、`Hedgehug-Task` が全タスク分、順序、引数の優先、`assignees: inherit`、種類の既定、`NoTemplate`、既定の不在は警告、指定の不在はエラー、`only` にないテンプレートの拒否、別ワークスペースの `sprint`・`releases` を外して `Detached` に出す、タイトルなしのエラー、子のタイトルなしのエラー
- [x] `tasks/template.go` を書く

```go
// templateFor is the template the add takes, or nil.
func templateFor(ss *usecase.Session, s *store.Store, ix *index.Index, in AddInput, t *model.Task) (*model.Template, error)
// applyItem gives t what it does not have from it.
func applyItem(t *model.Task, it *model.TemplateItem, parent *model.Task)
// expandChildren makes the tasks under parent, in order.
func expandChildren(ix *index.Index, parent *model.Task, items []model.TemplateItem, created *[]*model.Task) error
// dropForeign takes off the sprint and the releases of another workspace.
func dropForeign(ix *index.Index, t *model.Task) (usecase.Detached, bool)
```

- [x] `AddInput` に `Template string`・`NoTemplate bool` を足し、`Add` の中でワークスペースが決まった後に呼ぶ
- [x] `TaskResult` に `Children int`・`Detached []Detached` を足し、`Write` で `TaskWrite.Children`・`Detached` に出す
- [x] `hh add --template <name> [title]`・`--no-template`。テンプレートがあるときはタイトルを省ける。子の件数と外した名前を出す
- [x] `POST /api/tasks` の `title` を省略可にして `template`・`no_template` を足す
- [x] コミット: `feat: テンプレートを展開する`

## Task 3: テンプレートの編集

- [x] テストを書く（`usecase/templates/template_test.go`、`cli/template_test.go`）: 一覧、新規、不正な `kind`・`status`、壊れた YAML、`rm` の参照の拒否（`kinds`・`only`・`default`）、`hh template edit` の変更なしで作らない
- [x] `usecase/templates/template.go` を書く

```go
type TemplateView struct{ Name, Kind, Title string; Children int }
type TemplateFile struct{ Name, Content string; Exists bool }
type TemplateWrite struct{ Template TemplateView; Commit *string }
func List(ss *usecase.Session) ([]TemplateView, error)
func Read(ss *usecase.Session, name string) (TemplateFile, error) // 無ければ雛形で Exists: false
func Put(ss *usecase.Session, name, content string) (TemplateWrite, error)
func Remove(ss *usecase.Session, name string) (TemplateWrite, error)
```

- [x] `cli/template.go`: `ls`・`show`・`edit`・`rm`。`edit` は `a.edit` と同じく、検査に失敗すれば開き直す
- [x] `server/templates.go`: `GET /api/templates`、`GET`・`PUT`・`DELETE /api/templates/{name}`
- [x] コミット: `feat: テンプレートの編集を足す`

## Task 4: scoped の templates

- [x] テストを書く（`schema/scoped_test.go`、`cli/schema_scope_test.go`、`server`）: `--templates`、`--template-default`、空で外す、他の部分を保つ、ないテンプレートの拒否、展開が `only` と `default` に従う
- [x] `ScopedChange` に `TemplateOnly *[]string`・`TemplateDefault *map[string]string` を足し、`WithScoped` で反映する
- [x] `hh schema scope set` のフラグと、`PUT /api/schema/scoped/{project}` の `templates`・`template_defaults`
- [x] コミット: `feat(schema): プロジェクトで使うテンプレートを設定する`

## Task 5: 一括更新

- [x] テストを書く（`tasks/bulk_test.go`、`cli/set_query_test.go`、`server`）: 1コミット、`Hedgehug-Task` の全件、変更なしのタスクを含む、1件の違反で何も書かない、違反の全件表示、Store ごとのコミット、閲覧のみの Store で書く前に拒否、`--dry-run`、`--yes` なしの非端末のエラー、端末の確認、0件、`is:deleted` の拒否、`hh undo` で全件が戻る
- [x] `tasks/bulk.go` を書く

```go
type BulkInput struct {
	Targets []usecase.TaskTarget
	Assigns []usecase.Assign
	DryRun  bool
}
type BulkResult struct {
	Tasks      []BulkTask      `json:"tasks"`
	Violations []BulkViolation `json:"violations,omitempty"`
	Commits    []BulkCommit    `json:"commits,omitempty"`
}
func Bulk(ss *usecase.Session, in BulkInput) (BulkResult, error)
```

- [x] `browse/targets.go`: `Targets(ss, query)`・`TargetsOf(ss, ulids)`
- [x] `hh set --query '<q>' k=v... [--dry-run] [--yes]`
- [x] `POST /api/tasks/bulk`
- [x] コミット: `feat: hh set --query と一括更新の API を足す`

## Task 6: 仕上げ

- [x] `cli/scenario` に、テンプレートの展開から一括更新までの通しを足す
- [x] spec の `POST /api/tasks/bulk` の本文を直す。構成見直しの spec の領域の数を 17 にする
- [x] `docs/superpowers/plans/2026-11-05-phase6b-handoff.md` を書く
- [x] 実際のバイナリで手の確認を流す
- [x] `make check`
- [x] コミット: `docs(plan): 6b の引き継ぎを書く`
