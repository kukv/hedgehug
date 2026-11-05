# フェーズ 5c: Issue 同期 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Issue のリンクの `mirror`・`push`・`link` と、`hh issue`・`hh gh import` を作り、3 つのモードと一括取り込みが fakegh で動き、手編集の上書きが inbox に出るようにする（spec の 5c の完了条件）。

**Architecture:** 3 者の比較は I/O のない `domain/automation/issue.go` に置く。`mirror` は取り込みのトランザクションの中で評価し、ストアのロックを持ったままタスクごとの `gh-mirror` コミットにする。ルールは `mirror` の後のステータスから評価する。`push` はロックを放してから GitHub へ書き、成功した分を `gh-push` コミットで `synced` に残す。

**Tech Stack:** Go、GitHub GraphQL・REST API、fakegh、cobra。

**Spec:** `docs/superpowers/specs/2026-11-05-phase5-github-design.md` の「データの形」「5c Issue 同期」「テスト」。

**PR:** この plan・spec の決定・引き継ぎは実装の PR に入れる。

## Global Constraints

- コミットの署名は 1Password。失敗したら index に積んで利用者に知らせる
- `make` は `MAKEFLAGS=-j10`
- cli と server は usecase だけを通す。usecase の領域どうしは import しない。共有するものは usecase 直下に置く
- OS に触る呼び出しは `lib/osfs` を通す。外部の `git` コマンドは使わない
- 生成物は `go generate ./internal/app/server` と `pnpm gen` で作る
- 既存のテストの期待値は、この plan が明示したものの他は変えない
- 5d の画面と API は作らない。web は op の表示名と `SYNC_OPS` だけを足す
- 各 Task の終わりに `go vet ./...` と、変えたパッケージの `go test` を通してコミットする

## この回で決めたこと

