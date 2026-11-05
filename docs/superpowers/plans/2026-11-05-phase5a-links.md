# フェーズ 5a: リンクと取り込み Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **注意:** 実装は構成見直しの回 6 のマージ後の main から切る。着手前に、plan のパスと行番号をその時点のコードで確かめ直す。この plan の行番号と関数名は 2026-11-05 の main（4a9cf71）のもの。

**Goal:** `hh link`・`hh unlink` と自動リンクでタスクと PR・Issue を結び、`hh sync` で状態を `gh` に取り込み、`hh show`・`has:pr` で見られるようにする（spec の 5a の完了条件）。

**Architecture:** `gh` の値は `model.Task.GH`（`*yaml.Node`）のまま持ち、型付きの読み書きを `Task.GitHub`・`Task.SetGitHub` で行う。`lib/github` に GraphQL を足し、`lib/fakegh` が同じクエリに答える。取り込みは新しい領域 `usecase/integration` の `Fetch` が担い、`replication.SyncStore` が同期の後に呼ぶ。repo ごとの取得時刻は `storage/ghstate` が `.hedgehug/local/github.json` に持つ。

**Tech Stack:** Go、go-git、huma（OpenAPI の再生成のみ）、GitHub GraphQL API、GitHub Actions。

**Spec:** `docs/superpowers/specs/2026-11-05-phase5-github-design.md` の「データの形」「5a リンクと取り込み」「パッケージ」「テスト」。

**PR:** 文書だけの PR は出さない。spec と引き継ぎの変更（ブランチ `phase5-design` の e134ccf まで）と、この plan を、5a の PR に入れる。

## Global Constraints

- コミットは利用者が席にいるときだけ（1Password の署名）。離席中は index に積んでおく
- `make` は `MAKEFLAGS=-j10` で並列に動く。順番が要るものは Makefile の依存で書く
- 必須チェックの名前 `test` を変えない。ruleset と必須チェックは Terraform 管理なので `gh api` で直接変えない
- cli と server は usecase だけを通す（depguard の `entry`）。テストも同じ。`lib/fakegh` と `usecasetest` は使ってよい
- OS に触る呼び出しは `lib/osfs` を通す（forbidigo）
- 生成物（`api/openapi.yaml`、`web/core/api/schema.d.ts`）は手で書かず、`go generate ./internal/app/server` と `pnpm gen` で作る
- 既存のテストの期待値は、この plan が明示したものの他は変えない
- 各 Task の終わりに `go vet ./...` と、変えたパッケージの `go test` を通す。全体の検査は Task 16 で行う
- 5b 以降の項目（`automations.yaml`、ルール、`issue_fields`、Issue の `mirror`・`push` の反映）は作らない。`synced`・`state_reason`・`Link.Fields` は型とマージだけを入れる

## この回で決めたこと

spec に書いていない細部を、コードを読んで次のとおり決めた。

| # | 項目 | 決定 | 理由 |
|---|---|---|---|
| 1 | `Task.GH` の型 | `*yaml.Node` のまま持ち、`GitHub()`・`SetGitHub()` で型付きに読み書きする | `model/task_test.go` が `task.GH == nil`、`merge/task_test.go` が `gh.UnmarshalYAML(x.GH)` を使っており、spec の「テストを変えない」と両立する形がこれ。知らないキーもそのまま残る |
| 2 | `links` の衝突 | 追加の実装なし。テストで固定する | `setField` は同じ `ref` で値が違う要素を `pick` で扱い、`updated` の新しい側を採る。負けた値は `links` の Overwritten に入る。spec の条件を満たす |
| 3 | `hh undo` | `undoSkip` に `gh-fetch` を足す | 取り込みのコミットも `Hedgehug-Author` が本人なので、足さないと `hh undo` が取り消してしまう |
| 4 | `hh link --issue` の既定 | `link` 固定 | `issue_fields.default_sync` は 5b で読めるようになる。5c の Task でこの定数を置き換える |
| 5 | Issue の自動リンク | しない | 親設計の自動リンクの対象は PR の走査だけ |
| 6 | 初回の走査 | 更新の新しい 100 件まで | 前回の取得時刻がないとき、全件を読むと大きな repo でレート制限を使い切る。リンク済みのものは別のクエリで必ず読む |
| 7 | リンク済みの読み取り | 毎回すべて読む（1クエリ 100 件ずつ） | 新しくリンクしたものの更新時刻は前回の取得より古いことがある |
| 8 | `checks` | CI がない PR は書かない（`omitempty`） | spec の3値のどれにも当たらない。5d のチップは `—` と同じ扱いにする |
| 9 | `hh show --json` の `links` | `omitempty` | 既存のゴールデンと web のテストの型を変えずに足せる |
| 10 | リンク先を持たない `gh` の要素 | 取り込みのときに外す | `hh unlink` と `hh undo` のどちらでも、古い要素が残らない |
| 11 | remote のない Store | `hh sync --store` で取り込む（push はしない） | 個人の Store でも PR の状態は要る |
| 12 | 取り込みの多重実行 | `github.lock` で1つにする。`sync.lock` とは別 | 取り込みの後の push が `sync.lock` を取るため |
| 13 | 活動の分類 | `gh-fetch` は `syncOps` と web の `SYNC_OPS` に入れる | 人の変更に混ざらないようにする。「自動化」の絞り込みができる 5d-1 で、そちらへ移す（spec の 5a の表） |
| 14 | ref の大文字小文字 | 正規形は小文字。`ParseGitHubRef` が小文字にして読み、`String()` は小文字を返す。比較は `model.CanonRef` を通し、手で書かれた `links` は書き換えない | GitHub は repo の大文字小文字を区別しない。`merge` の `links` の見分け方は変えないので、既存のデータはそのまま読める |
| 15 | `hh show --json` の `links[]` | `merged_at`・`state_reason` を足す（`omitempty`） | 5d の画面が merge の時刻と Issue を閉じた理由を使う |

## File Structure

| ファイル | 役割 |
|---|---|
| `internal/app/domain/model/github.go`（新規） | リンクの定数、`GitHubRef`、`PRState`・`IssueState`、`Task.GitHub`・`SetGitHub` |
| `internal/app/domain/model/task.go` | `Link.Fields` |
| `internal/app/domain/merge/task.go` | `gh` の配列のマージ |
| `internal/app/remote/replicate/ops_test.go` | プロパティテストの操作 |
| `internal/app/storage/index/entry.go`・`build.go` | `Entry.Links`・`Repos`、`cacheFormat` |
| `internal/app/storage/query/eval.go` | `has:pr` |
| `internal/app/usecase/tasks/edit.go`・`undo.go` | `gh` の拒否、`gh-fetch` の除外 |
| `internal/app/storage/store/tx.go`・`config/author.go`・`usecase/session.go` | 自動化の author |
| `internal/lib/github/graphql.go`（新規） | GraphQL のクライアント |
| `internal/lib/fakegh/github.go`（新規）・`state.go`・`server.go`・`admin.go`、`cmd/fakegh/main.go` | PR・Issue・GraphQL・操作のコマンド |
| `internal/app/storage/config/hosts.go`、`usecase/conn.go` | `repo_accounts` の選択 |
| `internal/app/storage/ghstate/ghstate.go`（新規） | 取得時刻とレート制限 |
| `internal/app/usecase/integration/link.go`・`autolink.go`・`fetch.go`（新規） | リンク、自動リンク、取り込み |
| `internal/app/usecase/replication/sync.go` | 取り込みの呼び出し |
| `internal/app/cli/link.go`（新規）・`show.go`・`app.go` | コマンドと表示 |
| `internal/app/usecase/read_shared.go`・`browse/read.go` | `ShowView.Links` |
| `internal/lib/github/live_test.go`（新規）、`.github/workflows/github-live.yaml`（新規） | 実 API の定期テスト |

---

### Task 1: `gh` の型とリンクの定数

**Files:**
- Create: `internal/app/domain/model/github.go`、`internal/app/domain/model/github_test.go`
- Modify: `internal/app/domain/model/task.go:44-48`、`internal/app/domain/model/node.go`（`Doc.drop`）、`internal/app/domain/merge/task_test.go`（`Link` の比較）

- [ ] **Step 1: 失敗するテストを書く**

```go
package model

import (
	"strings"
	"testing"
	"time"
)

func taskWith(t *testing.T, front string) *Task {
	t.Helper()
	x, err := DecodeTask([]byte("---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n" + front + "---\n"))
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestGitHubReadsAndKeepsUnknownKeys(t *testing.T) {
	t.Parallel()
	x := taskWith(t, "gh:\n  number: 1\n  prs:\n    - ref: github.com/o/r/pull/1\n      state: open\n      review: none\n      merged_at: null\n      note: keep\n")
	g, err := x.GitHub()
	if err != nil || len(g.PRs) != 1 || g.PRs[0].State != "open" || g.PRs[0].Review != "none" {
		t.Fatalf("GitHub = %+v, %v", g, err)
	}
	at := time.Date(2026, 11, 5, 1, 2, 3, 0, time.UTC)
	g.PRs[0].State, g.PRs[0].MergedAt = "merged", &at
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	b, err := x.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"number: 1", "state: merged", "note: keep", "merged_at: 2026-11-05T01:02:03Z"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("no %q in\n%s", want, b)
		}
	}
}

func TestGitHubEmptyAndWrongShape(t *testing.T) {
	t.Parallel()
	if g, err := taskWith(t, "").GitHub(); err != nil || len(g.PRs) != 0 {
		t.Errorf("none = %+v, %v", g, err)
	}
	if g, err := taskWith(t, "gh: {}\n").GitHub(); err != nil || len(g.Issues) != 0 {
		t.Errorf("empty = %+v, %v", g, err)
	}
	if _, err := taskWith(t, "gh: text\n").GitHub(); err == nil {
		t.Error("a scalar reads")
	}
	x := taskWith(t, "")
	if err := x.SetGitHub(GitHubState{}); err != nil || x.GH != nil {
		t.Errorf("nothing to keep: %v, %v", x.GH, err)
	}
}

const ghFront = `gh:
  number: 1.0 # keep
  prs:
    - ref: github.com/o/r/pull/1
      extra: [1.0, 2]   # unknown
      state: open
      review: none
      merged_at: null
    - state: open
      ref: github.com/o/r/pull/2
      review: approved
      note: {a: 1.0}
      merged_at: 2026-11-05T01:02:03Z
  issues:
    - ref: github.com/o/r/issues/3
      state: closed
      state_reason: null
      synced:
        title: "x" # comment
        n: 1.0
`

func encoded(t *testing.T, x *Task) string {
	t.Helper()
	b, err := x.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetGitHubUnchangedKeepsBytes(t *testing.T) {
	t.Parallel()
	x := taskWith(t, ghFront)
	before := encoded(t, x)
	g, err := x.GitHub()
	if err != nil {
		t.Fatal(err)
	}
	if g.Issues[0].StateReason != nil || g.Issues[0].Synced == nil || g.PRs[0].MergedAt != nil || g.PRs[1].MergedAt == nil {
		t.Fatalf("issue = %+v", g.Issues[0])
	}
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	if after := encoded(t, x); after != before {
		t.Errorf("round trip changed the file:\n%s\nwant\n%s", after, before)
	}
	for _, want := range []string{"number: 1.0 # keep", "extra: [1.0, 2]", "note: {a: 1.0}", "n: 1.0", "state_reason: null", "merged_at: 2026-11-05T01:02:03Z", "title: \"x\" # comment"} {
		if !strings.Contains(before, want) {
			t.Errorf("no %q in\n%s", want, before)
		}
	}
}

func TestSetGitHubChangeKeepsTheOthers(t *testing.T) {
	t.Parallel()
	x := taskWith(t, ghFront)
	before := encoded(t, x)
	g, _ := x.GitHub()
	g.PRs[0].State = "merged"
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(before, "state: open\n    review: none", "state: merged\n    review: none", 1)
	if got := encoded(t, x); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	g, _ = x.GitHub()
	r := "completed"
	g.Issues[0].StateReason = &r
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	got := encoded(t, x)
	if !strings.Contains(got, "state_reason: completed") || !strings.Contains(got, "n: 1.0") || !strings.Contains(got, "extra: [1.0, 2]") {
		t.Errorf("got\n%s", got)
	}
}

func TestSetGitHubDropsAnItemAndKeepsKeysOfTheRest(t *testing.T) {
	t.Parallel()
	x := taskWith(t, ghFront)
	g, _ := x.GitHub()
	g.PRs = g.PRs[1:]
	g.PRs[0].State = "closed"
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	got := encoded(t, x)
	if strings.Contains(got, "pull/1\n") || !strings.Contains(got, "note: {a: 1.0}") || strings.Contains(got, "extra: [1.0, 2]") || !strings.Contains(got, "state: closed") {
		t.Errorf("got\n%s", got)
	}
}

func TestSetGitHubEmptiedLeavesGHOut(t *testing.T) {
	t.Parallel()
	x := taskWith(t, ghFront)
	if err := x.SetGitHub(GitHubState{}); err != nil {
		t.Fatal(err)
	}
	if got := encoded(t, x); strings.Contains(got, "prs:") || strings.Contains(got, "issues:") || !strings.Contains(got, "number: 1.0 # keep") {
		t.Errorf("other keys stay:\n%s", got)
	}
	y := taskWith(t, "gh:\n  prs:\n    - ref: github.com/o/r/pull/1\n      state: open\n      review: none\n      merged_at: null\n")
	if err := y.SetGitHub(GitHubState{}); err != nil || y.GH != nil {
		t.Errorf("gh = %v, %v", y.GH, err)
	}
	if strings.Contains(encoded(t, y), "gh:") {
		t.Error("gh is written")
	}
}

func TestSetGitHubReplacesANonMapping(t *testing.T) {
	t.Parallel()
	x := taskWith(t, "gh: text\n")
	g := GitHubState{PRs: []PRState{{Ref: "github.com/o/r/pull/1", State: "open", Review: "none"}}}
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	if got := encoded(t, x); strings.Contains(got, "text") || !strings.Contains(got, "pull/1") {
		t.Errorf("got\n%s", got)
	}
	y := taskWith(t, "gh: text\n")
	if err := y.SetGitHub(GitHubState{}); err != nil || y.GH != nil {
		t.Errorf("gh = %v, %v", y.GH, err)
	}
}

func TestLinkFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	x := taskWith(t, "links:\n  - {type: github_issue, ref: github.com/o/r/issues/3, sync: mirror, fields: [title, status]}\n")
	if len(x.Links) != 1 || len(x.Links[0].Fields) != 2 || x.Links[0].Fields[1] != "status" {
		t.Fatalf("links = %+v", x.Links)
	}
	before := encoded(t, x)
	x.Links[0].Sync = SyncPush
	got := encoded(t, x)
	if !strings.Contains(before, "fields: [title, status]") || !strings.Contains(got, "sync: push") || !strings.Contains(got, "fields: [title, status]") {
		t.Errorf("before\n%s\nafter\n%s", before, got)
	}
}

func TestGitHubRef(t *testing.T) {
	t.Parallel()
	r, ok := ParseGitHubRef("github.com/org-a/web/pull/123")
	if !ok || r.Repo != "github.com/org-a/web" || r.Kind != LinkPR || r.Number != 123 {
		t.Fatalf("ParseGitHubRef = %+v, %v", r, ok)
	}
	if r.String() != "github.com/org-a/web/pull/123" || r.Label() != "org-a/web#123" {
		t.Errorf("%s, %s", r, r.Label())
	}
	i, ok := ParseGitHubRef("github.com/org-a/web/issues/45")
	if !ok || i.Kind != LinkIssue || i.String() != "github.com/org-a/web/issues/45" {
		t.Errorf("issue = %+v, %v", i, ok)
	}
	for _, bad := range []string{"", "github.com/org-a/web", "github.com/org-a/web/pull/0", "org-a/web/pull/1"} {
		if _, ok := ParseGitHubRef(bad); ok {
			t.Errorf("%q parses", bad)
		}
	}
	if !(Link{Sync: SyncMirror}).Live() || (Link{Sync: SyncOff}).Live() {
		t.Error("Live")
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/domain/model -run 'GitHub'`
Expected: FAIL（`GitHub`、`ParseGitHubRef` が未定義）

- [ ] **Step 3: 実装する**

`task.go` の `Link` に `Fields` を足す。

```go
type Link struct {
	Type   string   `yaml:"type"`
	Ref    string   `yaml:"ref"`
	Sync   string   `yaml:"sync,omitempty"`
	Fields []string `yaml:"fields,omitempty,flow"`
}
```

`task.go:32` のコメントを `// values from GitHub, read-only; read them with GitHub` に直す。

`node.go` の `Doc` に、キーを消す `drop` を足す（`SetGitHub` が `gh` を空にしたとき、`Encode` が `gh: null` を書かないため）。

```go
// drop removes key from the mapping, so that the file no longer has it.
func (d *Doc) drop(key string) {
	m := d.mapping()
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i:i], m.Content[i+2:]...)
			return
		}
	}
}
```

`github.go`:

```go
package model

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"
)

// The types of a link, and how a link syncs.
const (
	LinkPR    = "github_pr"
	LinkIssue = "github_issue"

	SyncMirror = "mirror"
	SyncPush   = "push"
	SyncLink   = "link"
	SyncOff    = "off" // unlinked: kept so that the automatic linking does not link it again
)

// Live reports whether the link is in use.
func (l Link) Live() bool { return l.Sync != SyncOff }

// GitHubRef is the ref of a link: <host>/<owner>/<repo>/pull/<n> or
// <host>/<owner>/<repo>/issues/<n>, in lower case: GitHub does not tell
// repositories apart by case, so a ref has one form.
type GitHubRef struct {
	Repo   string // <host>/<owner>/<repo>
	Kind   string // LinkPR or LinkIssue
	Number int
}

var githubRef = regexp.MustCompile(`^([^/]+/[^/]+/[^/]+)/(pull|issues)/([1-9][0-9]*)$`)

// ParseGitHubRef reads the ref of a link.
func ParseGitHubRef(s string) (GitHubRef, bool) {
	m := githubRef.FindStringSubmatch(strings.ToLower(s))
	if m == nil {
		return GitHubRef{}, false
	}
	n, err := strconv.Atoi(m[3])
	if err != nil {
		return GitHubRef{}, false
	}
	kind := LinkPR
	if m[2] == "issues" {
		kind = LinkIssue
	}
	return GitHubRef{Repo: m[1], Kind: kind, Number: n}, true
}

// CanonRef is s in the form String gives, or s itself when it is not a ref.
// Links written by hand may be in other letters; compare them through this.
func CanonRef(s string) string {
	if r, ok := ParseGitHubRef(s); ok {
		return r.String()
	}
	return s
}

// String is the ref as a link holds it.
func (r GitHubRef) String() string {
	part := "pull"
	if r.Kind == LinkIssue {
		part = "issues"
	}
	return fmt.Sprintf("%s/%s/%d", r.Repo, part, r.Number)
}

// Label is the ref as people write it: org-a/web#123.
func (r GitHubRef) Label() string {
	_, full, _ := strings.Cut(r.Repo, "/")
	return fmt.Sprintf("%s#%d", full, r.Number)
}

// PRState is what GitHub says of a pull request. The keys it does not know
// stay in the file: SetGitHub merges into the nodes that are there.
type PRState struct {
	Ref      string     `yaml:"ref"`
	State    string     `yaml:"state"`            // open, draft, merged, closed
	Review   string     `yaml:"review"`           // none, pending, approved, changes_requested
	Checks   string     `yaml:"checks,omitempty"` // success, failure, pending; empty with no checks
	MergedAt *time.Time `yaml:"merged_at"`
}

// IssueState is what GitHub says of an issue. Synced is the values as they
// were last made the same, which the issue sync of 5c reads and writes.
type IssueState struct {
	Ref         string     `yaml:"ref"`
	State       string     `yaml:"state"` // open, closed
	StateReason *string    `yaml:"state_reason"`
	Synced      *yaml.Node `yaml:"synced,omitempty"`
}

// GitHubState is the part of gh that Hedgehug writes. The other keys of gh
// stay in the file as they are.
type GitHubState struct {
	PRs    []PRState    `yaml:"prs,omitempty"`
	Issues []IssueState `yaml:"issues,omitempty"`
}

// GitHub reads gh. A task with none has an empty state; a gh that is not a
// mapping is an error, and SetGitHub would replace it.
func (t *Task) GitHub() (GitHubState, error) {
	var g GitHubState
	if t.GH == nil || t.GH.Tag == "!!null" {
		return g, nil
	}
	if t.GH.Kind != yaml.MappingNode {
		return g, &DecodeError{Field: "gh", Err: fmt.Errorf("want a mapping")}
	}
	if err := t.GH.Decode(&g); err != nil {
		return g, &DecodeError{Field: "gh", Err: err}
	}
	// The decoder reads a null into a pointer as a pointer to a zero value.
	for i := range g.PRs {
		if p := &g.PRs[i]; p.MergedAt != nil && p.MergedAt.IsZero() {
			p.MergedAt = nil
		}
	}
	for i := range g.Issues {
		if x := &g.Issues[i]; x.StateReason != nil && *x.StateReason == "" {
			x.StateReason = nil
		}
	}
	return g, nil
}

// SetGitHub writes prs and issues of gh and keeps the rest as it is: the
// other keys, and in the items of prs and issues the keys and the parts that
// did not change, with their comments and look. With nothing to keep it
// leaves gh out.
func (t *Task) SetGitHub(g GitHubState) error {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if t.GH != nil && t.GH.Kind == yaml.MappingNode {
		c := *t.GH
		c.Content = append([]*yaml.Node(nil), t.GH.Content...)
		n = &c
	}
	if err := putGH(n, "prs", g.PRs, len(g.PRs) == 0); err != nil {
		return err
	}
	if err := putGH(n, "issues", g.Issues, len(g.Issues) == 0); err != nil {
		return err
	}
	if len(n.Content) == 0 {
		t.GH = nil
		if t.doc != nil {
			t.doc.drop("gh") // or Encode writes the key as null
		}
		return nil
	}
	t.GH = n
	return nil
}

// putGH sets key of the mapping n to v, or removes it when empty. The node
// that n holds for key is merged with v, not replaced.
func putGH(n *yaml.Node, key string, v any, empty bool) error {
	at := -1
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			at = i
		}
	}
	if empty {
		if at >= 0 {
			n.Content = append(n.Content[:at:at], n.Content[at+2:]...)
		}
		return nil
	}
	value := new(yaml.Node)
	if err := value.Encode(v); err != nil {
		return err
	}
	if at >= 0 {
		n.Content[at+1] = mergeByRef(n.Content[at+1], value, reflect.ValueOf(v))
		return nil
	}
	n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
	return nil
}

// mergeByRef merges the list nv (the encoding of v, a slice of structs with a
// ref) into old. An item takes the old item with the same ref, so that the
// keys it does not know stay with it, and an item with a new ref starts bare.
func mergeByRef(old, nv *yaml.Node, v reflect.Value) *yaml.Node {
	if old.Kind != yaml.SequenceNode || v.Len() != len(nv.Content) {
		return nv
	}
	refOf := func(n *yaml.Node) string {
		if n.Kind != yaml.MappingNode {
			return ""
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == "ref" {
				return n.Content[i+1].Value
			}
		}
		return ""
	}
	items := make([]*yaml.Node, len(nv.Content))
	for i, x := range nv.Content {
		items[i] = x
		for _, o := range old.Content {
			if r := refOf(o); r != "" && r == refOf(x) {
				items[i] = mergeNode(o, x, v.Index(i))
				break
			}
		}
	}
	out := *old
	out.Content = items
	return &out
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/domain/...`
Expected: PASS（`merge/task_test.go` の `gh` のテストは通る。ただし `Link` が `Fields`（スライス）を持って comparable でなくなるので、`merge/task_test.go` の `slices.Equal(x.Links, want)` の2箇所だけ、`reflect.DeepEqual` を使う `sameLink` と `slices.EqualFunc` に直す。期待値は変えない）

- [ ] **Step 5: コミットする**

```bash
git add internal/app/domain/model
git commit -m "feat(model): gh の PR・Issue の型とリンクの定数を足す"
```

---

### Task 2: `gh` の配列を要素ごとにマージする

**Files:**
- Modify: `internal/app/domain/merge/task.go`（`gh`、`ghList`、`edited`）、`fields.go`（`field.machine`）
- Test: `internal/app/domain/merge/gh_test.go`（新規）

- [ ] **Step 1: 失敗するテストを書く**

`task`・`side`・`decoded`・`mergeTask`・`mergeWith` は `helpers_test.go` の補助をそのまま使う。次のことを固定する。

- `prs` を `ref` で見分けて要素ごとにマージし、衝突は新しい側を採って Overwritten に出さない
- 要素内と `gh` 直下の知らないキーが残る。ours と theirs を入れ替えても結果が同じ
- 片側が要素を消し、他方が同じ要素を変えたら変更が残る。別の要素を変えたら消えたまま
- `ref` のない要素を含む配列は、配列ごとにマージする
- `OursWins` では ours を採り、記録しない
- `gh` と `title` が同時に衝突したら、`title` だけが Overwritten に残る
- `gh` だけを更新した側は、削除との競合で編集に数えない（削除が残り、Revived が出ない）。`links` の変更は数える

