# フェーズ 5b: 自動化と開発の開始 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `automations.yaml` のルールで PR・Issue の変化からステータスを動かし、`hh start`・`hh pr` と合わせて、CLI だけで開発の開始から PR のマージまでを手動のステータス変更なしで回せるようにする（spec の 5b の完了条件）。

**Architecture:** 形式・検査・イベント判定・ルール評価は I/O のない `domain/automation` に置く。`usecase.Fetch` は取り込みのトランザクションの中でルールを評価し、拒否を取り込みのコミットの trailer に書き、採用した変更をストアのロックを持ったままタスクごとの `set` コミットにする。`hh start`・`hh pr` は `usecase/integration`、ルールの編集は新しい領域 `usecase/automations` に置く。コードの repo の操作は `lib/gitrepo/code.go`（go-git のみ）。

**Tech Stack:** Go、go-git、cobra、GitHub GraphQL・REST API、fakegh。

**Spec:** `docs/superpowers/specs/2026-11-05-phase5-github-design.md` の「`automations.yaml`」「5b 自動化と開発の開始」「テスト」。

**PR:** 文書だけの PR は出さない。この plan と 5a の引き継ぎの更新は、5b の実装の PR に入れる。

## Global Constraints

- コミットの署名は 1Password。失敗したら index に積んで利用者に知らせる
- `make` は `MAKEFLAGS=-j10`。順番が要るものは Makefile の依存で書く
- cli と server は usecase だけを通す（depguard の `entry`）。usecase の領域どうしは import しない。共有するものは usecase 直下に置く
- OS に触る呼び出しは `lib/osfs` を通す（forbidigo）。外部の `git` コマンドは使わない（PATH に git がない CI ジョブがある）
- 生成物（`api/openapi.yaml`、`web/core/api/schema.d.ts`）は `go generate ./internal/app/server` と `pnpm gen` で作る
- 既存のテストの期待値は、この plan が明示したものの他は変えない
- 5c の項目（Issue の `mirror`・`push` の反映、`synced` の書き込み）は作らない。5d の画面と API は作らない
- 各 Task の終わりに `go vet ./...` と、変えたパッケージの `go test` を通してコミットする。コミットメッセージは日本語、`feat(...)`・`test(...)`・`docs:` の形

## この回で決めたこと

spec に書いていなかった細部を、コードを読んで次のとおり決めた。spec にも反映する。

