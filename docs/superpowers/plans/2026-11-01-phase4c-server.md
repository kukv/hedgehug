# フェーズ 4c: サーバー Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `internal/server` に huma v2 の REST API・SSE・同期ループ・埋め込みの配信を作り、`hh ui` で起動できるようにする。Go のテストで全エンドポイントと SSE を確かめ、CLI の変更が SSE で届く。

**Architecture:** `server.New` が `http.ServeMux` の上に humago で huma の API を載せ、外側を Host の検査で包む。`/api` はトークンの検査を huma のミドルウェアで行う。要求ごとに `ops.Session` を作り（`?store=` を `StoreName` に）、ops の結果を API の型に詰め替える。長く生きる状態（同期ループ、SSE の購読と HEAD のポーリング、表示の状態）は `Server` が持つ。時計は `Clock` で差し替える。OpenAPI 3.1 は `go generate ./internal/server` で `api/openapi.yaml` に書き、テストが差分なしを確かめる。応答はテストで `pb33f/libopenapi-validator` を使って `api/openapi.yaml` と照合する。

**Tech Stack:** Go 1.27、`github.com/danielgtaylor/huma/v2` v2.40.0（`adapters/humago`、`sse`）、`github.com/pkg/browser`、`github.com/pb33f/libopenapi-validator`（テストだけ）、既存の internal パッケージ（ops・config・store・index・gitrepo・sync・inbox・view・schema）

**Spec:** `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（「4c サーバー」と「利用者の決定」）。親設計（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「GUI 仕様」が正で、spec はそれに合わせてある。

## Global Constraints

- spec・親設計と違う挙動にしない。迷ったら実装を止めてコントローラーに報告する（自分で決めない）
- 既存の CLI の出力・終了コード・エラー文言を変えない（`hh ui` を足すだけ）。`cmd/hedgehug` のゴールデンの期待値は、ヘルプのコマンド一覧に出る `ui` の1行（Task 9）のほかは変えない
- `internal/ops` と `internal/server` は標準出力に何も印字しない。サーバーの警告とループのエラーは `Options.Log`（`hh ui` では標準エラー）に1行ずつ書く
- 用語: 人が読める識別子は ID、内部の識別子は ULID
- パスはすべて `/api` の下。待ち受けは `127.0.0.1` だけ
- エラーの応答は `application/problem+json`。`type` は ops のエラーの `Code`、引数は `args` に入れる（「この plan で埋める spec にない細部」6）
- コードのコメント・識別子・API の英語のメッセージは英語（既存に合わせる）。コミットメッセージは日本語で、末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` を付ける
- 各タスクの最後に `go test -race ./...`、`golangci-lint run ./...`、`golangci-lint fmt --diff ./...` を通す。テストは前面で実行し、Bash の timeout は 600000 にする
- API を変えたタスクは `go generate ./internal/server` を実行し、`api/openapi.yaml` を同じコミットに入れる
- Windows ではファイルの権限（0600）の検査を飛ばす（`runtime.GOOS == "windows"`）
- コミットの署名は既存の設定（1Password）に任せる。署名に失敗したら止まって知らせる
- シェルでパスに `gitrepo` を書くとサンドボックスに拒否される。`internal/g*repo/` と書く

## 利用者の決定（2026-11-01、spec に反映する）

| 項目 | 決定 |
|---|---|
| タスクの更新 | CLI のコマンドに合わせる。`PATCH /tasks/{id}` は `hh set` の範囲（title・kind・status・assignees・watchers・labels・repos・blocked_by・プロパティ）と本文。本文だけなら `Hedgehug-Op: edit`、ほかは `set`（本文を含めても1コミット）。移動は `POST /tasks/{id}/move`（`mv`）、復元は `POST /tasks/{id}/restore`（`restore`）を足し、spec の API の表を直す |
| 完全削除の確認 | `DELETE /tasks/{id}?purge=true` を `confirm` なしで呼ぶと 409 `purge.unconfirmed` と対象（ULID・ID・タイトル）を返す。SPA は確認のあと `&confirm=<ULID>,<ULID>…` を付けて送り直す。対象が変わっていれば再び 409（`purge.unconfirmed`、新しい対象） |
| Store の指定 | Store ごとの資源はクエリ `?store=<名前>`。省略したら CLI と同じ選び方（`config.Select("")`）。フェーズ3b の統合ビューが入ったら、4f で一覧を全 Store に広げる |
| 表示の状態 | SSE の接続に結びつける。接続ごとに最初のイベント `hello` で `client`（接続の ID）と各 Store の HEAD・同期の状態・未読数を送る。`PUT /api/presence {client, visible}`。接続が切れたらそのタブは数えない |
| 応答の検証 | テストで `pb33f/libopenapi-validator` を使い、コミットした `api/openapi.yaml` で全応答を検証する |

## この plan で埋める spec にない細部（2026-11-01 に利用者が承認済み。spec の「利用者の決定」から参照）

1. huma は v2.40.0、`pkg/browser` は最新の疑似版（v0.0.0-20240102092130-5ac0b6a4141c）
2. トークンは `crypto/rand` の 32 バイトを base64url（パディングなし、43 文字）にする。比較は `subtle.ConstantTimeCompare`
3. トークンがない・違う: 401 `auth.token`。`Host` が違う: 403 `host.refused`（静的ファイルも同じ。本文は problem+json）
4. `GET /api/health` もトークンを要る（`ui.json` のトークンで既存のサーバーを確かめるため）。応答は `{version, pid}`
5. 要求ごとの `ops.Session` の警告（`Warn`）は応答に入れず、`Options.Log` に `warning: <msg>` で書く
6. problem+json の形: `{type, title, status, detail, args?, errors?}`。`type` は ops の Code（`task.not_found` など）、`title` は HTTP の状態の文（`Not Found`）、`detail` は今の CLI の英文、`args` は ops の Args。ops が分類しないエラー（`KindUnknown`）は 500 `internal`。huma の要求の検証エラー（本文の型違いなど。huma は 422 を出す）は 400 `request.invalid` にし、huma の `errors` をそのまま付ける。`schema.violation` には `args.violations: [{code, field, message}]` を足す
7. HTTP の状態: NotFound 404、Ambiguous 409、Invalid 422、Conflict 409（読み取り専用の `store.read_only` も 409）、Usage 400、Unknown 500
8. タスクの応答の型は `ops.TaskView`（`checklist`・`progress` を一覧でも入れる。`context`・`in_cycle`・`children` はツリーだけ）。詳細は `ops.ShowView`（`hh show --json` と同じ）
9. 書き込みの応答: タスク1つは `{task: TaskView, commit: string|null}`、子孫も変わる rm・restore・purge は `{tasks: [TaskView], commit}`（purge は消えたので `ulids: [string]` だけ）。何も変わらなかったら `commit: null`
10. `GET /tasks`: `q`（クエリ文字列、省略は全件）、`tree`（bool）。tree でなければ `{tasks, groups}`（`groups` は `group:` を使ったときだけ。`[{key, ulids}]`）、tree なら `{roots, cycle}`（`hh ls --tree --json` と同じ入れ子）
11. `PATCH /tasks/{id}` の本文: `{title?, kind?, status?, assignees?, watchers?, labels?, repos?, blocked_by?, props?, body?}`。配列は置き換え（`=`）。`props` は名前 → 値（`null` で消す）。値は CLI の `key=value` の値と同じ文字列で、ops の `ApplyAssign` に渡す。空の本文（何も送らない）は 400 `patch.empty`
12. `POST /tasks/{id}/move`: `{parent?, root?, workspace?, before?, after?}`（`hh mv` のフラグと同じ）
13. `POST /tasks`: `{title, parent?, workspace?, kind?, status?, props?, assignees?, labels?}`。`kind: "project"` で add --kind project。応答は 201
14. 履歴: `GET /tasks/{id}/history?limit=`（既定 50、最大 1000）は `[LogView]`。`POST /tasks/{id}/history/{commit}/restore` は `ops.RestoreAt(id, commit)`、応答に `note`
15. アクティビティ: `GET /activity?limit=&author=&sync=`。`limit` 既定 100、最大 1000。`author` は `Hedgehug-Author` のアカウント ID と一致するコミットだけ。`sync=only` は op が `merge`・`repair` のコミットだけ、`sync=exclude` はそれ以外だけ、省略は全部
16. 通知: `GET /notifications?all=`（`all` で既読も）は `[{...InboxItemView, read}]`。`PATCH /notifications/{id}` は `{read: true}` だけ（`false` は 400 `notification.unread`。inbox は未読に戻せない）。`POST /notifications/{id}/revert` は `{task, commit}`
17. 取り消し: `POST /undo` は `{undone: LogView, commit, kept: [{path, task, field, author}]}`。取り消すものがなければ 409 `undo.nothing`
18. ワークスペース: `PATCH /workspaces/{id}` は `{name}` か `{prefix}` のどちらか1つ（両方は 400 `workspace.one_change`。CLI も別のコマンドで、1回1コミットのため）。応答は `{workspace, commit, ids?}`
19. スキーマ: `GET /schema` は schema.yaml の形をそのまま JSON にしたもの（`statuses`・`kinds`・`props`（名前 → 定義）・`workflow`・`scoped`）。`GET /schema/check` は `{findings}`。`POST /schema/migrate` は `{target: status|kind|prop|value, prop?, old, new, dry_run?}` → `{changed: [TaskView], skipped: [ulid], commit}`
20. Store: `GET /stores` は `[StoreView]` に `default: bool` を足す。`POST /stores` は `{name, prefix, workspace?, remotes?, account?}` → 201 `{name, path, workspace, remotes, pushed}`
21. 保守: `GET /doctor` は `{findings}`（修復しない）。`POST /doctor/fix` は `{repaired, findings}`
22. 同期: `GET /sync` は `[StoreStatus に next_fetch を足したもの]`。`POST /sync` は `?store=` があればその Store だけ、なければ remote のある全 Store を同期し `[SyncReport]`
23. 保存済みビュー: `PUT /views/{name}` の本文は `{layout?, query?, group_by?, kanban_columns?, swimlane?, columns?}`（`hh view save` のフラグ。省いたものは保存済みのまま）。`PATCH /views/{name}` は `{name}`（新しい名前）
24. SSE: 15 秒ごとに `: ping` のコメントを送る（中継やブラウザに切られないため）。`tasks.changed` の `ulids` は前回の HEAD の木から今の木への差分（`tasks/` と `comments/` の下）で変わったタスク、`removed` はファイルが消えたタスク、`commit` は今の HEAD、`op`・`author` は今の HEAD のコミットの `Hedgehug-Op`・`Hedgehug-Author`（複数のコミットが進んでも最新の1つ）。`schema.changed` は `schema.yaml`、`views.changed` は `views/` の下が差分にあれば送る。`sync.state` は sync.json・未 push の数・次の fetch の時刻のどれかが変わったら、`notifications.unread` は全 Store の未読の合計が変わったら送る
25. 同期ループ: remote のある Store ごとに、表示中なら `sync.interval`、非表示なら `max(10 分, sync.interval)` ごとに `SyncStore(…, explicit=false)`。書き込みの後は `PendingSince + sync.push_interval` に同期する。起動時に1回、全 Store を同期する。同じ Store の同期は直列にする（既存のロックに加え、ループの中でも重ねない）
26. 停止: Ctrl-C を受けたら新しい要求を受けるのをやめ（`http.Server.Shutdown`）、push 待ち（PendingSince のある Store）があれば同期を1回だけ走らせ、最大 10 秒待って終わる。`ui.json` は自分の pid のときだけ消す
27. `hh ui` は起動したら標準出力に `Hedgehug UI: http://127.0.0.1:<port>/#token=<token>` を1行出す。既存のサーバーを開いたときは `Hedgehug UI is running: <URL>`。`--no-open` はブラウザを開かずに URL を出すだけ。`--port` が使用中なら `listen tcp …` のエラーで終了コード 1
28. `ui.json` はデータディレクトリ（`HEDGEHUG_HOME` があればそこ、なければ `$XDG_DATA_HOME/hedgehug`・OS の既定のデータディレクトリ/hedgehug）に置く。`config.Paths.DataDir` を足す。形は `{port, token, pid, version}`。既存のサーバーの確認は `GET /api/health` を 1 秒のタイムアウトで呼ぶ
29. 埋め込み: `web/dist/.gitkeep` をコミットし、`.gitignore` は `web/dist/*` と `!web/dist/.gitkeep`。dist に `index.html` がなければ、`/api` 以外のすべての GET に「GUI がビルドされていません。make web を実行してください」のページ（200、text/html）を返す
30. OpenAPI の生成: `internal/server/gen.go` に `//go:generate go run ./openapigen -o ../../api/openapi.yaml`。テスト `TestOpenAPIUpToDate` が生成結果とコミットしたファイルを比べ、違えば「go generate ./internal/server を実行」と失敗する（CI は `go test` で差分を検出する）

## ファイル構成

```
internal/config/paths.go          Paths.DataDir を足す
internal/ops/patch.go             Patch（hh set の範囲＋本文を1コミット）
internal/ops/schemaview.go        SchemaView、SchemaViewOf
internal/server/
  server.go                       Options、Server、New、Handler、Start、Shutdown、session
  clock.go                        Clock、realClock
  problem.go                      Problem、huma.NewError の差し替え、problemOf（ops のエラー → HTTP）
  guard.go                        Host の検査、トークンの検査
  static.go                       埋め込みの配信、SPA の index.html、未ビルドのページ
  health.go                       GET /health
  tasks.go                        タスクの読み取り・書き込み・履歴
  comments.go                     コメント、ウォッチ
  activity.go                     undo、通知、アクティビティ
  admin.go                        ワークスペース、スキーマ、Store、保守、保存済みビュー
  syncapi.go                      GET/POST /sync、PUT /presence
  loop.go                         同期ループ（Loop）
  presence.go                     表示の状態（Presence）
  events.go                       SSE（GET /events、Hub）
  poll.go                         HEAD のポーリング（Poller）
  gen.go                          //go:generate
  openapigen/main.go              api/openapi.yaml を書く
  *_test.go                       testServer、validator、各テスト
api/openapi.yaml                  生成物（コミットする）
web/embed.go                      //go:embed all:dist、Dist()
web/dist/.gitkeep
cmd/hedgehug/ui.go                hh ui、ui.json、ブラウザ、シグナル
cmd/hedgehug/app.go               uiCmd の登録
Makefile                          generate ターゲット
.gitignore                        web/dist
```

---
### Task 1: ops の追加（DataDir・Patch・SchemaView）

**Files:**
- Modify: `internal/config/paths.go`（`Paths` に `DataDir`、`Resolve` で埋める）
- Test: `internal/config/paths_test.go`（既存に追加）
- Create: `internal/ops/patch.go`, `internal/ops/patch_test.go`
- Create: `internal/ops/schemaview.go`, `internal/ops/schemaview_test.go`

**Interfaces:**
- Produces:
  - `config.Paths.DataDir string` — `HEDGEHUG_HOME` があればそれ、なければ `<データの基底>/hedgehug`（`StoresDir` の親）
  - `type ops.PatchInput struct { Assigns []Assign; Body *string }`
  - `func (ss *ops.Session) Patch(arg string, in ops.PatchInput) (ops.TaskResult, error)` — 1コミット。`Assigns` が空なら op `edit`、ほかは `set`。何もなければ `KindUsage` `patch.empty`
  - `type ops.SchemaView struct`（下のコード）と `func ops.SchemaViewOf(sc *schema.Schema) ops.SchemaView`

- [ ] **Step 1: DataDir の失敗するテストを書く**

`internal/config/paths_test.go` の末尾に足す。絶対パスの作り方は、同じファイルの既存のテスト（Windows でも通る書き方）に合わせる:

```go
func TestResolveDataDir(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "h")
	env := map[string]string{"HOME": home, "USERPROFILE": home}
	get := func(k string) string { return env[k] }

	env["HEDGEHUG_HOME"] = filepath.Join(root, "hh")
	p, err := Resolve(get)
	if err != nil {
		t.Fatal(err)
	}
	if p.DataDir != env["HEDGEHUG_HOME"] {
		t.Errorf("with HEDGEHUG_HOME: DataDir = %q", p.DataDir)
	}

	delete(env, "HEDGEHUG_HOME")
	env["XDG_DATA_HOME"] = filepath.Join(root, "data")
	if p, err = Resolve(get); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "data", "hedgehug"); p.DataDir != want {
		t.Errorf("with XDG_DATA_HOME: DataDir = %q, want %q", p.DataDir, want)
	}
	if filepath.Dir(p.StoresDir) != p.DataDir {
		t.Errorf("StoresDir %q is not under DataDir %q", p.StoresDir, p.DataDir)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/config/ -run TestResolveDataDir`
Expected: FAIL（`p.DataDir undefined`）

- [ ] **Step 3: DataDir を足す**

`internal/config/paths.go`:

```go
type Paths struct {
	Home       string // the user's home directory
	ConfigFile string // config.yaml
	DataDir    string // the data of hedgehug: stores/ and ui.json
	StoresDir  string // the directory new stores go in, one per name
}
```

`Resolve` の `HEDGEHUG_HOME` の枝は `p.DataDir = hh` と `p.StoresDir = filepath.Join(hh, "stores")`、最後の枝は `p.DataDir = filepath.Join(data, "hedgehug")` と `p.StoresDir = filepath.Join(p.DataDir, "stores")` にする（`StoresDir` の値は変わらない）。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/config/`
Expected: PASS

- [ ] **Step 5: Patch の失敗するテストを書く**

`internal/ops/patch_test.go`。`doing` が `schema.Default()` のステータスになければ、既定の work カテゴリ（未着手でも完了でもない）のステータス名に置き換える:

```go
package ops

import "testing"