```go
package merge

import (
	"strings"
	"testing"
)

const (
	refA = "github.com/o/r/pull/1"
	refB = "github.com/o/r/pull/2"
)

func prLines(state1, state2 string) []string {
	return []string{
		"gh:", "  prs:",
		"    - ref: " + refA, "      state: " + state1, "      review: none", "      merged_at: null",
		"    - ref: " + refB, "      state: " + state2, "      review: none", "      merged_at: null",
	}
}

func TestGHMergesPRsByRef(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	ours := task("2026-10-31T11:00:00Z", prLines("merged", "open")...)
	theirs := task("2026-10-31T12:00:00Z", prLines("open", "closed")...)
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	x := decoded(t, r.Data)
	g, err := x.GitHub()
	if err != nil || len(g.PRs) != 2 || g.PRs[0].State != "merged" || g.PRs[1].State != "closed" {
		t.Fatalf("prs = %+v, %v\n%s", g.PRs, err, r.Data)
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestGHConflictTakesTheLaterAndTellsNobody(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	ours := task("2026-10-31T11:00:00Z", prLines("merged", "open")...)
	theirs := task("2026-10-31T12:00:00Z", prLines("closed", "open")...)
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	g, err := decoded(t, r.Data).GitHub()
	if err != nil || g.PRs[0].State != "closed" {
		t.Fatalf("prs = %+v, %v", g.PRs, err)
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestGHAddsTheElementsOfBothSides(t *testing.T) {
	t.Parallel()
	one := func(ref string) []string {
		return []string{"gh:", "  prs:", "    - ref: " + ref, "      state: open", "      review: none", "      merged_at: null"}
	}
	r := mergeTask(t,
		side(task("2026-10-31T10:00:00Z"), ""),
		side(task("2026-10-31T11:00:00Z", one(refA)...), "a"),
		side(task("2026-10-31T12:00:00Z", one(refB)...), "b"))
	if s := string(r.Data); !strings.Contains(s, refA) || !strings.Contains(s, refB) {
		t.Errorf("merged:\n%s", s)
	}
}

// The value of a key that is not a list of refs merges as before.
func TestGHOtherKeysAsBefore(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", "gh:", "  prs: [1]")
	ours := task("2026-10-31T11:00:00Z", "gh:", "  prs: [2]")
	theirs := task("2026-10-31T12:00:00Z", "gh:", "  prs: [3]")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	if !strings.Contains(string(r.Data), "prs: [3]") {
		t.Errorf("merged:\n%s", r.Data)
	}
}

// Two people change the same link: the later wins, as links are told apart
// by ref (setField). This fixes unlink against a change of the mode.
func TestLinkOfBothSidesTakesTheLater(t *testing.T) {
	t.Parallel()
	link := func(sync string) []string {
		return []string{"links:", "  - {type: github_issue, ref: github.com/o/r/issues/1, sync: " + sync + "}"}
	}
	r := mergeTask(t,
		side(task("2026-10-31T10:00:00Z", link("link")...), ""),
		side(task("2026-10-31T11:00:00Z", link("off")...), "a"),
		side(task("2026-10-31T12:00:00Z", link("push")...), "b"))
	x := decoded(t, r.Data)
	if len(x.Links) != 1 || x.Links[0].Sync != "push" {
		t.Errorf("links = %+v", x.Links)
	}
	if len(r.Overwritten) != 1 || r.Overwritten[0].Field != "links" {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

// A key of an item that this version does not know merges key by key, and
// stays when only one side has it.
func TestGHKeepsUnknownKeysOfItems(t *testing.T) {
	t.Parallel()
	item := func(state, extra string) []string {
		l := []string{"gh:", "  prs:", "    - ref: " + refA, "      state: " + state, "      review: none", "      merged_at: null"}
		if extra != "" {
			l = append(l, "      "+extra)
		}
		return l
	}
	base := task("2026-10-31T10:00:00Z", item("open", "")...)
	ours := task("2026-10-31T11:00:00Z", item("merged", "")...)
	theirs := task("2026-10-31T12:00:00Z", item("open", "future: 1")...)
	for _, c := range []struct{ o, t []byte }{{ours, theirs}, {theirs, ours}} {
		r := mergeTask(t, side(base, ""), side(c.o, "a"), side(c.t, "b"))
		s := string(r.Data)
		if !strings.Contains(s, "future: 1") || !strings.Contains(s, "state: merged") || len(r.Overwritten) != 0 {
			t.Errorf("merged:\n%s\noverwritten %v", s, r.Overwritten)
		}
	}
}

// A key of gh this version does not know stays beside prs.
func TestGHKeepsUnknownKeys(t *testing.T) {
	t.Parallel()
	pr := func(state string, extra ...string) []string {
		return append([]string{"gh:", "  prs:", "    - ref: " + refA, "      state: " + state, "      review: none", "      merged_at: null"}, extra...)
	}
	base := task("2026-10-31T10:00:00Z", pr("open")...)
	ours := task("2026-10-31T11:00:00Z", pr("merged")...)
	theirs := task("2026-10-31T12:00:00Z", pr("closed", "  future: {a: 1}")...)
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	if s := string(r.Data); !strings.Contains(s, "future:") || !strings.Contains(s, "state: closed") || len(r.Overwritten) != 0 {
		t.Errorf("merged:\n%s\noverwritten %v", s, r.Overwritten)
	}
}

// The result does not depend on which clone is ours.
func TestGHMergeIsSymmetric(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	a := task("2026-10-31T11:00:00Z", prLines("merged", "open")...)
	b := task("2026-10-31T12:00:00Z", prLines("closed", "closed")...)
	r1 := mergeTask(t, side(base, ""), side(a, "a"), side(b, "b"))
	r2 := mergeTask(t, side(base, ""), side(b, "b"), side(a, "a"))
	if string(r1.Data) != string(r2.Data) {
		t.Errorf("%s\n---\n%s", r1.Data, r2.Data)
	}
}

// A side that only updated gh did not edit the task: the deletion by the
// other side stays, and nobody is told the task came back.
func TestGHOnlyChangeDoesNotReviveADeletedTask(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	del := task("2026-10-31T11:00:00Z", append(prLines("open", "open"), "deleted_at: 2026-10-31T11:00:00Z")...)
	fetched := task("2026-10-31T12:00:00Z", prLines("merged", "open")...)
	for _, c := range []struct{ o, t Side }{{side(del, "a"), side(fetched, "b")}, {side(fetched, "b"), side(del, "a")}} {
		r := mergeTask(t, side(base, ""), c.o, c.t)
		x := decoded(t, r.Data)
		if x.DeletedAt == nil || len(r.Notices) != 0 {
			t.Errorf("deleted_at %v notices %v", x.DeletedAt, r.Notices)
		}
	}
}

// A change of links is a person's edit, so it still revives.
func TestLinksChangeRevivesADeletedTask(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z")
	del := task("2026-10-31T11:00:00Z", "deleted_at: 2026-10-31T11:00:00Z")
	linked := task("2026-10-31T12:00:00Z", "links:", "  - {type: github_pr, ref: "+refA+", sync: link}")
	r := mergeTask(t, side(base, ""), side(del, "a"), side(linked, "b"))
	if x := decoded(t, r.Data); x.DeletedAt != nil || len(r.Notices) != 1 {
		t.Errorf("deleted_at %v notices %v", x.DeletedAt, r.Notices)
	}
}

// Only the title is told: a conflict in gh is not.
func TestGHAndTitleConflictTellsOnlyTheTitle(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", "title: x", "gh:", "  prs: [1]")
	ours := task("2026-10-31T11:00:00Z", "title: o", "gh:", "  prs: [2]")
	theirs := task("2026-10-31T12:00:00Z", "title: t", "gh:", "  prs: [3]")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	want := []Overwritten{{Path: taskPath, Field: "title", Value: `"o"`, Author: "a"}}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

// One side removed an element and the other changed it, or another one.
func TestGHRemovedAndChangedElement(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	ours := task("2026-10-31T11:00:00Z", "gh:", "  prs:", "    - ref: "+refA, "      state: open", "      review: none", "      merged_at: null")
	// B was removed by ours and changed by theirs: the change stays.
	theirs := task("2026-10-31T12:00:00Z", prLines("open", "closed")...)
	g, err := decoded(t, mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b")).Data).GitHub()
	if err != nil || len(g.PRs) != 2 || g.PRs[1].Ref != refB || g.PRs[1].State != "closed" {
		t.Errorf("prs = %+v, %v", g.PRs, err)
	}
	// B was removed by ours while theirs changed A: B is gone.
	theirs = task("2026-10-31T12:00:00Z", prLines("merged", "open")...)
	g, err = decoded(t, mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b")).Data).GitHub()
	if err != nil || len(g.PRs) != 1 || g.PRs[0].Ref != refA || g.PRs[0].State != "merged" {
		t.Errorf("prs = %+v, %v", g.PRs, err)
	}
}

// Items without a ref cannot be told apart: the list merges whole.
func TestGHItemsWithoutRefMergeWhole(t *testing.T) {
	t.Parallel()
	list := func(s string) []string { return []string{"gh:", "  prs:", "    - state: " + s} }
	r := mergeTask(t,
		side(task("2026-10-31T10:00:00Z", list("open")...), ""),
		side(task("2026-10-31T11:00:00Z", list("merged")...), "a"),
		side(task("2026-10-31T12:00:00Z", list("closed")...), "b"))
	if s := string(r.Data); !strings.Contains(s, "state: closed") || strings.Contains(s, "merged") || len(r.Overwritten) != 0 {
		t.Errorf("merged:\n%s\noverwritten %v", s, r.Overwritten)
	}
}

func TestGHUnderOursWins(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	ours := task("2026-10-31T11:00:00Z", prLines("merged", "open")...)
	theirs := task("2026-10-31T12:00:00Z", prLines("closed", "closed")...)
	r := mergeWith(t, OursWins, side(base, ""), side(ours, "a"), side(theirs, "b"))
	g, err := decoded(t, r.Data).GitHub()
	if err != nil || g.PRs[0].State != "merged" || g.PRs[1].State != "closed" || len(r.Overwritten) != 0 {
		t.Errorf("prs = %+v, %v, overwritten %v", g.PRs, err, r.Overwritten)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/domain/merge -run 'GH|LinkOfBoth|Links'`
Expected: マージ系のテストと `TestGHOnlyChangeDoesNotReviveADeletedTask` が FAIL。他は現状で PASS（固定のためのテスト）

- [ ] **Step 3: 実装する**

`task.go` の import に `"go.yaml.in/yaml/v4"` を足し、`gh` を次に置き換える。`ghList` は関数にする（`taskMerge` は `fileMerge[model.Task]` の別名で、インスタンス化した型にメソッドは定義できない）。`gh` は、どの経路でもその間に増えた Overwritten を切り詰める。

```go
// ghLists are the lists of gh whose items are told apart by ref.
var ghLists = []string{"prs", "issues"}

// gh merges the values from GitHub key by key, or whole when one side's is
// not a mapping. prs and issues merge item by item. Neither side's value is
// recorded as lost: the next fetch brings GitHub's.
func gh(m *taskMerge) {
	lost := len(m.res.Overwritten)
	defer func() { m.res.Overwritten = m.res.Overwritten[:lost] }()
	b, o, t := m.b.GH, m.o.GH, m.t.GH
	if model.NodeEqual(o, t) || !isMapping(o) || !isMapping(t) || b != nil && !isMapping(b) {
		m.dst.GH = m.node("gh", b, o, t, m.dst.GH, false)
		return
	}
	bf, of, tf := fieldsOf(b), fieldsOf(o), fieldsOf(t)
	f := m.fields("gh.", bf, of, tf, fieldsOf(m.dst.GH), false)
	for i := range f {
		if slices.Contains(ghLists, f[i].Key) {
			if v := ghList(m, "gh."+f[i].Key, bf.Get(f[i].Key), of.Get(f[i].Key), tf.Get(f[i].Key)); v != nil {
				f[i].Value = v
			}
		}
	}
	m.dst.GH = mappingLike(m.dst.GH, f)
}

// ghList merges a list of gh that both sides changed, item by item, the
// items told apart by ref. It is nil when the node merge already has the
// answer: a side did not change the list, or an item has no ref.
func ghList(m *taskMerge, name string, b, o, t *yaml.Node) *yaml.Node {
	isList := func(n *yaml.Node) bool { return n != nil && n.Kind == yaml.SequenceNode }
	if !isList(o) || !isList(t) || model.NodeEqual(b, o) || model.NodeEqual(b, t) || model.NodeEqual(o, t) {
		return nil
	}
	var bs []*yaml.Node
	if isList(b) {
		bs = b.Content
	}
	ref := func(n *yaml.Node) string {
		if v := fieldsOf(n).Get("ref"); v != nil {
			return v.Value
		}
		return ""
	}
	if slices.ContainsFunc(slices.Concat(bs, o.Content, t.Content), func(n *yaml.Node) bool { return ref(n) == "" }) {
		return nil
	}
	// The keys of an item keep the order of the side the result is built on.
	top, other := t, o
	if m.oursTop {
		top, other = o, t
	}
	items := mergeSet(bs, o.Content, t.Content, ref)
	for i, x := range items {
		r := ref(x)
		bi, oi, ti := find(bs, r, ref), find(o.Content, r, ref), find(t.Content, r, ref)
		if oi < 0 || ti < 0 {
			continue
		}
		var bx *yaml.Node
		if bi >= 0 {
			bx = bs[bi]
		}
		d, from := top.Content[find(top.Content, r, ref)], other.Content[find(other.Content, r, ref)]
		f := m.fields(name+".", fieldsOf(bx), fieldsOf(o.Content[oi]), fieldsOf(t.Content[ti]), fieldsOf(d), false)
		items[i] = mappingLike(d, f, from)
	}
	out := *top
	out.Content = items
	return &out
}
```

`fields.go` の `field` に、機械だけが書く値の印を足す。

```go
	// machine marks a field only a machine writes, which does not count as
	// an edit of the file.
	machine bool
```

`taskFields` の `gh` の要素に `machine: true` を足し、`edited` から外す。`edited` の呼び出し元は `taskKind.after` の `deletedAt` だけで、`TaskChanges` は別に `taskFields` を見るので変わらない。

```go
// edited reports whether x changed a field of b other than updated,
// deleted_at and gh, which a machine writes: a fetch must not bring back a
// task someone deleted.
func edited(b, x *model.Task) bool {
	return slices.ContainsFunc(taskFields, func(f field[model.Task]) bool { return !f.machine && !f.same(b, x) })
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/domain/merge`
Expected: PASS

- [ ] **Step 5: コミットする**

```bash
git add internal/app/domain/merge
git commit -m "feat(merge): gh の prs と issues を ref で見分けてマージする"
```

---

### Task 3: プロパティテストに `links` と `gh` の同時更新を足す

**Files:**
- Modify: `internal/app/remote/replicate/ops_test.go:51` と `opLabel` の下

- [ ] **Step 1: テストを足す**

`ops` の `{"label", opLabel},` の下に足す。

```go
	{"link", opLink},
	{"gh", opGH},
```

`opLabel` の下に足す。`write` を返さないのは、`gh` の値は負けても記録されず、`links` は `off` と別のモードがぶつかると片方が消えるため。収束と不変条件だけを確かめる。

```go
// opLink links a task to one of a few pull requests, or unlinks it: clones
// change the same link at the same time. It keeps no write: a value that
// loses is recorded, and the check of the clones being the same covers it.
func opLink(c *cluster, i int, rnd *rand.Rand) []write {
	t := c.view(i).pickTask(rnd, live)
	if t == nil {
		return nil
	}
	ref := fmt.Sprintf("github.com/o/r/pull/%d", rnd.Intn(3)+1)
	sync := []string{model.SyncMirror, model.SyncOff}[rnd.Intn(2)]
	c.update(i, c.taskOp(i, "link", t.ULID), func(tx *store.Tx) error {
		x, err := tx.Task(t.ULID)
		if err != nil {
			return err
		}
		x.Links = slices.DeleteFunc(slices.Clone(x.Links), func(l model.Link) bool { return l.Ref == ref })
		x.Links = append(x.Links, model.Link{Type: model.LinkPR, Ref: ref, Sync: sync})
		return tx.PutTask(x)
	})
	return nil
}

// opGH writes what a fetch writes: the state of one of a few pull requests.
func opGH(c *cluster, i int, rnd *rand.Rand) []write {
	t := c.view(i).pickTask(rnd, live)
	if t == nil {
		return nil
	}
	ref := fmt.Sprintf("github.com/o/r/pull/%d", rnd.Intn(3)+1)
	state := []string{"open", "draft", "merged", "closed"}[rnd.Intn(4)]
	c.update(i, c.taskOp(i, "gh-fetch", t.ULID), func(tx *store.Tx) error {
		x, err := tx.Task(t.ULID)
		if err != nil {
			return err
		}
		g, err := x.GitHub()
		if err != nil {
			return err
		}
		g.PRs = slices.DeleteFunc(slices.Clone(g.PRs), func(p model.PRState) bool { return p.Ref == ref })
		g.PRs = append(g.PRs, model.PRState{Ref: ref, State: state, Review: "none"})
		if err := x.SetGitHub(g); err != nil {
			return err
		}
		return tx.PutTask(x)
	})
	return nil
}
```

`ops_test.go` の import に `fmt`・`slices` がなければ足す。

- [ ] **Step 2: 通ることを確かめる**

Run: `go test -count=1 ./internal/app/remote/replicate -run 'Property|Converge|Random'`
Expected: PASS。全クローンの `gh` が同じ値に収束する。落ちたら `ghList` の順序（`mergeSet` の結果の順）を疑い、`-run` の seed の出力を作業記録に残す

- [ ] **Step 3: コミットする**

```bash
git add internal/app/remote/replicate/ops_test.go
git commit -m "test(replicate): リンクと gh の同時更新をプロパティテストに足す"
```

---

### Task 4: 索引に `links`・`repos` を持たせ、`has:pr` を足す

**Files:**
- Modify: `internal/app/storage/index/entry.go:16-48,88-106`、`internal/app/storage/index/build.go:183`
- Modify: `internal/app/storage/query/eval.go`（`term` の switch と `is` の下）
- Modify: `web/app/list/suggest.ts:22`
- Test: `internal/app/storage/query/eval_test.go`、`internal/app/storage/index/promised_test.go`（`TestOpenCacheEqualsRebuild` に `Links`・`Repos` を持つタスクを足す）

- [ ] **Step 1: 失敗するテストを書く**

`eval_test.go` に足す。補助は `helpers_test.go` の `newStore`・`newTask`・`put`・`mustOpen`・`run` を使う。

```go
func TestRunHasPR(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	linked, unlinked, issue, none := newTask("linked"), newTask("unlinked"), newTask("issue"), newTask("none")
	linked.Links = []model.Link{{Type: model.LinkPR, Ref: "github.com/o/r/pull/1", Sync: model.SyncMirror}}
	unlinked.Links = []model.Link{{Type: model.LinkPR, Ref: "github.com/o/r/pull/2", Sync: model.SyncOff}}
	issue.Links = []model.Link{{Type: model.LinkIssue, Ref: "github.com/o/r/issues/1", Sync: model.SyncLink}}
	put(t, s, linked, unlinked, issue, none)
	ix := mustOpen(t, s)
	if got := run(t, ix, "has:pr", Options{}); !reflect.DeepEqual(got, [][]string{{"linked"}}) {
		t.Errorf("has:pr = %v", got)
	}
	q, err := Parse("has:wiki")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ix, q, Options{}); err == nil {
		t.Error("has:wiki is accepted")
	}
}
```

`reflect` が import になければ足す。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/storage/query -run HasPR`
Expected: FAIL（`unknown key "has"`）

- [ ] **Step 3: 実装する**

`entry.go` の `Entry` の `BlockedBy` の下に足す。

```go
	Repos     []string     // the repositories of the task
	Links     []model.Link // the links, those unlinked included
```

`entryOf` の `BlockedBy: t.BlockedBy,` の下に足す。

```go
		Repos:     t.Repos,
		Links:     t.Links,
```

`build.go:183` の `cacheFormat = 8` を `9` にする。

`eval.go` の `term` の `case "is":` の下に足す。

```go
	case "has":
		return c.has(t)
```

`is` の下に足す。

```go
// has matches what a task is linked to.
func (c *compiler) has(t Term) (pred, error) {
	var ps []pred
	for _, v := range t.Values {
		if v != "pr" {
			return nil, c.fail(t, "unknown condition has:%s", v)
		}
		ps = append(ps, func(e *index.Entry) bool {
			return slices.ContainsFunc(e.Links, func(l model.Link) bool { return l.Type == model.LinkPR && l.Live() })
		})
	}
	return anyOf(ps), nil
}
```

`web/app/list/suggest.ts` のキーの一覧の `"is",` の下に `"has",` を足す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/storage/... && cd web && pnpm vitest run app/list`
Expected: PASS。`index/build_test.go` は `cacheFormat` の定数を使うので変えずに通る

- [ ] **Step 5: コミットする**

```bash
git add internal/app/storage web/app/list/suggest.ts
git commit -m "feat(query): has:pr を足す"
```

---

### Task 5: `hh edit` で `gh` の変更を拒否する