| # | 項目 | 決定 | 理由 |
|---|---|---|---|
| 1 | 評価の時点 | 取り込みのトランザクションの中で評価する | 拒否は取り込みのコミットの trailer に書くので、コミットの前に分かっている必要がある |
| 2 | ステータスのコミットのロック | 取り込みのコミットからルールのコミットまで `store.LockInOrder` を持ち、各 `Op.Locked` に渡す。読み取り（通信）の間は持たない | 評価と書き込みの間に別の書き手が入らないので、評価の結果がそのまま通る。後段で拒否されて trailer を書く先がなくなる場面を作らない |
| 3 | 中断 | 取り込みのコミットの直後に落ちると、そのイベントは失われる | イベントは `gh` の差分から作るので再計算できない。ロックの窓だけの事象で、やり直しのために履歴を読むほどの価値がない。人が手で動かせる |
| 4 | 初めて見る要素 | `gh` に前の値がないときは、`state` は未定、`review` は `none` として判定する。`merged` の PR を初めて見たら `pr.merged`、`closed` の Issue なら `issue.closed`、`pending` なら `pr.review_requested` が発火する | 発火しないと、取得の間に開いてマージされた PR でタスクが動かない |
| 5 | 同じ取得の複数の発火 | ルールを順に当てて、途中の値から `ValidateTransition` する。コミットは最後に通った値へ1つ。trailer は最後に通ったルール。途中で拒否されたルールは飛ばして次へ進み、拒否として記録する。`Op.SkipTransitions` を立てる（通した経路が保証するので、始点と終点の直接の遷移は問わない） | spec の「後が勝つ」と「タスクごとに1コミット」を両立する |
| 6 | `set: {category: c}` | 今のステータスがそのカテゴリなら何もしない。違えば `Schema.DefaultStatus(c)` | 既定でない完了のステータス（`shipped` など）を `done` に戻さない |
| 7 | `all_linked_prs_merged` | `off` でない PR のリンクすべてが、取り込み後の `gh.prs` で `merged`。`gh` にまだ入っていない PR と、`closed` の PR は満たさない | spec のとおり。閉じた PR は `hh unlink` で外す |
| 8 | `where` の適用範囲 | そのプロジェクトが `ix.Ancestors(e)` に入るタスク。`disable` も同じ | `schema.For` と同じ見方 |
| 9 | slug | タイトルを小文字にし、ASCII の英数字の並びを `-` でつなぐ。40 文字まで（単語の途中でも切り、末尾の `-` は落とす）。空なら `hh/<ID>` | 親設計に規則がない。`branchMark` は `hh/<ID>` も読める。`hh pr` は `hh/<ID>` と `hh/<ID>-*` の両方を探す |
| 10 | `hh start` のステータス | すでに `doing` カテゴリなら動かさない | `review` のタスクを `in_progress` に戻さない |
| 11 | `hh start` の担当者 | `ss.AccountID()` を `assignees` に足す。`watchNewAssignees` と同じくウォッチャーにも足す | 親設計の「自分に割り当てる」。`hh set assignees+=` と同じ結果にする。関数は `usecase` 直下に移す |
| 12 | `hh pr` のやり直し | ブランチに開いた PR があれば作らずリンクする（`GET /pulls?head=`）。リンク済みなら飛ばす | push の後で落ちたときに、もう一度打って続きから進める |
| 13 | push の失敗 | fast-forward でないなら止める。強制 push はしない | 利用者の履歴を壊さない |
| 14 | PR のタイトルと本文 | タイトルはタスクの `title`。本文はタスクの `body` の後に空行と `hedgehug:<ULID>`。本文が空なら印だけ | spec のとおり |
| 15 | `dev` テンプレートのルール | `store.Create` は変えず、`StoreInit` が `Create` の直後に `op: init` で `automations.yaml` を書く | `Create` の呼び出しが 23 箇所ある。`init` は `undoSkip` に入っている |
| 16 | `automations.yaml` が読めないとき | 取り込みは続け、警告を出してルールを飛ばす。`hh doctor`・`hh schema check` が `automation` の指摘を出す | 自動化の失敗で同期を止めない。5a の「失敗は警告」と同じ |
| 17 | `hh automation fields` | 引数なしで今の値を出す。`--default-sync`・`--push a,b`・`--mirror a,b`・`--prop milestone=target` で変える | spec の「`...`」を埋める |
| 18 | `5a の通しのテストの置き場所` | `internal/app/cli/github_scenario_test.go` は `cli` に残す | 中身は `newTestEnv` のプロセス内のテストで、`cli/scenario` はビルドしたバイナリを動かす場所。5b の通しのテストだけ `cli/scenario/github_test.go` に足す |
| 19 | web の inbox | 5b では触らない。`describe` の既定の分岐が `種類: automation` と出す | タブと文言は spec の 5d-2 |
| 20 | `hh mv --store` の選択 | `automations.yaml` に対象があるだけでも `--scoped` を求める。元の Store からは、持っていくときも捨てるときも消す | `schema.yaml` の `scoped` と同じ扱いにする |
| 21 | `hh link --issue` と使えない `automations.yaml` | ファイルが読めないか `default_sync` が不正なら、`--sync` なしの指定は `automation.invalid` で失敗する。`--sync` の指定と `--pr` はファイルを読まないので通る | モードはタスクに残る値なので推測しない。決定 16 は同期を止めないための決まりで、利用者がその場で直せるこの操作には当てはめない |

## File Structure

| ファイル | 役割 |
|---|---|
| `internal/app/domain/automation/file.go`（新規） | 形式、読み書き、既定の `issue_fields`、編集 |
| `internal/app/domain/automation/check.go`（新規） | 検査 |
| `internal/app/domain/automation/eval.go`（新規） | イベントの判定、ルールの評価、trailer の形 |
| `internal/app/storage/store/automations.go`（新規） | `Store.Automations()` |
| `internal/app/usecase/github_fetch.go`・`github_apply.go` | ルールの実行、`FetchRepo` |
| `internal/app/usecase/github_rules.go`（新規） | 評価とタスクごとのコミット |
| `internal/app/usecase/integration/link.go` | `issue_fields.default_sync` |
| `internal/app/usecase/integration/start.go`・`pr.go`（新規） | `hh start`・`hh pr` |
| `internal/app/usecase/automations/automations.go`（新規） | ルールの一覧と編集 |
| `internal/app/usecase/schema_findings.go`・`finding.go` | `automation` の指摘 |
| `internal/app/usecase/conn.go` | `ConnForRepo` |
| `internal/app/usecase/assign.go`・`tasks/task.go` | `WatchNewAssignees` の移動 |
| `internal/app/usecase/stores/storeops.go`・`movestore.go`・`movestore_view.go` | `dev` のルール、持ち運び |
| `internal/app/storage/store/move.go` | `MoveIn`・`MoveOut` の `Automations` |
| `internal/app/storage/inbox/inbox.go`・`collect.go`、`usecase/notifications/inbox.go` | `automation`・`automation_rejected` |
| `internal/lib/gitrepo/code.go`（新規） | コードの repo の操作 |
| `internal/lib/github/pulls.go`（新規）・`repos.go` | PR の作成と検索、`default_branch` |
| `internal/lib/fakegh/server.go`・`github.go`・`state.go` | REST の `pulls`、`default_branch` |
| `internal/app/cli/automation.go`・`start.go`・`pr.go`（新規）、`app.go`、`store_init.go`、`mv.go` | コマンド |
| `internal/app/cli/scenario/github_test.go`（新規） | 通しのテスト |