| # | 項目 | 決定 | 理由 |
|---|---|---|---|
| 1 | `mirror` のコミット | タスクごとの `gh-mirror`（author は自動化）に、フィールド、ステータス、`synced`、trailer をまとめる。ロックは取り込みから持ち続ける | `gh-fetch` の中で `PutTask` が検証に落ちると取り込み全体が失敗する。`synced` とフィールドが同じコミットなら、途中で落ちても次の取得でやり直せる |
| 2 | `push` の順 | ルールのコミットの後、ロックを放してから `PATCH` し、成功した分を `gh-push` にする | 通信の間はロックを持たない（5b 決定 2）。ルールが動かした後のステータスを送れる。`synced` は書けたものだけ進むので、失敗は次の同期で再試行される |
| 3 | `gh-mirror`・`gh-push` の扱い | `undoSkip`・`syncOps`・`SYNC_OPS` に入れる | 取り消しても次の同期で戻る。ルールの `set` は 5b のまま別のコミットで、1 件ずつ取り消せる |
| 4 | `synced` の形 | `{title, body, status, assignees, labels, milestone}`。`status` は `open`・`completed`・`not_planned`。リストは小文字の login とラベル名を整列して持つ。フィールドが無ければ基準なし | spec の例と同じ。比べる値は GitHub 側の表現に揃える |
| 5 | 基準がないフィールド | モードの向きへ黙って揃え、inbox に出さない | 手編集かどうか分からない。リンクした直後に通知を出さない |
| 6 | 手編集の通知 | trailer `Hedgehug-Issue-Overwritten: <ULID> <field> <ref> <side> <JSON の値>`、inbox の種類 `issue_overwritten`、宛先は担当者とウォッチャー。`side` は上書きされた側（`task`・`github`） | `Hedgehug-Overwritten` は負けた author 宛てだが、`mirror` と `push` には author がいない |
| 7 | `mirror` の拒否 | GitHub が基準から変わった回だけ `Hedgehug-Automation-Rejected`（rule は `mirror`）を書き、`synced` は GitHub の値へ進める。タスクが基準と違うまま拒否され続ける間は何も書かない | 取得のたびに通知とコミットが増えない。ワークフローが許すようになれば次の反映で揃う |
| 8 | `mirror` のステータスの trailer | `Hedgehug-Automation: mirror issue.mirror`。inbox は `automation` で出る | spec の「自動化と同じ trailer と通知」 |
| 9 | not_planned と `issue.closed` | ステータスを `mirror` するリンクで not_planned に閉じた Issue は `issue.closed` を発火しない。ほかは spec のとおり `mirror` の後に当てる | `issue-closed` の `set: {category: done}` が `canceled` を `done` に変え、次の反映で手編集として戻る。通知が嘘になる |
| 10 | 同じタスクの複数の `mirror` リンク | フィールドごとに `links` の先頭のリンクが持つ。後のリンクはそのフィールドを見ない | 2 つの Issue の値でタスクが毎回振れない |
| 11 | ルールの起点 | `automation.Input.Status` を足す。空なら `Task.Status`。`mirror` の後のステータスを渡し、`required` の判定には `mirror` の値を入れたタスクの写しを渡す | spec の「`mirror` の反映の後に当てる」 |
| 12 | 除いた値の `synced` | 存在しないラベル、割り当てできない人、ない milestone を除いた値を `synced` に書く。別の host の担当者も除く | そのまま基準に入れると、GitHub が基準と違うと見えて毎回上書きの通知が出る |
| 13 | 名前の解決の呼び出し | タスクの値が基準と GitHub のどちらとも違うときだけ読み、repo ごとに 1 回の同期の中でキャッシュする | 変更のない同期で API を使わない |
| 14 | 大文字小文字と本文 | login とラベルは小文字にして比べ、ラベルは GitHub の表記で送る。本文は印を除き、CRLF を LF にし、末尾の空白を落として比べる | GitHub の Web の編集は CRLF で返す |
| 15 | `gh.issues` の更新 | `gh-push` は書いた `state`・`state_reason` も `gh` に写す | 取得まで古い `closed` が残ると、次に閉じたときの `issue.closed` が発火しない |
| 16 | モードの変更 | `hh link --sync` で変えたら `synced` を消す | 古い基準が手編集に見える |
| 17 | `hh issue` の二重実行 | そのタスクにその repo の使用中の Issue リンクがあれば `link.exists`。作成の後のコミットが失敗したら Issue の番号をエラーに出す | 作成の前に落ちる場面と区別する |
| 18 | `hh issue` のフィールド | `issue_fields.push` のものを渡す。本文が対象外なら印だけ。タスクが閉じる側なら作成の後に閉じる。`synced` と `gh` を同じコミットに書く | 次の同期で差分が出ない |
| 19 | `hh gh import` のコミット | タスクごとに op `add` の 1 コミット。途中で落ちても打ち直せる。`repos` に repo を入れる。`off` のリンクも「リンク済み」として飛ばす。終わったらその repo を取り込む（`hh pr` と同じ順） | `hh undo` が 1 件ずつ効く。`hh unlink` した Issue が戻らない |
| 20 | `hh gh import --workspace` | spec に足す。`--parent` がなく Workspace が複数あるときに要る | `hh add` と同じ |
| 21 | 開いた Issue の一覧 | GraphQL の `issues(first: 100, after: $after, states: [OPEN], labels: [$label])` | spec の「読み取りは GraphQL」。REST は PR も返す |
| 22 | `hh issue`・`hh gh import` の repo 指定 | `--repo org/repo` と引数は `[<host>/]owner/repo`。host が無ければ書き手の host | `hh link` と同じ |
| 23 | `issue_fields` の `milestone` | `props.milestone` がないと対象から外す。`hh doctor` は今の検査のまま | spec の「`props` で対応先のキーを決めると有効」 |
| 24 | `link.fields` の知らない名前 | 飛ばす | 手で書いた値で同期を止めない（5b 決定 16 と同じ） |

## File Structure

| ファイル | 役割 |
|---|---|
| `internal/app/domain/model/github.go` | `IssueValues`、`IssueState.Values`・`SetValues`、印の `MarkedBody`・`StripMark` |
| `internal/app/domain/automation/issue.go`（新規） | 対象のフィールド、状態の対応、3 者の比較、trailer |
| `internal/app/domain/automation/eval.go` | `Input.Status` |
| `internal/lib/github/graphql.go`・`issues.go`（新規） | Issue の読み取りのフィールド、`OpenIssues`、作成・更新、ラベル・担当者・マイルストーン |
| `internal/lib/fakegh/*`、`cmd/fakegh/main.go` | Issue のフィールド、REST、`OpenIssues` のクエリ、操作コマンド |
| `internal/app/usecase/github_issues.go`（新規） | 値の読み替え、`mirror` の計画とコミット |
| `internal/app/usecase/github_push.go`（新規） | 名前の解決、`push` |
| `internal/app/usecase/github_fetch.go`・`github_rules.go`・`github_apply.go` | 上の呼び出し、イベントの絞り込み |
| `internal/app/usecase/integration/issue.go`（新規）・`import.go`（新規）・`link.go`・`pr.go` | `hh issue`、`hh gh import`、モードの変更、取り込みの共通化 |
| `internal/app/cli/issue.go`（新規）・`gh.go`（新規）・`app.go` | コマンド |
| `internal/app/storage/inbox/inbox.go`・`collect.go` | `issue_overwritten` |
| `internal/app/usecase/tasks/undo.go`・`server/activity.go`・`web/core/cache.ts`・`web/core/activity.ts`・`web/app/detail/history-labels.ts` | op の分類と表示名 |