**Files:**
- Modify: `internal/app/usecase/tasks/edit.go`（`CheckEdit` の switch）
- Test: `internal/app/usecase/tasks/edit_test.go`

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestCheckEditRefusesGH(t *testing.T) {
	t.Parallel()
	const head = "---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nid: ME-1\ntitle: a\n"
	old, err := model.DecodeTask([]byte(head + "gh:\n  prs:\n    - {ref: github.com/o/r/pull/1, state: open, review: none, merged_at: null}\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	edited := []byte(head + "gh:\n  prs:\n    - {ref: github.com/o/r/pull/1, state: merged, review: none, merged_at: null}\n---\n")
	_, err = tasks.CheckEdit(nil, old.ULID, old, edited)
	if err == nil || !strings.Contains(err.Error(), "gh cannot be changed") {
		t.Fatalf("err = %v", err)
	}
	same := []byte(head + "gh:\n  prs:\n    - {ref: github.com/o/r/pull/1, state: open, review: none, merged_at: null}\n---\n")
	if _, err := tasks.CheckEdit(nil, old.ULID, old, same); err != nil {
		t.Fatalf("the same gh: %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/usecase/tasks -run CheckEditRefusesGH`
Expected: FAIL（`err = <nil>`）

- [ ] **Step 3: 実装する**

`CheckEdit` の `case !slices.Equal(t.FormerIDs, old.FormerIDs):` の下に足す。

```go
	case !model.NodeEqual(t.GH, old.GH):
		return nil, fixed("gh")
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/usecase/tasks`
Expected: PASS

- [ ] **Step 5: コミットする**

```bash
git add internal/app/usecase/tasks
git commit -m "feat(edit): gh の手での変更を拒否する"
```

---

### Task 6: 自動化の author と、`gh-fetch` の扱い

**Files:**
- Modify: `internal/app/storage/config/author.go`、`internal/app/storage/store/tx.go:26-30`、`internal/app/usecase/session.go:534`
- Modify: `internal/app/usecase/tasks/undo.go:75`、`internal/app/server/activity.go:62`
- Modify: `web/app/detail/history-labels.ts`、`history-labels.test.ts`（`link`・`unlink`・`gh-fetch` の名前）
- Modify: `web/core/activity.ts:9`、`web/core/activity.test.ts`、`web/core/cache.ts:15`
- Test: `internal/app/usecase/tasks/undo_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`undo_test.go` に足す。

```go
func TestUndoSkipsTheFetchOfGitHub(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "b"}}); err != nil {
		t.Fatal(err)
	}
	s, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	u := ix.Tasks()[0].ULID
	if _, err := ts.Update(s, store.Op{Name: "gh-fetch", Automated: true}, func(tx *store.Tx) error {
		x, err := tx.Task(u)
		if err != nil {
			return err
		}
		if err := x.SetGitHub(model.GitHubState{PRs: []model.PRState{{Ref: "github.com/o/r/pull/1", State: "open", Review: "none"}}}); err != nil {
			return err
		}
		return tx.PutTask(x)
	}); err != nil {
		t.Fatal(err)
	}
	log, err := usecasetest.LogOf(s.Repo(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(log[0].Author.Name, "hedgehug-automation") {
		t.Errorf("author = %q", log[0].Author.Name)
	}
	if v, _ := log[0].Trailer("Hedgehug-Author"); v == "" {
		t.Error("no Hedgehug-Author: who ran it is not told")
	}
	res, err := tasks.Undo(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if res.Undone == nil || res.Undone.Op == nil || *res.Undone.Op != "set" {
		t.Errorf("undone = %+v", res.Undone)
	}
	// gh is a machine's value, which nobody touched: it is not reported as kept,
	// and it stays while the title goes back.
	for _, k := range res.Kept {
		if k.Field == "gh" || strings.HasPrefix(k.Field, "gh.") {
			t.Errorf("Kept has %+v", k)
		}
	}
	_, ix, _ = ts.Open()
	e, _ = ts.Resolve(ix, "ME-1")
	if e.Title != "a" {
		t.Errorf("title after undo = %q; want a", e.Title)
	}
	tk, err := s.Task(e.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if g, err := tk.GitHub(); err != nil || len(g.PRs) != 1 {
		t.Errorf("gh after undo = %+v, %v; want the pr kept", g, err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/usecase/tasks -run UndoSkips`
Expected: FAIL（`Automated` が未定義）

- [ ] **Step 3: 実装する**

`config/author.go` に足す。

```go
// AutomationAuthor is the author of what Hedgehug wrote on its own while
// acct ran it.
func AutomationAuthor(acct Account) gitrepo.Signature {
	s := CommitAuthor(acct)
	s.Name = "hedgehug-automation (" + acct.Login + ")"
	return s
}
```

`store/tx.go` の `Op` の `Author` の下に足す。

```go
	// Automated says that Hedgehug wrote this on its own: the session makes
	// hedgehug-automation the author. Account stays the one who ran it.
	Automated bool
```

`session.go:534` を次に置き換える。

```go
	op.Author = config.CommitAuthor(acct)
	if op.Automated {
		op.Author = config.AutomationAuthor(acct)
	}
```

`undo.go:75` の `undoSkip` に足す。`undoer.keep` では、`merge.TaskChanges` の結果から `gh`（と `gh.` で始まるもの）を除く。`gh` は機械の値で、除かないと `gh-fetch` を飛ばした `hh undo` の `Kept` に毎回出る。

`web/app/detail/history-labels.ts` の `labels` に `link: "リンク"`、`unlink: "リンク解除"`、`"gh-fetch": "GitHub の取り込み"` を足し、`history-labels.test.ts` の op の一覧にも足す。

```go
var undoSkip = []string{"merge", "repair", "setup", "undo", "init", "gh-fetch"}
```

`server/activity.go:62`:

```go
var syncOps = []string{"merge", "repair", "gh-fetch"}
```

`web/core/activity.ts` の `OP_LABELS` の `watch` の前に足す。

```ts
  link: "リンク",
  unlink: "リンクの解除",
  "gh-fetch": "GitHub の取り込み",
```

`web/core/activity.test.ts` の op の一覧と期待値に3つを足す。`web/core/cache.ts:15`:

```ts
const SYNC_OPS = new Set(["merge", "repair", "gh-fetch"]);
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/usecase/tasks ./internal/app/server ./internal/app/storage/... && cd web && pnpm vitest run core/activity core/cache`
Expected: PASS

- [ ] **Step 5: コミットする**

```bash
git add internal/app web/core
git commit -m "feat(store): 取り込みのコミットを hedgehug-automation で書き、undo の対象から外す"
```

---

### Task 7: GraphQL のクライアント

**Files:**
- Create: `internal/lib/github/graphql.go`、`internal/lib/github/graphql_test.go`、`internal/lib/github/graphql_edge_test.go`
- Create: `internal/lib/github/testdata/scan-pulls.json`、`internal/lib/github/testdata/linked.json`

`testdata` の2つは、GitHub の GraphQL のスキーマの形で手で書いた応答である。Task 15 の実 API のテストが、同じクエリが実際の GitHub に通ることを確かめる。

- [ ] **Step 1: 応答の記録を書く**

`testdata/scan-pulls.json`:

```json
{"data":{"rateLimit":{"limit":5000,"remaining":4812,"resetAt":"2026-11-05T12:00:00Z"},
"repository":{"pullRequests":{"pageInfo":{"hasNextPage":false,"endCursor":"c2"},"nodes":[
{"number":12,"title":"Login","body":"hedgehug:01JBZ8K3QW7M2X9V4T6R5N8P0A","state":"OPEN","isDraft":false,"mergedAt":null,"updatedAt":"2026-11-05T10:00:00Z","headRefName":"hh/WEB-42-login","reviewDecision":"REVIEW_REQUIRED","reviewRequests":{"totalCount":1},"latestOpinionatedReviews":{"nodes":[]},"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"SUCCESS"}}}]},"comments":{"nodes":[{"body":"see WEB-7"}]}},
{"number":11,"title":"Old","body":"","state":"MERGED","isDraft":false,"mergedAt":"2026-11-04T09:00:00Z","updatedAt":"2026-11-04T09:00:00Z","headRefName":"fix","reviewDecision":"APPROVED","reviewRequests":{"totalCount":0},"latestOpinionatedReviews":{"nodes":[]},"commits":{"nodes":[{"commit":{"statusCheckRollup":null}}]},"comments":{"nodes":[]}},
{"number":10,"title":"Draft","body":"","state":"OPEN","isDraft":true,"mergedAt":null,"updatedAt":"2026-11-03T09:00:00Z","headRefName":"wip","reviewDecision":null,"reviewRequests":{"totalCount":0},"latestOpinionatedReviews":{"nodes":[]},"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"FAILURE"}}}]},"comments":{"nodes":[]}}
]}}}}
```

`testdata/linked.json`:

```json
{"data":{"rateLimit":{"limit":5000,"remaining":90,"resetAt":"2026-11-05T12:00:00Z"},
"repository":{
"p12":{"number":12,"title":"Login","body":"","state":"CLOSED","isDraft":false,"mergedAt":null,"updatedAt":"2026-11-05T10:00:00Z","headRefName":"x","reviewDecision":"CHANGES_REQUESTED","reviewRequests":{"totalCount":0},"latestOpinionatedReviews":{"nodes":[]},"commits":{"nodes":[]},"comments":{"nodes":[]}},
"p13":null,
"i45":{"number":45,"state":"CLOSED","stateReason":"NOT_PLANNED","updatedAt":"2026-11-05T10:00:00Z"}}},
"errors":[{"type":"NOT_FOUND","path":["repository","p13"],"message":"Could not resolve to a PullRequest with the number of 13."}]}
```

- [ ] **Step 2: 失敗するテストを書く**

```go
package github

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recorded answers /graphql with the file, and hands back the requests.
func recorded(t *testing.T, file string) (*Client, *[]map[string]any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", file)) //nolint:gosec // a file of the test
	if err != nil {
		t.Fatal(err)
	}
	var reqs []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			t.Errorf("path %s", r.URL.Path)
		}
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		reqs = append(reqs, b)
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return New(Options{API: srv.URL}, "t", nil), &reqs
}

func TestScanPulls(t *testing.T) {
	t.Parallel()
	c, _ := recorded(t, "scan-pulls.json")
	since := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	pulls, rl, err := c.ScanPulls("org-a", "web", since)
	if err != nil {
		t.Fatal(err)
	}
	if rl.Remaining != 4812 || rl.Limit != 5000 {
		t.Errorf("rate limit = %+v", rl)
	}
	if len(pulls) != 2 {
		t.Fatalf("pulls = %+v", pulls)
	}
	p := pulls[0]
	if p.Number != 12 || p.State != "open" || p.Review != "pending" || p.Checks != "success" || p.Head != "hh/WEB-42-login" || len(p.Comments) != 1 {
		t.Errorf("first = %+v", p)
	}
	q := pulls[1]
	if q.State != "merged" || q.Review != "approved" || q.Checks != "" || q.MergedAt == nil {
		t.Errorf("second = %+v", q)
	}
}

func TestScanPullsFirstTimeReadsOnePage(t *testing.T) {
	t.Parallel()
	c, reqs := recorded(t, "scan-pulls.json")
	pulls, _, err := c.ScanPulls("org-a", "web", time.Time{})
	if err != nil || len(pulls) != 3 || len(*reqs) != 1 || pulls[2].State != "draft" || pulls[2].Checks != "failure" || pulls[2].Review != "none" {
		t.Fatalf("pulls = %+v, %d requests, %v", pulls, len(*reqs), err)
	}
}

func TestLinkedItems(t *testing.T) {
	t.Parallel()
	c, reqs := recorded(t, "linked.json")
	got, rl, err := c.LinkedItems("org-a", "web", []int{12, 13}, []int{45})
	if err != nil {
		t.Fatal(err)
	}
	if rl.Remaining != 90 {
		t.Errorf("rate limit = %+v", rl)
	}
	if len(got.Pulls) != 1 || got.Pulls[0].State != "closed" || got.Pulls[0].Review != "changes_requested" {
		t.Errorf("pulls = %+v", got.Pulls)
	}
	if len(got.Issues) != 1 || got.Issues[0].State != "closed" || got.Issues[0].StateReason != "not_planned" {
		t.Errorf("issues = %+v", got.Issues)
	}
	if len(got.MissingPulls) != 1 || got.MissingPulls[0] != 13 {
		t.Errorf("missing = %+v", got)
	}
	q, _ := (*reqs)[0]["query"].(string)
	if !strings.Contains(q, "p13: pullRequest(number: 13)") || !strings.Contains(q, "i45: issue(number: 45)") {
		t.Errorf("query:\n%s", q)
	}
}

func TestLinkedItemsAskInBatches(t *testing.T) {
	t.Parallel()
	c, reqs := recorded(t, "linked.json")
	numbers := make([]int, 150)
	for i := range numbers {
		numbers[i] = i + 1
	}
	if _, _, err := c.LinkedItems("org-a", "web", numbers, nil); err != nil {
		t.Fatal(err)
	}
	if len(*reqs) != 2 {
		t.Errorf("%d requests", len(*reqs))
	}
}

func graphqlErr(t *testing.T, answer string) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(answer)) }))
	t.Cleanup(srv.Close)
	_, _, err := New(Options{API: srv.URL}, "t", nil).ScanPulls("o", "r", time.Time{})
	return err
}

func TestGraphQLErrors(t *testing.T) {
	t.Parallel()
	if err := graphqlErr(t, `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"],"message":"no"}]}`); !errors.Is(err, ErrNotFound) {
		t.Errorf("not found: %v", err)
	}
	if err := graphqlErr(t, `{"data":null,"errors":[{"type":"FORBIDDEN","message":"Resource not accessible by integration"}]}`); !errors.Is(err, ErrForbidden) {
		t.Errorf("forbidden: %v", err)
	}
	var rl *RateLimitError
	if err := graphqlErr(t, `{"data":null,"errors":[{"type":"RATE_LIMITED","message":"used up"}]}`); !errors.As(err, &rl) {
		t.Errorf("rate limited: %v", err)
	}
	if err := graphqlErr(t, `{"data":null,"errors":[{"message":"boom"}]}`); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("other: %v", err)
	}
}

func TestGraphQLURL(t *testing.T) {
	t.Parallel()
	for api, want := range map[string]string{
		"https://api.github.com":     "https://api.github.com/graphql",
		"https://ghe.example/api/v3": "https://ghe.example/api/graphql",
	} {
		if got := New(Options{API: api}, "t", nil).graphqlURL(); got != want {
			t.Errorf("%s: %s", api, got)
		}
	}
}
```

`graphql_edge_test.go`（レビューの判定、`state_reason` の写し方、path に整数を含むエラー、`RATE_LIMITED` の `X-RateLimit-Reset`、401・403・429・5xx、`data` なしの応答、番号の重複）:

```go
package github

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type reviewNodes = struct {
	Nodes []struct {
		State string `json:"state"`
	} `json:"nodes"`
}

func reviews(states ...string) reviewNodes {
	var r reviewNodes
	for _, s := range states {
		r.Nodes = append(r.Nodes, struct {
			State string `json:"state"`
		}{s})
	}
	return r
}

func str(s string) *string { return &s }

func TestPullReview(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		decision *string
		latest   []string
		requests int
		want     string
	}{
		{"decision approved", str("APPROVED"), nil, 0, "approved"},
		{"decision changes", str("CHANGES_REQUESTED"), []string{"APPROVED"}, 0, "changes_requested"},
		{"no rule, latest changes", nil, []string{"APPROVED", "CHANGES_REQUESTED"}, 0, "changes_requested"},
		{"no rule, latest approved", nil, []string{"COMMENTED", "APPROVED"}, 1, "approved"},
		{"no rule, requested", nil, []string{"COMMENTED"}, 1, "pending"},
		{"no rule, nothing", nil, nil, 0, "none"},
		{"required but nobody asked", str("REVIEW_REQUIRED"), nil, 0, "none"},
		{"required and asked", str("REVIEW_REQUIRED"), nil, 2, "pending"},
		{"required, approved by one", str("REVIEW_REQUIRED"), []string{"APPROVED"}, 1, "approved"},
	} {
		n := pullNode{ReviewDecision: c.decision, LatestOpinionatedReviews: reviews(c.latest...)}
		n.ReviewRequests.TotalCount = c.requests
		if got := n.pull().Review; got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestIssueStateReason(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		state  string
		reason *string
		want   string
	}{
		{"OPEN", nil, ""},
		{"OPEN", str("REOPENED"), ""},
		{"CLOSED", str("COMPLETED"), "completed"},
		{"CLOSED", str("NOT_PLANNED"), "not_planned"},
		{"CLOSED", str("DUPLICATE"), "not_planned"},
		{"CLOSED", nil, "completed"},
	} {
		got := issueNode{State: c.state, StateReason: c.reason}.issue()
		if got.StateReason != c.want {
			t.Errorf("%s %v: %q, want %q", c.state, c.reason, got.StateReason, c.want)
		}
	}
}

// answering serves body with the status and headers for every request.
func answering(t *testing.T, status int, headers map[string]string, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(Options{API: srv.URL}, "t", nil)
}

func TestGraphQLPathWithIndex(t *testing.T) {
	t.Parallel()
	c := answering(t, 200, nil, `{"data":{"rateLimit":{"limit":1,"remaining":1,"resetAt":"2026-11-05T12:00:00Z"},"repository":{"p1":null}},
"errors":[{"type":"NOT_FOUND","path":["repository","pullRequests","nodes",0,"x"],"message":"gone"},{"type":"NOT_FOUND","path":["repository","p1"],"message":"gone"}]}`)
	got, _, err := c.LinkedItems("o", "r", []int{1}, nil)
	if err != nil || len(got.MissingPulls) != 1 {
		t.Fatalf("%+v, %v", got, err)
	}
}

func TestGraphQLRateLimitedResetHeader(t *testing.T) {
	t.Parallel()
	c := answering(t, 200, map[string]string{"X-RateLimit-Reset": "1793880000"}, `{"data":null,"errors":[{"type":"RATE_LIMITED","message":"used up"}]}`)
	_, _, err := c.ScanPulls("o", "r", time.Time{})
	var rl *RateLimitError
	if !errors.As(err, &rl) || !rl.Reset.Equal(time.Unix(1793880000, 0)) {
		t.Fatalf("err = %v", err)
	}
	c = answering(t, 200, nil, `{"data":null,"errors":[{"type":"RATE_LIMITED","message":"used up"}]}`)
	_, _, err = c.ScanPulls("o", "r", time.Time{})
	if !errors.As(err, &rl) || time.Until(rl.Reset) < 30*time.Second {
		t.Fatalf("no header: %v", err)
	}
}

func TestGraphQLStatusErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		status  int
		headers map[string]string
		check   func(error) bool
	}{
		{401, nil, func(e error) bool { return errors.Is(e, ErrUnauthorized) }},
		{403, nil, func(e error) bool { return errors.Is(e, ErrForbidden) }},
		{429, map[string]string{"Retry-After": "7"}, func(e error) bool { var r *RateLimitError; return errors.As(e, &r) }},
		{502, nil, func(e error) bool { return e != nil && strings.Contains(e.Error(), "502") }},
	} {
		_, _, err := answering(t, c.status, c.headers, `{"message":"x"}`).ScanPulls("o", "r", time.Time{})
		if !c.check(err) {
			t.Errorf("%d: %v", c.status, err)
		}
	}
}

func TestGraphQLNoData(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"data":null}`, `{}`} {
		if _, _, err := answering(t, 200, nil, body).ScanPulls("o", "r", time.Time{}); err == nil {
			t.Errorf("%s: no error", body)
		}
	}
}

func TestLinkedItemsDeduplicates(t *testing.T) {
	t.Parallel()
	c, reqs := recorded(t, "linked.json")
	if _, _, err := c.LinkedItems("org-a", "web", []int{12, 12, 13}, []int{45, 45}); err != nil {
		t.Fatal(err)
	}
	q, _ := (*reqs)[0]["query"].(string)
	if len(*reqs) != 1 || strings.Count(q, "p12:") != 1 || strings.Count(q, "i45:") != 1 {
		t.Errorf("%d requests:\n%s", len(*reqs), q)
	}
}
```

取り込みの規則（クライアントが決める）:

- `review`: `reviewDecision` が `APPROVED`・`CHANGES_REQUESTED` ならそれ。必須レビューのない repo では `reviewDecision` が null になるので、それ以外のときは `latestOpinionatedReviews` に `CHANGES_REQUESTED` があれば `changes_requested`、なければ `APPROVED` があれば `approved`。どちらもなく `reviewRequests.totalCount > 0` なら `pending`。ほかは `none`（`REVIEW_REQUIRED` で依頼が0件も `none`）
- `state_reason`: open なら空（`REOPENED` も空）。closed は `completed` か `not_planned` だけ。`DUPLICATE` は `not_planned`、理由のない古い closed は `completed`
- NOT_FOUND で path が2つ以上のものは、その要素が null になるだけでエラーにしない。それ以外のエラーは、`data` があっても全体の失敗にする（一部だけの答えを全部と取り違えないため。repo 単位の警告になる）
- `LinkedItems` は番号の重複を1つにする

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./internal/lib/github -run 'ScanPulls|LinkedItems|GraphQL'`
Expected: FAIL（`ScanPulls` が未定義）

- [ ] **Step 4: 実装する**

`graphql.go`:

```go
package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// RateLimit is what GraphQL says of the rate limit.
type RateLimit struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"resetAt"`
}

// Pull is a pull request as Hedgehug keeps it.
type Pull struct {
	Number    int
	Title     string
	Body      string
	Head      string // the branch
	State     string // open, draft, merged, closed
	Review    string // none, pending, approved, changes_requested
	Checks    string // success, failure, pending; empty with no checks
	MergedAt  *time.Time
	UpdatedAt time.Time
	Comments  []string
}

// Issue is an issue as Hedgehug keeps it.
type Issue struct {
	Number      int
	State       string // open, closed
	StateReason string // completed, not_planned; empty when open
	UpdatedAt   time.Time
}

// Linked is the answer of LinkedItems. The numbers GitHub does not know are
// in the Missing lists.
type Linked struct {
	Pulls         []Pull
	Issues        []Issue
	MissingPulls  []int
	MissingIssues []int
}

const pullFields = `number title body state isDraft mergedAt updatedAt headRefName reviewDecision
reviewRequests(first: 1) { totalCount }
latestOpinionatedReviews(first: 100) { nodes { state } }
commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
comments(first: 100) { nodes { body } }`

const issueFields = `number state stateReason updatedAt`

const scanPullsQuery = `query ScanPulls($owner: String!, $name: String!, $after: String) {
rateLimit { limit remaining resetAt }
repository(owner: $owner, name: $name) {
pullRequests(first: 100, after: $after, orderBy: {field: UPDATED_AT, direction: DESC}) {
pageInfo { hasNextPage endCursor }
nodes { ` + pullFields + ` }
}
}
}`

// linkedBatch is how many numbers one query of LinkedItems asks for.
const linkedBatch = 100

type pullNode struct {
	Number         int        `json:"number"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	State          string     `json:"state"`
	IsDraft        bool       `json:"isDraft"`
	MergedAt       *time.Time `json:"mergedAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	HeadRefName    string     `json:"headRefName"`
	ReviewDecision *string    `json:"reviewDecision"`
	ReviewRequests struct {
		TotalCount int `json:"totalCount"`
	} `json:"reviewRequests"`
	LatestOpinionatedReviews struct {
		Nodes []struct {
			State string `json:"state"`
		} `json:"nodes"`
	} `json:"latestOpinionatedReviews"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
	Comments struct {
		Nodes []struct {
			Body string `json:"body"`
		} `json:"nodes"`
	} `json:"comments"`
}

func (n pullNode) pull() Pull {
	p := Pull{Number: n.Number, Title: n.Title, Body: n.Body, Head: n.HeadRefName, MergedAt: n.MergedAt, UpdatedAt: n.UpdatedAt}
	switch {
	case n.State == "MERGED":
		p.State = "merged"
	case n.State == "CLOSED":
		p.State = "closed"
	case n.IsDraft:
		p.State = "draft"
	default:
		p.State = "open"
	}
	p.Review = n.review()
	if len(n.Commits.Nodes) > 0 {
		if r := n.Commits.Nodes[0].Commit.StatusCheckRollup; r != nil {
			switch r.State {
			case "SUCCESS":
				p.Checks = "success"
			case "FAILURE", "ERROR":
				p.Checks = "failure"
			default:
				p.Checks = "pending"
			}
		}
	}
	for _, c := range n.Comments.Nodes {
		p.Comments = append(p.Comments, c.Body)
	}
	return p
}

// review is the state of the review. reviewDecision is null in a repository
// with no required review, so the latest review of each reviewer decides
// then.
func (n pullNode) review() string {
	if d := n.ReviewDecision; d != nil && (*d == "APPROVED" || *d == "CHANGES_REQUESTED") {
		return strings.ToLower(*d)
	}
	approved := false
	for _, r := range n.LatestOpinionatedReviews.Nodes {
		switch r.State {
		case "CHANGES_REQUESTED":
			return "changes_requested"
		case "APPROVED":
			approved = true
		}
	}
	switch {
	case approved:
		return "approved"
	case n.ReviewRequests.TotalCount > 0:
		return "pending"
	}
	return "none"
}

type issueNode struct {
	Number      int       `json:"number"`
	State       string    `json:"state"`
	StateReason *string   `json:"stateReason"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (n issueNode) issue() Issue {
	i := Issue{Number: n.Number, State: strings.ToLower(n.State), UpdatedAt: n.UpdatedAt}
	if i.State != "closed" {
		return i // REOPENED is no reason to keep
	}
	// An issue closed before GitHub kept the reason has none: it was completed.
	i.StateReason = "completed"
	if r := n.StateReason; r != nil && (*r == "NOT_PLANNED" || *r == "DUPLICATE") {
		i.StateReason = "not_planned"
	}
	return i
}

type graphqlError struct {
	Type    string `json:"type"`
	Path    []any  `json:"path"` // names and list indexes
	Message string `json:"message"`
}

// graphqlURL is the GraphQL endpoint of the host: /graphql on github.com,
// /api/graphql on GitHub Enterprise Server, whose REST base ends in /api/v3.
func (c *Client) graphqlURL() string { return strings.TrimSuffix(c.api, "/v3") + "/graphql" }

// graphql runs query and decodes data into out. An error of one item (a
// NOT_FOUND with a path of two or more) leaves that item null in data and
// is no error here.
//
// Any other error fails the whole call even when data came with it: GitHub
// then answers part of the query, and the caller would take the part for the
// whole. The failure is a warning of the repository to the caller.
func (c *Client) graphql(query string, vars map[string]any, out any) error {
	var res struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphqlError  `json:"errors"`
	}
	in := map[string]any{"query": query, "variables": vars}
	h, err := c.do("POST", c.graphqlURL(), in, &res)
	if err != nil {
		return err
	}
	for _, e := range res.Errors {
		switch {
		case e.Type == "NOT_FOUND" && len(e.Path) > 1:
		case e.Type == "NOT_FOUND":
			return fmt.Errorf("%w: %s", ErrNotFound, e.Message)
		case e.Type == "FORBIDDEN":
			return fmt.Errorf("%w: %s", ErrForbidden, e.Message)
		case e.Type == "RATE_LIMITED":
			reset := time.Now().Add(time.Minute)
			if s, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil {
				reset = time.Unix(s, 0)
			}
			return &RateLimitError{Reset: reset}
		default:
			return errors.New("GitHub GraphQL: " + e.Message)
		}
	}
	if len(res.Data) == 0 || string(res.Data) == "null" {
		return errors.New("GitHub GraphQL: the answer has no data")
	}
	return json.Unmarshal(res.Data, out)
}

// ScanPulls lists the pull requests of owner/name updated at since or later,
// the latest first. With a zero since it reads the latest page only.
func (c *Client) ScanPulls(owner, name string, since time.Time) ([]Pull, RateLimit, error) {
	var out []Pull
	var rl RateLimit
	vars := map[string]any{"owner": owner, "name": name}
	for {
		var data struct {
			RateLimit  RateLimit `json:"rateLimit"`
			Repository struct {
				PullRequests struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []pullNode `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		}
		if err := c.graphql(scanPullsQuery, vars, &data); err != nil {
			return nil, rl, err
		}
		rl = data.RateLimit
		page := data.Repository.PullRequests
		for _, n := range page.Nodes {
			if !since.IsZero() && n.UpdatedAt.Before(since) {
				return out, rl, nil
			}
			out = append(out, n.pull())
		}
		if since.IsZero() || !page.PageInfo.HasNextPage {
			return out, rl, nil
		}
		vars["after"] = page.PageInfo.EndCursor
	}
}

// LinkedItems reads the pull requests and the issues of owner/name by number,
// in as many queries as it takes.
func (c *Client) LinkedItems(owner, name string, pulls, issues []int) (Linked, RateLimit, error) {
	var out Linked
	var rl RateLimit
	pulls, issues = dedupe(pulls), dedupe(issues)
	for len(pulls)+len(issues) > 0 {
		n := min(len(pulls), linkedBatch)
		ps := pulls[:n]
		m := min(len(issues), linkedBatch-n)
		is := issues[:m]
		pulls, issues = pulls[n:], issues[m:]
		var data struct {
			RateLimit  RateLimit                  `json:"rateLimit"`
			Repository map[string]json.RawMessage `json:"repository"`
		}
		vars := map[string]any{"owner": owner, "name": name}
		if err := c.graphql(linkedQuery(ps, is), vars, &data); err != nil {
			return out, rl, err
		}
		rl = data.RateLimit
		for _, x := range ps {
			raw := data.Repository[fmt.Sprintf("p%d", x)]
			var node *pullNode
			if err := json.Unmarshal(raw, &node); err != nil && len(raw) > 0 {
				return out, rl, err
			}
			if node == nil {
				out.MissingPulls = append(out.MissingPulls, x)
				continue
			}
			out.Pulls = append(out.Pulls, node.pull())
		}
		for _, x := range is {
			raw := data.Repository[fmt.Sprintf("i%d", x)]
			var node *issueNode
			if err := json.Unmarshal(raw, &node); err != nil && len(raw) > 0 {
				return out, rl, err
			}
			if node == nil {
				out.MissingIssues = append(out.MissingIssues, x)
				continue
			}
			out.Issues = append(out.Issues, node.issue())
		}
	}
	return out, rl, nil
}