---

### Task 1: 文書の更新

**Files:** Modify: `docs/superpowers/plans/2026-11-05-phase5a-handoff.md`、`docs/superpowers/specs/2026-11-05-phase5-github-design.md`、`docs/superpowers/specs/2026-11-04-codebase-restructure-design.md`

- [ ] **Step 1:** 引き継ぎの冒頭を「PR #61 でマージ済み」に直し、「次」の 1・2 を完了にする。2 の `github_scenario_test.go` は決定 18 を書く
- [ ] **Step 2:** spec の 5b の節に決定 4〜7・9・10・12 を足す
- [ ] **Step 3:** 構成見直しの spec の 2.3 を 14 個にする。`domain/automation`、`usecase/automations` の配置と表は Task 8 の実装と一緒に足す
- [ ] **Step 4:** コミットする（`docs: 5b の plan を書き、5a の引き継ぎを閉じる`）

### Task 2: `automations.yaml` の形式

**Files:** Create: `internal/app/domain/automation/file.go`、`file_test.go`、`testdata/`

`model.Fields` で全キーを順番どおりに持ち、知らないキー・コメントを残す。`.golangci` の `domain` の対象に自動で入ることを `golangci-lint run ./internal/app/domain/...` で確かめる。

```go
package automation

const (
	PROpened          = "pr.opened"
	PRReviewRequested = "pr.review_requested"
	PRMerged          = "pr.merged"
	IssueClosed       = "issue.closed"
	AllLinkedPRsMerged = "all_linked_prs_merged"
	WherePrefix       = "project:"
)

// Events lists what a rule may be on, in the order they are handled.
var Events = []string{PROpened, PRReviewRequested, PRMerged, IssueClosed}

type Set struct {
	Status   string
	Category string
}

type Rule struct {
	ID, On, When, Where string
	Set                 Set
	Enabled             bool // false only when the file says so
	fields              model.Fields
}

type Scoped struct {
	Where   string
	Disable []string
	fields  model.Fields
}

type IssueFields struct {
	DefaultSync string
	Push, Mirror []string
	Props       map[string]string
	fields      model.Fields
}

type File struct {
	Rules       []Rule
	Scoped      []Scoped
	IssueFields IssueFields
	fields      model.Fields
}

func Default() *File                                  // no rules, the default issue_fields of the spec
func DevRules() *File                                 // pr-opened, pr-review, pr-merged, issue-closed
func Decode(b []byte) (*File, error)                  // a missing key has its default
func (f *File) Encode() ([]byte, error)               // the keys in their order, unknown keys kept
func (f *File) Rule(id string) (Rule, bool)
func (f *File) AddRule(r Rule) error                  // automation.exists when the id is taken
func (f *File) RemoveRule(id string) error            // also takes the id out of scoped.disable
func (f *File) SetEnabled(id string, on bool) error   // true drops the enabled key
func (f *File) SetDisabledIn(where, id string, off bool) error
func (f *File) SetIssueFields(in IssueFieldsChange) error
```

`DevRules` の値は次のとおり（`pr-review` は `status: review`）。

```yaml
rules:
  - {id: pr-opened, on: pr.opened, set: {status: in_progress}}
  - {id: pr-review, on: pr.review_requested, set: {status: review}}
  - {id: pr-merged, on: pr.merged, when: all_linked_prs_merged, set: {category: done}}
  - {id: issue-closed, on: issue.closed, set: {category: done}}
```

- [ ] **Step 1:** 失敗するテストを書く。ゴールデン（`testdata/full.yaml`。spec の例に知らないキーとコメントを足したもの）で `Decode` → `Encode` のバイト一致。キーのない既定、`AddRule` の重複、`RemoveRule` が `disable` からも消す、`SetEnabled` の往復（`enabled: true` を書かない）、`SetDisabledIn` が空の `scoped` を消す
- [ ] **Step 2:** `go test ./internal/app/domain/automation` が失敗することを確かめる
- [ ] **Step 3:** 実装する
- [ ] **Step 4:** テストを通す
- [ ] **Step 5:** `merge/yaml_test.go` に `automations.yaml` のテストを足す。両側が別の `rules` を足す、同じ `id` の `enabled` が衝突して新しい側を採る、`issue_fields.push` と `default_sync` をそれぞれが変える。実装の変更は要らない（`elemKey` が `rules`・`scoped` を見ている）
- [ ] **Step 6:** コミットする

### Task 3: 検査

**Files:** Create: `internal/app/domain/automation/check.go`、`check_test.go`、`internal/app/storage/store/automations.go`。Modify: `internal/app/usecase/schema_findings.go`、`finding.go`、`usecase/maintenance/doctor_test.go`、`usecase/schemas`

```go
// Check lists the defects of f against sc. project says whether a ULID is a
// project of the store that is not deleted.
func (f *File) Check(sc *schema.Schema, project func(model.ULID) bool) []schema.Problem
```