// headOp is the Hedgehug-Op of the commit at HEAD of the test store.
func headOp(t *testing.T, ts *testSession) string {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	log, err := s.Repo().Log(nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	op, _ := log[0].Trailer("Hedgehug-Op")
	return op
}

func TestPatchSetsFieldsAndBodyInOneCommit(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.Repo().Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	body := "new body\n"
	res, err := ts.Patch("ME-1", PatchInput{
		Assigns: []Assign{{Key: "title", Op: "=", Value: "b"}, {Key: "status", Op: "=", Value: "doing"}},
		Body:    &body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Committed || res.Commit == "" {
		t.Fatalf("result = %+v, want a commit", res)
	}
	after, err := s.Repo().Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("commits: %d → %d, want one more", len(before), len(after))
	}
	if op := headOp(t, ts); op != "set" {
		t.Errorf("op = %q, want set", op)
	}
	got, err := s.Task(res.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "b" || got.Status != "doing" || got.Body != body {
		t.Errorf("task = %q %q %q", got.Title, got.Status, got.Body)
	}
}

func TestPatchBodyOnlyIsEdit(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	body := "only the body\n"
	if _, err := ts.Patch("ME-1", PatchInput{Body: &body}); err != nil {
		t.Fatal(err)
	}
	if op := headOp(t, ts); op != "edit" {
		t.Errorf("op = %q, want edit", op)
	}
}

func TestPatchNothingIsUsage(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.Patch("ME-1", PatchInput{})
	wantErr(t, err, KindUsage, "patch.empty")
}

func TestPatchDeletedIsConflict(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	body := "x"
	_, err := ts.Patch("ME-1", PatchInput{Body: &body})
	wantErr(t, err, KindConflict, "task.deleted")
}

func TestPatchUnknownKeyIsInvalid(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.Patch("ME-1", PatchInput{Assigns: []Assign{{Key: "nope", Op: "=", Value: "x"}}})
	wantErr(t, err, KindInvalid, "assign.unknown_key")
}
```

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/ops/ -run TestPatch`
Expected: FAIL（`ts.Patch undefined`）

- [ ] **Step 7: Patch を書く**

`internal/ops/patch.go`:

```go
package ops

import (
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/store"
)

// PatchInput is what PATCH /tasks/{id} changes: Assigns as hedgehug set
// takes them, and the body when Body is not nil.
type PatchInput struct {
	Assigns []Assign
	Body    *string
}

// Patch makes the changes of in to the task arg names in one commit. Its op
// is edit when only the body changes, set otherwise, as hedgehug edit and
// hedgehug set name theirs.
func (ss *Session) Patch(arg string, in PatchInput) (TaskResult, error) {
	if len(in.Assigns) == 0 && in.Body == nil {
		return TaskResult{}, newErr(KindUsage, "patch.empty", errors.New("nothing to change"))
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return TaskResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return TaskResult{}, err
	}
	if err := NotDeleted(ix, e); err != nil {
		return TaskResult{}, err
	}
	var host string
	if in.Body != nil {
		if host, err = ss.WriterHost(); err != nil {
			return TaskResult{}, err
		}
	}
	op := "set"
	if len(in.Assigns) == 0 {
		op = "edit"
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: op, Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		assignees := slices.Clone(t.Assignees)
		for _, as := range in.Assigns {
			if err := ApplyAssign(t, as, ix.Schema(), ss.ResolveULID(ix)); err != nil {
				return err
			}
		}
		watchNewAssignees(t, assignees)
		if in.Body != nil {
			old := t.Body
			t.Body = *in.Body
			AddWatchers(t, NewMentions(old, t.Body, host)...)
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return TaskResult{}, err
	}
	return TaskResult{Index: ix, ULID: e.ULID, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}
```

- [ ] **Step 8: 通ることを確かめる**

Run: `go test ./internal/ops/ -run TestPatch`
Expected: PASS

- [ ] **Step 9: SchemaView の失敗するテストを書く**

`internal/ops/schemaview_test.go`:

```go
package ops

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/schema"
)

func TestSchemaViewOfDefault(t *testing.T) {
	v := SchemaViewOf(schema.Default())
	if len(v.Statuses) == 0 || len(v.Kinds) == 0 {
		t.Fatalf("view = %+v, want the statuses and kinds of the default", v)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"statuses", "kinds", "props", "workflow", "scoped"} {
		if m[k] == nil {
			t.Errorf("key %q missing or null in %s", k, b)
		}
	}
	if st := v.Statuses[0]; st.Name == "" || st.Category == "" {
		t.Errorf("status = %+v", st)
	}
}
```

- [ ] **Step 10: 失敗を確かめる**

Run: `go test ./internal/ops/ -run TestSchemaViewOf`
Expected: FAIL（`SchemaViewOf undefined`）

- [ ] **Step 11: SchemaView を書く**

`internal/ops/schemaview.go`。schema.yaml のキーと同じ名前の JSON にする。空の配列・マップは `[]`・`{}` で出す（`null` にしない。`KindDef.Parents` だけは `null` が「どこでも」の意味なので残す）。`schema.Lists` と `schema.Templates` の定義を `internal/schema` で読み、`listsOf` と `TemplatesDef` をその形に合わせて書く（`Templates` は schema.yaml のキーどおりの JSON にする）:

```go
package ops

import "github.com/kukv/hedgehug/internal/schema"

// SchemaView is schema.yaml as GET /api/schema returns it: the keys of the
// file, with props as a mapping of name to definition.
type SchemaView struct {
	Statuses []StatusDef        `json:"statuses"`
	Kinds    []KindDef          `json:"kinds"`
	Props    map[string]PropDef `json:"props"`
	Workflow WorkflowDef        `json:"workflow"`
	Scoped   []ScopedDef        `json:"scoped"`
}

// StatusDef is a status of the schema.
type StatusDef struct {
	Name     string `json:"name"`
	Label    string `json:"label,omitempty"`
	Category string `json:"category"`
	Color    string `json:"color,omitempty"`
	Default  bool   `json:"default,omitempty"`
}

// KindDef is a kind of the schema; Parents nil means anywhere.
type KindDef struct {
	Name     string   `json:"name"`
	Label    string   `json:"label,omitempty"`
	Role     string   `json:"role"`
	Parents  []string `json:"parents"`
	Icon     string   `json:"icon,omitempty"`
	Template string   `json:"template,omitempty"`
	Default  bool     `json:"default,omitempty"`
}

// PropDef is a property of the schema.
type PropDef struct {
	Type      string   `json:"type"`
	Values    []string `json:"values,omitempty"`
	Multi     bool     `json:"multi,omitempty"`
	Label     string   `json:"label,omitempty"`
	Inverse   string   `json:"inverse,omitempty"`
	Symmetric bool     `json:"symmetric,omitempty"`
}

// WorkflowDef is the workflow: status → statuses it may move to, and
// status → fields needed to enter it.
type WorkflowDef struct {
	Transitions map[string][]string `json:"transitions"`
	Required    map[string][]string `json:"required"`
}

// ScopedDef is an override of a project.
type ScopedDef struct {
	Where     string        `json:"where"`
	Statuses  []string      `json:"statuses,omitempty"`
	Workflow  *WorkflowDef  `json:"workflow,omitempty"`
	Templates *TemplatesDef `json:"templates,omitempty"`
}

// SchemaViewOf is sc as SchemaView.
func SchemaViewOf(sc *schema.Schema) SchemaView {
	v := SchemaView{
		Statuses: []StatusDef{}, Kinds: []KindDef{}, Props: map[string]PropDef{},
		Workflow: workflowDefOf(sc.Workflow), Scoped: []ScopedDef{},
	}
	for _, st := range sc.Statuses {
		v.Statuses = append(v.Statuses, StatusDef{st.Name, st.Label, string(st.Category), st.Color, st.Default})
	}
	for _, k := range sc.Kinds {
		v.Kinds = append(v.Kinds, KindDef{k.Name, k.Label, string(k.Role), k.Parents, k.Icon, k.Template, k.Default})
	}
	for _, p := range sc.Props {
		v.Props[p.Name] = PropDef{string(p.Type), p.Values, p.Multi, p.Label, p.Inverse, p.Symmetric}
	}
	for _, s := range sc.Scoped {
		d := ScopedDef{Where: s.Where, Statuses: s.Statuses}
		if s.Workflow != nil {
			w := workflowDefOf(*s.Workflow)
			d.Workflow = &w
		}
		if s.Templates != nil {
			d.Templates = templatesDefOf(s.Templates)
		}
		v.Scoped = append(v.Scoped, d)
	}
	return v
}

func workflowDefOf(w schema.Workflow) WorkflowDef {
	return WorkflowDef{Transitions: listsOf(w.Transitions), Required: listsOf(w.Required)}
}
```

- [ ] **Step 12: 通ることを確かめる**

Run: `go test ./internal/ops/ ./internal/config/`
Expected: PASS

- [ ] **Step 13: 全体の検査とコミット**

Run: `go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...`
Expected: すべて成功、fmt の差分なし

```bash
git add internal/config/paths.go internal/config/paths_test.go internal/ops/patch.go internal/ops/patch_test.go internal/ops/schemaview.go internal/ops/schemaview_test.go
git commit -m "feat(ops): API 用に Patch と SchemaView、設定に DataDir を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: サーバーの土台（huma、Host とトークン、problem+json、埋め込み、OpenAPI の生成）

**Files:**
- Create: `internal/server/server.go`, `internal/server/problem.go`, `internal/server/guard.go`, `internal/server/static.go`, `internal/server/health.go`, `internal/server/gen.go`, `internal/server/openapigen/main.go`
- Create: `internal/server/helpers_test.go`, `internal/server/server_test.go`
- Create: `web/embed.go`, `web/dist/.gitkeep`
- Create: `api/openapi.yaml`（生成物）
- Modify: `.gitignore`、`Makefile`、`go.mod`・`go.sum`

**Interfaces:**
- Consumes: `ops.Session`、`ops.AsError`、`ops.Kind*`、`ops.ViolationsText`、`store.ValidationError`、`store.ErrLocked`
- Produces（後のタスクが使う）:
  - `type server.Options struct { Getenv func(string) string; Now func() time.Time; Keyring auth.Keyring; HTTPClient *http.Client; Run func(string, ...string) ([]byte, []byte, error); Token, Version string; Static fs.FS; Log io.Writer }`（Task 7 で `Clock` を足す）
  - `func server.New(opt Options) *Server`、`(*Server).Handler() http.Handler`、`(*Server).SetPort(port int)`、`(*Server).API() huma.API`
  - `func (s *Server) session(store string) *ops.Session` — 要求ごとの Session
  - `func (s *Server) logf(format string, args ...any)`
  - `type server.Problem struct`、`func problemOf(err error) *Problem`、`func writeProblem(w http.ResponseWriter, p *Problem)`
  - `type storeParam struct { Store string \`query:"store"\` }` — Store ごとの資源の入力に埋め込む
  - `func server.OpenAPIYAML() ([]byte, error)`
  - `func web.Dist() fs.FS`
  - テスト: `newTestServer(t) *testServer`、`(*testServer).call(method, path string, body any, want int, out any) *http.Response`、`(*testServer).raw(req *http.Request) *http.Response`、`(*testServer).session() *ops.Session`

- [ ] **Step 1: 依存を足す**

```bash
go get github.com/danielgtaylor/huma/v2@v2.40.0
go get github.com/pb33f/libopenapi-validator@latest
```

`github.com/pkg/browser` は Task 9 で足す。

- [ ] **Step 2: テストの土台を書く**

`internal/server/helpers_test.go`。テストの Store は `ops.StoreInit` で作る（名前 `me`、接頭辞 `ME`）。応答は毎回、コミットした `api/openapi.yaml` で検証する:

```go
package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"

	"github.com/kukv/hedgehug/internal/ops"
)

const testToken = "test-token"

var (
	specOnce sync.Once
	specV    validator.Validator
	specErr  error
)

// specValidator is the validator of api/openapi.yaml as committed.
func specValidator(t *testing.T) validator.Validator {
	t.Helper()
	specOnce.Do(func() {
		var b []byte
		if b, specErr = os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml")); specErr != nil {
			return
		}
		doc, err := libopenapi.NewDocument(b)
		if err != nil {
			specErr = err
			return
		}
		v, errs := validator.NewValidator(doc)
		if len(errs) > 0 {
			specErr = errs[0]
			return
		}
		specV = v
	})
	if specErr != nil {
		t.Fatal(specErr)
	}
	return specV
}

// testServer is a Server over a fresh store "me" with one workspace "me"
// (prefix ME), in temporary HOME and HEDGEHUG_HOME directories, behind an
// httptest server.
type testServer struct {
	t    *testing.T
	srv  *Server
	hs   *httptest.Server
	env  map[string]string
	port int
	log  *bytes.Buffer
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	d := t.TempDir()
	home := filepath.Join(d, "home")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Alice\n\temail = alice@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ts := &testServer{t: t, log: &bytes.Buffer{}, env: map[string]string{
		"HOME": home, "USERPROFILE": home,
		"HEDGEHUG_HOME":      filepath.Join(d, "hh"),
		"HEDGEHUG_AUTO_SYNC": "0",
	}}
	if _, err := ts.session().StoreInit(ops.StoreInitInput{Name: "me", Prefix: "ME", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	ts.srv = New(Options{
		Getenv:  ts.getenv,
		Now:     time.Now,
		Token:   testToken,
		Version: "test",
		Log:     ts.log,
	})
	ts.hs = httptest.NewServer(ts.srv.Handler())
	t.Cleanup(ts.hs.Close)
	_, p, err := net.SplitHostPort(ts.hs.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ts.port, _ = strconv.Atoi(p)
	ts.srv.SetPort(ts.port)
	return ts
}

func (ts *testServer) getenv(k string) string { return ts.env[k] }

// session is an ops.Session on the same config, as the CLI would make.
func (ts *testServer) session() *ops.Session {
	return &ops.Session{Getenv: ts.getenv, Now: time.Now, Warn: func(string) {}}
}

// raw sends req as it is and returns the response, its body read into
// memory so that it can be read again.
func (ts *testServer) raw(req *http.Request) *http.Response {
	ts.t.Helper()
	resp, err := ts.hs.Client().Do(req)
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		ts.t.Fatal(err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return resp
}

// call sends method path with body as JSON (none when nil) and the token,
// checks the status is want and the response is as api/openapi.yaml says,
// and decodes the body into out (when not nil).
func (ts *testServer) call(method, path string, body any, want int, out any) *http.Response {
	ts.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			ts.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, ts.hs.URL+path, rd)
	if err != nil {
		ts.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp := ts.raw(req)
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		ts.t.Fatalf("%s %s: status %d, want %d; body %s", method, path, resp.StatusCode, want, b)
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	if ok, errs := specValidator(ts.t).ValidateHttpResponse(req, resp); !ok {
		for _, e := range errs {
			ts.t.Errorf("%s %s: response not as api/openapi.yaml says: %s (%s)", method, path, e.Message, e.Reason)
		}
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			ts.t.Fatalf("%s %s: %v; body %s", method, path, err, b)
		}
	}
	return resp
}

// problem calls method path, wants status, and returns the problem.
func (ts *testServer) problem(method, path string, body any, want int) Problem {
	ts.t.Helper()
	var p Problem
	resp := ts.call(method, path, body, want, &p)
	if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
		ts.t.Errorf("%s %s: Content-Type %q, want application/problem+json", method, path, ct)
	}
	return p
}
```

`ValidateHttpResponse` の `ValidationError` のフィールド名（`Message`・`Reason`）は、入れた版の定義に合わせる。

- [ ] **Step 3: 土台の失敗するテストを書く**

`internal/server/server_test.go`:

```go
package server

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHealth(t *testing.T) {
	ts := newTestServer(t)
	var out struct {
		Version string `json:"version"`
		PID     int    `json:"pid"`
	}
	ts.call("GET", "/api/health", nil, 200, &out)
	if out.Version != "test" || out.PID != os.Getpid() {
		t.Errorf("health = %+v", out)
	}
}

func TestTokenRequired(t *testing.T) {
	ts := newTestServer(t)
	for _, auth := range []string{"", "Bearer wrong", "Basic " + testToken, testToken} {
		req, _ := http.NewRequest("GET", ts.hs.URL+"/api/health", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp := ts.raw(req)
		if resp.StatusCode != 401 {
			t.Errorf("Authorization %q: status %d, want 401", auth, resp.StatusCode)
		}
		b, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(b), `"type":"auth.token"`) {
			t.Errorf("Authorization %q: body %s", auth, b)
		}
	}
}

func TestHostRefused(t *testing.T) {
	ts := newTestServer(t)
	for _, host := range []string{"evil.example:80", "127.0.0.1:1", "localhost", "example.com"} {
		for _, path := range []string{"/api/health", "/", "/index.html"} {
			req, _ := http.NewRequest("GET", ts.hs.URL+path, nil)
			req.Host = host
			req.Header.Set("Authorization", "Bearer "+testToken)
			resp := ts.raw(req)
			if resp.StatusCode != 403 {
				t.Errorf("Host %q %s: status %d, want 403", host, path, resp.StatusCode)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
				t.Errorf("Host %q %s: Content-Type %q", host, path, ct)
			}
		}
	}
	req, _ := http.NewRequest("GET", ts.hs.URL+"/api/health", nil)
	req.Host = "localhost:" + itoa(ts.port)
	req.Header.Set("Authorization", "Bearer "+testToken)
	if resp := ts.raw(req); resp.StatusCode != 200 {
		t.Errorf("Host localhost:<port>: status %d, want 200", resp.StatusCode)
	}
}

func TestNotBuiltPage(t *testing.T) {
	ts := newTestServer(t) // Options.Static is nil
	for _, path := range []string{"/", "/tasks/ME-1", "/index.html"} {
		req, _ := http.NewRequest("GET", ts.hs.URL+path, nil)
		resp := ts.raw(req)
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 || !strings.Contains(string(b), "make web") {
			t.Errorf("%s: status %d body %s", path, resp.StatusCode, b)
		}
	}
	p := ts.problemNoSpec("GET", "/api/nope", nil, 404) // not in the spec, so not validated
	if p.Type != "http.404" {
		t.Errorf("unknown /api path: %+v", p)
	}
}

func TestStaticAndSPAFallback(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	h := staticHandler(dist)
	for path, want := range map[string]string{
		"/":              "<html>app</html>",
		"/tasks/ME-1":    "<html>app</html>",
		"/assets/app.js": "console.log(1)",
	} {
		rec := httptestRecorder(h, "GET", path)
		if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Errorf("%s: %d %q", path, rec.Code, rec.Body.String())
		}
	}
	if rec := httptestRecorder(h, "GET", "/api/nope"); rec.Code != 404 {
		t.Errorf("/api/nope: %d", rec.Code)
	}
}

func TestBadRequestBodyIsRequestInvalid(t *testing.T) {
	ts := newTestServer(t)
	// Any operation with a body will do; PUT /presence comes in Task 7, so
	// this test uses the body of a test-only operation registered below.
	registerEcho(ts.srv)
	p := ts.problemNoSpec("POST", "/api/test-echo", map[string]any{"n": "not a number"}, 400)
	if p.Type != "request.invalid" || len(p.Errors) == 0 {
		t.Errorf("problem = %+v", p)
	}
}

func TestOpenAPIUpToDate(t *testing.T) {
	got, err := OpenAPIYAML()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("api/openapi.yaml is not what the server generates; run go generate ./internal/server")
	}
	if !bytes.HasPrefix(got, []byte("openapi: 3.1")) && !bytes.Contains(got, []byte("\nopenapi: 3.1")) {
		t.Errorf("not OpenAPI 3.1: %.80s", got)
	}
}
```

同じファイルに、テストだけの小物を足す:

```go
func itoa(n int) string { return strconv.Itoa(n) }

func httptestRecorder(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

// registerEcho adds POST /api/test-echo, which takes {n: integer}; it is not
// in api/openapi.yaml, so its responses are checked without the spec.
func registerEcho(s *Server) {
	type in struct {
		Body struct {
			N int `json:"n"`
		}
	}
	huma.Register(s.api, huma.Operation{OperationID: "test-echo", Method: http.MethodPost, Path: "/api/test-echo"},
		func(_ context.Context, _ *in) (*struct{}, error) { return nil, nil })
}

// problemNoSpec is problem for an operation that is not in the spec.
func (ts *testServer) problemNoSpec(method, path string, body any, want int) Problem {
	ts.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, ts.hs.URL+path, rd)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	resp := ts.raw(req)
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		ts.t.Fatalf("%s %s: status %d, want %d; body %s", method, path, resp.StatusCode, want, raw)
	}
	var p Problem
	if err := json.Unmarshal(raw, &p); err != nil {
		ts.t.Fatal(err)
	}
	return p
}
```

（import に `context`・`encoding/json`・`net/http/httptest`・`strconv`・`github.com/danielgtaylor/huma/v2` を足す。）

problemOf の表のテストも足す:

```go
func TestProblemOf(t *testing.T) {
	cases := []struct {
		err    error
		status int
		typ    string
	}{
		{&ops.Error{Kind: ops.KindNotFound, Code: "task.not_found", Err: errors.New("x")}, 404, "task.not_found"},
		{&ops.Error{Kind: ops.KindAmbiguous, Code: "task.ambiguous", Err: errors.New("x")}, 409, "task.ambiguous"},
		{&ops.Error{Kind: ops.KindInvalid, Code: "status.unknown", Err: errors.New("x")}, 422, "status.unknown"},
		{&ops.Error{Kind: ops.KindConflict, Code: "task.deleted", Err: errors.New("x")}, 409, "task.deleted"},
		{&ops.Error{Kind: ops.KindUsage, Code: "patch.empty", Err: errors.New("x")}, 400, "patch.empty"},
		{store.ErrLocked, 409, "store.locked"},
		{errors.New("boom"), 500, "internal"},
	}
	for _, c := range cases {
		p := problemOf(c.err)
		if p.Status != c.status || p.Type != c.typ || p.Title != http.StatusText(c.status) {
			t.Errorf("%v: %+v, want %d %s", c.err, p, c.status, c.typ)
		}
	}
	if p := problemOf(store.ErrLocked); p.Detail != "another hedgehug is writing to the store; try again" {
		t.Errorf("locked detail = %q", p.Detail)
	}
	ve := &store.ValidationError{Violations: []schema.Violation{{Code: "required", Field: "title", Msg: "is required"}}}
	p := problemOf(ve)
	vs, _ := p.Args["violations"].([]map[string]string)
	if p.Status != 422 || p.Type != "schema.violation" || len(vs) != 1 || vs[0]["field"] != "title" {
		t.Errorf("validation: %+v", p)
	}
}
```

（import に `errors`・`internal/ops`・`internal/schema`・`internal/store` を足す。）

- [ ] **Step 4: 失敗を確かめる**

Run: `go test ./internal/server/`
Expected: FAIL（`New`・`Problem` などが未定義）

- [ ] **Step 5: problem.go を書く**

```go
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// Problem is the problem+json body of every error. Type is the Code of the
// ops error (task.not_found...), for the GUI to pick its words; Detail is the
// English message the CLI prints; Args are the arguments of the error.
type Problem struct {
	Type   string              `json:"type" doc:"What went wrong, such as task.not_found"`
	Title  string              `json:"title" doc:"The text of the HTTP status"`
	Status int                 `json:"status"`
	Detail string              `json:"detail,omitempty" doc:"The message in English, as the CLI prints it"`
	Args   map[string]any      `json:"args,omitempty" doc:"The arguments of the error, by name"`
	Errors []*huma.ErrorDetail `json:"errors,omitempty" doc:"What was wrong in the request, for request.invalid"`
}

func (p *Problem) Error() string               { return p.Detail }
func (p *Problem) GetStatus() int              { return p.Status }
func (p *Problem) ContentType(string) string   { return "application/problem+json" }

func init() { huma.NewError = newHumaError }

// newHumaError is the error huma makes itself: a request it cannot read or
// that fails its checks is request.invalid (400), whatever status huma
// chose; the others are typed by their status.
func newHumaError(status int, msg string, errs ...error) huma.StatusError {
	p := &Problem{Status: status, Detail: msg}
	for _, e := range errs {
		if e == nil {
			continue
		}
		var d huma.ErrorDetailer
		if errors.As(e, &d) {
			p.Errors = append(p.Errors, d.ErrorDetail())
		} else {
			p.Errors = append(p.Errors, &huma.ErrorDetail{Message: e.Error()})
		}
	}
	switch {
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		p.Status, p.Type = http.StatusBadRequest, "request.invalid"
	case status >= 500:
		p.Type = "internal"
	default:
		p.Type = "http." + strconv.Itoa(status)
	}
	p.Title = http.StatusText(p.Status)
	return p
}

// problemOf is the problem of an error an operation returned.
func problemOf(err error) *Problem {
	e := ops.AsError(err)
	p := &Problem{Type: e.Code, Detail: detailOf(err), Args: e.Args}
	switch e.Kind {
	case ops.KindNotFound:
		p.Status = http.StatusNotFound
	case ops.KindAmbiguous, ops.KindConflict:
		p.Status = http.StatusConflict
	case ops.KindInvalid:
		p.Status = http.StatusUnprocessableEntity
	case ops.KindUsage:
		p.Status = http.StatusBadRequest
	default:
		p.Status, p.Type, p.Args = http.StatusInternalServerError, "internal", nil
	}
	if p.Type == "" {
		p.Type = "internal"
	}
	var ve *store.ValidationError
	if errors.As(err, &ve) {
		vs := make([]map[string]string, len(ve.Violations))
		for i, v := range ve.Violations {
			vs[i] = map[string]string{"code": v.Code, "field": v.Field, "message": v.Msg}
		}
		if p.Args == nil {
			p.Args = map[string]any{}
		}
		p.Args["violations"] = vs
	}
	p.Title = http.StatusText(p.Status)
	return p
}

// detailOf is the message of err as the CLI prints it after "hedgehug: "
// (cmd/hedgehug describe).
func detailOf(err error) string {
	var ve *store.ValidationError
	if errors.As(err, &ve) {
		return ops.ViolationsText(ve)
	}
	if errors.Is(err, store.ErrLocked) {
		return "another hedgehug is writing to the store; try again"
	}
	return err.Error()
}

// writeProblem writes p outside of huma (the Host and token checks, static
// files).
func writeProblem(w http.ResponseWriter, p *Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// newProblem is a problem of status with type typ and detail msg.
func newProblem(status int, typ, msg string) *Problem {
	return &Problem{Type: typ, Title: http.StatusText(status), Status: status, Detail: msg}
}
```

- [ ] **Step 6: guard.go を書く**

```go
package server

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// guardHost refuses a request whose Host is not 127.0.0.1:<port> or
// localhost:<port>, static files included: a page of another site that
// a DNS name points at 127.0.0.1 cannot reach the server.
func (s *Server) guardHost(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		port := strconv.Itoa(int(s.port.Load()))
		if r.Host != "127.0.0.1:"+port && r.Host != "localhost:"+port {
			writeProblem(w, newProblem(http.StatusForbidden, "host.refused", "the Host header must be 127.0.0.1:"+port+" or localhost:"+port))
			return
		}
		h.ServeHTTP(w, r)
	})
}

// requireToken refuses an API request without Authorization: Bearer <token>.
func (s *Server) requireToken(ctx huma.Context, next func(huma.Context)) {
	got, ok := strings.CutPrefix(ctx.Header("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(s.opt.Token)) != 1 {
		p := newProblem(http.StatusUnauthorized, "auth.token", "the request needs Authorization: Bearer <token>")
		ctx.SetHeader("Content-Type", "application/problem+json")
		ctx.SetStatus(p.Status)
		_ = json.NewEncoder(ctx.BodyWriter()).Encode(p)
		return
	}
	next(ctx)
}
```

（import に `encoding/json` を足す。）

- [ ] **Step 7: static.go と web/embed.go を書く**

`internal/server/static.go`:

```go
package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// notBuiltPage is what / serves when the binary was built without the GUI
// (go build without make web).
const notBuiltPage = `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>Hedgehug</title></head>
<body><p>GUI がビルドされていません。make web を実行してください。</p></body></html>
`

// staticHandler serves the built GUI in dist: a file when there is one,
// index.html for any other path (the SPA routes itself), and a 404 problem
// under /api. Without index.html it serves notBuiltPage.
func staticHandler(dist fs.FS) http.Handler {
	built := dist != nil
	if built {
		if _, err := fs.Stat(dist, "index.html"); err != nil {
			built = false
		}
	}
	var files http.Handler
	if built {
		files = http.FileServerFS(dist)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			writeProblem(w, newProblem(http.StatusNotFound, "http.404", "no such API: "+r.URL.Path))
			return
		}
		if !built {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(notBuiltPage))
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" || p == "index.html" {
			http.ServeFileFS(w, r, dist, "index.html")
			return
		}
		if st, err := fs.Stat(dist, p); err != nil || st.IsDir() {
			http.ServeFileFS(w, r, dist, "index.html")
			return
		}
		files.ServeHTTP(w, r)
	})
}
```

`/index.html` を `http.ServeFileFS` に渡すと `/` へのリダイレクト（301）になるので、テストで `/index.html` を確かめるのは未ビルドのページだけにしてある。

`web/embed.go`:

```go
// Package web holds the GUI built into web/dist by make web.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist is the built GUI, with the files of web/dist at its root. Without
// make web it holds only .gitkeep.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded; Sub fails only on a bad name
	}
	return sub
}
```

`web/dist/.gitkeep` は空のファイル。`.gitignore` の `### GoReleaser ###` の `dist/` の下に足す:

```
!/web/dist/
/web/dist/*
!/web/dist/.gitkeep
```

`git status --ignored web` で `.gitkeep` が追跡され、ほかの dist のファイルが無視されることを確かめる。

- [ ] **Step 8: server.go と health.go を書く**

`internal/server/server.go`:

```go
// Package server is the HTTP server of hh ui: the REST API under /api, its
// events, the sync loop, and the built GUI.
package server

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/kukv/hedgehug/internal/auth"
	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// APIVersion is the version of the API in api/openapi.yaml. It is not the
// version of hedgehug, so that the file does not change with each build.
const APIVersion = "1"

// Options are what the server is made with.
type Options struct {
	// The fields of each request's ops.Session.
	Getenv     func(string) string
	Now        func() time.Time
	Keyring    auth.Keyring
	HTTPClient *http.Client
	Run        func(name string, args ...string) (stdout, stderr []byte, err error)

	Token   string    // the bearer token every /api request needs
	Version string    // of hedgehug, for GET /api/health
	Static  fs.FS     // the built GUI (web.Dist()); nil serves the page that says to build it
	Log     io.Writer // warnings and the errors of the loop, a line each; nil discards them
}

// Server is the server of hh ui. Make it with New.
type Server struct {
	opt  Options
	port atomic.Int32 // the port the Host check allows
	mux  *http.ServeMux
	api  huma.API

	logMu sync.Mutex
}

// New makes the server. Call SetPort with the port it listens on before
// serving.
func New(opt Options) *Server {
	if opt.Log == nil {
		opt.Log = io.Discard
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	s := &Server{opt: opt, mux: http.NewServeMux()}
	s.api = humago.New(s.mux, apiConfig())
	s.api.UseMiddleware(s.requireToken)
	s.register()
	s.mux.Handle("/", staticHandler(opt.Static))
	return s
}

// apiConfig is the huma config: OpenAPI 3.1, bearer auth, no docs or
// schema routes, no $schema links in the bodies.
func apiConfig() huma.Config {
	c := huma.DefaultConfig("Hedgehug", APIVersion)
	c.OpenAPIPath, c.DocsPath, c.SchemasPath = "", "", ""
	c.CreateHooks = nil
	c.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {Type: "http", Scheme: "bearer"},
	}
	c.Security = []map[string][]string{{"bearer": {}}}
	return c
}

// register adds every operation. Each later file adds its own register
// function here.
func (s *Server) register() {
	s.registerHealth()
}

// SetPort sets the port the Host check allows.
func (s *Server) SetPort(port int) { s.port.Store(int32(port)) } //nolint:gosec // a port fits in an int32

// Handler is the handler to serve: the Host check, then the API and the
// static files.
func (s *Server) Handler() http.Handler { return s.guardHost(s.mux) }

// API is the huma API, for the OpenAPI document.
func (s *Server) API() huma.API { return s.api }

// session is the ops.Session of one request, on the store named store ("" as
// config.Select picks). Its warnings go to the log; its writes tell the sync
// loop (Task 7).
func (s *Server) session(storeName string) *ops.Session {
	return &ops.Session{
		Getenv:     s.opt.Getenv,
		Now:        s.opt.Now,
		Keyring:    s.opt.Keyring,
		HTTPClient: s.opt.HTTPClient,
		Run:        s.opt.Run,
		StoreName:  storeName,
		Warn:       func(m string) { s.logf("warning: %s", m) },
		AfterWrite: func(*store.Store, config.StoreEntry) {},
	}
}

// logf writes a line to the log.
func (s *Server) logf(format string, args ...any) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	_, _ = fmt.Fprintf(s.opt.Log, format+"\n", args...)
}

// storeParam is the store a request is on: ?store=<name>, or as
// config.Select picks without it. The inputs of per-store operations embed it.
type storeParam struct {
	Store string `query:"store" doc:"The store; without it, the one hedgehug picks as the CLI does"`
}

// OpenAPIYAML is api/openapi.yaml: the OpenAPI document of the API.
func OpenAPIYAML() ([]byte, error) {
	return New(Options{}).API().OpenAPI().YAML()
}
```

`internal/server/health.go`:

```go
package server

import (
	"context"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
)

type healthOutput struct {
	Body struct {
		Version string `json:"version" doc:"The version of hedgehug"`
		PID     int    `json:"pid" doc:"The process of the server"`
	}
}

func (s *Server) registerHealth() {
	huma.Register(s.api, huma.Operation{
		OperationID: "get-health", Method: http.MethodGet, Path: "/api/health",
		Summary: "Check that the server is up", Tags: []string{"server"},
	}, func(context.Context, *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Version, out.Body.PID = s.opt.Version, os.Getpid()
		return out, nil
	})
}
```

- [ ] **Step 9: 生成のプログラムを書く**

`internal/server/gen.go`:

```go
package server

//go:generate go run ./openapigen -o ../../api/openapi.yaml
```

`internal/server/openapigen/main.go`:

```go
// Command openapigen writes the OpenAPI document of the server.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kukv/hedgehug/internal/server"
)

func main() {
	out := flag.String("o", "", "the file to write")
	flag.Parse()
	b, err := server.OpenAPIYAML()
	if err == nil {
		err = os.WriteFile(*out, b, 0o644) //nolint:gosec // a file of the repository
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "openapigen:", err)
		os.Exit(1)
	}
}
```

`Makefile` の `.PHONY` に `generate` を足し、`build:` の前に足す:

```make
# Write the generated files (api/openapi.yaml).
generate:
	go generate ./...
```

- [ ] **Step 10: 生成して通ることを確かめる**

```bash
mkdir -p api
go generate ./internal/server
go test ./internal/server/
```

Expected: PASS。`api/openapi.yaml` の先頭が `openapi: 3.1.0` で、`/api/health` と `bearer` のセキュリティがあり、`$schema` のフィールドがないこと（`grep -c '\$schema' api/openapi.yaml` が 0）を目で確かめる。

- [ ] **Step 11: 全体の検査とコミット**

Run: `go mod tidy && go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...`
Expected: すべて成功

```bash
git add go.mod go.sum .gitignore Makefile api/openapi.yaml web/embed.go web/dist/.gitkeep internal/server
git commit -m "feat(server): huma の API の土台、Host とトークンの検査、problem+json、埋め込みの配信を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: タスクの読み取り（一覧・ツリー・詳細・履歴）

**Files:**
- Create: `internal/server/tasks.go`, `internal/server/tasks_read_test.go`
- Modify: `internal/server/server.go`（`register` に `s.registerTaskReads()`）、`api/openapi.yaml`（生成）

**Interfaces:**
- Consumes: `ops.ParseQuery`、`ops.HasTerm`、`(*ops.Session).List/Tree/Show/Log`、`ops.TaskViewOf`、`ops.TreeViewOf`、`ops.ShowViewOf`、`ops.LogViewsOf`、`ops.ChecklistOf`、`ops.ProgressOf`
- Produces:
  - `type taskParam struct { storeParam; ID string \`path:"id"\` }`
  - `func listView(ix *index.Index, e *index.Entry) *ops.TaskView` — 一覧の1行（チェックリストと進捗を入れる）
  - `GET /api/tasks`、`GET /api/tasks/{id}`、`GET /api/tasks/{id}/history`

**型の注意:** ops の型（`TaskView`・`ShowView`・`LogView`）をそのまま応答に使う。huma はポインタのスカラーを nullable にし、埋め込みの構造体を平らにする。検証（`testServer.call`）が `null` やフィールドの欠けで失敗したら、ops の型に huma のタグ（`nullable:"true"` など）を足して直す。JSON の出力（CLI の `--json`）は変えない。足したタグは報告に書く。

- [ ] **Step 1: 失敗するテストを書く**

`internal/server/tasks_read_test.go`:

```go
package server

import (
	"net/url"
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

// add adds a task through ops, as the CLI would, and returns its ID.
func (ts *testServer) add(title, parent string) string {
	ts.t.Helper()
	res, err := ts.session().Add(ops.AddInput{Title: title, Parent: parent})
	if err != nil {
		ts.t.Fatal(err)
	}
	v, err := res.View()
	if err != nil {
		ts.t.Fatal(err)
	}
	return v.ID
}

type listBody struct {
	Tasks  []*ops.TaskView `json:"tasks"`
	Groups []struct {
		Key   string   `json:"key"`
		ULIDs []string `json:"ulids"`
	} `json:"groups"`
	Roots []*ops.TaskView `json:"roots"`
	Cycle []*ops.TaskView `json:"cycle"`
}

func TestListTasks(t *testing.T) {
	ts := newTestServer(t)
	ts.add("one", "")
	ts.add("two", "")
	var out listBody
	ts.call("GET", "/api/tasks", nil, 200, &out)
	if len(out.Tasks) != 2 || out.Roots != nil {
		t.Fatalf("tasks = %+v", out)
	}
	ts.call("GET", "/api/tasks?q="+url.QueryEscape("title:one"), nil, 200, &out)
	if len(out.Tasks) != 1 || out.Tasks[0].Title != "one" {
		t.Errorf("q=title:one: %+v", out.Tasks)
	}
}

func TestListTasksEmptyIsArray(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.call("GET", "/api/tasks", nil, 200, nil)
	b := readAll(t, resp)
	if string(b) != `{"tasks":[]}`+"\n" && string(b) != `{"tasks":[]}` {
		t.Errorf("body = %s, want an empty tasks array", b)
	}
}

func TestListTasksGroups(t *testing.T) {
	ts := newTestServer(t)
	ts.add("one", "")
	var out listBody
	ts.call("GET", "/api/tasks?q="+url.QueryEscape("group:status"), nil, 200, &out)
	if len(out.Groups) == 0 || len(out.Groups[0].ULIDs) != 1 {
		t.Errorf("groups = %+v", out.Groups)
	}
}

func TestTreeHasProgress(t *testing.T) {
	ts := newTestServer(t)
	res, err := ts.session().Add(ops.AddInput{Title: "p", Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	pv, _ := res.View()
	ts.add("child", pv.ID)
	var out listBody
	ts.call("GET", "/api/tasks?tree=true", nil, 200, &out)
	if len(out.Roots) != 1 || len(out.Roots[0].Children) != 1 || out.Tasks != nil {
		t.Fatalf("tree = %+v", out)
	}
	if p := out.Roots[0].Progress; p == nil || p.Total != 1 {
		t.Errorf("progress = %+v", p)
	}
}

func TestListHasChecklistAndProgress(t *testing.T) {
	ts := newTestServer(t)
	res, err := ts.session().Add(ops.AddInput{Title: "p", Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	pv, _ := res.View()
	id := ts.add("c", pv.ID)
	body := "- [x] a\n- [ ] b\n"
	if _, err := ts.session().Patch(id, ops.PatchInput{Body: &body}); err != nil {
		t.Fatal(err)
	}
	var out listBody
	ts.call("GET", "/api/tasks", nil, 200, &out)
	for _, v := range out.Tasks {
		switch v.ID {
		case pv.ID:
			if v.Progress == nil || v.Progress.Total != 1 {
				t.Errorf("project progress = %+v", v.Progress)
			}
		case id:
			if v.Checklist == nil || *v.Checklist != (ops.Count{Done: 1, Total: 2}) {
				t.Errorf("checklist = %+v", v.Checklist)
			}
		}
	}
}

func TestShowTask(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	var out ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &out)
	if out.TaskView == nil || out.ID != id || out.Comments == nil || len(out.History) == 0 {
		t.Errorf("show = %+v", out)
	}
	ulid := out.ULID
	ts.call("GET", "/api/tasks/"+ulid, nil, 200, &out)
	if out.ID != id {
		t.Errorf("by ULID: %s", out.ID)
	}
}

func TestShowTaskNotFound(t *testing.T) {
	ts := newTestServer(t)
	p := ts.problem("GET", "/api/tasks/ME-99", nil, 404)
	if p.Status != 404 || p.Type == "" || p.Detail == "" {
		t.Errorf("problem = %+v", p)
	}
}

func TestUnknownStoreIsProblem(t *testing.T) {
	ts := newTestServer(t)
	p := ts.problem("GET", "/api/tasks?store=nope", nil, 404)
	if p.Type == "internal" {
		t.Errorf("problem = %+v, want the store's not-found code", p)
	}
}

func TestBadQueryIsProblem(t *testing.T) {
	ts := newTestServer(t)
	p := ts.problem("GET", "/api/tasks?q="+url.QueryEscape("sort:"), nil, 400)
	if p.Type == "" {
		t.Errorf("problem = %+v", p)
	}
}

func TestTaskHistory(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	title := "two"
	if _, err := ts.session().Patch(id, ops.PatchInput{Assigns: []ops.Assign{{Key: "title", Op: "=", Value: title}}}); err != nil {
		t.Fatal(err)
	}
	var out []ops.LogView
	ts.call("GET", "/api/tasks/"+id+"/history", nil, 200, &out)
	if len(out) != 2 || out[0].Op == nil || *out[0].Op != "set" {
		t.Fatalf("history = %+v", out)
	}
	ts.call("GET", "/api/tasks/"+id+"/history?limit=1", nil, 200, &out)
	if len(out) != 1 {
		t.Errorf("limit=1: %d entries", len(out))
	}
	ts.problem("GET", "/api/tasks/"+id+"/history?limit=0", nil, 400)
}
```

`helpers_test.go` に足す:

```go
func readAll(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
```

存在しない Store の Code と状態（404 か 409 か）は `config.Select` のエラーを ops がどう分類するかで決まる。分類されず 500 になったら止めてコントローラーに報告する。`sort:`（値なし）が Usage にならないときは、`ops.ParseQuery` が Usage を返す別の誤りに置き換える。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/server/ -run 'TestList|TestTree|TestShow|TestUnknownStore|TestBadQuery|TestTaskHistory'`
Expected: FAIL（404 の problem。操作がまだない）

- [ ] **Step 3: tasks.go を書く**

```go
package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/ops"
)

// taskParam is the task a request is on: an ID, a former ID or a ULID, as
// the CLI resolves it.
type taskParam struct {
	storeParam
	ID string `path:"id" doc:"The ID, a former ID, or the ULID of the task"`
}

// listView is a row of a list: the task with its checklist and progress.
func listView(ix *index.Index, e *index.Entry) *ops.TaskView {
	v := ops.TaskViewOf(ix, e)
	v.Checklist = ops.ChecklistOf(e)
	v.Progress = ops.ProgressOf(ix, e)
	return v
}

type groupView struct {
	Key   string   `json:"key" doc:"The value of the group; empty for the tasks with none"`
	ULIDs []string `json:"ulids"`
}

type listInput struct {
	storeParam
	Q    string `query:"q" doc:"The query, as hh ls takes it; empty for every task"`
	Tree bool   `query:"tree" doc:"Return the tasks as a tree, as hh ls --tree"`
}

type listOutput struct {
	Body struct {
		Tasks  *[]*ops.TaskView `json:"tasks,omitempty" doc:"The tasks found, without tree"`
		Groups []groupView      `json:"groups,omitempty" doc:"The groups, when the query has group:"`
		Roots  *[]*ops.TaskView `json:"roots,omitempty" doc:"The roots of the tree, with tree"`
		Cycle  []*ops.TaskView  `json:"cycle,omitempty" doc:"The tasks in a parent cycle, with tree"`
	}
}

type showOutput struct{ Body *ops.ShowView }

type historyInput struct {
	taskParam
	Limit int `query:"limit" default:"50" minimum:"1" maximum:"1000"`
}

type historyOutput struct{ Body []ops.LogView }

func (s *Server) registerTaskReads() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-tasks", Method: http.MethodGet, Path: "/api/tasks",
		Summary: "List the tasks a query finds", Tags: []string{"tasks"},
	}, func(_ context.Context, in *listInput) (*listOutput, error) {
		q, err := ops.ParseQuery(in.Q)
		if err != nil {
			return nil, problemOf(err)
		}
		ss := s.session(in.Store)
		out := &listOutput{}
		if in.Tree {
			r, err := ss.Tree(q)
			if err != nil {
				return nil, problemOf(err)
			}
			roots := make([]*ops.TaskView, 0, len(r.Roots))
			for _, n := range r.Roots {
				roots = append(roots, ops.TreeViewOf(r.Index, n))
			}
			out.Body.Roots = &roots
			for _, n := range r.Cycle {
				out.Body.Cycle = append(out.Body.Cycle, ops.TreeViewOf(r.Index, n))
			}
			return out, nil
		}
		r, err := ss.List(q)
		if err != nil {
			return nil, problemOf(err)
		}
		tasks := make([]*ops.TaskView, 0, len(r.Entries))
		for _, e := range r.Entries {
			tasks = append(tasks, listView(r.Index, e))
		}
		out.Body.Tasks = &tasks
		if ops.HasTerm(q, "group") {
			for _, g := range r.Groups {
				gv := groupView{Key: g.Key, ULIDs: make([]string, len(g.Tasks))}
				for i, e := range g.Tasks {
					gv.ULIDs[i] = e.ULID.String()
				}
				out.Body.Groups = append(out.Body.Groups, gv)
			}
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-task", Method: http.MethodGet, Path: "/api/tasks/{id}",
		Summary: "Show a task, as hh show", Tags: []string{"tasks"},
	}, func(_ context.Context, in *taskParam) (*showOutput, error) {
		r, err := s.session(in.Store).Show(in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		return &showOutput{Body: ops.ShowViewOf(r)}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-task-history", Method: http.MethodGet, Path: "/api/tasks/{id}/history",
		Summary: "List the commits of a task, newest first", Tags: []string{"tasks"},
	}, func(_ context.Context, in *historyInput) (*historyOutput, error) {
		ix, entries, err := s.session(in.Store).Log([]string{in.ID}, in.Limit)
		if err != nil {
			return nil, problemOf(err)
		}
		return &historyOutput{Body: ops.LogViewsOf(ix, entries)}, nil
	})
}
```

`TreeViewOf` がチェックリストと進捗を入れていないとき（4b の実装を見る）は、`listView` と同じく入れる。`ShowViewOf` も同じ。`e.ULID.String()` の名前は `model.ULID` の定義に合わせる。`server.go` の `register` に `s.registerTaskReads()` を足す。

- [ ] **Step 4: 生成して通ることを確かめる**

Run: `go generate ./internal/server && go test ./internal/server/`
Expected: PASS（検証の失敗が出たら上の「型の注意」のとおり直す）

- [ ] **Step 5: 全体の検査とコミット**

Run: `go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...`
Expected: すべて成功

```bash
git add api/openapi.yaml internal/server internal/ops
git commit -m "feat(server): タスクの一覧・ツリー・詳細・履歴の API を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: タスクの書き込み（作成・更新・移動・削除・復元・完全削除・時点への復元）

**Files:**
- Create: `internal/server/tasks_write.go`, `internal/server/tasks_write_test.go`
- Modify: `internal/server/server.go`（`register` に `s.registerTaskWrites()`）、`api/openapi.yaml`（生成）

**Interfaces:**
- Consumes: Task 1 の `ops.Patch`、Task 3 の `taskParam`・`listView`、`(*ops.Session).Add/Move/Remove/Restore/PurgeTargets/Purge/RestoreAt`
- Produces:
  - `func commitPtr(hex string) *string` — `""` は nil
  - `func (s *Server) viewsOf(ss *ops.Session, ulids ...model.ULID) ([]*ops.TaskView, error)` — 書き込みの後のタスクを、開き直したインデックスから `listView` で返す
  - `type taskWriteOutput struct { Body taskWrite }`、`type taskWrite struct { Task *ops.TaskView \`json:"task"\`; Commit *string \`json:"commit"\` }` — Task 5 も使う
  - `func assignsOf(key string, v any) ([]ops.Assign, error)` — JSON の値を `ApplyAssign` の代入に直す（Task 5 は使わない）

- [ ] **Step 1: 失敗するテストを書く**

`internal/server/tasks_write_test.go`:

```go
package server

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

type taskWriteBody struct {
	Task   *ops.TaskView `json:"task"`
	Commit *string       `json:"commit"`
	Note   string        `json:"note"`
}

type affectedBody struct {
	Tasks  []*ops.TaskView `json:"tasks"`
	ULIDs  []string        `json:"ulids"`
	Commit *string         `json:"commit"`
}

func TestCreateTask(t *testing.T) {
	ts := newTestServer(t)
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one", "labels": []string{"a", "b"}}, 201, &out)
	if out.Task == nil || out.Task.ID != "ME-1" || out.Commit == nil || len(out.Task.Labels) != 2 {
		t.Fatalf("created = %+v", out)
	}
	ts.call("POST", "/api/tasks", map[string]any{"title": "p", "kind": "project"}, 201, &out)
	if out.Task.Kind != "project" {
		t.Errorf("kind = %q", out.Task.Kind)
	}
	ts.call("POST", "/api/tasks", map[string]any{"title": "c", "parent": "ME-2"}, 201, &out)
	if out.Task.Parent == nil {
		t.Errorf("parent = nil")
	}
	ts.problem("POST", "/api/tasks", map[string]any{}, 400)
}

func TestPatchTask(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	blocker := ts.add("blocker", "")
	var out taskWriteBody
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{
		"title": "two", "labels": []string{"x"}, "blocked_by": []string{blocker}, "body": "hello\n",
	}, 200, &out)
	if out.Task.Title != "two" || len(out.Task.Labels) != 1 || len(out.Task.BlockedBy) != 1 || out.Commit == nil {
		t.Fatalf("patched = %+v", out)
	}
	s, _ := ts.session().OpenStore()
	got, _ := s.Task(mustULID(t, out.Task.ULID))
	if got.Body != "hello\n" {
		t.Errorf("body = %q", got.Body)
	}
	// The same values again: no commit.
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"title": "two"}, 200, &out)
	if out.Commit != nil {
		t.Errorf("no change made commit %v", *out.Commit)
	}
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"labels": []string{}}, 200, &out)
	if len(out.Task.Labels) != 0 {
		t.Errorf("labels = %v, want none", out.Task.Labels)
	}
	if p := ts.problem("PATCH", "/api/tasks/"+id, map[string]any{}, 400); p.Type != "patch.empty" {
		t.Errorf("empty patch: %+v", p)
	}
	if p := ts.problem("PATCH", "/api/tasks/"+id, map[string]any{"status": "nope"}, 422); p.Type == "" {
		t.Errorf("bad status: %+v", p)
	}
	if p := ts.problem("PATCH", "/api/tasks/"+id, map[string]any{"props": map[string]any{"nope": "x"}}, 422); p.Type != "assign.unknown_key" {
		t.Errorf("unknown prop: %+v", p)
	}
}

func TestMoveTask(t *testing.T) {
	ts := newTestServer(t)
	a := ts.add("a", "")
	b := ts.add("b", "")
	var out taskWriteBody
	ts.call("POST", "/api/tasks/"+b+"/move", map[string]any{"parent": a}, 200, &out)
	if out.Task.Parent == nil || out.Commit == nil {
		t.Fatalf("moved = %+v", out)
	}
	ts.call("POST", "/api/tasks/"+b+"/move", map[string]any{"root": true}, 200, &out)
	if out.Task.Parent != nil {
		t.Errorf("parent = %v, want none", *out.Task.Parent)
	}
	ts.problem("POST", "/api/tasks/"+b+"/move", map[string]any{"root": true, "parent": a}, 400)
}

func TestRemoveRestoreTask(t *testing.T) {
	ts := newTestServer(t)
	p := ts.add("p", "")
	ts.add("c", p)
	var out affectedBody
	ts.call("DELETE", "/api/tasks/"+p, nil, 200, &out)
	if len(out.Tasks) != 2 || out.Tasks[0].DeletedAt == nil || out.Commit == nil {
		t.Fatalf("removed = %+v", out)
	}
	if pr := ts.problem("PATCH", "/api/tasks/"+p, map[string]any{"title": "x"}, 409); pr.Type != "task.deleted" {
		t.Errorf("patch of deleted: %+v", pr)
	}
	ts.call("POST", "/api/tasks/"+p+"/restore", nil, 200, &out)
	if len(out.Tasks) != 2 || out.Tasks[0].DeletedAt != nil {
		t.Errorf("restored = %+v", out)
	}
}

func TestPurgeNeedsConfirm(t *testing.T) {
	ts := newTestServer(t)
	p := ts.add("p", "")
	c := ts.add("c", p)
	var out affectedBody
	ts.call("DELETE", "/api/tasks/"+p, nil, 200, &out)
	pr := ts.problem("DELETE", "/api/tasks/"+p+"?purge=true", nil, 409)
	targets, _ := pr.Args["targets"].([]any)
	if pr.Type != "purge.unconfirmed" || len(targets) != 2 {
		t.Fatalf("unconfirmed = %+v", pr)
	}
	var ulids []string
	for _, x := range targets {
		m := x.(map[string]any)
		ulids = append(ulids, m["ulid"].(string))
		if m["id"] == "" || m["title"] == "" {
			t.Errorf("target = %v", m)
		}
	}
	// A wrong list is refused with the targets as they are.
	pr = ts.problem("DELETE", "/api/tasks/"+p+"?purge=true&confirm="+ulids[0], nil, 409)
	if pr.Type != "purge.unconfirmed" {
		t.Errorf("partial confirm: %+v", pr)
	}
	ts.call("DELETE", "/api/tasks/"+p+"?purge=true&confirm="+strings.Join(ulids, ","), nil, 200, &out)
	if len(out.ULIDs) != 2 || out.Commit == nil {
		t.Fatalf("purged = %+v", out)
	}
	ts.problem("GET", "/api/tasks/"+c, nil, 404)
}

func TestPurgeLiveTaskIsConflict(t *testing.T) {
	ts := newTestServer(t)
	p := ts.add("p", "")
	if pr := ts.problem("DELETE", "/api/tasks/"+p+"?purge=true", nil, 409); pr.Type == "purge.unconfirmed" {
		t.Errorf("a live task: %+v, want the refusal of ops", pr)
	}
}

func TestRestoreAtCommit(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	var hist []ops.LogView
	ts.call("GET", "/api/tasks/"+id+"/history", nil, 200, &hist)
	first := hist[0].Hash
	var out taskWriteBody
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"title": "two"}, 200, &out)
	ts.call("POST", "/api/tasks/"+id+"/history/"+first+"/restore", nil, 200, &out)
	if out.Task.Title != "one" || out.Commit == nil {
		t.Errorf("restored at = %+v", out)
	}
}

func TestAssignsOf(t *testing.T) {
	cases := []struct {
		v    any
		want []ops.Assign
	}{
		{"x", []ops.Assign{{Key: "k", Op: "=", Value: "x"}}},
		{float64(3), []ops.Assign{{Key: "k", Op: "=", Value: "3"}}},
		{true, []ops.Assign{{Key: "k", Op: "=", Value: "true"}}},
		{nil, []ops.Assign{{Key: "k", Op: "=", Value: ""}}},
		{[]any{"a", "b"}, []ops.Assign{{Key: "k", Op: "=", Value: ""}, {Key: "k", Op: "+=", Value: "a"}, {Key: "k", Op: "+=", Value: "b"}}},
		{[]any{}, []ops.Assign{{Key: "k", Op: "=", Value: ""}}},
	}
	for _, c := range cases {
		got, err := assignsOf("k", c.v)
		if err != nil || !slicesEqual(got, c.want) {
			t.Errorf("assignsOf(%v) = %v, %v; want %v", c.v, got, err, c.want)
		}
	}
	if _, err := assignsOf("k", map[string]any{}); err == nil {
		t.Error("an object is refused")
	}
}
```

`helpers_test.go` に足す:

```go
func mustULID(t *testing.T, s string) model.ULID {
	t.Helper()
	u, err := model.ParseULID(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func slicesEqual[T comparable](a, b []T) bool { return slices.Equal(a, b) }
```

（`model.ParseULID` の名前は `internal/model` の定義に合わせる。）

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/server/ -run 'TestCreate|TestPatchTask|TestMove|TestRemove|TestPurge|TestRestoreAt|TestAssignsOf'`
Expected: FAIL

- [ ] **Step 3: tasks_write.go を書く**

```go
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
)

// commitPtr is the commit of a write: nil when nothing was committed.
func commitPtr(hex string) *string {
	if hex == "" {
		return nil
	}
	return &hex
}

// viewsOf opens the index again, after a write, and returns the tasks as
// rows of a list.
func (s *Server) viewsOf(ss *ops.Session, ulids ...model.ULID) ([]*ops.TaskView, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return nil, err
	}
	out := make([]*ops.TaskView, 0, len(ulids))
	for _, u := range ulids {
		e, ok := ix.Task(u)
		if !ok {
			return nil, fmt.Errorf("task %s is not in the index after the write", u)
		}
		out = append(out, listView(ix, e))
	}
	return out, nil
}

// assignsOf turns the JSON value v of key into what hedgehug set takes: a
// scalar is key=value, null is key= (clear), a list is clear then key+=item
// for each.
func assignsOf(key string, v any) ([]ops.Assign, error) {
	scalar := func(x any) (string, error) {
		switch x := x.(type) {
		case string:
			return x, nil
		case bool:
			return strconv.FormatBool(x), nil
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64), nil
		}
		return "", fmt.Errorf("%s: %v is not a string, a number or a boolean", key, x)
	}
	switch x := v.(type) {
	case nil:
		return []ops.Assign{{Key: key, Op: "=", Value: ""}}, nil
	case []any:
		out := []ops.Assign{{Key: key, Op: "=", Value: ""}}
		for _, it := range x {
			s, err := scalar(it)
			if err != nil {
				return nil, err
			}
			out = append(out, ops.Assign{Key: key, Op: "+=", Value: s})
		}
		return out, nil
	}
	s, err := scalar(v)
	if err != nil {
		return nil, err
	}
	return []ops.Assign{{Key: key, Op: "=", Value: s}}, nil
}