// dedupe drops the repeated numbers, keeping the order.
func dedupe(in []int) []int {
	var out []int
	for _, n := range in {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

func linkedQuery(pulls, issues []int) string {
	var b strings.Builder
	b.WriteString("query Linked($owner: String!, $name: String!) {\nrateLimit { limit remaining resetAt }\nrepository(owner: $owner, name: $name) {\n")
	for _, n := range pulls {
		fmt.Fprintf(&b, "p%d: pullRequest(number: %d) { %s }\n", n, n, pullFields)
	}
	for _, n := range issues {
		fmt.Fprintf(&b, "i%d: issue(number: %d) { %s }\n", n, n, issueFields)
	}
	b.WriteString("}\n}")
	return b.String()
}
```

`TestLinkedItemsAskInBatches` は同じ応答を2回返すので、`p1` などは Missing に入る。この Task のテストでは回数だけを見る。

- [ ] **Step 5: 通ることを確かめる**

Run: `go test ./internal/lib/github`
Expected: PASS

- [ ] **Step 6: コミットする**

```bash
git add internal/lib/github
git commit -m "feat(github): PR と Issue を読む GraphQL のクライアントを足す"
```

---

### Task 8: fakegh の PR・Issue・GraphQL

**Files:**
- Create: `internal/lib/fakegh/github.go`、`internal/lib/fakegh/github_test.go`、`internal/lib/fakegh/query.go`、`internal/lib/fakegh/query_test.go`
- Modify: `internal/lib/fakegh/state.go`、`server.go`、`admin.go`
- Modify: `cmd/fakegh/main.go`、`cmd/fakegh/main_test.go`

`lib/github` のテストパッケージは `fakegh` を import している（`client_test.go`）。`fakegh` のテストは循環しないよう外部テストパッケージ `fakegh_test` にする。

- [ ] **Step 1: 失敗するテストを書く**

`github_test.go`:

```go
package fakegh_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/github"
)

func client(t *testing.T) (*fakegh.Server, *github.Client) {
	t.Helper()
	s := fakegh.New(fakegh.State{
		Users:  []string{"alice"},
		Tokens: map[string]string{"t": "alice", "t2": "bob"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return s, github.New(github.Options{API: srv.URL}, "t", nil)
}

func TestPullLifecycle(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	n, err := s.OpenPull("org-a/web", "hh/WEB-1-a", "A", "body", true)
	if err != nil {
		t.Fatal(err)
	}
	pulls, _, err := c.ScanPulls("org-a", "web", time.Time{})
	if err != nil || len(pulls) != 1 || pulls[0].State != "draft" || pulls[0].Review != "none" || pulls[0].Head != "hh/WEB-1-a" {
		t.Fatalf("pulls = %+v, %v", pulls, err)
	}
	for _, f := range []func() error{
		func() error { return s.ReadyPull("org-a/web", n) },
		func() error { return s.RequestReview("org-a/web", n) },
		func() error { return s.SetChecks("org-a/web", n, "success") },
		func() error { return s.CommentPull("org-a/web", n, "see WEB-2") },
	} {
		if err := f(); err != nil {
			t.Fatal(err)
		}
	}
	got, _, err := c.LinkedItems("org-a", "web", []int{n}, nil)
	if err != nil || len(got.Pulls) != 1 {
		t.Fatalf("linked = %+v, %v", got, err)
	}
	p := got.Pulls[0]
	if p.State != "open" || p.Review != "pending" || p.Checks != "success" || len(p.Comments) != 1 {
		t.Errorf("pull = %+v", p)
	}
	if err := s.ReviewPull("org-a/web", n, "approved"); err != nil {
		t.Fatal(err)
	}
	if err := s.MergePull("org-a/web", n); err != nil {
		t.Fatal(err)
	}
	got, _, err = c.LinkedItems("org-a", "web", []int{n, n + 1}, nil)
	if err != nil || got.Pulls[0].State != "merged" || got.Pulls[0].Review != "approved" || got.Pulls[0].MergedAt == nil {
		t.Fatalf("merged = %+v, %v", got, err)
	}
	if len(got.MissingPulls) != 1 {
		t.Errorf("missing = %v", got.MissingPulls)
	}
}

func TestIssueLifecycle(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	n, err := s.OpenIssue("org-a/web", "I", "b")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue("org-a/web", n, "not_planned"); err != nil {
		t.Fatal(err)
	}
	got, _, err := c.LinkedItems("org-a", "web", nil, []int{n})
	if err != nil || len(got.Issues) != 1 || got.Issues[0].State != "closed" || got.Issues[0].StateReason != "not_planned" {
		t.Fatalf("issues = %+v, %v", got, err)
	}
	if err := s.EditIssue("org-a/web", n, "new", "c"); err != nil {
		t.Fatal(err)
	}
}

func TestGraphQLNumbersAndPaging(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	var last int
	for range 101 {
		n, err := s.OpenPull("org-a/web", "b", "t", "", false)
		if err != nil {
			t.Fatal(err)
		}
		last = n
	}
	i, err := s.OpenIssue("org-a/web", "I", "")
	if err != nil || i != last+1 {
		t.Fatalf("issue %d after pull %d, %v", i, last, err)
	}
	all, _, err := c.ScanPulls("org-a", "web", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(all) != 101 {
		t.Fatalf("%d pulls, %v", len(all), err)
	}
}

func TestGraphQLRateLimitAndAccess(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	at := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	s.SetRateLimit(50, 5000, at)
	if _, rl, err := c.ScanPulls("org-a", "web", time.Time{}); err != nil || rl.Remaining != 50 || !rl.ResetAt.Equal(at) {
		t.Errorf("rate limit = %+v, %v", rl, err)
	}
	if _, _, err := c.ScanPulls("org-a", "nothing", time.Time{}); err == nil {
		t.Error("no error for a missing repository")
	}
	if err := s.CloseIssue("org-a/web", 99, "completed"); err == nil {
		t.Error("no error for a missing issue")
	}
}
```

`query_test.go`（クライアントが送る実際のクエリを写し取り、誤記・欠落・知らないフィールドを fakegh が `errors` で返すことを確かめる）:

```go
package fakegh_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// spy records the bodies of the requests that pass.
type spy struct{ bodies []string }

func (s *spy) RoundTrip(r *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(r.Body)
	s.bodies = append(s.bodies, string(b))
	r.Body = io.NopCloser(strings.NewReader(string(b)))
	return http.DefaultTransport.RoundTrip(r)
}

// realQueries are the queries internal/lib/github sends: ScanPulls, then
// LinkedItems.
func realQueries(t *testing.T) (scan, linked string) {
	t.Helper()
	s := fakegh.New(fakegh.State{
		Tokens: map[string]string{"t": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "o", Name: "r", Perms: map[string]string{"alice": "write"}}},
	})
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	var sp spy
	c := github.New(github.Options{API: srv.URL}, "t", &http.Client{Transport: &sp})
	if _, _, err := c.ScanPulls("o", "r", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.LinkedItems("o", "r", []int{1}, []int{2}); err != nil {
		t.Fatal(err)
	}
	pick := func(i int) string {
		var b struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal([]byte(sp.bodies[i]), &b); err != nil {
			t.Fatal(err)
		}
		return b.Query
	}
	return pick(0), pick(1)
}

// ask posts query to the fake and returns the messages of "errors".
func ask(t *testing.T, query string) []string {
	t.Helper()
	s := fakegh.New(fakegh.State{
		Tokens: map[string]string{"t": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "o", Name: "r", Perms: map[string]string{"alice": "write"}}},
	})
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	// The pull request 1 and the issue 2 of the real queries are there.
	if _, err := s.OpenPull("o/r", "b", "t", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenIssue("o/r", "t", ""); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"query": query, "variables": map[string]any{"owner": "o", "name": "r"}})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/graphql", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer t")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var out struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	var msgs []string
	for _, e := range out.Errors {
		msgs = append(msgs, e.Message)
	}
	return msgs
}

func TestGraphQLChecksTheShapeOfTheQuery(t *testing.T) {
	t.Parallel()
	scan, linked := realQueries(t)
	if m := ask(t, scan); m != nil {
		t.Fatalf("scan: %v", m)
	}
	if m := ask(t, linked); m != nil {
		t.Fatalf("linked: %v", m)
	}
	for _, c := range []struct{ name, query, from, to string }{
		{"typo", scan, "reviewDecision", "reviewDecison"},
		{"missing field", scan, "latestOpinionatedReviews(first: 100) { nodes { state } }", ""},
		{"missing nested", scan, "statusCheckRollup { state }", "statusCheckRollup { }"},
		{"unknown field", scan, "headRefName", "headRefName authorAssociation"},
		{"no rateLimit", scan, "rateLimit { limit remaining resetAt }", ""},
		{"wrong order", scan, "direction: DESC", "direction: ASC"},
		{"wrong field order", scan, "field: UPDATED_AT", "field: CREATED_AT"},
		{"page size", scan, "pullRequests(first: 100", "pullRequests(first: 50"},
		{"comments size", scan, "comments(first: 100)", "comments(first: 10)"},
		{"stateReason", linked, "number state stateReason updatedAt", "number state updatedAt"},
		{"issue typo", linked, "stateReason", "stateReasons"},
		{"alias mismatch", linked, "p1: pullRequest(number: 1)", "p1: pullRequest(number: 3)"},
		{"alias kind", linked, "i2: issue(number: 2)", "i2: pullRequest(number: 2)"},
		{"alias form", linked, "p1:", "x1:"},
	} {
		q := strings.Replace(c.query, c.from, c.to, 1)
		if q == c.query {
			t.Fatalf("%s: %q is not in the query:\n%s", c.name, c.from, c.query)
		}
		if m := ask(t, q); len(m) == 0 {
			t.Errorf("%s: the fake accepted\n%s", c.name, q)
		}
	}
	if m := ask(t, "query Other { viewer { login } }"); len(m) == 0 {
		t.Error("unknown query accepted")
	}
}

func TestScanPullsStopsAtOnePageWithoutSince(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	for range 101 {
		if _, err := s.OpenPull("org-a/web", "b", "t", "", false); err != nil {
			t.Fatal(err)
		}
	}
	got, _, err := c.ScanPulls("org-a", "web", time.Time{})
	if err != nil || len(got) != 100 {
		t.Fatalf("%d pulls, %v", len(got), err)
	}
}

func TestReviewsInTheFake(t *testing.T) {
	t.Parallel()
	s, c := client(t)
	n, _ := s.OpenPull("org-a/web", "b", "t", "", false)
	review := func() string {
		got, _, err := c.LinkedItems("org-a", "web", []int{n}, nil)
		if err != nil || len(got.Pulls) != 1 {
			t.Fatalf("%+v, %v", got, err)
		}
		return got.Pulls[0].Review
	}
	if r := review(); r != "none" {
		t.Errorf("fresh: %s", r)
	}
	_ = s.RequestReview("org-a/web", n)
	if r := review(); r != "pending" {
		t.Errorf("requested: %s", r)
	}
	_ = s.ReviewPull("org-a/web", n, "changes_requested")
	if r := review(); r != "changes_requested" {
		t.Errorf("changes: %s", r)
	}
	_ = s.ReviewPull("org-a/web", n, "approved")
	if r := review(); r != "approved" {
		t.Errorf("approved: %s", r)
	}
}
```

`query.go`（クエリを読んで、GitHub のスキーマの形と照らす。`ScanPulls`・`Linked` が要求するフィールドと引数、`orderBy`、`first: 100`、`pN`・`iN` の別名を確かめる）:

```go
package fakegh

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// shape is what a field may have: its arguments (without spaces) and its
// subfields. A field with no subfields is a leaf.
type shape struct {
	args string
	kids map[string]shape
}

// leaves makes the shapes of fields with no arguments and no subfields.
func leaves(names ...string) map[string]shape {
	m := make(map[string]shape, len(names))
	for _, n := range names {
		m[n] = shape{}
	}
	return m
}

func with(m map[string]shape, more map[string]shape) map[string]shape {
	for k, v := range more {
		m[k] = v
	}
	return m
}

// The fields the fake knows of GitHub's schema, which is what the queries of
// internal/lib/github must stay within: a typo or a missing field there is
// an error here, as GitHub would answer. They are written again from the
// client's queries on purpose.
var (
	rateLimitShape = shape{kids: leaves("limit", "remaining", "resetAt")}

	pullShape = shape{kids: with(
		leaves("number", "title", "body", "state", "isDraft", "mergedAt", "updatedAt", "headRefName", "reviewDecision"),
		map[string]shape{
			"reviewRequests":           {args: "first:1", kids: leaves("totalCount")},
			"latestOpinionatedReviews": {args: "first:100", kids: map[string]shape{"nodes": {kids: leaves("state")}}},
			"commits": {args: "last:1", kids: map[string]shape{"nodes": {kids: map[string]shape{
				"commit": {kids: map[string]shape{"statusCheckRollup": {kids: leaves("state")}}},
			}}}},
			"comments": {args: "first:100", kids: map[string]shape{"nodes": {kids: leaves("body")}}},
		})}
	issueShape = shape{kids: leaves("number", "state", "stateReason", "updatedAt")}

	repositoryArgs = "owner:$owner,name:$name"

	scanShape = map[string]shape{
		"rateLimit": rateLimitShape,
		"repository": {args: repositoryArgs, kids: map[string]shape{
			"pullRequests": {
				args: "first:100,after:$after,orderBy:{field:UPDATED_AT,direction:DESC}",
				kids: map[string]shape{
					"pageInfo": {kids: leaves("hasNextPage", "endCursor")},
					"nodes":    pullShape,
				},
			},
		}},
	}
)

// field is a field of a parsed query.
type field struct {
	alias, name, args string
	kids              []*field
}

// parseQuery parses the selection of a query: "query Name(...) { ... }". It
// returns the name of the query and its top-level fields.
func parseQuery(q string) (string, []*field, error) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(q), "query ")
	if !ok {
		return "", nil, errors.New("only a query is answered")
	}
	i := strings.IndexAny(rest, "({ \n")
	if i < 0 {
		return "", nil, errors.New("syntax error: no selection")
	}
	name := rest[:i]
	p := &parser{s: rest[i:]}
	p.skip()
	if p.peek() == '(' { // the variables
		if _, err := p.group(); err != nil {
			return "", nil, err
		}
		p.skip()
	}
	if p.peek() != '{' {
		return "", nil, errors.New("syntax error: want {")
	}
	p.i++
	kids, err := p.set()
	if err != nil {
		return "", nil, err
	}
	p.skip()
	if p.i != len(p.s) {
		return "", nil, errors.New("syntax error: text after the query")
	}
	return name, kids, nil
}

type parser struct {
	s string
	i int
}

func (p *parser) peek() byte {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

func (p *parser) skip() {
	for p.i < len(p.s) && strings.IndexByte(" \t\r\n,", p.s[p.i]) >= 0 {
		p.i++
	}
}

func (p *parser) word() string {
	j := p.i
	for j < len(p.s) && (p.s[j] == '_' || p.s[j] >= '0' && p.s[j] <= '9' || p.s[j] >= 'a' && p.s[j] <= 'z' || p.s[j] >= 'A' && p.s[j] <= 'Z') {
		j++
	}
	w := p.s[p.i:j]
	p.i = j
	return w
}

// group reads (...) and returns what is inside, without spaces.
func (p *parser) group() (string, error) {
	depth, start := 0, p.i+1
	for ; p.i < len(p.s); p.i++ {
		switch p.s[p.i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				in := p.s[start:p.i]
				p.i++
				return strings.Join(strings.Fields(in), ""), nil
			}
		}
	}
	return "", errors.New("syntax error: ( is not closed")
}

// set reads fields up to the closing }.
func (p *parser) set() ([]*field, error) {
	var out []*field
	for {
		p.skip()
		if p.peek() == '}' {
			p.i++
			return out, nil
		}
		f := &field{name: p.word()}
		if f.name == "" {
			return nil, fmt.Errorf("syntax error at %q", p.s[p.i:min(p.i+20, len(p.s))])
		}
		p.skip()
		if p.peek() == ':' {
			p.i++
			p.skip()
			f.alias, f.name = f.name, p.word()
			p.skip()
		}
		if p.peek() == '(' {
			var err error
			if f.args, err = p.group(); err != nil {
				return nil, err
			}
			p.skip()
		}
		if p.peek() == '{' {
			p.i++
			var err error
			if f.kids, err = p.set(); err != nil {
				return nil, err
			}
		}
		out = append(out, f)
		if p.i >= len(p.s) {
			return nil, errors.New("syntax error: } is missing")
		}
	}
}

// check compares fields with what the schema allows and returns the errors
// GitHub would give: fields that do not exist, arguments that differ, and
// fields the query lacks (which the client reads).
func check(fields []*field, want map[string]shape, on string) []string {
	var errs []string
	seen := map[string]bool{}
	for _, f := range fields {
		w, ok := want[f.name]
		if !ok {
			errs = append(errs, fmt.Sprintf("Field '%s' doesn't exist on type '%s'", f.name, on))
			continue
		}
		seen[f.name] = true
		if f.args != w.args {
			errs = append(errs, fmt.Sprintf("Field '%s' on type '%s' is called with the arguments (%s), want (%s)", f.name, on, f.args, w.args))
		}
		if w.kids == nil && len(f.kids) > 0 {
			errs = append(errs, fmt.Sprintf("Field '%s' must not have a selection", f.name))
		}
		if w.kids != nil {
			errs = append(errs, check(f.kids, w.kids, f.name)...)
		}
	}
	for _, name := range sortedKeys(want) {
		if !seen[name] {
			errs = append(errs, fmt.Sprintf("Field '%s' is missing on type '%s'", name, on))
		}
	}
	return errs
}

func sortedKeys(m map[string]shape) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

var linkedAliasName = regexp.MustCompile(`^([pi])(\d+)$`)

// checkLinked checks the query of LinkedItems: the repository holds aliased
// pullRequest(number: n) and issue(number: n), whose alias is p<n> and i<n>.
func checkLinked(top []*field) []string {
	var errs []string
	for _, f := range top {
		switch f.name {
		case "rateLimit":
			errs = append(errs, check([]*field{f}, map[string]shape{"rateLimit": rateLimitShape}, "Query")...)
		case "repository":
			if f.args != repositoryArgs {
				errs = append(errs, "Field 'repository' is called with the arguments ("+f.args+")")
			}
			for _, a := range f.kids {
				errs = append(errs, checkAlias(a)...)
			}
		default:
			errs = append(errs, fmt.Sprintf("Field '%s' doesn't exist on type 'Query'", f.name))
		}
	}
	return errs
}

func checkAlias(a *field) []string {
	m := linkedAliasName.FindStringSubmatch(a.alias)
	if m == nil {
		return []string{fmt.Sprintf("The alias '%s' is not p<number> or i<number>", a.alias)}
	}
	kind, name, sh := "pullRequest", "PullRequest", pullShape
	if m[1] == "i" {
		kind, name, sh = "issue", "Issue", issueShape
	}
	if a.name != kind {
		return []string{fmt.Sprintf("The alias '%s' must be of %s, not %s", a.alias, kind, a.name)}
	}
	if a.args != "number:"+m[2] {
		return []string{fmt.Sprintf("The alias '%s' is called with the arguments (%s), want (number:%s)", a.alias, a.args, m[2])}
	}
	return check(a.kids, sh.kids, name)
}
```

`cmd/fakegh/main_test.go` の既存のテストの書き方に合わせて、`pr open` から `pr merge` までと `issue open`・`close` のコマンドのテストを足す。

```go
func TestPullAndIssueCommands(t *testing.T) {
	t.Parallel()
	s := fakegh.New(fakegh.State{Repos: []*fakegh.Repo{{Owner: "o", Name: "r"}}})
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	for _, args := range [][]string{
		{"pr", "open", "o/r", "--branch", "hh/WEB-1-a", "--title", "A", "--server", srv.URL},
		{"pr", "review", "o/r#1", "--request", "--server", srv.URL},
		{"pr", "review", "o/r#1", "--state", "approved", "--server", srv.URL},
		{"pr", "checks", "o/r#1", "success", "--server", srv.URL},
		{"pr", "comment", "o/r#1", "--body", "see WEB-2", "--server", srv.URL},
		{"pr", "merge", "o/r#1", "--server", srv.URL},
		{"issue", "open", "o/r", "--title", "I", "--server", srv.URL},
		{"issue", "edit", "o/r#2", "--title", "J", "--server", srv.URL},
		{"issue", "close", "o/r#2", "--reason", "not_planned", "--server", srv.URL},
	} {
		var out, errs strings.Builder
		if code := run(args, &out, &errs); code != 0 {
			t.Fatalf("%v: %d %s", args, code, errs.String())
		}
	}
	r, ok := s.Repo("o/r")
	if !ok || r.Pulls[0].State != "merged" || r.Issues[0].State != "closed" || r.Issues[0].Title != "J" {
		t.Errorf("repo = %+v", r)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/lib/fakegh ./cmd/fakegh`
Expected: FAIL（`OpenPull` が未定義）

- [ ] **Step 3: 実装する**

`state.go` の `Repo` に足す。

```go
	Pulls  []*Pull  `json:"pulls,omitempty"`
	Issues []*Issue `json:"issues,omitempty"`
```

型を足す。

```go
// Pull is a pull request.
type Pull struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Head      string     `json:"head"`
	State     string     `json:"state"` // open, closed, merged
	Draft     bool       `json:"draft"`
	Review    string     `json:"review"`    // "", approved, changes_requested
	Requested bool       `json:"requested"` // a review is asked for
	Checks    string     `json:"checks"`    // "", success, failure, pending
	MergedAt  *time.Time `json:"merged_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Comments  []string   `json:"comments"`
}

// Issue is an issue.
type Issue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	State       string    `json:"state"`        // open, closed
	StateReason string    `json:"state_reason"` // completed, not_planned
	UpdatedAt   time.Time `json:"updated_at"`
}
```

`state.go` の import に `"time"` を足す。

`server.go` の `Server` に `rate rateLimit` を足す。`Reset` の `c.Perms` の下に足す。

```go
		c.Pulls, c.Issues = clonePulls(r.Pulls), cloneIssues(r.Issues)
```

`Reset` の末尾に足す。

```go
	s.rate = rateLimit{Limit: 5000, Remaining: 5000, ResetAt: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)}
```

`Repo` の `c.Topics = slices.Clone(r.Topics)` の下に足す。

```go
	c.Pulls, c.Issues = clonePulls(r.Pulls), cloneIssues(r.Issues)
```

`ServeHTTP` の `case r.Method == "GET" && r.URL.Path == "/user":` の前に足す。

```go
	case r.Method == "POST" && r.URL.Path == "/graphql":
		s.graphql(w, r, login)
```

`github.go`（`graphql` はクエリを `query.go` で確かめてから答える。`Server.pull`・`Server.issue` は `server.go` のハンドラと名前が重なるので `changePull`・`changeIssue`）:

```go
package fakegh

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type rateLimit struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"resetAt"`
}

func clonePulls(in []*Pull) []*Pull {
	out := make([]*Pull, len(in))
	for i, p := range in {
		c := *p
		c.Comments = slices.Clone(p.Comments)
		out[i] = &c
	}
	return out
}

func cloneIssues(in []*Issue) []*Issue {
	out := make([]*Issue, len(in))
	for i, x := range in {
		c := *x
		out[i] = &c
	}
	return out
}

// SetRateLimit sets what GraphQL says of the rate limit.
func (s *Server) SetRateLimit(remaining, limit int, reset time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rate = rateLimit{Limit: limit, Remaining: remaining, ResetAt: reset}
}

func nextNumber(r *Repo) int {
	n := 0
	for _, p := range r.Pulls {
		n = max(n, p.Number)
	}
	for _, i := range r.Issues {
		n = max(n, i.Number)
	}
	return n + 1
}

// OpenPull opens a pull request and returns its number. Pull requests and
// issues share their numbers, as on GitHub.
func (s *Server) OpenPull(repo, head, title, body string, draft bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return 0, fmt.Errorf("no repository %s", repo)
	}
	p := &Pull{Number: nextNumber(r), Title: title, Body: body, Head: head, State: "open", Draft: draft, UpdatedAt: s.now()}
	r.Pulls = append(r.Pulls, p)
	return p.Number, nil
}

// changePull changes the pull request number of repo with fn and marks it updated.
func (s *Server) changePull(repo string, number int, fn func(*Pull) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return fmt.Errorf("no repository %s", repo)
	}
	for _, p := range r.Pulls {
		if p.Number == number {
			if err := fn(p); err != nil {
				return err
			}
			p.UpdatedAt = s.now()
			return nil
		}
	}
	return fmt.Errorf("no pull request %s#%d", repo, number)
}

// ReadyPull takes a pull request out of draft.
func (s *Server) ReadyPull(repo string, number int) error {
	return s.changePull(repo, number, func(p *Pull) error { p.Draft = false; return nil })
}

// RequestReview asks for a review.
func (s *Server) RequestReview(repo string, number int) error {
	return s.changePull(repo, number, func(p *Pull) error { p.Requested = true; return nil })
}

// ReviewPull gives the review: approved or changes_requested.
func (s *Server) ReviewPull(repo string, number int, state string) error {
	return s.changePull(repo, number, func(p *Pull) error {
		if state != "approved" && state != "changes_requested" {
			return fmt.Errorf("unknown review %q", state)
		}
		p.Review, p.Requested = state, false
		return nil
	})
}

// SetChecks sets the state of the checks: success, failure or pending.
func (s *Server) SetChecks(repo string, number int, state string) error {
	return s.changePull(repo, number, func(p *Pull) error { p.Checks = state; return nil })
}

// CommentPull adds a comment.
func (s *Server) CommentPull(repo string, number int, body string) error {
	return s.changePull(repo, number, func(p *Pull) error { p.Comments = append(p.Comments, body); return nil })
}

// MergePull merges a pull request.
func (s *Server) MergePull(repo string, number int) error {
	return s.changePull(repo, number, func(p *Pull) error {
		at := s.now()
		p.State, p.Draft, p.MergedAt = "merged", false, &at
		return nil
	})
}

// OpenIssue opens an issue and returns its number.
func (s *Server) OpenIssue(repo, title, body string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return 0, fmt.Errorf("no repository %s", repo)
	}
	i := &Issue{Number: nextNumber(r), Title: title, Body: body, State: "open", UpdatedAt: s.now()}
	r.Issues = append(r.Issues, i)
	return i.Number, nil
}

// changeIssue changes the issue number of repo with fn and marks it updated.
func (s *Server) changeIssue(repo string, number int, fn func(*Issue)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return fmt.Errorf("no repository %s", repo)
	}
	for _, i := range r.Issues {
		if i.Number == number {
			fn(i)
			i.UpdatedAt = s.now()
			return nil
		}
	}
	return fmt.Errorf("no issue %s#%d", repo, number)
}

// CloseIssue closes an issue: completed or not_planned.
func (s *Server) CloseIssue(repo string, number int, reason string) error {
	return s.changeIssue(repo, number, func(i *Issue) { i.State, i.StateReason = "closed", reason })
}

// EditIssue changes the title and the body of an issue; an empty one is left
// as it is.
func (s *Server) EditIssue(repo string, number int, title, body string) error {
	return s.changeIssue(repo, number, func(i *Issue) {
		if title != "" {
			i.Title = title
		}
		if body != "" {
			i.Body = body
		}
	})
}