報告するもの: `rules[i].id` の空・重複、`on` が `Events` にない、`when` が知らない値、`set` が2つとも・どちらもなし、ステータス（`where` 付きは `sc.For([project]).Statuses()` で見る）・カテゴリがない、`where` の形とプロジェクト、`scoped[i].where` の重複とプロジェクト、`disable` の知らない id、`issue_fields.default_sync` が `mirror`・`push`・`link` 以外、`push`・`mirror` が `title`・`body`・`status`・`assignees`・`labels`・`milestone` 以外、`props` のキーが `milestone` 以外。

`Store.Automations()` は `s.path(model.AutomationsPath)` を読み、ファイルがなければ `automation.Default()`。

`CheckSchema` に足す（`checkOrder` の `override` の後に `automation`）。読めないファイルは1件の指摘。`doctor.checkStore` が消すのは `Check == "schema"` だけなので、両方の経路で出る。

- [ ] **Step 1:** 表形式のテスト（上の項目それぞれ1行以上）と、`hh doctor`・`hh schema check` の両方に指摘が出るテストを書く
- [ ] **Step 2:** 失敗を確かめる
- [ ] **Step 3:** 実装する
- [ ] **Step 4:** テストを通してコミットする

### Task 4: イベントとルールの評価

**Files:** Create: `internal/app/domain/automation/eval.go`、`eval_test.go`

```go
// EventsOf lists what happened between the two values of gh, in the order of
// Events, each name once (decision 4 for an item with no old value).
func EventsOf(old, next model.GitHubState) []string

type Input struct {
	Events    []string
	Task      *model.Task // with next in gh
	Ancestors []model.ULID
	Schema    *schema.Schema
}

type Change struct{ Rule, Event, Status string }
type Rejection struct{ Rule, Reason string }

type Outcome struct {
	Change   *Change // nil when the status stays
	Rejected []Rejection
}

func (f *File) Evaluate(in Input) Outcome

const (
	TrailerApplied  = "Hedgehug-Automation"
	TrailerRejected = "Hedgehug-Automation-Rejected"
)

func AppliedTrailer(c Change) string            // "<rule> <event>"
func ParseApplied(v string) (rule, event string, ok bool)
func RejectedTrailer(u model.ULID, r Rejection) string // "<ULID> <rule> <reason>", the reason on one line
func ParseRejected(v string) (u model.ULID, r Rejection, ok bool)
```

評価は決定 5・6・7・8 のとおり。

```go
func (f *File) Evaluate(in Input) Outcome {
	var out Outcome
	e := in.Schema.For(in.Ancestors)
	cur := in.Task.Status
	for _, event := range in.Events {
		for _, r := range f.rulesFor(event, in.Ancestors) { // store rules, then where rules
			if r.When == AllLinkedPRsMerged && !allMerged(in.Task) {
				continue
			}
			to, skip, err := target(r, cur, e)
			if err != nil {
				out.Rejected = append(out.Rejected, Rejection{r.ID, err.Error()})
				continue
			}
			if skip {
				continue
			}
			if vs := schema.ValidateTransition(cur, to, in.Task, e); len(vs) > 0 {
				out.Rejected = append(out.Rejected, Rejection{r.ID, joined(vs)})
				continue
			}
			cur = to
			out.Change = &Change{r.ID, event, to}
		}
	}
	return out
}
```

`rulesFor` は `Enabled`、`On`、`Where` が祖先にあること、`scoped` の `disable`（`where` が祖先にあるもの）で絞る。元のステータスに戻った場合は `Change` を nil にする。

- [ ] **Step 1:** 表形式のテストを書く。`EventsOf`: 新しい open・draft、`none`→`pending`、`approved`→`pending` は発火しない、`merged` への変化、Issue の `closed`、前の値なしの各場合、2つの PR で名前が1回。`Evaluate`: 適用の順、`where` が後に勝つ、`enabled: false`、`disable`、`when`（未取得・`closed` を含むと不成立、`off` のリンクは数えない）、すでに目的の値、カテゴリの一致、`ValidateTransition` の拒否（プロジェクトの上書き）、拒否の後に次のルールが通る、同じ取得の opened と merged で `done`、1回目で動いて2回目で戻ると `Change` が nil。trailer の往復（理由に空白と改行）
- [ ] **Step 2:** 失敗を確かめる
- [ ] **Step 3:** 実装する
- [ ] **Step 4:** テストを通してコミットする

### Task 5: 取り込みでルールを実行する

**Files:** Modify: `usecase/github_apply.go`、`github_fetch.go`、`export_test.go`。Create: `usecase/github_rules.go`、`github_rules_test.go`

`apply` の戻り値を次に変える。

```go
type applied struct {
	Changed   bool
	Old, Next model.GitHubState
}
func apply(t *model.Task, got fetched, auto []string) (applied, error)
```

`fetchLocked` の書き込みを次の形にする。