// badRequest is a 400 request.invalid problem for err.
func badRequest(err error) *Problem { return newProblem(http.StatusBadRequest, "request.invalid", err.Error()) }

// fields are the fields hedgehug set changes, as POST and PATCH take them.
type fields struct {
	Title     *string        `json:"title,omitempty"`
	Kind      *string        `json:"kind,omitempty"`
	Status    *string        `json:"status,omitempty"`
	Assignees *[]string      `json:"assignees,omitempty" doc:"Replaces the assignees"`
	Watchers  *[]string      `json:"watchers,omitempty" doc:"Replaces the watchers"`
	Labels    *[]string      `json:"labels,omitempty" doc:"Replaces the labels"`
	Repos     *[]string      `json:"repos,omitempty" doc:"Replaces the repos"`
	BlockedBy *[]string      `json:"blocked_by,omitempty" doc:"Replaces the blockers: IDs or ULIDs"`
	Props     map[string]any `json:"props,omitempty" doc:"Property name to value; null clears it, a list replaces a multiple value"`
}

// assigns is f as hedgehug set's assignments, the properties by name.
func (f *fields) assigns() ([]ops.Assign, error) {
	var out []ops.Assign
	add := func(key string, v any) error {
		as, err := assignsOf(key, v)
		out = append(out, as...)
		return err
	}
	for _, sc := range []struct {
		key string
		v   *string
	}{{"title", f.Title}, {"kind", f.Kind}, {"status", f.Status}} {
		if sc.v != nil {
			if err := add(sc.key, *sc.v); err != nil {
				return nil, err
			}
		}
	}
	for _, l := range []struct {
		key string
		v   *[]string
	}{{"assignees", f.Assignees}, {"watchers", f.Watchers}, {"labels", f.Labels}, {"repos", f.Repos}, {"blocked_by", f.BlockedBy}} {
		if l.v != nil {
			items := make([]any, len(*l.v))
			for i, s := range *l.v {
				items[i] = s
			}
			if err := add(l.key, items); err != nil {
				return nil, err
			}
		}
	}
	names := make([]string, 0, len(f.Props))
	for n := range f.Props {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		if err := add(n, f.Props[n]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

type taskWrite struct {
	Task   *ops.TaskView `json:"task"`
	Commit *string       `json:"commit" doc:"The commit made; null when nothing changed"`
	Note   string        `json:"note,omitempty" doc:"What restore at a commit has to say, as hh restore --at prints it"`
}

type taskWriteOutput struct {
	Status int
	Body   taskWrite
}

type affectedWrite struct {
	Tasks  []*ops.TaskView `json:"tasks,omitempty" doc:"The task and the tasks under it, as they are now"`
	ULIDs  []string        `json:"ulids,omitempty" doc:"The tasks purged"`
	Commit *string         `json:"commit"`
}

type affectedOutput struct{ Body affectedWrite }

// writeOut is the output of a write to one task.
func (s *Server) writeOut(ss *ops.Session, u model.ULID, commit string, status int) (*taskWriteOutput, error) {
	vs, err := s.viewsOf(ss, u)
	if err != nil {
		return nil, problemOf(err)
	}
	return &taskWriteOutput{Status: status, Body: taskWrite{Task: vs[0], Commit: commitPtr(commit)}}, nil
}

// affectedOut is the output of rm and restore: the task, then the tasks
// under it.
func (s *Server) affectedOut(ss *ops.Session, r ops.AffectedResult) (*affectedOutput, error) {
	ulids := []model.ULID{r.Entry.ULID}
	for _, e := range r.Targets {
		if e.ULID != r.Entry.ULID {
			ulids = append(ulids, e.ULID)
		}
	}
	vs, err := s.viewsOf(ss, ulids...)
	if err != nil {
		return nil, problemOf(err)
	}
	return &affectedOutput{Body: affectedWrite{Tasks: vs, Commit: commitPtr(r.Commit)}}, nil
}

type createInput struct {
	storeParam
	Body struct {
		Title     string         `json:"title" minLength:"1"`
		Parent    string         `json:"parent,omitempty" doc:"The parent: an ID or a ULID"`
		Workspace string         `json:"workspace,omitempty" doc:"The workspace, by name or prefix"`
		Kind      string         `json:"kind,omitempty" doc:"project for hh add --kind project"`
		Status    string         `json:"status,omitempty"`
		Assignees []string       `json:"assignees,omitempty"`
		Labels    []string       `json:"labels,omitempty"`
		Props     map[string]any `json:"props,omitempty" doc:"Property name to value"`
	}
}

type patchInput struct {
	taskParam
	Body struct {
		fields
		Body *string `json:"body,omitempty" doc:"The body in Markdown"`
	}
}

type moveInput struct {
	taskParam
	Body struct {
		Parent    string `json:"parent,omitempty" doc:"The new parent: an ID or a ULID"`
		Root      bool   `json:"root,omitempty" doc:"Move to the top, with no parent"`
		Workspace string `json:"workspace,omitempty" doc:"Move to another workspace"`
		Before    string `json:"before,omitempty" doc:"Put it before this sibling"`
		After     string `json:"after,omitempty" doc:"Put it after this sibling"`
	}
}

type deleteInput struct {
	taskParam
	Purge   bool   `query:"purge" doc:"Remove the files for good; the task must be deleted already"`
	Confirm string `query:"confirm" doc:"With purge: the ULIDs the user agreed to purge, comma-separated, as purge.unconfirmed listed them"`
}

type restoreAtInput struct {
	taskParam
	Commit string `path:"commit" doc:"The commit to bring the task back to"`
}

func (s *Server) registerTaskWrites() {
	huma.Register(s.api, huma.Operation{
		OperationID: "create-task", Method: http.MethodPost, Path: "/api/tasks", DefaultStatus: http.StatusCreated,
		Summary: "Add a task, as hh add", Tags: []string{"tasks"},
	}, func(_ context.Context, in *createInput) (*taskWriteOutput, error) {
		b := &in.Body
		f := fields{Props: b.Props}
		if b.Assignees != nil {
			f.Assignees = &b.Assignees
		}
		if b.Labels != nil {
			f.Labels = &b.Labels
		}
		assigns, err := f.assigns()
		if err != nil {
			return nil, badRequest(err)
		}
		ss := s.session(in.Store)
		res, err := ss.Add(ops.AddInput{Title: b.Title, Parent: b.Parent, Workspace: b.Workspace, Kind: b.Kind, Status: b.Status, Assigns: assigns})
		if err != nil {
			return nil, problemOf(err)
		}
		return s.writeOut(ss, res.ULID, res.Commit, http.StatusCreated)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-task", Method: http.MethodPatch, Path: "/api/tasks/{id}",
		Summary: "Change the fields sent, in one commit, as hh set and hh edit", Tags: []string{"tasks"},
	}, func(_ context.Context, in *patchInput) (*taskWriteOutput, error) {
		assigns, err := in.Body.fields.assigns()
		if err != nil {
			return nil, badRequest(err)
		}
		ss := s.session(in.Store)
		res, err := ss.Patch(in.ID, ops.PatchInput{Assigns: assigns, Body: in.Body.Body})
		if err != nil {
			return nil, problemOf(err)
		}
		return s.writeOut(ss, res.ULID, res.Commit, http.StatusOK)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "move-task", Method: http.MethodPost, Path: "/api/tasks/{id}/move",
		Summary: "Move a task, as hh mv", Tags: []string{"tasks"},
	}, func(_ context.Context, in *moveInput) (*taskWriteOutput, error) {
		b := in.Body
		if b.Root && b.Parent != "" {
			return nil, badRequest(errors.New("root and parent cannot both be given"))
		}
		ss := s.session(in.Store)
		res, err := ss.Move(ops.MoveInput{Arg: in.ID, Workspace: b.Workspace, Parent: b.Parent, Root: b.Root, Before: b.Before, After: b.After})
		if err != nil {
			return nil, problemOf(err)
		}
		return s.writeOut(ss, res.ULID, res.Commit, http.StatusOK)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "restore-task", Method: http.MethodPost, Path: "/api/tasks/{id}/restore",
		Summary: "Restore a deleted task and the tasks under it, as hh restore", Tags: []string{"tasks"},
	}, func(_ context.Context, in *taskParam) (*affectedOutput, error) {
		ss := s.session(in.Store)
		res, err := ss.Restore(in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		return s.affectedOut(ss, res)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "delete-task", Method: http.MethodDelete, Path: "/api/tasks/{id}",
		Summary: "Delete a task and the tasks under it (hh rm), or purge them (hh purge)", Tags: []string{"tasks"},
	}, func(_ context.Context, in *deleteInput) (*affectedOutput, error) {
		ss := s.session(in.Store)
		if !in.Purge {
			res, err := ss.Remove(in.ID)
			if err != nil {
				return nil, problemOf(err)
			}
			return s.affectedOut(ss, res)
		}
		return s.purge(ss, in.ID, in.Confirm)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "restore-task-at", Method: http.MethodPost, Path: "/api/tasks/{id}/history/{commit}/restore",
		Summary: "Bring a task back to how it was at a commit, as hh restore --at", Tags: []string{"tasks"},
	}, func(_ context.Context, in *restoreAtInput) (*taskWriteOutput, error) {
		ss := s.session(in.Store)
		res, err := ss.RestoreAt(in.ID, in.Commit)
		if err != nil {
			return nil, problemOf(err)
		}
		out, err := s.writeOut(ss, res.ULID, res.Commit, http.StatusOK)
		if err != nil {
			return nil, err
		}
		out.Body.Note = res.Note
		return out, nil
	})
}

// purge purges the task id after the confirmation: without confirm, or
// with a list that is not the tasks to purge, it is refused with
// purge.unconfirmed and those tasks, for the GUI to ask the user.
func (s *Server) purge(ss *ops.Session, id, confirm string) (*affectedOutput, error) {
	ix, _, targets, err := ss.PurgeTargets(id)
	if err != nil {
		return nil, problemOf(err)
	}
	var confirmed []model.ULID
	for _, c := range strings.Split(confirm, ",") {
		if c == "" {
			continue
		}
		u, err := model.ParseULID(c)
		if err != nil {
			return nil, badRequest(fmt.Errorf("confirm: %w", err))
		}
		confirmed = append(confirmed, u)
	}
	if !sameULIDs(targets, confirmed) {
		return nil, unconfirmed(ix, targets)
	}
	res, err := ss.Purge(id, confirmed)
	if ops.AsError(err) != nil && ops.AsError(err).Code == "purge.changed" {
		ix, _, targets, err2 := ss.PurgeTargets(id)
		if err2 != nil {
			return nil, problemOf(err2)
		}
		return nil, unconfirmed(ix, targets)
	}
	if err != nil {
		return nil, problemOf(err)
	}
	out := &affectedOutput{Body: affectedWrite{Commit: commitPtr(res.Commit)}}
	for _, e := range res.Targets {
		out.Body.ULIDs = append(out.Body.ULIDs, e.ULID.String())
	}
	return out, nil
}

// unconfirmed is the purge.unconfirmed problem with the tasks to purge.
func unconfirmed(ix *index.Index, targets []*index.Entry) *Problem {
	ts := make([]map[string]string, len(targets))
	for i, e := range targets {
		v := ops.TaskViewOf(ix, e)
		ts[i] = map[string]string{"ulid": v.ULID, "id": v.ID, "title": v.Title}
	}
	p := newProblem(http.StatusConflict, "purge.unconfirmed", fmt.Sprintf("purging %d tasks for good needs a confirmation", len(targets)))
	p.Args = map[string]any{"targets": ts}
	return p
}

// sameULIDs reports whether confirmed holds the ULIDs of targets, in any
// order.
func sameULIDs(targets []*index.Entry, confirmed []model.ULID) bool {
	if len(targets) != len(confirmed) {
		return false
	}
	for _, e := range targets {
		if !slices.Contains(confirmed, e.ULID) {
			return false
		}
	}
	return true
}
```

`PurgeTargets` の戻り値の `targets` にタスク自身が含まれるか（`TestPurgeNeedsConfirm` は親と子の2件を期待）を ops で確かめ、含まれないときは `[]*index.Entry{e}` を前に足してから比べる。`model.ParseULID`・`e.ULID.String()` の名前は定義に合わせる（import に `internal/index` を足す）。`register` に `s.registerTaskWrites()` を足す。

- [ ] **Step 4: 生成して通ることを確かめる**

Run: `go generate ./internal/server && go test ./internal/server/`
Expected: PASS

- [ ] **Step 5: 全体の検査とコミット**

Run: `go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...`
Expected: すべて成功

```bash
git add api/openapi.yaml internal/server internal/ops
git commit -m "feat(server): タスクの作成・更新・移動・削除・復元・完全削除の API を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: コメント・ウォッチ・取り消し・通知・アクティビティ

**Files:**
- Create: `internal/server/comments.go`, `internal/server/activity.go`, `internal/server/comments_test.go`, `internal/server/activity_test.go`
- Modify: `internal/server/server.go`（`register` に `s.registerComments()`・`s.registerActivity()`）、`api/openapi.yaml`（生成）

**Interfaces:**
- Consumes: Task 3 の `taskParam`、Task 4 の `taskWriteOutput`・`writeOut`・`commitPtr`、`(*ops.Session).CommentAdd/CommentEdit/CommentRemove/Watch/Undo/InboxList/InboxRead/InboxRevert/Log/Open/Resolve`、`ops.LogViewsOf`、`ops.UndoSubject`
- Produces: `POST /api/tasks/{id}/comments`、`PATCH|DELETE /api/tasks/{id}/comments/{cid}`、`PUT|DELETE /api/tasks/{id}/watchers/me`、`POST /api/undo`、`GET /api/notifications`、`PATCH /api/notifications/{id}`、`POST /api/notifications/{id}/revert`、`GET /api/activity`

**テストのアカウントと通知:** ウォッチと取り消しは書き手のアカウントが要る。`internal/ops` のテスト（`useAccount`・`addItem`）と同じ作り方の道具を `helpers_test.go` に足す。トークンは要らない（GitHub に通信しない）:

```go
// withAccount makes github.com/alice the account of config.yaml, as
// ops's useAccount does.
func (ts *testServer) withAccount() {
	ts.t.Helper()
	_, p, err := ts.session().Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	f, err := os.OpenFile(p.ConfigFile, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		ts.t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString("accounts:\n  - name: alice\n    host: github.com\n    login: alice\n"); err != nil {
		ts.t.Fatal(err)
	}
}

// accountID is the ID of the account withAccount made, as the commits
// carry it in Hedgehug-Author.
func (ts *testServer) accountID() string {
	ts.t.Helper()
	c, _, err := ts.session().Config()
	if err != nil || len(c.Accounts) == 0 {
		ts.t.Fatalf("no account: %v", err)
	}
	return c.Accounts[0].ID()
}

// notifyMe puts it into the inbox of the store "me", as a sync would, and
// returns its ULID.
func (ts *testServer) notifyMe(it inbox.Item) string {
	ts.t.Helper()
	s, err := ts.session().OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	added, err := inbox.Open(s).Add([]inbox.Item{it})
	if err != nil || len(added) != 1 {
		ts.t.Fatalf("Add item = %v, %v", added, err)
	}
	return added[0].ULID.String()
}

// overwrittenItem is the item a sync makes when it overwrote field of the
// task (ulid, id) of mine; value is the JSON of my value.
func overwrittenItem(ulid model.ULID, id, field, value string) inbox.Item {
	return inbox.Item{
		Kind: inbox.Overwritten, Task: ulid, ID: id, Path: model.TaskPath(ulid), Commit: "c1",
		Time:   time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC),
		Detail: json.RawMessage(`{"field":"` + field + `","value":` + value + `}`),
	}
}
```

`InboxList` は `c.Me()` の通知だけを出すかを ops で確かめる（`TestInboxListNewestFirstWithWordsAndJSON` はアカウントなしで項目を出しているので、`To` を空のまま足してよいはず）。

- [ ] **Step 1: 失敗するテストを書く**

`internal/server/comments_test.go`:

```go
package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

type commentBody struct {
	Comment struct {
		ULID string `json:"ulid"`
		Body string `json:"body"`
	} `json:"comment"`
	Commit *string `json:"commit"`
}

func TestComments(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	var out commentBody
	ts.call("POST", "/api/tasks/"+id+"/comments", map[string]any{"body": "first"}, 201, &out)
	if out.Comment.ULID == "" || out.Comment.Body != "first" || out.Commit == nil {
		t.Fatalf("added = %+v", out)
	}
	cid := out.Comment.ULID
	ts.call("PATCH", "/api/tasks/"+id+"/comments/"+cid, map[string]any{"body": "edited"}, 200, &out)
	if out.Comment.Body != "edited" {
		t.Errorf("edited = %+v", out)
	}
	var show ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if len(show.Comments) != 1 || show.Comments[0].Body != "edited" {
		t.Errorf("comments = %+v", show.Comments)
	}
	// A comment of another task is not found under this one.
	other := ts.add("two", "")
	ts.problem("PATCH", "/api/tasks/"+other+"/comments/"+cid, map[string]any{"body": "x"}, 404)
	ts.call("DELETE", "/api/tasks/"+id+"/comments/"+cid, nil, 200, &out)
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if len(show.Comments) != 0 {
		t.Errorf("comments after delete = %+v", show.Comments)
	}
	ts.problem("POST", "/api/tasks/"+id+"/comments", map[string]any{"body": ""}, 400)
}

func TestWatchMe(t *testing.T) {
	ts := newTestServer(t)
	ts.withAccount()
	id := ts.add("one", "")
	var out taskWriteBody
	ts.call("PUT", "/api/tasks/"+id+"/watchers/me", nil, 200, &out)
	var show ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if len(show.Watchers) != 1 {
		t.Fatalf("watchers = %v", show.Watchers)
	}
	ts.call("DELETE", "/api/tasks/"+id+"/watchers/me", nil, 200, &out)
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if len(show.Watchers) != 0 {
		t.Errorf("watchers = %v", show.Watchers)
	}
}
```

`internal/server/activity_test.go`:

```go
package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

func TestUndo(t *testing.T) {
	ts := newTestServer(t)
	ts.withAccount()
	id := ts.add("one", "")
	var w taskWriteBody
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"title": "two"}, 200, &w)
	var out struct {
		Undone ops.LogView `json:"undone"`
		Commit *string     `json:"commit"`
		Kept   []any       `json:"kept"`
	}
	ts.call("POST", "/api/undo", nil, 200, &out)
	if out.Commit == nil || out.Undone.Op == nil || *out.Undone.Op != "set" || out.Kept == nil {
		t.Fatalf("undo = %+v", out)
	}
	var show ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if show.Title != "one" {
		t.Errorf("title = %q", show.Title)
	}
}

func TestUndoWithoutAccountIsConflict(t *testing.T) {
	ts := newTestServer(t)
	if p := ts.problem("POST", "/api/undo", nil, 409); p.Type != "account.none" {
		t.Errorf("problem = %+v", p)
	}
}

func TestActivity(t *testing.T) {
	ts := newTestServer(t)
	ts.withAccount()
	ts.add("one", "")
	ts.add("two", "")
	var out []ops.LogView
	ts.call("GET", "/api/activity", nil, 200, &out)
	if len(out) < 3 { // the store's first commit and the two adds
		t.Fatalf("activity = %d entries", len(out))
	}
	ts.call("GET", "/api/activity?limit=1", nil, 200, &out)
	if len(out) != 1 || out[0].Op == nil || *out[0].Op != "add" {
		t.Errorf("limit=1: %+v", out)
	}
	ts.call("GET", "/api/activity?sync=only", nil, 200, &out)
	if len(out) != 0 {
		t.Errorf("sync=only: %d entries, want none", len(out))
	}
	ts.call("GET", "/api/activity?author="+ts.accountID(), nil, 200, &out)
	if len(out) != 2 {
		t.Errorf("author: %d entries, want the two adds", len(out))
	}
	ts.problem("GET", "/api/activity?sync=maybe", nil, 400)
}

func TestNotifications(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("mine", "")
	var show ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	u := mustULID(t, show.ULID)
	// Another writer's title won over mine ("first"): the sync told me.
	nid := ts.notifyMe(overwrittenItem(u, id, "title", `"first"`))
	var items []struct {
		ULID string `json:"ulid"`
		Kind string `json:"kind"`
		Read bool   `json:"read"`
	}
	ts.call("GET", "/api/notifications", nil, 200, &items)
	if len(items) != 1 || items[0].ULID != nid || items[0].Read {
		t.Fatalf("unread = %+v", items)
	}
	ts.call("PATCH", "/api/notifications/"+nid, map[string]any{"read": true}, 204, nil)
	ts.call("GET", "/api/notifications", nil, 200, &items)
	if len(items) != 0 {
		t.Errorf("unread after read = %+v", items)
	}
	ts.call("GET", "/api/notifications?all=true", nil, 200, &items)
	if len(items) != 1 || !items[0].Read {
		t.Errorf("all = %+v", items)
	}
	if p := ts.problem("PATCH", "/api/notifications/"+nid, map[string]any{"read": false}, 400); p.Type != "notification.unread" {
		t.Errorf("unread: %+v", p)
	}
	var w taskWriteBody
	ts.call("POST", "/api/notifications/"+nid+"/revert", nil, 200, &w)
	if w.Task == nil || w.Task.Title != "first" || w.Commit == nil {
		t.Errorf("revert = %+v", w)
	}
	// A mention cannot be reverted.
	mention := ts.notifyMe(inbox.Item{Kind: inbox.Mention, Task: u, ID: id, Commit: "c2", Time: time.Now()})
	if p := ts.problem("POST", "/api/notifications/"+mention+"/revert", nil, 409); p.Type == "" {
		t.Errorf("revert of a mention: %+v", p)
	}
	ts.problem("PATCH", "/api/notifications/"+model.NewULID().String(), map[string]any{"read": true}, 404)
}
```

（import に `time`・`internal/inbox`・`internal/model` を足す。）revert できない通知と存在しない通知の Kind（409・404 を期待している）は ops の `InboxRevertOf`・`InboxReadOf` が返すものを確かめ、違えばテストの期待をその状態に合わせて報告に書く。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/server/ -run 'TestComments|TestWatchMe|TestUndo|TestActivity|TestNotifications'`
Expected: FAIL

- [ ] **Step 3: comments.go を書く**

```go
package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
)

type commentParam struct {
	taskParam
	CID string `path:"cid" doc:"The ULID of the comment"`
}

type commentInput struct {
	taskParam
	Body struct {
		Body string `json:"body" minLength:"1" doc:"The comment in Markdown"`
	}
}

type commentEditInput struct {
	commentParam
	Body struct {
		Body string `json:"body" minLength:"1"`
	}
}

type commentWrite struct {
	Comment ops.CommentView `json:"comment"`
	Commit  *string         `json:"commit"`
}

type commentOutput struct {
	Status int
	Body   commentWrite
}

// commentOut is the output of a write to a comment.
func commentOut(r ops.CommentResult, status int) *commentOutput {
	out := &commentOutput{Status: status, Body: commentWrite{Commit: commitPtr(r.Commit)}}
	if c := r.Comment; c != nil {
		out.Body.Comment = ops.CommentView{ULID: c.ULID.String(), Author: c.Author, Created: c.Created, Body: c.Body}
	}
	return out
}

// taskULID resolves the task of a request.
func taskULID(ss *ops.Session, id string) (model.ULID, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return model.ULID{}, err
	}
	e, err := ss.Resolve(ix, id)
	if err != nil {
		return model.ULID{}, err
	}
	return e.ULID, nil
}

// commentOf refuses a comment that is not of the task of the path.
func commentOf(ss *ops.Session, id, cid string) error {
	u, err := taskULID(ss, id)
	if err != nil {
		return problemOf(err)
	}
	ces, err := ss.OpenComment(cid)
	if err != nil {
		return problemOf(err)
	}
	if ces.Ref.Task != u {
		return newProblem(http.StatusNotFound, "comment.not_found", "comment "+cid+" is not a comment of "+id)
	}
	return nil
}

type watchOutput = taskWriteOutput

func (s *Server) registerComments() {
	huma.Register(s.api, huma.Operation{
		OperationID: "add-comment", Method: http.MethodPost, Path: "/api/tasks/{id}/comments", DefaultStatus: http.StatusCreated,
		Summary: "Add a comment, as hh comment add", Tags: []string{"comments"},
	}, func(_ context.Context, in *commentInput) (*commentOutput, error) {
		r, err := s.session(in.Store).CommentAdd(in.ID, in.Body.Body)
		if err != nil {
			return nil, problemOf(err)
		}
		return commentOut(r, http.StatusCreated), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "edit-comment", Method: http.MethodPatch, Path: "/api/tasks/{id}/comments/{cid}",
		Summary: "Change a comment, as hh comment edit", Tags: []string{"comments"},
	}, func(_ context.Context, in *commentEditInput) (*commentOutput, error) {
		ss := s.session(in.Store)
		if err := commentOf(ss, in.ID, in.CID); err != nil {
			return nil, err
		}
		r, err := ss.CommentEdit(in.CID, in.Body.Body)
		if err != nil {
			return nil, problemOf(err)
		}
		return commentOut(r, http.StatusOK), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-comment", Method: http.MethodDelete, Path: "/api/tasks/{id}/comments/{cid}",
		Summary: "Remove a comment, as hh comment rm", Tags: []string{"comments"},
	}, func(_ context.Context, in *commentParam) (*commentOutput, error) {
		ss := s.session(in.Store)
		if err := commentOf(ss, in.ID, in.CID); err != nil {
			return nil, err
		}
		r, err := ss.CommentRemove(in.CID)
		if err != nil {
			return nil, problemOf(err)
		}
		return commentOut(r, http.StatusOK), nil
	})

	for _, w := range []struct {
		method, id, summary string
		on                  bool
	}{
		{http.MethodPut, "watch-task", "Watch a task, as hh watch", true},
		{http.MethodDelete, "unwatch-task", "Stop watching a task, as hh unwatch", false},
	} {
		huma.Register(s.api, huma.Operation{
			OperationID: w.id, Method: w.method, Path: "/api/tasks/{id}/watchers/me",
			Summary: w.summary, Tags: []string{"tasks"},
		}, func(_ context.Context, in *taskParam) (*watchOutput, error) {
			ss := s.session(in.Store)
			r, err := ss.Watch(in.ID, w.on)
			if err != nil {
				return nil, problemOf(err)
			}
			return s.writeOut(ss, r.ULID, r.Commit, http.StatusOK)
		})
	}
}
```

`CommentResult.Comment` が削除のときに nil か（中身があるか）を ops で確かめ、`DELETE` の応答の `comment` は削除前のコメントにする（nil なら `OpenComment` で先に読んだものを使う）。`CommentView` のフィールドは `internal/ops/read.go` の定義どおり。

- [ ] **Step 4: activity.go を書く**

```go
package server

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/ops"
)

type undoKept struct {
	Path   string `json:"path"`
	Task   string `json:"task,omitempty" doc:"The ID of the task; empty for other files"`
	Field  string `json:"field"`
	Author string `json:"author"`
}

type undoOutput struct {
	Body struct {
		Undone ops.LogView `json:"undone" doc:"The commit taken back"`
		Commit *string     `json:"commit"`
		Kept   []undoKept  `json:"kept" doc:"What others changed after it, which undo kept"`
	}
}

type activityInput struct {
	storeParam
	Limit  int    `query:"limit" default:"100" minimum:"1" maximum:"1000"`
	Author string `query:"author" doc:"Only the commits of this account (its ID)"`
	Sync   string `query:"sync" enum:"only,exclude" doc:"only: the merges and repairs of syncs; exclude: all but them"`
}

type activityOutput struct{ Body []ops.LogView }

type notificationsInput struct {
	All bool `query:"all" doc:"Include the items read"`
}

type notification struct {
	ops.InboxItemView
	Read bool `json:"read"`
}

type notificationsOutput struct{ Body []notification }

type notificationParam struct {
	ID string `path:"id" doc:"The ULID of the item"`
}

type notificationPatchInput struct {
	notificationParam
	Body struct {
		Read bool `json:"read" doc:"true marks it read; items cannot be marked unread"`
	}
}

// syncOps are the ops of the commits a sync makes.
var syncOps = []string{"merge", "repair"}

func (s *Server) registerActivity() {
	huma.Register(s.api, huma.Operation{
		OperationID: "undo", Method: http.MethodPost, Path: "/api/undo",
		Summary: "Take back my last operation, as hh undo", Tags: []string{"tasks"},
	}, func(_ context.Context, in *storeParam) (*undoOutput, error) {
		ss := s.session(in.Store)
		r, err := ss.Undo()
		if err != nil {
			return nil, problemOf(err)
		}
		_, ix, err := ss.Open()
		if err != nil {
			return nil, problemOf(err)
		}
		out := &undoOutput{}
		out.Body.Undone = ops.LogViewsOf(ix, []gitrepo.LogEntry{r.Commit})[0]
		out.Body.Commit = commitPtr(r.NewCommit)
		out.Body.Kept = []undoKept{}
		for _, k := range r.Kept {
			out.Body.Kept = append(out.Body.Kept, undoKept{k.Path, k.Task, k.Field, k.Author})
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "list-activity", Method: http.MethodGet, Path: "/api/activity",
		Summary: "List the commits of the store, newest first, as hh log", Tags: []string{"activity"},
	}, func(_ context.Context, in *activityInput) (*activityOutput, error) {
		ix, entries, err := s.session(in.Store).Log(nil, 0)
		if err != nil {
			return nil, problemOf(err)
		}
		var kept []gitrepo.LogEntry
		for _, e := range entries {
			op, _ := e.Trailer("Hedgehug-Op")
			author, _ := e.Trailer("Hedgehug-Author")
			isSync := slices.Contains(syncOps, op)
			switch {
			case in.Sync == "only" && !isSync, in.Sync == "exclude" && isSync:
				continue
			case in.Author != "" && author != in.Author:
				continue
			}
			kept = append(kept, e)
			if len(kept) == in.Limit {
				break
			}
		}
		return &activityOutput{Body: ops.LogViewsOf(ix, kept)}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "list-notifications", Method: http.MethodGet, Path: "/api/notifications",
		Summary: "List the inbox of every store, newest first, as hh inbox", Tags: []string{"notifications"},
	}, func(_ context.Context, in *notificationsInput) (*notificationsOutput, error) {
		ss := s.session("")
		_, unread, err := ss.InboxList(false)
		if err != nil {
			return nil, problemOf(err)
		}
		items := unread
		if in.All {
			if _, items, err = ss.InboxList(true); err != nil {
				return nil, problemOf(err)
			}
		}
		isUnread := map[string]bool{}
		for _, it := range unread {
			isUnread[it.ULID.String()] = true
		}
		out := &notificationsOutput{Body: make([]notification, len(items))}
		for i, it := range items {
			out.Body[i] = notification{InboxItemView: it, Read: !isUnread[it.ULID.String()]}
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-notification", Method: http.MethodPatch, Path: "/api/notifications/{id}",
		Summary: "Mark an item read, as hh inbox read", Tags: []string{"notifications"},
	}, func(_ context.Context, in *notificationPatchInput) (*struct{}, error) {
		if !in.Body.Read {
			return nil, newProblem(http.StatusBadRequest, "notification.unread", "an item cannot be marked unread")
		}
		if _, err := s.session("").InboxRead([]string{in.ID}, false); err != nil {
			return nil, problemOf(err)
		}
		return nil, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "revert-notification", Method: http.MethodPost, Path: "/api/notifications/{id}/revert",
		Summary: "Write my overwritten value back, as hh inbox revert", Tags: []string{"notifications"},
	}, func(_ context.Context, in *notificationParam) (*taskWriteOutput, error) {
		ss := s.session("")
		r, err := ss.InboxRevert(in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		// The item may be of any store; the task is in the store that
		// InboxRevert wrote to, which r.Index is the index of.
		e, ok := r.Index.Task(r.ULID)
		if !ok {
			return nil, problemOf(errors.New("the task reverted is not in the index"))
		}
		return &taskWriteOutput{Status: http.StatusOK, Body: taskWrite{Task: listView(r.Index, e), Commit: commitPtr(r.Commit)}}, nil
	})
}
```

`PATCH /notifications/{id}` の応答は 204 になる（huma は nil の出力を 204 にする）。`InboxRead` が既に既読の項目や存在しない項目に何を返すかを ops で確かめ、存在しない項目は 404 になることをテストに足す。`r.Index` が書き込み前のインデックスで、revert の後の値を持たないときは、Store を開き直して `listView` を作る（`InboxRevertOf` がどの Store に書いたかを返さないなら、止めてコントローラーに報告する）。

- [ ] **Step 5: 生成して通ることを確かめる**

Run: `go generate ./internal/server && go test ./internal/server/`
Expected: PASS

- [ ] **Step 6: 全体の検査とコミット**

Run: `go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...`
Expected: すべて成功

```bash
git add api/openapi.yaml internal/server internal/ops
git commit -m "feat(server): コメント・ウォッチ・取り消し・通知・アクティビティの API を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: ワークスペース・スキーマ・Store・保守・保存済みビュー

**Files:**
- Create: `internal/server/admin.go`, `internal/server/views.go`, `internal/server/admin_test.go`, `internal/server/views_test.go`
- Modify: `internal/server/server.go`（`register` に `s.registerAdmin()`・`s.registerViews()`）、`api/openapi.yaml`（生成）

**Interfaces:**
- Consumes: Task 1 の `ops.SchemaViewOf`、Task 4 の `listView`・`commitPtr`・`badRequest`、`(*ops.Session).WorkspaceList/WorkspaceAdd/WorkspaceRename/WorkspacePrefix/WorkspaceRemove/SchemaCheck/Migrate/StoreList/StoreInit/Doctor/ViewList/ViewGet/ViewSave/ViewRemove/ViewRename/Open/Config`、`ops.WorkspaceViewOf`、`ops.SavedViewOf`
- Produces: `GET|POST /api/workspaces`、`PATCH|DELETE /api/workspaces/{id}`、`GET /api/schema`、`GET /api/schema/check`、`POST /api/schema/migrate`、`GET|POST /api/stores`、`GET /api/doctor`、`POST /api/doctor/fix`、`GET /api/views`、`GET|PUT|PATCH|DELETE /api/views/{name}`

- [ ] **Step 1: 失敗するテストを書く**

`internal/server/admin_test.go`:

```go
package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

type workspaceBody struct {
	Workspace ops.WorkspaceView `json:"workspace"`
	Commit    *string           `json:"commit"`
	IDs       []struct {
		Old string `json:"old"`
		New string `json:"new"`
	} `json:"ids"`
}

func TestWorkspaces(t *testing.T) {
	ts := newTestServer(t)
	ts.add("one", "")
	var list []ops.WorkspaceView
	ts.call("GET", "/api/workspaces", nil, 200, &list)
	if len(list) != 1 || list[0].IDPrefix != "ME" || list[0].Tasks != 1 {
		t.Fatalf("workspaces = %+v", list)
	}
	var out workspaceBody
	ts.call("POST", "/api/workspaces", map[string]any{"name": "web", "prefix": "WEB"}, 201, &out)
	if out.Workspace.Name != "web" || out.Commit == nil {
		t.Fatalf("added = %+v", out)
	}
	ts.call("PATCH", "/api/workspaces/web", map[string]any{"name": "site"}, 200, &out)
	if out.Workspace.Name != "site" {
		t.Errorf("renamed = %+v", out)
	}
	ts.call("PATCH", "/api/workspaces/me", map[string]any{"prefix": "MY"}, 200, &out)
	if out.Workspace.IDPrefix != "MY" || len(out.IDs) != 1 || out.IDs[0].New != "MY-1" {
		t.Errorf("prefix = %+v", out)
	}
	if p := ts.problem("PATCH", "/api/workspaces/me", map[string]any{"name": "a", "prefix": "B"}, 400); p.Type != "workspace.one_change" {
		t.Errorf("both: %+v", p)
	}
	ts.problem("PATCH", "/api/workspaces/me", map[string]any{}, 400)
	ts.call("DELETE", "/api/workspaces/site", nil, 200, &out)
	ts.call("GET", "/api/workspaces", nil, 200, &list)
	if len(list) != 1 {
		t.Errorf("after delete = %+v", list)
	}
}

func TestSchema(t *testing.T) {
	ts := newTestServer(t)
	var sc ops.SchemaView
	ts.call("GET", "/api/schema", nil, 200, &sc)
	if len(sc.Statuses) == 0 {
		t.Fatalf("schema = %+v", sc)
	}
	var check struct {
		Findings []map[string]any `json:"findings"`
	}
	ts.call("GET", "/api/schema/check", nil, 200, &check)
	if check.Findings == nil || len(check.Findings) != 0 {
		t.Errorf("check = %+v", check)
	}
	id := ts.add("one", "")
	var show ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	old := show.Status // the status the new task has
	var mig struct {
		Changed []*ops.TaskView `json:"changed"`
		Skipped []string        `json:"skipped"`
		Commit  *string         `json:"commit"`
	}
	ts.call("POST", "/api/schema/migrate", map[string]any{"target": "status", "old": old, "new": "renamed", "dry_run": true}, 200, &mig)
	if mig.Commit != nil || len(mig.Changed) != 1 {
		t.Errorf("dry run = %+v", mig)
	}
	ts.call("POST", "/api/schema/migrate", map[string]any{"target": "status", "old": old, "new": "renamed"}, 200, &mig)
	if mig.Commit == nil || mig.Changed[0].Status != "renamed" {
		t.Errorf("migrate = %+v", mig)
	}
	ts.problem("POST", "/api/schema/migrate", map[string]any{"target": "value", "old": "a", "new": "b"}, 400) // value needs prop
	ts.problem("POST", "/api/schema/migrate", map[string]any{"target": "nope", "old": "a", "new": "b"}, 400)
}

func TestStores(t *testing.T) {
	ts := newTestServer(t)
	var list []struct {
		ops.StoreView
		Default bool `json:"default"`
	}
	ts.call("GET", "/api/stores", nil, 200, &list)
	if len(list) != 1 || list[0].Name != "me" || !list[0].Default {
		t.Fatalf("stores = %+v", list)
	}
	var made struct {
		Name      string   `json:"name"`
		Path      string   `json:"path"`
		Workspace struct{ Name string } `json:"workspace"`
	}
	ts.call("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TM"}, 201, &made)
	if made.Name != "team" || made.Path == "" {
		t.Fatalf("made = %+v", made)
	}
	var tasks listBody
	ts.call("GET", "/api/tasks?store=team", nil, 200, &tasks)
	if len(tasks.Tasks) != 0 {
		t.Errorf("tasks of team = %+v", tasks.Tasks)
	}
	ts.problem("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TX"}, 409)
}

func TestDoctor(t *testing.T) {
	ts := newTestServer(t)
	var rep struct {
		Findings []map[string]any `json:"findings"`
		Repaired []map[string]any `json:"repaired"`
	}
	ts.call("GET", "/api/doctor", nil, 200, &rep)
	if rep.Findings == nil {
		t.Errorf("doctor = %+v", rep)
	}
	ts.call("POST", "/api/doctor/fix", nil, 200, &rep)
	if rep.Repaired == nil || rep.Findings == nil {
		t.Errorf("doctor fix = %+v", rep)
	}
}
```

既存の Store と同じ名前の作成の状態（409 を期待）は ops の `StoreInit` の Kind に合わせる。`schema check` に何も見つからないときの `findings` が `[]` で出ることも確かめている（`null` にしない）。

`internal/server/views_test.go`:

```go
package server

import (
	"net/url"
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

func TestViews(t *testing.T) {
	ts := newTestServer(t)
	var list []ops.SavedView
	ts.call("GET", "/api/views", nil, 200, &list)
	if list == nil || len(list) != 0 {
		t.Fatalf("views = %+v", list)
	}
	var out struct {
		View   ops.SavedView `json:"view"`
		Commit *string       `json:"commit"`
	}
	name := url.PathEscape("今週")
	ts.call("PUT", "/api/views/"+name, map[string]any{"layout": "table", "query": "status:todo", "columns": []string{"id", "title"}}, 200, &out)
	if out.View.Name != "今週" || out.View.Layout != "table" || len(out.View.Table.Columns) != 2 || out.Commit == nil {
		t.Fatalf("saved = %+v", out)
	}
	ts.call("PUT", "/api/views/"+name, map[string]any{"query": "status:done"}, 200, &out)
	if out.View.Layout != "table" || out.View.Query != "status:done" {
		t.Errorf("partial save = %+v", out)
	}
	var v ops.SavedView
	ts.call("GET", "/api/views/"+name, nil, 200, &v)
	if v.Query != "status:done" {
		t.Errorf("get = %+v", v)
	}
	ts.call("PATCH", "/api/views/"+name, map[string]any{"name": "来週"}, 200, &out)
	if out.View.Name != "来週" {
		t.Errorf("renamed = %+v", out)
	}
	ts.problem("GET", "/api/views/"+name, nil, 404)
	if p := ts.problem("PUT", "/api/views/"+url.PathEscape("a/b"), map[string]any{"layout": "table"}, 400); p.Type != "view.bad_name" {
		t.Errorf("bad name: %+v", p)
	}
	ts.call("DELETE", "/api/views/"+url.PathEscape("来週"), nil, 200, &out)
	ts.call("GET", "/api/views", nil, 200, &list)
	if len(list) != 0 {
		t.Errorf("after delete = %+v", list)
	}
}
```

`a/b` はパスの区切りとして扱われてルートに合わないことがある。その場合は 404 になるので、`view.bad_name` を確かめる名前を `a:b` に変える。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/server/ -run 'TestWorkspaces|TestSchema|TestStores|TestDoctor|TestViews'`
Expected: FAIL

- [ ] **Step 3: admin.go を書く**

```go
package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/schema"
)

type workspaceParam struct {
	storeParam
	ID string `path:"id" doc:"The workspace: its name, prefix or ULID"`
}

type workspaceWrite struct {
	Workspace ops.WorkspaceView `json:"workspace"`
	Commit    *string           `json:"commit"`
	IDs       []idChange        `json:"ids,omitempty" doc:"With a new prefix: the IDs the tasks had and have now"`
}

type idChange struct {
	Old string `json:"old"`
	New string `json:"new"`
}

type workspaceOutput struct {
	Status int
	Body   workspaceWrite
}

type workspaceAddInput struct {
	storeParam
	Body struct {
		Name   string `json:"name" minLength:"1"`
		Prefix string `json:"prefix" minLength:"1"`
	}
}

type workspacePatchInput struct {
	workspaceParam
	Body struct {
		Name   *string `json:"name,omitempty" doc:"A new name; give it or prefix, not both"`
		Prefix *string `json:"prefix,omitempty" doc:"A new prefix, which renumbers the tasks"`
	}
}

type findingsOutput struct {
	Body struct {
		Findings []doctor.Finding `json:"findings"`
	}
}

type migrateInput struct {
	storeParam
	Body struct {
		Target string `json:"target" enum:"status,kind,prop,value"`
		Prop   string `json:"prop,omitempty" doc:"With value: the property whose value it is"`
		Old    string `json:"old" minLength:"1"`
		New    string `json:"new" minLength:"1"`
		DryRun bool   `json:"dry_run,omitempty"`
	}
}

type migrateOutput struct {
	Body struct {
		Changed []*ops.TaskView `json:"changed"`
		Skipped []string        `json:"skipped"`
		Commit  *string         `json:"commit"`
	}
}

type storeItem struct {
	ops.StoreView
	Default bool `json:"default" doc:"The store hedgehug picks without --store"`
}

type storesOutput struct{ Body []storeItem }

type storeInitInput struct {
	Body struct {
		Name      string   `json:"name" minLength:"1"`
		Prefix    string   `json:"prefix" minLength:"1"`
		Workspace string   `json:"workspace,omitempty" doc:"The name of the first workspace; the name of the store without it"`
		Remotes   []string `json:"remotes,omitempty"`
		Account   string   `json:"account,omitempty"`
	}
}

type storeInitOutput struct {
	Status int
	Body   struct {
		Name      string            `json:"name"`
		Path      string            `json:"path"`
		Workspace ops.WorkspaceView `json:"workspace"`
		Remotes   []string          `json:"remotes"`
		Pushed    []string          `json:"pushed"`
	}
}

type doctorOutput struct{ Body ops.DoctorReport }

// workspaceOut is the output of a write to a workspace.
func (s *Server) workspaceOut(ss *ops.Session, r ops.WorkspaceResult, status int) (*workspaceOutput, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return nil, problemOf(err)
	}
	out := &workspaceOutput{Status: status, Body: workspaceWrite{Workspace: ops.WorkspaceViewOf(ix, r.Workspace), Commit: commitPtr(r.Commit)}}
	for _, c := range r.IDs {
		out.Body.IDs = append(out.Body.IDs, idChange{c.Old, c.New})
	}
	return out, nil
}

func (s *Server) registerAdmin() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-workspaces", Method: http.MethodGet, Path: "/api/workspaces",
		Summary: "List the workspaces, as hh workspace ls", Tags: []string{"workspaces"},
	}, func(_ context.Context, in *storeParam) (*struct{ Body []ops.WorkspaceView }, error) {
		_, ws, err := s.session(in.Store).WorkspaceList()
		if err != nil {
			return nil, problemOf(err)
		}
		if ws == nil {
			ws = []ops.WorkspaceView{}
		}
		return &struct{ Body []ops.WorkspaceView }{ws}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "add-workspace", Method: http.MethodPost, Path: "/api/workspaces", DefaultStatus: http.StatusCreated,
		Summary: "Add a workspace, as hh workspace add", Tags: []string{"workspaces"},
	}, func(_ context.Context, in *workspaceAddInput) (*workspaceOutput, error) {
		ss := s.session(in.Store)
		r, err := ss.WorkspaceAdd(in.Body.Name, in.Body.Prefix)
		if err != nil {
			return nil, problemOf(err)
		}
		return s.workspaceOut(ss, r, http.StatusCreated)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-workspace", Method: http.MethodPatch, Path: "/api/workspaces/{id}",
		Summary: "Rename a workspace or change its prefix", Tags: []string{"workspaces"},
	}, func(_ context.Context, in *workspacePatchInput) (*workspaceOutput, error) {
		b := in.Body
		ss := s.session(in.Store)
		var r ops.WorkspaceResult
		var err error
		switch {
		case b.Name != nil && b.Prefix != nil:
			return nil, newProblem(http.StatusBadRequest, "workspace.one_change", "give name or prefix, not both")
		case b.Name != nil:
			r, err = ss.WorkspaceRename(in.ID, *b.Name)
		case b.Prefix != nil:
			r, err = ss.WorkspacePrefix(in.ID, *b.Prefix)
		default:
			return nil, badRequest(errors.New("give name or prefix"))
		}
		if err != nil {
			return nil, problemOf(err)
		}
		return s.workspaceOut(ss, r, http.StatusOK)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-workspace", Method: http.MethodDelete, Path: "/api/workspaces/{id}",
		Summary: "Remove an empty workspace, as hh workspace rm", Tags: []string{"workspaces"},
	}, func(_ context.Context, in *workspaceParam) (*workspaceOutput, error) {
		ss := s.session(in.Store)
		r, err := ss.WorkspaceRemove(in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		out := &workspaceOutput{Status: http.StatusOK, Body: workspaceWrite{Commit: commitPtr(r.Commit)}}
		out.Body.Workspace = ops.WorkspaceView{ULID: r.Workspace.ULID.String(), Name: r.Workspace.Name, IDPrefix: r.Workspace.IDPrefix}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-schema", Method: http.MethodGet, Path: "/api/schema",
		Summary: "Show schema.yaml", Tags: []string{"schema"},
	}, func(_ context.Context, in *storeParam) (*struct{ Body ops.SchemaView }, error) {
		_, ix, err := s.session(in.Store).Open()
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body ops.SchemaView }{ops.SchemaViewOf(ix.Schema())}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "check-schema", Method: http.MethodGet, Path: "/api/schema/check",
		Summary: "Check the schema and the tasks, as hh schema check", Tags: []string{"schema"},
	}, func(_ context.Context, in *storeParam) (*findingsOutput, error) {
		fs, err := s.session(in.Store).SchemaCheck()
		if err != nil {
			return nil, problemOf(err)
		}
		out := &findingsOutput{}
		out.Body.Findings = append([]doctor.Finding{}, fs...)
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "migrate-schema", Method: http.MethodPost, Path: "/api/schema/migrate",
		Summary: "Rename or merge a status, kind, property or value, as hh migrate", Tags: []string{"schema"},
	}, func(_ context.Context, in *migrateInput) (*migrateOutput, error) {
		b := in.Body
		m := schema.Migration{Target: schema.Target(b.Target), Prop: b.Prop, Old: b.Old, New: b.New}
		if (m.Target == schema.TargetValue) != (b.Prop != "") {
			return nil, badRequest(errors.New("prop is given with target value, and only then"))
		}
		ss := s.session(in.Store)
		ix, plan, err := ss.Migrate(m, b.DryRun)
		if err != nil {
			return nil, problemOf(err)
		}
		out := &migrateOutput{}
		out.Body.Changed, out.Body.Skipped = []*ops.TaskView{}, []string{}
		out.Body.Commit = commitPtr(plan.Commit)
		for _, t := range plan.Changed {
			e, ok := ix.Task(t.ULID)
			if !ok {
				continue
			}
			v := listView(ix, e)
			if b.DryRun {
				v.Status, v.Kind = t.Status, t.Kind // as the migration would make it
			}
			out.Body.Changed = append(out.Body.Changed, v)
		}
		for _, u := range plan.Skipped {
			out.Body.Skipped = append(out.Body.Skipped, u.String())
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "list-stores", Method: http.MethodGet, Path: "/api/stores",
		Summary: "List the stores, as hh store list", Tags: []string{"stores"},
	}, func(context.Context, *struct{}) (*storesOutput, error) {
		ss := s.session("")
		list, err := ss.StoreList()
		if err != nil {
			return nil, problemOf(err)
		}
		def := ""
		if c, _, err := ss.Config(); err == nil {
			if e, err := c.Select(""); err == nil {
				def = e.Name
			}
		}
		out := &storesOutput{Body: []storeItem{}}
		for _, v := range list {
			out.Body = append(out.Body, storeItem{StoreView: v, Default: v.Name == def})
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "init-store", Method: http.MethodPost, Path: "/api/stores", DefaultStatus: http.StatusCreated,
		Summary: "Make a store, as hh store init", Tags: []string{"stores"},
	}, func(_ context.Context, in *storeInitInput) (*storeInitOutput, error) {
		b := in.Body
		ss := s.session("")
		r, err := ss.StoreInit(ops.StoreInitInput{Name: b.Name, Prefix: b.Prefix, Workspace: b.Workspace, Remotes: b.Remotes, Account: b.Account})
		if err != nil {
			return nil, problemOf(err)
		}
		out := &storeInitOutput{Status: http.StatusCreated}
		out.Body.Name, out.Body.Path = r.Name, r.Path
		out.Body.Remotes, out.Body.Pushed = append([]string{}, r.Remotes...), append([]string{}, r.Pushed...)
		out.Body.Workspace = ops.WorkspaceView{ULID: r.Workspace.ULID.String(), Name: r.Workspace.Name, IDPrefix: r.Workspace.IDPrefix}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "doctor", Method: http.MethodGet, Path: "/api/doctor",
		Summary: "Check the store, as hh doctor", Tags: []string{"maintenance"},
	}, func(_ context.Context, in *storeParam) (*doctorOutput, error) {
		return s.doctor(in.Store, false)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "doctor-fix", Method: http.MethodPost, Path: "/api/doctor/fix",
		Summary: "Check the store and repair what can be, as hh doctor --fix", Tags: []string{"maintenance"},
	}, func(_ context.Context, in *storeParam) (*doctorOutput, error) {
		return s.doctor(in.Store, true)
	})
}

func (s *Server) doctor(store string, fix bool) (*doctorOutput, error) {
	r, err := s.session(store).Doctor(fix)
	if err != nil && !errors.Is(err, ops.ErrFindings) {
		return nil, problemOf(err)
	}
	if r.Findings == nil {
		r.Findings = []doctor.Finding{}
	}
	if r.Repaired == nil {
		r.Repaired = []ops.Fixed{}
	}
	return &doctorOutput{Body: r}, nil
}
```

確かめて合わせること:
- `ops.WorkspaceView` の `PrefixSet`・`Tasks` は、削除と作成の応答では空（0）でよい。Store の作成で `r.Workspace` が nil になることはない（ops を見る）
- `Doctor` が見つけたものがあるときに `ErrFindings` を返すかを ops で確かめる（返さないなら `errors.Is` の分岐を消す）
- `Migrate` の dry run で `plan.Changed` が移行後の値を持つかを確かめ、持たないなら dry run の `Changed` は今の値のまま返す（その場合テストの dry run の確かめはタスク数だけにする）
- `StoreInit` が `Session.StoreName` を使わないことを確かめる

- [ ] **Step 4: views.go を書く**

```go
package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/ops"
)