## Tasks

### Task 1: `synced` の型と印

**Files:** `domain/model/github.go`、`domain/model/github_values_test.go`（新規）、`usecase/github_autolink.go`、`usecase/integration/pr.go`

- [ ] テスト: `SetValues` → `Values` の往復、`Put` のゴールデン（`synced: {title: a, status: open}` の知らないキーと順が残る）、`StripMark` の表（印あり、印のみ、CRLF、他の行の印は残す）
- [ ] 実装: `type IssueValues map[string][]string`。`assignees`・`labels` はリスト、ほかは 1 要素。`func (x IssueState) Values() (IssueValues, error)`、`func (x *IssueState) SetValues(IssueValues) error`（キーは固定の順）、`func MarkedBody(u ULID, body string) string`、`func StripMark(body string) string`
- [ ] `prBody` を `model.MarkedBody` に置き換える
- [ ] `go test ./internal/app/domain/... ./internal/app/usecase/...`、コミット `feat(model): Issue の基準の値と本文の印を足す`

### Task 2: 3 者の比較

**Files:** `domain/automation/issue.go`（新規）、`issue_test.go`（新規）、`eval.go`、`eval_test.go`

- [ ] テスト: `Reflect` の表形式。モード × フィールド × (GitHub が違う / タスクが違う / 両方 / 基準なし / 同じ)。spec の「基準の値」の表をそのまま行にする。`link` は空。`Fields`: リンクの `fields` が先、`issue_fields` が次、`milestone` は `props` なしで外れる、知らない名前は外れる。`StatusToken`。trailer の往復（本文の改行を含む値）。`Input.Status` の起点
- [ ] 実装:

```go
type Overwrite struct{ Field, Side string; Lost []string }
type Reflection struct {
	ToTask, ToGitHub, Synced model.IssueValues
	Overwritten          []Overwrite
}
func (f IssueFields) For(l model.Link) []string
func StatusToken(c schema.Category) string // done→completed, canceled→not_planned, ほか open
func Reflect(mode string, fields []string, task, gh, base model.IssueValues) Reflection
func IssueOverwrittenTrailer(u model.ULID, ref string, o Overwrite) string
func ParseIssueOverwritten(v string) (u model.ULID, ref string, o Overwrite, ok bool)
```

- [ ] `go test ./internal/app/domain/automation`、コミット `feat(automation): Issue の 3 者の比較を足す`

### Task 3: GitHub の Issue の読み書きと fakegh

`issueFields`・fakegh の `issueShape`・`Issue` はいっしょに動かす（`check` が過不足を拒否する）。

**Files:** `lib/github/graphql.go`、`lib/github/issues.go`（新規）、`lib/github/issues_test.go`（新規）、`lib/fakegh/state.go`・`github.go`・`server.go`・`query.go`・`admin.go`、`cmd/fakegh/main.go`、`lib/github/live_test.go`

- [ ] テスト: fakegh を相手に `LinkedItems` が `Title`・`Body`・`Assignees`・`Labels`・`Milestone` を読む。`OpenIssues` がラベルで絞り、ページをまたぎ、PR を含まない。`CreateIssue`・`UpdateIssue`（`state_reason`、`milestone: null`）。`Labels`・`Assignees`・`Milestones`。権限のないトークンは 403。存在しないラベルと割り当てできない人は 422
- [ ] 実装: `Issue` に `Title, Body string; Assignees, Labels []string; Milestone string`。

```go
const issueFields = `number title body state stateReason updatedAt
assignees(first: 100) { nodes { login } }
labels(first: 100) { nodes { name } }
milestone { title }`

type NewIssue struct{ Title, Body string; Assignees, Labels []string; Milestone int }
type IssueUpdate struct {
	Title, Body, State, StateReason *string
	Assignees, Labels               *[]string
	Milestone                       *int // 0 で外す
}
type Milestone struct{ Number int; Title string }
func (c *Client) OpenIssues(owner, name, label string) ([]Issue, RateLimit, error)
func (c *Client) CreateIssue(owner, name string, in NewIssue) (int, error)
func (c *Client) UpdateIssue(owner, name string, n int, u IssueUpdate) error
func (c *Client) Labels(owner, name string) ([]string, error)
func (c *Client) Assignees(owner, name string) ([]string, error)
func (c *Client) Milestones(owner, name string) ([]Milestone, error)
```