```go
held, err := store.LockInOrder(s)
if err != nil { return err }
defer held.Release()
if err := ix.Refresh(); err != nil { return err }
rules, err := s.Automations()
if err != nil {
	rep.Warnings = append(rep.Warnings, model.AutomationsPath+": "+err.Error())
	rules = nil // decision 16
}
var changes []taskChange
hash, err := ss.UpdateCommit(s, store.Op{Name: "gh-fetch", Automated: true, Locked: held}, func(tx *store.Tx) error {
	// ... apply as today ...
	if rules != nil && a.Changed {
		o := rules.Evaluate(automation.Input{Events: automation.EventsOf(a.Old, a.Next), Task: t, Ancestors: ix.Ancestors(e), Schema: s.Schema()})
		for _, r := range o.Rejected {
			tx.AddTrailer(automation.TrailerRejected, automation.RejectedTrailer(u, r))
		}
		if o.Change != nil {
			changes = append(changes, taskChange{u, *o.Change})
		}
	}
	...
})
```

`rules.go`:

```go
func applyChanges(ss *usecase.Session, s *store.Store, held *store.Locked, changes []taskChange, rep *FetchReport) {
	for _, c := range changes {
		_, err := ss.UpdateCommit(s, store.Op{Name: "set", Task: c.ulid, Automated: true, SkipTransitions: true, Locked: held}, func(tx *store.Tx) error {
			t, err := tx.Task(c.ulid)
			if err != nil { return err }
			t.Status = c.Status
			tx.AddTrailer(automation.TrailerApplied, automation.AppliedTrailer(c.Change))
			return tx.PutTask(t)
		})
		if err != nil {
			rep.Warnings = append(rep.Warnings, c.ulid.String()+": "+err.Error())
		}
	}
}
```

`rep.Committed` は取り込みのコミットがあれば true のまま（`set` は取り込みのコミットがあるときしか起きない）。

`FetchRepo(ss, c, e, repo)` を足す。`fetchLocked` に `force string` を渡し、その repo だけ `Due` を見ず、ほかの repo は読まない。

- [ ] **Step 1:** fakegh でテストを書く。`pr.opened` で `in_progress`、レビュー依頼で `review`（`dev` のスキーマ）、すべてのマージで `done`・1つ残るとそのまま、拒否で trailer が取り込みのコミットに付きステータスが動かない、`set` のコミットの author・op・trailer、`hh undo` が `set` を1件戻し次の取得で再発火しない、別のクローンが書いた `gh` を同期しても発火しない（2つの Session）、人が戻した後の取得で再発火しない、削除済みのタスク、`hh unlink`→`hh link`→取得で `pr.opened`、壊れた `automations.yaml` で取り込みが成功し警告が出る、`FetchRepo` が間隔を無視し他の repo を読まない
- [ ] **Step 2:** 失敗を確かめる
- [ ] **Step 3:** 実装する。`ix.Refresh` の呼び出しと `Locked` が `tx` の中の処理と噛み合うことを `go test -race ./internal/app/usecase/...` で確かめる
- [ ] **Step 4:** テストを通してコミットする

### Task 6: `issue_fields.default_sync` を読む

**Files:** Modify: `usecase/integration/link.go`、`link_test.go`、`cli/link.go`

定数 `defaultIssueSync` を消し、`change` の中で `s.Automations()` の `IssueFields.DefaultSync` を `LinkInput.sync` に渡す。`--sync` のヘルプを `(default issue_fields.default_sync)` にする。

- [ ] **Step 1:** `default_sync: mirror` のファイルで `hh link --issue` が `mirror` になるテストを書く。`--sync` が勝つ、ファイルがないと `link`
- [ ] **Step 2:** 失敗を確かめ、実装し、通してコミットする

### Task 7: inbox

**Files:** Modify: `storage/inbox/inbox.go`、`collect.go`、`collect_test.go`、`usecase/notifications/inbox.go`、`inbox_test.go`

`Automation Kind = "automation"`、`AutomationRejected Kind = "automation_rejected"`。`Detail` は `{"rule","event","status"}` と `{"rule","reason"}`。

`Collect` の trailer の switch に足す。`Hedgehug-Automation` は `e.Trailer("Hedgehug-Task")` のタスク、`Hedgehug-Automation-Rejected` は値の ULID のタスクについて、`readTask(c.r, c.tree, u)` の `Assignees` と `Watchers` のうち `me` に当たるものへ `addChange` する（重複は1つ）。「自分の書き込みは通知しない」の判定の外にある。`dropCoveredWatches` の `covered` に `Automation` を足す。

`InboxContent` に足す。

```go
case inbox.Automation:
	return fmt.Sprintf("rule %s moved it to %s (%s)", d.Rule, d.Status, d.Event)
case inbox.AutomationRejected:
	return fmt.Sprintf("rule %s did not move it: %s", d.Rule, d.Reason)
```