type viewParam struct {
	storeParam
	Name string `path:"name" doc:"The name of the view"`
}

type viewWrite struct {
	View   ops.SavedView `json:"view"`
	Commit *string       `json:"commit"`
}

type viewOutput struct{ Body viewWrite }

type viewSaveInput struct {
	viewParam
	Body struct {
		Layout        *string   `json:"layout,omitempty" enum:"tree,table,kanban"`
		Query         *string   `json:"query,omitempty"`
		GroupBy       *string   `json:"group_by,omitempty"`
		KanbanColumns *string   `json:"kanban_columns,omitempty"`
		Swimlane      *string   `json:"swimlane,omitempty"`
		Columns       *[]string `json:"columns,omitempty" doc:"The columns of the table"`
	}
}

type viewRenameInput struct {
	viewParam
	Body struct {
		Name string `json:"name" minLength:"1" doc:"The new name"`
	}
}

func viewOut(r ops.ViewResult) *viewOutput {
	out := &viewOutput{Body: viewWrite{Commit: commitPtr(r.Commit)}}
	if r.View != nil {
		out.Body.View = ops.SavedViewOf(r.View)
	}
	return out
}

func (s *Server) registerViews() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-views", Method: http.MethodGet, Path: "/api/views",
		Summary: "List the saved views, as hh view list", Tags: []string{"views"},
	}, func(_ context.Context, in *storeParam) (*struct{ Body []ops.SavedView }, error) {
		vs, err := s.session(in.Store).ViewList()
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body []ops.SavedView }{vs}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-view", Method: http.MethodGet, Path: "/api/views/{name}",
		Summary: "Show a saved view, as hh view show", Tags: []string{"views"},
	}, func(_ context.Context, in *viewParam) (*struct{ Body ops.SavedView }, error) {
		v, err := s.session(in.Store).ViewGet(in.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body ops.SavedView }{ops.SavedViewOf(v)}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "save-view", Method: http.MethodPut, Path: "/api/views/{name}",
		Summary: "Save a view; what is not sent stays as saved, as hh view save", Tags: []string{"views"},
	}, func(_ context.Context, in *viewSaveInput) (*viewOutput, error) {
		b := in.Body
		r, err := s.session(in.Store).ViewSave(in.Name, ops.ViewChange{
			Layout: b.Layout, Query: b.Query, GroupBy: b.GroupBy, KanbanColumns: b.KanbanColumns, Swimlane: b.Swimlane, Columns: b.Columns,
		})
		if err != nil {
			return nil, problemOf(err)
		}
		return viewOut(r), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "rename-view", Method: http.MethodPatch, Path: "/api/views/{name}",
		Summary: "Rename a saved view, as hh view rename", Tags: []string{"views"},
	}, func(_ context.Context, in *viewRenameInput) (*viewOutput, error) {
		r, err := s.session(in.Store).ViewRename(in.Name, in.Body.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		return viewOut(r), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-view", Method: http.MethodDelete, Path: "/api/views/{name}",
		Summary: "Remove a saved view, as hh view rm", Tags: []string{"views"},
	}, func(_ context.Context, in *viewParam) (*viewOutput, error) {
		r, err := s.session(in.Store).ViewRemove(in.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		return viewOut(r), nil
	})
}
```

ビューの名前の変更・削除の応答の `view` は、壊れたファイルでは名前だけ（`ViewResult.View` の説明どおり）。`SavedViewOf` がその場合にも使えるか確かめる。`register` に2つを足す。

- [ ] **Step 5: 生成して通ることを確かめる**

Run: `go generate ./internal/server && go test ./internal/server/`
Expected: PASS

- [ ] **Step 6: 全体の検査とコミット**

Run: `go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...`
Expected: すべて成功

```bash
git add api/openapi.yaml internal/server internal/ops
git commit -m "feat(server): ワークスペース・スキーマ・Store・保守・保存済みビューの API を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: 同期ループ・表示の状態・同期の API