func pullJSON(p *Pull) map[string]any {
	state := strings.ToUpper(p.State)
	var decision any
	if p.Review != "" {
		decision = strings.ToUpper(p.Review)
	}
	requests := 0
	if p.Requested {
		requests = 1
	}
	latest := []map[string]string{}
	if p.Review != "" {
		latest = append(latest, map[string]string{"state": strings.ToUpper(p.Review)})
	}
	var rollup any
	if p.Checks != "" {
		rollup = map[string]string{"state": strings.ToUpper(p.Checks)}
	}
	comments := make([]map[string]string, len(p.Comments))
	for i, c := range p.Comments {
		comments[i] = map[string]string{"body": c}
	}
	return map[string]any{
		"number": p.Number, "title": p.Title, "body": p.Body, "state": state, "isDraft": p.Draft,
		"mergedAt": p.MergedAt, "updatedAt": p.UpdatedAt, "headRefName": p.Head, "reviewDecision": decision,
		"reviewRequests":           map[string]int{"totalCount": requests},
		"latestOpinionatedReviews": map[string]any{"nodes": latest},
		"commits":                  map[string]any{"nodes": []any{map[string]any{"commit": map[string]any{"statusCheckRollup": rollup}}}},
		"comments":                 map[string]any{"nodes": comments},
	}
}

func issueJSON(i *Issue) map[string]any {
	var reason any
	if i.StateReason != "" {
		reason = strings.ToUpper(i.StateReason)
	}
	return map[string]any{"number": i.Number, "state": strings.ToUpper(i.State), "stateReason": reason, "updatedAt": i.UpdatedAt}
}

// graphql answers the two queries of internal/lib/github: ScanPulls and
// Linked.
func (s *Server) graphql(w http.ResponseWriter, r *http.Request, login string) {
	var body struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	owner, _ := body.Variables["owner"].(string)
	name, _ := body.Variables["name"].(string)
	x := s.find(owner + "/" + name)
	if x == nil || perm(x, login) == "" {
		writeJSON(w, 200, map[string]any{
			"data":   map[string]any{"rateLimit": s.rate, "repository": nil},
			"errors": []map[string]any{{"type": "NOT_FOUND", "path": []string{"repository"}, "message": "Could not resolve to a Repository"}},
		})
		return
	}
	kind, top, err := parseQuery(body.Query)
	var problems []string
	switch {
	case err != nil:
		problems = []string{err.Error()}
	case kind == "ScanPulls":
		problems = check(top, scanShape, "Query")
	case kind == "Linked":
		problems = checkLinked(top)
	default:
		problems = []string{"unknown query " + kind}
	}
	if len(problems) > 0 {
		errs := make([]map[string]any, len(problems))
		for i, m := range problems {
			errs[i] = map[string]any{"message": m}
		}
		writeJSON(w, 200, map[string]any{"errors": errs})
		return
	}
	if kind == "ScanPulls" {
		after, _ := body.Variables["after"].(string)
		s.scanPulls(w, x, after)
		return
	}
	s.linked(w, x, top)
}

func (s *Server) scanPulls(w http.ResponseWriter, x *Repo, after string) {
	pulls := slices.Clone(x.Pulls)
	slices.SortStableFunc(pulls, func(a, b *Pull) int {
		if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
			return c
		}
		return cmp.Compare(b.Number, a.Number)
	})
	from, _ := strconv.Atoi(after)
	to := min(from+100, len(pulls))
	nodes := make([]map[string]any, 0, to-from)
	for _, p := range pulls[from:to] {
		nodes = append(nodes, pullJSON(p))
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{
		"rateLimit": s.rate,
		"repository": map[string]any{"pullRequests": map[string]any{
			"pageInfo": map[string]any{"hasNextPage": to < len(pulls), "endCursor": strconv.Itoa(to)},
			"nodes":    nodes,
		}},
	}})
}

// linked answers the query that checkLinked passed.
func (s *Server) linked(w http.ResponseWriter, x *Repo, top []*field) {
	repo := map[string]any{}
	var errs []map[string]any
	var aliases []*field
	for _, f := range top {
		if f.name == "repository" {
			aliases = f.kids
		}
	}
	for _, a := range aliases {
		n, _ := strconv.Atoi(strings.TrimPrefix(a.args, "number:"))
		var node map[string]any
		if a.name == "pullRequest" {
			if i := slices.IndexFunc(x.Pulls, func(p *Pull) bool { return p.Number == n }); i >= 0 {
				node = pullJSON(x.Pulls[i])
			}
		} else if i := slices.IndexFunc(x.Issues, func(p *Issue) bool { return p.Number == n }); i >= 0 {
			node = issueJSON(x.Issues[i])
		}
		if node == nil {
			repo[a.alias] = nil
			errs = append(errs, map[string]any{"type": "NOT_FOUND", "path": []string{"repository", a.alias}, "message": "Could not resolve"})
			continue
		}
		repo[a.alias] = node
	}
	out := map[string]any{"data": map[string]any{"rateLimit": s.rate, "repository": repo}}
	if errs != nil {
		out["errors"] = errs
	}
	writeJSON(w, 200, out)
}
```

`admin.go`: `adminBody` に `Number int`、`Head`、`Title`、`Body`、`Draft bool`、`State`、`Reason`、`Request bool` を足し、`admin` の switch に足す（`s.mu` を取らない。各メソッドが取る）。

```go
	case "/_fakegh/pr/open":
		_, err = s.OpenPull(b.Repo, b.Head, b.Title, b.Body, b.Draft)
	case "/_fakegh/pr/merge":
		err = s.MergePull(b.Repo, b.Number)
	case "/_fakegh/pr/review":
		if b.Request {
			err = s.RequestReview(b.Repo, b.Number)
		} else {
			err = s.ReviewPull(b.Repo, b.Number, b.State)
		}
	case "/_fakegh/pr/checks":
		err = s.SetChecks(b.Repo, b.Number, b.State)
	case "/_fakegh/pr/comment":
		err = s.CommentPull(b.Repo, b.Number, b.Body)
	case "/_fakegh/issue/open":
		_, err = s.OpenIssue(b.Repo, b.Title, b.Body)
	case "/_fakegh/issue/close":
		err = s.CloseIssue(b.Repo, b.Number, b.Reason)
	case "/_fakegh/issue/edit":
		err = s.EditIssue(b.Repo, b.Number, b.Title, b.Body)
```

`Admin` に足す。

```go
// Pull calls /_fakegh/pr/<what> with b; Issue calls /_fakegh/issue/<what>.
func (a Admin) Pull(what string, b AdminBody) error  { return a.post("/_fakegh/pr/"+what, b) }
func (a Admin) Issue(what string, b AdminBody) error { return a.post("/_fakegh/issue/"+what, b) }
```

`adminBody` を公開の `AdminBody` に改名し、`admin.go` の使用箇所を置き換える。

`cmd/fakegh/main.go`: `usage` に足す。

```
  fakegh pr open <owner/repo> --branch <b> [--title t] [--body b] [--draft]
  fakegh pr review <owner/repo#n> (--request | --state approved|changes_requested)
  fakegh pr checks <owner/repo#n> <success|failure|pending>
  fakegh pr comment <owner/repo#n> --body <b>
  fakegh pr merge <owner/repo#n>
  fakegh issue open <owner/repo> --title <t> [--body b]
  fakegh issue edit <owner/repo#n> [--title t] [--body b]
  fakegh issue close <owner/repo#n> [--reason completed|not_planned]
```

`dispatch` の switch に足す。`repoNumber` は `owner/repo#n` を分ける。

```go
	case "pr open":
		fs := flagSet("fakegh pr open")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		branch := fs.String("branch", "", "the branch")
		title := fs.String("title", "", "the title")
		body := fs.String("body", "", "the body")
		draft := fs.Bool("draft", false, "open it as a draft")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		if *branch == "" {
			return usageError{errors.New("--branch is required")}
		}
		return admin(*server).Pull("open", fakegh.AdminBody{Repo: pos[0], Head: *branch, Title: *title, Body: *body, Draft: *draft})
	case "pr review":
		fs := flagSet("fakegh pr review")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		request := fs.Bool("request", false, "ask for a review")
		state := fs.String("state", "", "approved or changes_requested")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Pull("review", fakegh.AdminBody{Repo: repo, Number: n, Request: *request, State: *state})
	case "pr checks":
		fs := flagSet("fakegh pr checks")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		pos, err := parse(fs, rest, 2)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Pull("checks", fakegh.AdminBody{Repo: repo, Number: n, State: pos[1]})
	case "pr comment":
		fs := flagSet("fakegh pr comment")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		body := fs.String("body", "", "the comment")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Pull("comment", fakegh.AdminBody{Repo: repo, Number: n, Body: *body})
	case "pr merge":
		fs := flagSet("fakegh pr merge")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Pull("merge", fakegh.AdminBody{Repo: repo, Number: n})
	case "issue open":
		fs := flagSet("fakegh issue open")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		title := fs.String("title", "", "the title")
		body := fs.String("body", "", "the body")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		return admin(*server).Issue("open", fakegh.AdminBody{Repo: pos[0], Title: *title, Body: *body})
	case "issue edit":
		fs := flagSet("fakegh issue edit")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		title := fs.String("title", "", "the title")
		body := fs.String("body", "", "the body")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Issue("edit", fakegh.AdminBody{Repo: repo, Number: n, Title: *title, Body: *body})
	case "issue close":
		fs := flagSet("fakegh issue close")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		reason := fs.String("reason", "completed", "completed or not_planned")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Issue("close", fakegh.AdminBody{Repo: repo, Number: n, Reason: *reason})
```

`main.go` に足し、import に `"strconv"` を足す。

```go
// repoNumber splits owner/repo#n.
func repoNumber(s string) (string, int, error) {
	repo, num, ok := strings.Cut(s, "#")
	n, err := strconv.Atoi(num)
	if !ok || err != nil {
		return "", 0, usageError{fmt.Errorf("want owner/repo#n, got %q", s)}
	}
	return repo, n, nil
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/lib/... ./cmd/...`
Expected: PASS

- [ ] **Step 5: コミットする**

```bash
git add internal/lib/fakegh cmd/fakegh
git commit -m "feat(fakegh): PR・Issue・GraphQL と操作のコマンドを足す"
```

---

### Task 9: `repo_accounts` でアカウントを選ぶ

**Files:**
- Modify: `internal/app/storage/config/hosts.go`、`internal/app/usecase/conn.go`
- Test: `internal/app/storage/config/hosts_test.go`、`internal/app/usecase/conn_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`hosts_test.go`:

```go
func TestRepoAccountFor(t *testing.T) {
	t.Parallel()
	c := &Config{RepoAccounts: []RepoAccount{
		{Match: "github.com/org-a/*", Account: "work"},
		{Match: "*", Account: "personal"},
	}}
	for id, want := range map[string]string{
		"github.com/org-a/web": "work",
		"github.com/ORG-A/Web": "work",
		"github.com/org-b/ops": "personal",
	} {
		if got, ok := c.RepoAccountFor(id); !ok || got != want {
			t.Errorf("%s = %q, %v", id, got, ok)
		}
	}
	if _, ok := (&Config{}).RepoAccountFor("github.com/o/r"); ok {
		t.Error("no rules match")
	}
}
```

`conn_test.go`（`package usecase_test` の書き方に合わせる）:

```go
func TestGitHubForRepoFollowsRepoAccounts(t *testing.T) {
	t.Parallel()
	ts, _ := usecasetest.FakeSession(t, fakegh.State{Tokens: map[string]string{"t-work": "bob", "t-alice": "alice"}}, "")
	ts.Env["T_WORK"] = "t-work"
	ts.AppendConfig("accounts:\n" +
		"  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"  - {name: work, host: github.com, login: bob, token: \"env:T_WORK\"}\n" +
		"repo_accounts:\n  - {match: github.com/org-a/*, account: work}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	e := config.StoreEntry{Account: "me"}
	_, acct, err := ts.GitHubForRepo(c, e, "github.com/org-a/web")
	if err != nil || acct.Name != "work" {
		t.Fatalf("org-a = %+v, %v", acct, err)
	}
	_, acct, err = ts.GitHubForRepo(c, e, "github.com/org-b/ops")
	if err != nil || acct.Name != "me" {
		t.Fatalf("the store's account = %+v, %v", acct, err)
	}
	// An account of another host never gets the token sent there.
	for _, e := range []config.StoreEntry{{Account: "me"}, {}} {
		cl, _, err := ts.GitHubForRepo(c, e, "ghe.example.com/org-b/ops")
		if cl != nil || err == nil || !strings.Contains(err.Error(), "ghe.example.com/org-b/ops") {
			t.Errorf("other host (store account %q) = %v, %v", e.Account, cl, err)
		}
	}
}

func TestGitHubForRepoRejectsOtherHost(t *testing.T) {
	t.Parallel()
	ts, _ := usecasetest.FakeSession(t, fakegh.State{}, "")
	ts.AppendConfig("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"repo_accounts:\n  - {match: \"*\", account: me}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	cl, _, err := ts.GitHubForRepo(c, config.StoreEntry{}, "ghe.example.com/o/r")
	if cl != nil || err == nil || !strings.Contains(err.Error(), "me") || !strings.Contains(err.Error(), "ghe.example.com/o/r") {
		t.Errorf("repo_accounts, other host = %v, %v", cl, err)
	}
	cl, _, err = ts.GitHubForRepo(c, config.StoreEntry{}, "github.com/o/r")
	if cl == nil || err != nil {
		t.Errorf("same host = %v, %v", cl, err)
	}
}
```

`ts.Entry()` はない。`Store` の account は `config.StoreEntry{Account: ...}` で渡す。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/storage/config ./internal/app/usecase -run 'RepoAccountFor|GitHubForRepo'`
Expected: FAIL（未定義）

- [ ] **Step 3: 実装する**

`hosts.go` に足す（import に `"path"` を足す）。`RepoAccount` のコメントの `Phase 5 uses it.` を消す。

```go
// RepoAccountFor returns the account of the first rule that matches the
// repo ID, without case; "*" matches every ID.
func (c *Config) RepoAccountFor(id string) (string, bool) {
	id = strings.ToLower(id)
	for _, r := range c.RepoAccounts {
		if r.Match == "*" {
			return r.Account, true
		}
		if ok, err := path.Match(strings.ToLower(r.Match), id); err == nil && ok {
			return r.Account, true
		}
	}
	return "", false
}
```

`conn.go` に足す（import に `"github.com/kukv/hedgehug/internal/lib/giturl"` は不要。repo ID は `<host>/<owner>/<repo>` の形）。

```go
// GitHubForRepo is a client for the repository id (<host>/<owner>/<repo>),
// with the account repo_accounts picks, else the account of the store e,
// else the only account. It returns the account too, for the messages.
func (ss *Session) GitHubForRepo(c *config.Config, e config.StoreEntry, id string) (*github.Client, config.Account, error) {
	host, _, _ := strings.Cut(id, "/")
	name, ok := c.RepoAccountFor(id)
	if !ok {
		name = e.Account
	}
	acct, ok, err := ss.PickAccount(c, name)
	if err != nil {
		return nil, config.Account{}, err
	}
	if !ok {
		return nil, config.Account{}, NewErr(KindConflict, "account.none",
			fmt.Errorf("no account can read %s; set repo_accounts in config.yaml", id), "repo", id)
	}
	if !config.SameHost(acct.Host, host) {
		return nil, config.Account{}, NewErr(KindInvalid, "account.host",
			fmt.Errorf("account %s is of %s, so it cannot read %s; set repo_accounts in config.yaml", acct.Name, acct.Host, id),
			"repo", id, "account", acct.Name)
	}
	cl, err := ss.GitHub(c, acct, host)
	return cl, acct, err
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/storage/config ./internal/app/usecase`
Expected: PASS

- [ ] **Step 5: コミットする**

```bash
git add internal/app/storage/config internal/app/usecase
git commit -m "feat(conn): repo_accounts で GitHub のアカウントを選ぶ"
```

---

### Task 10: 取得時刻を手元に持つ `storage/ghstate`

**Files:**
- Create: `internal/app/storage/ghstate/ghstate.go`、`internal/app/storage/ghstate/ghstate_test.go`

- [ ] **Step 1: 失敗するテストを書く**

```go
package ghstate

import (
	"testing"
	"time"
)

func TestDue(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	for name, c := range map[string]struct {
		r    Repo
		want bool
	}{
		"never":      {Repo{}, true},
		"too soon":   {Repo{LastFetch: now.Add(-4 * time.Minute)}, false},
		"five":       {Repo{LastFetch: now.Add(-5 * time.Minute)}, true},
		"low, soon":  {Repo{LastFetch: now.Add(-14 * time.Minute), Limit: 5000, Remaining: 499, ResetAt: now.Add(time.Hour)}, false},
		"low, late":  {Repo{LastFetch: now.Add(-15 * time.Minute), Limit: 5000, Remaining: 499, ResetAt: now.Add(time.Hour)}, true},
		"low, reset": {Repo{LastFetch: now.Add(-5 * time.Minute), Limit: 5000, Remaining: 499, ResetAt: now.Add(-time.Minute)}, true},
		"held back":  {Repo{NotBefore: now.Add(time.Minute)}, false},
		"hold over":  {Repo{NotBefore: now.Add(-time.Minute)}, true},

		"exactly 10%":     {Repo{LastFetch: now.Add(-6 * time.Minute), Limit: 5000, Remaining: 500, ResetAt: now.Add(time.Hour)}, true},
		"no limit known":  {Repo{LastFetch: now.Add(-6 * time.Minute), Remaining: 0, ResetAt: now.Add(time.Hour)}, true},
		"clock went back": {Repo{LastFetch: now.Add(time.Minute)}, true},
		"hold too far":    {Repo{NotBefore: now.Add(2 * time.Hour)}, true},
		"reset too far":   {Repo{LastFetch: now.Add(-6 * time.Minute), Limit: 5000, Remaining: 1, ResetAt: now.Add(2 * time.Hour)}, true},
		"hold at the cap": {Repo{NotBefore: now.Add(time.Hour)}, false},

		"failed, soon":       {Repo{LastFetch: now.Add(-time.Hour), LastTry: now.Add(-4 * time.Minute)}, false},
		"failed, late":       {Repo{LastFetch: now.Add(-time.Hour), LastTry: now.Add(-5 * time.Minute)}, true},
		"failed first, soon": {Repo{LastTry: now.Add(-time.Minute)}, false},
		"fetched after":      {Repo{LastFetch: now.Add(-time.Minute), LastTry: now.Add(-time.Hour)}, false},
		"try went forward":   {Repo{LastTry: now.Add(time.Minute)}, true},
		"failed, low, soon":  {Repo{LastTry: now.Add(-14 * time.Minute), Limit: 5000, Remaining: 1, ResetAt: now.Add(time.Hour)}, false},
	} {
		if got := c.r.Due(now); got != c.want {
			t.Errorf("%s = %v", name, got)
		}
	}
}
```

読み書きのテストを足す（壊れたファイルは空の状態になる `TestReadBrokenFileIsEmpty` と、`TestMain(m) { testenv.Main(m) }` も要る）。`newStore` は `query/helpers_test.go` の `store.Create` の呼び方を写す。

```go
func TestReadWrite(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	st, err := Read(s)
	if err != nil || len(st.Repos) != 0 {
		t.Fatalf("empty = %+v, %v", st, err)
	}
	at := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	st.Repos["github.com/o/r"] = Repo{LastFetch: at, Limit: 5000, Remaining: 1}
	if err := Write(s, st); err != nil {
		t.Fatal(err)
	}
	got, err := Read(s)
	if err != nil || !got.Repos["github.com/o/r"].LastFetch.Equal(at) {
		t.Errorf("read = %+v, %v", got, err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/storage/ghstate`
Expected: FAIL（パッケージがない）

- [ ] **Step 3: 実装する**

`LastFetch` は間隔の判定だけに使う。走査の起点は GitHub の時計で測った `Since`（走査で見た最大の `UpdatedAt`）で、手元の時計がずれても取りこぼさない。失敗した試行は `LastTry` と `Warning` に残し、`Due` は `LastFetch` と `LastTry` の新しいほうから間隔を数える。

```go
// Package ghstate keeps, on this machine, when each repository was last read
// from GitHub and how much of the rate limit is left. It is a cache: losing
// it reads the repositories again.
package ghstate

import (
	"encoding/json"
	"errors"
	"io/fs"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/store"
)

const (
	fileName = "github.json"

	// Interval is how often a repository is read.
	Interval = 5 * time.Minute
	// slowFactor stretches Interval while the rate limit runs low.
	slowFactor = 3
	// maxAhead is how far ahead NotBefore and ResetAt are believed.
	maxAhead = time.Hour
)

// Repo is what is known of one repository.
type Repo struct {
	// LastFetch is when the repository was last read, by the clock of this
	// machine. It tells the interval only.
	LastFetch time.Time `json:"last_fetch"`
	// Since is the latest UpdatedAt GitHub reported in a scan: the next scan
	// reads what was updated at or after it. It is GitHub's clock, not ours.
	Since time.Time `json:"since,omitzero"`
	// LastTry and Warning are the last read that failed: it is not tried
	// again within the interval, and Warning is told again meanwhile. A read
	// that succeeds clears them; Warning may then hold what the read found
	// (an item that is not on GitHub), told until the next read.
	LastTry   time.Time `json:"last_try,omitzero"`
	Warning   string    `json:"warning,omitempty"` // one warning per line
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"reset_at"`
	NotBefore time.Time `json:"not_before,omitzero"` // GitHub refused: the rate limit is used up until then
}

// State is the file, by repo ID.
type State struct {
	Repos map[string]Repo `json:"repos"`
}

// Due reports whether the repository is to be read at now. The interval
// counts from the later of LastFetch and LastTry. Under 10% of the
// rate limit the interval is three times as long, until the limit resets.
// The file is only a cache, so a clock that went back, or a time further
// ahead than maxAhead, is not trusted: the repository is read.
func (r Repo) Due(now time.Time) bool {
	if now.Before(r.NotBefore) && !r.NotBefore.After(now.Add(maxAhead)) {
		return false
	}
	last := r.LastFetch
	if r.LastTry.After(last) {
		last = r.LastTry
	}
	if last.IsZero() || last.After(now) {
		return true
	}
	wait := Interval
	if r.Limit > 0 && r.Remaining*10 < r.Limit && now.Before(r.ResetAt) && !r.ResetAt.After(now.Add(maxAhead)) {
		wait *= slowFactor
	}
	return !now.Before(last.Add(wait))
}

// Read reads the file of s. A missing or broken file is an empty state.
func Read(s *store.Store) (State, error) {
	st := State{Repos: map[string]Repo{}}
	b, err := s.ReadLocal(fileName)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if json.Unmarshal(b, &st) != nil || st.Repos == nil {
		return State{Repos: map[string]Repo{}}, nil
	}
	return st, nil
}

// Write replaces the file of s.
func Write(s *store.Store, st State) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.WriteLocal(fileName, b)
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/storage/ghstate`
Expected: PASS

- [ ] **Step 5: コミットする**

```bash
git add internal/app/storage/ghstate
git commit -m "feat(ghstate): repo ごとの取得時刻とレート制限を手元に持つ"
```

---

### Task 11: `hh link`・`hh unlink`

**Files:**
- Create: `internal/app/usecase/integration/link.go`、`internal/app/usecase/integration/link_test.go`、`internal/app/usecase/integration/testmain_test.go`（他の領域と同じ `testenv.Main`）
- Create: `internal/app/cli/link.go`、`internal/app/cli/link_test.go`
- Modify: `internal/app/cli/app.go:72`、`internal/app/cli/readonly_test.go`（閲覧のみの Store で拒否するコマンドの一覧に `link`・`unlink` を足す）

- [ ] **Step 1: 失敗するテストを書く**

`link_test.go`（usecase）:

```go
package integration_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// linksOf is the links of the task id names, in the store ss works on.
func linksOf(t *testing.T, ss *usecase.Session, id string) []model.Link {
	t.Helper()
	s, ix, err := ss.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, err := ss.Resolve(ix, id)
	if err != nil {
		t.Fatal(err)
	}
	x, err := s.Task(e.ULID)
	if err != nil {
		t.Fatal(err)
	}
	return x.Links
}

func TestLinkAndUnlink(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	r, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "Org-A/Web#123"})
	if err != nil || !r.Committed {
		t.Fatalf("link = %+v, %v", r, err)
	}
	if op := usecasetest.LastOp(t, ts); op != "link" {
		t.Errorf("op = %s", op)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 || l[0].Ref != "github.com/org-a/web/pull/123" || l[0].Sync != model.SyncMirror || l[0].Type != model.LinkPR {
		t.Fatalf("links = %+v", l)
	}
	head := ts.Head()
	_, err = integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#123"})
	usecasetest.MustCode(t, err, usecase.KindInvalid, "link.exists")
	if ts.Head() != head {
		t.Error("linking twice committed")
	}
	if _, err := integration.Unlink(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#123"}); err != nil {
		t.Fatal(err)
	}
	if op := usecasetest.LastOp(t, ts); op != "unlink" {
		t.Errorf("op = %s", op)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 || l[0].Sync != model.SyncOff {
		t.Fatalf("after unlink = %+v", l)
	}
	head = ts.Head()
	_, err = integration.Unlink(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#123"})
	usecasetest.MustCode(t, err, usecase.KindInvalid, "link.not_found")
	if ts.Head() != head {
		t.Error("unlinking twice committed")
	}
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#123"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 || l[0].Sync != model.SyncMirror {
		t.Fatalf("linked again = %+v", l)
	}
}

func TestLinkIssueModes(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "o/r#45"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); l[0].Type != model.LinkIssue || l[0].Sync != model.SyncLink || l[0].Ref != "github.com/o/r/issues/45" {
		t.Fatalf("links = %+v", l)
	}
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "o/r#45", Sync: "push"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 || l[0].Sync != model.SyncPush {
		t.Fatalf("mode = %+v", l)
	}
}

func TestLinkInputs(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	for name, in := range map[string]integration.LinkInput{
		"nothing":  {},
		"both":     {PR: "o/r#1", Issue: "o/r#2"},
		"bad":      {PR: "web#1"},
		"pr sync":  {PR: "o/r#1", Sync: "push"},
		"off":      {Issue: "o/r#1", Sync: "off"},
		"bad sync": {Issue: "o/r#1", Sync: "both"},
	} {
		_, err := integration.Link(ts.Session, "ME-1", in)
		t.Log(name)
		usecasetest.MustCode(t, err, usecase.KindUsage, "link.invalid")
	}
	// A host may be written.
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "ghe.example/o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); l[0].Ref != "ghe.example/o/r/pull/1" {
		t.Errorf("links = %+v", l)
	}
}

func TestLinkInAnotherStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	// WEB-1 is in org-b, whose account is bob; the host is bob's.
	if _, err := integration.Link(ts.Session, "org-b:WEB-1", integration.LinkInput{PR: "o/r#7"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.ForStore("org-b"), "WEB-1"); len(l) != 1 || l[0].Ref != "github.com/o/r/pull/7" {
		t.Errorf("org-b links = %+v", l)
	}
	if l := linksOf(t, ts.ForStore("org-a"), "API-1"); len(l) != 0 {
		t.Errorf("org-a links = %+v", l)
	}
	// Without the store name the id is looked up in every store.
	if _, err := integration.Unlink(ts.Session, "API-1", integration.LinkInput{PR: "o/r#7"}); err == nil {
		t.Error("unlinked what API-1 never had")
	}
	if _, err := integration.Unlink(ts.Session, "WEB-1", integration.LinkInput{PR: "o/r#7"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.ForStore("org-b"), "WEB-1"); len(l) != 1 || l[0].Sync != model.SyncOff {
		t.Errorf("org-b links = %+v", l)
	}
}

func TestLinkRefusesDeletedTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	ts.DeleteTask("ME-1")
	head := ts.Head()
	_, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "o/r#1"})
	usecasetest.MustCode(t, err, usecase.KindConflict, "task.deleted")
	if ts.Head() != head {
		t.Error("linking a deleted task committed")
	}
}