- [ ] **Step 1:** テストを書く。本人が担当者なら本人に出る、ウォッチャーに出る、どちらでもない本人には出ない、同期で受け取った別のメンバーに出て `watch` が重ならない、拒否の宛先、壊れた trailer は無視、`hh inbox` の文言
- [ ] **Step 2:** 失敗を確かめ、実装し、通す。`go generate ./internal/app/server` で差分がないことを確かめる（`Kind` は文字列で、enum ではない）
- [ ] **Step 3:** コミットする

### Task 8: `hh automation`

**Files:** Create: `usecase/automations/automations.go`、`automations_test.go`、`testmain_test.go`、`cli/automation.go`、`cli/automation_test.go`。Modify: `cli/app.go`、`.golangci.yml`（領域の一覧があれば）

```go
package automations

type RuleView struct {
	ID       string   `json:"id"`
	On       string   `json:"on"`
	When     string   `json:"when,omitempty"`
	Status   string   `json:"status,omitempty"`
	Category string   `json:"category,omitempty"`
	Project  *usecase.Ref `json:"project,omitempty"`
	Enabled  bool     `json:"enabled"`
	DisabledIn []usecase.Ref `json:"disabled_in" nullable:"false"`
}

type AddInput struct{ ID, On, Status, Category, When, Project string }

func List(ss *usecase.Session, project string) ([]RuleView, error)
func Add(ss *usecase.Session, in AddInput) error
func Remove(ss *usecase.Session, id string) error
func SetEnabled(ss *usecase.Session, id, project string, on bool) error
func Fields(ss *usecase.Session) (FieldsView, error)
func SetFields(ss *usecase.Session, in FieldsInput) error
```

書き込みは `ss.OpenForWrite` → `ss.UpdateCommit(s, store.Op{Name: "automation", Detail: ...}, ...)` で `tx.PutFile(model.AutomationsPath, data)`。書く前に `f.Check` の新しい問題を拒否する（`schema.NewProblems` と同じ考え方）。`--project` は kind が `project` のタスク（`usecase.ProjectKind`）だけ受ける。`List --project` は Store のルールとそのプロジェクトのルールを適用の順に出し、`enabled` にその `scoped` の停止を反映する。

`hh automation list` の表: `ID  ON  WHEN  SET  SCOPE  STATE`。

- [ ] **Step 1:** usecase のテストとコマンドのゴールデンを書く。add・rm・disable・enable（`--project` の有無）・fields、`--json`、存在しないステータスの拒否、重複、閲覧のみの Store の拒否、`hh undo` で戻る
- [ ] **Step 2:** 失敗を確かめ、実装し、通す。`golangci-lint run ./...` で depguard を確かめる
- [ ] **Step 3:** コミットする

### Task 9: `--template dev` のルール

**Files:** Modify: `usecase/stores/storeops.go`、`storeops_test.go`、`cli/store_init.go`、`cli/store_init_test.go`

`StoreInitInput` に `Automations *automation.File` を足す。`Create` の後、`RegisterMergeDriver` の後、push の前に書く。

```go
if in.Automations != nil {
	data, err := in.Automations.Encode()
	if err != nil { return StoreInitResult{}, undo(err) }
	if _, err := s.UpdateCommit(store.Op{Name: "init", Author: author, Account: accountID}, func(tx *store.Tx) error {
		return tx.PutFile(model.AutomationsPath, data)
	}); err != nil {
		return StoreInitResult{}, undo(err)
	}
}
```

- [ ] **Step 1:** テストを書く。`--template dev` で4つのルールが有効、`hh doctor` が通る、`hh undo` の対象にならない、テンプレートなしではファイルがない
- [ ] **Step 2:** 失敗を確かめ、実装し、通してコミットする

### Task 10: `hh mv --store` の持ち運び

**Files:** Modify: `usecase/stores/movestore.go`、`movestore_view.go`、`movestore_test.go`、`storage/store/move.go`、`cli/mv_store_test.go`。Create: `domain/automation/carry.go`、`carry_test.go`

```go
// Carry returns what of src applies to the projects and fits dst: the where
// rules and the disable entries, each dropped thing as "rule <id>" or
// "disable <id>".
func (dst *File) Carry(src *File, projects []model.ULID, sc *schema.Schema) (next *File, dropped []string)
// Without returns f without the where rules and scoped entries of projects.
func (f *File) Without(projects []model.ULID) *File
```

外すもの: ステータス・カテゴリが移動先にないルール、`id` が移動先にあるルール、移動先にない `id` を指す `disable`。

`moveWork` に `automationsIn`・`automationsOut`（エンコード後の `[]byte`、nil は変更なし）を足す。`store.MoveIn`・`MoveOut` に `Automations []byte` を足し、nil でなければ `tx.PutFile`。`ScopedPlan` は `schema.yaml` の対象がなくても `automations.yaml` に対象があれば作る（`Projects` は対象のプロジェクト）。step 6 の `switch j.in.Scoped` は両方を処理する。

再実行（`resumeCheck`・`carriedBefore`）は、移動先の `automations.yaml` に `where` のルールがあるかで `carriedBefore` を足す。