**Files:**
- Create: `internal/server/clock.go`, `internal/server/presence.go`, `internal/server/loop.go`, `internal/server/syncapi.go`
- Create: `internal/server/clock_test.go`（偽の時計）, `internal/server/presence_test.go`, `internal/server/loop_test.go`, `internal/server/syncapi_test.go`
- Modify: `internal/server/server.go`（`Options.Clock`、`Server` に `presence`・`loop`、`Start`・`Shutdown`、`session` の `AfterWrite`）、`api/openapi.yaml`（生成）

**Interfaces:**
- Consumes: `(*ops.Session).Config/SyncTargets/SyncStore/Sync/SyncStatus/OpenStore`、`ops.SyncsInBackground`、`sync.ReadState`、`config.Config.Sync.Interval/PushInterval`
- Produces:
  - `type Clock interface { Now() time.Time; After(d time.Duration) <-chan time.Time }`、`realClock`
  - `type Presence struct`: `Connect() string`、`Disconnect(id string)`、`Set(id string, visible bool) bool`、`Visible() bool`。変化は `onChange func()` で知らせる
  - `type Loop struct`: `Run(ctx context.Context)`、`Wake()`、`NextFetch(store string) (time.Time, bool)`、`Synced(store string)`、`Drain(timeout time.Duration)`。状態の変化は `onState func(store string)`（Task 8 が SSE に流す）
  - `(*Server).Start(ctx context.Context)`、`(*Server).Shutdown(ctx context.Context) error` — Task 9 の `hh ui` が使う
  - `GET /api/sync`、`POST /api/sync`、`PUT /api/presence`
  - テスト: `fakeClock`（`newFakeClock(t time.Time)`、`Advance(d)`）、`eventually(t, what string, cond func() bool)`、`(*testServer).addRemote() string`