- [ ] fakegh: `Repo.Labels []string`・`Repo.Milestones []string`（番号は順）、`Issue.Assignees`・`Labels`・`Milestone`。REST の `POST/PATCH issues`、`GET labels|assignees|milestones`。クエリ `OpenIssues`。`EditIssue` を `IssueEdit` にしてラベルと担当者も変える。`fakegh issue edit` に `--assignee`・`--label`・`--milestone`、`fakegh repo label <owner/repo> <name>`
- [ ] `go test ./internal/lib/... ./cmd/...`、`go vet -tags live ./internal/lib/github`、コミット `feat(github): Issue の読み取りと書き込みを足す`

### Task 4: `mirror`

**Files:** `usecase/github_issues.go`（新規）、`github_issues_test.go`（新規）、`github_fetch.go`、`github_rules.go`、`domain/automation/eval.go`

- [ ] テスト（fakegh）: 基準なしで GitHub へ揃い通知がない。GitHub のタイトル・担当者の変更が入る。タスクの手編集が戻り trailer が出る。closed（completed・not_planned）・再 open。`todo`↔`doing` は動かない。既定でない完了のステータスは動かない。拒否は 1 回だけ。`issue-closed` が `mirror` の後に当たる。not_planned で `canceled` のまま。複数リンクは先頭。`required` に落ちても取り込みは成功し警告になり、次で再試行。`canceled` の既定がない schema は拒否。読めなかった Issue はそのまま
- [ ] 実装:

```go
type mirrorPlan struct {
	ulid        model.ULID
	set         model.IssueValues
	status      string
	change      *automation.Change
	rejected    []automation.Rejection
	synced      map[string]model.IssueValues // ref → 値
	overwritten []issueNote
}
func planMirror(t *model.Task, g fetched, f *automation.File, e *schema.Effective) (*mirrorPlan, error)
func taskValues(t *model.Task, f *automation.File, e *schema.Effective, host string) model.IssueValues
func gitHubValues(x github.Issue) model.IssueValues
func applyMirror(ss *Session, s *store.Store, held *store.Locked, plans []*mirrorPlan, rep *GitHubFetchReport) (failed map[model.ULID]bool)
```

- [ ] `fetchLocked` のループ: `apply` の後に `planMirror`。`a.Changed` なら `runRules` に計画の後のステータスを渡す。`gh-fetch` の後に `applyMirror`、次に `applyChanges`（`mirror` に失敗したタスクは飛ばす）。`rep.Committed` はどれかのコミットで立てる
- [ ] `runRules`: 決定 9 のイベントの絞り込みと `Input.Status`
- [ ] `go test ./internal/app/usecase/...`、コミット `feat(usecase): Issue の mirror を取り込みに足す`

### Task 5: `push`

**Files:** `usecase/github_push.go`（新規）、`github_push_test.go`（新規）、`github_fetch.go`

- [ ] テスト（fakegh）: 基準なしで GitHub へ揃う。印が本文の末尾に付き、比べるときは除かれる（次の取得で書かない）。CRLF の本文で差分が出ない。各フィールドが届く。GitHub の手編集が戻り trailer が出る。`done`・`canceled` で閉じ、再 open。ルールの `done` が同じ同期で閉じる。ない名前は除いて警告し、毎回通知しない。別の host の担当者は除く。書き込みの失敗は警告で `synced` が進まず、次で届く。リンクの `fields` が効く。milestone は `props` で有効。削除したタスクは触らない。Issue が消えたら警告でリンクは残る。読み取りのみの Store は書かない
- [ ] 実装: `fetchLocked` が `fetched` を返し、`fetchStore` が `github.lock` の中、`fetchLocked` の後で `pushIssues(ss, c, e, s, ix, got, &rep)` を呼ぶ。`held` は `fetchLocked` の終わりで放す。repo ごとの `resolver{cl, acct, labels, logins, milestones}` は遅延で読む
- [ ] `go test ./internal/app/usecase/...`、コミット `feat(usecase): Issue の push を足す`

### Task 6: inbox と op の分類

**Files:** `storage/inbox/inbox.go`・`collect.go`、`collect` のテスト、`usecase/tasks/undo.go`、`server/activity.go`、`web/core/cache.ts`・`activity.ts`、`web/app/detail/history-labels.ts`、各テスト