`Dropped` の文書（`movestore_view.go:55`）に `rule`・`disable` を足し、`go generate ./internal/app/server` と `pnpm gen` を流す。

- [ ] **Step 1:** テストを書く。持っていく・捨てる、移動先に合わないものが `Dropped` に出る、`automations.yaml` だけに対象があるとき `--scoped` を求める、元の Store から消える、`--scoped carry` の途中で止まって再実行しても重ならない
- [ ] **Step 2:** 失敗を確かめ、実装し、通す
- [ ] **Step 3:** `git status` で生成物の差分を確かめてコミットする

### Task 11: コードの repo の操作

**Files:** Create: `internal/lib/gitrepo/code.go`、`code_test.go`

```go
// Code is the repository the developer works in, as opposed to a store.
type Code struct{ r *git.Repository }

var ErrNoRepository = errors.New("not in a git repository")
var ErrNoCommit = errors.New("the repository has no commit")

func OpenCode(dir string) (*Code, error)                 // DetectDotGit, EnableDotGitCommonDir
func (c *Code) Branch() (name string, ok bool, err error) // ok is false on a detached HEAD
func (c *Code) HasBranch(name string) (bool, error)
func (c *Code) StartBranch(name string) error            // at HEAD, and HEAD to it; files stay
func (c *Code) Branches(match func(string) bool) ([]string, error) // sorted
func (c *Code) RemoteURL(name string) (string, bool, error)
func (c *Code) PushBranch(remote, branch string, conn Conn) error // ErrRejected, ErrPushDenied as Push
```

`StartBranch` は `Storer.SetReference` で `refs/heads/<name>` を作ってから HEAD を symbolic にする。`git worktree` の中では HEAD が worktree のものになることをテストで確かめる。`PushBranch` は `RefSpec("refs/heads/x:refs/heads/x")`、`NoErrAlreadyUpToDate` は成功。

- [ ] **Step 1:** `gitrepotest` でテストを書く。作業ツリーの未コミットの変更が残る、detached HEAD から作れる、コミットなしは `ErrNoCommit`、linked worktree、ローカルの bare への push、fast-forward でない push は `ErrRejected`
- [ ] **Step 2:** 失敗を確かめ、実装し、通してコミットする

### Task 12: `lib/github` と fakegh の PR

**Files:** Create: `internal/lib/github/pulls.go`、`pulls_test.go`。Modify: `lib/github/repos.go`、`lib/fakegh/state.go`、`server.go`、`github.go`、`server_test.go`、`usecase/conn.go`、`conn_test.go`

```go
// repos.go: Repo に足す
DefaultBranch string `json:"default_branch"`

// pulls.go
type NewPull struct{ Title, Body, Head, Base string; Draft bool }
func (c *Client) CreatePull(owner, name string, p NewPull) (int, error)
func (c *Client) OpenPullFor(owner, name, branch string) (int, bool, error) // GET pulls?state=open&head=owner:branch
```

fakegh: `Repo.DefaultBranch`（JSON は `default_branch`、空なら `main`）を `repoJSON` に足す。`repoRoute` に `POST /repos/o/r/pulls`（`write` 以上。`title`・`head`・`base` が空なら 422、同じ `head` の開いた PR があれば 422）と `GET /repos/o/r/pulls`（`head`・`state` で絞る）を足す。

`usecase.ConnForRepo` を足す。`GitHubForRepo` のアカウントの選び方を `AccountForRepo(c, e, id)` に切り出し、`Conn` の中身を `connOf(c, acct, ok)` に切り出す。

```go
func (ss *Session) ConnForRepo(c *config.Config, e config.StoreEntry, id string) (gitrepo.Conn, config.Account, error)
```

- [ ] **Step 1:** テストを書く。`CreatePull` の要求の形（`httptest`）と draft、422 の説明、`OpenPullFor` の有無、権限のない 403、`ConnForRepo` が `repo_accounts` のトークンを使う
- [ ] **Step 2:** 失敗を確かめ、実装し、通してコミットする

### Task 13: `hh start`

**Files:** Create: `usecase/integration/start.go`、`start_test.go`、`cli/start.go`、`cli/start_test.go`。Modify: `usecase/assign.go`、`usecase/tasks/task.go`、`cli/app.go`

`tasks.watchNewAssignees` を `usecase.WatchNewAssignees` に移し、呼び出しを直す。

```go
type StartResult struct {
	usecase.TaskResult
	Branch  string `json:"branch"`
	Created bool   `json:"created"`
	Switch  bool   `json:"switch"` // the branch is there, and the developer is on another
}

func Slug(title string) string
func Start(ss *usecase.Session, arg, dir string) (StartResult, error)
```

流れは spec の `hh start` のとおり。順番はブランチ → タスクの書き込み（op `start`、`Assignees` と `Watchers`、決定 10 のステータス）。書き込みの拒否はそのまま返す（ブランチは残る）。`cli/start.go` は `a.getwd` の結果を渡す。`Switch` のときは `git switch <branch>` を案内する行を出す。`--json` は `StartResult`。