**同期の規則（spec と「この plan で埋める spec にない細部」25・26）:**
- 対象は remote のある Store（`ops.SyncsInBackground`）。毎回の判断の前に config.yaml を読み直す（Store の追加・削除に追いつくため）
- fetch の間隔: 表示中なら `sync.interval`、非表示なら `max(10 分, sync.interval)`。最後の同期（成功でも失敗でも）からの時間で測る
- push 待ち: sync.json の `PendingSince` があれば `PendingSince + sync.push_interval` に同期する。ただし最後の同期から `push_interval` 経つまでは次を始めない（失敗したときに詰めて繰り返さないため）
- 起動時に、対象の Store を1回ずつ同期する
- 同じ Store の同期は重ねない。違う Store は並べて走らせてよい
- 同期は `s.session(name).SyncStore(c, e, false)`。返ったエラーとレポートの `Err` は `Options.Log` に `sync <store>: <msg>` で書く（sync.json の `LastError` には ops が書く）

- [ ] **Step 1: 偽の時計と待つ道具を書く**

`internal/server/clock_test.go`:

```go
package server

import (
	"sync"
	"testing"
	"time"
)

// fakeClock is a Clock that moves only when Advance is called.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []fakeWaiter
}

type fakeWaiter struct {
	at time.Time
	c  chan time.Time
}

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{now: t} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- c.now
		return ch
	}
	c.waiters = append(c.waiters, fakeWaiter{c.now.Add(d), ch})
	return ch
}

// Advance moves the clock by d and fires the waiters that are due.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	kept := c.waiters[:0]
	for _, w := range c.waiters {
		if !w.at.After(c.now) {
			w.c <- c.now
			continue
		}
		kept = append(kept, w)
	}
	c.waiters = kept
}

// eventually waits up to 10 seconds of real time for cond.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestFakeClock(t *testing.T) {
	c := newFakeClock(time.Unix(0, 0))
	ch := c.After(time.Minute)
	c.Advance(30 * time.Second)
	select {
	case <-ch:
		t.Fatal("fired early")
	default:
	}
	c.Advance(30 * time.Second)
	select {
	case <-ch:
	default:
		t.Fatal("did not fire")
	}
}
```

- [ ] **Step 2: 表示の状態の失敗するテストを書く**

`internal/server/presence_test.go`:

```go
package server

import "testing"

func TestPresence(t *testing.T) {
	changes := 0
	p := newPresence(func() { changes++ })
	if p.Visible() {
		t.Fatal("no client is visible")
	}
	a := p.Connect()
	b := p.Connect()
	if a == b || !p.Visible() {
		t.Fatalf("clients %q %q, visible %v", a, b, p.Visible())
	}
	if !p.Set(a, false) || !p.Visible() {
		t.Error("b is still visible")
	}
	p.Set(b, false)
	if p.Visible() {
		t.Error("no tab is visible")
	}
	p.Set(b, true)
	p.Disconnect(b)
	if p.Visible() {
		t.Error("a closed tab does not count")
	}
	if p.Set("nope", true) {
		t.Error("an unknown client is refused")
	}
	if changes == 0 {
		t.Error("onChange was not called")
	}
}
```

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./internal/server/ -run 'TestFakeClock|TestPresence'`
Expected: FAIL（`newPresence undefined`）

- [ ] **Step 4: clock.go と presence.go を書く**

`internal/server/clock.go`:

```go
package server

import "time"

// Clock is the time the server's loops wait on; tests replace it.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
```

`internal/server/presence.go`:

```go
package server

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// Presence is whether the GUI is on screen: a client per connection to
// /api/events, visible until it says otherwise. With no client visible,
// nothing is.
type Presence struct {
	mu       sync.Mutex
	clients  map[string]bool // id → visible
	onChange func()
}

func newPresence(onChange func()) *Presence {
	if onChange == nil {
		onChange = func() {}
	}
	return &Presence{clients: map[string]bool{}, onChange: onChange}
}

// Connect adds a client, visible, and returns its id.
func (p *Presence) Connect() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	p.mu.Lock()
	p.clients[id] = true
	p.mu.Unlock()
	p.onChange()
	return id
}

// Disconnect removes the client id.
func (p *Presence) Disconnect(id string) {
	p.mu.Lock()
	delete(p.clients, id)
	p.mu.Unlock()
	p.onChange()
}

// Set sets whether the client id is visible; false for an unknown client.
func (p *Presence) Set(id string, visible bool) bool {
	p.mu.Lock()
	_, ok := p.clients[id]
	if ok {
		p.clients[id] = visible
	}
	p.mu.Unlock()
	if ok {
		p.onChange()
	}
	return ok
}

// Visible reports whether a client is visible.
func (p *Presence) Visible() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, v := range p.clients {
		if v {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: 通ることを確かめる**

Run: `go test ./internal/server/ -run 'TestFakeClock|TestPresence'`
Expected: PASS

- [ ] **Step 6: ループの失敗するテストを書く**

`helpers_test.go` に、偽の時計で作るサーバーと remote の道具を足す。`newTestServer` を `newTestServerWith(t, func(*Options))` から作るように直し、`Options.Clock` と `Options.Now` を差し替えられるようにする:

```go
// newTestServer is newTestServerWith with the options as they are.
func newTestServer(t *testing.T) *testServer { return newTestServerWith(t, nil) }

// newTestServerWith is the test server with opt changed by edit (when not
// nil) before New. It is the body newTestServer had in Task 2, with the
// Options made first.
func newTestServerWith(t *testing.T, edit func(*Options)) *testServer {
	t.Helper()
	d := t.TempDir()
	home := filepath.Join(d, "home")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Alice\n\temail = alice@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ts := &testServer{t: t, log: &bytes.Buffer{}, env: map[string]string{
		"HOME": home, "USERPROFILE": home,
		"HEDGEHUG_HOME":      filepath.Join(d, "hh"),
		"HEDGEHUG_AUTO_SYNC": "0",
	}}
	if _, err := ts.session().StoreInit(ops.StoreInitInput{Name: "me", Prefix: "ME", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	opt := Options{Getenv: ts.getenv, Now: time.Now, Token: testToken, Version: "test", Log: ts.log}
	if edit != nil {
		edit(&opt)
	}
	ts.srv = New(opt)
	ts.hs = httptest.NewServer(ts.srv.Handler())
	t.Cleanup(ts.hs.Close)
	_, p, err := net.SplitHostPort(ts.hs.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ts.port, _ = strconv.Atoi(p)
	ts.srv.SetPort(ts.port)
	return ts
}

// addRemote makes a bare repository the remote of the store "me" and
// returns its path.
func (ts *testServer) addRemote() string {
	ts.t.Helper()
	remote := filepath.Join(ts.t.TempDir(), "r.git")
	if err := gitrepo.InitBare(remote); err != nil {
		ts.t.Fatal(err)
	}
	c, _, err := ts.session().Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	e, err := c.Select("me")
	if err != nil {
		ts.t.Fatal(err)
	}
	if err := c.RemoveStore("me"); err != nil {
		ts.t.Fatal(err)
	}
	if err := c.AddStore("me", e.Path, []string{remote}); err != nil {
		ts.t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		ts.t.Fatal(err)
	}
	return remote
}

// state is sync.json of the store "me".
func (ts *testServer) state() sync.State {
	ts.t.Helper()
	s, err := ts.session().OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	st, err := sync.ReadState(s)
	if err != nil {
		ts.t.Fatal(err)
	}
	return st
}

// remoteHead is the commit main points at in the bare repository remote;
// "" before the first push.
func remoteHead(t *testing.T, remote string) string {
	t.Helper()
	r, err := git.PlainOpen(remote)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Reference(plumbing.NewBranchReferenceName("main"), true)
	if err != nil {
		return ""
	}
	return ref.Hash().String()
}

// waitFirstSync waits for the sync the loop makes at its start to end.
func waitFirstSync(t *testing.T, ts *testServer) {
	t.Helper()
	eventually(t, "the sync at start", func() bool { return !ts.state().LastSuccess.IsZero() })
}
```

（`git` は `github.com/go-git/go-git/v5`、`plumbing` は `github.com/go-git/go-git/v5/plumbing`。Store の push 先のブランチが `main` でないときは、`internal/g*repo/remote.go` の `Push` が押すブランチ名に合わせる。）

（`sync` は `internal/sync`。標準の `sync` と名前がぶつかるので `hhsync` として import する。）

`internal/server/loop_test.go`:

```go
package server

import (
	"context"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

// loopServer is a test server with a fake clock, a remote, and its loop
// running until the test ends.
func loopServer(t *testing.T) (*testServer, *fakeClock, string) {
	t.Helper()
	clk := newFakeClock(t0)
	ts := newTestServerWith(t, func(o *Options) { o.Clock, o.Now = clk, clk.Now })
	remote := ts.addRemote()
	ctx, cancel := context.WithCancel(context.Background())
	ts.srv.Start(ctx)
	t.Cleanup(func() {
		cancel()
		_ = ts.srv.Shutdown(context.Background())
	})
	return ts, clk, remote
}

func TestLoopSyncsAtStart(t *testing.T) {
	ts, _, _ := loopServer(t)
	waitFirstSync(t, ts)
}

func TestLoopIntervalVisibleAndHidden(t *testing.T) {
	ts, clk, _ := loopServer(t)
	waitFirstSync(t, ts)
	first := ts.state().LastAttempt

	// Hidden (no client): 2 minutes is not enough, 10 is.
	clk.Advance(2 * time.Minute)
	time.Sleep(100 * time.Millisecond)
	if got := ts.state().LastAttempt; !got.Equal(first) {
		t.Fatalf("hidden: synced after 2 minutes (%v)", got)
	}
	clk.Advance(8 * time.Minute)
	eventually(t, "the hidden fetch", func() bool { return ts.state().LastAttempt.After(first) })
	second := ts.state().LastAttempt

	// Visible: every 2 minutes.
	id := ts.srv.presence.Connect()
	defer ts.srv.presence.Disconnect(id)
	clk.Advance(2 * time.Minute)
	eventually(t, "the visible fetch", func() bool { return ts.state().LastAttempt.After(second) })
}

func TestLoopPushesAfterWrite(t *testing.T) {
	ts, clk, remote := loopServer(t)
	waitFirstSync(t, ts)
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	if ts.state().PendingSince.IsZero() {
		t.Fatal("the write is not pending")
	}
	clk.Advance(29 * time.Second)
	time.Sleep(100 * time.Millisecond)
	if remoteHead(t, remote) == *out.Commit {
		t.Fatal("pushed before push_interval")
	}
	clk.Advance(time.Second)
	eventually(t, "the push", func() bool { return remoteHead(t, remote) == *out.Commit })
	if !ts.state().PendingSince.IsZero() {
		t.Error("still pending after the push")
	}
}

func TestShutdownWaitsForPush(t *testing.T) {
	ts, clk, remote := loopServer(t)
	waitFirstSync(t, ts)
	_ = clk
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	if err := ts.srv.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := remoteHead(t, remote); got != *out.Commit {
		t.Errorf("remote at %s after shutdown, want %s", got, *out.Commit)
	}
}

func TestDrainGivesUp(t *testing.T) {
	ts, _, _ := loopServer(t)
	waitFirstSync(t, ts)
	block := make(chan struct{})
	defer close(block)
	ts.srv.loop.setSyncFn(func(string) { <-block })
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	start := time.Now()
	ts.srv.loop.Drain(200 * time.Millisecond)
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("Drain took %v, want about 200ms", d)
	}
}
```

`TestShutdownWaitsForPush` は `loopServer` の Cleanup でもう一度 `Shutdown` が呼ばれる。2回目は何もしないで nil を返すようにする（Step 8）。

- [ ] **Step 7: 失敗を確かめる**

Run: `go test ./internal/server/ -run 'TestLoop|TestShutdown|TestDrain'`
Expected: FAIL（`Start`・`loop` が未定義）

- [ ] **Step 8: loop.go と Server の配線を書く**

`internal/server/loop.go`:

```go
package server

import (
	"context"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/ops"
	hhsync "github.com/kukv/hedgehug/internal/sync"
)

// hiddenInterval is the shortest time between fetches while no tab shows
// the GUI.
const hiddenInterval = 10 * time.Minute

// Loop syncs the stores with a remote: once at the start, then every
// sync.interval while the GUI is on screen (hiddenInterval at least while it
// is not), and push_interval after a write.
type Loop struct {
	s       *Server
	clock   Clock
	visible func() bool
	onState func(store string)
	syncFn  func(store string) // the sync of one store; tests replace it

	wake chan struct{}

	mu      sync.Mutex
	last    map[string]time.Time // store → when its last sync started
	next    map[string]time.Time // store → when the next fetch is due
	running map[string]bool
	wg      sync.WaitGroup
}

func newLoop(s *Server, clock Clock, visible func() bool) *Loop {
	l := &Loop{
		s: s, clock: clock, visible: visible, onState: func(string) {},
		wake: make(chan struct{}, 1),
		last: map[string]time.Time{}, next: map[string]time.Time{}, running: map[string]bool{},
	}
	l.syncFn = l.syncStore
	return l
}

// Wake makes the loop look again now: after a write, or when the GUI shows
// or hides.
func (l *Loop) Wake() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// Run runs the loop until ctx is done.
func (l *Loop) Run(ctx context.Context) {
	for {
		wait := l.step()
		select {
		case <-ctx.Done():
			return
		case <-l.wake:
		case <-l.clock.After(wait):
		}
	}
}

// step starts the syncs that are due and returns how long to wait for the
// next one.
func (l *Loop) step() time.Duration {
	c, targets := l.targets()
	if c == nil {
		return time.Minute // config.yaml does not read; look again later
	}
	now := l.clock.Now()
	interval := c.Sync.Interval
	if !l.visible() && interval < hiddenInterval {
		interval = hiddenInterval
	}
	wait := interval
	for _, e := range targets {
		due := l.due(e, now, interval, c.Sync.PushInterval)
		if !due.After(now) {
			l.start(e.Name)
			continue
		}
		wait = min(wait, due.Sub(now))
	}
	return wait
}

// targets are the stores with a remote, as config.yaml has them now.
func (l *Loop) targets() (*config.Config, []config.StoreEntry) {
	ss := l.s.session("")
	c, _, err := ss.Config()
	if err != nil {
		l.s.logf("sync: %v", err)
		return nil, nil
	}
	all, err := ss.SyncTargets(c, true)
	if err != nil {
		l.s.logf("sync: %v", err)
		return nil, nil
	}
	var out []config.StoreEntry
	for _, e := range all {
		if ops.SyncsInBackground(e) {
			out = append(out, e)
		}
	}
	return c, out
}

// due is when e is to be synced next: now for one never synced, the fetch
// interval after its last sync, or push after its first write that is not
// pushed, but not sooner than push after its last sync.
func (l *Loop) due(e config.StoreEntry, now time.Time, interval, push time.Duration) time.Time {
	l.mu.Lock()
	last, synced := l.last[e.Name]
	running := l.running[e.Name]
	l.mu.Unlock()
	if running {
		return now.Add(interval) // looked at again when it ends
	}
	if !synced {
		return now
	}
	due := last.Add(interval)
	if pending := l.pendingSince(e); !pending.IsZero() {
		due = minTime(due, maxTime(pending.Add(push), last.Add(push)))
	}
	l.mu.Lock()
	l.next[e.Name] = last.Add(interval)
	l.mu.Unlock()
	return due
}

// pendingSince is the first write of e not pushed; zero when there is none
// or sync.json does not read.
func (l *Loop) pendingSince(e config.StoreEntry) time.Time {
	s, err := l.s.session(e.Name).OpenStore()
	if err != nil {
		return time.Time{}
	}
	st, err := hhsync.ReadState(s)
	if err != nil {
		return time.Time{}
	}
	return st.PendingSince
}

// start runs the sync of name in its own goroutine, unless one runs.
func (l *Loop) start(name string) {
	l.mu.Lock()
	if l.running[name] {
		l.mu.Unlock()
		return
	}
	l.running[name] = true
	l.last[name] = l.clock.Now()
	fn := l.syncFn
	l.mu.Unlock()
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		fn(name)
		l.mu.Lock()
		l.running[name] = false
		l.mu.Unlock()
		l.onState(name)
		l.Wake()
	}()
}

// syncStore is the sync of one store, as the background sync of the CLI
// does it.
func (l *Loop) syncStore(name string) {
	ss := l.s.session(name)
	c, _, err := ss.Config()
	if err != nil {
		l.s.logf("sync %s: %v", name, err)
		return
	}
	e, err := c.Select(name)
	if err != nil {
		l.s.logf("sync %s: %v", name, err)
		return
	}
	rep, err := ss.SyncStore(c, e, false)
	if err == nil {
		err = rep.Err
	}
	if err != nil {
		l.s.logf("sync %s: %s", name, detailOf(err))
	}
}

// setSyncFn replaces the sync of one store (tests).
func (l *Loop) setSyncFn(fn func(store string)) {
	l.mu.Lock()
	l.syncFn = fn
	l.mu.Unlock()
}

// Synced notes a sync made outside the loop (POST /api/sync).
func (l *Loop) Synced(name string) {
	l.mu.Lock()
	l.last[name] = l.clock.Now()
	l.mu.Unlock()
	l.Wake()
}

// NextFetch is when the loop fetches name next; false for a store it has
// not synced yet.
func (l *Loop) NextFetch(name string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.next[name]
	return t, ok
}

// Drain syncs the stores with a write not pushed, and waits for them and
// for the syncs that run, timeout at most. A sync that ran already when
// Drain began may have started before the write; so once they end, the
// stores still pending are synced once more.
func (l *Loop) Drain(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for range 2 {
		pending := false
		if _, targets := l.targets(); targets != nil {
			for _, e := range targets {
				if !l.pendingSince(e).IsZero() {
					pending = true
					l.start(e.Name) // nothing new when one runs; it is waited for below
				}
			}
		}
		if !l.waitUntil(deadline) {
			l.s.logf("sync: stopped waiting for the push after %v", timeout)
			return
		}
		if !pending {
			return
		}
	}
}

// waitUntil waits for the syncs that run, until deadline; false when it
// came first.
func (l *Loop) waitUntil(deadline time.Time) bool {
	done := make(chan struct{})
	go func() { l.wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(time.Until(deadline)):
		return false
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
```

`Drain` の待ちは実時間（`time.After`）にする（偽の時計では止まってしまうため）。

`internal/server/server.go` を直す:

- `Options` に `Clock Clock // nil is the real clock` を足し、`New` で nil なら `realClock{}` にする
- `Server` に `presence *Presence`、`loop *Loop`、`stopOnce sync.Once`、`cancel context.CancelFunc` を足す
- `New` で `s.presence = newPresence(func() { s.loop.Wake() })`、`s.loop = newLoop(s, opt.Clock, s.presence.Visible)`（presence の onChange が loop を参照するので、loop を先に作るか、onChange の中で nil を確かめる）
- `session` の `AfterWrite` を `func(*store.Store, config.StoreEntry) { s.loop.Wake() }` にする
- `register` に `s.registerSync()` を足す

```go
// Start runs the sync loop (and, from Task 8, the events) until ctx is
// done or Shutdown is called.
func (s *Server) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	go s.loop.Run(ctx)
}

// Shutdown stops the loop and waits for the pushes of the writes not
// pushed, 10 seconds at most. A second call does nothing.
func (s *Server) Shutdown(context.Context) error {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.loop.Drain(10 * time.Second)
	})
	return nil
}
```

- [ ] **Step 9: 通ることを確かめる**

Run: `go test -race ./internal/server/ -run 'TestLoop|TestShutdown|TestDrain|TestPresence'`
Expected: PASS

- [ ] **Step 10: 同期の API の失敗するテストを書く**

`internal/server/syncapi_test.go`:

```go
package server

import (
	"testing"
	"time"
)

type syncStatusBody struct {
	Store     string     `json:"store"`
	Remotes   []string   `json:"remotes"`
	NextFetch *time.Time `json:"next_fetch"`
	Unpushed  *int       `json:"unpushed"`
}

func TestSyncAPI(t *testing.T) {
	ts, _, remote := loopServer(t)
	waitFirstSync(t, ts)
	var st []syncStatusBody
	ts.call("GET", "/api/sync", nil, 200, &st)
	if len(st) != 1 || st[0].Store != "me" || st[0].NextFetch == nil {
		t.Fatalf("status = %+v", st)
	}
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	var reps []struct {
		Store  string   `json:"store"`
		Pushed []string `json:"pushed"`
		Error  string   `json:"error"`
	}
	ts.call("POST", "/api/sync", nil, 200, &reps)
	if len(reps) != 1 || reps[0].Error != "" || remoteHead(t, remote) != *out.Commit {
		t.Errorf("sync = %+v", reps)
	}
	ts.call("POST", "/api/sync?store=me", nil, 200, &reps)
	if len(reps) != 1 {
		t.Errorf("sync of me = %+v", reps)
	}
}

func TestPresenceAPI(t *testing.T) {
	ts := newTestServer(t)
	id := ts.srv.presence.Connect()
	ts.call("PUT", "/api/presence", map[string]any{"client": id, "visible": false}, 204, nil)
	if ts.srv.presence.Visible() {
		t.Error("still visible")
	}
	if p := ts.problem("PUT", "/api/presence", map[string]any{"client": "nope", "visible": true}, 404); p.Type != "presence.unknown_client" {
		t.Errorf("unknown client: %+v", p)
	}
}
```

- [ ] **Step 11: 失敗を確かめる**

Run: `go test ./internal/server/ -run 'TestSyncAPI|TestPresenceAPI'`
Expected: FAIL

- [ ] **Step 12: syncapi.go を書く**

```go
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/ops"
)

type syncStatus struct {
	ops.StoreStatus
	NextFetch *time.Time `json:"next_fetch,omitempty" doc:"When the loop fetches next; absent before its first sync"`
}

type presenceInput struct {
	Body struct {
		Client  string `json:"client" minLength:"1" doc:"The client that hello gave"`
		Visible bool   `json:"visible"`
	}
}

func (s *Server) registerSync() {
	huma.Register(s.api, huma.Operation{
		OperationID: "get-sync", Method: http.MethodGet, Path: "/api/sync",
		Summary: "How the syncs went, as hh sync --status", Tags: []string{"sync"},
	}, func(context.Context, *struct{}) (*struct{ Body []syncStatus }, error) {
		sts, err := s.session("").SyncStatus()
		if err != nil {
			return nil, problemOf(err)
		}
		out := &struct{ Body []syncStatus }{[]syncStatus{}}
		for _, st := range sts {
			v := syncStatus{StoreStatus: st}
			if t, ok := s.loop.NextFetch(st.Store); ok {
				v.NextFetch = &t
			}
			out.Body = append(out.Body, v)
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "sync", Method: http.MethodPost, Path: "/api/sync",
		Summary: "Sync now: the store given, or every store with a remote, as hh sync", Tags: []string{"sync"},
	}, func(_ context.Context, in *storeParam) (*struct{ Body []ops.SyncReport }, error) {
		reps, err := s.session(in.Store).Sync(false, true)
		if len(reps) == 0 && err != nil {
			return nil, problemOf(err)
		}
		for i := range reps {
			if reps[i].Err != nil {
				reps[i].Error = detailOf(reps[i].Err)
			}
			s.loop.Synced(reps[i].Store)
			s.loop.onState(reps[i].Store)
		}
		if reps == nil {
			reps = []ops.SyncReport{}
		}
		return &struct{ Body []ops.SyncReport }{reps}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "set-presence", Method: http.MethodPut, Path: "/api/presence",
		Summary: "Tell whether a tab shows the GUI", Tags: []string{"sync"},
	}, func(_ context.Context, in *presenceInput) (*struct{}, error) {
		if !s.presence.Set(in.Body.Client, in.Body.Visible) {
			return nil, newProblem(http.StatusNotFound, "presence.unknown_client", "no events connection is "+in.Body.Client)
		}
		return nil, nil
	})
}
```

`Sync(false, true)` が `StoreName` のないときに remote のある全 Store を同期すること、remote のない Store を指定したときに何を返すか（`NoRemote`）を ops で確かめる。`SyncReport` の nil のスライス（`Pushed` など）が `null` で出て検証に落ちるときは、応答の前に空のスライスに直す。

- [ ] **Step 13: 生成して通ることを確かめる**

Run: `go generate ./internal/server && go test -race ./internal/server/`
Expected: PASS

- [ ] **Step 14: 全体の検査とコミット**

Run: `go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...`
Expected: すべて成功

```bash
git add api/openapi.yaml internal/server
git commit -m "feat(server): 同期ループと表示の状態、同期の API を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: SSE と HEAD のポーリング

**Files:**
- Create: `internal/server/hub.go`, `internal/server/poll.go`, `internal/server/events.go`
- Create: `internal/server/hub_test.go`, `internal/server/poll_test.go`, `internal/server/events_test.go`
- Modify: `internal/inbox/inbox.go`（`(*Box).Stamp`）、`internal/inbox/inbox_test.go`、`internal/server/server.go`（`hub`・`poller`、`Start`・`Shutdown`、`register`）、`internal/server/loop.go`（`onState` を poller に）、`api/openapi.yaml`（生成）

**Interfaces:**
- Consumes: Task 7 の `Presence`・`Loop.NextFetch`・`syncStatus`・`Clock`、`(*ops.Session).SyncTargets/SyncStatus/InboxList/OpenStore`、`(*gitrepo.Repo).Head/TreeOf/DiffTrees/Log...`、`index.Open`・`(*index.Index).Refresh`、`ops.TaskOfPath`
- Produces:
  - `func (b *inbox.Box) Stamp() string` — inbox のローカルのファイル（項目・既読・印）の大きさと更新時刻をつないだ文字列。変われば未読数を数え直す
  - `type Hub struct`: `Subscribe() (<-chan sseEvent, func())`、`Publish(e any)`、`Close()`
  - `type Poller struct`: `Run(ctx)`、`Snapshot() []storeState`、`Unread() int`、`Kick()`
  - `GET /api/events`（`text/event-stream`）。イベントの型: `helloEvent`・`tasksChanged`・`syncStatus`（`sync.state`）・`unreadEvent`・`schemaChanged`・`viewsChanged`
  - テスト: `(*testServer).events() *eventStream`、`(*eventStream).next(t, name string) json.RawMessage`、`(*eventStream).Close()`

**イベント（spec の表と「この plan で埋める spec にない細部」24）:**

| イベント | 型 | 中身 |
|---|---|---|
| `hello` | `helloEvent` | `client`、`stores: [{store, head, sync: syncStatus}]`、`unread` |
| `tasks.changed` | `tasksChanged` | `store`、`ulids`、`removed`、`commit`、`op`、`author` |
| `sync.state` | `syncStatus`（Task 7） | `store`、`last_success`、`unpushed`、`last_error`・`error`、`next_fetch`、`pending_since` など |
| `notifications.unread` | `unreadEvent` | `count`（全 Store の合計） |
| `schema.changed` | `schemaChanged` | `store` |
| `views.changed` | `viewsChanged` | `store` |

- [ ] **Step 1: Box.Stamp の失敗するテストを書く**

`internal/inbox/inbox_test.go` に足す（既存のテストの Store の作り方に合わせる）:

```go
func TestStampChangesWithTheFiles(t *testing.T) {
	s := newTestStore(t) // 既存のテストの道具の名前に合わせる
	b := Open(s)
	before := b.Stamp()
	if _, err := b.Add([]Item{{ULID: model.NewULID(), Kind: Mention, Commit: "c", Time: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	after := b.Stamp()
	if before == after {
		t.Fatalf("stamp did not change: %q", after)
	}
	if b.Stamp() != after {
		t.Error("stamp changed with nothing written")
	}
}
```

- [ ] **Step 2: 失敗を確かめ、Stamp を書く**

Run: `go test ./internal/inbox/ -run TestStamp` → FAIL（`b.Stamp undefined`）

`internal/inbox/inbox.go` に足す（ローカルのファイルのパスは `Store` の既存の関数で得る。`itemsFile`・`readFile`・`markFile` の3つ）:

```go
// Stamp tells whether the files of the box changed: the size and the time
// of each, joined. A missing file stamps as "-".
func (b *Box) Stamp() string {
	var sb strings.Builder
	for _, name := range []string{itemsFile, readFile, markFile} {
		st, err := os.Stat(b.s.LocalPath(name))
		if err != nil {
			sb.WriteString("-;")
			continue
		}
		fmt.Fprintf(&sb, "%d:%d;", st.Size(), st.ModTime().UnixNano())
	}
	return sb.String()
}
```

`Store` にローカルのファイルのパスを返す関数がなければ、`internal/store/local.go` に `func (s *Store) LocalPath(name string) string`（`s.path(LocalDir+"/"+name)`）を足す。

Run: `go test ./internal/inbox/` → PASS

- [ ] **Step 3: Hub の失敗するテストを書く**

`internal/server/hub_test.go`:

```go
package server

import "testing"

func TestHub(t *testing.T) {
	h := newHub()
	a, cancelA := h.Subscribe()
	b, cancelB := h.Subscribe()
	h.Publish(viewsChanged{Store: "me"})
	for _, ch := range []<-chan sseEvent{a, b} {
		if e := <-ch; e.(viewsChanged).Store != "me" {
			t.Errorf("event = %+v", e)
		}
	}
	cancelA()
	h.Publish(viewsChanged{Store: "x"})
	if e := <-b; e.(viewsChanged).Store != "x" {
		t.Errorf("event = %+v", e)
	}
	// A subscriber that does not read is dropped (its channel closed), not
	// waited for.
	for range hubBuffer + 1 {
		h.Publish(viewsChanged{})
	}
	for range b {
	}
	cancelB() // after the drop: does nothing
	h.Close()
	c, _ := h.Subscribe()
	if _, ok := <-c; ok {
		t.Error("a subscription after Close is closed")
	}
}
```

- [ ] **Step 4: hub.go を書く**

```go
package server

import "sync"

// hubBuffer is how many events a subscriber may be behind before it is
// dropped; the GUI then connects again and fetches what it shows.
const hubBuffer = 256

// sseEvent is one of the event types of /api/events.
type sseEvent = any

// Hub hands the events to the connections of /api/events.
type Hub struct {
	mu     sync.Mutex
	subs   map[chan sseEvent]struct{}
	closed bool
}

func newHub() *Hub { return &Hub{subs: map[chan sseEvent]struct{}{}} }

// Subscribe returns the events from now on, and the function that stops
// them. The channel is closed when the subscriber falls behind or the hub
// closes.
func (h *Hub) Subscribe() (<-chan sseEvent, func()) {
	ch := make(chan sseEvent, hubBuffer)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch, func() {}
	}
	h.subs[ch] = struct{}{}
	return ch, func() { h.drop(ch) }
}

func (h *Hub) drop(ch chan sseEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
}

// Publish sends e to every subscriber, dropping those that are behind.
func (h *Hub) Publish(e sseEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
			delete(h.subs, ch)
			close(ch)
		}
	}
}

// Close ends every subscription, and the ones after.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for ch := range h.subs {
		delete(h.subs, ch)
		close(ch)
	}
}
```

Run: `go test ./internal/server/ -run TestHub` → PASS

- [ ] **Step 5: ポーリングの失敗するテストを書く**

`internal/server/poll_test.go`。Poller を直接回し、Hub に出たイベントを読む（SSE を通さない）:

```go
package server

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/schema"
	hhsync "github.com/kukv/hedgehug/internal/sync"
)