- [ ] テスト: trailer から `issue_overwritten` が担当者とウォッチャーに出る。1 コミットの複数の値が別の項目になる。読めない trailer は飛ばす。`gh-mirror`・`gh-push` が `hh undo` の対象にならない
- [ ] 実装: `IssueOverwritten Kind = "issue_overwritten"`、詳細は `{"ref","field","side","lost"}`。`key` は `AutomationRejected` と同じ扱い。`dropCoveredWatches` に足す
- [ ] `go generate ./internal/app/server`、`pnpm gen` で差分を確かめる
- [ ] コミット `feat(inbox): Issue の上書きの通知と op の分類を足す`

### Task 7: モードの変更と取り込みの共通化

**Files:** `usecase/integration/link.go`・`link_test.go`・`pr.go`

- [ ] テスト: 使用中のリンクの `--sync` の変更で `synced` が消える。`state` は残る
- [ ] 実装: `dropSynced(t, ref)`。`pr.go` の同期と `FetchGitHubRepo` の部分を `fetchNow(ss, c, se, s, repo) []string` に切り出す（`PullRequest` のテストは変えない）
- [ ] コミット `feat(integration): モードを変えたら基準を捨てる`

### Task 8: `hh issue`

**Files:** `usecase/integration/issue.go`（新規）・`issue_test.go`（新規）、`cli/issue.go`（新規）、`cli/app.go`、`cli/testdata`、`cli/json_types_test.go`

- [ ] テスト: `repos` が 1 つ、複数で `--repo` なしは選択のエラー、なしでカレントの repo。作成された Issue の値と印、リンク `push`、`gh.issues` と `synced`。次の同期で何も書かない。`done` のタスクは閉じて作る。ない名前は除いて警告。`link.exists`。削除済みと読み取りのみの拒否
- [ ] 実装: `IssueInput{Repo, Dir string}`、`IssueResult`、`IssueWrite{usecase.TaskWrite; Ref string; Warnings []string}`、`func CreateIssue(ss, arg, in) (IssueResult, error)`。解決は Task 5 の `usecase.NewIssueResolver` を使う
- [ ] コミット `feat(cli): hh issue を足す`

### Task 9: `hh gh import`

**Files:** `usecase/integration/import.go`（新規）・`import_test.go`（新規）、`usecase/tasks/task.go`、`usecase/task_shared.go`、`cli/gh.go`（新規）、`cli/app.go`、`cli/testdata`

- [ ] テスト: 開いた Issue がタスクになる。`mirror`・`repos`・本文（印を除く）・`parent`。本文は次の同期で写らない（`mirror` の既定に `body` がない）。リンク済み（`off` を含む）を飛ばす。2 回目は 0 件。`--dry-run` は件数だけで何も書かない。担当者と closed は取り込みで入る。Workspace が複数なら `--workspace`
- [ ] 実装: `tasks.Add` の既定の種類・ステータス・Workspace の決め方を `usecase.DefaultKind`・`DefaultTodo`・`OnlyWorkspace` に切り出す（既存のテストは変えない）。`ImportInput{Repo, Label, Parent, Workspace string; DryRun bool}`、`ImportResult{Repo string; Created []string; Skipped int; Count int; DryRun bool; Warnings []string}`
- [ ] コミット `feat(cli): hh gh import を足す`

### Task 10: 通しのテストと文書

**Files:** `cli/scenario/github_test.go`、`docs/superpowers/specs/2026-11-05-phase5-github-design.md`、`docs/superpowers/plans/2026-11-05-phase5c-handoff.md`（新規）、`README.md`・`README.ja.md`（コマンドの説明がある場合）

- [ ] シナリオ: ビルドしたバイナリで `hh issue`（push）、`hh gh import`（mirror）、`hh link --sync link`、両方向の手編集の上書きが `hh inbox` に出る
- [ ] spec の 5c に決定 1〜24 のうち形式と動きを足す。`hh gh import` の `--workspace`、`gh-mirror`・`gh-push`、`issue_overwritten`
- [ ] 引き継ぎを書く
- [ ] 全体の検査: `gofmt -l .`、`go vet ./...`、`go vet -tags e2e ./internal/app/cli`、`go vet -tags live ./internal/lib/github`、`golangci-lint run ./...`、`go test -race -count=1 ./...`、`go generate ./internal/app/server`、`pnpm gen`、`pnpm check`、`pnpm biome ci .`、`vitest run`、`pnpm build`、`make e2e`
- [ ] コミット `test(scenario): Issue 同期を通しで確かめる` と `docs: 5c の決定を spec に書き、引き継ぎを足す`