- [ ] **Step 1:** テストを書く。`Slug` の表（英語、記号、日本語だけで空、40 文字）。作る、すでにそのブランチ、別のブランチにいる（`Switch`）、repo の外のエラー、遷移の拒否でブランチが残り担当者も変わらない、`doing` カテゴリで動かない、もう一度打っても変わらない（`No changes`）
- [ ] **Step 2:** 失敗を確かめ、実装し、通してコミットする

### Task 14: `hh pr`

**Files:** Create: `usecase/integration/pr.go`、`pr_test.go`、`cli/pr.go`、`cli/pr_test.go`。Modify: `cli/app.go`

```go
type PRInput struct{ Dir string; Draft bool }
type PRResult struct {
	usecase.TaskResult
	Ref     string   `json:"ref"`
	Branch  string   `json:"branch"`
	Created bool     `json:"created"`
	Warnings []string `json:"warnings" nullable:"false"`
}

func PullRequest(ss *usecase.Session, arg string, in PRInput) (PRResult, error)
```

流れ:

1. `ForTask`、`OpenForWrite`、`Resolve`、`NotDeleted`
2. `gitrepo.OpenCode(dir)`。ブランチを決める（spec。`hh/<ID>` と `hh/<ID>-*`）。なければ `pr.no_branch` で `hh start <ID>` を案内。複数なら `ChoiceRequired`
3. `RemoteURL("origin")`、`giturl.RepoID`。なければ `pr.no_origin`
4. `ss.GitHubForRepo`・`ss.ConnForRepo`、`PushBranch`
5. `OpenPullFor`。なければ `Repo` の `DefaultBranch` へ `CreatePull`
6. リンク済みでなければ `op: link` でリンクする（`change` の中身を共有）
7. `usecase.FetchRepo` の後、`Committed` なら `ss.MarkWritten(s)`。取り込みの失敗は `Warnings`

エラーは `github.Describe` を通す。push の `ErrRejected` は `pr.rejected`、`ErrPushDenied` は `pr.denied`。

- [ ] **Step 1:** テストを書く（`url_rewrites` で `https://github.com/org-a/web.git` をローカルの bare に向ける）。PR ができ本文の末尾に印、向き先が既定のブランチ、`--draft`、リンクと `gh`、`pr-opened` の発火、もう一度打っても重ならない、ブランチなしの案内、origin なし、fast-forward でない push で止まる、`repo_accounts` のトークンで push と作成が行われる
- [ ] **Step 2:** 失敗を確かめ、実装し、通してコミットする

### Task 15: 通しのテスト

**Files:** Create: `internal/app/cli/scenario/github_test.go`

ビルドしたバイナリで spec の表を順に回す。

1. fakegh（プロセス内の `httptest`）、`url_rewrites`、`--template dev` の Store、origin 付きのコードの repo を作る（go-git のみ）
2. `hh add`、`hh start`: `in_progress`、ブランチ、担当者
3. コミットして `hh pr`: PR、`gh`、`in_progress`
4. fakegh でレビュー依頼。`.hedgehug/local/github.json` の時刻を戻して `hh sync`: `review`
5. マージ、同じく `hh sync`: `done`
6. `hh inbox` に `automation` が3件。手動の `hh status` は一度も打たない

- [ ] **Step 1:** `scenario` の補助を確かめて書く
- [ ] **Step 2:** `go test ./internal/app/cli/scenario -run GitHub` を通し、`PATH` から git を外しても通ることを確かめる
- [ ] **Step 3:** コミットする

### Task 16: 全体の検査と引き継ぎ

- [ ] **Step 1:** 次をすべて通す。`gofmt -l .`、`go vet ./...`、`go vet -tags e2e ./internal/app/cli`、`go vet -tags live ./internal/lib/github`、`golangci-lint run ./...`、`go test -race -count=1 ./...`、`go generate ./internal/app/server` と `pnpm gen` の後に生成物の差分なし、`pnpm check`、`pnpm biome ci .`、`vitest run`、`pnpm build`、`make e2e`
- [ ] **Step 2:** `docs/superpowers/plans/2026-11-05-phase5b-handoff.md` を書く（5c が使うもの、plan から変えた点）
- [ ] **Step 3:** 親設計の「自動化ルール」「タスクから開発を始める」に決定 4〜7・9・10・12 を反映する
- [ ] **Step 4:** コミットして PR を出す

## spec との対応

| spec の項目 | Task |
|---|---|
| `automations.yaml` の形式・マージ | 2 |
| `hh doctor`・`hh schema check` | 3 |
| イベント、ルールの実行 | 4、5 |
| 通知 | 7 |
| `hh start` | 11、13 |
| `hh pr` | 11、12、14 |
| `hh automation` | 8 |
| `--template dev` | 9 |
| `hh mv --store` | 10 |
| `issue_fields.default_sync` | 6 |
| 通しのテスト | 15 |