// pollServer runs the poller of a test server (real clock) and returns the
// events it publishes.
func pollServer(t *testing.T) (*testServer, <-chan sseEvent) {
	t.Helper()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.Subscribe()
	ctx, stop := context.WithCancel(context.Background())
	go ts.srv.poller.Run(ctx)
	t.Cleanup(func() { stop(); cancel() })
	return ts, ch
}

// waitEvent returns the first event of type T, skipping the others.
func waitEvent[T any](t *testing.T, ch <-chan sseEvent, match func(T) bool) T {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				t.Fatal("the hub dropped the subscription")
			}
			if v, ok := e.(T); ok && (match == nil || match(v)) {
				return v
			}
		case <-timeout:
			var zero T
			t.Fatalf("no %T event", zero)
			return zero
		}
	}
}

func TestPollTasksChanged(t *testing.T) {
	ts, ch := pollServer(t)
	time.Sleep(2 * pollInterval) // the first look sets the HEADs; it sends nothing
	res, err := ts.session().Add(ops.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	e := waitEvent[tasksChanged](t, ch, nil)
	if e.Store != "me" || !slices.Contains(e.ULIDs, res.ULID.String()) || e.Commit != res.Commit || e.Op != "add" {
		t.Errorf("event = %+v", e)
	}
}

func TestPollCommentChangesItsTask(t *testing.T) {
	ts, ch := pollServer(t)
	res, _ := ts.session().Add(ops.AddInput{Title: "one"})
	time.Sleep(2 * pollInterval)
	if _, err := ts.session().CommentAdd(res.ULID.String(), "hi"); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[tasksChanged](t, ch, func(e tasksChanged) bool { return e.Op == "comment" })
	if !slices.Contains(e.ULIDs, res.ULID.String()) {
		t.Errorf("event = %+v", e)
	}
}

func TestPollRemoved(t *testing.T) {
	ts, ch := pollServer(t)
	res, _ := ts.session().Add(ops.AddInput{Title: "one"})
	if _, err := ts.session().Remove(res.ULID.String()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * pollInterval)
	if _, err := ts.session().PurgeUnchecked(res.ULID.String()); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[tasksChanged](t, ch, func(e tasksChanged) bool { return e.Op == "purge" })
	if !slices.Contains(e.Removed, res.ULID.String()) {
		t.Errorf("event = %+v", e)
	}
}

func TestPollSchemaAndViews(t *testing.T) {
	ts, ch := pollServer(t)
	ts.session().Add(ops.AddInput{Title: "one"})
	time.Sleep(2 * pollInterval)
	sc := schema.Default()
	if _, _, err := ts.session().Migrate(schema.Migration{Target: schema.TargetStatus, Old: sc.Statuses[0].Name, New: "renamed"}, false); err != nil {
		t.Fatal(err)
	}
	waitEvent[schemaChanged](t, ch, nil)
	layout := "table"
	if _, err := ts.session().ViewSave("v", ops.ViewChange{Layout: &layout}); err != nil {
		t.Fatal(err)
	}
	waitEvent[viewsChanged](t, ch, nil)
}

func TestPollSyncState(t *testing.T) {
	ts, ch := pollServer(t)
	time.Sleep(2 * pollInterval)
	s, _ := ts.session().OpenStore()
	if err := hhsync.MarkPending(s, time.Now()); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[syncStatus](t, ch, func(e syncStatus) bool { return e.PendingSince != nil })
	if e.Store != "me" {
		t.Errorf("event = %+v", e)
	}
}

func TestPollUnread(t *testing.T) {
	ts, ch := pollServer(t)
	res, err := ts.session().Add(ops.AddInput{Title: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := res.View()
	time.Sleep(2 * pollInterval)
	nid := ts.notifyMe(overwrittenItem(res.ULID, v.ID, "title", `"first"`))
	waitEvent[unreadEvent](t, ch, func(e unreadEvent) bool { return e.Count == 1 })
	// hh inbox read in another process: no HEAD moves, the files of the
	// inbox do.
	if _, err := ts.session().InboxRead([]string{nid}, false); err != nil {
		t.Fatal(err)
	}
	waitEvent[unreadEvent](t, ch, func(e unreadEvent) bool { return e.Count == 0 })
}
```

（`ts.session().Add` の戻り値を捨てているところは、lint に合わせて `if _, err := …; err != nil { t.Fatal(err) }` にする。）

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/server/ -run TestPoll`
Expected: FAIL（`poller`・`pollInterval` が未定義）

- [ ] **Step 7: poll.go を書く**

```go
package server

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// pollInterval is how often the poller reads the HEAD and the sync state of
// each store.
const pollInterval = 300 * time.Millisecond

type tasksChanged struct {
	Store   string   `json:"store"`
	ULIDs   []string `json:"ulids" doc:"The tasks whose file or comments changed"`
	Removed []string `json:"removed" doc:"The tasks whose file is gone (purged)"`
	Commit  string   `json:"commit" doc:"HEAD now"`
	Op      string   `json:"op" doc:"The Hedgehug-Op of HEAD; empty for a commit made by hand"`
	Author  string   `json:"author" doc:"The Hedgehug-Author of HEAD; empty when unknown"`
}

type unreadEvent struct {
	Count int `json:"count" doc:"The items not read, in every store"`
}

type schemaChanged struct {
	Store string `json:"store"`
}

type viewsChanged struct {
	Store string `json:"store"`
}

type storeState struct {
	Store string     `json:"store"`
	Head  string     `json:"head"`
	Sync  syncStatus `json:"sync"`
}

// polled is what the poller knows of a store.
type polled struct {
	path  string
	s     *store.Store
	ix    *index.Index
	head  plumbing.Hash
	sync  syncStatus
	stamp string
}

// Poller reads the HEAD and the sync state of each store every
// pollInterval, and publishes what changed.
type Poller struct {
	s     *Server
	clock Clock
	hub   *Hub
	kick  chan struct{}

	mu     sync.Mutex
	stores map[string]*polled
	unread int
}

func newPoller(s *Server, clock Clock, hub *Hub) *Poller {
	return &Poller{s: s, clock: clock, hub: hub, kick: make(chan struct{}, 1), stores: map[string]*polled{}}
}

// Kick makes the poller look now (after the server marked items read).
func (p *Poller) Kick() {
	select {
	case p.kick <- struct{}{}:
	default:
	}
}

// Run polls until ctx is done.
func (p *Poller) Run(ctx context.Context) {
	for {
		p.poll(true)
		select {
		case <-ctx.Done():
			return
		case <-p.kick:
		case <-p.clock.After(pollInterval):
		}
	}
}

// poll reads every store once. With publish false it only records (for
// Snapshot before Run has looked).
func (p *Poller) poll(publish bool) {
	ss := p.s.session("")
	c, _, err := ss.Config()
	if err != nil {
		return
	}
	targets, err := ss.SyncTargets(c, true)
	if err != nil {
		return
	}
	statuses := map[string]syncStatus{}
	if sts, err := ss.SyncStatus(); err == nil {
		for _, st := range sts {
			v := syncStatus{StoreStatus: st}
			if t, ok := p.s.loop.NextFetch(st.Store); ok {
				v.NextFetch = &t
			}
			statuses[st.Store] = v
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	seen := map[string]bool{}
	recount := false
	for _, e := range targets {
		seen[e.Name] = true
		st := p.stores[e.Name]
		if st == nil || st.path != e.Path {
			s, err := p.s.session(e.Name).OpenStore()
			if err != nil {
				continue
			}
			st = &polled{path: e.Path, s: s}
			p.stores[e.Name] = st
		}
		head, err := st.s.Repo().Head()
		if err != nil {
			continue
		}
		if head != st.head {
			if !st.head.IsZero() && publish {
				p.headMoved(e.Name, st, head)
			}
			st.head = head
			p.refresh(st)
			recount = true
		}
		if v, ok := statuses[e.Name]; ok && !sameStatus(v, st.sync) {
			st.sync = v
			if publish {
				p.hub.Publish(v)
			}
		}
		if inbox.Open(st.s).Stamp() != st.stamp {
			recount = true
		}
	}
	for name := range p.stores {
		if !seen[name] {
			delete(p.stores, name)
		}
	}
	if recount {
		if _, items, err := ss.InboxList(false); err == nil && len(items) != p.unread {
			p.unread = len(items)
			if publish {
				p.hub.Publish(unreadEvent{Count: p.unread})
			}
		}
		// InboxList writes the files of the boxes (Catch moves the mark),
		// so the stamps are taken after it: else every poll would count
		// again.
		for _, st := range p.stores {
			st.stamp = inbox.Open(st.s).Stamp()
		}
	}
}

// headMoved publishes the changes from st.head to head.
func (p *Poller) headMoved(name string, st *polled, head plumbing.Hash) {
	r := st.s.Repo()
	from, err1 := r.TreeOf(st.head)
	to, err2 := r.TreeOf(head)
	if err1 != nil || err2 != nil {
		p.hub.Publish(tasksChanged{Store: name, ULIDs: []string{}, Removed: []string{}, Commit: head.String()})
		return
	}
	changes, err := r.DiffTrees(from, to, "")
	if err != nil {
		return
	}
	ev := tasksChanged{Store: name, ULIDs: []string{}, Removed: []string{}, Commit: head.String()}
	if log, err := r.LogFrom(head, 1); err == nil && len(log) == 1 {
		ev.Op, _ = log[0].Trailer("Hedgehug-Op")
		ev.Author, _ = log[0].Trailer("Hedgehug-Author")
	}
	var schemaCh, viewsCh bool
	for _, c := range changes {
		switch {
		case c.Path == "schema.yaml":
			schemaCh = true
		case strings.HasPrefix(c.Path, "views/"):
			viewsCh = true
		}
		if u, ok := ops.TaskOfPath(c.Path); ok {
			if c.Deleted {
				addOnce(&ev.Removed, u)
			} else {
				addOnce(&ev.ULIDs, u)
			}
			continue
		}
		if u, ok := commentTask(c.Path); ok {
			addOnce(&ev.ULIDs, u)
		}
	}
	if len(ev.ULIDs) > 0 || len(ev.Removed) > 0 || (!schemaCh && !viewsCh) {
		p.hub.Publish(ev)
	}
	if schemaCh {
		p.hub.Publish(schemaChanged{Store: name})
	}
	if viewsCh {
		p.hub.Publish(viewsChanged{Store: name})
	}
}

// refresh brings the index of st to HEAD, for the requests that follow.
func (p *Poller) refresh(st *polled) {
	if st.ix != nil && st.ix.Refresh() == nil {
		return
	}
	ix, err := index.Open(st.s)
	if err != nil {
		st.ix = nil
		return
	}
	st.ix = ix
}

// commentTask is the task of the comment file p: comments/<a>/<b>/<task>/<comment>.md.
func commentTask(p string) (model.ULID, bool) {
	parts := strings.Split(p, "/")
	if len(parts) != 5 || parts[0] != "comments" {
		return model.ULID{}, false
	}
	u, err := model.ParseULID(parts[3])
	return u, err == nil
}

func addOnce(list *[]string, u model.ULID) {
	if s := u.String(); !slices.Contains(*list, s) {
		*list = append(*list, s)
	}
}

// Snapshot is the HEAD and the sync state of each store, for hello.
func (p *Poller) Snapshot() ([]storeState, int) {
	p.mu.Lock()
	empty := len(p.stores) == 0
	p.mu.Unlock()
	if empty {
		p.poll(false)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	out := []storeState{}
	for name, st := range p.stores {
		out = append(out, storeState{Store: name, Head: st.head.String(), Sync: st.sync})
	}
	slices.SortFunc(out, func(a, b storeState) int { return strings.Compare(a.Store, b.Store) })
	return out, p.unread
}
```

確かめて合わせること:
- `sameStatus(a, b syncStatus) bool` を書く（ポインタの時刻と数を値で比べる。`reflect.DeepEqual` でよい）
- コメントのファイルのパスの形を `model.CommentPath` で確かめ、`commentTask` を合わせる（model にパスからタスクを読む関数があればそれを使う）。worklogs/ も同じ形ならタスクの変更に入れる
- `LogFrom` の名前と引数は `internal/g*repo/log.go` の定義に合わせる
- HEAD が動いたのにタスクも schema も views も変わっていない（store.yaml やワークスペースだけの）ときも、`tasks.changed`（`ulids` が空）を送る。クライアントは HEAD の移動を知って、必要なら取り直す
- `Snapshot` の初回の `poll(false)` で `unread` も数える（`recount` は最初の1回で必ず真になる）

`internal/server/server.go`: `Server` に `hub *Hub`、`poller *Poller` を足し、`New` で作る。`s.loop.onState = func(string) { s.poller.Kick() }` にする（同期が終わったら状態をすぐ読み直す）。`Start` で `go s.poller.Run(ctx)`。`Shutdown` で `s.cancel()` の直後に `s.hub.Close()`（SSE の接続を終わらせる。`http.Server.Shutdown` が SSE を待ち続けないため）。`PATCH /notifications/{id}` の成功の後に `s.poller.Kick()` を足す（activity.go）。

Run: `go test -race ./internal/server/ -run TestPoll` → PASS

- [ ] **Step 8: SSE の失敗するテストを書く**

`internal/server/events_test.go`:

```go
package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// eventStream reads /api/events.
type eventStream struct {
	cancel context.CancelFunc
	resp   *http.Response
	events chan [2]string // name, data
}

func (ts *testServer) events() *eventStream {
	ts.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.hs.URL+"/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := ts.hs.Client().Do(req)
	if err != nil {
		cancel()
		ts.t.Fatal(err)
	}
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		cancel()
		ts.t.Fatalf("events: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	es := &eventStream{cancel: cancel, resp: resp, events: make(chan [2]string, 64)}
	go func() {
		defer close(es.events)
		sc := bufio.NewScanner(resp.Body)
		var name, data string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data += strings.TrimPrefix(line, "data: ")
			case line == "":
				if data != "" {
					es.events <- [2]string{name, data}
				}
				name, data = "", ""
			}
		}
	}()
	ts.t.Cleanup(es.Close)
	return es
}

func (es *eventStream) Close() { es.cancel(); _ = es.resp.Body.Close() }

// next returns the data of the first event named name, skipping others.
func (es *eventStream) next(t *testing.T, name string) json.RawMessage {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e, ok := <-es.events:
			if !ok {
				t.Fatalf("the stream ended before %s", name)
			}
			if e[0] == name {
				return json.RawMessage(e[1])
			}
		case <-timeout:
			t.Fatalf("no %s event", name)
		}
	}
}

func startedServer(t *testing.T) *testServer {
	t.Helper()
	ts := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	ts.srv.Start(ctx)
	t.Cleanup(func() { cancel(); _ = ts.srv.Shutdown(context.Background()) })
	return ts
}

func TestEventsHelloFirst(t *testing.T) {
	ts := startedServer(t)
	es := ts.events()
	var hello helloEvent
	if err := json.Unmarshal(es.next(t, "hello"), &hello); err != nil {
		t.Fatal(err)
	}
	if hello.Client == "" || len(hello.Stores) != 1 || hello.Stores[0].Store != "me" || hello.Stores[0].Head == "" {
		t.Fatalf("hello = %+v", hello)
	}
	if !ts.srv.presence.Visible() {
		t.Error("the connection is not visible")
	}
	ts.call("PUT", "/api/presence", map[string]any{"client": hello.Client, "visible": false}, 204, nil)
	if ts.srv.presence.Visible() {
		t.Error("still visible")
	}
	es.Close()
	eventually(t, "the client to go", func() bool {
		ts.srv.presence.mu.Lock()
		defer ts.srv.presence.mu.Unlock()
		return len(ts.srv.presence.clients) == 0
	})
}

func TestEventsAPIWrite(t *testing.T) {
	ts := startedServer(t)
	es := ts.events()
	es.next(t, "hello")
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	var e tasksChanged
	_ = json.Unmarshal(es.next(t, "tasks.changed"), &e)
	if e.Commit != *out.Commit || len(e.ULIDs) != 1 || e.ULIDs[0] != out.Task.ULID {
		t.Errorf("event = %+v", e)
	}
}

var (
	hhOnce sync.Once
	hhBin  string
	hhErr  error
)

// hhBinary builds hedgehug once for the tests that run it as another process.
func hhBinary(t *testing.T) string {
	t.Helper()
	hhOnce.Do(func() {
		dir, err := os.MkdirTemp("", "hh-server-test")
		if err != nil {
			hhErr = err
			return
		}
		hhBin = filepath.Join(dir, "hedgehug")
		if out, err := exec.Command("go", "build", "-o", hhBin, "github.com/kukv/hedgehug/cmd/hedgehug").CombinedOutput(); err != nil { //nolint:gosec // our own package
			hhErr = fmt.Errorf("%v: %s", err, out)
		}
	})
	if hhErr != nil {
		t.Fatal(hhErr)
	}
	return hhBin
}

func TestEventsCLIWrite(t *testing.T) {
	ts := startedServer(t)
	es := ts.events()
	es.next(t, "hello")
	cmd := exec.Command(hhBinary(t), "add", "from the CLI") //nolint:gosec // the binary the test built
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	for k, v := range ts.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hedgehug add: %v: %s", err, out)
	}
	var e tasksChanged
	_ = json.Unmarshal(es.next(t, "tasks.changed"), &e)
	if e.Op != "add" || len(e.ULIDs) != 1 {
		t.Errorf("event = %+v", e)
	}
}

func TestEventsReconnect(t *testing.T) {
	ts := startedServer(t)
	es := ts.events()
	var h1 helloEvent
	_ = json.Unmarshal(es.next(t, "hello"), &h1)
	es.Close()
	if _, err := ts.session().Add(ops.AddInput{Title: "while away"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the poller to see HEAD", func() bool {
		st, _ := ts.srv.poller.Snapshot()
		return st[0].Head != h1.Stores[0].Head
	})
	es = ts.events()
	var h2 helloEvent
	_ = json.Unmarshal(es.next(t, "hello"), &h2)
	if h2.Stores[0].Head == h1.Stores[0].Head || h2.Client == h1.Client {
		t.Errorf("hello after reconnect = %+v (before %+v)", h2, h1)
	}
}

func TestEventsEndOnShutdown(t *testing.T) {
	ts := startedServer(t)
	es := ts.events()
	es.next(t, "hello")
	_ = ts.srv.Shutdown(context.Background())
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-es.events:
			if !ok {
				return
			}
		case <-timeout:
			t.Fatal("the stream did not end on Shutdown")
		}
	}
}

func TestEventsNeedToken(t *testing.T) {
	ts := newTestServer(t)
	req, _ := http.NewRequest("GET", ts.hs.URL+"/api/events", nil)
	if resp := ts.raw(req); resp.StatusCode != 401 {
		t.Errorf("status %d", resp.StatusCode)
	}
}
```

（import に `fmt` と `internal/ops` を足す。Windows では `hhBin` に `.exe` を付ける。）

- [ ] **Step 9: 失敗を確かめる**

Run: `go test ./internal/server/ -run TestEvents`
Expected: FAIL（`/api/events` がない）

- [ ] **Step 10: events.go を書く**

```go
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"
)

// pingInterval keeps the connection open through proxies and browsers.
const pingInterval = 15 * time.Second

type helloEvent struct {
	Client string       `json:"client" doc:"This connection, for PUT /api/presence"`
	Stores []storeState `json:"stores" doc:"The HEAD and the sync state of each store now"`
	Unread int          `json:"unread"`
}

func (s *Server) registerEvents() {
	sse.Register(s.api, huma.Operation{
		OperationID: "events", Method: http.MethodGet, Path: "/api/events",
		Summary: "The changes, as server-sent events; hello comes first", Tags: []string{"events"},
	}, map[string]any{
		"hello":                helloEvent{},
		"tasks.changed":        tasksChanged{},
		"sync.state":           syncStatus{},
		"notifications.unread": unreadEvent{},
		"schema.changed":       schemaChanged{},
		"views.changed":        viewsChanged{},
	}, func(ctx context.Context, _ *struct{}, send sse.Sender) {
		events, stop := s.hub.Subscribe() // before hello: nothing after it is missed
		defer stop()
		id := s.presence.Connect()
		defer s.presence.Disconnect(id)
		stores, unread := s.poller.Snapshot()
		if err := send.Data(helloEvent{Client: id, Stores: stores, Unread: unread}); err != nil {
			return
		}
		ping := time.NewTicker(pingInterval)
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-events:
				if !ok {
					return
				}
				if err := send.Data(e); err != nil {
					return
				}
			case <-ping.C:
				if err := send.Comment("ping"); err != nil {
					return
				}
			}
		}
	})
}
```

`register` に `s.registerEvents()` を足す。トークンの検査（huma のミドルウェア）は SSE の操作にも効くことを `TestEventsNeedToken` で確かめている。`sse.Register` のイベント名の付け方（map のキーがそのまま `event:` になる）を huma の版で確かめる。`api/openapi.yaml` に `/api/events` と6つのイベントの型が出ることを目で確かめる。

- [ ] **Step 11: 生成して通ることを確かめる**

Run: `go generate ./internal/server && go test -race ./internal/server/ ./internal/inbox/`
Expected: PASS

- [ ] **Step 12: 全体の検査とコミット**

Run: `go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...`
Expected: すべて成功

```bash
git add api/openapi.yaml internal/server internal/inbox internal/store
git commit -m "feat(server): HEAD のポーリングと SSE のイベントを足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: `hh ui`（起動、ui.json、ブラウザ、停止）

**Files:**
- Create: `cmd/hedgehug/ui.go`, `cmd/hedgehug/ui_test.go`
- Modify: `cmd/hedgehug/main.go`（`env` に `uiContext`、`openURL` を `pkg/browser` に）、`cmd/hedgehug/app.go`（`uiCmd` の登録）、`internal/server/server.go`（`CloseEvents`）、`go.mod`・`go.sum`

**Interfaces:**
- Consumes: Task 2 の `server.New`・`Handler`・`SetPort`、Task 7・8 の `Start`・`Shutdown`、Task 1 の `config.Paths.DataDir`、`web.Dist()`、`atomicfile.WriteFile`
- Produces:
  - `hh ui [--port N] [--no-open]`
  - `type uiInfo struct { Port int; Token string; PID int; Version string }`（`ui.json`。キーは `port`・`token`・`pid`・`version`）
  - `(*server.Server).CloseEvents()` — SSE の接続を終わらせる（`hub.Close()`）
  - `env.uiContext func() (context.Context, context.CancelFunc)` — nil なら `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`。テストが止めるために差し替える

**手順（spec「起動とセキュリティ」と「この plan で埋める spec にない細部」26〜28）:**
1. `config.Resolve` の `DataDir` の `ui.json` を読む。読めて、`GET http://127.0.0.1:<port>/api/health`（`Authorization: Bearer <token>`、1 秒のタイムアウト）が 200 なら、URL を出し（`Hedgehug UI is running: <URL>`）、`--no-open` でなければブラウザを開いて終わる（終了コード 0）
2. そうでなければ `127.0.0.1:<--port>` で待ち受ける（0 は空いているポート）。使用中なら `listen tcp …` のエラーで終了コード 1
3. トークンは `crypto/rand` の 32 バイトを `base64.RawURLEncoding` にする
4. `server.New`（`Static: web.Dist()`、`Log: stderr`、`Version: version`、env の `getenv`・`now`・`keyring`・`httpClient`・`run`）、`SetPort`、`Start(ctx)`
5. `ui.json` を `atomicfile.WriteFile` で書き直す（一時ファイルは 0600 で作られる）
6. `Hedgehug UI: http://127.0.0.1:<port>/#token=<token>` を標準出力に出し、`--no-open` でなければ開く。開けなければ `warning: the browser did not open: <err>` を標準エラーに出して続ける
7. `ctx` が終わったら（Ctrl-C、SIGTERM。Windows の Ctrl-Break も `os.Interrupt` で届く）: `CloseEvents()` → `http.Server.Shutdown`（5 秒）→ `Shutdown`（push 待ち。最大 10 秒）→ `ui.json` の pid が自分なら消す

- [ ] **Step 1: 依存を足す**

```bash
go get github.com/pkg/browser@latest
```

- [ ] **Step 2: 失敗するテストを書く**

`cmd/hedgehug/ui_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer safe to read while the command writes.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

var uiURL = regexp.MustCompile(`http://127\.0\.0\.1:(\d+)/#token=([A-Za-z0-9_-]+)`)

// runningUI is hh ui running in the background.
type runningUI struct {
	stop   context.CancelFunc
	done   chan int
	stdout *syncBuffer
	port   int
	token  string
	opened []string
}

func (te *testEnv) startUI(args ...string) *runningUI {
	te.t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	u := &runningUI{stop: stop, done: make(chan int, 1), stdout: &syncBuffer{}}
	var mu sync.Mutex
	e := &env{
		stdin: strings.NewReader(""), stdout: u.stdout, stderr: &syncBuffer{},
		getenv: te.getenv, isTerminal: func(any) bool { return false },
		now:     time.Now,
		openURL: func(s string) error { mu.Lock(); u.opened = append(u.opened, s); mu.Unlock(); return nil },
		uiContext: func() (context.Context, context.CancelFunc) { return ctx, stop },
	}
	go func() { u.done <- run(append([]string{"ui"}, args...), e) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if m := uiURL.FindStringSubmatch(u.stdout.String()); m != nil {
			u.port, _ = strconv.Atoi(m[1])
			u.token = m[2]
			break
		}
		select {
		case code := <-u.done:
			te.t.Fatalf("hh ui exited %d: %s", code, u.stdout.String())
		default:
		}
		if time.Now().After(deadline) {
			te.t.Fatalf("hh ui printed no URL: %q", u.stdout.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	te.t.Cleanup(func() { u.stop(); <-u.done })
	return u
}

func health(t *testing.T, port int, token string) int {
	t.Helper()
	req, _ := http.NewRequest("GET", "http://127.0.0.1:"+strconv.Itoa(port)+"/api/health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func (te *testEnv) uiJSON() (map[string]any, os.FileInfo) {
	te.t.Helper()
	p := filepath.Join(te.hhHome, "ui.json")
	st, err := os.Stat(p)
	if err != nil {
		return nil, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		te.t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		te.t.Fatal(err)
	}
	return m, st
}

func TestUIStartsAndStops(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	u := te.startUI("--no-open")
	if code := health(t, u.port, u.token); code != 200 {
		t.Fatalf("health = %d", code)
	}
	if len(u.token) != 43 {
		t.Errorf("token %q, want 43 characters (32 bytes)", u.token)
	}
	if len(u.opened) != 0 {
		t.Errorf("--no-open opened %v", u.opened)
	}
	info, st := te.uiJSON()
	if info == nil || int(info["port"].(float64)) != u.port || info["token"] != u.token || int(info["pid"].(float64)) != os.Getpid() {
		t.Fatalf("ui.json = %v", info)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Errorf("ui.json mode %v, want 0600", st.Mode().Perm())
	}
	u.stop()
	if code := <-u.done; code != 0 {
		t.Errorf("exit %d", code)
	}
	u.done <- 0 // for the Cleanup
	if info, _ := te.uiJSON(); info != nil {
		t.Errorf("ui.json left: %v", info)
	}
}

func TestUIOpensTheBrowser(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	u := te.startUI()
	if len(u.opened) != 1 || !strings.Contains(u.opened[0], "#token="+u.token) {
		t.Errorf("opened %v", u.opened)
	}
}

func TestUIReusesTheRunningServer(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	u := te.startUI("--no-open")
	var opened []string
	te.openURL = func(s string) error { opened = append(opened, s); return nil }
	code, out, errs := te.run("ui")
	if code != 0 || !strings.Contains(out, "Hedgehug UI is running: ") || !strings.Contains(out, "#token="+u.token) {
		t.Fatalf("second hh ui: %d %q %q", code, out, errs)
	}
	if len(opened) != 1 {
		t.Errorf("opened %v", opened)
	}
}

func TestUIOverwritesAStaleUIJSON(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	stale := `{"port":` + strconv.Itoa(dead) + `,"token":"old","pid":1,"version":"x"}`
	if err := os.WriteFile(filepath.Join(te.hhHome, "ui.json"), []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	u := te.startUI("--no-open")
	info, _ := te.uiJSON()
	if info["token"] != u.token {
		t.Errorf("ui.json = %v, want the new server", info)
	}
}

func TestUIPortInUse(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	code, _, errs := te.run("ui", "--no-open", "--port", port)
	if code != 1 || !strings.Contains(errs, "listen tcp") {
		t.Errorf("exit %d, stderr %q", code, errs)
	}
}
```

`testEnv` に `uiContext` はないので、`te.run("ui")`（2回目の起動）は既存のサーバーを開いてすぐ終わる場合だけに使う。`te.ok("store", "init", …)` のフラグは今の `hh store init` の定義に合わせる。

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./cmd/hedgehug/ -run TestUI`
Expected: FAIL（`env.uiContext` が未定義）

- [ ] **Step 4: ui.go を書く**

```go
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/server"
	"github.com/kukv/hedgehug/web"
)

// uiInfo is ui.json: how to reach the server that runs, for the next hh ui.
type uiInfo struct {
	Port    int    `json:"port"`
	Token   string `json:"token"`
	PID     int    `json:"pid"`
	Version string `json:"version"`
}

func (u uiInfo) url() string {
	return "http://127.0.0.1:" + strconv.Itoa(u.Port) + "/#token=" + u.Token
}

func (a *app) uiCmd() *cobra.Command {
	var port int
	var noOpen bool
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Open the GUI in the browser",
		Long:  "Ui serves the GUI on 127.0.0.1 and opens it in the browser. When one runs already, it opens that one. Stop it with Ctrl-C.",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return a.ui(port, noOpen)
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "the port to listen on; 0 picks a free one")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "print the URL and do not open the browser")
	return cmd
}

func (a *app) ui(port int, noOpen bool) error {
	paths, err := config.Resolve(a.getenv)
	if err != nil {
		return err
	}
	file := filepath.Join(paths.DataDir, "ui.json")
	if info, ok := runningUI(file); ok {
		_, _ = fmt.Fprintln(a.stdout, "Hedgehug UI is running: "+info.url())
		a.open(info.url(), noOpen)
		return nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		_ = ln.Close()
		return err
	}
	info := uiInfo{Port: ln.Addr().(*net.TCPAddr).Port, Token: base64.RawURLEncoding.EncodeToString(b), PID: os.Getpid(), Version: version}
	srv := server.New(server.Options{
		Getenv: a.getenv, Now: a.now, Keyring: a.keyring, HTTPClient: a.httpClient, Run: a.env.run,
		Token: info.Token, Version: version, Static: web.Dist(), Log: a.stderr,
	})
	srv.SetPort(info.Port)
	data, err := json.Marshal(info)
	if err == nil {
		err = atomicfile.WriteFile(file, append(data, '\n'))
	}
	if err != nil {
		_ = ln.Close()
		return err
	}
	defer removeOwnUIJSON(file, info.Token)

	ctx, stop := a.uiContextOrSignals()
	defer stop()
	srv.Start(ctx)
	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	served := make(chan error, 1)
	go func() { served <- hs.Serve(ln) }()

	_, _ = fmt.Fprintln(a.stdout, "Hedgehug UI: "+info.url())
	a.open(info.url(), noOpen)

	select {
	case <-ctx.Done():
	case err := <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			_ = srv.Shutdown(context.Background())
			return err
		}
	}
	srv.CloseEvents()
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = hs.Shutdown(shutCtx)
	return srv.Shutdown(context.Background())
}

// open opens url in the browser, unless noOpen.
func (a *app) open(url string, noOpen bool) {
	if noOpen || a.openURL == nil {
		return
	}
	if err := a.openURL(url); err != nil {
		_, _ = fmt.Fprintln(a.stderr, "warning: the browser did not open:", err)
	}
}

// uiContextOrSignals is the context hh ui runs until: the test's, or one
// that Ctrl-C (Ctrl-Break on Windows) and SIGTERM end.
func (a *app) uiContextOrSignals() (context.Context, context.CancelFunc) {
	if a.uiContext != nil {
		return a.uiContext()
	}
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// runningUI reads ui.json and reports whether the server it names answers.
func runningUI(file string) (uiInfo, bool) {
	var info uiInfo
	b, err := os.ReadFile(file) //nolint:gosec // our own file in the data directory
	if err != nil || json.Unmarshal(b, &info) != nil || info.Port == 0 {
		return info, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(info.Port)+"/api/health", nil)
	if err != nil {
		return info, false
	}
	req.Header.Set("Authorization", "Bearer "+info.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return info, false
	}
	_ = resp.Body.Close()
	return info, resp.StatusCode == http.StatusOK
}

// removeOwnUIJSON removes ui.json when it is still this server's (its pid and
// token): a later hh ui may have written its own.
func removeOwnUIJSON(file, token string) {
	var info uiInfo
	b, err := os.ReadFile(file) //nolint:gosec // our own file in the data directory
	if err != nil || json.Unmarshal(b, &info) != nil || info.PID != os.Getpid() || info.Token != token {
		return
	}
	_ = os.Remove(file)
}
```

テストは同じプロセスで何度も `hh ui` を起動して pid が同じになるので、`removeOwnUIJSON` は pid とトークンの両方を比べる。

`cmd/hedgehug/main.go`:
- `env` に `uiContext func() (context.Context, context.CancelFunc) // for tests: what hh ui runs until; nil is Ctrl-C and SIGTERM` を足す
- `openURL` の中身を `browser.OpenURL(u)` にし、`browser.Stdout`・`browser.Stderr` を `io.Discard` にする（開くコマンドの出力を CLI に混ぜない）。`runtime` と `os/exec` の import が要らなくなれば消す

`cmd/hedgehug/app.go` の `rootCmd` のサブコマンドの登録に `a.uiCmd()` を足す（ほかの登録と同じ並びの決まりに合わせる）。`hh ui` は `beforeCommand`（バックグラウンド同期の起動）の対象から外す（サーバー自身が同期するため）。外し方は `sync`・`merge-driver` と同じ判定に `ui` を足す。

`internal/server/server.go` に足す:

```go
// CloseEvents ends the connections to /api/events, so that the HTTP server
// can shut down; the GUI connects again to the next server.
func (s *Server) CloseEvents() { s.hub.Close() }
```

- [ ] **Step 5: 通ることを確かめる**

Run: `go test ./cmd/hedgehug/ -run TestUI && go test ./cmd/hedgehug/`
Expected: PASS。`hh --help` のゴールデンがあるなら、`ui` の行の分だけ期待値を更新する（`-update` のあと `git diff --stat -- cmd/hedgehug/testdata` で `ui` の行以外の差分がないことを確かめる）

- [ ] **Step 6: 手で動かす**

```bash
go build -o bin/hedgehug ./cmd/hedgehug
HEDGEHUG_HOME=$(mktemp -d) sh -c './bin/hedgehug store init me --prefix ME && ./bin/hedgehug ui --no-open & sleep 2; kill -INT $!; wait'
```

Expected: `Hedgehug UI: http://127.0.0.1:…/#token=…` が出て、Ctrl-C（SIGINT）で終了コード 0 で終わる。ブラウザで URL を開くと「GUI がビルドされていません。make web を実行してください。」のページが出る（`--no-open` を外して確かめる）。

- [ ] **Step 7: 全体の検査とコミット**

Run: `go mod tidy && go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...`
Expected: すべて成功

```bash
git add go.mod go.sum cmd/hedgehug internal/server
git commit -m "feat(cli): hh ui でサーバーを起動し、ui.json で既存のサーバーを再利用する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: 仕上げ（全操作のテスト・README・最終の検査）

**Files:**
- Modify: `README.md`・`README.ja.md`（コマンドの一覧があれば `hh ui`）
- Test: 足りない操作のテストを、その操作のテストファイルに足す

spec への決定の反映は、この plan と同じコミットで済ませてある（`hello`・`?store=`・`/move`・`/restore`・`confirm=`・PATCH の範囲）。

- [ ] **Step 1: 全操作にテストがあることを確かめる**

```bash
grep -ho 'OperationID: "[a-z-]*"' internal/server/*.go | sort -u
```

出た操作ごとに、`internal/server/*_test.go` にその方法とパスを呼ぶテスト（正常系と、ops のエラーの型・HTTP の状態）があることを確かめる。`testServer.call` は呼んだ操作の応答を毎回 `api/openapi.yaml` で検証するので、呼んでいない操作だけが検証から漏れる。足りない操作があればテストを足す。

- [ ] **Step 2: README を直す**

`grep -n "hh view\|hedgehug view" README.md README.ja.md` でコマンドの一覧の場所を探し、`hh ui [--port] [--no-open]`（英語は "Open the GUI in the browser"、日本語は「GUI をブラウザで開く」）を同じ形で足す。一覧がなければ何もしない。

- [ ] **Step 3: 最終の検査**

```bash
go generate ./internal/server
git diff --exit-code -- api/openapi.yaml
go mod tidy && git diff --exit-code -- go.mod go.sum
go test -race ./...
golangci-lint run ./...
golangci-lint fmt --diff ./...
GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/hedgehug
GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/hedgehug
```

Expected: すべて成功、差分なし

- [ ] **Step 4: コミット**

```bash
git add README.md README.ja.md internal/server
git commit -m "docs: README に hh ui を足し、全操作のテストをそろえる

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

## 自己レビューの結果（plan を書いた時点）

| spec の項目 | タスク |
|---|---|
| API の表（全資源） | Task 3〜7 |
| `{id}` は ID・以前の ID・ULID | Task 3（`TestShowTask` で ULID） |
| 1回の PATCH は1コミット、送ったフィールドだけ | Task 1・4 |
| 書き込みの応答に変わったタスクとコミット | Task 4〜6 |
| タスクの応答に checklist と progress | Task 3（一覧・ツリー・詳細）、Task 4（`listView`） |
| problem+json、`type` に Code、引数 | Task 2 |
| OpenAPI 3.1 の生成とコミット、CI で差分なし | Task 2（`TestOpenAPIUpToDate`）、Task 10 |
| SSE（300ms、Refresh、5つのイベント、最初に今の状態） | Task 8 |
| 同期ループ（表示・非表示の間隔、起動時、書き込み後の push、ロック） | Task 7 |
| 表示の状態（`PUT /presence`） | Task 7・8 |
| `hh ui`、127.0.0.1、ポート、トークン、URL、sessionStorage | Task 9（URL の形。sessionStorage は 4d） |
| Bearer の必須、Host の検査 | Task 2 |
| `ui.json`（0600、再利用、書き直し、終了時に消す） | Task 9 |
| 前面で動き Ctrl-C で止まる、push を最大 10 秒待つ | Task 7（`Drain`）、Task 9 |
| 埋め込み、`index.html` の返し、未ビルドのページ | Task 2 |
| テスト（全エンドポイント、OpenAPI の照合、SSE と別プロセス、ループ、ui.json） | Task 2〜10 |