func TestLinkByFormerID(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", func(x *model.Task) { x.FormerIDs = []string{"OLD-7"} })
	if _, err := integration.Link(ts.Session, "OLD-7", integration.LinkInput{PR: "o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 {
		t.Errorf("links = %+v", l)
	}
}

func TestUndoTakesBackLink(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 0 {
		t.Errorf("links after undo = %+v", l)
	}
	// An unlink is taken back as well: the link is live again.
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := integration.Unlink(ts.Session, "ME-1", integration.LinkInput{PR: "o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 || l[0].Sync != model.SyncMirror {
		t.Errorf("links after undoing unlink = %+v", l)
	}
}
```

`link_test.go`（cli）:

```go
package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

func TestLinkCommands(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	if out := te.ok("link", a.ID, "--pr", "org-a/web#1"); !strings.Contains(out, "Linked "+a.ID) {
		t.Errorf("link: %s", out)
	}
	if out := te.ok("unlink", a.ID, "--pr", "org-a/web#1"); !strings.Contains(out, "Unlinked "+a.ID) {
		t.Errorf("unlink: %s", out)
	}
	if code, _, _ := te.run("link", a.ID); code != 2 {
		t.Errorf("no target: %d", code)
	}
	if code, _, _ := te.run("link", a.ID, "--pr", "org-a/web#1", "--issue", "org-a/web#2"); code != 2 {
		t.Errorf("both: %d", code)
	}
}

func TestLinkJSON(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	var got usecase.TaskWrite
	if err := json.Unmarshal([]byte(te.ok("link", a.ID, "--issue", "org-a/web#2", "--sync", "push", "--json")), &got); err != nil || got.Task == nil || got.Commit == nil {
		t.Fatalf("link --json = %+v, %v", got, err)
	}
	if l := te.task(a.ULID).Links; len(l) != 1 || l[0].Sync != "push" || l[0].Ref != "github.com/org-a/web/issues/2" {
		t.Errorf("links = %+v", l)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/usecase/integration ./internal/app/cli -run 'Link'`
Expected: FAIL（パッケージがない）

- [ ] **Step 3: 実装する**

`integration/link.go`:

```go
// Package integration ties tasks to pull requests and issues on GitHub:
// the links, and reading GitHub's values into gh.
package integration

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// defaultIssueSync is the mode of a new link to an issue. Phase 5c reads it
// from issue_fields.default_sync of automations.yaml.
const defaultIssueSync = model.SyncLink

// LinkInput names what to link: PR or Issue, as [<host>/]owner/repo#n, and
// for an issue how it syncs.
type LinkInput struct {
	PR, Issue string
	Sync      string
}

var written = regexp.MustCompile(`^(?:([^/#]+)/)?([^/#]+)/([^/#]+)#([1-9][0-9]*)$`)

// ref reads in into a ref. host is the writer's, for a ref without one.
func (in LinkInput) ref(host string) (model.GitHubRef, error) {
	invalid := func(err error) (model.GitHubRef, error) {
		return model.GitHubRef{}, usecase.NewErr(usecase.KindUsage, "link.invalid", err)
	}
	s, kind := in.PR, model.LinkPR
	switch {
	case in.PR == "" && in.Issue == "":
		return invalid(errors.New("say what to link: --pr or --issue"))
	case in.PR != "" && in.Issue != "":
		return invalid(errors.New("say one of --pr and --issue"))
	case in.Issue != "":
		s, kind = in.Issue, model.LinkIssue
	}
	m := written.FindStringSubmatch(s)
	if m == nil {
		return invalid(fmt.Errorf("%q: want owner/repo#number", s))
	}
	if m[1] != "" {
		host = m[1]
	}
	if host == "" {
		host = "github.com"
	}
	n, err := strconv.Atoi(m[4])
	if err != nil {
		return invalid(fmt.Errorf("%q: %w", s, err))
	}
	return model.GitHubRef{Repo: strings.ToLower(host + "/" + m[2] + "/" + m[3]), Kind: kind, Number: n}, nil
}

func (in LinkInput) sync() (string, error) {
	invalid := func(err error) (string, error) { return "", usecase.NewErr(usecase.KindUsage, "link.invalid", err) }
	switch {
	case in.PR != "" && in.Sync != "":
		return invalid(errors.New("--sync is for an issue: GitHub is always the source of a pull request"))
	case in.PR != "":
		return model.SyncMirror, nil
	case in.Sync == "":
		return defaultIssueSync, nil
	case slices.Contains([]string{model.SyncMirror, model.SyncPush, model.SyncLink}, in.Sync):
		return in.Sync, nil
	}
	return invalid(fmt.Errorf("--sync: %q is not mirror, push or link", in.Sync))
}

// Link links the task arg names. Linking again what is unlinked brings it
// back; linking an issue again with another Sync changes its mode.
func Link(ss *usecase.Session, arg string, in LinkInput) (usecase.TaskResult, error) {
	return change(ss, arg, "link", in, func(links []model.Link, ref model.GitHubRef) ([]model.Link, error) {
		sync, err := in.sync()
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(links, func(l model.Link) bool { return model.CanonRef(l.Ref) == ref.String() })
		if i < 0 {
			return append(slices.Clone(links), model.Link{Type: ref.Kind, Ref: ref.String(), Sync: sync}), nil
		}
		if links[i].Live() && (in.Sync == "" || links[i].Sync == sync) {
			return nil, usecase.NewErr(usecase.KindInvalid, "link.exists", fmt.Errorf("%s is linked already", ref.Label()))
		}
		out := slices.Clone(links)
		out[i].Sync = sync
		return out, nil
	})
}

// Unlink turns a link off. It stays in the file, so that the automatic
// linking does not link the pull request again.
func Unlink(ss *usecase.Session, arg string, in LinkInput) (usecase.TaskResult, error) {
	return change(ss, arg, "unlink", in, func(links []model.Link, ref model.GitHubRef) ([]model.Link, error) {
		i := slices.IndexFunc(links, func(l model.Link) bool { return model.CanonRef(l.Ref) == ref.String() && l.Live() })
		if i < 0 {
			return nil, usecase.NewErr(usecase.KindInvalid, "link.not_found", fmt.Errorf("%s is not linked", ref.Label()))
		}
		out := slices.Clone(links)
		out[i].Sync = model.SyncOff
		return out, nil
	})
}

func change(ss *usecase.Session, arg, op string, in LinkInput, fn func([]model.Link, model.GitHubRef) ([]model.Link, error)) (usecase.TaskResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if ts != ss {
		return change(ts, arg, op, in, fn)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return usecase.TaskResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return usecase.TaskResult{}, err
	}
	host, err := ss.WriterHost()
	if err != nil {
		return usecase.TaskResult{}, err
	}
	ref, err := in.ref(host)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: op, Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		if t.Links, err = fn(t.Links, ref); err != nil {
			return err
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return usecase.TaskResult{}, err
	}
	return usecase.NewTaskResult(ix, e.ULID, hash), nil
}
```

`cli/link.go`:

```go
package cli

import (
	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase/integration"
)

func (a *app) linkCmd() *cobra.Command {
	var in integration.LinkInput
	cmd := &cobra.Command{
		Use:   "link <id> (--pr owner/repo#n | --issue owner/repo#n) [--sync mirror|push|link]",
		Short: "Link a task to a pull request or an issue",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			r, err := integration.Link(a.ss, args[0], in)
			if err != nil {
				return err
			}
			return a.printTask(r, verb(r.Committed, "Linked"))
		},
	}
	linkFlags(cmd, &in)
	cmd.Flags().StringVar(&in.Sync, "sync", "", "how an issue syncs: mirror, push or link (default link)")
	return cmd
}

func (a *app) unlinkCmd() *cobra.Command {
	var in integration.LinkInput
	cmd := &cobra.Command{
		Use:   "unlink <id> (--pr owner/repo#n | --issue owner/repo#n)",
		Short: "Unlink a task from a pull request or an issue",
		Args:  exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			r, err := integration.Unlink(a.ss, args[0], in)
			if err != nil {
				return err
			}
			return a.printTask(r, verb(r.Committed, "Unlinked"))
		},
	}
	linkFlags(cmd, &in)
	return cmd
}

func linkFlags(cmd *cobra.Command, in *integration.LinkInput) {
	cmd.Flags().StringVar(&in.PR, "pr", "", "the pull request, as owner/repo#n")
	cmd.Flags().StringVar(&in.Issue, "issue", "", "the issue, as owner/repo#n")
}
```

`link.invalid` は `KindUsage` なので終了コード 2 になる。

`app.go:72` の `a.depCmd(),` の後に `a.linkCmd(), a.unlinkCmd(),` を足す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/usecase/integration ./internal/app/cli`
Expected: PASS。コマンド一覧のゴールデンがあれば `go test ./internal/app/cli -update` で更新し、差分が `link` と `unlink` の2行だけであることを確かめる

- [ ] **Step 5: コミットする**

```bash
git add internal/app/usecase/integration internal/app/cli
git commit -m "feat(cli): hh link と hh unlink を足す"
```

---

### Task 12: 自動リンクと `gh` の更新

**Files:**
- Create: `internal/app/usecase/integration/autolink.go`、`internal/app/usecase/integration/apply.go`
- Test: `internal/app/usecase/integration/apply_test.go`・`autolink_test.go`（`package integration`）。`autolink_test.go` は `usecasetest.New`・`AddTask`・`tasks.Remove` で索引を作り、`autoLinks` を確かめる（`ts.Entry()` は使わない）。
- 決まり: `found` は ID・印の直前と直後が `-` のマッチを捨てる。`prefixesOf` はワークスペースの接頭辞と、タスクの `id`・`former_ids` の接頭辞を集める。`autoLinks` は repo を小文字にし、`apply` は ref を大文字小文字を無視して突き合わせる

- [ ] **Step 1: 失敗するテストを書く**

```go
package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/lib/github"
)

const repo = "github.com/org-a/web"

func prRef(n int) string {
	return model.GitHubRef{Repo: repo, Kind: model.LinkPR, Number: n}.String()
}

func TestApplyWritesWhatChanged(t *testing.T) {
	t.Parallel()
	x := &model.Task{Links: []model.Link{{Type: model.LinkPR, Ref: prRef(1), Sync: model.SyncMirror}}}
	got := fetched{pulls: map[string]github.Pull{prRef(1): {State: "open", Review: "pending", Checks: "success"}}}
	changed, err := apply(x, got, nil)
	if err != nil || !changed {
		t.Fatalf("apply = %v, %v", changed, err)
	}
	g, _ := x.GitHub()
	if len(g.PRs) != 1 || g.PRs[0].Ref != prRef(1) || g.PRs[0].Review != "pending" {
		t.Fatalf("prs = %+v", g.PRs)
	}
	if changed, err := apply(x, got, nil); err != nil || changed {
		t.Errorf("the same values: %v, %v", changed, err)
	}
}

func TestApplyKeepsWhatWasNotRead(t *testing.T) {
	t.Parallel()
	x := &model.Task{Links: []model.Link{{Type: model.LinkPR, Ref: prRef(1), Sync: model.SyncMirror}}}
	if err := x.SetGitHub(model.GitHubState{PRs: []model.PRState{{Ref: prRef(1), State: "open", Review: "none"}}}); err != nil {
		t.Fatal(err)
	}
	if changed, err := apply(x, fetched{}, nil); err != nil || changed {
		t.Fatalf("apply = %v, %v", changed, err)
	}
}

func TestApplyDropsWhatIsNoLongerLinked(t *testing.T) {
	t.Parallel()
	x := &model.Task{Links: []model.Link{{Type: model.LinkPR, Ref: prRef(1), Sync: model.SyncOff}}}
	if err := x.SetGitHub(model.GitHubState{PRs: []model.PRState{{Ref: prRef(1), State: "open", Review: "none"}}}); err != nil {
		t.Fatal(err)
	}
	changed, err := apply(x, fetched{}, nil)
	g, _ := x.GitHub()
	if err != nil || !changed || len(g.PRs) != 0 {
		t.Fatalf("apply = %v, %v, %+v", changed, err, g)
	}
}

func TestApplyLinksAutomatically(t *testing.T) {
	t.Parallel()
	x := &model.Task{Links: []model.Link{{Type: model.LinkPR, Ref: prRef(2), Sync: model.SyncOff}}}
	got := fetched{pulls: map[string]github.Pull{prRef(1): {State: "open", Review: "none"}, prRef(2): {State: "open", Review: "none"}}}
	if _, err := apply(x, got, []string{prRef(1), prRef(2)}); err != nil {
		t.Fatal(err)
	}
	if len(x.Links) != 2 || x.Links[1].Ref != prRef(1) || x.Links[1].Sync != model.SyncMirror || x.Links[0].Sync != model.SyncOff {
		t.Fatalf("links = %+v", x.Links)
	}
}

func TestApplyIssueKeepsSynced(t *testing.T) {
	t.Parallel()
	ref := model.GitHubRef{Repo: repo, Kind: model.LinkIssue, Number: 5}.String()
	d, err := model.DecodeTask([]byte("---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nlinks:\n  - {type: github_issue, ref: " + ref + ", sync: link}\ngh:\n  issues:\n    - ref: " + ref + "\n      state: open\n      state_reason: null\n      synced: {title: a}\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := fetched{issues: map[string]github.Issue{ref: {State: "closed", StateReason: "completed", UpdatedAt: time.Now()}}}
	if changed, err := apply(d, got, nil); err != nil || !changed {
		t.Fatalf("apply = %v, %v", changed, err)
	}
	g, _ := d.GitHub()
	if g.Issues[0].State != "closed" || *g.Issues[0].StateReason != "completed" || g.Issues[0].Synced == nil {
		t.Fatalf("issues = %+v", g.Issues)
	}
}

func TestTasksOfPull(t *testing.T) {
	t.Parallel()
	const u = "01JBZ8K3QW7M2X9V4T6R5N8P0A"
	for name, c := range map[string]struct {
		p    github.Pull
		want []string
	}{
		"branch":     {github.Pull{Head: "hh/WEB-42-login"}, []string{"WEB-42"}},
		"long":       {github.Pull{Head: "hedgehug/WEB-42"}, []string{"WEB-42"}},
		"not ours":   {github.Pull{Head: "feature/WEB-42"}, nil},
		"mark":       {github.Pull{Body: "hedgehug:" + u}, []string{u}},
		"short mark": {github.Pull{Title: "x hh:" + u}, []string{u}},
		"id":         {github.Pull{Comments: []string{"fixes WEB-7 and OPS-1"}}, []string{"WEB-7"}},
		"lower":      {github.Pull{Body: "web-7"}, nil},
		"inside":     {github.Pull{Body: "xhh:" + u + " my_hedgehug:" + u + " " + u}, nil},
		"too long":   {github.Pull{Body: "hh:" + u + "X"}, nil},
		"id inside":  {github.Pull{Body: "PRE-WEB-7 WEB-7x XWEB-7 WEB-77"}, []string{"WEB-77"}},
		"id twice":   {github.Pull{Title: "WEB-7", Body: "WEB-7, (WEB-8)"}, []string{"WEB-7", "WEB-8"}},
		"prefix":     {github.Pull{Head: "hh/WEB-42x"}, nil},
		"id range":   {github.Pull{Body: "WEB-1-2 WEB-3"}, []string{"WEB-3"}},
	} {
		got := mentioned(c.p, []string{"WEB"})
		if len(got) != len(c.want) {
			t.Errorf("%s = %v", name, got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s = %v", name, got)
			}
		}
	}
}

func TestApplyMatchesRefsInAnyCase(t *testing.T) {
	t.Parallel()
	x := &model.Task{Links: []model.Link{{Type: model.LinkPR, Ref: prRef(1), Sync: model.SyncMirror}}}
	got := fetched{pulls: map[string]github.Pull{prRef(1): {State: "open", Review: "none"}}}
	if _, err := apply(x, got, []string{strings.ToUpper(prRef(1))}); err != nil {
		t.Fatal(err)
	}
	if len(x.Links) != 1 {
		t.Fatalf("links = %+v", x.Links)
	}
	if err := x.SetGitHub(model.GitHubState{PRs: []model.PRState{{Ref: strings.ToUpper(prRef(1)), State: "open", Review: "none"}}}); err != nil {
		t.Fatal(err)
	}
	if changed, err := apply(x, got, nil); err != nil || changed {
		t.Errorf("a ref in another case: %v, %v", changed, err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/usecase/integration -run 'Apply|TasksOfPull'`
Expected: FAIL（`apply` が未定義）

- [ ] **Step 3: 実装する**

`autolink.go`:

```go
package integration

import (
	"regexp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/lib/github"
)

var (
	branchMark = regexp.MustCompile(`^(?:hh|hedgehug)/([A-Za-z][A-Za-z0-9]*-[0-9]+)(?:-|$)`)
	ulidMark   = regexp.MustCompile(`\b(?:hedgehug|hh):([0-9A-Za-z]{26})\b`)
	idMark     = regexp.MustCompile(`(\b[A-Z][A-Z0-9]*-[0-9]+\b)`)
)

// found returns the last group of the matches of re in text, leaving out a
// match next to a hyphen: in PRE-WEB-7 the id is PRE-WEB-7, not WEB-7, and
// WEB-1-2 is not WEB-1.
// \b does not tell a hyphen from a space, and RE2 has no look-behind.
func found(re *regexp.Regexp, text string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
		if m[0] > 0 && text[m[0]-1] == '-' || m[1] < len(text) && text[m[1]] == '-' {
			continue
		}
		out = append(out, text[m[len(m)-2]:m[len(m)-1]])
	}
	return out
}

// mentioned returns the ids and ULIDs by which p points at a task: its
// branch, and in its title, body and comments a mark or an id with one of
// the prefixes. Each is there once.
func mentioned(p github.Pull, prefixes []string) []string {
	var out []string
	add := func(s string) {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	if m := branchMark.FindStringSubmatch(p.Head); m != nil {
		add(m[1])
	}
	for _, text := range append([]string{p.Title, p.Body}, p.Comments...) {
		for _, u := range found(ulidMark, text) {
			add(strings.ToUpper(u))
		}
		for _, id := range found(idMark, text) {
			if i := strings.LastIndex(id, "-"); slices.Contains(prefixes, id[:i]) {
				add(id)
			}
		}
	}
	return out
}

// prefixesOf returns the id prefixes of the store of ix: those of its
// workspaces, and those its tasks had, as ids and as former ids.
func prefixesOf(ix *index.Index) []string {
	var out []string
	add := func(p string) {
		if p != "" && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	for _, w := range ix.Workspaces() {
		add(w.IDPrefix)
	}
	for _, e := range ix.Tasks() {
		for _, k := range append([]string{e.ID}, e.FormerIDs...) {
			if p, _, ok := model.ParseTaskID(k); ok {
				add(p)
			}
		}
	}
	return out
}

// autoLinks finds the tasks of ix the pull requests of repo point at: by
// task, the refs to link. Ids of the past count, as ix.Resolve finds them.
func autoLinks(ix *index.Index, repo string, pulls []github.Pull) map[model.ULID][]string {
	repo = strings.ToLower(repo) // as hh link writes the ref
	prefixes := prefixesOf(ix)
	out := map[model.ULID][]string{}
	for _, p := range pulls {
		ref := model.GitHubRef{Repo: repo, Kind: model.LinkPR, Number: p.Number}.String()
		for _, name := range mentioned(p, prefixes) {
			u, err := ix.Resolve(name)
			if err != nil {
				continue // a task of no store here, or a number that only looks like an id
			}
			if e, ok := ix.Task(u); !ok || e.DeletedAt != nil || e.Broken != "" {
				continue
			}
			if !slices.Contains(out[u], ref) {
				out[u] = append(out[u], ref)
			}
		}
	}
	return out
}
```

`apply.go`:

```go
package integration

import (
	"bytes"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// fetched is what GitHub said, by ref.
type fetched struct {
	pulls  map[string]github.Pull
	issues map[string]github.Issue
}

// apply brings t up to date: it adds the links in auto that it does not have
// in any form (an unlinked one stays unlinked), and rewrites gh from what was
// fetched. What was not fetched keeps its value; an item of gh with no link
// in use goes. It reports whether t changed.
func apply(t *model.Task, got fetched, auto []string) (bool, error) {
	linked := false
	for _, ref := range auto {
		if !slices.ContainsFunc(t.Links, func(l model.Link) bool { return model.CanonRef(l.Ref) == model.CanonRef(ref) }) {
			t.Links = append(t.Links, model.Link{Type: model.LinkPR, Ref: ref, Sync: model.SyncMirror})
			linked = true
		}
	}
	old, err := t.GitHub()
	if err != nil {
		return false, err
	}
	var next model.GitHubState
	for _, l := range t.Links {
		if !l.Live() {
			continue
		}
		switch l.Type {
		case model.LinkPR:
			ref := model.CanonRef(l.Ref)
			i := slices.IndexFunc(old.PRs, func(p model.PRState) bool { return model.CanonRef(p.Ref) == ref })
			p, ok := got.pulls[ref]
			switch {
			case ok && i >= 0:
				next.PRs = append(next.PRs, withPull(old.PRs[i], p))
			case ok:
				next.PRs = append(next.PRs, withPull(model.PRState{Ref: ref}, p))
			case i >= 0:
				next.PRs = append(next.PRs, old.PRs[i])
			}
		case model.LinkIssue:
			ref := model.CanonRef(l.Ref)
			i := slices.IndexFunc(old.Issues, func(x model.IssueState) bool { return model.CanonRef(x.Ref) == ref })
			x, ok := got.issues[ref]
			switch {
			case ok && i >= 0:
				next.Issues = append(next.Issues, withIssue(old.Issues[i], x))
			case ok:
				next.Issues = append(next.Issues, withIssue(model.IssueState{Ref: ref}, x))
			case i >= 0:
				next.Issues = append(next.Issues, old.Issues[i])
			}
		}
	}
	same, err := sameState(old, next)
	if err != nil {
		return false, err
	}
	if same {
		return linked, nil
	}
	return true, t.SetGitHub(next)
}

func withPull(s model.PRState, p github.Pull) model.PRState {
	s.State, s.Review, s.Checks, s.MergedAt = p.State, p.Review, p.Checks, utc(p.MergedAt)
	return s
}

func withIssue(s model.IssueState, x github.Issue) model.IssueState {
	s.State, s.StateReason = x.State, nil
	if x.StateReason != "" {
		s.StateReason = &x.StateReason
	}
	return s
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// sameState compares what the two write, so that a time read from the file
// equals the same time read from GitHub.
func sameState(a, b model.GitHubState) (bool, error) {
	x, err := yaml.Marshal(a)
	if err != nil {
		return false, err
	}
	y, err := yaml.Marshal(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(x, y), nil
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/usecase/integration`
Expected: PASS

- [ ] **Step 5: コミットする**

```bash
git add internal/app/usecase/integration
git commit -m "feat(integration): 自動リンクと gh の更新を足す"
```

---

### Task 13: 取り込みと `hh sync`

**Files:**
- Create: `internal/app/usecase/integration/fetch.go`、`internal/app/usecase/integration/fetch_test.go`
- Modify: `internal/app/usecase/replication/sync.go:66-72` と `SyncStore` の末尾

`integration` は `replication` を import しない。`replication.SyncStore` が `integration.Fetch` を呼ぶ。

- [ ] **Step 1: 失敗するテストを書く**

`fetch_test.go`:

```go
package integration_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/ghstate"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"go.yaml.in/yaml/v4"
)

const web = "github.com/org-a/web"

func withFake(t *testing.T) (*usecasetest.Session, *fakegh.Server) {
	t.Helper()
	ts, fake := usecasetest.FakeSession(t, fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	}, "")
	ts.AppendConfig("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	ts.ForgetWriter()
	fake.SetNow(func() time.Time { return ts.Now() })
	return ts, fake
}

func fetch(t *testing.T, ts *usecasetest.Session) integration.FetchReport {
	t.Helper()
	c, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	r, err := integration.Fetch(ts.Session, c, e)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func later(ts *usecasetest.Session, d time.Duration) {
	now := ts.Now().Add(d)
	ts.Now = func() time.Time { return now }
}

func TestFetchLinksByBranchAndWritesState(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	n, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.RequestReview("org-a/web", n); err != nil {
		t.Fatal(err)
	}
	r := fetch(t, ts)
	if !r.Committed || len(r.Warnings) != 0 {
		t.Fatalf("report = %+v", r)
	}
	x := taskIn(t, ts, "me", "ME-1")
	g, _ := x.GitHub()
	if len(x.Links) != 1 || len(g.PRs) != 1 || g.PRs[0].State != "open" || g.PRs[0].Review != "pending" {
		t.Fatalf("links = %+v, gh = %+v", x.Links, g)
	}
	if usecasetest.LastOp(t, ts) != "gh-fetch" {
		t.Errorf("op = %s", usecasetest.LastOp(t, ts))
	}
}

func TestFetchKeepsTheInterval(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#1"}); err != nil {
		t.Fatal(err)
	}
	n, err := fake.OpenPull("org-a/web", "b", "A", "", false)
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	fetch(t, ts)
	if err := fake.MergePull("org-a/web", 1); err != nil {
		t.Fatal(err)
	}
	later(ts, 4*time.Minute)
	if r := fetch(t, ts); r.Committed {
		t.Fatal("read within five minutes")
	}
	later(ts, time.Minute)
	if r := fetch(t, ts); !r.Committed {
		t.Fatal("not read after five minutes")
	}
	g, _ := taskIn(t, ts, "me", "ME-1").GitHub()
	if g.PRs[0].State != "merged" {
		t.Errorf("prs = %+v", g.PRs)
	}
	later(ts, 10*time.Minute)
	if r := fetch(t, ts); r.Committed {
		t.Error("committed with nothing new")
	}
}

func TestFetchSlowsDownNearTheRateLimit(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	fake.SetRateLimit(10, 5000, ts.Now().Add(time.Hour))
	fetch(t, ts)
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	later(ts, 14*time.Minute)
	if r := fetch(t, ts); r.Committed {
		t.Fatal("read within fifteen minutes")
	}
	later(ts, time.Minute)
	if r := fetch(t, ts); !r.Committed {
		t.Fatal("not read after fifteen minutes")
	}
}

func TestFetchDoesNotLinkBackWhatWasUnlinked(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	if _, err := integration.Unlink(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#1"}); err != nil {
		t.Fatal(err)
	}
	if err := fake.CommentPull("org-a/web", 1, "ping"); err != nil {
		t.Fatal(err)
	}
	later(ts, time.Hour)
	fetch(t, ts)
	x := taskIn(t, ts, "me", "ME-1")
	g, _ := x.GitHub()
	if len(x.Links) != 1 || x.Links[0].Sync != model.SyncOff || len(g.PRs) != 0 {
		t.Fatalf("links = %+v, gh = %+v", x.Links, g)
	}
}

func TestFetchWarnsAndGoesOn(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{"github.com/org-b/secret", web} })
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "org-a/web#7"}); err != nil {
		t.Fatal(err)
	}
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	r := fetch(t, ts)
	if len(r.Warnings) != 2 || !strings.Contains(r.Warnings[0], "org-a/web#7") || !strings.Contains(r.Warnings[1], "org-b/secret") {
		t.Fatalf("warnings = %q", r.Warnings)
	}
	if !r.Committed {
		t.Error("the repository that reads is not written")
	}
}

func TestFetchSkipsAReadOnlyStore(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	usecasetest.MakeReadOnly(t, ts, "no push access")
	if r := fetch(t, ts); r.Committed || len(r.Warnings) != 0 {
		t.Fatalf("report = %+v", r)
	}
}

func TestSyncReadsGitHubAndPushes(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	usecasetest.AddBareRemote(t, ts)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	r, err := replication.SyncOne(ts.Session)
	if err != nil || r.Err != nil {
		t.Fatalf("sync: %v, %v", err, r.Err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if n, err := usecase.Unpushed(s); err != nil || n != 0 {
		t.Fatalf("unpushed = %d, %v", n, err)
	}
	if len(taskIn(t, ts, "me", "ME-1").Links) != 1 {
		t.Error("not linked")
	}
}

func TestSyncOfAStoreWithoutARemoteReadsGitHub(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := replication.SyncOne(ts.Session); err != nil {
		t.Fatal(err)
	}
	if len(taskIn(t, ts, "me", "ME-1").Links) != 1 {
		t.Error("not linked")
	}
}

// A member who reads what another one wrote already has nothing to write.
// The state of this machine is lost, as on a clone of its own.
func TestFetchOfTheSameValueWritesNothing(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteLocal("github.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if r := fetch(t, ts); r.Committed {
		t.Error("wrote the same value again")
	}
}

func TestSyncOfATaskWithNoRepositoryNeedsNoAccount(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.DropAccounts()
	r, err := replication.SyncOne(ts.Session)
	if err != nil || len(r.Warnings) != 0 {
		t.Fatalf("sync = %+v, %v", r, err)
	}
}

func lastFetch(t *testing.T, ts *usecasetest.Session, store, repo string) time.Time {
	t.Helper()
	s, err := ts.ForStore(store).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	st, err := ghstate.Read(s)
	if err != nil {
		t.Fatal(err)
	}
	return st.Repos[repo].LastFetch
}

// The time moves only for the repository that was read.
func TestFetchAdvancesTheTimeOnlyOfWhatWasRead(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{"github.com/org-b/secret", web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	if got := lastFetch(t, ts, "me", web); !got.Equal(ts.Now()) {
		t.Errorf("read repository = %v, want %v", got, ts.Now())
	}
	if got := lastFetch(t, ts, "me", "github.com/org-b/secret"); !got.IsZero() {
		t.Errorf("repository that failed = %v, want zero", got)
	}
}

// When what was found cannot be written to a task, the repository is not
// marked as read, so that the automatic link is not lost with the time.
func TestFetchKeepsTheTimeWhenATaskCannotBeWritten(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) {
		x.Repos = []string{web}
		x.GH = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "text"}
	})
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	r := fetch(t, ts)
	if len(r.Warnings) != 1 || r.Committed {
		t.Fatalf("report = %+v", r)
	}
	if got := lastFetch(t, ts, "me", web); !got.IsZero() {
		t.Errorf("time = %v, want zero", got)
	}
}

// Two stores that read the same pull request each link only their own tasks,
// and each keeps its own time.
func TestFetchOfAPullRequestAcrossStores(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	fake := fakegh.New(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Users:  []string{"alice", "bob"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	fake.SetNow(ts.Now)
	ts.Env["HH_TOKEN"] = "t-alice"
	ts.AppendConfig("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alice", "bob"} {
		if err := c.SetAccountToken(name, config.TokenSource{Kind: "env", Env: "HH_TOKEN"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	// Both tasks name the repository, so both stores read it.
	for _, arg := range []string{"org-a:API-1", "org-b:WEB-1"} {
		if _, err := integration.Link(ts.Session, arg, integration.LinkInput{PR: "org-a/web#2"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fake.OpenPull("org-a/web", "hh/API-1-x", "WEB-1 too", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := fake.OpenPull("org-a/web", "other", "shared", "", false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"org-a", "org-b"} {
		c, e, err := ts.ForStore(name).Entry()
		if err != nil {
			t.Fatal(err)
		}
		r, err := integration.Fetch(ts.Session, c, e)
		if err != nil || !r.Committed || len(r.Warnings) != 0 {
			t.Fatalf("%s: report = %+v, %v", name, r, err)
		}
	}
	for _, w := range []struct {
		store, id string
		n         int // links
	}{{"org-a", "API-1", 2}, {"org-b", "WEB-1", 2}} {
		x := taskIn(t, ts, w.store, w.id)
		g, _ := x.GitHub()
		if len(x.Links) != w.n || len(g.PRs) != w.n {
			t.Errorf("%s: links = %+v, gh = %+v", w.id, x.Links, g)
		}
		if !lastFetch(t, ts, w.store, web).Equal(ts.Now()) {
			t.Errorf("%s: no time", w.store)
		}
	}
}

func taskIn(t *testing.T, ts *usecasetest.Session, store, id string) *model.Task {
	t.Helper()
	s, ix, err := ts.ForStore(store).Open()
	if err != nil {
		t.Fatal(err)
	}
	e, err := ts.Resolve(ix, id)
	if err != nil {
		t.Fatal(err)
	}
	x, err := s.Task(e.ULID)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
```

以下のテストも足す。取得時刻と失敗の扱い、remote のない Store、push の失敗を確かめる。

`fetch_repeat_test.go`:

```go
package integration_test

import (
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/ghstate"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func stateOf(t *testing.T, ts *usecasetest.Session) ghstate.Repo {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	st, err := ghstate.Read(s)
	if err != nil {
		t.Fatal(err)
	}
	return st.Repos[web]
}

// counting passes the requests on, or answers status for all of them.
type counting struct {
	n      int
	status int
}

func (c *counting) RoundTrip(r *http.Request) (*http.Response, error) {
	c.n++
	if c.status != 0 {
		return &http.Response{StatusCode: c.status, Status: http.StatusText(c.status), Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}")), Request: r}, nil
	}
	return http.DefaultTransport.RoundTrip(r)
}

// GitHub's clock is behind this machine's: what was updated since the last
// read is told by GitHub's own times.
func TestFetchScansFromTheTimeGitHubGave(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	fake.SetNow(func() time.Time { return ts.Now().Add(-time.Hour) })
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "other", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	since := stateOf(t, ts).Since
	if !since.Equal(ts.Now().Add(-time.Hour)) {
		t.Fatalf("since = %v", since)
	}
	later(ts, 6*time.Minute)
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-b", "B", "", false); err != nil {
		t.Fatal(err)
	}
	if r := fetch(t, ts); !r.Committed {
		t.Fatal("the pull request opened after the last read was not read")
	}
	if got := stateOf(t, ts).Since; !got.After(since) {
		t.Errorf("since = %v, want after %v", got, since)
	}
	later(ts, 6*time.Minute)
	fetch(t, ts)
	if got := stateOf(t, ts).Since; !got.Equal(ts.Now().Add(-12*time.Minute - time.Hour + 6*time.Minute)) {
		t.Errorf("since moved with nothing updated: %v", got)
	}
}

func TestFetchDoesNotAskAgainAFailureWithinTheInterval(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		repo   string
		status int
		bad    bool // the token is not known
	}{
		"unauthorized": {repo: web, bad: true},
		"not found":    {repo: "github.com/org-b/secret"},
		"server error": {repo: web, status: 502},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts, fake := withFake(t)
			if c.bad {
				ts.Env["HH_TOKEN"] = "unknown"
			}
			ts.AddTask("a", func(x *model.Task) { x.Repos = []string{c.repo} })
			ct := &counting{status: c.status}
			ts.HTTPClient = &http.Client{Transport: ct}
			first := fetch(t, ts)
			asked := ct.n
			if len(first.Warnings) != 1 || asked == 0 {
				t.Fatalf("report = %+v, requests = %d", first, asked)
			}
			later(ts, 4*time.Minute)
			again := fetch(t, ts)
			if ct.n != asked || !slices.Equal(again.Warnings, first.Warnings) {
				t.Fatalf("requests = %d (was %d), warnings = %q, want %q", ct.n, asked, again.Warnings, first.Warnings)
			}
			later(ts, time.Minute)
			fetch(t, ts)
			if ct.n == asked {
				t.Fatal("not asked again after five minutes")
			}
			// It reads, and the warning goes.
			ct.status = 0
			fake.AddToken("unknown", "alice")
			if c.repo == web {
				later(ts, 5*time.Minute)
				if r := fetch(t, ts); len(r.Warnings) != 0 {
					t.Fatalf("warnings = %q", r.Warnings)
				}
				if st := stateOf(t, ts); st.Warning != "" || !st.LastTry.Before(st.LastFetch) {
					t.Errorf("state = %+v", st)
				}
			}
		})
	}
}

// What a read found, an item that is not on GitHub, is told again until the
// next read, as a failure is, so that it does not fill the log of the GUI.
func TestFetchTellsAgainWhatTheLastReadFound(t *testing.T) {
	t.Parallel()
	ts, _ := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := integrationLinkIssue(ts, 7); err != nil {
		t.Fatal(err)
	}
	ct := &counting{}
	ts.HTTPClient = &http.Client{Transport: ct}
	first := fetch(t, ts)
	asked := ct.n
	if len(first.Warnings) != 1 || !strings.Contains(first.Warnings[0], "org-a/web#7") {
		t.Fatalf("warnings = %q", first.Warnings)
	}
	later(ts, time.Minute)
	again := fetch(t, ts)
	if ct.n != asked || !slices.Equal(again.Warnings, first.Warnings) {
		t.Errorf("requests = %d (was %d), warnings = %q", ct.n, asked, again.Warnings)
	}
}

func TestSyncPushesOnceToEachRemote(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	usecasetest.AddBareRemote(t, ts)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	r, err := replication.SyncOne(ts.Session)
	if err != nil || r.Err != nil {
		t.Fatalf("sync: %v, %v", err, r.Err)
	}
	if len(r.Pushed) == 0 {
		t.Fatal("nothing pushed")
	}
	sorted := slices.Sorted(slices.Values(r.Pushed))
	if len(slices.Compact(slices.Clone(sorted))) != len(sorted) {
		t.Errorf("pushed = %q", r.Pushed)
	}
}

// The push of what GitHub gave fails: the sync has succeeded, and the commit
// is pushed by the next one.
func TestSyncWithAFailedPushOfWhatGitHubGave(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	remote := usecasetest.AddBareRemote(t, ts)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	away := remote + ".away"
	ts.BeforeUpdate = func() { // the remote goes away after the sync, before the push
		ts.BeforeUpdate = nil
		if err := os.Rename(remote, away); err != nil {
			t.Error(err)
		}
	}
	r, err := replication.SyncOne(ts.Session)
	if err != nil || r.Err != nil {
		t.Fatalf("sync: %v, %v", err, r.Err)
	}
	if !slices.ContainsFunc(r.Warnings, func(w string) bool { return strings.Contains(w, "push the values from GitHub") }) {
		t.Fatalf("warnings = %q", r.Warnings)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if n, err := usecase.Unpushed(s); err != nil || n == 0 {
		t.Fatalf("unpushed = %d, %v", n, err)
	}
	if err := os.Rename(away, remote); err != nil {
		t.Fatal(err)
	}
	if r, err := replication.SyncOne(ts.Session); err != nil || r.Err != nil {
		t.Fatalf("next sync: %v, %v", err, r.Err)
	}
	if n, err := usecase.Unpushed(s); err != nil || n != 0 {
		t.Errorf("unpushed = %d, %v", n, err)
	}
}

func TestFetchReadsEveryPageOfTheScan(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "other", "first", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	later(ts, 6*time.Minute)
	for range 150 {
		if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
			t.Fatal(err)
		}
	}
	fetch(t, ts)
	if x := taskIn(t, ts, "me", "ME-1"); len(x.Links) != 150 {
		t.Errorf("links = %d, want 150", len(x.Links))
	}
}

// A write made while the fetch waits for the commit is kept: the fetch
// changes links and gh of the task as the store has it then.
func TestFetchKeepsAWriteMadeDuringIt(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	task := ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	ts.BeforeUpdate = func() {
		ts.BeforeUpdate = nil
		ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
			x, err := tx.Task(task.ULID)
			if err != nil {
				return err
			}
			x.Title = "changed meanwhile"
			return tx.PutTask(x)
		})
	}
	if r := fetch(t, ts); !r.Committed {
		t.Fatal("not committed")
	}
	x := taskIn(t, ts, "me", "ME-1")
	if x.Title != "changed meanwhile" || len(x.Links) != 1 {
		t.Errorf("title = %q, links = %+v", x.Title, x.Links)
	}
}

func integrationLinkIssue(ts *usecasetest.Session, n int) (usecase.TaskResult, error) {
	return integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "org-a/web#" + strconv.Itoa(n)})
}
```

`fetch_targets_test.go`:

```go
package integration_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
)

// A store with no remote that names a repository is synced by hh sync with
// no arguments, and by the loop of the GUI.
func TestSyncOfAllReadsGitHubForAStoreWithoutRemote(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	reps, err := replication.Sync(ts.Session, false, true)
	if err != nil || len(reps) != 1 {
		t.Fatalf("reports = %+v, %v", reps, err)
	}
	if r := reps[0]; r.Store != "me" || !r.NoRemote || !r.GitHubUpdated {
		t.Errorf("report = %+v", r)
	}
	if len(taskIn(t, ts, "me", "ME-1").Links) != 1 {
		t.Error("not linked")
	}
}

func TestSyncOfAllSkipsAStoreWithoutRemoteAndRepository(t *testing.T) {
	t.Parallel()
	ts, _ := withFake(t)
	ts.AddTask("a", nil)
	reps, err := replication.Sync(ts.Session, false, true)
	if err != nil || len(reps) != 0 {
		t.Fatalf("reports = %+v, %v", reps, err)
	}
}
```

`internal/app/server/loop_fetch_test.go`（remote のない Store に書き込んだ後、loop が push_interval で同期せず、`Drain` が待たない）:

```go
package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// A write to a store with no remote has nothing to push. The loop reads
// GitHub for it every interval, not push_interval after each write, and Drain
// does not wait for it.
func TestLoopDoesNotSyncAStoreWithoutRemoteAfterAWrite(t *testing.T) {
	t.Parallel()
	clk := newFakeClock(t0)
	ts := newTestServerWith(t, func(o *Options) { o.Clock, o.Now = clk, clk.Now })
	ts.uc.AddTask("a", func(x *model.Task) { x.Repos = []string{"github.com/org-a/web"} })
	var calls atomic.Int32
	ts.srv.loop.setSyncFn(func(name string) {
		calls.Add(1)
		ts.srv.loop.syncStore(name)
	})
	ctx, cancel := context.WithCancel(context.Background())
	ts.srv.Start(ctx)
	t.Cleanup(func() {
		cancel()
		_ = ts.srv.Shutdown(context.Background())
	})
	eventually(t, "the sync at start", func() bool { return calls.Load() == 1 })

	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	// The next read is an interval after the last one (hidden: ten minutes),
	// not push_interval after the write.
	settle(t, ts, clk, t0.Add(10*time.Minute))
	clk.Advance(5 * time.Minute)
	ts.srv.loop.Wake()
	eventually(t, "the loop to park", ts.srv.loop.parked)
	if n := calls.Load(); n != 1 {
		t.Fatalf("%d syncs after a write", n)
	}
	start := time.Now()
	ts.srv.loop.Drain(5 * time.Second)
	if d := time.Since(start); d > time.Second || calls.Load() != 1 {
		t.Errorf("Drain took %v and synced %d times", d, calls.Load()-1)
	}
	clk.Advance(5 * time.Minute)
	eventually(t, "the next read", func() bool { return calls.Load() == 2 })
}
```

`internal/app/usecase/replication/schedule_test.go` の `TestLoadScheduleSkipsStoresWithoutRemote` は `TestLoadScheduleListsStoresWithoutRemote` にし、remote のない Store が `Stores` に入ることと、`PendingSince` が0であることを確かめる。

`internal/app/cli/sync_github_test.go`:

```go
package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

// githubEnv is a store with no remote whose task is linked to a pull request
// that exists on the fake.
func githubEnv(t *testing.T) (te *testEnv, ulid string) {
	t.Helper()
	te = newStoreEnv(t)
	fake := te.withFake(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	te.writeConfigYAML("accounts:\n  - {name: alice, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	te.env["HH_TOKEN"] = "t-alice"
	if _, err := fake.OpenPull("org-a/web", "other", "A", "", false); err != nil {
		t.Fatal(err)
	}
	a := te.addJSON("a")
	te.ok("link", a.ID, "--pr", "org-a/web#1")
	return te, a.ULID
}

func TestSyncWithoutArgumentsReadsGitHubForAStoreWithoutRemote(t *testing.T) {
	t.Parallel()
	te, ulid := githubEnv(t)
	out := te.ok("sync")
	if strings.Contains(out, "Nothing to sync") || !strings.Contains(out, "Read GitHub for me") {
		t.Errorf("sync = %q", out)
	}
	g, err := te.task(ulid).GitHub()
	if err != nil || len(g.PRs) != 1 || g.PRs[0].State != "open" {
		t.Errorf("gh = %+v, %v", g, err)
	}
}

func TestSyncJSONSaysGitHubWasRead(t *testing.T) {
	t.Parallel()
	te, _ := githubEnv(t)
	var reps []usecase.SyncReport
	if err := json.Unmarshal([]byte(te.ok("sync", "--json")), &reps); err != nil || len(reps) != 1 || !reps[0].NoRemote || !reps[0].GitHubUpdated {
		t.Errorf("reports = %+v, %v", reps, err)
	}
}
```

`autolink_test.go` は `usecasetest` を使うので、`replication` を通る import の循環を避けるため `package integration_test` にし、`export_test.go` の `AutoLinks` を呼ぶ。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/usecase/integration -run 'Fetch|Sync'`
Expected: FAIL（`Fetch` が未定義）

- [ ] **Step 3: `Fetch` を実装する**

`fetch.go`:

```go
package integration

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/ghstate"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// FetchReport is what a fetch did.
type FetchReport struct {
	Committed bool
	Warnings  []string
}

// target is what a repository is read for.
type target struct{ pulls, issues []int }

// Fetch reads the repositories of the store e that are due, links the pull
// requests that point at a task, and writes what GitHub says to gh, as one
// commit. A repository that cannot be read is a warning: the others go on.
// A read-only store is not fetched.
func Fetch(ss *usecase.Session, c *config.Config, e config.StoreEntry) (FetchReport, error) {
	ss = ss.ForStore(e.Name)
	ss.BackgroundSync = false // the sync that called this pushes what it writes
	var rep FetchReport
	s, err := ss.OpenStoreAt(e.Path)
	if err != nil {
		return rep, err
	}
	defer func() { _ = s.Repo().Close() }()
	var ro *usecase.ReadOnlyError
	if err := ss.CheckWritable(s); errors.As(err, &ro) {
		return rep, nil
	} else if err != nil {
		return rep, err
	}
	err = s.WithLocalLock("github.lock", usecase.SyncLockWait, func() error {
		ix, err := ss.IndexOf(e.Name, s)
		if err != nil {
			return err
		}
		return fetchLocked(ss, c, e, s, ix, &rep)
	})
	return rep, err
}

func fetchLocked(ss *usecase.Session, c *config.Config, e config.StoreEntry, s *store.Store, ix *index.Index, rep *FetchReport) error {
	state, err := ghstate.Read(s)
	if err != nil {
		return err
	}
	targets := targetsOf(ix)
	if len(targets) == 0 {
		return nil // no repository: nothing needs an account
	}
	now := ss.Now()
	got := fetched{pulls: map[string]github.Pull{}, issues: map[string]github.Issue{}}
	auto := map[model.ULID][]string{}
	// read is what each repository that was read now knows. It goes into the
	// state only when what was found is written: a repository whose result
	// was lost must be read again from its old time.
	read := map[string]ghstate.Repo{}
	for _, repo := range slices.Sorted(maps.Keys(targets)) {
		st := state.Repos[repo]
		if !st.Due(now) {
			// Not asked again, and what the last read said is said again.
			if st.Warning != "" {
				rep.Warnings = append(rep.Warnings, strings.Split(st.Warning, "\n")...)
			}
			continue
		}
		// fail notes a read that did not happen: it is not tried again within
		// the interval, and its warning is told meanwhile.
		fail := func(msg string) {
			st.LastTry, st.Warning = now, msg
			state.Repos[repo] = st
			rep.Warnings = append(rep.Warnings, msg)
		}
		t := targets[repo]
		cl, acct, err := ss.GitHubForRepo(c, e, repo)
		if err != nil {
			fail(fmt.Sprintf("%s: %v", repo, err))
			continue
		}
		parts := strings.SplitN(repo, "/", 3)
		owner, name := parts[1], parts[2]
		scanned, rl, err := cl.ScanPulls(owner, name, st.Since)
		var linked github.Linked
		if err == nil {
			var rl2 github.RateLimit
			linked, rl2, err = cl.LinkedItems(owner, name, t.pulls, t.issues)
			if rl2.Limit > 0 {
				rl = rl2
			}
		}
		if err != nil {
			var limited *github.RateLimitError
			if errors.As(err, &limited) {
				st.NotBefore = limited.Reset
			}
			fail(github.Describe(err, acct.ID(), acct.Name, "read "+repo).Error())
			continue
		}
		st.LastFetch, st.Limit, st.Remaining, st.ResetAt = now, rl.Limit, rl.Remaining, rl.ResetAt
		st.LastTry, st.Warning = time.Time{}, ""
		for _, p := range scanned {
			if p.UpdatedAt.After(st.Since) {
				st.Since = p.UpdatedAt // GitHub's own time: the clocks may differ
			}
		}
		var found []string
		for _, p := range slices.Concat(scanned, linked.Pulls) {
			got.pulls[model.GitHubRef{Repo: repo, Kind: model.LinkPR, Number: p.Number}.String()] = p
		}
		for _, x := range linked.Issues {
			got.issues[model.GitHubRef{Repo: repo, Kind: model.LinkIssue, Number: x.Number}.String()] = x
		}
		for _, n := range slices.Concat(linked.MissingPulls, linked.MissingIssues) {
			kind := model.LinkPR
			if slices.Contains(linked.MissingIssues, n) {
				kind = model.LinkIssue
			}
			found = append(found, model.GitHubRef{Repo: repo, Kind: kind, Number: n}.Label()+" is not on GitHub")
		}
		rep.Warnings = append(rep.Warnings, found...)
		st.Warning = strings.Join(found, "\n")
		read[repo] = st
		for u, refs := range autoLinks(ix, repo, scanned) {
			auto[u] = append(auto[u], refs...)
		}
	}
	if len(got.pulls)+len(got.issues) == 0 {
		maps.Copy(state.Repos, read) // nothing to write; a rate limit may be noted
		return ghstate.Write(s, state)
	}
	// Only a task with a link to something that was read, or one to link, has
	// anything to write.
	ulids := make([]model.ULID, 0)
	reads := func(x *index.Entry) bool {
		return len(auto[x.ULID]) > 0 || slices.ContainsFunc(x.Links, func(l model.Link) bool {
			_, ok := read[repoOf(l.Ref)]
			return ok
		})
	}
	for _, x := range ix.Tasks() {
		if x.DeletedAt == nil && x.Broken == "" && reads(x) {
			ulids = append(ulids, x.ULID)
		}
	}
	slices.SortFunc(ulids, func(a, b model.ULID) int { return cmp.Compare(a.String(), b.String()) })
	failed := map[string][]string{} // repositories whose result a task could not take, with why
	hash, err := ss.UpdateCommit(s, store.Op{Name: "gh-fetch", Automated: true}, func(tx *store.Tx) error {
		for _, u := range ulids {
			t, err := tx.Task(u)
			if err != nil {
				return err
			}
			changed, err := apply(t, got, auto[u])
			if err != nil {
				rep.Warnings = append(rep.Warnings, u.String()+": "+err.Error())
				for _, ref := range auto[u] {
					failed[repoOf(ref)] = append(failed[repoOf(ref)], u.String()+": "+err.Error())
				}
				for _, l := range t.Links {
					failed[repoOf(l.Ref)] = append(failed[repoOf(l.Ref)], u.String()+": "+err.Error())
				}
				continue
			}
			if changed {
				if err := tx.PutTask(t); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		// The reads that failed are noted all the same, or each try would ask
		// them again. What was read stays unmarked: it must be read again.
		return errors.Join(err, ghstate.Write(s, state))
	}
	rep.Committed = !hash.IsZero()
	for repo, st := range read {
		if why, bad := failed[repo]; bad {
			// Read again from the old time. The warning was told already.
			old := state.Repos[repo]
			old.LastTry, old.Warning = now, strings.Join(slices.Compact(slices.Sorted(slices.Values(why))), "\n")
			state.Repos[repo] = old
			continue
		}
		state.Repos[repo] = st
	}
	return ghstate.Write(s, state)
}

// targetsOf collects the repositories the tasks of ix name, by repo ID: those
// in repos and those of the links in use, with the numbers of the links.
func targetsOf(ix *index.Index) map[string]*target {
	out := map[string]*target{}
	at := func(repo string) *target {
		repo = strings.ToLower(repo)
		if out[repo] == nil {
			out[repo] = &target{}
		}
		return out[repo]
	}
	for _, e := range ix.Tasks() {
		if e.DeletedAt != nil || e.Broken != "" {
			continue
		}
		for _, r := range e.Repos {
			at(r)
		}
		for _, l := range e.Links {
			r, ok := model.ParseGitHubRef(l.Ref)
			if !ok || !l.Live() {
				continue
			}
			t := at(r.Repo)
			switch {
			case r.Kind == model.LinkPR && !slices.Contains(t.pulls, r.Number):
				t.pulls = append(t.pulls, r.Number)
			case r.Kind == model.LinkIssue && !slices.Contains(t.issues, r.Number):
				t.issues = append(t.issues, r.Number)
			}
		}
	}
	for _, t := range out {
		slices.Sort(t.pulls)
		slices.Sort(t.issues)
	}
	return out
}

// repoOf is the repo ID of a ref, as targetsOf keys it; "" for a ref that is
// not a GitHub one.
func repoOf(ref string) string {
	r, ok := model.ParseGitHubRef(ref)
	if !ok {
		return ""
	}
	return strings.ToLower(r.Repo)
}

// HasTargets reports whether the tasks of the store e name a repository to
// read: one in repos, or one a link in use points at. A store that does not
// open has none.
func HasTargets(ss *usecase.Session, e config.StoreEntry) bool {
	ss = ss.ForStore(e.Name)
	s, err := ss.OpenStoreAt(e.Path)
	if err != nil {
		return false
	}
	defer func() { _ = s.Repo().Close() }()
	ix, err := ss.IndexOf(e.Name, s)
	return err == nil && len(targetsOf(ix)) > 0
}
```

`rep.Warnings` は repo の名前の順に積まれる。`TestFetchWarnsAndGoesOn` は `org-a/web` のあとに `org-b/secret` が来る順を確かめている。

- 走査の起点は `Since`（GitHub の時計）で、`LastFetch`（手元の時計）は間隔の判定だけに使う。更新がなければ `Since` は据え置く。
- 読めなかった repo は `LastTry` と `Warning` を残す。間隔内は問い合わせず、`Warning` を再掲する。読めたら消す。自動リンク先が GitHub にない警告も `Warning` に残して再掲する。
- 取得時刻は、見つけたものがコミットできた repo だけ進める。タスクが書けなかった repo は `LastFetch` を据え置き、`LastTry` を残す。コミット自体が失敗したときも、読めなかった repo の `LastTry` と `Warning` は `ghstate` に書く（読めた repo は進めない）。
- ref は小文字の形が正規形。`ParseGitHubRef` は大文字を小文字にして読み、`String()` は常に小文字を返す。比較は `model.CanonRef` を通す（`apply`・`link`・`LinkViewsOf`）。手で書かれた大文字の `links` は書き換えず、`merge` の見分け方も変えない。`gh` に新しく入る要素は正規形で書く
- コミットの中で読むタスクは、今回読めた repo にリンクがあるものと、自動リンクの対象に限る。
- `HasTargets` は、Store が取得の対象の repo を持つかを返す。remote のない Store を `hh sync`（引数なし）の対象に入れるために使う。

- [ ] **Step 4: `SyncStore` から呼ぶ**

`sync.go` の `SyncStore` は、remote のない Store でも `fetchGitHub` を呼び（push はしない）、remote のある Store では同期の後に呼ぶ。`fetchGitHub` はコミットができたときだけもう一度 push し、`Pushed` は remote ごとに1回だけ数える。失敗は警告にする。

`hh sync`（引数なし）は、remote のない Store のうち取得対象の repo を持つもの（`fetchOnly`）も対象に入れる。`SyncTargets` は他の用途があるので変えない。`SyncReport.GitHubUpdated` を足し、`printSync` は取り込んだときに「Nothing to sync」でなく `Read GitHub for <store>: …` を出す。

```go
	// SyncStore の冒頭
	if len(e.Remotes) == 0 {
		r.NoRemote = true
		fetchGitHub(ss, c, e, &r, nil)
		return r, nil
	}
	// SyncStore の末尾の return の前
	fetchGitHub(ss, c, e, &r, func() (replicate.Result, error) {
		again, err := ss.OpenStoreAt(e.Path)
		if err != nil {
			return replicate.Result{}, err
		}
		defer func() { _ = again.Repo().Close() }()
		return ss.RunSync(again, opt, true)
	})
```

```go
// fetchGitHub reads GitHub for the store e after its sync. push sends what it
// wrote; nil for a store with no remote. What fails here is a warning, as
// the sync is done.
func fetchGitHub(ss *usecase.Session, c *config.Config, e config.StoreEntry, r *usecase.SyncReport, push func() (replicate.Result, error)) {
	f, err := integration.Fetch(ss, c, e)
	r.GitHubUpdated = f.Committed
	r.Warnings = append(r.Warnings, f.Warnings...)
	if err != nil {
		r.Warnings = append(r.Warnings, "read GitHub: "+err.Error())
		return
	}
	if !f.Committed || push == nil {
		return
	}
	res, err := push()
	if err != nil {
		r.Warnings = append(r.Warnings, "push the values from GitHub: "+err.Error())
		return
	}
	r.Merges += res.Merges
	for _, remote := range res.Pushed {
		if !slices.Contains(r.Pushed, remote) {
			r.Pushed = append(r.Pushed, remote)
		}
	}
	r.Warnings = append(r.Warnings, res.Warnings...)
	r.Notices = append(r.Notices, res.Notices...)
}

```

`Sync` では、`SyncTargets` の後に足す。

```go
	if ss.StoreName == "" && !all {
		targets = append(targets, fetchOnly(ss, c, targets)...)
	}
```

```go
// fetchOnly are the stores with no remote that name a repository on GitHub:
// they have nothing to sync, but GitHub is read for them. Those in done are
// left out.
func fetchOnly(ss *usecase.Session, c *config.Config, done []config.StoreEntry) []config.StoreEntry {
	var out []config.StoreEntry
	for _, e := range c.Stores {
		if len(e.Remotes) == 0 && !slices.ContainsFunc(done, func(d config.StoreEntry) bool { return d.Name == e.Name }) && integration.HasTargets(ss, e) {
			out = append(out, e)
		}
	}
	return out
}
```

`LoadSchedule`（GUI の定期同期）は、remote のない Store も含めてすべての Store を `Stores` に入れる。`LoadSchedule` は書き込みのたびに呼ばれるので、ここでは索引を開いて `HasTargets` を問わない。取得対象の repo がなければ `SyncStore` の中の `Fetch` が何もせずに終わる。`replication.PendingSince` は、remote のない Store では常に 0 を返す。書き込みが印を付けても push するものはなく、そのままだと loop が書き込みのたびに push_interval 後に同期し、`Drain` が待ってしまう。次の取り込みは interval 後になる。

- [ ] **Step 5: 通ることを確かめる**

Run: `go test ./internal/app/usecase/... ./internal/app/cli ./internal/app/server`
Expected: PASS。既存の同期のテストは `repos` のタスクを持たないので GitHub を呼ばない

- [ ] **Step 6: コミットする**

```bash
git add internal/app/usecase internal/app/cli
git commit -m "feat(sync): 同期の後に GitHub の状態を取り込む"
```

---

### Task 14: `hh show` にリンクを出す

**Files:**
- Modify: `internal/app/usecase/read_shared.go:49-63`、`internal/app/usecase/browse/read.go:357-364`、`internal/app/cli/show.go`
- Modify: `api/openapi.yaml`、`web/core/api/schema.d.ts`（生成）
- Test: `internal/app/usecase/browse/read_test.go`、`internal/app/cli/show_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`read_test.go`:

```go
func TestShowLinks(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	x := ts.AddTask("a", func(x *model.Task) {
		x.Links = []model.Link{
			{Type: model.LinkPR, Ref: "github.com/org-a/web/pull/1", Sync: model.SyncMirror},
			{Type: model.LinkPR, Ref: "github.com/org-a/web/pull/2", Sync: model.SyncOff},
			{Type: model.LinkIssue, Ref: "github.com/org-a/web/issues/3", Sync: model.SyncLink},
		}
		if err := x.SetGitHub(model.GitHubState{PRs: []model.PRState{{Ref: "github.com/org-a/web/pull/1", State: "open", Review: "pending", Checks: "success"}}}); err != nil {
			t.Fatal(err)
		}
	})
	v, err := browse.Show(ts.Session, x.ULID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Links) != 2 {
		t.Fatalf("links = %+v", v.Links)
	}
	p, i := v.Links[0], v.Links[1]
	if p.Label != "org-a/web#1" || p.Type != model.LinkPR || p.State != "open" || p.Review != "pending" || p.Checks != "success" {
		t.Errorf("pull = %+v", p)
	}
	if i.Label != "org-a/web#3" || i.Sync != model.SyncLink || i.State != "" {
		t.Errorf("issue = %+v", i)
	}
}
```

`show_test.go`:

```go
func TestShowPrintsLinks(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	te.ok("link", a.ID, "--pr", "org-a/web#1")
	if out := te.ok("show", a.ID); !strings.Contains(out, "Links:\n  pull request  org-a/web#1  not read yet\n") {
		t.Errorf("show:\n%s", out)
	}
	if out := te.ok("show", a.ID, "--json"); !strings.Contains(out, `"label": "org-a/web#1"`) {
		t.Errorf("json:\n%s", out)
	}
}

func TestLinkLine(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   usecase.LinkView
		want string
	}{
		{"pull request not read", usecase.LinkView{Type: "github_pr", Label: "o/r#1"}, "pull request  o/r#1  not read yet"},
		{"pull request with checks", usecase.LinkView{Type: "github_pr", Label: "o/r#1", State: "open", Review: "approved", Checks: "success"}, "pull request  o/r#1  open  review: approved  checks: success"},
		{"pull request without checks", usecase.LinkView{Type: "github_pr", Label: "o/r#1", State: "merged", Review: "none"}, "pull request  o/r#1  merged  review: none"},
		{"issue", usecase.LinkView{Type: "github_issue", Label: "o/r#2", State: "closed", Sync: "push"}, "issue  o/r#2  closed  sync: push"},
		{"issue not read", usecase.LinkView{Type: "github_issue", Label: "o/r#2", Sync: "link"}, "issue  o/r#2  not read yet"},
	} {
		if got := linkLine(c.in); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/usecase/browse ./internal/app/cli -run 'Links'`
Expected: FAIL（`v.Links` が未定義）

- [ ] **Step 3: 実装する**

`read_shared.go` の `ShowView` の `History` の下に足す。

```go
	Links        []LinkView `json:"links,omitempty" nullable:"false" doc:"The pull requests and issues linked, with what GitHub says of them"`
```

型と関数を足す。

```go
// LinkView is a link in use and what GitHub said of it when it was last read.
type LinkView struct {
	Type   string `json:"type" enum:"github_pr,github_issue"`
	Ref    string `json:"ref"`
	Label  string `json:"label" doc:"The ref as people write it: org-a/web#123"`
	Sync   string `json:"sync" enum:"mirror,push,link"`
	State  string `json:"state,omitempty" doc:"Empty until GitHub is read"`
	Review string `json:"review,omitempty"`
	Checks string `json:"checks,omitempty"`
	// MergedAt is when GitHub merged the pull request.
	MergedAt *time.Time `json:"merged_at,omitempty"`
	// StateReason is why an issue closed: completed or not_planned.
	StateReason string `json:"state_reason,omitempty"`
}

// LinkViewsOf lists the links of t in use, in file order. A gh that does not
// read shows the links without state.
func LinkViewsOf(t *model.Task) []LinkView {
	g, _ := t.GitHub()
	var out []LinkView
	for _, l := range t.Links {
		if !l.Live() {
			continue
		}
		v := LinkView{Type: l.Type, Ref: l.Ref, Label: l.Ref, Sync: l.Sync}
		if r, ok := model.ParseGitHubRef(l.Ref); ok {
			v.Ref, v.Label = r.String(), r.Label()
		}
		ref := model.CanonRef(l.Ref)
		for _, p := range g.PRs {
			if model.CanonRef(p.Ref) == ref {
				v.State, v.Review, v.Checks, v.MergedAt = p.State, p.Review, p.Checks, p.MergedAt
			}
		}
		for _, x := range g.Issues {
			if model.CanonRef(x.Ref) == ref {
				v.State = x.State
				if x.StateReason != nil {
					v.StateReason = *x.StateReason
				}
			}
		}
		out = append(out, v)
	}
	return out
}
```

`browse/read.go` の `Show` の `j.Body, j.Watchers, ...` の行の下に足す。

```go
		j.Links = usecase.LinkViewsOf(t)
```

`cli/show.go` の `Referenced by:` の節の下に足す。

```go
	if len(v.Links) > 0 {
		b.WriteString("\nLinks:\n")
		for _, l := range v.Links {
			b.WriteString("  " + linkLine(l) + "\n")
		}
	}
```

ファイルの末尾に足す。

```go
// linkLine is one link of hedgehug show.
func linkLine(l usecase.LinkView) string {
	kind := "issue"
	if l.Type == "github_pr" {
		kind = "pull request"
	}
	parts := []string{kind, l.Label}
	switch {
	case l.State == "":
		parts = append(parts, "not read yet")
	case l.Type == "github_pr":
		parts = append(parts, l.State, "review: "+l.Review)
		if l.Checks != "" {
			parts = append(parts, "checks: "+l.Checks)
		}
	default:
		parts = append(parts, l.State, "sync: "+l.Sync)
	}
	return strings.Join(parts, "  ")
}
```

- [ ] **Step 4: 生成物を作り直す**

Run: `go generate ./internal/app/server && cd web && pnpm gen`
Expected: `api/openapi.yaml` と `web/core/api/schema.d.ts` に `LinkView` と `links` が足される。他の差分はない

- [ ] **Step 5: 通ることを確かめる**

Run: `go test ./internal/app/... && cd web && pnpm check`
Expected: PASS、エラー 0。`links` は任意の項目なので web のテストの型は変わらない

- [ ] **Step 6: コミットする**

```bash
git add internal/app api web/core/api
git commit -m "feat(show): リンクと GitHub の状態を出す"
```

---

### Task 15: 実 API の定期テスト

**Files:**
- Create: `internal/lib/github/live_test.go`、`.github/workflows/github-live.yaml`

検証用 Org の固定のリポジトリ `hh-live` を読む。書き込みはしない。secret `HH_LIVE_ORG` と `HH_LIVE_TOKEN` が未設定のときは、ジョブは手順を実行せずに成功する。テストも `t.Skip` せず、何もせずに終わる。

- [ ] **Step 1: テストを書く**

```go
//go:build live

package github

import (
	"os"
	"testing"
	"time"
)

// TestLiveQueries runs the queries against GitHub: the fields of the queries
// must exist, and the answers must decode. .github/workflows/github-live.yaml
// runs it on a schedule. It only reads.
func TestLiveQueries(t *testing.T) {
	org, token := os.Getenv("HH_LIVE_ORG"), os.Getenv("HH_LIVE_TOKEN")
	if org == "" || token == "" {
		t.Log("HH_LIVE_ORG or HH_LIVE_TOKEN is not set: nothing to check")
		return
	}
	c := New(Options{API: "https://api.github.com"}, token, nil)
	pulls, rl, err := c.ScanPulls(org, "hh-live", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if rl.Limit == 0 || rl.ResetAt.IsZero() {
		t.Errorf("rate limit = %+v", rl)
	}
	if len(pulls) == 0 {
		t.Fatal("hh-live has no pull request to read")
	}
	for _, p := range pulls {
		if p.State == "" || p.Review == "" || p.UpdatedAt.IsZero() {
			t.Errorf("pull #%d did not decode: %+v", p.Number, p)
		}
	}
	got, _, err := c.LinkedItems(org, "hh-live", []int{pulls[0].Number, 999999}, []int{999998})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pulls) != 1 || got.Pulls[0].Number != pulls[0].Number {
		t.Errorf("pulls = %+v", got.Pulls)
	}
	if len(got.MissingPulls) != 1 || len(got.MissingIssues) != 1 {
		t.Errorf("missing = %+v", got)
	}
}
```

- [ ] **Step 2: タグなしのビルドに影響しないことと、未設定で成功することを確かめる**

Run: `go vet -tags live ./internal/lib/github && go test -tags live -count=1 -run TestLiveQueries ./internal/lib/github`
Expected: PASS（環境変数が未設定なので何もせず終わる）

- [ ] **Step 3: ワークフローを書く**

`actions/checkout` と `actions/setup-go` の SHA は `ci-backend.yaml` と同じものを使う。

```yaml
name: github-live

# 実際の GitHub に対して、Hedgehug の GraphQL のクエリが通ることを確かめる。
# 検証用 Org の固定のリポジトリ hh-live を読むだけで、書き込みはしない。
# secret の HH_LIVE_ORG と HH_LIVE_TOKEN が未設定なら、手順を実行せずに成功する。
# トークンは job の env に置かず、テストの step にだけ渡す。

on:
  schedule:
    - cron: "0 3 * * 1" # 週次
  workflow_dispatch:

permissions: {}

jobs:
  live:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
    env:
      HH_LIVE_ON: ${{ secrets.HH_LIVE_ORG != '' && secrets.HH_LIVE_TOKEN != '' }}
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        if: env.HH_LIVE_ON == 'true'
        with:
          persist-credentials: false

      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        if: env.HH_LIVE_ON == 'true'
        with:
          go-version-file: go.mod

      - name: Run the queries against GitHub
        if: env.HH_LIVE_ON == 'true'
        env:
          HH_LIVE_ORG: ${{ secrets.HH_LIVE_ORG }}
          HH_LIVE_TOKEN: ${{ secrets.HH_LIVE_TOKEN }}
        run: go test -tags live -count=1 -run TestLiveQueries ./internal/lib/github
```

- [ ] **Step 4: ワークフローの検査を通す**

Run: `actionlint .github/workflows/github-live.yaml`
Expected: 指摘なし。`actionlint` が手元になければ `mise exec actionlint -- actionlint .github/workflows/github-live.yaml`

- [ ] **Step 5: コミットする**

```bash
git add internal/lib/github/live_test.go .github/workflows/github-live.yaml
git commit -m "ci: GraphQL のクエリを実際の GitHub で定期的に確かめる"
```

---

### Task 16: 通しのテストと仕上げ

**Files:**
- Create: `internal/app/cli/scenario/github_test.go`（回 5 で `cli/scenario` ができている。なければ `internal/app/cli/github_scenario_test.go`）
- Modify: `docs/superpowers/specs/2026-11-04-codebase-restructure-design.md`（2.3 の領域の数）
- Create: `docs/superpowers/plans/2026-11-05-phase5a-handoff.md`

- [ ] **Step 1: 完了条件の通しのテストを書く**

`withFake` は `auth_test.go` の `te.withFake` を使う。アカウントを持つ設定は `writeConfigYAML` で足す。

```go
func TestSyncBringsGitHubIntoTheTask(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.dropDefaultAccount()
	fake := te.withFake(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	})
	te.writeConfigYAML("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	te.env["HH_TOKEN"] = "t-alice"
	te.ok("add", "a")
	te.ok("set", "ME-1", "repos=github.com/org-a/web")
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	te.ok("sync", "--store", "me")
	if out := te.ok("show", "ME-1"); !strings.Contains(out, "pull request  org-a/web#1  open  review: none") {
		t.Errorf("show:\n%s", out)
	}
	if out := te.ok("ls", "has:pr"); !strings.Contains(out, "ME-1") {
		t.Errorf("has:pr:\n%s", out)
	}
	te.ok("unlink", "ME-1", "--pr", "org-a/web#1")
	if out := te.ok("ls", "has:pr"); strings.Contains(out, "ME-1") {
		t.Errorf("has:pr after unlink:\n%s", out)
	}
}
```

このテストの下に、手で書いた大文字の `links`（`github.com/Org-A/web/pull/1`）が取り込みで `gh` に入り、`hh link` し直しても重複せず、`hh show` に出る `TestSyncTakesALinkWrittenInOtherLetters` も足す。`usecase/integration` には `review_fixes_test.go` を足す（同じ大文字の link、`LinkView` の `merged_at`・`state_reason`、コミットが失敗したときの `LastTry`・`Warning`）。

- [ ] **Step 2: 通ることを確かめる**

Run: `go test ./internal/app/cli/...`
Expected: PASS

- [ ] **Step 3: 構成見直しの spec を直す**

`2026-11-04-codebase-restructure-design.md` の 2.3 の「領域は 12 個（11 個に回 4 で watch を足した。決定 15）」を「領域は 13 個（11 個に回 4 で watch を足し、フェーズ 5a で integration を足した。フェーズ 5b で automations を足して 14 個になる）」にする（5a の時点で `usecase/automations` はまだなく、`integration` だけが増えるため）。2.1 の配置の一覧には `storage/ghstate` と `usecase/integration` を足す。`domain/automation` と `usecase/automations` は 5b で足す。

- [ ] **Step 4: 全体の検査を流す**

Run:

```bash
go vet ./... && go vet -tags e2e ./internal/app/cli && go vet -tags live ./internal/lib/github
gofmt -l .
golangci-lint run ./...
go test -race -count=1 ./...
go generate ./internal/app/server
cd web && pnpm gen && pnpm check && pnpm biome ci . && pnpm vitest run && pnpm build
cd .. && make e2e
git status --short
```

Expected: `gofmt -l` の出力なし、lint は 0 issues、テストはすべて PASS、`git status --short` に生成物の差分なし

- [ ] **Step 5: 検証用 Org で Device Flow と GitHub App を手で確かめる**

自動にできないので、利用者が席にいるときに1回行い、結果を引き継ぎに書く。

1. `hh auth login <name>` の Device Flow でログインし、8 時間の期限の前にトークンが更新されることを、`hh auth status` の残り時間で確かめる
2. Hedgehug の GitHub App を検証用 Org に入れた状態で、`hh sync` が `hh-live` の PR を取り込むことを確かめる
3. App を外した Org の repo では、repo 名付きの警告が出て、同期が成功のままであることを確かめる

- [ ] **Step 6: 引き継ぎを書く**

`2026-11-05-phase5a-handoff.md` に、4f-3 の引き継ぎと同じ形で「いまの状態」「やったこと」「実装中の決定」「次」を書く。「次」は 5b の plan を main から切って書くこととし、この回で 5b が使うもの（`Fetch` の差分の取り方、`Automated`、`defaultIssueSync` の置き換え）を挙げる。5a の spec の項目で残っているものがないことを確かめて書く。

- [ ] **Step 7: コミットする**

```bash
git add internal/app/cli docs/superpowers
git commit -m "test(cli): 取り込みの通しのテストと 5a の引き継ぎを足す"
```

---

## spec との対応

| spec の項目 | Task |
|---|---|
| `links`・`gh` の形、`ref`・`review: none`・`state_reason`・`synced` | 1 |
| `sync` の値、`off`、PR の `sync` | 1、11 |
| `Task.GH` の型付け、知らないキーの保持 | 1 |
| `gh.prs`・`gh.issues` のマージ、inbox に出さない | 2 |
| `links` の同時変更 | 2 |
| `hh link`・`hh unlink` | 11 |
| `hh show`・`--json` | 14 |
| `has:pr` | 4 |
| `hh edit` の拒否 | 5 |
| 取り込みの順番、1コミット、再 push | 13 |
| 対象の repo、間隔、レート制限 | 10、13 |
| 閲覧のみ、失敗の警告 | 13 |
| アカウント | 9 |
| author、op `gh-fetch` | 6、13 |
| GHES の GraphQL の場所 | 7 |
| 自動リンク | 12 |
| fakegh | 8 |
| パッケージ（`storage/ghstate`、`usecase/integration`、2.3 の修正） | 10、11、16 |
| テスト（マージ、形式、`lib/github`、usecase、CLI、実 API） | 2、3、1、7、13、16、15 |
