# フェーズ 4e-3: カンバン・テーブル・保存済みビュー・マイタスク Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** カンバンの移動、テーブルのセル編集、保存済みビューの保存・開く・改名・削除とレイアウトの切り替え、保存していないクエリの画面、マイタスクを GUI で動かす。

**Architecture:** フェーズ 4e の4回のうちの3回目。決定事項と spec にない細部は概要 `docs/superpowers/plans/2026-11-03-phase4e-screens.md` にあり（この回は細部 36〜58 と、細部 16・17・34 のうち 4e-3 に送った部分）、各 Task はそこを前提にする（細部の番号はそこを指す）。この回で決めたことは末尾の「決めたこと」（D1〜D10 と 11〜21）にある。Go は Task 1 の `hh view show` の評価範囲の直しだけで、ほかは web。フロントとバックは REST で分け（`web/core` はフレームワークに依存しない）、画面は Svelte 5 の runes。Task 2 で骨組み（ルート、ビューの一覧の `ViewDirectory`、CLI の文字列、画面の範囲 `ListScope`、フィルタの段 `ListBar`、TanStack Table の追加）を作り、Task 3 でツリーを `ListScope` と外から渡すクエリに対応させ、Task 4・5 でカンバン・テーブルの部品、Task 6 でそれを入れるビューの画面・クエリの画面・ワークスペースとプロジェクトのタブの入れ物とビューのダイアログを作り、Task 7 でマイタスクと「もしかして」を作って入口（タブ・サイドバー・パレット・App の振り分け）を結線する。

**Tech Stack:** Go（cobra、ゴールデンテスト）、TypeScript、Svelte 5（runes）、Vite、TanStack Table 9（`@tanstack/svelte-table` 9.3.1、exact）・TanStack Virtual、Pragmatic drag and drop、Vitest と @testing-library/svelte、Playwright（`web/e2e`、`harness.ts`）、Biome、svelte-check。

**Spec:** `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（「画面と範囲」の Kanban・Table・MyTasks・Main・Dialogs・Sidebar・CommandPalette・States）と、概要 `docs/superpowers/plans/2026-11-03-phase4e-screens.md`、4d の概要 `docs/superpowers/plans/2026-11-02-phase4d-web.md`。親設計（Claude Docs の Hedgehug 設計ドキュメント）の「保存済みビュー」「複数 Store の見せ方」「統合ビューの細部」「プロジェクトごとの上書き」。

**この回の Task:** Task 1（Go: `hh view show` を全 Store で評価）、Task 2（骨組み）、Task 3（ツリーを `ListScope` に対応）、Task 4（カンバン）、Task 5（テーブル）、Task 6（ビューの画面・クエリの画面・タブの入れ物・ビューのダイアログ）、Task 7（マイタスク・もしかして・結線・E2E）。番号の順に実行する。Task 2〜6 の画面は Task 7 の結線まで入口が出ないので、E2E の全件は Task 7 で流す。

**着手の前に:** main を取り込み、各 Task の Files にある関数・型がまだその場所と名前であることを確かめる（動いていれば、コードをその時点の形に合わせ直す。API の形と画面の言葉は変えない）。この plan は組み上げで確かめてある: main（79a5277）の写し1つ（ブランチ `integ-4e3`）に Task 1→7 の Step を順にそのまま当て、各 Task の終わりに検査（Go: `go test -race`・`go vet`・`gofmt`、web: `pnpm fix`・`pnpm check`（警告 0）・`biome ci`（指摘 0）・`vitest run` 全件）を流した。最後に `go test -race ./...`（全パッケージ ok）、`vitest` 全件（120 ファイル・1238 件）、`pnpm build`、`make e2e` の全件を 3 回（1 回目は 4e-3 と関係のない既存の `live.spec.ts` の1件が負荷で落ち、2・3 回目は 83 件すべて PASS）。食い違って plan を直した点は末尾の「組み上げで直した点」にある（後の Task を前の Task の実物に合わせた）。

## Global Constraints

4d の概要 plan と 4e の概要 plan の Global Constraints をすべて引き継ぐ。

- 画面の文言はすべて日本語。コードの識別子・コメントは英語（既存のコードと同じ）
- コンポーネントは色・寸法の 16 進数や px を直接書かず、`web/app/styles/tokens.css` の CSS 変数を使う（Foundations: 「色はトークン名で実装し、画面に 16 進数を直書きしない」）
- ステータスは色だけでなく形で区別する（Foundations のステータスの形）
- 行とカードは ID だけを出し、ULID は ID にカーソルを合わせたときに出してコピーできるようにする。「キー未確定」・短縮 ID・「次の同期で確定」は作らない
- 複数 Store のタスクが並ぶ画面では、タスクごとに Store のバッジを出す（親設計「タスクごとに、Store の色とバッジを常に表示する」）
- フォントは `@fontsource` で同梱し、外部に通信しない。ダークモードは作らない
- キーボード: ⌘K コマンドパレット、C タスクを作成、⇧⌘S 今すぐ同期、Enter 下に追加（ツリー）、Tab / ⇧Tab 字下げ・戻す、⌥↑↓ 並べ替え、Space 完了、S ステータス、K 種類、⌘↵ 詳細を開く・作成。Windows と Linux では ⌘ を Ctrl にする
- 保存は入力が 2 秒止まったとき、またはフォーカスが外れたときに `PATCH`。1操作1コミット
- トーストは左下に 6 秒、取り消し付きは 10 秒。取り消しにくい操作だけ確認ダイアログ。undo は確認しない
- ダイアログとフィルタ欄の足元には同じ操作の `hh` コマンドを出し、コピーできるようにする
- 依存は exact で固定し、公開から 7 日以上の版だけ（`pnpm-workspace.yaml` の `minimumReleaseAge`）。この回で足すのは `@tanstack/svelte-table` 9.3.1 だけ
- `svelte-check` は警告ゼロ、`biome ci` は指摘ゼロ。Go のテストは `-race` を含めて通す。Go のジョブは `web/dist` なし（`.gitkeep` だけ）で通る
- spec・設計と違う挙動にしたくなったら、実装を止めて報告する（自分で決めない）
- まだない画面への入口は出さない。この回で出さないのは、通知・アクティビティのサイドバーの行、プロジェクトの設定タブ、設定の同期と保守・スキーマ（4e-4）。後のフェーズの画面と項目（ガント・バックログ・スプリント・リリース・レポート、WIP 上限、カードの PR、一括編集、アカウント・Store・自動化の設定）も出さない。出さないことはテストで確かめる
- キャンバスの CLI の表記が実際のコマンドと違うときは実際のコマンドに合わせる
- テストは前面で実行し、Bash の timeout は 600000。落ちたテストは失敗の出力と実行条件を残す
- コミットのメッセージの末尾は `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`

---
### Task 1: `hh view show` を全 Store で評価する（D2）

保存済みビューは「常に統合する」。GUI のビューは全マウント Store で評価するので、CLI の `hh view show <名前>` も同じ結果にする。今は `--store` がビューを読む場所とクエリを評価する Store の両方を決めていて、2 Store のとき GUI（Task 6 の `ViewPage`）と結果が食い違う。`--store` はビューを読む場所だけにし、クエリはマウントされた全 Store で評価する（クエリの `store:` で絞れる）。

**Files:**
- Modify: `cmd/hedgehug/view.go`（`viewShow`）
- Modify: `cmd/hedgehug/view_test.go`（`TestViewShowAcrossStores`）
- Create: `cmd/hedgehug/testdata/TestViewShowAcrossStores/{table,tree,narrowed,saved-in-bravo}.golden`

**Interfaces:**
- Consumes: `ops.Session.ViewGet`（`--store` の Store からビューを読む）、`ops.Session.ForStore("")`（`StoreName` が空の同じ外界のセッション。マウントのキャッシュは共有）、`app.ls` / `app.lsTree`（`a.ss` を使う）、`newTwoStoreEnv`（`ls_stores_test.go`。alpha=AA・bravo-long=BB の 2 Store）、`te.goldenOK` / `te.ok` / `te.run`
- Produces: `hh view show <名前> [--store S]`: ビューは S（なければ既定の Store）から読み、タスクはクエリを全 Store で評価して出す。`--json` の形・列・ツリーの形は `hh ls` / `hh ls --tree` と同じ（Store が 2 つ以上のとき STORE 列つき）。親設計の「保存済みビュー」に「`hh view show` は全 Store で評価する」を1行足すのは controller が行う（この plan では触らない）

**確かめたこと（書き手）**: この節のコードは、リポジトリの写し（rsync）に当てて `go test ./cmd/hedgehug`（全件）・`gofmt -l`・`go vet` を通した。`ForStore("")` の行を戻すと、新しいテストが失敗する（`table` は bravo-long の行が出ず、`narrowed` は空になる）ことも確かめた。

- [ ] **Step 1: 失敗するテストを書く**

`cmd/hedgehug/view_test.go` の差分（`@@` の行は目安。該当する箇所に当てる）:

```diff
@@ -117,3 +117,35 @@
 		t.Error("show in me found a view of the other store")
 	}
 }
+
+// --store names the store a view is saved in; view show lists the tasks of its
+// query on every store, and store: in the query narrows it.
+func TestViewShowAcrossStores(t *testing.T) {
+	t.Parallel()
+	te := newTwoStoreEnv(t)
+	te.ok("--store", "alpha", "view", "save", "open", "--layout", "table", "--query", "status:todo")
+	te.ok("--store", "alpha", "view", "save", "all-tree", "--layout", "tree")
+	te.ok("--store", "alpha", "view", "save", "only-bravo", "--query", "store:bravo-long")
+	te.ok("--store", "bravo-long", "view", "save", "from-bravo", "--layout", "table", "--query", "kind:task")
+	te.goldenOK("table", "--store", "alpha", "view", "show", "open")
+	te.goldenOK("tree", "--store", "alpha", "view", "show", "all-tree")
+	te.goldenOK("narrowed", "--store", "alpha", "view", "show", "only-bravo")
+	te.goldenOK("saved-in-bravo", "--store", "bravo-long", "view", "show", "from-bravo")
+
+	var got []ops.TaskView
+	if err := json.Unmarshal([]byte(te.ok("--store", "alpha", "view", "show", "open", "--json")), &got); err != nil {
+		t.Fatal(err)
+	}
+	if len(got) != 3 || got[0].Store != "alpha" || got[2].Store != "bravo-long" {
+		t.Errorf("view show --json = %+v; want the 3 todo tasks of both stores", got)
+	}
+	// a view is read from the store named, and from no other
+	if code, _, _ := te.run("--store", "bravo-long", "view", "show", "open"); code == 0 {
+		t.Error("view show in bravo-long found a view of alpha")
+	}
+	// the same tasks as hh ls with the query of the view
+	shown := te.ok("--store", "alpha", "view", "show", "open")
+	if want := te.ok("ls", "status:todo"); shown != want {
+		t.Errorf("view show differs from ls status:todo:\n%s\nvs\n%s", shown, want)
+	}
+}
```


ゴールデンファイルを置く（`go test -update` で作ってもよい。ULID は `<ULID-n>` に正規化される）。

`cmd/hedgehug/testdata/TestViewShowAcrossStores/table.golden`:

```text
STORE       ID    ULID                        STATUS  KIND     ASSIGNEES  LABELS  TITLE
alpha       AA-1  <ULID-1>  todo    project                     Alpha project
alpha       AA-2  <ULID-2>  todo    task                        Alpha child
bravo-long  BB-2  <ULID-3>  todo    task                        Bravo other
```

`cmd/hedgehug/testdata/TestViewShowAcrossStores/tree.golden`:

```text
== alpha
AA-1 <ULID-1>  todo  project  Alpha project  0/1
└─ AA-2 <ULID-2>  todo  task  Alpha child

== bravo-long
BB-1 <ULID-3>  done  task  Bravo task
BB-2 <ULID-4>  todo  task  Bravo other
```

`cmd/hedgehug/testdata/TestViewShowAcrossStores/narrowed.golden`:

```text
== bravo-long
BB-1 <ULID-1>  done  task  Bravo task
BB-2 <ULID-2>  todo  task  Bravo other
```

`cmd/hedgehug/testdata/TestViewShowAcrossStores/saved-in-bravo.golden`:

```text
STORE       ID    ULID                        STATUS  KIND  ASSIGNEES  LABELS  TITLE
alpha       AA-2  <ULID-1>  todo    task                     Alpha child
bravo-long  BB-1  <ULID-2>  done    task                     Bravo task
bravo-long  BB-2  <ULID-3>  todo    task                     Bravo other
```


- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/hedgehug -run TestViewShowAcrossStores -count=1`
Expected: FAIL（`table` と `tree` に bravo-long の行がなく、`narrowed` は空、`--json` は 2 件）。

- [ ] **Step 3: 実装する**

`cmd/hedgehug/view.go` の差分（`@@` の行は目安。該当する箇所に当てる）:

```diff
@@ -59,8 +59,11 @@
 		&cobra.Command{
 			Use:   "show <name>",
 			Short: "List the tasks of a saved view",
-			Args:  exactArgs(1),
-			RunE:  func(_ *cobra.Command, args []string) error { return a.viewShow(args[0]) },
+			Long: "List the tasks of a saved view.\n\n" +
+				"--store names the store the view is saved in. The query runs on every\n" +
+				"store, as the GUI shows a view; store: in the query narrows it.",
+			Args: exactArgs(1),
+			RunE: func(_ *cobra.Command, args []string) error { return a.viewShow(args[0]) },
 		},
 		&cobra.Command{
 			Use:   "rm <name>",
@@ -134,6 +137,11 @@
 	if err != nil {
 		return err
 	}
+	// --store says where the view is read from. The tasks are the query's, on
+	// every store, as the GUI shows a view; store: in the query narrows it.
+	own := a.ss
+	a.ss = own.ForStore("")
+	defer func() { a.ss = own }()
 	if v.Layout == "tree" {
 		return a.lsTree(q, false, nil)
 	}
```


`a.ss` を差し替えるのは、`ls` / `lsTree` が `a.ss` を読むため（引数を足すと `hh ls` の呼び出しすべてに触る）。`defer` で戻す。`--store` のフラグは元の `a.ss.StoreName` に結びついたままなので、差し替えの影響を受けない。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./cmd/hedgehug -count=1 && gofmt -l cmd/hedgehug && go vet ./cmd/hedgehug`
Expected: PASS、`gofmt -l` は何も出さない。既存の `TestViewCommands`・`TestViewStoreFlag`（`--store me view show only-other` が見つからない、を含む）はそのまま通る。`view show` の `--help` の Short は変えない（`TestViewCommands/no-sub` のゴールデンが Short を含むため）。

- [ ] **Step 5: コミット**

```bash
git add cmd/hedgehug/view.go cmd/hedgehug/view_test.go cmd/hedgehug/testdata/TestViewShowAcrossStores
git commit -m "fix(cli): hh view show はクエリを全 Store で評価する

--store はビューを読む Store だけを決める。GUI の保存済みビューと同じ結果になる。

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 2: 骨組み（依存・ルート・ビューのキャッシュ・CLI の文字列・ListScope・ListBar）

4e-3 の 3 画面（ビュー・クエリ・マイタスク）とカンバン・テーブルが共有するものを先に作る。画面への入口（タブ・サイドバー・パレット・App の振り分け）は出さない（Task 7 で結線する）。この Task は単独で通る。

**Files:**
- Modify: `web/package.json`、`web/pnpm-lock.yaml`（`@tanstack/svelte-table` 9.3.1）
- Create: `web/core/views.ts`、`web/core/views.test.ts`
- Modify: `web/core/cli.ts`、`web/core/cli.test.ts`
- Modify: `web/app/context.ts`、`web/app/core-factory.ts`、`web/app/core-factory.test.ts`、`web/app/test-utils.ts`
- Modify: `web/app/router.ts`、`web/app/router.test.ts`
- Create: `web/app/list/scope.ts`、`web/app/list/scope.svelte.ts`、`web/app/list/scope.svelte.test.ts`
- Create: `web/app/list/ListBar.svelte`、`web/app/list/ListBar.test.ts`
- Modify: `web/app/tree/FilterBar.svelte`、`web/app/tree/FilterBar.test.ts`（`base` が空なら固定クエリの札を出さない）

**Interfaces:**
- Consumes（main にあるもの）: `Api`・`unwrap`・`Schemas`（`web/core/api/client.ts`）、`Connection.on("views.changed" | "resync")`、`StoreDirectory.stores()/tone()/loaded()/subscribe()`、`TaskCache.noteOwnCommit()/task()/watchDetail()`、`SchemaCache.get()/load()`、`useSubscribe`、`core()`、`baseQuery`（`tree/filter.ts`）、`quoteArg`・`withPositionals`（`core/cli.ts`）、`FilterBar`、`FakeConn`・`fakeApi`・`problem`・`settle`（`core/testing.ts`）、`emitter`（`app/test-utils.ts`）、`entry`（`app/workspace/testkit.ts`）
- Produces（他の部はこの名前と型を使う）:
  - `web/app/router.ts`:
    ```ts
    export type Layout = "tree" | "kanban" | "table"; // core/views.ts の型の再 export
    export type WorkspaceTab = "overview" | "tree" | "kanban" | "table" | "settings";
    export type ProjectTab = "tree" | "kanban" | "table";
    export type Route =
      | { name: "home" }
      | { name: "workspace"; store: string; ulid: string; tab: WorkspaceTab }
      | { name: "project"; ulid: string; tab: ProjectTab }
      | { name: "task"; ulid: string }
      | { name: "view"; store: string; view: string }
      | { name: "query"; layout: Layout; q: string }
      | { name: "my" }
      | ({ name: "settings" } & SettingsPage);
    export function parseRoute(pathname: string, search?: string): Route; // search は /query/<layout> の ?q= だけに使う
    router.replace(path: string): void; // go(path, { replace: true })。履歴を積まず、購読者には知らせる
    ```
    パス: `/workspaces/<s>/<u>/{tree,kanban,table,settings}`、`/projects/<u>`（tree）・`/projects/<u>/{kanban,table}`、`/views/<s>/<名前>`、`/query/<layout>?q=`（q が空なら `?q=` なし）、`/my`。`router.go` はクエリ画面の `?q=` だけ残し、ほかのパスの query・hash は捨てる
  - `web/core/views.ts`: `Layout`・`SavedView`・`ViewSaveBody`・`KanbanSettings`・`TableSettings`・`ViewDraft`・`DEFAULT_KANBAN`・`DEFAULT_TABLE`・`draftOf(v)`・`saveBodyOf(d)`・`sameDraft(a, b)`（layout を除き、query は前後の空白を除いて比べる）・`ViewEntry = { store; tone; view }`・`class ViewDirectory`（`list()`/`get(store, name)`/`loaded()`/`subscribe()`/`save(store, name, body)`/`rename(store, name, to)`/`remove(store, name)`。型は概要の通り）。`save`・`rename` は応答の view を返し、一覧にも即座に反映する。書き込みの応答の commit は `cache.noteOwnCommit`。`loaded()` は Store 一覧が分かり、各 Store の初回の取得が終わった（失敗も終わりに数える）とき true
  - `AppCore.views: ViewDirectory`（`context.ts`・`core-factory.ts`）。`fakeCore()` は `views: { list: () => [], get: () => undefined, loaded: () => true, subscribe }` を持つ
  - `web/core/cli.ts`: `cliViewSave(i: { name: Arg; store?: string; body: ViewSaveBody })`（body にある項目だけ `--layout --query --columns a,b --group-by --kanban-columns --swimlane` を書き、`--store` は最後。`store` が空なら付けない）、`cliViewShow(name: string, store?: string)`、`cliViewRm(name: string, store?: string)`、`cliViewRename(name: string, to: Arg, store?: string)`。`--store` は `cliAdd`・`cliLs` と同じく「渡されたときだけ」付ける（付けるかどうかは呼び出し側が決める）
  - `web/app/list/scope.ts`: `ListScope`（概要の通り）
  - `web/app/list/scope.svelte.ts`: `useListScope(get: () => ListScope)`。コンポーネントの初期化中に呼ぶ。`router` は import しない。返す getter: `ready: boolean`（all は常に true。workspace/project は解決できたとき）、`notFound: boolean`（ワークスペースがない、またはプロジェクトの取得が 404）、`error: ApiError | null`（プロジェクトの取得の 404 以外のエラー。TreeView の今の `projectError`）、`apiBase: string`・`cliBase: string`（all と、ready でないときは `""`。API はプロジェクトを ULID、cli は ID）、`store: string | undefined`、`workspace: WorkspaceEntry | undefined`、`project: TaskView | undefined`、`saveBase: string`（D8: cliBase。マウントされた Store が 2 つ以上なら先頭に `store:<Store> `。all と ready でないときは `""`）、`schemas(stores: string[]): SchemaView[]`（読み込み済みのものを渡した順に。ないものは `schema.load` を始める。スキーマのキャッシュが通知したら読み直される）
  - `web/app/list/ListBar.svelte`: props `{ base: string; applied: string; count: string; cli: string; error: unknown; onchange: (q: string) => void; controls?: Snippet; actions?: Snippet }`。`FilterBar` の右に `controls`、さらに右に `actions`
  - `web/app/tree/FilterBar.svelte`: `base` が `""` のとき固定クエリの札（`title="この画面で固定のクエリ"`）を出さない。それ以外は変えない

**この Task で決めたこと**
- `Layout` の型は `core/views.ts` に置き、`router.ts` が再 export する（core は app に依存しない）
- `router.replace` は購読者にも知らせる（`route` は URL から作るため）。クエリ画面はフィルタ欄の変更を `replace` で URL に書くので、戻ってきた `q` が自分の状態と同じなら何もしないこと（Task 6）
- `ViewDirectory.save` は `PUT`。`rename` の応答の view は新しい名前のもの（サーバーの `ViewRename`）、`remove` の応答は消す前の view
- `useListScope.schemas` が `schema.load` を始めるのは、呼び出し側（カンバン・テーブル）が Store ごとの読み込みを自分で書かなくてよいように

**確かめたこと（書き手）**: この節のコードは、`web/` の写し（main の 79a5277）に Step の差分を当てて、`pnpm fix`・`pnpm check`（警告 0）・`pnpm exec biome ci --vcs-enabled=false .`・`pnpm vitest run`（918 件）を通した。全件の1回目に、この Task と関係のない `app/tree/TreeView.test.ts` の「lets a late wait for a row give way to a newer one」が `AssertionError: expected null to be 'M199'`（1505ms、94 ファイルの並列実行、WSL2）で落ちた。単独では 53 件が通り、2回目の全件も通った（混んだときの時間切れ）。

- [ ] **Step 1: 依存を入れる**

Run: `cd web && pnpm add -E @tanstack/svelte-table@9.3.1`
`package.json` の `dependencies` に `"@tanstack/svelte-table": "9.3.1"`（exact）が入り、`pnpm-lock.yaml` が更新される。pnpm の「Agent skills awaiting approval」の表示は無視してよい（`pnpm install --frozen-lockfile` は通る）。

- [ ] **Step 2: ビューの型とキャッシュのテストを書く（失敗する）**

`web/core/views.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import type { TaskCache } from "./cache";
import type { StoreDirectory, StoreTone } from "./stores";
import { FakeConn, fakeApi, problem, settle } from "./testing";
import {
  DEFAULT_KANBAN,
  DEFAULT_TABLE,
  draftOf,
  type SavedView,
  sameDraft,
  saveBodyOf,
  ViewDirectory,
  type ViewDraft,
} from "./views";

const view = (name: string, over: Partial<SavedView> = {}): SavedView => ({
  name,
  layout: "table",
  query: "status:todo",
  table: { columns: ["id", "title", "due"], group_by: "parent" },
  kanban: { columns: "category", swimlane: "assignee" },
  ...over,
});

describe("draftOf", () => {
  it("reads every setting of the view", () => {
    expect(draftOf(view("a"))).toEqual({
      layout: "table",
      query: "status:todo",
      kanban: { columns: "category", swimlane: "assignee" },
      table: { columns: ["id", "title", "due"], groupBy: "parent" },
    });
  });

  it("takes the default for a value it does not know, and for columns that are none", () => {
    const d = draftOf(
      view("a", {
        layout: "gantt",
        table: { columns: null, group_by: "sprint" },
        kanban: { columns: "wip", swimlane: "team" },
      }),
    );
    expect(d.layout).toBe("tree");
    expect(d.kanban).toEqual(DEFAULT_KANBAN);
    expect(d.table).toEqual(DEFAULT_TABLE);
    expect(d.table.columns).not.toBe(DEFAULT_TABLE.columns);
  });
});

describe("saveBodyOf", () => {
  it("holds every setting, in the names of the API", () => {
    const d = draftOf(view("a"));
    expect(saveBodyOf(d)).toEqual({
      layout: "table",
      query: "status:todo",
      columns: ["id", "title", "due"],
      group_by: "parent",
      kanban_columns: "category",
      swimlane: "assignee",
    });
    expect(saveBodyOf(d).columns).not.toBe(d.table.columns);
  });
});

describe("sameDraft", () => {
  const base = (): ViewDraft => draftOf(view("a"));

  it("leaves the layout out", () => {
    expect(sameDraft(base(), { ...base(), layout: "kanban" })).toBe(true);
  });

  it("tells a change of the query, the kanban or the table", () => {
    expect(sameDraft(base(), { ...base(), query: "status:todo " })).toBe(true);
    expect(sameDraft(base(), { ...base(), query: "status:done" })).toBe(false);
    expect(
      sameDraft(base(), { ...base(), kanban: { columns: "status", swimlane: "assignee" } }),
    ).toBe(false);
    expect(
      sameDraft(base(), { ...base(), kanban: { columns: "category", swimlane: "none" } }),
    ).toBe(false);
    expect(
      sameDraft(base(), { ...base(), table: { columns: ["id", "title", "due"], groupBy: "none" } }),
    ).toBe(false);
    expect(
      sameDraft(base(), {
        ...base(),
        table: { columns: ["id", "due", "title"], groupBy: "parent" },
      }),
    ).toBe(false);
    expect(
      sameDraft(base(), { ...base(), table: { columns: ["id", "title"], groupBy: "parent" } }),
    ).toBe(false);
  });
});

// A StoreDirectory that the test drives: the stores, their colours and whether they are known.
class FakeStores {
  names: string[];
  known = true;
  tones = new Map<string, StoreTone>();
  private readonly subs = new Set<() => void>();
  constructor(names: string[]) {
    this.names = names;
  }
  stores() {
    return this.names.map((name) => ({ name }));
  }
  tone(s: string): StoreTone {
    return this.tones.get(s) ?? "teal";
  }
  loaded() {
    return this.known;
  }
  subscribe(fn: () => void) {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }
  change() {
    for (const fn of [...this.subs]) fn();
  }
}

function setup(world: Record<string, SavedView[] | "fail"> = {}) {
  const w = { ...world };
  const { api, calls } = fakeApi((r) => {
    const store = r.query.get("store") ?? "";
    if (r.path === "/api/views" && r.method === "GET") {
      const vs = w[store];
      if (vs === "fail") return problem(500, "internal");
      return vs ?? [];
    }
    const name = decodeURIComponent(r.path.slice("/api/views/".length));
    const body = r.body as Record<string, unknown>;
    if (r.method === "PUT")
      return { view: view(name, { ...body } as Partial<SavedView>), commit: "c1" };
    if (r.method === "PATCH") return { view: view(body.name as string), commit: "c2" };
    return { view: view(name), commit: "c3" };
  });
  const conn = new FakeConn();
  const stores = new FakeStores(Object.keys(w));
  const noteOwnCommit = vi.fn();
  const dir = new ViewDirectory({
    api,
    conn: conn.asConnection(),
    stores: stores as unknown as StoreDirectory,
    cache: { noteOwnCommit } as unknown as TaskCache,
  });
  const views = (store: string) =>
    calls.filter((c) => c.path === "/api/views" && c.query.get("store") === store);
  return { dir, conn, stores, w, calls, noteOwnCommit, views };
}

const names = (dir: ViewDirectory) => dir.list().map((e) => `${e.store}/${e.view.name}`);

describe("ViewDirectory", () => {
  it("lists the views of every store: in the order of the stores, then by name", async () => {
    const { dir } = setup({
      shared: [view("weekly"), view("board")],
      personal: [view("mine")],
    });
    expect(dir.loaded()).toBe(false);
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    expect(names(dir)).toEqual(["shared/board", "shared/weekly", "personal/mine"]);
    expect(dir.get("shared", "weekly")?.view.query).toBe("status:todo");
    expect(dir.get("personal", "weekly")).toBeUndefined();
  });

  it("is not loaded until the stores are known, and then until every store has answered", async () => {
    const stores = new FakeStores([]);
    stores.known = false;
    const { api } = fakeApi(() => []);
    const dir = new ViewDirectory({
      api,
      conn: new FakeConn().asConnection(),
      stores: stores as unknown as StoreDirectory,
      cache: { noteOwnCommit() {} } as unknown as TaskCache,
    });
    await settle();
    expect(dir.loaded()).toBe(false);
    const fn = vi.fn();
    dir.subscribe(fn);
    stores.known = true;
    stores.names = ["shared"];
    stores.change();
    expect(dir.loaded()).toBe(false);
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    expect(fn).toHaveBeenCalled();
  });

  it("counts a store that cannot be read as done, with no views, and keeps what it had", async () => {
    const { dir, w, conn } = setup({ shared: [view("weekly")], broken: "fail" });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    expect(names(dir)).toEqual(["shared/weekly"]);
    w.shared = "fail";
    conn.emit("views.changed", { store: "shared" });
    await settle();
    expect(names(dir)).toEqual(["shared/weekly"]);
  });

  it("fetches again only the store of views.changed", async () => {
    const { dir, conn, w, views } = setup({ shared: [view("weekly")], personal: [] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    const fn = vi.fn();
    dir.subscribe(fn);
    w.shared = [view("weekly"), view("monthly")];
    conn.emit("views.changed", { store: "shared" });
    await vi.waitFor(() => expect(names(dir)).toEqual(["shared/monthly", "shared/weekly"]));
    expect(views("shared")).toHaveLength(2);
    expect(views("personal")).toHaveLength(1);
    expect(fn).toHaveBeenCalledTimes(1);
    conn.emit("views.changed", { store: "nobody" });
    await settle();
    expect(views("nobody")).toHaveLength(0);
  });

  it("fetches every store again at resync, and keeps the entries that did not change", async () => {
    const { dir, conn, views } = setup({ shared: [view("weekly")], personal: [view("mine")] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    const before = dir.list();
    const fn = vi.fn();
    dir.subscribe(fn);
    conn.emit("resync");
    await vi.waitFor(() => expect(views("personal")).toHaveLength(2));
    await settle();
    expect(views("shared")).toHaveLength(2);
    expect(dir.list()).toEqual(before);
    expect(dir.list()[0]).toBe(before[0]);
    expect(fn).not.toHaveBeenCalled();
  });

  it("follows the stores: fetches one that came, drops one that went, takes a new colour", async () => {
    const { dir, stores, w } = setup({ shared: [view("weekly")] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    w.personal = [view("mine")];
    stores.names = ["shared", "personal"];
    stores.change();
    await vi.waitFor(() => expect(names(dir)).toEqual(["shared/weekly", "personal/mine"]));
    expect(dir.loaded()).toBe(true);
    stores.tones.set("personal", "readonly");
    stores.change();
    expect(dir.get("personal", "mine")?.tone).toBe("readonly");
    expect(dir.get("shared", "weekly")?.tone).toBe("teal");
    stores.names = ["personal"];
    stores.change();
    expect(names(dir)).toEqual(["personal/mine"]);
  });

  it("save puts the settings, notes the commit and shows the view at once", async () => {
    const { dir, calls, noteOwnCommit } = setup({ shared: [view("weekly")] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    const saved = await dir.save("shared", "board", { layout: "kanban", query: "kind:task" });
    expect(saved.name).toBe("board");
    const put = calls.find((c) => c.method === "PUT");
    expect(put?.path).toBe("/api/views/board");
    expect(put?.query.get("store")).toBe("shared");
    expect(put?.body).toEqual({ layout: "kanban", query: "kind:task" });
    expect(noteOwnCommit).toHaveBeenCalledWith("c1");
    expect(names(dir)).toEqual(["shared/board", "shared/weekly"]);
    expect(dir.get("shared", "board")?.view.query).toBe("kind:task");
    await dir.save("shared", "weekly", { query: "status:done" });
    expect(names(dir)).toEqual(["shared/board", "shared/weekly"]);
    expect(dir.get("shared", "weekly")?.view.query).toBe("status:done");
  });

  it("encodes the name in the path, and rename and remove change the list", async () => {
    const { dir, calls, noteOwnCommit } = setup({ shared: [view("今週 の作業"), view("b")] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    const renamed = await dir.rename("shared", "今週 の作業", "来週");
    expect(renamed.name).toBe("来週");
    const patch = calls.find((c) => c.method === "PATCH");
    expect(patch?.path).toBe(`/api/views/${encodeURIComponent("今週 の作業")}`);
    expect(patch?.body).toEqual({ name: "来週" });
    expect(patch?.query.get("store")).toBe("shared");
    expect(noteOwnCommit).toHaveBeenCalledWith("c2");
    expect(names(dir)).toEqual(["shared/b", "shared/来週"]);
    await dir.remove("shared", "b");
    const del = calls.find((c) => c.method === "DELETE");
    expect(del?.path).toBe("/api/views/b");
    expect(del?.query.get("store")).toBe("shared");
    expect(noteOwnCommit).toHaveBeenCalledWith("c3");
    expect(names(dir)).toEqual(["shared/来週"]);
  });

  it("throws the error of a write and leaves the list as it was", async () => {
    const { api } = fakeApi((r) =>
      r.method === "GET" ? [view("weekly")] : problem(409, "view.name_taken", { name: "weekly" }),
    );
    const stores = new FakeStores(["shared"]);
    const noteOwnCommit = vi.fn();
    const dir = new ViewDirectory({
      api,
      conn: new FakeConn().asConnection(),
      stores: stores as unknown as StoreDirectory,
      cache: { noteOwnCommit } as unknown as TaskCache,
    });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    await expect(dir.save("shared", "Weekly", {})).rejects.toMatchObject({
      type: "view.name_taken",
      status: 409,
    });
    expect(names(dir)).toEqual(["shared/weekly"]);
    expect(noteOwnCommit).not.toHaveBeenCalled();
  });

  it("drops a fetch that a write overtook", async () => {
    let release: (vs: SavedView[]) => void = () => undefined;
    let n = 0;
    const { api } = fakeApi((r) => {
      if (r.method !== "GET") return { view: view("new"), commit: "c1" };
      n++;
      return n === 1 ? Promise.resolve([view("old")]) : new Promise((res) => (release = res));
    });
    const conn = new FakeConn();
    const dir = new ViewDirectory({
      api,
      conn: conn.asConnection(),
      stores: new FakeStores(["shared"]) as unknown as StoreDirectory,
      cache: { noteOwnCommit() {} } as unknown as TaskCache,
    });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    conn.emit("views.changed", { store: "shared" }); // held
    await settle();
    await dir.save("shared", "new", {});
    expect(names(dir)).toEqual(["shared/new", "shared/old"]);
    release([view("old")]); // read before the write
    await settle();
    expect(names(dir)).toEqual(["shared/new", "shared/old"]);
  });
});
```


- [ ] **Step 3: `core/views.ts` を書く**

`web/core/views.ts`:

```ts
import { type Api, type Schemas, unwrap } from "./api/client";
import type { TaskCache } from "./cache";
import type { Connection } from "./connection";
import { callAll } from "./listeners";
import type { StoreDirectory, StoreTone } from "./stores";

export type Layout = "tree" | "kanban" | "table";
export type SavedView = Schemas["SavedView"];
export type ViewSaveBody = Schemas["ViewSaveInputBody"];

export type KanbanSettings = {
  columns: "status" | "category";
  swimlane: "none" | "assignee" | "project";
};
export type TableSettings = {
  columns: string[];
  groupBy: "none" | "parent" | "status" | "assignee" | "project";
};
// What a screen of a view holds while it is open: the saved settings, and what was changed since.
export type ViewDraft = {
  layout: Layout;
  query: string;
  kanban: KanbanSettings;
  table: TableSettings;
};

// The defaults of internal/view: what a view that leaves a setting out has.
export const DEFAULT_KANBAN: KanbanSettings = { columns: "status", swimlane: "none" };
export const DEFAULT_TABLE: TableSettings = {
  columns: ["id", "title", "status", "assignee", "due"],
  groupBy: "none",
};

const LAYOUTS: readonly string[] = ["tree", "kanban", "table"];
const KANBAN_COLUMNS: readonly string[] = ["status", "category"];
const SWIMLANES: readonly string[] = ["none", "assignee", "project"];
const GROUP_BYS: readonly string[] = ["none", "parent", "status", "assignee", "project"];

// draftOf reads a saved view; a value this client does not know (a newer server) is the default.
export function draftOf(v: SavedView): ViewDraft {
  const columns = v.table.columns ?? [];
  return {
    layout: LAYOUTS.includes(v.layout) ? (v.layout as Layout) : "tree",
    query: v.query,
    kanban: {
      columns: KANBAN_COLUMNS.includes(v.kanban.columns)
        ? (v.kanban.columns as KanbanSettings["columns"])
        : DEFAULT_KANBAN.columns,
      swimlane: SWIMLANES.includes(v.kanban.swimlane)
        ? (v.kanban.swimlane as KanbanSettings["swimlane"])
        : DEFAULT_KANBAN.swimlane,
    },
    table: {
      columns: columns.length > 0 ? [...columns] : [...DEFAULT_TABLE.columns],
      groupBy: GROUP_BYS.includes(v.table.group_by)
        ? (v.table.group_by as TableSettings["groupBy"])
        : DEFAULT_TABLE.groupBy,
    },
  };
}

// saveBodyOf is the body of a PUT that saves every setting of d: hh view save with all its flags.
export function saveBodyOf(d: ViewDraft): ViewSaveBody {
  return {
    layout: d.layout,
    query: d.query,
    columns: [...d.table.columns],
    group_by: d.table.groupBy,
    kanban_columns: d.kanban.columns,
    swimlane: d.kanban.swimlane,
  };
}

// sameDraft says whether a and b hold the same settings. The layout is left out: a screen changes it
// at once (PUT {layout}) and does not count it as an unsaved change.
export function sameDraft(a: ViewDraft, b: ViewDraft): boolean {
  return (
    a.query.trim() === b.query.trim() &&
    a.kanban.columns === b.kanban.columns &&
    a.kanban.swimlane === b.kanban.swimlane &&
    a.table.groupBy === b.table.groupBy &&
    a.table.columns.length === b.table.columns.length &&
    a.table.columns.every((c, i) => c === b.table.columns[i])
  );
}

export type ViewEntry = { store: string; tone: StoreTone; view: SavedView };

const byName = (a: SavedView, b: SavedView): number => a.name.localeCompare(b.name, "ja");

function sameEntry(a: ViewEntry, b: ViewEntry): boolean {
  return (
    a.store === b.store && a.tone === b.tone && JSON.stringify(a.view) === JSON.stringify(b.view)
  );
}

// ViewDirectory is the saved views of every store, for the sidebar, the palette and the screens of a
// view. The API lists them store by store (GET /api/views?store=), so each store is fetched on its
// own; views.changed fetches that store again, resync and a change of the stores fetch them all.
export class ViewDirectory {
  private readonly api: Api;
  private readonly cache: TaskCache;
  private readonly dir: StoreDirectory;
  private names: string[] = [];
  private readonly byStore = new Map<string, SavedView[]>();
  private readonly fetched = new Set<string>();
  private readonly seqs = new Map<string, number>();
  private result: ViewEntry[] = [];
  private isLoaded = false;
  private readonly subs = new Set<() => void>();

  constructor(opts: { api: Api; conn: Connection; stores: StoreDirectory; cache: TaskCache }) {
    this.api = opts.api;
    this.cache = opts.cache;
    this.dir = opts.stores;
    opts.stores.subscribe(() => this.onStores());
    opts.conn.on("views.changed", (e) => {
      if (this.names.includes(e.store)) void this.fetch(e.store);
    });
    opts.conn.on("resync", () => {
      for (const n of this.names) void this.fetch(n);
    });
    this.onStores();
  }

  // list is every view: in the order of the stores, then by name.
  list(): ViewEntry[] {
    return this.result;
  }

  get(store: string, name: string): ViewEntry | undefined {
    return this.result.find((e) => e.store === store && e.view.name === name);
  }

  // loaded is true once the stores are known and each has been asked for its views (a store that
  // could not be read counts: its views are none).
  loaded(): boolean {
    return this.isLoaded;
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  // save creates the view, or changes the settings body holds (PUT: what is not sent stays).
  async save(store: string, name: string, body: ViewSaveBody): Promise<SavedView> {
    const r = await unwrap(
      this.api.PUT("/api/views/{name}", { params: { path: { name }, query: { store } }, body }),
    );
    this.cache.noteOwnCommit(r.commit);
    this.apply(store, (vs) => [...vs.filter((v) => v.name !== name), r.view]);
    return r.view;
  }

  async rename(store: string, name: string, to: string): Promise<SavedView> {
    const r = await unwrap(
      this.api.PATCH("/api/views/{name}", {
        params: { path: { name }, query: { store } },
        body: { name: to },
      }),
    );
    this.cache.noteOwnCommit(r.commit);
    this.apply(store, (vs) => [...vs.filter((v) => v.name !== name && v.name !== to), r.view]);
    return r.view;
  }

  async remove(store: string, name: string): Promise<void> {
    const r = await unwrap(
      this.api.DELETE("/api/views/{name}", { params: { path: { name }, query: { store } } }),
    );
    this.cache.noteOwnCommit(r.commit);
    this.apply(store, (vs) => vs.filter((v) => v.name !== name));
  }

  // onStores follows the stores: it fetches the views of one that came, drops those of one that went,
  // and builds the list again (the colour of a store may have changed).
  private onStores(): void {
    const names = this.dir.stores().map((s) => s.name);
    if (names.join("\0") !== this.names.join("\0")) {
      const old = this.names;
      this.names = names;
      for (const n of old.filter((o) => !names.includes(o))) {
        this.byStore.delete(n);
        this.fetched.delete(n);
        this.seqs.delete(n);
      }
      for (const n of names.filter((x) => !old.includes(x))) void this.fetch(n);
    }
    this.rebuild();
  }

  // fetch reads the views of a store. A fetch that a newer one or a write has overtaken is dropped; one
  // that fails leaves the views the store had.
  private async fetch(store: string): Promise<void> {
    const seq = this.bump(store);
    let views: SavedView[] | null = null;
    try {
      views = (await unwrap(this.api.GET("/api/views", { params: { query: { store } } }))) ?? [];
    } catch {
      views = null;
    }
    if (this.seqs.get(store) !== seq || !this.names.includes(store)) return;
    if (views) this.byStore.set(store, [...views].sort(byName));
    this.fetched.add(store);
    this.rebuild();
  }

  private bump(store: string): number {
    const seq = (this.seqs.get(store) ?? 0) + 1;
    this.seqs.set(store, seq);
    return seq;
  }

  // apply takes in the answer of a write, and drops a fetch that is on its way: it may have read the
  // store before the write.
  private apply(store: string, change: (vs: SavedView[]) => SavedView[]): void {
    this.bump(store);
    this.byStore.set(store, change(this.byStore.get(store) ?? []).sort(byName));
    this.fetched.add(store);
    this.rebuild();
  }

  // rebuild makes the list again, keeping the entries that did not change as they were, and tells the
  // subscribers if the list or loaded() is not the same.
  private rebuild(): void {
    const next: ViewEntry[] = [];
    for (const store of this.names) {
      const tone = this.dir.tone(store);
      for (const view of this.byStore.get(store) ?? []) {
        const entry: ViewEntry = { store, tone, view };
        const prev = this.result.find((e) => e.store === store && e.view.name === view.name);
        next.push(prev && sameEntry(prev, entry) ? prev : entry);
      }
    }
    const loaded = this.dir.loaded() && this.names.every((n) => this.fetched.has(n));
    const same = next.length === this.result.length && next.every((e, i) => e === this.result[i]);
    if (same && loaded === this.isLoaded) return;
    if (!same) this.result = next;
    this.isLoaded = loaded;
    callAll(this.subs);
  }
}
```


Run: `cd web && pnpm vitest run core/views.test.ts`
Expected: PASS

- [ ] **Step 4: CLI の文字列（テストが先）**

`web/core/cli.test.ts` の差分（`@@` の行は目安。該当する箇所に当てる）:

```diff
@@ -10,6 +10,10 @@
   cliSet,
   cliStoreInit,
   cliSync,
+  cliViewRename,
+  cliViewRm,
+  cliViewSave,
+  cliViewShow,
   cliWatch,
   cliWorkspaceAdd,
   cliWorkspaceRename,
@@ -128,6 +132,55 @@
     );
   });
 
+  it("writes hh view save with a flag for each setting the body holds", () => {
+    expect(
+      cliViewSave({
+        name: "weekly",
+        store: "shared",
+        body: {
+          layout: "table",
+          query: "status:todo assignee:@me",
+          columns: ["id", "title", "due"],
+          group_by: "parent",
+        },
+      }),
+    ).toBe(
+      'hh view save weekly --layout table --query "status:todo assignee:@me" --columns id,title,due --group-by parent --store shared',
+    );
+    expect(
+      cliViewSave({
+        name: "board",
+        body: { layout: "kanban", kanban_columns: "category", swimlane: "assignee" },
+      }),
+    ).toBe("hh view save board --layout kanban --kanban-columns category --swimlane assignee");
+    expect(cliViewSave({ name: "board", body: {} })).toBe("hh view save board");
+    expect(cliViewSave({ name: "board", body: { query: "" } })).toBe(
+      'hh view save board --query ""',
+    );
+    expect(cliViewSave({ name: "w", body: { query: "-status:canceled" } })).toBe(
+      "hh view save w --query -status:canceled",
+    );
+    expect(
+      cliViewSave({ name: new Placeholder("名前"), store: "a b", body: { layout: "tree" } }),
+    ).toBe('hh view save <名前> --layout tree --store "a b"');
+    expect(cliViewSave({ name: "-x", body: { layout: "tree" } })).toBe(
+      "hh view save --layout tree -- -x",
+    );
+  });
+
+  it("writes hh view show, rm and rename; --store only when it is given", () => {
+    expect(cliViewShow("weekly")).toBe("hh view show weekly");
+    expect(cliViewShow("今週の作業", "shared")).toBe("hh view show 今週の作業 --store shared");
+    expect(cliViewRm("weekly", "shared")).toBe("hh view rm weekly --store shared");
+    expect(cliViewRm("weekly")).toBe("hh view rm weekly");
+    expect(cliViewRename("weekly", "monthly", "shared")).toBe(
+      "hh view rename weekly monthly --store shared",
+    );
+    expect(cliViewRename("weekly", new Placeholder("新しい名前"))).toBe(
+      "hh view rename weekly <新しい名前>",
+    );
+  });
+
   it("writes the others", () => {
     expect(cliRm("REL-8")).toBe("hh rm REL-8");
     expect(cliRestore("REL-8")).toBe("hh restore REL-8");
```


`web/core/cli.ts` の差分（`@@` の行は目安。該当する箇所に当てる）:

```diff
@@ -1,5 +1,6 @@
 // The hh command that does what a dialog or a filter does, shown under it
 // for the user to copy (Foundations: the same operation as a command).
+import type { ViewSaveBody } from "./views";
 
 const PLAIN = /^[\p{L}\p{N}_\-.,:/@%+=]+$/u;
 
@@ -152,6 +153,33 @@
 export const cliWorkspaceRm = (i: { workspace: string; store: string }): string =>
   cliLine(["workspace", "rm", i.workspace, "--store", i.store]);
 
+// cliViewSave is hh view save <name>; a flag is written for each setting body holds (the others stay
+// as saved, as a PUT leaves them). --store is the global flag that picks the store.
+export function cliViewSave(i: { name: Arg; store?: string; body: ViewSaveBody }): string {
+  const b = i.body;
+  const a: Arg[] = [];
+  if (b.layout !== undefined) a.push("--layout", b.layout);
+  if (b.query !== undefined) a.push("--query", b.query);
+  if (b.columns !== undefined) a.push("--columns", b.columns.join(","));
+  if (b.group_by !== undefined) a.push("--group-by", b.group_by);
+  if (b.kanban_columns !== undefined) a.push("--kanban-columns", b.kanban_columns);
+  if (b.swimlane !== undefined) a.push("--swimlane", b.swimlane);
+  if (i.store) a.push("--store", i.store);
+  return withPositionals(["view", "save"], [i.name], a, true);
+}
+
+export function cliViewShow(name: string, store?: string): string {
+  return withPositionals(["view", "show"], [name], store ? ["--store", store] : [], true);
+}
+
+export function cliViewRm(name: string, store?: string): string {
+  return withPositionals(["view", "rm"], [name], store ? ["--store", store] : [], true);
+}
+
+export function cliViewRename(name: string, to: Arg, store?: string): string {
+  return withPositionals(["view", "rename"], [name, to], store ? ["--store", store] : [], true);
+}
+
 export function cliStoreInit(i: {
   name: Arg;
   prefix: Arg;
```


Run: `cd web && pnpm vitest run core/cli.test.ts`
Expected: PASS

- [ ] **Step 5: `AppCore` に `views` を足す**

`web/app/context.ts` の差分（`@@` の行は目安。該当する箇所に当てる）:

```diff
@@ -5,6 +5,7 @@
 import type { SaveController } from "../core/save";
 import type { SchemaCache } from "../core/schema";
 import type { StoreDirectory } from "../core/stores";
+import type { ViewDirectory } from "../core/views";
 
 export type AppCore = {
   api: Api;
@@ -13,6 +14,7 @@
   save: SaveController;
   stores: StoreDirectory;
   schema: SchemaCache;
+  views: ViewDirectory;
 };
 
 /** The key of the context; tests pass it to render to replace the core. */
```


`web/app/core-factory.ts` の差分（`@@` の行は目安。該当する箇所に当てる）:

```diff
@@ -4,6 +4,7 @@
 import { SaveController } from "../core/save";
 import { SchemaCache } from "../core/schema";
 import { StoreDirectory } from "../core/stores";
+import { ViewDirectory } from "../core/views";
 import type { AppCore } from "./context";
 
 /** Builds the one set of core objects the whole app shares. */
@@ -23,5 +24,6 @@
   const save = new SaveController({ api, cache, conn });
   const stores = new StoreDirectory({ api, conn, cache });
   const schema = new SchemaCache({ api, conn });
-  return { api, conn, cache, save, stores, schema };
+  const views = new ViewDirectory({ api, conn, stores, cache });
+  return { api, conn, cache, save, stores, schema, views };
 }
```


`web/app/core-factory.test.ts` の差分（`@@` の行は目安。該当する箇所に当てる）:

```diff
@@ -2,12 +2,20 @@
 import { createCore } from "./core-factory";
 
 describe("createCore", () => {
-  it("6つの core を作る", () => {
+  it("7つの core を作る", () => {
     const c = createCore({
       token: () => "t",
       fetch: (() => new Promise(() => {})) as typeof fetch,
     });
-    expect(Object.keys(c).sort()).toEqual(["api", "cache", "conn", "save", "schema", "stores"]);
+    expect(Object.keys(c).sort()).toEqual([
+      "api",
+      "cache",
+      "conn",
+      "save",
+      "schema",
+      "stores",
+      "views",
+    ]);
     expect(typeof c.conn.start).toBe("function");
   });
 });
```


`web/app/test-utils.ts` の差分（`@@` の行は目安。該当する箇所に当てる）:

```diff
@@ -21,7 +21,7 @@
   };
 }
 
-/** A core made of fakes; `over` replaces any of the six objects. */
+/** A core made of fakes; `over` replaces any of the seven objects. */
 export function fakeCore(over: Partial<Record<keyof AppCore, unknown>> = {}): AppCore {
   const e = emitter();
   const base = {
@@ -48,6 +48,12 @@
       subscribe: e.subscribe,
     },
     schema: { subscribe: e.subscribe },
+    views: {
+      list: () => [],
+      get: () => undefined,
+      loaded: () => true,
+      subscribe: e.subscribe,
+    },
   };
   return { ...base, ...over } as unknown as AppCore;
 }
```


- [ ] **Step 6: ルート（テストが先）**

`web/app/router.test.ts` の差分（`@@` の行は目安。該当する箇所に当てる）:

```diff
@@ -28,12 +28,31 @@
       "/workspaces/org-b%2Fops/01HWS/tree",
       { name: "workspace", store: "org-b/ops", ulid: "01HWS", tab: "tree" },
     ],
-    ["/workspaces/shared/01HWS/kanban", { name: "home" }],
+    [
+      "/workspaces/shared/01HWS/kanban",
+      { name: "workspace", store: "shared", ulid: "01HWS", tab: "kanban" },
+    ],
+    [
+      "/workspaces/shared/01HWS/table",
+      { name: "workspace", store: "shared", ulid: "01HWS", tab: "table" },
+    ],
+    ["/workspaces/shared/01HWS/gantt", { name: "home" }],
     ["/workspaces/shared/01HWS/tree/extra", { name: "home" }],
     ["/projects/01HPR", { name: "project", ulid: "01HPR", tab: "tree" }],
     ["/projects/01HPR/tree", { name: "project", ulid: "01HPR", tab: "tree" }],
-    ["/projects/01HPR/kanban", { name: "home" }],
+    ["/projects/01HPR/kanban", { name: "project", ulid: "01HPR", tab: "kanban" }],
+    ["/projects/01HPR/table", { name: "project", ulid: "01HPR", tab: "table" }],
     ["/projects/01HPR/settings", { name: "home" }],
+    ["/views/shared/%E4%BB%8A%E9%80%B1", { name: "view", store: "shared", view: "今週" }],
+    ["/views/org-b%2Fops/weekly", { name: "view", store: "org-b/ops", view: "weekly" }],
+    ["/views/shared", { name: "home" }],
+    ["/views/shared/weekly/extra", { name: "home" }],
+    ["/query/table", { name: "query", layout: "table", q: "" }],
+    ["/query/kanban/", { name: "query", layout: "kanban", q: "" }],
+    ["/query/gantt", { name: "home" }],
+    ["/query", { name: "home" }],
+    ["/my", { name: "my" }],
+    ["/my/extra", { name: "home" }],
     ["/tasks/01HTK/", { name: "task", ulid: "01HTK" }],
     ["/settings", { name: "settings", page: "workspaces", store: "" }],
     ["/settings/workspaces/shared", { name: "settings", page: "workspaces", store: "shared" }],
@@ -47,6 +66,21 @@
   ])("%s", (path, route) => {
     expect(parseRoute(path)).toEqual(route);
   });
+
+  it("/query/<layout> は search の q を読む。ほかのパスは search を捨てる", () => {
+    expect(parseRoute("/query/table", "?q=status%3Atodo%20assignee%3A%40me")).toEqual({
+      name: "query",
+      layout: "table",
+      q: "status:todo assignee:@me",
+    });
+    expect(parseRoute("/query/tree", "?x=1")).toEqual({ name: "query", layout: "tree", q: "" });
+    expect(parseRoute("/my", "?q=a")).toEqual({ name: "my" });
+    expect(parseRoute("/projects/01HPR/table", "?q=a")).toEqual({
+      name: "project",
+      ulid: "01HPR",
+      tab: "table",
+    });
+  });
 });
 
 describe("hrefOf", () => {
@@ -74,6 +108,42 @@
     }
   });
 
+  it("プロジェクトとワークスペースのカンバン・テーブルのパス。ツリーだけ末尾を付けない", () => {
+    expect(hrefOf({ name: "project", ulid: "01HPR", tab: "kanban" })).toBe(
+      "/projects/01HPR/kanban",
+    );
+    expect(hrefOf({ name: "project", ulid: "01HPR", tab: "table" })).toBe("/projects/01HPR/table");
+    const ws = { name: "workspace", store: "shared", ulid: "01HWS" } as const;
+    expect(hrefOf({ ...ws, tab: "kanban" })).toBe("/workspaces/shared/01HWS/kanban");
+    expect(hrefOf({ ...ws, tab: "table" })).toBe("/workspaces/shared/01HWS/table");
+  });
+
+  it("ビュー・クエリ・マイタスクのパス。名前と q は符号化し、q が空なら ?q= を付けない", () => {
+    expect(hrefOf({ name: "view", store: "org-b/ops", view: "今週 の/作業" })).toBe(
+      "/views/org-b%2Fops/%E4%BB%8A%E9%80%B1%20%E3%81%AE%2F%E4%BD%9C%E6%A5%AD",
+    );
+    expect(hrefOf({ name: "query", layout: "table", q: "" })).toBe("/query/table");
+    expect(hrefOf({ name: "query", layout: "kanban", q: "assignee:@me -status:done" })).toBe(
+      "/query/kanban?q=assignee%3A%40me%20-status%3Adone",
+    );
+    expect(hrefOf({ name: "my" })).toBe("/my");
+  });
+
+  it("ビュー・クエリ・マイタスクは parseRoute と往復できる", () => {
+    for (const r of [
+      { name: "view", store: "org-b/ops", view: "今週 の/作業" },
+      { name: "query", layout: "table", q: "" },
+      { name: "query", layout: "tree", q: 'title:"a b" +x -y&z=1' },
+      { name: "my" },
+      { name: "project", ulid: "01HPR", tab: "kanban" },
+      { name: "workspace", store: "shared", ulid: "01HWS", tab: "table" },
+    ] as const) {
+      const href = hrefOf(r);
+      const url = new URL(href, "http://x");
+      expect(parseRoute(url.pathname, url.search)).toEqual(r);
+    }
+  });
+
   it("設定のパスは Store 名を符号化し、Store がなければ /settings", () => {
     expect(hrefOf({ name: "settings", page: "workspaces", store: "org-b/ops" })).toBe(
       "/settings/workspaces/org-b%2Fops",
@@ -108,10 +178,37 @@
     expect(history.length).toBe(before);
   });
 
-  it("クエリとハッシュは捨てる", () => {
+  it("クエリとハッシュは捨てる。ただし /query/<layout> の ?q= は残す", () => {
     router.go("/projects/01HPR?x=1#y");
     expect(location.pathname).toBe("/projects/01HPR");
     expect(location.search).toBe("");
+    router.go("/query/table?q=status%3Atodo&x=1#y");
+    expect(location.pathname).toBe("/query/table");
+    expect(location.search).toBe("?q=status%3Atodo");
+    expect(location.hash).toBe("");
+    expect(router.route).toEqual({ name: "query", layout: "table", q: "status:todo" });
+    router.go("/my?q=status%3Atodo");
+    expect(location.pathname).toBe("/my");
+    expect(location.search).toBe("");
+  });
+
+  it("q だけ替えた /query への go は route を替える", () => {
+    router.go("/query/tree?q=a");
+    expect(router.route).toEqual({ name: "query", layout: "tree", q: "a" });
+    router.go("/query/tree?q=b");
+    expect(router.route).toEqual({ name: "query", layout: "tree", q: "b" });
+  });
+
+  it("replace は履歴を積まずに URL を替える", () => {
+    router.go("/query/kanban?q=a");
+    const before = history.length;
+    router.replace("/query/kanban?q=a%20b");
+    expect(history.length).toBe(before);
+    expect(location.search).toBe("?q=a%20b");
+    expect(router.route).toEqual({ name: "query", layout: "kanban", q: "a b" });
+    router.replace("/unknown");
+    expect(location.pathname).toBe("/");
+    expect(history.length).toBe(before);
   });
 });
 
```


`web/app/router.ts`:

```ts
import { createSubscriber } from "svelte/reactivity";
import type { Layout } from "../core/views";

export type { Layout };
// The tabs the screens have so far; 4e-4 adds the settings of a project.
export type WorkspaceTab = "overview" | "tree" | "kanban" | "table" | "settings";
export type ProjectTab = "tree" | "kanban" | "table";
// store is "" for /settings, which names no Store: the screen goes to the first one.
export type SettingsPage = { page: "workspaces"; store: string };

export type Route =
  | { name: "home" }
  | { name: "workspace"; store: string; ulid: string; tab: WorkspaceTab }
  | { name: "project"; ulid: string; tab: ProjectTab }
  | { name: "task"; ulid: string }
  | { name: "view"; store: string; view: string }
  | { name: "query"; layout: Layout; q: string }
  | { name: "my" }
  | ({ name: "settings" } & SettingsPage);

const WORKSPACE_TABS: readonly string[] = ["tree", "kanban", "table", "settings"];
const PROJECT_TABS: readonly string[] = ["tree", "kanban", "table"];
const LAYOUTS: readonly string[] = ["tree", "kanban", "table"];

function decode(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}

/** The route of a path, or null when the path is none of ours. Only /query/<layout> reads the search (?q=). */
function matchRoute(pathname: string, search: string): Route | null {
  const seg = pathname.split("/").filter(Boolean).map(decode);
  const [head, a, b, c] = seg;
  if (head === undefined) return { name: "home" };
  if (head === "workspaces" && a && b) {
    if (seg.length === 3) return { name: "workspace", store: a, ulid: b, tab: "overview" };
    if (seg.length === 4 && c && WORKSPACE_TABS.includes(c)) {
      return { name: "workspace", store: a, ulid: b, tab: c as WorkspaceTab };
    }
    return null;
  }
  if (head === "projects" && a) {
    if (seg.length === 2) return { name: "project", ulid: a, tab: "tree" };
    if (seg.length === 3 && b && PROJECT_TABS.includes(b)) {
      return { name: "project", ulid: a, tab: b as ProjectTab };
    }
    return null;
  }
  if (head === "tasks" && seg.length === 2 && a) return { name: "task", ulid: a };
  if (head === "views" && seg.length === 3 && a && b) return { name: "view", store: a, view: b };
  if (head === "query" && seg.length === 2 && a && LAYOUTS.includes(a)) {
    return { name: "query", layout: a as Layout, q: new URLSearchParams(search).get("q") ?? "" };
  }
  if (head === "my" && seg.length === 1) return { name: "my" };
  if (head === "settings" && seg.length === 1) {
    return { name: "settings", page: "workspaces", store: "" };
  }
  if (head === "settings" && seg.length === 3 && a === "workspaces" && b) {
    return { name: "settings", page: "workspaces", store: b };
  }
  return null;
}

/** Unknown paths are the home route. search is read for /query/<layout> only. */
export function parseRoute(pathname: string, search = ""): Route {
  return matchRoute(pathname, search) ?? { name: "home" };
}

/** The path of a route; the query screen's also holds ?q= (left out when q is empty). */
export function hrefOf(r: Route): string {
  switch (r.name) {
    case "home":
      return "/";
    case "workspace": {
      const base = `/workspaces/${encodeURIComponent(r.store)}/${encodeURIComponent(r.ulid)}`;
      return r.tab === "overview" ? base : `${base}/${r.tab}`;
    }
    case "project": {
      const base = `/projects/${encodeURIComponent(r.ulid)}`;
      return r.tab === "tree" ? base : `${base}/${r.tab}`;
    }
    case "task":
      return `/tasks/${encodeURIComponent(r.ulid)}`;
    case "view":
      return `/views/${encodeURIComponent(r.store)}/${encodeURIComponent(r.view)}`;
    case "query":
      return r.q === "" ? `/query/${r.layout}` : `/query/${r.layout}?q=${encodeURIComponent(r.q)}`;
    case "my":
      return "/my";
    case "settings":
      return r.store === "" ? "/settings" : `/settings/workspaces/${encodeURIComponent(r.store)}`;
  }
}

const listeners = new Set<() => void>();
const subscribe = createSubscriber((update) => {
  listeners.add(update);
  window.addEventListener("popstate", update);
  return () => {
    listeners.delete(update);
    window.removeEventListener("popstate", update);
  };
});

let last: { key: string; route: Route } | null = null;

export const router = {
  get route(): Route {
    subscribe();
    const key = location.pathname + location.search;
    if (last?.key !== key) last = { key, route: parseRoute(location.pathname, location.search) };
    return last.route;
  },

  /**
   * Moves to a path of ours; anything else goes to "/". The query and the hash are dropped,
   * except for the ?q= of /query/<layout>.
   */
  go(path: string, opts: { replace?: boolean } = {}): void {
    const url = new URL(path, location.origin);
    const route = matchRoute(url.pathname, url.search);
    const target = route ? hrefOf(route) : "/";
    if (target !== location.pathname + location.search || location.hash !== "") {
      if (opts.replace) history.replaceState(null, "", target);
      else history.pushState(null, "", target);
    }
    for (const fn of listeners) fn();
  },

  /** go without adding to the history: the address changes and the back button skips it. */
  replace(path: string): void {
    router.go(path, { replace: true });
  },

  href(r: Route): string {
    return hrefOf(r);
  },
};

/** A link click that stays in the app; with ⌘/Ctrl/⇧ the page opens in a new tab that keeps the token. */
export function onLinkClick(e: MouseEvent, path: string): void {
  if (e.defaultPrevented || e.button !== 0 || e.altKey) return;
  e.preventDefault();
  if (e.metaKey || e.ctrlKey || e.shiftKey) {
    // The token is in sessionStorage, which is per tab. A tab the browser opens from a link
    // starts without it ("open it from hh ui" screen); window.open without noopener copies it.
    window.open(new URL(path, location.origin).href, "_blank");
    return;
  }
  router.go(path);
}
```


Run: `cd web && pnpm vitest run app/router.test.ts app/core-factory.test.ts`
Expected: PASS。既存の `home` だった `/workspaces/…/kanban`・`/projects/…/kanban` の期待は、上の差分で新しいルートに直してある。

- [ ] **Step 7: ListScope**

`web/app/list/scope.ts`:

```ts
// What a screen of tasks lists: one workspace, one project, or every store (the saved views, the
// query screen and the tasks assigned to me).
export type ListScope =
  | { kind: "workspace"; store: string; ulid: string }
  | { kind: "project"; ulid: string }
  | { kind: "all" }; // views, the query screen, my tasks: every store
```


`web/app/list/scope.svelte.ts`:

```ts
import type { ApiError, TaskView } from "../../core/api/client";
import { quoteArg } from "../../core/cli";
import type { SchemaView } from "../../core/schema";
import type { WorkspaceEntry } from "../../core/stores";
import { core } from "../context";
import { useSubscribe } from "../svelte-store";
import { baseQuery } from "../tree/filter";
import type { ListScope } from "./scope";

// useListScope resolves a ListScope against the core: the workspace or project it names, the store
// it is in, and the fixed part of the query that lists it. Call it while a component is created.
// The tree, the kanban and the table of one workspace or project read the same things from it.
export function useListScope(get: () => ListScope) {
  const c = core();
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));

  type DetailHandle = ReturnType<typeof c.cache.watchDetail>;
  let detail = $state<DetailHandle | null>(null);
  $effect(() => {
    const s = get();
    if (s.kind !== "project") return;
    const h = c.cache.watchDetail(s.ulid);
    detail = h;
    return () => {
      h.dispose();
      detail = null;
    };
  });

  const project = $derived.by<TaskView | undefined>(() => {
    cacheRev.current;
    const s = get();
    return s.kind === "project" ? c.cache.task(s.ulid) : undefined;
  });
  const store = $derived.by<string | undefined>(() => {
    const s = get();
    if (s.kind === "workspace") return s.store;
    return s.kind === "project" ? project?.store : undefined;
  });
  const workspace = $derived.by<WorkspaceEntry | undefined>(() => {
    storesRev.current;
    const s = get();
    if (s.kind === "workspace") return c.stores.workspace(s.store, s.ulid);
    if (s.kind !== "project" || !project) return undefined;
    return c.stores
      .workspaces()
      .find((w) => w.store === project.store && w.ulid === project.workspace.ulid);
  });
  // The workspace the address names is not in the stores (removed, or never there).
  const workspaceMissing = $derived.by(() => {
    storesRev.current;
    return get().kind === "workspace" && !workspace && c.stores.loaded();
  });
  // Why the project cannot be shown (deleted, not found, not fetched).
  const error = $derived.by<ApiError | null>(() => {
    cacheRev.current;
    return project ? null : (detail?.error() ?? null);
  });
  const base = $derived.by(() => {
    const s = get();
    if (s.kind === "workspace")
      return workspace ? { kind: "workspace" as const, name: workspace.name } : null;
    if (s.kind === "project") {
      return project ? { kind: "project" as const, ulid: project.ulid, id: project.id } : null;
    }
    return null;
  });
  const ready = $derived(get().kind === "all" || base !== null);
  const manyStores = $derived.by(() => {
    storesRev.current;
    return c.stores.stores().length >= 2;
  });

  return {
    /** The workspace or project is known (always true for every store). */
    get ready(): boolean {
      return ready;
    },
    /** The workspace or project is not there: the address names none, or the server says 404. */
    get notFound(): boolean {
      return workspaceMissing || error?.status === 404;
    },
    /** Why the project could not be read, when it is not a 404; null otherwise. */
    get error(): ApiError | null {
      return error;
    },
    /** The fixed part of the query for the API (the project by ULID); "" for every store or while not ready. */
    get apiBase(): string {
      return base ? baseQuery(base, "api") : "";
    },
    /** The fixed part as the user would type it in hh (the project by ID); "" for every store or while not ready. */
    get cliBase(): string {
      return base ? baseQuery(base, "cli") : "";
    },
    /** The store of the workspace or project; undefined for every store. */
    get store(): string | undefined {
      return store;
    },
    get workspace(): WorkspaceEntry | undefined {
      return workspace;
    },
    get project(): TaskView | undefined {
      return project;
    },
    /**
     * The fixed part of the query of a view saved from this screen: the cliBase, with the store in
     * front when two or more stores are mounted (a view is listed on every store). "" for every
     * store or while not ready.
     */
    get saveBase(): string {
      if (!base || !store) return "";
      const own = baseQuery(base, "cli");
      return manyStores ? `store:${quoteArg(store)} ${own}` : own;
    },
    /**
     * The schemas of the stores that are loaded, in the order asked. One that is not is fetched, and
     * the caller reads again when the schema cache notifies.
     */
    schemas(stores: string[]): SchemaView[] {
      schemaRev.current;
      const out: SchemaView[] = [];
      for (const name of stores) {
        const s = c.schema.get(name);
        if (s) out.push(s);
        else void c.schema.load(name).catch(() => undefined);
      }
      return out;
    },
  };
}
```


テスト（`useListScope` は `core()` を使うので `../context` をモックする。値はコンポーネントと同じく effect の中で読む）:

`web/app/list/scope.svelte.test.ts`:

```ts
import { flushSync } from "svelte";
import { describe, expect, it, vi } from "vitest";
import { ApiError, type TaskView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import type { WorkspaceEntry } from "../../core/stores";
import { emitter } from "../test-utils";
import { entry } from "../workspace/testkit";
import type { ListScope } from "./scope";
import { useListScope } from "./scope.svelte";

const h = vi.hoisted(() => ({ core: undefined as unknown }));
vi.mock("../context", () => ({ core: () => h.core }));

const task = (over: Partial<TaskView> = {}) =>
  ({
    ulid: "P1",
    id: "REL-15",
    store: "shared",
    kind: "project",
    title: "リリース準備",
    workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
    ...over,
  }) as TaskView;

const schema = {
  statuses: [],
  kinds: [],
  props: {},
  workflow: {},
  scoped: [],
} as unknown as SchemaView;

type World = {
  tasks?: Record<string, TaskView>;
  workspaces?: WorkspaceEntry[];
  stores?: string[];
  loaded?: boolean;
  schemas?: Record<string, SchemaView>;
  detailError?: ApiError | null;
};

function setup(world: World = {}) {
  const cache = emitter();
  const stores = emitter();
  const schemas = emitter();
  const w = { workspaces: [entry()], stores: ["shared"], loaded: true, ...world };
  const disposed = vi.fn();
  const watched: string[] = [];
  const loads: string[] = [];
  h.core = {
    cache: {
      subscribe: cache.subscribe,
      task: (u: string) => w.tasks?.[u],
      watchDetail: (u: string) => {
        watched.push(u);
        return { error: () => w.detailError ?? null, dispose: disposed };
      },
    },
    stores: {
      subscribe: stores.subscribe,
      workspace: (s: string, u: string) => w.workspaces.find((x) => x.store === s && x.ulid === u),
      workspaces: () => w.workspaces,
      stores: () => w.stores.map((name) => ({ name })),
      loaded: () => w.loaded,
    },
    schema: {
      subscribe: schemas.subscribe,
      get: (s: string) => w.schemas?.[s],
      load: (s: string) => {
        loads.push(s);
        return Promise.resolve(schema);
      },
    },
  };
  return { w, cache, stores, schemas, disposed, watched, loads };
}

type Scope = ReturnType<typeof useListScope>;
// The fields as an effect last read them: a value that depends on the core is read in an effect, as a
// component does, since the core's notifications only reach what is read there.
type Snapshot = {
  -readonly [K in
    | "ready"
    | "notFound"
    | "error"
    | "apiBase"
    | "cliBase"
    | "store"
    | "workspace"
    | "project"
    | "saveBase"
    | "schemas"]: Scope[K];
};

function use(get: () => ListScope) {
  const scope = {} as Snapshot;
  const stop = $effect.root(() => {
    const s = useListScope(get);
    scope.schemas = s.schemas;
    $effect(() => {
      scope.ready = s.ready;
      scope.notFound = s.notFound;
      scope.error = s.error;
      scope.apiBase = s.apiBase;
      scope.cliBase = s.cliBase;
      scope.store = s.store;
      scope.workspace = s.workspace;
      scope.project = s.project;
      scope.saveBase = s.saveBase;
    });
  });
  flushSync();
  return { scope, stop };
}

describe("useListScope: a workspace", () => {
  it("is ready with the workspace, and its query names the workspace", () => {
    setup();
    const { scope, stop } = use(() => ({ kind: "workspace", store: "shared", ulid: "W1" }));
    expect(scope.ready).toBe(true);
    expect(scope.notFound).toBe(false);
    expect(scope.store).toBe("shared");
    expect(scope.workspace?.name).toBe("release");
    expect(scope.project).toBeUndefined();
    expect(scope.apiBase).toBe("workspace:release");
    expect(scope.cliBase).toBe("workspace:release");
    expect(scope.saveBase).toBe("workspace:release");
    stop();
  });

  it("quotes a name with a space, and puts the store first when two or more are mounted", () => {
    setup({ workspaces: [entry({ name: "web app" })], stores: ["shared", "personal"] });
    const { scope, stop } = use(() => ({ kind: "workspace", store: "shared", ulid: "W1" }));
    expect(scope.cliBase).toBe('workspace:"web app"');
    expect(scope.saveBase).toBe('store:shared workspace:"web app"');
    stop();
  });

  it("is not ready while the stores are not read, and not found once they are", () => {
    const { w, stores } = setup({ workspaces: [], loaded: false });
    const { scope, stop } = use(() => ({ kind: "workspace", store: "shared", ulid: "W1" }));
    expect(scope.ready).toBe(false);
    expect(scope.notFound).toBe(false);
    expect(scope.apiBase).toBe("");
    expect(scope.saveBase).toBe("");
    w.loaded = true;
    stores.emit();
    flushSync();
    expect(scope.notFound).toBe(true);
    stop();
  });

  it("follows the scope it is given", () => {
    setup({ workspaces: [entry(), entry({ ulid: "W2", name: "web" })] });
    let current = $state<ListScope>({ kind: "workspace", store: "shared", ulid: "W1" });
    const { scope, stop } = use(() => current);
    expect(scope.cliBase).toBe("workspace:release");
    current = { kind: "workspace", store: "shared", ulid: "W2" };
    flushSync();
    expect(scope.cliBase).toBe("workspace:web");
    stop();
  });
});

describe("useListScope: a project", () => {
  it("reads the project from the cache: the API takes its ULID, the command its ID", () => {
    const { watched } = setup({ tasks: { P1: task() } });
    const { scope, stop } = use(() => ({ kind: "project", ulid: "P1" }));
    expect(scope.ready).toBe(true);
    expect(scope.project?.id).toBe("REL-15");
    expect(scope.store).toBe("shared");
    expect(scope.workspace?.ulid).toBe("W1");
    expect(scope.apiBase).toBe("project:P1");
    expect(scope.cliBase).toBe("project:REL-15");
    expect(scope.saveBase).toBe("project:REL-15");
    expect(watched).toEqual(["P1"]);
    stop();
  });

  it("puts the store first in saveBase when two or more are mounted", () => {
    setup({ tasks: { P1: task() }, stores: ["personal", "shared"] });
    const { scope, stop } = use(() => ({ kind: "project", ulid: "P1" }));
    expect(scope.saveBase).toBe("store:shared project:REL-15");
    stop();
  });

  it("is not ready until the project is fetched; a 404 is not found, another error is kept", () => {
    const { w, cache } = setup({ tasks: {} });
    const { scope, stop } = use(() => ({ kind: "project", ulid: "P1" }));
    expect(scope.ready).toBe(false);
    expect(scope.notFound).toBe(false);
    expect(scope.error).toBeNull();
    expect(scope.store).toBeUndefined();
    w.detailError = new ApiError({ status: 500, type: "internal" });
    cache.emit();
    flushSync();
    expect(scope.notFound).toBe(false);
    expect(scope.error?.status).toBe(500);
    w.detailError = new ApiError({ status: 404, type: "task.not_found" });
    cache.emit();
    flushSync();
    expect(scope.notFound).toBe(true);
    w.tasks = { P1: task() };
    cache.emit();
    flushSync();
    expect(scope.ready).toBe(true);
    expect(scope.error).toBeNull();
    stop();
  });

  it("lets go of the detail when it goes away", () => {
    const { disposed } = setup({ tasks: { P1: task() } });
    const { stop } = use(() => ({ kind: "project", ulid: "P1" }));
    expect(disposed).not.toHaveBeenCalled();
    stop();
    expect(disposed).toHaveBeenCalledTimes(1);
  });
});

describe("useListScope: every store", () => {
  it("is ready at once and has no fixed query, store, workspace or project", () => {
    const { watched } = setup({ stores: ["shared", "personal"] });
    const { scope, stop } = use(() => ({ kind: "all" }));
    expect(scope.ready).toBe(true);
    expect(scope.notFound).toBe(false);
    expect(scope.error).toBeNull();
    expect(scope.apiBase).toBe("");
    expect(scope.cliBase).toBe("");
    expect(scope.saveBase).toBe("");
    expect(scope.store).toBeUndefined();
    expect(scope.workspace).toBeUndefined();
    expect(scope.project).toBeUndefined();
    expect(watched).toEqual([]);
    stop();
  });
});

describe("useListScope: schemas", () => {
  it("gives the schemas that are loaded in the order asked, and fetches the others", () => {
    const other = { ...schema, kinds: [] } as SchemaView;
    const { loads, schemas, w } = setup({ schemas: { shared: schema } });
    const { scope, stop } = use(() => ({ kind: "all" }));
    expect(scope.schemas(["personal", "shared"])).toEqual([schema]);
    expect(loads).toEqual(["personal"]);
    w.schemas = { shared: schema, personal: other };
    schemas.emit();
    expect(scope.schemas(["personal", "shared"])).toEqual([other, schema]);
    expect(scope.schemas([])).toEqual([]);
    stop();
  });

  it("is read again when the schema cache notifies", () => {
    const { schemas, w } = setup({ schemas: {} });
    const { scope, stop } = use(() => ({ kind: "all" }));
    const seen: number[] = [];
    const stopRead = $effect.root(() => {
      $effect(() => {
        seen.push(scope.schemas(["shared"]).length);
      });
    });
    flushSync();
    w.schemas = { shared: schema };
    schemas.emit();
    flushSync();
    expect(seen).toEqual([0, 1]);
    stopRead();
    stop();
  });
});
```


Run: `cd web && pnpm vitest run app/list/scope.svelte.test.ts`
Expected: PASS

- [ ] **Step 8: FilterBar の札と ListBar**

`web/app/tree/FilterBar.svelte` の差分（`@@` の行は目安。該当する箇所に当てる）:

```diff
@@ -4,7 +4,7 @@
   import CliLine from "../ui/CliLine.svelte";
 
   type Props = {
-    base: string; // the fixed part, as the user would type it
+    base: string; // the fixed part, as the user would type it; "" for none (every store)
     applied: string; // what is applied now
     count: string; // "16 件 · 中止 1 件を除外"
     cli: string; // the hh ls line
@@ -42,7 +42,9 @@
 <div class="bar">
   <div class="field" class:bad={message !== null}>
     <Icon name="filter" size={15} />
-    <span class="base" title="この画面で固定のクエリ">{base}</span>
+    {#if base}
+      <span class="base" title="この画面で固定のクエリ">{base}</span>
+    {/if}
     <label class="sr" for="tree-q">フィルタ（hh ls と同じクエリ）</label>
     <input
       id="tree-q"
```


`web/app/tree/FilterBar.test.ts` の差分（`@@` の行は目安。該当する箇所に当てる）:

```diff
@@ -3,6 +3,15 @@
 import FilterBar from "./FilterBar.svelte";
 
 describe("FilterBar", () => {
+  it("shows the fixed part of the query, and nothing for none", () => {
+    const props = { applied: "", count: "0 件", cli: "hh ls", error: null, onchange: vi.fn() };
+    const a = render(FilterBar, { ...props, base: "project:REL-15" });
+    expect(a.getByTitle("この画面で固定のクエリ").textContent).toBe("project:REL-15");
+    a.unmount();
+    const b = render(FilterBar, { ...props, base: "" });
+    expect(b.queryByTitle("この画面で固定のクエリ")).toBeNull();
+  });
+
   it("does not apply the typed query after it is removed", async () => {
     vi.useFakeTimers();
     try {
```


`web/app/list/ListBar.svelte`:

```svelte
<script lang="ts">
  import type { Snippet } from "svelte";
  import FilterBar from "../tree/FilterBar.svelte";

  type Props = {
    base: string; // the fixed part of the query, as the user would type it; "" for every store
    applied: string; // what is applied now
    count: string; // "16 件 · 中止 1 件を除外"
    cli: string; // the hh line under the field
    error: unknown;
    onchange: (q: string) => void;
    controls?: Snippet; // the switches of the layout (columns, swimlane...)
    actions?: Snippet; // the buttons ("ビューを保存"...)
  };
  let { base, applied, count, cli, error, onchange, controls, actions }: Props = $props();
</script>

<!-- The filter of every list screen, with the switches of its layout and its buttons at the right. -->
<div class="row">
  <div class="filter">
    <FilterBar {base} {applied} {count} {cli} {error} {onchange} />
  </div>
  {#if controls || actions}
    <div class="side">
      {#if controls}
        <div class="controls">{@render controls()}</div>
      {/if}
      {#if actions}
        <div class="actions">{@render actions()}</div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .row {
    display: flex;
    align-items: flex-start;
  }
  .filter {
    flex: 1;
    min-width: 0;
  }
  /* The bar has its own gutter at both ends; the side takes the right one. */
  .filter :global(.bar) {
    padding-right: 0.75rem;
  }
  .side {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    min-height: 2.25rem;
    margin: 0.75rem 1.75rem 0 0;
  }
  .controls,
  .actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
</style>
```


`web/app/list/ListBar.test.ts`:

```ts
import { fireEvent, render } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { describe, expect, it, vi } from "vitest";
import ListBar from "./ListBar.svelte";

const text = (s: string) => createRawSnippet(() => ({ render: () => `<button>${s}</button>` }));

const props = () => ({
  base: "workspace:release",
  applied: "-status:canceled",
  count: "16 件",
  cli: "hh ls workspace:release -- -status:canceled",
  error: null,
  onchange: vi.fn(),
});

describe("ListBar", () => {
  it("shows the filter: the fixed part, the query applied, the count and the hh line", () => {
    const { getByLabelText, getByText, getByTitle } = render(ListBar, props());
    expect(getByTitle("この画面で固定のクエリ").textContent).toBe("workspace:release");
    expect((getByLabelText("フィルタ（hh ls と同じクエリ）") as HTMLInputElement).value).toBe(
      "-status:canceled",
    );
    expect(getByText("16 件")).toBeTruthy();
    expect(getByText("$ hh ls workspace:release -- -status:canceled")).toBeTruthy();
  });

  it("tells the query typed with Enter", async () => {
    const p = props();
    const { getByLabelText } = render(ListBar, p);
    const input = getByLabelText("フィルタ（hh ls と同じクエリ）");
    await fireEvent.input(input, { target: { value: " assignee:@me " } });
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(p.onchange).toHaveBeenCalledWith("assignee:@me");
  });

  it("shows the error of the query", () => {
    const { getByRole } = render(ListBar, {
      ...props(),
      error: new Error("bad query"),
    });
    expect(getByRole("alert")).toBeTruthy();
  });

  it("puts the controls and the actions at the right, when given", () => {
    const none = render(ListBar, props());
    expect(none.queryByText("列: ステータス")).toBeNull();
    none.unmount();
    const { getByText } = render(ListBar, {
      ...props(),
      controls: text("列: ステータス"),
      actions: text("ビューを保存"),
    });
    expect(getByText("列: ステータス")).toBeTruthy();
    expect(getByText("ビューを保存")).toBeTruthy();
  });

  it("has no fixed part for every store", () => {
    const { queryByTitle } = render(ListBar, { ...props(), base: "" });
    expect(queryByTitle("この画面で固定のクエリ")).toBeNull();
  });
});
```


- [ ] **Step 9: 全部を通す**

Run: `cd web && pnpm fix && pnpm check && pnpm exec biome ci --vcs-enabled=false . && pnpm test`
Expected: すべて PASS（警告 0）。`TreeView.test.ts` の時間切れが1件出たら、単独で再実行して通ることを確かめ、失敗の出力と条件を残す。

- [ ] **Step 10: コミット**

```bash
git add web/package.json web/pnpm-lock.yaml web/core web/app
git commit -m "feat(web): ビューのキャッシュ・ルート・ListScope・ListBar の骨組み

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

### Task 3: ツリーを `ListScope` と外から渡すクエリに対応させる（D5）

ビュー・クエリの画面のツリーは、全 Store のタスクを1本のツリーに並べる。`TreeView` は今、ワークスペースかプロジェクトの1つの Store を前提にスキーマ・ワークスペース・作成先を1つだけ持っている。これを「行ごとの Store」で引く形に直す。ワークスペース・プロジェクトの画面の挙動は変えない（4d・4e-2 の既存テストがそのまま通ること）。

**順序と並行性の扱い（窓を塞がず構造で直す）:** 全 Store のツリーの根は1回の応答で届く。Store の順への並べ替えは、ツリー・タスク・Store の一覧のそのときの値から毎回作り直す派生にして、どこにも持たない（`orderRootsByStore`）。ツリーとタスクの到着の前後で並びが狂う窓を作らない。キーのスキーマ待ちは「押した行の Store」と一緒に待ち行列に入れ、その Store のスキーマが読めた順に流す。失敗は Store ごとに持ち、スキーマが無いときにだけ読む（古い読み込みの失敗が新しい読み込みの成功を上書きしない）。スキーマの読み込みは Store の集合が変わったときだけやり直す（文字列のキーで派生させる）。

**Files:**
- Modify: `web/app/tree/TreeView.svelte`
- Modify: `web/app/tree/TreeRow.svelte`（`storeTone`）
- Modify: `web/app/tree/AddRow.svelte`（`where` が空のときヒントを出さない）
- Modify: `web/app/tree/keyboard.ts`（兄弟は同じ Store の行だけ）
- Modify: `web/app/tree/filter.ts`（`combine` の base が空のとき）
- Create: `web/app/tree/stores-order.ts`
- Test: `web/app/tree/keyboard.test.ts`、`web/app/tree/filter.test.ts`、`web/app/tree/stores-order.test.ts`（新規）、`web/app/tree/TreeView.stores.test.ts`（新規）

**Interfaces:**
- Consumes（部 A。Task 2 でできている）:
  - `ListScope`（`web/app/list/scope.ts`）: `{ kind: "workspace"; store: string; ulid: string } | { kind: "project"; ulid: string } | { kind: "all" }`
  - `useListScope(get: () => ListScope)`（`web/app/list/scope.svelte.ts`）の getter: `ready: boolean`、`notFound: boolean`、`apiBase: string`（all は `""`）、`cliBase: string`（all は `""`）、`store: string | undefined`（all は undefined）、`workspace: WorkspaceEntry | undefined`、`project: TaskView | undefined`、`error: ApiError | null`（プロジェクトを出せない理由。404 以外の表示用）
  - `ListBar`（`web/app/list/ListBar.svelte`）の props: `{ base; applied; count; cli; error; onchange; controls?; actions? }`
  - 既存: `openCreate`（`create-state.svelte.ts`）、`core().stores.stores()`（`StoreItem[]`、GET /api/stores の順）、`core().stores.workspace(store, ulid)`、`core().stores.tone(store)`、`core().schema.get/load`
- Produces:
  - `TreeView` の props: `{ view: ListScope; query?: string; onquery?: (q: string) => void; cli?: string; actions?: Snippet }`
    - `query` が undefined なら今どおり自分で持つ（既定 `-status:canceled`）。`query !== undefined` なら `""` でも外が正（内部の既定にしない。実装は `query ?? own`）で、フィルタ欄の変更は `onquery` に渡すだけ（自分では持たない）
    - `cli` があれば足元の行に出す（なければ `hh ls --tree <クエリ>`）
    - `actions` はフィルタの段の右（`ListBar` の `actions`）
    - `view` が `{ kind: "all" }` のとき: `watchTree({ q, tree: true })`・`is:blocked` と canceled の件数の `watchList` を store なしで張る。根を Store の順に並べ、Store が2つ以上なら各行に Store のバッジ。種類・親の規則・キー操作・メニューは行の Store のスキーマ。「下に追加」は行の Store とワークスペースに作る。足元の「タスクを追加」は作成ダイアログ（`openCreate({ kind: "task" })`）を開く。Store をまたぐドロップは既存の `dropDecision` が拒否する
  - `orderRootsByStore(roots: TreeNode[], storeOrder: readonly string[], taskOf: (ulid: string) => TaskView | undefined): TreeNode[]`、`storesOf(roots: TreeNode[], taskOf): string[]`（`web/app/tree/stores-order.ts`）
  - `TreeRow` の新しい prop: `storeTone?: StoreTone`（あれば行の右に `StoreBadge`（compact）を出す）
  - `combine("", user)` は `user.trim()`（先頭に空白を付けない）。部 C・D・F が `view={{ kind: "all" }}` のクエリを作るときもこれを使う

- [ ] **Step 0: 既存のテストの負荷下の落ち方を直す（Task 3 のほかの変更と分けて先にコミット。TreeView・BodyEditor のテストと `vitest.config.ts` の3つを1つのコミットで）**

`web/app/tree/TreeView.test.ts` の「lets a late wait for a row give way to a newer one」が、他のテストが走っていて詰まったマシンで落ちる（`expected null to be 'M199'`）。証拠: main 79a5277 の web で、16 コアの WSL2 に全件を 2 本裏で流しながらこのテストを含む全件を 4 回続けて流すと 4 回中 2 回落ちる（全件 3 本を裏で流しながらこのテストを 8 本同時に流すと 8 回中 7 回）。負荷なしの全件 4 回は通る。計測では 80ms のタイマーが約 1.1 秒遅れて動き、`waitFor` の 1 秒が切れる。`focusRow` の `FOCUS_FRAMES` の打ち切りではない（2000 にしても 8 回中 5 回落ちた）。原因はテストが実時間（`setTimeout` の 10ms・80ms、rAF、`waitFor` の 1 秒、200ms の待ち）に頼っていること。製品のコードの不具合ではない。このテストだけ偽のタイマーにする。

このテストを次に置き換える（`it("lets a late wait for a row give way to a newer one", async () => {` から対応する `});` まで）:

```ts
  it("lets a late wait for a row give way to a newer one", async () => {
    // The clock is a fake one: with real timers, a busy machine (the suites of other workers)
    // held the 10ms draw past the 1s of waitFor, and the test failed with the focus on nothing.
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "requestAnimationFrame"] });
    try {
      setup({ many: 250 });
      render(TreeView, { view: { kind: "project", ulid: "P" } });
      const first = screen.getAllByRole("treeitem")[0] as HTMLElement;
      await fireEvent.click(first);
      selection.set("M100");
      h.vdelay = 80;
      await fireEvent.keyDown(first, { key: "ArrowDown" }); // M101, drawn after 80ms
      selection.set("M200");
      h.vdelay = 10;
      await fireEvent.keyDown(first, { key: "ArrowUp" }); // M199, drawn after 10ms
      await vi.advanceTimersByTimeAsync(40);
      expect(document.activeElement?.getAttribute("data-ulid")).toBe("M199");
      // The first request draws its window late (and moves the mock window back): it must not
      // then take the focus for M101.
      await vi.advanceTimersByTimeAsync(200);
      expect(document.activeElement?.getAttribute("data-ulid")).not.toBe("M101");
    } finally {
      vi.useRealTimers();
    }
  });
```

続けて、同じ種類の時間切れをあと2か所直す（証拠は決めたこと 21）。

(a) `web/app/detail/BodyEditor.test.ts` の「offers the people for @ from GET /api/people」を偽のタイマーにする。CodeMirror は打鍵の 100ms 後（`activateOnTypingDelay`）に補完を始めるので、実時間の `vi.waitFor` の 1 秒では、詰まったマシンで切れる。この `it` を次に置き換える（`it("offers the people for @ from GET /api/people", async () => {` から対応する `});` まで）:

```ts
  it("offers the people for @ from GET /api/people", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation(() =>
      Promise.resolve({ data: ["suzuki", "tanaka"], response: new Response() }),
    );
    const { container } = mount(BodyEditor, props({ value: "" }), k);
    const view = viewOf(container);
    // The completion starts 100ms after the typing (CodeMirror's activateOnTypingDelay). The clock
    // is a fake one: on a busy machine the real 100ms ran past the 1s of waitFor.
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    try {
      view.dispatch({
        changes: { from: 0, insert: "@su" },
        selection: { anchor: 3 },
        userEvent: "input.type",
      });
      await vi.advanceTimersByTimeAsync(150);
      expect(k.api.GET).toHaveBeenCalledWith("/api/people", {
        params: { query: { store: "shared" } },
      });
    } finally {
      vi.useRealTimers();
    }
  });
```

(b) `web/vitest.config.ts` の `test` に `testTimeout: 20_000` を足す。TaskDetail 系・CreateTask の各ファイルの最初のテストは、単独で約 0.5 秒だが、全件を同時に何本も流すと既定の 5 秒を超える。止まったテストはそれでも 20 秒で落ちる:

```ts
  test: {
    environment: "jsdom",
    include: ["core/**/*.test.ts", "app/**/*.test.ts"],
    setupFiles: ["./vitest.setup.ts"],
    // A test takes about 0.5s alone; on a busy machine (other suites at the same time) it took
    // over the default 5s. A test that hangs still fails, at 20s.
    testTimeout: 20_000,
  },
```

```bash
cd web && pnpm vitest run app/tree/TreeView.test.ts app/detail/BodyEditor.test.ts
```

期待: 全件 PASS。確かめたこと: 直す前は負荷下で 4 回中 2 回落ち、直すと同じ負荷で 4 回とも通る（裏の全件 866 件も通る）。`focusRow` の `seq` の判定を外すと `expected 'M101' not to be 'M101'` で落ちる（テストが不具合を捕まえることの保証）。負荷なしで 359ms。

```bash
git add web/app/tree/TreeView.test.ts web/app/detail/BodyEditor.test.ts web/vitest.config.ts
git commit -m "test(web): 実時間に頼るテストを偽のタイマーにし、テストの時間切れを 20 秒にして、負荷下で落ちないようにする

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 1: 失敗するテストを書く（キーボード・combine・並べ替え）**

`web/app/tree/keyboard.test.ts` の末尾（`describe("other keys", …)` の後）に足す:

```ts
describe("a view over several stores", () => {
  // The top rows of two stores sit under the same null parent.
  const two = new Map([
    ["A1", task("A1", "project", null, { store: "alpha" })],
    ["A2", task("A2", "project", null, { store: "alpha" })],
    ["B1", task("B1", "project", null, { store: "beta" })],
    ["B2", task("B2", "project", null, { store: "beta" })],
  ]);
  const twoOf = (u: string) => two.get(u);
  const roots = ["A1", "A2", "B1", "B2"].map((ulid) => ({ ulid, children: [] }));
  const multi = (selected: string): KeyContext => ({
    rows: flatten(roots, null, twoOf, new Set()),
    selected,
    schema,
    viewRoot: null,
    taskOf: twoOf,
  });

  it("reorders within a store and stops at the edge of it", () => {
    expect(keyAction(key("ArrowUp", { alt: true }), multi("A2"))).toEqual({
      type: "move",
      ulid: "A2",
      to: { before: "A1" },
    });
    expect(keyAction(key("ArrowDown", { alt: true }), multi("A2"))).toEqual({ type: "none" });
    expect(keyAction(key("ArrowUp", { alt: true }), multi("B1"))).toEqual({ type: "none" });
  });
  it("does not indent the first top row of a store under the last one of another", () => {
    expect(keyAction(key("Tab"), multi("B1"))).toEqual({ type: "none" });
    expect(keyAction(key("Tab"), multi("B2"))).toMatchObject({ type: "reject" });
  });
});
```

`web/app/tree/filter.test.ts` の `describe("combine", …)` の中、既存の `it` の後に足す:

```ts
  it("has no space to lead with when there is no base", () => {
    expect(combine("", " assignee:@me ")).toBe("assignee:@me");
    expect(combine("", "")).toBe("");
  });
```

`web/app/tree/stores-order.test.ts` を新規作成:

```ts
import { describe, expect, it } from "vitest";
import type { TreeNode } from "../../core/cache";
import { task } from "./fixtures";
import { orderRootsByStore, storesOf } from "./stores-order";

const tasks = new Map([
  ["B1", task("B1", "project", null, { store: "beta" })],
  ["A1", task("A1", "project", null, { store: "alpha" })],
  ["B2", task("B2", "project", null, { store: "beta" })],
  ["A2", task("A2", "project", null, { store: "alpha" })],
  ["X1", task("X1", "project", null, { store: "gone" })],
]);
const taskOf = (u: string) => tasks.get(u);
const node = (ulid: string): TreeNode => ({ ulid, children: [] });
const ulids = (ns: TreeNode[]) => ns.map((n) => n.ulid);

describe("orderRootsByStore", () => {
  it("orders by the stores and keeps the order within a store", () => {
    const roots = ["B1", "A1", "B2", "A2"].map(node);
    expect(ulids(orderRootsByStore(roots, ["alpha", "beta"], taskOf))).toEqual([
      "A1",
      "A2",
      "B1",
      "B2",
    ]);
    expect(ulids(orderRootsByStore(roots, ["beta", "alpha"], taskOf))).toEqual([
      "B1",
      "B2",
      "A1",
      "A2",
    ]);
  });
  it("puts a root of an unlisted store, or not in the cache, last", () => {
    const roots = ["X1", "N9", "A1", "B1"].map(node);
    expect(ulids(orderRootsByStore(roots, ["alpha", "beta"], taskOf))).toEqual([
      "A1",
      "B1",
      "X1",
      "N9",
    ]);
  });
  it("returns the same array when nothing moves", () => {
    const roots = ["A1", "A2", "B1"].map(node);
    expect(orderRootsByStore(roots, ["alpha", "beta"], taskOf)).toBe(roots);
  });
});

describe("storesOf", () => {
  it("lists each store once, in the order of the roots", () => {
    expect(storesOf(["B1", "A1", "B2", "N9"].map(node), taskOf)).toEqual(["beta", "alpha"]);
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

```bash
cd web && pnpm vitest run app/tree/keyboard.test.ts app/tree/filter.test.ts app/tree/stores-order.test.ts
```

期待: `keyboard.test.ts` の「reorders within a store…」「does not indent…」と `filter.test.ts` の「has no space…」が失敗し、`stores-order.test.ts` は `./stores-order` が無くて失敗する。

- [ ] **Step 3: キーボードの兄弟と `combine` と並べ替えを実装する**

`web/app/tree/keyboard.ts` の `siblingsOf` を置き換える:

```ts
// The siblings share the parent and the store: the top rows of a view over several stores sit under
// the same null, and a task cannot be put before, after or under one of another store.
function siblingsOf(row: FlatRow, c: KeyContext): FlatRow[] {
  return c.rows.filter((r) => r.parent === row.parent && r.task.store === row.task.store);
}
```

`web/app/tree/filter.ts` の `combine` を置き換える:

```ts
// combine puts the base before the user's input. Every term of a query is
// ANDed, so joining them narrows the base and never leaves it. A view over
// every store has no base: its query is the user's input alone.
export function combine(base: string, user: string): string {
  const u = user.trim();
  if (base === "") return u;
  return u ? `${base} ${u}` : base;
}
```

`web/app/tree/stores-order.ts` を新規作成:

```ts
import type { TaskView } from "../../core/api/client";
import type { TreeNode } from "../../core/cache";

// orderRootsByStore puts the roots of a tree read over every store in the order of the stores
// (the order of GET /api/stores), keeping the order they came in within a store. A root whose task is
// not in the cache, or of a store that is not listed, goes last. It reads the cache at the moment it
// is called and holds nothing, so it needs no care about when the tree and the tasks arrive: it is
// called again with every change of either. It returns roots itself when nothing moves.
export function orderRootsByStore(
  roots: TreeNode[],
  storeOrder: readonly string[],
  taskOf: (ulid: string) => TaskView | undefined,
): TreeNode[] {
  const rank = (n: TreeNode): number => {
    const store = taskOf(n.ulid)?.store;
    const i = store === undefined ? -1 : storeOrder.indexOf(store);
    return i < 0 ? storeOrder.length : i;
  };
  const sorted = roots
    .map((node, at) => ({ node, at, rank: rank(node) }))
    .sort((a, b) => a.rank - b.rank || a.at - b.at)
    .map((x) => x.node);
  return sorted.every((n, i) => n === roots[i]) ? roots : sorted;
}

// storesOf lists the stores the roots belong to, in the order of the roots. A child is always in
// the store of its parent, so the roots hold every store of the tree.
export function storesOf(
  roots: TreeNode[],
  taskOf: (ulid: string) => TaskView | undefined,
): string[] {
  const out: string[] = [];
  for (const n of roots) {
    const store = taskOf(n.ulid)?.store;
    if (store !== undefined && !out.includes(store)) out.push(store);
  }
  return out;
}
```

- [ ] **Step 4: 通ることを確かめる**

```bash
cd web && pnpm vitest run app/tree/keyboard.test.ts app/tree/filter.test.ts app/tree/stores-order.test.ts
```

期待: 全部 PASS。

- [ ] **Step 5: `TreeRow` と `AddRow` を直す**

`web/app/tree/TreeRow.svelte`:

import に2行足す（Biome の並びに合わせる）。`SchemaView` の import の次に `StoreTone`、`IdLabel` の import の次に `StoreBadge`:

```ts
  import type { SchemaView } from "../../core/schema";
  import type { StoreTone } from "../../core/stores";
```

```ts
  import IdLabel from "../ui/IdLabel.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
```

`Props` の `dropMark` の行の次に足す:

```ts
    storeTone?: StoreTone; // set when the view lists the tasks of several stores: the row shows its store
```

props の分割代入の `dropMark = null,` の次に `storeTone,` を足す:

```ts
    dropMark = null,
    storeTone,
  }: Props = $props();
```

右側の `<span class="tail">` の直後に足す:

```svelte
  <span class="tail">
    {#if storeTone}
      <StoreBadge store={task.store} tone={storeTone} compact />
    {/if}
    {#if blockedBy}
```

`web/app/tree/AddRow.svelte`: `where` の説明を直す:

```ts
    where: string; // "release / リリース準備 v2.4"; "" for a button that opens the create dialog
```

ボタンの側（`{:else}` の枝）の `where` の行を置き換える:

```svelte
    <span>タスクを追加</span>
    {#if where !== ""}
      <span class="where">→ {where} · 種類: {kindLabel}</span>
    {/if}
```

- [ ] **Step 6: `TreeView` の失敗するテストを書く**

`web/app/tree/TreeView.stores.test.ts` を新規作成（Task 2 の部品はスタブでなく本物で通る。`../context` と `../router` をモックするので `useListScope` も同じモックの core を使う）:

```ts
import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { closeCreate, createState } from "../create-state.svelte";
import { selection } from "../selection.svelte";
import { emitter } from "../test-utils";
import { schema, task } from "./fixtures";

type Opts = {
  ulid: string;
  onOver: (place: string | null, source: string | null) => void;
  onDrop: (source: string, place: string) => void;
};
const h = vi.hoisted(() => ({
  core: undefined as unknown,
  toast: vi.fn(),
  rows: new Map<string, unknown>(),
}));

vi.mock("../context", () => ({ core: () => h.core }));
vi.mock("../router", () => ({
  router: { route: { name: "my" }, go: vi.fn(), href: () => "/" },
  onLinkClick: () => {},
}));
vi.mock("../toast.svelte", () => ({ toast: h.toast }));
vi.mock("@tanstack/svelte-virtual", () => ({
  createVirtualizer: () => ({
    subscribe(fn: (v: unknown) => void) {
      fn({ getVirtualItems: () => [], getTotalSize: () => 0, scrollToIndex() {}, setOptions() {} });
      return () => {};
    },
    setOptions() {},
  }),
}));
vi.mock("./dnd", async (orig) => {
  const real = await orig<typeof import("./dnd")>();
  return {
    ...real,
    rowDnd: (get: () => Opts) => () => {
      h.rows.set(get().ulid, get());
      return () => {};
    },
    treeDragEnd: () => () => () => {},
    treeAutoScroll: () => () => () => {},
  };
});

import TreeView from "./TreeView.svelte";

const ok = (data: unknown) => ({ data, response: new Response(null, { status: 200 }) });
const row = (u: string) => h.rows.get(u) as Opts;
const treeitem = (title: string) =>
  screen.getByText(title).closest('[role="treeitem"]') as HTMLElement;

// Two stores. The server lists the roots of beta before those of alpha; the stores are alpha, beta.
//   alpha: A1 (epic) > A2 (task)       beta: B1, B2 (tasks)
const noDone = {
  ...schema,
  statuses: schema.statuses?.filter((s) => s.category !== "done"),
} as typeof schema;
const wsOf = {
  alpha: { ulid: "WA", name: "alpha-ws", id_prefix: "AAA" },
  beta: { ulid: "WB", name: "beta-ws", id_prefix: "BBB" },
};

function setup(
  over: { oneStore?: boolean; betaSchema?: "none" | "ok" | "nodone"; roots?: unknown } = {},
) {
  const tasks = new Map([
    ["A1", task("A1", "epic", null, { store: "alpha", workspace: wsOf.alpha, title: "alpha one" })],
    ["A2", task("A2", "task", "A1", { store: "alpha", workspace: wsOf.alpha, title: "alpha two" })],
    ["B1", task("B1", "task", null, { store: "beta", workspace: wsOf.beta, title: "beta one" })],
    ["B2", task("B2", "task", null, { store: "beta", workspace: wsOf.beta, title: "beta two" })],
  ]);
  const roots = over.roots ?? [
    { ulid: "B1", children: [] },
    { ulid: "A1", children: [{ ulid: "A2", children: [] }] },
    { ulid: "B2", children: [] },
  ];
  const entry = (store: "alpha" | "beta") => ({
    store,
    tone: store === "alpha" ? "teal" : "orange",
    ulid: wsOf[store].ulid,
    name: wsOf[store].name,
    prefix: wsOf[store].id_prefix,
    projects: [],
  });
  const ev = emitter();
  let betaSchema: typeof schema | undefined =
    over.betaSchema === "none" ? undefined : over.betaSchema === "nodone" ? noDone : schema;
  const watchTree = vi.fn(() => ({
    roots: () => roots,
    error: () => null,
    loading: () => false,
    dispose() {},
  }));
  const watchList = vi.fn((q: { q: string }) => ({
    ulids: () => (q.q.includes("status:canceled") ? ["C1", "C2"] : []),
    error: () => null,
    loading: () => false,
    dispose() {},
  }));
  const load = vi.fn(async (store: string) => (store === "beta" ? (betaSchema ?? schema) : schema));
  const api = { POST: vi.fn(), PATCH: vi.fn() };
  h.core = {
    api,
    cache: {
      task: (u: string) => tasks.get(u),
      live: () => undefined,
      watchTree,
      watchList,
      watchDetail: vi.fn(),
      applyWrite: vi.fn(),
      refreshTrees: vi.fn(async () => {}),
      subscribe: ev.subscribe,
    },
    stores: {
      loaded: () => true,
      stores: () => (over.oneStore ? [{ name: "alpha" }] : [{ name: "alpha" }, { name: "beta" }]),
      workspace: (s: "alpha" | "beta", u: string) => (wsOf[s]?.ulid === u ? entry(s) : undefined),
      workspaces: () => [entry("alpha"), entry("beta")],
      tone: (s: string) => (s === "alpha" ? "teal" : "orange"),
      subscribe: ev.subscribe,
    },
    schema: {
      get: (s: string) => (s === "beta" ? betaSchema : schema),
      load,
      subscribe: ev.subscribe,
    },
  };
  return {
    api,
    watchTree,
    watchList,
    load,
    // The schema of beta arrives
    loadBeta() {
      betaSchema = schema;
      ev.emit();
    },
  };
}

const all = { kind: "all" } as const;

beforeEach(() => {
  h.toast.mockReset();
  h.rows.clear();
  selection.set(null);
  closeCreate();
});

describe("TreeView over every store", () => {
  it("lists the roots in the order of the stores, each row with the badge of its store", () => {
    setup();
    render(TreeView, { view: all, query: "" });
    expect(screen.getAllByRole("treeitem").map((el) => el.getAttribute("data-ulid"))).toEqual([
      "A1",
      "A2",
      "B1",
      "B2",
    ]);
    expect(screen.getAllByTitle("Store: alpha")).toHaveLength(2);
    expect(screen.getAllByTitle("Store: beta")).toHaveLength(2);
  });

  it("shows no badge when there is one store", () => {
    setup({ oneStore: true });
    render(TreeView, { view: all, query: "" });
    expect(screen.queryByTitle("Store: alpha")).toBeNull();
  });

  it("asks for the tree, the blocked tasks and the canceled ones without a store", () => {
    const { watchTree, watchList } = setup();
    render(TreeView, { view: all, query: "kind:task -status:canceled" });
    expect(watchTree).toHaveBeenCalledWith({
      q: "kind:task -status:canceled",
      tree: true,
      store: undefined,
    });
    expect(watchList).toHaveBeenCalledWith({ q: "is:blocked", tree: false, store: undefined });
    expect(watchList).toHaveBeenCalledWith({
      q: "kind:task status:canceled",
      tree: false,
      store: undefined,
    });
    expect(screen.getByText("4 件 · 中止 2 件を除外")).toBeInTheDocument();
  });

  it("keeps no filter of its own when the query comes from outside", async () => {
    const { watchTree } = setup();
    const onquery = vi.fn();
    render(TreeView, { view: all, query: "assignee:@me", onquery });
    expect(watchTree).toHaveBeenCalledWith({ q: "assignee:@me", tree: true, store: undefined });
    const input = screen.getByRole("textbox", { name: /フィルタ/ });
    expect(input).toHaveValue("assignee:@me");
    await fireEvent.input(input, { target: { value: "kind:epic" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(onquery).toHaveBeenCalledWith("kind:epic");
    // The owner has not changed the query: the tree is still asked for the old one.
    expect(watchTree).not.toHaveBeenCalledWith(expect.objectContaining({ q: "kind:epic" }));
  });

  it("takes an empty query from outside as it is, not as the default filter", () => {
    const { watchTree, watchList } = setup();
    render(TreeView, { view: all, query: "" });
    expect(watchTree).toHaveBeenCalledWith({ q: "", tree: true, store: undefined });
    expect(watchTree).not.toHaveBeenCalledWith(expect.objectContaining({ q: "-status:canceled" }));
    expect(watchList).not.toHaveBeenCalledWith(
      expect.objectContaining({ q: expect.stringContaining("status:canceled") }),
    );
    expect(screen.getByText("4 件")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: /フィルタ/ })).toHaveValue("");
  });

  it("shows the line it is given at the foot, else the hh ls of the filter", () => {
    setup();
    const { unmount } = render(TreeView, { view: all, query: "kind:task" });
    expect(screen.getByText("$ hh ls --tree kind:task")).toBeInTheDocument();
    unmount();
    render(TreeView, { view: all, query: "kind:task", cli: "hh view show 今週 --store alpha" });
    expect(screen.getByText("$ hh view show 今週 --store alpha")).toBeInTheDocument();
    expect(screen.queryByText("$ hh ls --tree kind:task")).toBeNull();
  });

  it("renders the actions it is given at the filter bar", () => {
    setup();
    const actions = createRawSnippet(() => ({
      render: () => `<button type="button">ビューを保存</button>`,
    }));
    render(TreeView, { view: all, query: "", actions });
    expect(screen.getByRole("button", { name: "ビューを保存" })).toBeInTheDocument();
  });

  it("adds a task below the selected row, in the store and workspace of that row", async () => {
    const { api } = setup();
    api.POST.mockResolvedValue(
      ok({ task: task("N1", "task", null, { store: "beta", workspace: wsOf.beta }), commit: "c1" }),
    );
    render(TreeView, { view: all, query: "" });
    const b1 = treeitem("beta one");
    await fireEvent.click(b1);
    await fireEvent.keyDown(b1, { key: "Enter" });
    expect(screen.getByText("→ beta / beta-ws · 種類: タスク")).toBeInTheDocument();
    const input = await screen.findByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）");
    await fireEvent.input(input, { target: { value: "新規" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    await waitFor(() =>
      expect(api.POST).toHaveBeenCalledWith("/api/tasks", {
        params: { query: { store: "beta" } },
        body: { title: "新規", kind: "task", workspace: "BBB", after: "B1" },
      }),
    );
  });

  it("opens the create dialog from the input at the foot", async () => {
    setup();
    render(TreeView, { view: all, query: "" });
    await fireEvent.click(screen.getByRole("button", { name: /タスクを追加/ }));
    expect(createState.request).toEqual({ kind: "task" });
    expect(screen.queryByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）")).toBeNull();
  });

  it("opens the create dialog from the foot when there are no rows, too", async () => {
    setup({ roots: [] });
    render(TreeView, { view: all, query: "" });
    expect(screen.getByText("一致するタスクはありません")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "フィルタを解除" })).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: /タスクを追加/ }));
    expect(createState.request).toEqual({ kind: "task" });
  });

  it("clears the filter from the empty state through the owner of the query", async () => {
    setup({ roots: [] });
    const onquery = vi.fn();
    render(TreeView, { view: all, query: "kind:nothing", onquery });
    await fireEvent.click(screen.getByRole("button", { name: "フィルタを解除" }));
    expect(onquery).toHaveBeenCalledWith("");
  });

  it("reads the keys from the schema of the store of the row", async () => {
    const { api } = setup({ betaSchema: "nodone" });
    api.PATCH.mockResolvedValue(
      ok({ task: task("A2", "task", "A1", { store: "alpha", status: "done" }), commit: "c1" }),
    );
    render(TreeView, { view: all, query: "" });
    const a2 = treeitem("alpha two");
    await fireEvent.click(a2);
    await fireEvent.keyDown(a2, { key: " " });
    await waitFor(() =>
      expect(api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
        params: { path: { id: "A2" }, query: { store: "alpha" } },
        body: { status: "done" },
      }),
    );
    // The schema of beta has no status to complete with: the key says so, in the words of beta.
    const b1 = treeitem("beta one");
    await fireEvent.click(b1);
    await fireEvent.keyDown(b1, { key: " " });
    await waitFor(() => expect(h.toast).toHaveBeenCalledTimes(1));
    expect(h.toast.mock.calls[0]?.[0].message).toContain("beta の schema.yaml");
    expect(api.PATCH).toHaveBeenCalledTimes(1);
  });

  it("holds a key for the schema of its own store, not of another", async () => {
    const { api, loadBeta, load } = setup({ betaSchema: "none" });
    api.PATCH.mockResolvedValue(
      ok({ task: task("B1", "task", null, { store: "beta", status: "done" }), commit: "c1" }),
    );
    render(TreeView, { view: all, query: "" });
    expect(load).toHaveBeenCalledWith("alpha");
    expect(load).toHaveBeenCalledWith("beta");
    const b1 = treeitem("beta one");
    await fireEvent.click(b1);
    await fireEvent.keyDown(b1, { key: " " });
    // alpha's schema is read, beta's is not: the key waits.
    expect(api.PATCH).not.toHaveBeenCalled();
    loadBeta();
    await waitFor(() =>
      expect(api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
        params: { path: { id: "B1" }, query: { store: "beta" } },
        body: { status: "done" },
      }),
    );
  });

  it("asks for the schemas again only when the stores of the rows change", async () => {
    const { load } = setup();
    render(TreeView, { view: all, query: "" });
    expect(load).toHaveBeenCalledTimes(2);
    await fireEvent.click(treeitem("alpha one"));
    await fireEvent.keyDown(treeitem("alpha one"), { key: "ArrowDown" });
    expect(load).toHaveBeenCalledTimes(2);
  });

  it("refuses to drop a task under a task of another store, and says why", async () => {
    const { api } = setup();
    render(TreeView, { view: all, query: "" });
    row("A1").onOver("child", "B1");
    expect(await screen.findByText("ここには置けません")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("別の Store のタスクの下には置けません");
    row("A1").onOver(null, null);
    row("A1").onDrop("B1", "child");
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
    expect(h.toast.mock.calls[0]?.[0].message).toContain("別の Store のタスクの下には置けません");
    expect(api.POST).not.toHaveBeenCalled();
  });

  it("refuses to drop a task between the tasks of another store", async () => {
    const { api } = setup();
    render(TreeView, { view: all, query: "" });
    row("B2").onDrop("A1", "before");
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
    expect(h.toast.mock.calls[0]?.[0].message).toContain("別の Store の並びには置けません");
    expect(api.POST).not.toHaveBeenCalled();
  });

  it("moves a task within its store as it does in a view of one", async () => {
    const { api } = setup();
    api.POST.mockResolvedValue(
      ok({ task: task("B2", "task", null, { store: "beta" }), commit: "c1" }),
    );
    render(TreeView, { view: all, query: "" });
    row("B1").onDrop("B2", "before");
    await waitFor(() =>
      expect(api.POST).toHaveBeenCalledWith("/api/tasks/{id}/move", {
        params: { path: { id: "B2" }, query: { store: "beta" } },
        body: { root: true, before: "B1" },
      }),
    );
  });
});
```

- [ ] **Step 7: 失敗を確かめる**

```bash
cd web && pnpm vitest run app/tree/TreeView.stores.test.ts
```

期待: ほぼ全件 FAIL（`TreeView` が `view.kind === "all"` を知らず、`query`・`onquery`・`cli`・`actions` を受け取らない）。

- [ ] **Step 8: `TreeView.svelte` を直す**

`web/app/tree/TreeView.svelte` を、次の順に置き換える。「置き換え前」は今のファイルにある文字列のまま（一意）、「置き換え後」が新しい文字列。

(8-1) import。`svelte` の import:

```ts
  import { onDestroy, tick, untrack } from "svelte";
```
→
```ts
  import { onDestroy, type Snippet, tick, untrack } from "svelte";
```

`../keys` の import の次に3行足す（`../router` の前）:

```ts
  import { kbdLabel, modKey } from "../keys";
  import ListBar from "../list/ListBar.svelte";
  import type { ListScope } from "../list/scope";
  import { useListScope } from "../list/scope.svelte";
  import { onLinkClick, router } from "../router";
```

`FilterBar` の import と `./filter` の import の2行:

```ts
  import FilterBar from "./FilterBar.svelte";
  import { baseQuery, combine, hasTerm, NO_CANCELED, type QueryBase, withoutTerm } from "./filter";
```
→
```ts
  import { combine, hasTerm, NO_CANCELED, withoutTerm } from "./filter";
```

`./placement` の import の次:

```ts
  import { checkParent, kindLabel, pickKind } from "./placement";
  import { orderRootsByStore, storesOf } from "./stores-order";
  import TreeRow from "./TreeRow.svelte";
```

(8-2) props。

```ts
  type Scope =
    | { kind: "workspace"; store: string; ulid: string }
    | { kind: "project"; ulid: string };
  // The prop is not called scope: Biome reads that name as the HTML attribute.
  let { view: scope }: { view: Scope } = $props();
```
→
```ts
  type Props = {
    view: ListScope;
    // The filter, when the screen that holds this view owns it (a saved view, the query screen).
    // Without it the view keeps the filter itself, from -status:canceled.
    query?: string;
    onquery?: (q: string) => void;
    // The line at the foot, in place of the hh ls of the filter.
    cli?: string;
    // At the right of the filter bar.
    actions?: Snippet;
  };
  // The prop is not called scope: Biome reads that name as the HTML attribute.
  let { view: scope, query, onquery, cli: cliLine, actions }: Props = $props();
```

(8-3) 「what the view shows」から `let applied = $state<string>(NO_CANCELED);` までを丸ごと置き換える。置き換え前は `// --- what the view shows ---` の行から `let applied = $state<string>(NO_CANCELED);` の行まで（`project`・`store`・`workspace`・`schema`・`viewRoot`・`workspaceMissing`・`base`・`apiBase`・`cliBase`・`detail`・`projectError`・`notFound`・`waiting`・`schemaFailed` と、スキーマの読み込みの2つの `$effect`、`// --- the query ---` の見出しとコメント）。置き換え後:

```ts
  // --- what the view shows -------------------------------------------------
  // Where the view is (a workspace, a project, or every store) is resolved once, by useListScope.
  const ls = useListScope(() => scope);
  const all = $derived(scope.kind === "all");
  const viewRoot = $derived(scope.kind === "project" ? scope.ulid : null);
  // A view over every store lists the tasks of several stores, so each row has the schema of its
  // own store: the kinds, the statuses and the keys are read from that one, never from a single schema.
  function schemaOf(store: string | undefined) {
    schemaRev.current;
    return store ? c.schema.get(store) : undefined;
  }
  function toneOf(store: string) {
    storesRev.current;
    return c.stores.tone(store);
  }
  function workspaceOf(store: string | undefined, ulid: string | undefined) {
    storesRev.current;
    return store && ulid ? c.stores.workspace(store, ulid) : undefined;
  }
  // The stores in the order of GET /api/stores: the order a view over every store lists them in.
  const storeOrder = $derived.by(() => {
    storesRev.current;
    return all ? c.stores.stores().map((s) => s.name) : [];
  });
  // Each row shows its store when the view can hold the tasks of more than one.
  const multi = $derived(all && storeOrder.length >= 2);

  // Keys that need the schema, pressed before it is read, wait here with the store of the task they
  // act on, and run once the schema of that store is read, in the order pressed. A store whose
  // schema could not be read is in failed; failed is read only while the schema is missing.
  type Waiting = { key: KeyInput; store: string };
  let waiting: Waiting[] = [];
  const failed = new Set<string>();
  // The stores of the rows. A child is in the store of its parent, so the roots hold them all. The
  // value is a string, so the schemas are asked for again only when the set of stores changes.
  const storesKey = $derived.by(() => {
    cacheRev.current;
    return (ls.store ? [ls.store] : storesOf(roots, taskOf)).join("\n");
  });
  $effect(() => {
    for (const store of storesKey === "" ? [] : storesKey.split("\n")) {
      failed.delete(store);
      c.schema.load(store).catch((err) => {
        failed.add(store);
        if (waiting.some((w) => w.store === store)) toast({ message: errorMessage(err) });
        waiting = waiting.filter((w) => w.store !== store);
      });
    }
  });
  $effect(() => {
    schemaRev.current;
    if (waiting.length === 0) return;
    // The keys go in order: one that waits for a schema holds back the ones after it.
    const n = waiting.findIndex((w) => !c.schema.get(w.store));
    const keys = n < 0 ? waiting : waiting.slice(0, n);
    if (keys.length === 0) return;
    waiting = n < 0 ? [] : waiting.slice(n);
    void (async () => {
      for (const w of keys) {
        await dispatch(w.key);
      }
    })();
  });
  // The keys act on the selected task, so on the schema of its store.
  const keyStore = $derived.by(() => {
    cacheRev.current;
    const selected = selection.ulid ? taskOf(selection.ulid) : undefined;
    return selected?.store ?? ls.store;
  });
  const keySchema = $derived(schemaOf(keyStore));

  // --- the query -----------------------------------------------------------
  // The default filter leaves the canceled tasks out; clearing the field shows them.
  let own = $state<string>(NO_CANCELED);
  const applied = $derived(query ?? own);
  function setApplied(q: string) {
    if (query === undefined) own = q;
    onquery?.(q);
  }
```

(8-4) ツリー・ブロック・中止の3つの `$effect` と `roots`・`loading`。置き換え前は `$effect(() => {\n    if (!apiBase || !store) {\n      tree = null;` から `loading` の `return base === null || (tree?.loading() ?? true);\n  });` まで。置き換え後:

```ts
  // A view over every store asks without a store: the answer lists the tasks of all of them.
  $effect(() => {
    if (!ls.ready) {
      tree = null;
      return;
    }
    const h = c.cache.watchTree({ q: combine(ls.apiBase, applied), tree: true, store: ls.store });
    tree = h;
    return () => h.dispose();
  });
  $effect(() => {
    if (!ls.ready) {
      blockedList = null;
      return;
    }
    const h = c.cache.watchList({
      q: combine(ls.apiBase, "is:blocked"),
      tree: false,
      store: ls.store,
    });
    blockedList = h;
    return () => h.dispose();
  });
  $effect(() => {
    if (!ls.ready || !hasTerm(applied, NO_CANCELED)) {
      canceledList = null;
      return;
    }
    const q = combine(ls.apiBase, `${withoutTerm(applied, NO_CANCELED)} status:canceled`);
    const h = c.cache.watchList({ q, tree: false, store: ls.store });
    canceledList = h;
    return () => h.dispose();
  });

  // The roots of a view over every store come in one answer. The order by store is derived from the
  // tree, the tasks and the stores as they are each time this is computed, and kept nowhere: when
  // any of them changes later, the order is made again, so no moment of arrival can leave it wrong.
  const roots = $derived.by(() => {
    cacheRev.current;
    const r = tree?.roots() ?? [];
    return all ? orderRootsByStore(r, storeOrder, taskOf) : r;
  });
  const loading = $derived.by(() => {
    cacheRev.current;
    return !ls.ready || (tree?.loading() ?? true);
  });
```

(8-5) `Adding` の型:

```ts
  // is shown, and moves below the new task once that is.
  type Adding = {
    after: string | null;
    parent: string | null;
    depth: number;
    trail?: string[];
  };
```
→
```ts
  // is shown, and moves below the new task once that is. store and workspace (its ULID) are where
  // the task is made: those of the row it goes below, or of the view for the input at the foot.
  type Adding = {
    after: string | null;
    parent: string | null;
    depth: number;
    store: string | undefined;
    workspace: string | undefined;
    trail?: string[];
  };
```

(8-6) キー。

```ts
    return keyAction(k, { rows, selected: selection.ulid, schema, viewRoot, taskOf });
```
→
```ts
    return keyAction(k, { rows, selected: selection.ulid, schema: keySchema, viewRoot, taskOf });
```

```ts
    if (!schema && needsSchema(k)) {
      e.preventDefault();
      if (schemaFailed) toast({ message: `${store} の schema.yaml を読み込めていません` });
      else waiting.push(k);
      return;
    }
```
→
```ts
    if (keyStore && !keySchema && needsSchema(k)) {
      e.preventDefault();
      if (failed.has(keyStore))
        toast({ message: `${keyStore} の schema.yaml を読み込めていません` });
      else waiting.push({ key: k, store: keyStore });
      return;
    }
```

```ts
        case "add-below": {
          const depth = rows.find((r) => r.ulid === a.after)?.depth ?? 0;
          adding = { after: a.after, parent: a.parent, depth };
          break;
        }
```
→
```ts
        case "add-below": {
          const row = rows.find((r) => r.ulid === a.after);
          // The task goes into the store and the workspace of the row it goes below.
          adding = {
            after: a.after,
            parent: a.parent,
            depth: row?.depth ?? 0,
            store: row?.task.store,
            workspace: row?.task.workspace.ulid,
          };
          break;
        }
```

(8-7) メニュー。

```ts
  const menuItems = $derived.by(() => {
    if (!menu || !menuTask || !schema) return [];
```
→
```ts
  const menuItems = $derived.by(() => {
    const menuSchema = schemaOf(menuTask?.store);
    if (!menu || !menuTask || !menuSchema) return [];
```

```ts
      return (schema.statuses ?? []).map((s) => ({ value: s.name, label: s.label || s.name }));
    }
    const parent = menuTask.parent ? taskOf(menuTask.parent) : undefined;
    return (schema.kinds ?? []).map((k) => {
      const why = checkParent(schema, { ...menuTask, kind: k.name }, parent ?? null);
```
→
```ts
      return (menuSchema.statuses ?? []).map((s) => ({ value: s.name, label: s.label || s.name }));
    }
    const parent = menuTask.parent ? taskOf(menuTask.parent) : undefined;
    return (menuSchema.kinds ?? []).map((k) => {
      const why = checkParent(menuSchema, { ...menuTask, kind: k.name }, parent ?? null);
```

(8-8) 追加。`// --- adding ---` の見出しの次から `submitAdd` の `const kind = pickKind(schema, parent?.kind ?? null);` の直前までを置き換える。置き換え前:

```ts
  const addKind = $derived.by(() => {
    if (!schema) return null;
    const parentId = adding ? adding.parent : viewRoot;
    const parent = parentId ? taskOf(parentId) : undefined;
    return pickKind(schema, parent?.kind ?? null);
  });
  const whereLabel = $derived(
    scope.kind === "project" && project
      ? `${project.workspace.name} / ${project.title}`
      : (workspace?.name ?? ""),
  );

  function startAdd(a: Adding) {
    adding = a;
  }

  // submitAdd says whether the task was made: the title typed stays when it was not.
  async function submitAdd(title: string): Promise<boolean> {
    const a = adding;
    if (!a || !store || !workspace || !schema) return false;
    const parent = a.parent ? taskOf(a.parent) : undefined;
```
置き換え後:

```ts
  // Where a task made now would go: the row it goes below, else the view's own workspace.
  const addStore = $derived(adding ? adding.store : ls.store);
  const addWorkspace = $derived(
    adding ? workspaceOf(adding.store, adding.workspace) : ls.workspace,
  );
  const addSchema = $derived(schemaOf(addStore));
  const addKind = $derived.by(() => {
    if (!addSchema) return null;
    const parentId = adding ? adding.parent : viewRoot;
    const parent = parentId ? taskOf(parentId) : undefined;
    return pickKind(addSchema, parent?.kind ?? null);
  });
  const whereLabel = $derived.by(() => {
    if (all) return addWorkspace ? `${addWorkspace.store} / ${addWorkspace.name}` : "";
    return scope.kind === "project" && ls.project
      ? `${ls.project.workspace.name} / ${ls.project.title}`
      : (ls.workspace?.name ?? "");
  });

  function startAdd(a: Adding) {
    adding = a;
  }
  // The input at the foot: a view over every store has no workspace of its own to make a task in, so
  // the create dialog, which asks for one, opens instead.
  function startFootAdd() {
    if (all) openCreate({ kind: "task" });
    else
      startAdd({
        after: null,
        parent: viewRoot,
        depth: 0,
        store: ls.store,
        workspace: ls.workspace?.ulid,
      });
  }

  // submitAdd says whether the task was made: the title typed stays when it was not.
  async function submitAdd(title: string): Promise<boolean> {
    const a = adding;
    const store = a?.store;
    const workspace = workspaceOf(store, a?.workspace);
    const schema = schemaOf(store);
    if (!a || !store || !workspace || !schema) return false;
    const parent = a.parent ? taskOf(a.parent) : undefined;
```

`submitAdd` の中の続けて入力する入力欄の移動:

```ts
      adding = { after: created.ulid, parent: a.parent, depth: a.depth, trail };
```
→
```ts
      adding = {
        after: created.ulid,
        parent: a.parent,
        depth: a.depth,
        store: a.store,
        workspace: a.workspace,
        trail,
      };
```

(8-9) ドロップ。

```ts
    return dropDecision({ dragged, target, place, schema, taskOf });
```
→
```ts
    return dropDecision({ dragged, target, place, schema: schemaOf(dragged.store), taskOf });
```

(8-10) 画面の読み取り。

```ts
  const cli = $derived(
    cliLs(combine(cliBase, applied), true, scope.kind === "workspace" ? store : undefined),
  );
```
→
```ts
  const footCli = $derived(
    cliLine ??
      cliLs(combine(ls.cliBase, applied), true, scope.kind === "workspace" ? ls.store : undefined),
  );
```

```ts
  const crumbTitle = $derived(
    scope.kind === "project" ? (project?.title ?? "") : (workspace?.name ?? ""),
  );
  // Spread, because Biome reads a prop called scope as the HTML attribute.
  const emptyProps = $derived({ scope: scope.kind, name: crumbTitle });
  const notFoundProps = $derived({
    scope: scope.kind,
    name: scope.kind === "workspace" ? scope.ulid : "",
  });
```
→
```ts
  const crumbTitle = $derived(
    scope.kind === "project" ? (ls.project?.title ?? "") : (ls.workspace?.name ?? ""),
  );
  // Spread, because Biome reads a prop called scope as the HTML attribute. A view over every store
  // is neither: it is never empty with a create button, and never not found.
  const emptyScope = $derived<"project" | "workspace">(
    scope.kind === "project" ? "project" : "workspace",
  );
  const emptyProps = $derived({ scope: emptyScope, name: crumbTitle });
  const notFoundProps = $derived({
    scope: emptyScope,
    name: scope.kind === "workspace" ? scope.ulid : "",
  });
```

(8-11) マークアップ。プロジェクトの見出しの `store`・`workspace`・`project` を `ls.store`・`ls.workspace`・`ls.project` に:

```svelte
        {#if store}
          <StoreBadge {store} tone={c.stores.tone(store)} />
        {/if}
        {#if workspace}
          {@const wsHref = router.href({
            name: "workspace",
            store: workspace.store,
            ulid: workspace.ulid,
            tab: "overview",
          })}
          <a class="ws" href={wsHref} onclick={(e) => onLinkClick(e, wsHref)}>
            <Icon name="workspace" size={16} />{workspace.name}
          </a>
```
→
```svelte
        {#if ls.store}
          <StoreBadge store={ls.store} tone={c.stores.tone(ls.store)} />
        {/if}
        {#if ls.workspace}
          {@const wsHref = router.href({
            name: "workspace",
            store: ls.workspace.store,
            ulid: ls.workspace.ulid,
            tab: "overview",
          })}
          <a class="ws" href={wsHref} onclick={(e) => onLinkClick(e, wsHref)}>
            <Icon name="workspace" size={16} />{ls.workspace.name}
          </a>
```

```svelte
        {#if project}
          <span class="id">{project.id}</span>
        {/if}
```
→
```svelte
        {#if ls.project}
          <span class="id">{ls.project.id}</span>
        {/if}
```

フィルタの段:

```svelte
  <FilterBar
    base={cliBase}
    {applied}
    count={countLabel(matched, canceledCount)}
    {cli}
    error={treeError}
    onchange={(q) => (applied = q)}
  />
```
→
```svelte
  <ListBar
    base={ls.cliBase}
    {applied}
    count={countLabel(matched, canceledCount)}
    cli={footCli}
    error={treeError}
    onchange={setApplied}
    {actions}
  />
```

```svelte
      aria-label="{crumbTitle} のタスク"
```
→
```svelte
      aria-label={all ? "タスク" : `${crumbTitle} のタスク`}
```

行:

```svelte
            row={item.row}
            {schema}
            selected={selection.ulid === item.row.ulid}
```
→
```svelte
            row={item.row}
            schema={schemaOf(item.row.task.store)}
            storeTone={multi ? toneOf(item.row.task.store) : undefined}
            selected={selection.ulid === item.row.ulid}
```

```svelte
            onmore={schema ? () => (menu = { which: "row", ulid: item.row.ulid }) : undefined}
```
→
```svelte
            onmore={schemaOf(item.row.task.store)
              ? () => (menu = { which: "row", ulid: item.row.ulid })
              : undefined}
```

入力行:

```svelte
            kindLabel={schema && addKind ? kindLabel(schema, addKind) : ""}
            ready={!!(store && workspace && schema)}
```
→
```svelte
            kindLabel={addSchema && addKind ? kindLabel(addSchema, addKind) : ""}
            ready={!!(addStore && addWorkspace && addSchema)}
```

空の状態:

```svelte
      {#if notFound}
```
→
```svelte
      {#if ls.notFound}
```

```svelte
      {:else if projectError}
        <p class="note" role="alert">{errorMessage(projectError)}</p>
```
→
```svelte
      {:else if ls.error}
        <p class="note" role="alert">{errorMessage(ls.error)}</p>
```

```svelte
        {#if noTasks}
          <Empty
            variant="empty"
```
→
```svelte
        {#if noTasks && !all}
          <Empty
            variant="empty"
```

```svelte
            onClear={() => (applied = "")}
```
→
```svelte
            onClear={applied.trim() === "" ? undefined : () => setApplied("")}
```

足元の入力:

```svelte
            where={whereLabel}
            kindLabel={schema && addKind ? kindLabel(schema, addKind) : ""}
            onstart={() => startAdd({ after: null, parent: viewRoot, depth: 0 })}
```
→
```svelte
            where={all ? "" : whereLabel}
            kindLabel={addSchema && addKind ? kindLabel(addSchema, addKind) : ""}
            onstart={startFootAdd}
```

- [ ] **Step 9: 整形・型・テストを通す**

```bash
cd web && pnpm fix && pnpm check && pnpm exec biome ci --vcs-enabled=false . && pnpm vitest run app/tree
```

期待: `pnpm check` は警告 0、`biome ci` は指摘 0、`app/tree` は全件 PASS（`TreeView.test.ts`・`TreeView.dnd.test.ts` の既存の件が変わらず通ること。`view={{ kind: "workspace", … }}`・`{ kind: "project", … }` の呼び出しはそのまま動く）。`pnpm fix` が `TreeView.svelte` の折り返しを直すことがあるが、その結果を採用する。

- [ ] **Step 10: 全体の確認とコミット**

```bash
cd web && pnpm vitest run
```

期待: 全件 PASS。

```bash
git add web/app/tree
git commit -m "feat(web): ツリーを ListScope と外から渡すクエリに対応させ、Store をまたぐツリーを出す

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

**決めたこと（Task 3）**
- 全 Store のツリーの根の並びは、毎回の派生（`orderRootsByStore`）で作る。持たないので、ツリー・タスク・Store の一覧の到着の前後で狂わない。Store の一覧が読めるまでは並べ替えず、バッジも出さない
- 全 Store の画面で足元の「タスクを追加」は作成ダイアログ（ワークスペースの選択欄つき）を開く。行がないときも同じ。Enter の「下に追加」は選んだ行の Store とワークスペースに作る（`createBelow`）
- 全 Store の画面で行がないときは常に「一致するタスクはありません」（`nomatch`）。「フィルタを解除」はフィルタが空でないときだけ
- キー・メニュー・ドロップ・追加のスキーマはすべて行（またはドラッグしたタスク）の Store のもの。`Alt+↑↓`・`Tab` の兄弟は同じ Store の行だけ（Store の境で止まる）
- スキーマの読み込みは行の Store の集合が変わったときだけやり直す。読み込めなかった Store は Store ごとに覚え、スキーマが無いあいだだけ読む
- `combine` は base が空のとき先頭に空白を付けない（全 Store の画面のクエリは利用者の入力だけ）

### Task 4: カンバン（列とスイムレーン・ドラッグ・必須の項目のダイアログ）

概要 plan の細部 36〜42（カンバン）・58（読み込み中と空の状態のうちカンバン）と、この回の決定 D3・D4 を作る。画面の状態（クエリ・列とスイムレーンの設定）はこの部品の外（Task 6 の `ListTab`・`ViewPage`・`QueryPage`）が持ち、`KanbanView` は props で受けて `onquery`・`onsettings` で返す。この Task は 3 つに分かれる。(a) 純関数 — 列と行の組み立て `board.ts`、ドロップの判定 `drop.ts`、Pragmatic DnD の薄い包み `dnd.ts`。(b) 作成ダイアログに開始時のステータスを渡す口（細部 42）。(c) 画面 — `KanbanCard`・`RequiredDialog`・`KanbanView`。画面への入口（タブ・App の振り分け）は Task 7 が結線するので、この Task の画面はまだ App から開けない。

**前提**:
- Task 2 が入っている。この Task が使うのは `web/core/views.ts` の `KanbanSettings`、`web/app/list/scope.ts` の `ListScope`、`web/app/list/scope.svelte.ts` の `useListScope` の getter 7 つ（`ready`・`notFound`・`apiBase`・`cliBase`・`store`・`workspace`・`project`）、`web/app/list/ListBar.svelte`、`web/core/cli.ts` の `cliViewSave`。このコードは、これらを同じ名前と型の最小のスタブに置き換えた `web/` の複製で通した。`KanbanView.test.ts` は `useListScope` を `vi.mock` で差し替えるので、Task 2 の中身がスタブと違っても通る。
- Task 3（TreeView）とは独立。`web/app/tree/` で触るのは `actions.ts` の型 1 行だけ（`patchTask` の型の Step）。

**実物を読んで確かめたこと**（コードはこれに合わせてある）:
- `TaskView.project`（`Ref`: `{ ulid, id, title }`。最も近い kind が `project` の祖先。なければない）は 4e-1 で入っている（`web/core/api/schema.d.ts`）。スイムレーンの「プロジェクト」とスコープの引き当てに使う。
- `SchemaView.scoped` は `ScopedDef[]`（`where: "project:<ULID>"`、`statuses?: string[] | null`、`workflow?: { transitions, required }`）。サーバーの規則（`internal/schema/effective.go`）: `Statuses()` は scoped の `statuses` に並ぶものだけ（表示のための絞り込みで、書き込みの検査には使わない）、`CanTransition` は Store と scoped の**両方**が許すこと（`transitions` に `from` が載っていなければ制限なし、載っていて `to` がなければ不可。載っていて空なら出られない）、`Required(status)` は Store と scoped の和。`scoped` を引くのは「祖先のうち `where` に当たる最初のもの」。
- 必須の項目の「空」の判定（`internal/schema/validate.go` の `filled`・`check.go` の `builtinFilled`）: 組み込みで必須にできるのは `parent`・`assignees`・`watchers`・`labels`・`repos`・`blocked_by`・`sprint`・`releases`・`links`、それ以外はプロパティ（空文字・null・空の列が空。`0` と `false` は入っている）。`TaskView` が持つのは `assignees`・`labels`・`blocked_by`・`parent` と `props` だけ。`watchers`・`repos` は `ShowView`（`GET /api/tasks/{id}`）にしかなく、`sprint`・`releases`・`links` は API のどの応答にも出ない（フェーズ 6）。
- `sort:status` の順（`internal/query/sort.go` の `case "status"`。`web/core/status-order.ts` がこれを持つ）: カテゴリの順（`schema.Categories` = todo・doing・done・canceled）、同じなら**そのスキーマの `statuses` 全体での位置**、同じなら Store の順。D4 の列の並びはこれと同じにした。
- 一覧の既定の順は rank と ULID（`sortEntries`）。カードは届いた順のまま置けば「列の中の順は rank のまま」になる。
- `POST /api/tasks` は `status` を受ける（`CreateInputBody.status`）。`CreateTask.svelte` はステータスを `$state` で持ち、スキーマが読めたら「そのスキーマにないステータスなら todo の既定」に直す effect を持っていた。`openCreate` の `CreateRequest` は `status` を持たなかった。
- `tree/actions.ts` の `patchTask(d, task, body)` は `PATCH /api/tasks/{id}` を呼び `cache.applyWrite` する。本文の型だけが `{ status?, kind? }` に狭かった。
- `Empty.svelte` の `scope` は `"project" | "workspace" | "store"`。`FilterBar` の `error` は `errorMessage` で出る（クエリの誤りも）。`ui/Menu.svelte` は `anchor`・`items`・`selected` を取る。`detail/PropField.svelte`・`FieldInput.svelte`・`prop-rows.ts` の `propRows` は作成ダイアログと同じ入力部品で、`RequiredDialog` はそれを使う。
- Biome は `div` に `role="listitem"` と `tabindex`・`onclick` を付けると指摘する（`useSemanticElements`・`noNoninteractiveTabindex`）。カードは `<li>` とタイトルの `<button>`（`::after` でカード全体を覆う）にして、a11y の指摘を出さない作りにした。`props` という名前の変数は Svelte が `$props` のストア記法と取り違える（`RequiredDialog` の入力値は `values`）。

**Files:**
- Create: `web/core/status-order.ts`、`status-order.test.ts`（sort:status の順。列の順とテーブルの並びで共有）
- Create: `web/app/kanban/board.ts`、`board.test.ts`（列と行の組み立て）
- Create: `web/app/kanban/drop.ts`、`drop.test.ts`（遷移の制約・必須の項目の欠け・スイムレーンをまたぐドロップの判定）
- Create: `web/app/kanban/dnd.ts`、`dnd.test.ts`（Pragmatic DnD の包み）
- Create: `web/app/kanban/RequiredDialog.svelte`、`RequiredDialog.test.ts`
- Create: `web/app/kanban/KanbanCard.svelte`
- Create: `web/app/kanban/KanbanView.svelte`、`KanbanView.test.ts`
- Modify: `web/core/schema.ts`、`schema.test.ts`（`defaultStatusIn`）
- Modify: `web/app/create-state.svelte.ts`、`web/app/create/CreateTask.svelte`、`CreateTask.test.ts`（`CreateRequest.status`・`category`）
- Modify: `web/app/tree/actions.ts`（`patchTask` の本文の型）

**Interfaces:**
- Consumes:
  - Task 2: `KanbanSettings = { columns: "status" | "category"; swimlane: "none" | "assignee" | "project" }`（`web/core/views.ts`）、`ListScope`（`web/app/list/scope.ts`）、`useListScope(get: () => ListScope)` の `ready: boolean`・`notFound: boolean`・`apiBase: string`・`cliBase: string`・`store: string | undefined`・`workspace: WorkspaceEntry | undefined`・`project: TaskView | undefined`（`web/app/list/scope.svelte.ts`）、`ListBar`（props `{ base: string; applied: string; count: string; cli: string; error: unknown; onchange: (q: string) => void; controls?: Snippet; actions?: Snippet }`）、`cliViewSave(i: { name: Arg; store?: string; body: ViewSaveBody }): string`（`web/core/cli.ts`。`Arg` は `string | Placeholder`）
  - 4d・4e-2 の core: `TaskCache.watchList({ q, tree: false, store? })`（`ulids()`・`loading()`・`error()`・`dispose()`）・`task(ulid)`・`applyWrite`・`subscribe`、`SchemaCache.get/load/subscribe`、`StoreDirectory.stores()/tone(store)/subscribe`、`unwrap`・`Schemas["PatchInputBody"]`、`errorMessage`、`cliLs`・`cliSet`・`Placeholder`、`formatDate`、`propEditorOf`・`defaultStatus`
  - 4d・4e-2 の app: `core()`、`useSubscribe`、`openCreate`、`router.go/href`、`selection`、`toast`、`patchTask`（`tree/actions`）、`combine`・`hasTerm`・`withoutTerm`・`NO_CANCELED`（`tree/filter`）、`countLabel`（`tree/flatten`）、`openBlocker`・`problemLabel`・`propText`（`tree/marks`）、`CATEGORIES`・`CATEGORY_LABEL`・`categoryOf`・`Category`（`workspace/counts`）、`propRows`（`detail/prop-rows`）、`FieldInput`・`PropField`、`Dialog`・`Button`・`Menu`・`CliLine`・`Avatar`・`IdLabel`・`StoreBadge`、`Icon`・`KindIcon`・`StatusIcon`、`Empty`
- Produces:
  - `web/core/status-order.ts`（フレームワークに依存しない。Task 5 の `table/columns.ts` の `statusRank` もこれを使う）:
    - `STATUS_CATEGORIES = ["todo", "doing", "done", "canceled"] as const`
    - `type StatusPlace = { category: number; pos: number; store: number }`
    - `statusPlace(schema: SchemaView | undefined, status: string, storeAt: number): StatusPlace`（`storeAt` は `GET /api/stores` の順での Store の位置。負なら不明。スキーマにないステータス・読めていないスキーマ・知らないカテゴリは最後）
    - `compareStatusPlace(a: StatusPlace, b: StatusPlace): number`（カテゴリ → スキーマでの位置 → Store の順）
    - `statusRank(schema: SchemaView | undefined, status: string, storeAt: number): number`（`compareStatusPlace` と同じ順を1つの数にしたもの。スキーマのステータスが 1000 未満、Store が 1000 未満のあいだ同じ順）
  - `web/core/schema.ts`: `defaultStatusIn(s: SchemaView, category: string): string`（`defaultStatus` はこれを呼ぶ）
  - `web/app/create-state.svelte.ts`: `CreateRequest = { kind: "task" | "project"; parent?: string; status?: string; category?: string }`。`status` はそのステータスで、`category` はそのカテゴリの既定のステータスで作成ダイアログを開く（`status` が入れ先のワークスペースのスキーマになければ `category`、それもなければ todo の既定）
  - `web/app/tree/actions.ts`: `patchTask(d, task, body: Schemas["PatchInputBody"]): Promise<TaskView>`
  - `web/app/kanban/board.ts`:
    - `NO_LANE = "#none"`（担当者なし・プロジェクトなしの行のキー）
    - `type Column = { key: string; label: string; category: Category; status: string | null; unknown: boolean }`（`key` はステータス名、カテゴリ列ではカテゴリ。`status` はドロップしたカードが得るステータスで、カテゴリ列とスキーマにないステータスでは null。`unknown` は「どのスキーマにも載っていないがタスクがいるステータス」）
    - `type Lane = { key: string; label: string; cells: TaskView[][] }`（`key` はスイムレーンなしでは `""`、担当者ならその値、プロジェクトならその ULID、なしなら `NO_LANE`。`cells[i]` は `columns[i]` のカード）
    - `type Board = { columns: Column[]; lanes: Lane[] }`
    - `type BoardOptions = { mode: KanbanSettings["columns"]; swimlane: KanbanSettings["swimlane"]; schemas: ReadonlyMap<string, SchemaView>; scopedStatuses?: readonly string[]; storeOrder: readonly string[]; store?: string }`
    - `buildBoard(tasks: readonly TaskView[], o: BoardOptions): Board`、`columnKeyOf(task, mode, schema | undefined): string`、`categoryIn(schema | undefined, status): Category`
  - `web/app/kanban/drop.ts`: `scopedStatusesOf(schema, project: string | undefined): string[] | undefined`、`canTransition(schema, project, from, to): boolean`、`requiredFields(schema, project, status): string[]`、`type RequiredGap = { fill: string[]; elsewhere: string[] }`、`requiredGap(task: TaskView, names: readonly string[], schema: SchemaView, detail?: ShowView): RequiredGap`（`fill` は担当者・ラベル・入力部品のあるプロパティ、`elsewhere` は親・ブロック・ウォッチャー・リポジトリ・タスク参照・スプリント・リリース・リンクなど詳細でしか入力できないもの。ウォッチャーとリポジトリは `detail` があるときだけ判定する）、`type DropDecision = { ok: true; status: string; fill: string[]; elsewhere: string[] } | { ok: false; silent: boolean; title: string; reason: string }`、`type DropInput = { task; column: Column; sameLane: boolean; swimlane; mode; schema: SchemaView | undefined; detail?: ShowView }`、`decideDrop(i: DropInput): DropDecision`
  - `web/app/kanban/dnd.ts`: `type DragSource = { ulid: string; lane: string }`、`cardDnd(get)`・`cellDnd(get)`・`boardDragEnd(onEnd)`・`boardAutoScroll()`（どれも `Attachment<HTMLElement>`）
  - `web/app/kanban/RequiredDialog.svelte`: props `{ task: TaskView; status: string; fields: string[]; elsewhere: string[]; schema: SchemaView; onsubmit: (body: Schemas["PatchInputBody"]) => Promise<void>; onclose: () => void }`
  - `web/app/kanban/KanbanCard.svelte`: props `{ task; schema: SchemaView | undefined; blockedBy: string | undefined; tone: StoreTone | undefined; dragging: boolean; onopen: () => void; dnd: Attachment<HTMLElement> }`
  - `web/app/kanban/KanbanView.svelte`: props `{ view: ListScope; query: string; onquery: (q: string) => void; settings: KanbanSettings; onsettings: (s: KanbanSettings) => void; cli?: string; actions?: Snippet }`
  - E2E（Task 7）が使う目印: 盤面は `section[aria-label="カンバンの盤面"]`、セルは `ul[data-column="<ステータス名またはカテゴリ>"][data-lane="<行のキー>"]`（`aria-label` は「列名」または「列名 · 行名」）、カードは `li[data-ulid]` でタイトルのボタン（名前はタイトル）が詳細を開く、列の「＋」は `button[aria-label="<列名> にタスクを追加"]`、移せない理由は `role="status"` の帯、必須の項目のダイアログは `role="dialog"` で名前が「<ID> を<ステータス>にするには <項目> が必要です」、設定のメニューは `button` の名前が「列: …」「スイムレーン: …」。ドラッグは HTML5 の drag and drop なので、Playwright の `dragTo` で動く

---

---

- [ ] **Step 1: `defaultStatusIn` のテストを書く（失敗する）**

カンバンのカテゴリ列は「そのカテゴリの既定のステータス」へ移す。今の `defaultStatus` は todo と done だけなので、どのカテゴリでも引ける `defaultStatusIn` を足す。`web/core/schema.test.ts` の差分（実装は次の Step）:

`web/core/schema.test.ts` の差分:

```diff
--- a/web/core/schema.test.ts
+++ b/web/core/schema.test.ts
@@ -3,6 +3,7 @@
 import {
   canParent,
   defaultStatus,
+  defaultStatusIn,
   parentRule,
   propEditorOf,
   SchemaCache,
@@ -109,6 +110,27 @@
   });
 });
 
+describe("defaultStatusIn", () => {
+  const s: SchemaView = {
+    ...schema,
+    statuses: [
+      { name: "todo", category: "todo", default: true },
+      { name: "wip", category: "doing" },
+      { name: "review", category: "doing", default: true },
+      { name: "done", category: "done", default: true },
+    ],
+  };
+
+  it("is the default of any category, else its first status", () => {
+    expect(defaultStatusIn(s, "doing")).toBe("review");
+    expect(defaultStatusIn(s, "todo")).toBe("todo");
+  });
+
+  it("is empty for a category the schema has no status in", () => {
+    expect(defaultStatusIn(s, "canceled")).toBe("");
+  });
+});
+
 describe("propEditorOf", () => {
   it("maps the types of the schema", () => {
     expect(propEditorOf(schema, "priority")).toBe("enum");
```

```bash
cd web && pnpm vitest run core/schema.test.ts
```

Expected: FAIL（`defaultStatusIn is not a function`、または import できない）。

- [ ] **Step 2: `defaultStatusIn` を実装する**

`web/core/schema.ts` の差分:

```diff
--- a/web/core/schema.ts
+++ b/web/core/schema.ts
@@ -113,12 +113,17 @@
   return `${label}を置ける親は${listJa(names)}だけです`;
 }
 
-// defaultStatus is the status a task gets when it moves to the todo or the
-// done category: the one the schema marks default, else the first of the
-// category. It is "" when the category has none.
-export function defaultStatus(s: SchemaView, category: "todo" | "done"): string {
+// defaultStatusIn is the status a task gets when it moves into a category
+// (todo, doing, done or canceled): the one the schema marks default, else the
+// first of the category. It is "" when the category has none.
+export function defaultStatusIn(s: SchemaView, category: string): string {
   const of = (s.statuses ?? []).filter((x) => x.category === category);
   return (of.find((x) => x.default) ?? of[0])?.name ?? "";
+}
+
+// defaultStatus is defaultStatusIn for the todo and the done category.
+export function defaultStatus(s: SchemaView, category: "todo" | "done"): string {
+  return defaultStatusIn(s, category);
 }
 
 export type PropEditor =
```

```bash
cd web && pnpm vitest run core/schema.test.ts
```

Expected: PASS（既存の `defaultStatus` のテストも通る）。

- [ ] **Step 3: sort:status の順 `status-order.ts` のテストを書く（失敗する）と実装する**

カンバンの列の順（D4）とテーブルの並び替え・グループ（Task 5）は、どちらも `sort:status`（`internal/query/sort.go`）の順を使う。同じ計算を2か所に書かないよう、`web/core/status-order.ts` に1つ置き、`board.ts` も Task 5 の `table/columns.ts` の `statusRank` もこれを使う。順は、カテゴリ（todo・doing・done・canceled）、同じならそのスキーマの `statuses` 全体での位置、同じなら Store の順。スキーマにないステータス・知らないカテゴリは最後。

`web/core/status-order.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { SchemaView } from "./schema";
import { compareStatusPlace, statusPlace, statusRank } from "./status-order";

const schema = {
  statuses: [
    { name: "backlog", category: "todo" },
    { name: "wip", category: "doing" },
    { name: "done", category: "done" },
    { name: "dropped", category: "canceled" },
    { name: "odd", category: "weird" },
  ],
} as unknown as SchemaView;

describe("statusPlace", () => {
  it("is the category, the place in the schema and the place of the store", () => {
    expect(statusPlace(schema, "wip", 2)).toEqual({ category: 1, pos: 1, store: 2 });
  });

  it("puts a status the schema lacks, a schema not read and a category not known after the rest", () => {
    const last = Number.MAX_SAFE_INTEGER;
    expect(statusPlace(schema, "gone", 0)).toEqual({ category: 4, pos: last, store: 0 });
    expect(statusPlace(undefined, "wip", 0)).toEqual({ category: 4, pos: last, store: 0 });
    expect(statusPlace(schema, "odd", 0).category).toBe(4);
    expect(statusPlace(schema, "wip", -1).store).toBe(last);
  });
});

describe("compareStatusPlace", () => {
  it("compares the category first, then the place in the schema, then the store", () => {
    const at = (s: string, store: number) => statusPlace(schema, s, store);
    expect(compareStatusPlace(at("backlog", 5), at("wip", 0))).toBeLessThan(0);
    expect(compareStatusPlace(at("done", 0), at("wip", 0))).toBeGreaterThan(0);
    expect(compareStatusPlace(at("wip", 0), at("wip", 1))).toBeLessThan(0);
    expect(compareStatusPlace(at("wip", 1), at("wip", 1))).toBe(0);
  });
});

describe("statusRank", () => {
  it("orders as the places do", () => {
    const rank = (s: string, store: number) => statusRank(schema, s, store);
    expect(rank("backlog", 3)).toBeLessThan(rank("wip", 0));
    expect(rank("wip", 0)).toBeLessThan(rank("wip", 1));
    expect(rank("dropped", 0)).toBeLessThan(rank("gone", 0));
  });
});
```

```bash
cd web && pnpm vitest run core/status-order.test.ts
```

Expected: FAIL（`Failed to resolve import "./status-order"`）。実装:

`web/core/status-order.ts`:

```ts
import type { SchemaView } from "./schema";

// The categories in the order sort:status puts them (internal/schema Categories).
export const STATUS_CATEGORIES = ["todo", "doing", "done", "canceled"] as const;

// Where a status stands in sort:status: the place of its category, then its place in the schema
// (the index in the whole list of statuses), then the place of its store.
export type StatusPlace = { category: number; pos: number; store: number };

const UNKNOWN = Number.MAX_SAFE_INTEGER;

// statusPlace is the place of a status in the schema of a store. storeAt is the index of the store
// in the order of GET /api/stores (negative: not known). A status the schema does not list, or one
// of a category it does not know, stands after all the others, as the server puts it.
export function statusPlace(
  schema: SchemaView | undefined,
  status: string,
  storeAt: number,
): StatusPlace {
  const statuses = schema?.statuses ?? [];
  const pos = statuses.findIndex((s) => s.name === status);
  const category = STATUS_CATEGORIES.indexOf(
    (pos >= 0 ? statuses[pos]?.category : "") as (typeof STATUS_CATEGORIES)[number],
  );
  return {
    category: category < 0 ? STATUS_CATEGORIES.length : category,
    pos: pos < 0 ? UNKNOWN : pos,
    store: storeAt < 0 ? UNKNOWN : storeAt,
  };
}

// compareStatusPlace orders two places: the category, then the place in the schema, then the store.
export function compareStatusPlace(a: StatusPlace, b: StatusPlace): number {
  return (
    a.category - b.category || (a.pos < b.pos ? -1 : a.pos > b.pos ? 1 : 0) || a.store - b.store
  );
}

// statusRank is a place as one number to compare, for a sort of many tasks: it orders as
// compareStatusPlace does while the schema has fewer than 1000 statuses and there are fewer than
// 1000 stores.
export function statusRank(
  schema: SchemaView | undefined,
  status: string,
  storeAt: number,
): number {
  const p = statusPlace(schema, status, storeAt);
  return p.category * 1_000_000 + Math.min(p.pos, 999) * 1_000 + Math.min(p.store, 999);
}
```

```bash
cd web && pnpm vitest run core/status-order.test.ts
```

Expected: PASS（4 件）。

- [ ] **Step 4: 列と行の組み立て `board.ts` のテストを書く（失敗する）**

細部 36・37 と D4 を純関数で確かめる。ステータスの和の並びは `sort:status`（`internal/query/sort.go`）と同じ: カテゴリ、同じなら各スキーマでの位置、同じなら Store の順。名前が同じステータスは1列で、表示名は最初の Store のもの。順の計算は `status-order.ts` を使う。

`web/app/kanban/board.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { SchemaView } from "../../core/schema";
import { task } from "../tree/fixtures";
import { buildBoard, columnKeyOf, NO_LANE } from "./board";

const sc = (statuses: { name: string; category: string; label?: string }[]) =>
  ({
    statuses,
    kinds: [],
    props: {},
    scoped: [],
    workflow: { transitions: {}, required: {} },
  }) as unknown as SchemaView;

// shared: backlog, todo, doing, review, done, canceled
const shared = sc([
  { name: "backlog", category: "todo", label: "バックログ" },
  { name: "todo", category: "todo", label: "未着手" },
  { name: "doing", category: "doing", label: "進行中" },
  { name: "review", category: "doing", label: "レビュー" },
  { name: "done", category: "done", label: "完了" },
  { name: "canceled", category: "canceled", label: "中止" },
]);
// work: todo, wip, shipped, done (its "todo" has another label)
const work = sc([
  { name: "todo", category: "todo", label: "ToDo" },
  { name: "wip", category: "doing" },
  { name: "shipped", category: "done" },
  { name: "done", category: "done" },
  { name: "dropped", category: "canceled" },
]);

const opts = (over: Partial<Parameters<typeof buildBoard>[1]> = {}) => ({
  mode: "status" as const,
  swimlane: "none" as const,
  schemas: new Map([["shared", shared]]),
  storeOrder: ["shared", "work"],
  ...over,
});
const keys = (b: ReturnType<typeof buildBoard>) => b.columns.map((c) => c.key);
const ulids = (list: { ulid: string }[]) => list.map((t) => t.ulid);

describe("buildBoard: status columns", () => {
  it("makes a column of every status of the store, in the order of the schema, but the canceled one has to have tasks", () => {
    const b = buildBoard([task("A", "task", null, { status: "todo" })], opts({ store: "shared" }));
    expect(keys(b)).toEqual(["backlog", "todo", "doing", "review", "done"]);
    expect(b.columns[1]).toMatchObject({
      label: "未着手",
      category: "todo",
      status: "todo",
      unknown: false,
    });
  });

  it("shows the canceled column when a task is in it", () => {
    const b = buildBoard([task("A", "task", null, { status: "canceled" })], opts());
    expect(keys(b)).toContain("canceled");
  });

  it("joins the statuses of the stores of the tasks: by category, then position, then store", () => {
    const tasks = [
      task("A", "task", null, { store: "shared", status: "todo" }),
      task("B", "task", null, { store: "work", status: "wip" }),
    ];
    const b = buildBoard(
      tasks,
      opts({
        schemas: new Map([
          ["shared", shared],
          ["work", work],
        ]),
      }),
    );
    // As sort:status does: the category, then the place in the schema (shared: backlog 0, todo 1,
    // doing 2, review 3, done 4; work: todo 0, wip 1, shipped 2), then the order of the stores.
    // A name two stores share is one column, placed by the first store.
    expect(keys(b)).toEqual(["backlog", "todo", "wip", "doing", "review", "shipped", "done"]);
    // the label is the first store's
    expect(b.columns.find((c) => c.key === "todo")?.label).toBe("未着手");
  });

  it("leaves out a store that has no task, unless it is the store of the screen", () => {
    const schemas = new Map([
      ["shared", shared],
      ["work", work],
    ]);
    const only = buildBoard([task("A", "task", null, { store: "shared" })], opts({ schemas }));
    expect(keys(only)).not.toContain("wip");
    const screen = buildBoard(
      [task("A", "task", null, { store: "shared" })],
      opts({ schemas, store: "work" }),
    );
    expect(keys(screen)).toContain("wip");
  });

  it("keeps to the statuses of the project, and never hides a task", () => {
    const tasks = [
      task("A", "task", null, { status: "todo" }),
      task("B", "task", null, { status: "review" }),
    ];
    const b = buildBoard(
      tasks,
      opts({ store: "shared", scopedStatuses: ["todo", "doing", "done"] }),
    );
    // review has a task, so it stays; backlog is out
    expect(keys(b)).toEqual(["todo", "doing", "review", "done"]);
  });

  it("adds a column for a status no schema lists, at the end", () => {
    const b = buildBoard([task("A", "task", null, { status: "gone" })], opts({ store: "shared" }));
    expect(keys(b).at(-1)).toBe("gone");
    expect(b.columns.at(-1)).toMatchObject({
      label: "gone",
      status: null,
      unknown: true,
      category: "todo",
    });
  });

  it("puts the tasks in the column of their status, in the order they come", () => {
    const tasks = [
      task("A", "task", null, { status: "todo" }),
      task("B", "task", null, { status: "doing" }),
      task("C", "task", null, { status: "todo" }),
    ];
    const b = buildBoard(tasks, opts());
    const lane = b.lanes[0];
    expect(lane?.key).toBe("");
    const at = (k: string) => keys(b).indexOf(k);
    expect(ulids(lane?.cells[at("todo")] ?? [])).toEqual(["A", "C"]);
    expect(ulids(lane?.cells[at("doing")] ?? [])).toEqual(["B"]);
  });
});

describe("buildBoard: category columns", () => {
  it("makes the three categories, and the canceled one when a task is in it", () => {
    const b = buildBoard([task("A", "task", null, { status: "wip", store: "work" })], {
      ...opts({ schemas: new Map([["work", work]]) }),
      mode: "category",
    });
    expect(keys(b)).toEqual(["todo", "doing", "done"]);
    expect(b.columns.map((c) => c.label)).toEqual(["未着手", "進行中", "完了"]);
    expect(b.columns.every((c) => c.status === null)).toBe(true);
    expect(ulids(b.lanes[0]?.cells[1] ?? [])).toEqual(["A"]);
    const c = buildBoard([task("A", "task", null, { status: "dropped", store: "work" })], {
      ...opts({ schemas: new Map([["work", work]]) }),
      mode: "category",
    });
    expect(keys(c)).toEqual(["todo", "doing", "done", "canceled"]);
  });

  it("puts a task of a store without a schema in todo", () => {
    const b = buildBoard([task("A", "task", null, { status: "wip", store: "work" })], {
      ...opts(),
      mode: "category",
    });
    expect(ulids(b.lanes[0]?.cells[0] ?? [])).toEqual(["A"]);
  });
});

describe("buildBoard: swimlanes", () => {
  it("puts a task with two assignees in the row of each, and the unassigned last", () => {
    const tasks = [
      task("A", "task", null, { assignees: ["gh/sato"] }),
      task("B", "task", null, { assignees: [] }),
      task("C", "task", null, { assignees: ["gh/koki", "gh/sato"] }),
    ];
    const b = buildBoard(tasks, opts({ swimlane: "assignee" }));
    expect(b.lanes.map((l) => l.key)).toEqual(["gh/koki", "gh/sato", NO_LANE]);
    expect(b.lanes.map((l) => l.label)).toEqual(["koki", "sato", "未割り当て"]);
    const todo = keys(b).indexOf("todo");
    expect(ulids(b.lanes[0]?.cells[todo] ?? [])).toEqual(["C"]);
    expect(ulids(b.lanes[1]?.cells[todo] ?? [])).toEqual(["A", "C"]);
    expect(ulids(b.lanes[2]?.cells[todo] ?? [])).toEqual(["B"]);
  });

  it("makes a row of each project in the order they come, and the tasks outside last", () => {
    const p2 = { ulid: "P2", id: "REL-20", title: "二つ目" };
    const p1 = { ulid: "P1", id: "REL-15", title: "一つ目" };
    const tasks = [
      task("A", "task", null, {}),
      task("B", "task", null, { project: p2 }),
      task("C", "task", null, { project: p1 }),
      task("D", "task", null, { project: p2 }),
    ];
    const b = buildBoard(tasks, opts({ swimlane: "project" }));
    expect(b.lanes.map((l) => l.key)).toEqual(["P2", "P1", NO_LANE]);
    expect(b.lanes.map((l) => l.label)).toEqual([
      "REL-20 二つ目",
      "REL-15 一つ目",
      "プロジェクトなし",
    ]);
    const todo = keys(b).indexOf("todo");
    expect(ulids(b.lanes[0]?.cells[todo] ?? [])).toEqual(["B", "D"]);
  });

  it("has one row without a name when there are no swimlanes, and none for no tasks", () => {
    const b = buildBoard([], opts({ swimlane: "assignee" }));
    expect(b.lanes).toEqual([]);
    const one = buildBoard([task("A", "task", null, {})], opts());
    expect(one.lanes).toHaveLength(1);
    expect(one.lanes[0]).toMatchObject({ key: "", label: "" });
  });
});

describe("columnKeyOf", () => {
  it("is the status, or the category of the status in the schema of the task", () => {
    const t = task("A", "task", null, { status: "wip", store: "work" });
    expect(columnKeyOf(t, "status", work)).toBe("wip");
    expect(columnKeyOf(t, "category", work)).toBe("doing");
    expect(columnKeyOf(t, "category", undefined)).toBe("todo");
  });
});
```

```bash
cd web && pnpm vitest run app/kanban/board.test.ts
```

Expected: FAIL（`Failed to resolve import "./board"`）。

- [ ] **Step 5: `board.ts` を実装する**

`web/app/kanban/board.ts`:

```ts
import type { TaskView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import { compareStatusPlace, type StatusPlace, statusPlace } from "../../core/status-order";
import type { KanbanSettings } from "../../core/views";
import { CATEGORIES, CATEGORY_LABEL, type Category, categoryOf } from "../workspace/counts";

// The key of the row that holds the tasks without an assignee (or outside every project).
export const NO_LANE = "#none";

export type Column = {
  // The status name; the category in the category mode.
  key: string;
  label: string;
  category: Category;
  // The status a card dropped here gets. null in the category mode (the schema of the task says
  // which status of the category) and for a status no schema lists.
  status: string | null;
  // A status the tasks have and no schema lists (or the schema of its store is not read).
  unknown: boolean;
};

export type Lane = {
  // "" when there are no swimlanes; the assignee; the ULID of the project; NO_LANE.
  key: string;
  label: string;
  // The tasks of each column, in the order of Board.columns.
  cells: TaskView[][];
};

export type Board = { columns: Column[]; lanes: Lane[] };

export type BoardOptions = {
  mode: KanbanSettings["columns"];
  swimlane: KanbanSettings["swimlane"];
  schemas: ReadonlyMap<string, SchemaView>;
  // The statuses a project shows (its scoped statuses); a task in another status keeps its column.
  scopedStatuses?: readonly string[];
  // The names of the stores in the order of GET /api/stores.
  storeOrder: readonly string[];
  // The store of a screen of one store: its statuses show even where it has no task.
  store?: string;
};

// categoryIn is the category of a status in a schema; todo when the schema is not read.
export function categoryIn(schema: SchemaView | undefined, status: string): Category {
  return schema ? categoryOf(schema, status) : "todo";
}

// columnKeyOf is the key of the column a task is in.
export function columnKeyOf(
  task: TaskView,
  mode: KanbanSettings["columns"],
  schema: SchemaView | undefined,
): string {
  return mode === "category" ? categoryIn(schema, task.status) : task.status;
}

type Def = { name: string; label: string; category: Category; place: StatusPlace };

const asCategory = (c: string): Category =>
  c === "doing" || c === "done" || c === "canceled" ? c : "todo";

// statusColumns joins the statuses of the stores of the tasks (and of the screen): one column for
// a name, labelled by the first store that has it, placed as sort:status places them: by category,
// then by the place in the schema, then by the order of the stores. A canceled status has a column
// only while a task is in it.
function statusColumns(tasks: readonly TaskView[], o: BoardOptions): Column[] {
  const inUse = new Set(tasks.map((t) => t.status));
  const stores = new Set(tasks.map((t) => t.store));
  if (o.store) stores.add(o.store);
  const order = [...o.storeOrder, ...[...stores].filter((s) => !o.storeOrder.includes(s))];
  const defs = new Map<string, Def>();
  order.forEach((store, storeAt) => {
    if (!stores.has(store)) return;
    const schema = o.schemas.get(store);
    for (const s of schema?.statuses ?? []) {
      if (defs.has(s.name)) continue;
      defs.set(s.name, {
        name: s.name,
        label: s.label || s.name,
        category: asCategory(s.category),
        place: statusPlace(schema, s.name, storeAt),
      });
    }
  });
  const shown = o.scopedStatuses ? new Set(o.scopedStatuses) : null;
  const kept = [...defs.values()].filter(
    (d) =>
      inUse.has(d.name) || (d.category !== "canceled" && (shown === null || shown.has(d.name))),
  );
  kept.sort((a, b) => compareStatusPlace(a.place, b.place));
  const columns: Column[] = kept.map((d) => ({
    key: d.name,
    label: d.label,
    category: d.category,
    status: d.name,
    unknown: false,
  }));
  const unknown = [...inUse].filter((s) => !defs.has(s)).sort();
  for (const s of unknown) {
    columns.push({ key: s, label: s, category: "todo", status: null, unknown: true });
  }
  return columns;
}

function categoryColumns(tasks: readonly TaskView[], o: BoardOptions): Column[] {
  const canceled = tasks.some((t) => categoryIn(o.schemas.get(t.store), t.status) === "canceled");
  return CATEGORIES.filter((c) => c !== "canceled" || canceled).map((c) => ({
    key: c,
    label: CATEGORY_LABEL[c],
    category: c,
    status: null,
    unknown: false,
  }));
}

// loginOf is the login of an assignee ("github.com/koki" is koki).
const loginOf = (assignee: string): string => assignee.slice(assignee.lastIndexOf("/") + 1);

type LaneDef = { key: string; label: string };

function laneKeysOf(t: TaskView, swimlane: BoardOptions["swimlane"]): string[] {
  if (swimlane === "assignee") {
    return t.assignees && t.assignees.length > 0 ? [...new Set(t.assignees)] : [NO_LANE];
  }
  if (swimlane === "project") return [t.project?.ulid ?? NO_LANE];
  return [""];
}

// lanesOf lists the rows: by name for the assignees, in the order they come for the projects, and
// the row for no assignee (no project) last.
function lanesOf(tasks: readonly TaskView[], swimlane: BoardOptions["swimlane"]): LaneDef[] {
  if (tasks.length === 0) return [];
  if (swimlane === "none") return [{ key: "", label: "" }];
  const found = new Map<string, string>();
  for (const t of tasks) {
    for (const key of laneKeysOf(t, swimlane)) {
      if (found.has(key)) continue;
      if (key === NO_LANE)
        found.set(key, swimlane === "assignee" ? "未割り当て" : "プロジェクトなし");
      else if (swimlane === "assignee") found.set(key, loginOf(key));
      else found.set(key, `${t.project?.id ?? ""} ${t.project?.title ?? ""}`.trim());
    }
  }
  const lanes = [...found].map(([key, label]) => ({ key, label }));
  if (swimlane === "assignee") lanes.sort((a, b) => a.key.localeCompare(b.key));
  return [...lanes.filter((l) => l.key !== NO_LANE), ...lanes.filter((l) => l.key === NO_LANE)];
}

// buildBoard puts the tasks in the columns and the rows of the board. The tasks of a cell keep the
// order they come in (the rank). A task with two assignees is in the row of each.
export function buildBoard(tasks: readonly TaskView[], o: BoardOptions): Board {
  const columns = o.mode === "category" ? categoryColumns(tasks, o) : statusColumns(tasks, o);
  const at = new Map(columns.map((c, i) => [c.key, i]));
  const lanes: Lane[] = lanesOf(tasks, o.swimlane).map((l) => ({
    ...l,
    cells: columns.map(() => []),
  }));
  const laneAt = new Map(lanes.map((l, i) => [l.key, i]));
  for (const t of tasks) {
    const col = at.get(columnKeyOf(t, o.mode, o.schemas.get(t.store)));
    if (col === undefined) continue;
    for (const key of laneKeysOf(t, o.swimlane)) {
      lanes[laneAt.get(key) ?? -1]?.cells[col]?.push(t);
    }
  }
  return { columns, lanes };
}
```

```bash
cd web && pnpm vitest run app/kanban/board.test.ts
```

Expected: PASS（13 件）。

- [ ] **Step 6: ドロップの判定 `drop.ts` のテストを書く（失敗する）**

細部 38・39 の判定を DOM なしで確かめる: スイムレーンをまたぐドロップの拒否、遷移の制約（Store と scoped の両方が許すこと）、必須の項目（Store と scoped の和）。移る先の必須の項目（Store と scoped の和）が欠けていれば、欠けているものを全部返す。担当者・ラベル・プロパティは `fill`（ダイアログで入力できる）、親・ブロック・ウォッチャー・リポジトリ・タスク参照・スプリント・リリース・リンクは `elsewhere`（詳細でしか入力できない）に分ける。ウォッチャーとリポジトリは一覧にないので、フルのタスク（`ShowView`）を渡したときだけ判定できる（渡さなければ欠けている扱い）。

`web/app/kanban/drop.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { ShowView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import { task } from "../tree/fixtures";
import type { Column } from "./board";
import { canTransition, decideDrop, requiredFields, requiredGap, scopedStatusesOf } from "./drop";

const schema = {
  statuses: [
    { name: "todo", category: "todo", default: true, label: "未着手" },
    { name: "doing", category: "doing", default: true, label: "進行中" },
    { name: "review", category: "doing", label: "レビュー" },
    { name: "done", category: "done", default: true, label: "完了" },
  ],
  kinds: [],
  props: {
    points: { type: "number" },
    area: { type: "text" },
    parent_ref: { type: "task_ref" },
    tags: { type: "multi_enum", values: ["a", "b"] },
  },
  scoped: [
    {
      where: "project:P1",
      statuses: ["todo", "doing", "done"],
      workflow: { transitions: { todo: ["doing"] }, required: { doing: ["area"] } },
    },
  ],
  workflow: {
    transitions: { done: ["todo"], review: [] },
    required: { done: ["points", "assignees"], doing: [] },
  },
} as unknown as SchemaView;

const col = (key: string, over: Partial<Column> = {}): Column => ({
  key,
  label: key,
  category: "todo",
  status: key,
  unknown: false,
  ...over,
});

const base = {
  task: task("A", "task", null, { id: "REL-1", title: "仕事", status: "todo" }),
  sameLane: true,
  swimlane: "none" as const,
  mode: "status" as const,
  schema,
};

describe("workflow of a task", () => {
  it("allows a move the store lists, and any move from a status it does not list", () => {
    expect(canTransition(schema, undefined, "done", "todo")).toBe(true);
    expect(canTransition(schema, undefined, "done", "doing")).toBe(false);
    expect(canTransition(schema, undefined, "todo", "done")).toBe(true);
    expect(canTransition(schema, undefined, "review", "todo")).toBe(false);
    expect(canTransition(schema, undefined, "todo", "todo")).toBe(true);
  });

  it("asks the project too: both have to allow it", () => {
    expect(canTransition(schema, "P1", "todo", "done")).toBe(false);
    expect(canTransition(schema, "P1", "todo", "doing")).toBe(true);
    expect(canTransition(schema, "P2", "todo", "done")).toBe(true);
  });

  it("joins the required fields of the store and of the project", () => {
    expect(requiredFields(schema, undefined, "done")).toEqual(["points", "assignees"]);
    expect(requiredFields(schema, "P1", "doing")).toEqual(["area"]);
    expect(requiredFields(schema, "P1", "todo")).toEqual([]);
  });

  it("reads the statuses a project shows", () => {
    expect(scopedStatusesOf(schema, "P1")).toEqual(["todo", "doing", "done"]);
    expect(scopedStatusesOf(schema, "P2")).toBeUndefined();
    expect(scopedStatusesOf(schema, undefined)).toBeUndefined();
  });
});

describe("requiredGap", () => {
  const t = task("A", "task", null, { props: { points: 0, area: "" }, assignees: [] });

  it("names the empty fields the dialog can take in", () => {
    expect(requiredGap(t, ["points", "area", "assignees", "labels"], schema)).toEqual({
      fill: ["area", "assignees", "labels"],
      elsewhere: [],
    });
  });

  it("counts 0 and false as filled, and an empty list as empty", () => {
    const u = task("B", "task", null, { props: { points: 0, tags: [] }, assignees: ["gh/koki"] });
    expect(requiredGap(u, ["points", "tags", "assignees"], schema).fill).toEqual(["tags"]);
  });

  it("sends the fields only the detail can fill there: the parent, blockers, task references, and what the API does not show", () => {
    const gap = requiredGap(
      t,
      ["parent", "blocked_by", "sprint", "releases", "links", "parent_ref", "nothing"],
      schema,
    );
    expect(gap).toEqual({
      fill: [],
      elsewhere: ["parent", "blocked_by", "sprint", "releases", "links", "parent_ref", "nothing"],
    });
  });

  it("leaves out the parent, blockers and a task reference the task has", () => {
    const u = task("B", "task", "P", {
      blocked_by: [{ ulid: "Z", id: "REL-9", title: "z" }],
      props: { parent_ref: "01Z" },
    });
    expect(requiredGap(u, ["parent", "blocked_by", "parent_ref"], schema).elsewhere).toEqual([]);
  });

  it("reads watchers and repos from the full task, and counts them as lacking without it", () => {
    const full = { watchers: ["gh/koki"], repos: [] } as unknown as ShowView;
    expect(requiredGap(t, ["watchers", "repos"], schema).elsewhere).toEqual(["watchers", "repos"]);
    expect(requiredGap(t, ["watchers", "repos"], schema, full).elsewhere).toEqual(["repos"]);
  });
});

describe("decideDrop", () => {
  it("is silent for the column the task is in", () => {
    expect(decideDrop({ ...base, column: col("todo") })).toMatchObject({ ok: false, silent: true });
  });

  it("moves to the status of the column, with the fields still empty", () => {
    const d = decideDrop({
      ...base,
      task: task("A", "task", null, { status: "doing", props: {}, assignees: [] }),
      column: col("review"),
    });
    expect(d).toEqual({ ok: true, status: "review", fill: [], elsewhere: [] });
    const done = decideDrop({
      ...base,
      task: task("A", "task", null, { status: "doing", props: { points: 3 }, assignees: [] }),
      column: col("done"),
    });
    expect(done).toEqual({ ok: true, status: "done", fill: ["assignees"], elsewhere: [] });
  });

  it("lists the required fields only the detail can fill, next to the ones the dialog can", () => {
    const needy = {
      ...schema,
      workflow: { ...schema.workflow, required: { done: ["points", "parent", "watchers"] } },
    } as unknown as SchemaView;
    const t = task("A", "task", null, { status: "doing", props: {} });
    const d = decideDrop({ ...base, task: t, schema: needy, column: col("done") });
    expect(d).toEqual({
      ok: true,
      status: "done",
      fill: ["points"],
      elsewhere: ["parent", "watchers"],
    });
    const full = { watchers: ["gh/koki"] } as unknown as ShowView;
    const e = decideDrop({ ...base, task: t, schema: needy, column: col("done"), detail: full });
    expect(e).toMatchObject({ elsewhere: ["parent"] });
  });

  it("refuses to cross a swimlane, and says what would change", () => {
    const a = decideDrop({
      ...base,
      column: col("doing"),
      sameLane: false,
      swimlane: "assignee",
    });
    expect(a).toMatchObject({
      ok: false,
      silent: false,
      title: "担当者の行をまたぐ移動はできません",
    });
    const p = decideDrop({ ...base, column: col("todo"), sameLane: false, swimlane: "project" });
    expect(p).toMatchObject({
      ok: false,
      silent: false,
      title: "プロジェクトをまたぐ移動はできません",
    });
  });

  it("refuses a move the workflow does not allow, with the reason", () => {
    const d = decideDrop({
      ...base,
      task: task("A", "task", null, { id: "REL-1", title: "仕事", status: "done" }),
      column: col("doing"),
    });
    expect(d).toMatchObject({
      ok: false,
      silent: false,
      title: "「完了」から「進行中」には移せません",
    });
    expect(d.ok ? "" : d.reason).toContain("REL-1 仕事");
  });

  it("uses the workflow of the project of the task", () => {
    const t = task("A", "task", null, {
      status: "todo",
      project: { ulid: "P1", id: "P-1", title: "p" },
    });
    expect(decideDrop({ ...base, task: t, column: col("done") })).toMatchObject({
      ok: false,
      silent: false,
    });
    expect(decideDrop({ ...base, task: t, column: col("doing") })).toMatchObject({
      ok: true,
      fill: ["area"],
    });
  });

  it("refuses a status the schema of the task does not have", () => {
    const d = decideDrop({ ...base, column: col("shipped") });
    expect(d).toMatchObject({ ok: false, silent: false, title: "「shipped」には移せません" });
  });

  it("refuses a column no schema lists", () => {
    const d = decideDrop({ ...base, column: col("gone", { status: null, unknown: true }) });
    expect(d).toMatchObject({ ok: false, silent: false });
  });

  it("waits for the schema", () => {
    const d = decideDrop({ ...base, schema: undefined, column: col("doing") });
    expect(d).toMatchObject({
      ok: false,
      silent: false,
      title: "少し待ってからやり直してください",
    });
  });

  describe("by category", () => {
    const by = { ...base, mode: "category" as const };

    it("moves to the default status of the category in the schema of the task", () => {
      const d = decideDrop({
        ...by,
        column: col("doing", { category: "doing", status: null }),
      });
      expect(d).toEqual({ ok: true, status: "doing", fill: [], elsewhere: [] });
    });

    it("is silent for the category the task is in", () => {
      const t = task("A", "task", null, { status: "review" });
      const d = decideDrop({
        ...by,
        task: t,
        column: col("doing", { category: "doing", status: null }),
      });
      expect(d).toMatchObject({ ok: false, silent: true });
    });

    it("refuses a category the schema has no status in", () => {
      const d = decideDrop({
        ...by,
        column: col("canceled", { category: "canceled", status: null, label: "中止" }),
      });
      expect(d).toMatchObject({ ok: false, silent: false, title: "「中止」には移せません" });
    });
  });
});
```

```bash
cd web && pnpm vitest run app/kanban/drop.test.ts
```

Expected: FAIL（`Failed to resolve import "./drop"`）。

- [ ] **Step 7: `drop.ts` を実装する**

`web/app/kanban/drop.ts`:

```ts
import type { ShowView, TaskView } from "../../core/api/client";
import { defaultStatusIn, propEditorOf, type SchemaView } from "../../core/schema";
import type { KanbanSettings } from "../../core/views";
import { CATEGORY_LABEL } from "../workspace/counts";
import { type Column, columnKeyOf } from "./board";

type Scoped = NonNullable<SchemaView["scoped"]>[number];
type Transitions = Record<string, string[] | null>;

// The override of a project (its scoped entry in schema.yaml), by the ULID of the project.
function scopedOf(schema: SchemaView, project: string | undefined): Scoped | undefined {
  if (!project) return undefined;
  return (schema.scoped ?? []).find((s) => s.where === `project:${project}`);
}

// scopedStatusesOf is the statuses a project shows, or undefined when it shows all of the store's.
export function scopedStatusesOf(
  schema: SchemaView,
  project: string | undefined,
): string[] | undefined {
  const list = scopedOf(schema, project)?.statuses;
  return list && list.length > 0 ? list : undefined;
}

// allows applies one workflow: only the statuses it lists are limited, and one it lists with no
// targets lets nothing out.
function allows(transitions: Transitions | undefined, from: string, to: string): boolean {
  if (!transitions || !Object.hasOwn(transitions, from)) return true;
  return (transitions[from] ?? []).includes(to);
}

// canTransition is whether a task may move from one status to another: the store and the
// project (when it has an override) both have to allow it, as the server asks.
export function canTransition(
  schema: SchemaView,
  project: string | undefined,
  from: string,
  to: string,
): boolean {
  if (from === to) return true;
  if (!allows(schema.workflow?.transitions, from, to)) return false;
  return allows(scopedOf(schema, project)?.workflow?.transitions, from, to);
}

// requiredFields are the fields a task needs to enter a status: the store's and the project's.
export function requiredFields(
  schema: SchemaView,
  project: string | undefined,
  status: string,
): string[] {
  const out = [...(schema.workflow?.required?.[status] ?? [])];
  for (const f of scopedOf(schema, project)?.workflow?.required?.[status] ?? []) {
    if (!out.includes(f)) out.push(f);
  }
  return out;
}

// What a required field of a task lacks: `fill` are the empty ones the dialog can take in (assignees,
// labels, a property it has an editor for); `elsewhere` are the ones that can only be filled in the
// task detail (the parent, blockers, watchers, repos, a task reference, and what the API does not
// show: sprint, releases, links), or whose value is not known here.
export type RequiredGap = { fill: string[]; elsewhere: string[] };

const empty = (v: unknown): boolean =>
  v === null || v === undefined || v === "" || (Array.isArray(v) && v.length === 0);

// requiredGap sorts the required fields the task lacks. detail is the full task (the list carries
// no watchers or repos): without it those two count as lacking. The fields the task has are in
// neither list.
export function requiredGap(
  t: TaskView,
  names: readonly string[],
  schema: SchemaView,
  detail?: ShowView,
): RequiredGap {
  const gap: RequiredGap = { fill: [], elsewhere: [] };
  for (const name of names) {
    switch (name) {
      case "assignees":
        if (empty(t.assignees)) gap.fill.push(name);
        break;
      case "labels":
        if (empty(t.labels)) gap.fill.push(name);
        break;
      case "parent":
        if (t.parent === null) gap.elsewhere.push(name);
        break;
      case "blocked_by":
        if (empty(t.blocked_by)) gap.elsewhere.push(name);
        break;
      case "watchers":
      case "repos":
        if (!detail || empty(detail[name])) gap.elsewhere.push(name);
        break;
      case "sprint":
      case "releases":
      case "links":
        gap.elsewhere.push(name);
        break;
      default: {
        if (!empty((t.props as Record<string, unknown> | undefined)?.[name])) break;
        const editor = propEditorOf(schema, name);
        (editor === null || editor === "taskref" ? gap.elsewhere : gap.fill).push(name);
      }
    }
  }
  return gap;
}

// silent: nothing to do and nothing to say (a drop on the column the card is in).
export type DropDecision =
  | { ok: true; status: string; fill: string[]; elsewhere: string[] }
  | { ok: false; silent: boolean; title: string; reason: string };

const silent: DropDecision = { ok: false, silent: true, title: "", reason: "" };

function refuse(title: string, reason: string): DropDecision {
  return { ok: false, silent: false, title, reason };
}

export type DropInput = {
  task: TaskView;
  column: Column;
  // The card is dropped in the row it came from.
  sameLane: boolean;
  swimlane: KanbanSettings["swimlane"];
  mode: KanbanSettings["columns"];
  // The schema of the store of the task.
  schema: SchemaView | undefined;
  // The full task, for the required fields the list does not carry (watchers, repos).
  detail?: ShowView;
};

// decideDrop says whether the card may go to the column, and the status it then gets. It reads no
// DOM; the checks come in the order the reasons are worth giving in.
export function decideDrop(i: DropInput): DropDecision {
  const { task, column, schema } = i;
  const name = `「${task.id} ${task.title}」`;

  if (!i.sameLane) {
    return i.swimlane === "project"
      ? refuse(
          "プロジェクトをまたぐ移動はできません",
          `${name}を別のプロジェクトへ移すことになります。ドラッグで変えられるのはステータスだけです。`,
        )
      : refuse(
          "担当者の行をまたぐ移動はできません",
          `${name}の担当者を変えることになります。ドラッグで変えられるのはステータスだけです。`,
        );
  }
  if (columnKeyOf(task, i.mode, schema) === column.key) return silent;
  if (column.unknown) {
    return refuse(`「${column.label}」には移せません`, "どのスキーマにもないステータスです。");
  }
  if (!schema) {
    return refuse(
      "少し待ってからやり直してください",
      `${task.store} の schema.yaml を読み込んでいます。`,
    );
  }

  const to = column.status ?? defaultStatusIn(schema, column.key);
  if (to === "" || !schema.statuses?.some((s) => s.name === to)) {
    return refuse(
      `「${column.label}」には移せません`,
      column.status === null
        ? `${task.store} の schema.yaml に${CATEGORY_LABEL[column.category]}のステータスがありません。`
        : `${name}は ${task.store} にあり、その schema.yaml に「${column.label}」というステータスがありません。`,
    );
  }
  const project = task.project?.ulid;
  if (!canTransition(schema, project, task.status, to)) {
    const label = (s: string) => schema.statuses?.find((x) => x.name === s)?.label || s;
    return refuse(
      `「${label(task.status)}」から「${label(to)}」には移せません`,
      `${name}は、遷移の制約（schema.yaml の workflow.transitions）で許されていません。`,
    );
  }
  const gap = requiredGap(task, requiredFields(schema, project, to), schema, i.detail);
  return { ok: true, status: to, ...gap };
}
```

```bash
cd web && pnpm vitest run app/kanban/drop.test.ts
```

Expected: PASS（21 件）。

- [ ] **Step 8: Pragmatic DnD の包み `dnd.ts` のテストを書く（失敗する）と実装する**

`tree/dnd.ts` と同じ流儀（オプションは関数で渡し、ドラッグ・ドロップのたびに読む。1つの attachment がカードの一生を通して使える）。テストは `tree/dnd.row.test.ts` と同じく、アダプターを `vi.mock` で差し替える。

`web/app/kanban/dnd.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";

type Target = {
  canDrop: (a: { source: { data: Record<string, unknown> } }) => boolean;
  onDragEnter: (a: { source: { data: Record<string, unknown> } }) => void;
  onDragLeave: () => void;
  onDrop: (a: { source: { data: Record<string, unknown> } }) => void;
};
const h = vi.hoisted(() => ({ target: undefined as unknown, drag: undefined as unknown }));

vi.mock("@atlaskit/pragmatic-drag-and-drop/element/adapter", () => ({
  draggable: (o: unknown) => {
    h.drag = o;
    return () => {};
  },
  dropTargetForElements: (o: unknown) => {
    h.target = o;
    return () => {};
  },
  monitorForElements: () => () => {},
}));

import { type CardDndOptions, type CellDndOptions, cardDnd, cellDnd } from "./dnd";

const el = {} as HTMLElement;
const card = { data: { type: "kanban-card", ulid: "A", lane: "gh/koki" } };

beforeEach(() => {
  h.target = undefined;
  h.drag = undefined;
});

describe("cardDnd", () => {
  it("carries the task and the row it was taken from, read when the drag starts", () => {
    const onDragStart = vi.fn();
    let opts: CardDndOptions = { ulid: "A", lane: "x", onDragStart };
    cardDnd(() => opts)(el);
    opts = { ulid: "B", lane: "y", onDragStart };
    const d = h.drag as { getInitialData: () => unknown; onDragStart: () => void };
    expect(d.getInitialData()).toEqual({ type: "kanban-card", ulid: "B", lane: "y" });
    d.onDragStart();
    expect(onDragStart).toHaveBeenCalled();
  });
});

describe("cellDnd", () => {
  function setup() {
    const opts: CellDndOptions = { onOver: vi.fn(), onDrop: vi.fn() };
    cellDnd(() => opts)(el);
    return { opts, target: h.target as Target };
  }

  it("takes cards only", () => {
    const { target } = setup();
    expect(target.canDrop({ source: card })).toBe(true);
    expect(target.canDrop({ source: { data: { type: "tree-row" } } })).toBe(false);
  });

  it("tells which card is over, and when it leaves", () => {
    const { opts, target } = setup();
    target.onDragEnter({ source: card });
    expect(opts.onOver).toHaveBeenLastCalledWith({ ulid: "A", lane: "gh/koki" });
    target.onDragLeave();
    expect(opts.onOver).toHaveBeenLastCalledWith(null);
  });

  it("clears the cell and drops the card", () => {
    const { opts, target } = setup();
    target.onDrop({ source: card });
    expect(opts.onOver).toHaveBeenCalledWith(null);
    expect(opts.onDrop).toHaveBeenCalledWith({ ulid: "A", lane: "gh/koki" });
  });
});
```

```bash
cd web && pnpm vitest run app/kanban/dnd.test.ts
```

Expected: FAIL（`Failed to resolve import "./dnd"`）。続けて実装:

`web/app/kanban/dnd.ts`:

```ts
import {
  draggable,
  dropTargetForElements,
  monitorForElements,
} from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { autoScrollForElements } from "@atlaskit/pragmatic-drag-and-drop-auto-scroll/element";
import type { Attachment } from "svelte/attachments";

const CARD = "kanban-card";

// A card being dragged: its task, and the row it was taken from ("" without swimlanes).
export type DragSource = { ulid: string; lane: string };

function sourceOf(data: Record<string | symbol, unknown>): DragSource {
  return { ulid: String(data.ulid), lane: String(data.lane) };
}

export type CardDndOptions = DragSource & { onDragStart: () => void };

// cardDnd makes a card draggable. Use it as {@attach cardDnd(get)}. The options are read when
// needed, so one attachment serves the card for as long as it lives.
export function cardDnd(get: () => CardDndOptions): Attachment<HTMLElement> {
  return (el) =>
    draggable({
      element: el,
      getInitialData: () => ({ type: CARD, ulid: get().ulid, lane: get().lane }),
      onDragStart: () => get().onDragStart(),
    });
}

export type CellDndOptions = {
  // The card that is over the cell, or null when it leaves.
  onOver: (source: DragSource | null) => void;
  onDrop: (source: DragSource) => void;
};

// cellDnd makes a cell of the board (a column in a row) a place cards are dropped on.
export function cellDnd(get: () => CellDndOptions): Attachment<HTMLElement> {
  return (el) =>
    dropTargetForElements({
      element: el,
      canDrop: ({ source }) => source.data.type === CARD,
      onDragEnter: ({ source }) => get().onOver(sourceOf(source.data)),
      onDragLeave: () => get().onOver(null),
      onDrop: ({ source }) => {
        get().onOver(null);
        get().onDrop(sourceOf(source.data));
      },
    });
}

// boardDragEnd calls onEnd when the drag of a card ends, wherever it ends: dropped outside the
// board, cancelled, or with the dragged card gone from the screen.
export function boardDragEnd(onEnd: () => void): Attachment<HTMLElement> {
  return () =>
    monitorForElements({
      canMonitor: ({ source }) => source.data.type === CARD,
      onDrop: onEnd,
    });
}

// boardAutoScroll scrolls the board when a drag nears its edge.
export function boardAutoScroll(): Attachment<HTMLElement> {
  return (el) => autoScrollForElements({ element: el });
}
```

```bash
cd web && pnpm vitest run app/kanban/dnd.test.ts
```

Expected: PASS（4 件）。

- [ ] **Step 9: 作成ダイアログに開始時のステータスを渡す（細部 42）**

`openCreate` は `status` を渡せなかった。列の「＋」はその列のステータスで開く。カテゴリ列は Store ごとにステータスが違うので、カテゴリ名（`category`）を渡し、作成ダイアログが入れ先のワークスペースのスキーマの既定のステータスに直す。`status` がそのワークスペースのスキーマになければ、`category` の既定、それもなければ今までどおり todo の既定に落ちる。テストを先に足す（`CreateTask.test.ts` の差分）:

`web/app/create/CreateTask.test.ts` の差分:

```diff
--- a/web/app/create/CreateTask.test.ts
+++ b/web/app/create/CreateTask.test.ts
@@ -1,4 +1,4 @@
-import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
+import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
 import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
 import { parentRule } from "../../core/schema";
 import {
@@ -245,4 +245,42 @@
     mount(CreateTask, { request: { kind: "task", parent: "UP" }, onclose: vi.fn() }, k);
     expect(screen.getByText("REL-8")).toBeInTheDocument();
   });
+
+  describe("the status of a column of the kanban", () => {
+    async function createWith(request: Record<string, unknown>) {
+      const k = makeCore({ tasks: [parent] });
+      k.api.POST.mockImplementation(() => ok({ task: created, commit: "c1" }));
+      mount(CreateTask, { request: { kind: "task", ...request }, onclose: vi.fn() }, k);
+      await typeTitle("列から");
+      await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
+      return {
+        k,
+        send: async () => {
+          await fireEvent.click(screen.getByRole("button", { name: /^作成/ }));
+          await waitFor(() => expect(k.api.POST).toHaveBeenCalled());
+          const [, init] = k.api.POST.mock.calls[0] as [string, { body: Record<string, unknown> }];
+          return init.body;
+        },
+      };
+    }
+
+    it("starts the task in the status the request names, and the command says so", async () => {
+      const { send } = await createWith({ status: "waiting" });
+      expect(screen.getByText(/-p status=waiting/)).toBeInTheDocument();
+      expect((await send()).status).toBe("waiting");
+    });
+
+    it("starts it in the default status of the category the request names", async () => {
+      const { send } = await createWith({ category: "doing" });
+      expect((await send()).status).toBe("in_progress");
+    });
+
+    it("falls back to the category, then to todo, for a status the workspace lacks", async () => {
+      const a = await createWith({ status: "review", category: "done" });
+      expect((await a.send()).status).toBe("done");
+      cleanup();
+      const b = await createWith({ status: "review" });
+      expect((await b.send()).status).toBe("todo");
+    });
+  });
 });
```

```bash
cd web && pnpm vitest run app/create/CreateTask.test.ts
```

Expected: FAIL（新しい3件。status が `todo` のまま）。実装:

`web/app/create-state.svelte.ts` の差分:

```diff
--- a/web/app/create-state.svelte.ts
+++ b/web/app/create-state.svelte.ts
@@ -1,5 +1,15 @@
-/** What the create dialog is asked for. `parent` is the ULID of the task the new one goes under. */
-export type CreateRequest = { kind: "task" | "project"; parent?: string };
+/**
+ * What the create dialog is asked for. `parent` is the ULID of the task the new one goes under.
+ * `status` is the status it starts with (the column of a kanban); `category` is the category whose
+ * default status it starts with (the column of a kanban by category), used when `status` is not one
+ * of the workspace's.
+ */
+export type CreateRequest = {
+  kind: "task" | "project";
+  parent?: string;
+  status?: string;
+  category?: string;
+};
 
 let request = $state<CreateRequest | null>(null);
```

`web/app/create/CreateTask.svelte` の差分:

```diff
--- a/web/app/create/CreateTask.svelte
+++ b/web/app/create/CreateTask.svelte
@@ -3,7 +3,7 @@
   import { type TaskView, unwrap } from "../../core/api/client";
   import { cliAdd, Placeholder } from "../../core/cli";
   import { errorMessage } from "../../core/messages";
-  import { canParent, defaultStatus, parentRule } from "../../core/schema";
+  import { canParent, defaultStatus, defaultStatusIn, parentRule } from "../../core/schema";
   import { core } from "../context";
   import type { CreateRequest } from "../create-state.svelte";
   import BodyEditor from "../detail/BodyEditor.svelte";
@@ -64,7 +64,7 @@
   let title = $state("");
   let body = $state("");
   let kind = $state("");
-  let status = $state("");
+  let status = $state(untrack(() => request.status ?? ""));
   let assignees = $state<string[]>([]);
   let propValues = $state<Record<string, unknown>>({});
   let busy = $state(false);
@@ -95,7 +95,10 @@
     const s = sc.current;
     if (!s) return;
     if (!s.statuses?.some((x) => x.name === untrack(() => status))) {
-      status = defaultStatus(s, "todo");
+      // A status that the request names but this workspace's schema lacks falls back to the
+      // default of the category the request names (a column of the category board), else todo.
+      const cat = untrack(() => request.category);
+      status = (cat ? defaultStatusIn(s, cat) : "") || defaultStatus(s, "todo");
     }
   });
```

```bash
cd web && pnpm vitest run app/create/CreateTask.test.ts
```

Expected: PASS（22 件）。

- [ ] **Step 10: `patchTask` の本文の型を広げる**

カンバンは `status` に加えて、必須の項目（`props`・`assignees`・`labels`）を同じ `PATCH` で送る（1コミット）。`tree/actions.ts` の `patchTask` は本文の型が `{ status?, kind? }` だったので、API の型 `PatchInputBody` にする。呼び出し側（ツリー）は変わらない。

`web/app/tree/actions.ts` の差分:

```diff
--- a/web/app/tree/actions.ts
+++ b/web/app/tree/actions.ts
@@ -1,4 +1,4 @@
-import { type TaskView, unwrap } from "../../core/api/client";
+import { type Schemas, type TaskView, unwrap } from "../../core/api/client";
 import type { AppCore } from "../context";
 import type { MoveArgs } from "./placement";
 
@@ -23,7 +23,7 @@
 export async function patchTask(
   d: Deps,
   task: TaskView,
-  body: { status?: string; kind?: string },
+  body: Schemas["PatchInputBody"],
 ): Promise<TaskView> {
   const r = await unwrap(
     d.api.PATCH("/api/tasks/{id}", {
```

```bash
cd web && pnpm vitest run app/tree/actions.test.ts && pnpm check
```

Expected: PASS、警告 0。

- [ ] **Step 11: ここまでをコミットする**

```bash
cd web && pnpm fix && pnpm check && pnpm exec biome ci . && pnpm vitest run core app/kanban app/create app/tree
```

Expected: 整形の差分なし、警告 0、指摘 0、全件 PASS。リポジトリの直下から:

```bash
git add web/core/schema.ts web/core/schema.test.ts web/core/status-order.ts web/core/status-order.test.ts web/app/kanban web/app/create-state.svelte.ts web/app/create/CreateTask.svelte web/app/create/CreateTask.test.ts web/app/tree/actions.ts
git commit -m "feat(web): カンバンの列と行の組み立て、ドロップの判定、作成ダイアログの開始時のステータス" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

（`web/app/kanban` にはこの時点で `board.ts`・`drop.ts`・`dnd.ts` とそのテストだけがある。）

- [ ] **Step 12: 必須の項目のダイアログ `RequiredDialog.svelte` のテストを書く（失敗する）**

細部 39。移る先のステータスに必須の項目が欠けていれば、「<ID> を<ステータス>にするには <項目> が必要です」のダイアログに欠けている項目を全部並べる。`fields`（担当者・ラベル・プロパティ）はその場で入力でき、ステータスと一緒に1回の `PATCH`（1コミット）で送る。`elsewhere`（詳細でしか入力できない項目）は「詳細で入力」のリンク（タスク詳細を開き、ダイアログを閉じる）を並べ、移動のボタンを無効にする（サーバーに 422 を起こさせない）。ダイアログ自身は送らず、`onsubmit(body)` を呼び、失敗（`throw`）はダイアログの中に `errorMessage` で出して開いたままにする。足元は `hh set <ID> status=<…> <項目>=<値>`（そろうまでコピー不可）。

`web/app/kanban/RequiredDialog.test.ts`:

```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "../../core/api/client";
import { makeCore, mount, schemaView, taskView } from "../detail/testkit";
import RequiredDialog from "./RequiredDialog.svelte";

const t = taskView({
  ulid: "U16",
  id: "REL-16",
  title: "アラートの閾値を見直す",
  status: "in_progress",
});

function setup(
  over: { fields?: string[]; elsewhere?: string[]; onsubmit?: (b: unknown) => Promise<void> } = {},
) {
  const onsubmit = vi.fn(over.onsubmit ?? (async () => {}));
  const onclose = vi.fn();
  mount(
    RequiredDialog,
    {
      task: t,
      status: "done",
      fields: over.fields ?? ["points"],
      elsewhere: over.elsewhere ?? [],
      schema: schemaView,
      onsubmit,
      onclose,
    },
    makeCore(),
  );
  return { onsubmit, onclose };
}

describe("RequiredDialog", () => {
  it("says what the status needs, and keeps the button off until the field is filled", async () => {
    setup();
    expect(
      screen.getByRole("dialog", { name: "REL-16 を完了にするには points が必要です" }),
    ).toBeInTheDocument();
    const go = screen.getByRole("button", { name: "完了にする" });
    expect(go).toBeDisabled();
    await fireEvent.input(screen.getByRole("spinbutton", { name: "points" }), {
      target: { value: "3" },
    });
    expect(go).toBeEnabled();
    expect(screen.getByText(/hh set REL-16 status=done points=3/)).toBeInTheDocument();
  });

  it("sends the status and the fields in one call, then closes", async () => {
    const { onsubmit, onclose } = setup({ fields: ["points", "assignees"] });
    await fireEvent.input(screen.getByRole("spinbutton", { name: "points" }), {
      target: { value: "3" },
    });
    expect(screen.getByRole("button", { name: "完了にする" })).toBeDisabled();
    const chips = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(chips, { target: { value: "github.com/koki" } });
    await fireEvent.keyDown(chips, { key: "Enter" });
    await fireEvent.click(screen.getByRole("button", { name: "完了にする" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(onsubmit).toHaveBeenCalledWith({
      status: "done",
      assignees: ["github.com/koki"],
      props: { points: 3 },
    });
  });

  it("keeps the dialog open and says why when the write is refused", async () => {
    const err = new ApiError({
      status: 422,
      type: "schema.violation",
      detail: "",
      args: { violations: [{ code: "required", field: "points", message: "x" }] },
    });
    const { onclose } = setup({
      onsubmit: async () => {
        throw err;
      },
    });
    await fireEvent.input(screen.getByRole("spinbutton", { name: "points" }), {
      target: { value: "3" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "完了にする" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("スキーマに合わない変更です");
    expect(onclose).not.toHaveBeenCalled();
  });

  it("closes on やめる without sending", async () => {
    const { onsubmit, onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "やめる" }));
    expect(onclose).toHaveBeenCalled();
    expect(onsubmit).not.toHaveBeenCalled();
  });

  describe("with a field only the detail can fill", () => {
    it("lists it next to the ones the dialog takes in, with a link to the detail, and does not let the card move", async () => {
      const { onsubmit, onclose } = setup({
        fields: ["points"],
        elsewhere: ["parent", "watchers"],
      });
      expect(
        screen.getByRole("dialog", {
          name: "REL-16 を完了にするには points・親・ウォッチャー が必要です",
        }),
      ).toBeInTheDocument();
      const links = screen.getAllByRole("link", { name: "詳細で入力" });
      expect(links).toHaveLength(2);
      expect(links[0]).toHaveAttribute("href", "/tasks/U16");
      await fireEvent.input(screen.getByRole("spinbutton", { name: "points" }), {
        target: { value: "3" },
      });
      const go = screen.getByRole("button", { name: "完了にする" });
      expect(go).toBeDisabled();
      await fireEvent.click(go);
      expect(onsubmit).not.toHaveBeenCalled();
      expect(onclose).not.toHaveBeenCalled();
    });

    it("opens the detail and closes the dialog from the link", async () => {
      const { onclose } = setup({ fields: [], elsewhere: ["sprint"] });
      await fireEvent.click(screen.getByRole("link", { name: "詳細で入力" }));
      expect(onclose).toHaveBeenCalled();
      expect(window.location.pathname).toBe("/tasks/U16");
    });
  });
});
```

```bash
cd web && pnpm vitest run app/kanban/RequiredDialog.test.ts
```

Expected: FAIL（`Failed to resolve import "./RequiredDialog.svelte"`）。

- [ ] **Step 13: `RequiredDialog.svelte` を実装する**

`web/app/kanban/RequiredDialog.svelte`:

```svelte
<script lang="ts">
  import type { Schemas, TaskView } from "../../core/api/client";
  import { cliSet } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { SchemaView } from "../../core/schema";
  import FieldInput from "../detail/FieldInput.svelte";
  import PropField from "../detail/PropField.svelte";
  import { propRows } from "../detail/prop-rows";
  import { onLinkClick, router } from "../router";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";

  type Props = {
    task: TaskView;
    status: string; // the status the card is dropped on
    fields: string[]; // the required fields the task lacks and this dialog takes in (requiredGap.fill)
    elsewhere: string[]; // the ones only the task detail can fill (requiredGap.elsewhere)
    schema: SchemaView;
    // Sends the status and the fields in one PATCH. A failure is thrown and shown here.
    onsubmit: (body: Schemas["PatchInputBody"]) => Promise<void>;
    onclose: () => void;
  };
  let { task, status, fields, elsewhere, schema, onsubmit, onclose }: Props = $props();

  const statusLabel = $derived(schema.statuses?.find((s) => s.name === status)?.label || status);
  const rows = $derived(propRows(schema, {}));
  const rowOf = (name: string) => rows.find((r) => r.name === name);
  const NAMES: Record<string, string> = {
    assignees: "担当者",
    labels: "ラベル",
    parent: "親",
    blocked_by: "ブロックしているタスク",
    watchers: "ウォッチャー",
    repos: "リポジトリ",
    sprint: "スプリント",
    releases: "リリース",
    links: "リンク",
  };
  const labelOf = (name: string): string =>
    NAMES[name] ?? schema.props[name]?.label ?? rowOf(name)?.label ?? name;
  const detailHref = $derived(router.href({ name: "task", ulid: task.ulid }));

  let assignees = $state<string[]>([]);
  let labels = $state<string[]>([]);
  let values = $state<Record<string, unknown>>({});
  let busy = $state(false);
  let error = $state("");

  const empty = (v: unknown) =>
    v === null || v === undefined || v === "" || (Array.isArray(v) && v.length === 0);
  const currentOf = (name: string): unknown =>
    name === "assignees" ? assignees : name === "labels" ? labels : values[name];
  const complete = $derived(fields.every((f) => !empty(currentOf(f))));

  const title = $derived(
    `${task.id} を${statusLabel}にするには ${[...fields, ...elsewhere].map(labelOf).join("・")} が必要です`,
  );
  const cli = $derived(
    cliSet(task.id, {
      status,
      ...Object.fromEntries(
        fields
          .filter((f) => !empty(currentOf(f)))
          .map((f) => {
            const v = currentOf(f);
            return [f, Array.isArray(v) ? v.join(",") : String(v)];
          }),
      ),
    }),
  );

  async function submit() {
    if (!complete || elsewhere.length > 0 || busy) return;
    busy = true;
    error = "";
    const body: Schemas["PatchInputBody"] = { status };
    if (fields.includes("assignees")) body.assignees = assignees;
    if (fields.includes("labels")) body.labels = labels;
    const sent = Object.fromEntries(
      fields.filter((f) => f !== "assignees" && f !== "labels").map((f) => [f, values[f]]),
    );
    if (Object.keys(sent).length > 0) body.props = sent;
    try {
      await onsubmit(body);
      onclose();
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open {title} width={560} {cli} cliDisabled={!complete || elsewhere.length > 0} {onclose}>
  <div class="form">
    {#each fields as name (name)}
      <div class="prow">
        <span class="lbl">{labelOf(name)}</span>
        {#if name === "assignees"}
          <FieldInput
            editor="chips"
            label="担当者"
            value={assignees}
            onedit={(v) => (assignees = v as string[])}
            oncommit={() => {}}
          />
        {:else if name === "labels"}
          <FieldInput
            editor="chips"
            label="ラベル"
            value={labels}
            onedit={(v) => (labels = v as string[])}
            oncommit={() => {}}
          />
        {:else}
          {@const row = rowOf(name)}
          {#if row}
            <PropField
              {row}
              value={values[name] ?? null}
              onedit={(v) => (values[name] = v)}
              oncommit={() => {}}
            />
          {/if}
        {/if}
      </div>
    {/each}
    {#each elsewhere as name (name)}
      <div class="prow">
        <span class="lbl">{labelOf(name)}</span>
        <a
          class="detail"
          href={detailHref}
          onclick={(e) => {
            onLinkClick(e, detailHref);
            onclose();
          }}
          >詳細で入力</a
        >
      </div>
    {/each}
    {#if elsewhere.length > 0}
      <p class="note">
        {fields.length > 0
          ? "ここで入力できない項目"
          : "この項目"}はタスクの詳細で入力してください。入力するまで{statusLabel}にはできません。
      </p>
    {:else}
      <p class="note">ステータスと入力した項目は、1回の保存（1コミット）で変わります。</p>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>

  {#snippet footer()}
    <Button onclick={onclose}>やめる</Button>
    <Button
      variant="primary"
      disabled={!complete || elsewhere.length > 0 || busy}
      onclick={() => void submit()}
    >
      {statusLabel}にする
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: var(--space-12);
  }
  .prow {
    display: grid;
    grid-template-columns: 7em minmax(0, 1fr);
    align-items: center;
    gap: var(--space-8);
  }
  .lbl {
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--ink-2);
  }
  .detail {
    color: var(--accent);
    font-size: var(--fs-13);
  }
  .note {
    margin: 0;
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .error {
    margin: 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
```

```bash
cd web && pnpm vitest run app/kanban/RequiredDialog.test.ts
```

Expected: PASS（6 件）。

- [ ] **Step 14: カード `KanbanCard.svelte` を作る**

細部 40・Foundations の並び順（状態の警告 → 優先度 → 期限 → 担当者、ID は先頭）。警告は `problems` があるときだけ（フロントで必須を判定しない）。PR・WIP 上限・「ほか n 件」は作らない（細部 41、PR はフェーズ 5）。カード全体は `<li>`、タイトルのボタンが `::after` でカードを覆い、どこを押しても詳細が開き、Tab で止まり Enter で開く（ID のボタンはその上にあって別に押せる）。テストは `KanbanView.test.ts`（後の Step）の中で行う。

`web/app/kanban/KanbanCard.svelte`:

```svelte
<script lang="ts">
  import type { Attachment } from "svelte/attachments";
  import type { TaskView } from "../../core/api/client";
  import { formatDate } from "../../core/format";
  import type { SchemaView } from "../../core/schema";
  import type { StoreTone } from "../../core/stores";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import { problemLabel, propText } from "../tree/marks";
  import Avatar from "../ui/Avatar.svelte";
  import IdLabel from "../ui/IdLabel.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";

  type Props = {
    task: TaskView;
    schema: SchemaView | undefined;
    blockedBy: string | undefined; // the ID of the first blocker, when the task is blocked
    tone: StoreTone | undefined; // the colour of the store, when the badge shows
    dragging: boolean;
    onopen: () => void;
    dnd: Attachment<HTMLElement>; // makes the card draggable
  };
  let { task, schema, blockedBy, tone, dragging, onopen, dnd }: Props = $props();

  const category = $derived(schema?.statuses?.find((s) => s.name === task.status)?.category);
  const role = $derived(schema?.kinds?.find((k) => k.name === task.kind)?.role);
  const closed = $derived(category === "done" || category === "canceled");
  const problem = $derived(task.problems?.[0]);
  const due = $derived(propText(task, "due"));
  const priority = $derived(propText(task, "priority"));
  const points = $derived(
    typeof task.props?.points === "number" ? (task.props.points as number) : undefined,
  );
  const kids = $derived(task.progress?.total ?? 0);
  const people = $derived((task.assignees ?? []).slice(0, 3));
  const more = $derived(Math.max(0, (task.assignees ?? []).length - 3));
</script>

<!-- One item of the list of its cell. The title is a button that covers the whole card, so a click
  anywhere on it and Enter on the title open the task; the ID stays above it for its own buttons. -->
<li class="card" class:dragging data-ulid={task.ulid} {@attach dnd}>
  <div class="top">
    <KindIcon kind={task.kind} {role} />
    <span class="id"><IdLabel id={task.id} ulid={task.ulid} /></span>
    {#if tone}
      <StoreBadge store={task.store} {tone} compact />
    {/if}
    {#if points !== undefined}
      <span class="points">{points} pt</span>
    {/if}
  </div>
  <button type="button" class="title" class:closed onclick={onopen}>{task.title}</button>
  <div class="meta">
    {#if task.broken}
      <span class="warn" title={task.broken}><Icon name="warning" size={11} />壊れています</span>
    {:else if problem}
      <span class="warn" title={problem.message}>
        <Icon name="warning" size={11} />{problemLabel(problem)}
      </span>
    {/if}
    {#if blockedBy}
      <span class="blocked"><Icon name="forbid" size={12} />{blockedBy} 待ち</span>
    {/if}
    {#if priority}
      <span class="priority {priority}">{priority}</span>
    {/if}
    {#if task.checklist && task.checklist.total > 0}
      <span class="chip" title="チェックリスト">
        <Icon name="checklist" size={10} />{task.checklist.done}/{task.checklist.total}
      </span>
    {/if}
    {#if kids > 0}
      <span class="chip">子 {kids}</span>
    {/if}
    {#if due}
      <span class="due">{formatDate(due, new Date())}</span>
    {/if}
    <span class="who">
      {#each people as a (a)}
        <Avatar name={a} />
      {/each}
      {#if more > 0}
        <span class="more">+{more}</span>
      {/if}
    </span>
  </div>
</li>

<style>
  .card {
    position: relative;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    list-style: none;
    padding: var(--space-8) var(--space-10);
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-8);
    background: var(--surface);
    box-shadow: 0 var(--hairline) 0 rgba(24, 24, 27, 0.04);
    cursor: pointer;
  }
  .card:hover {
    border-color: var(--line-input);
  }
  .card:focus-within {
    border-color: var(--accent);
  }
  .id {
    position: relative;
    z-index: 1;
  }
  .card.dragging {
    opacity: 0.4;
    border-style: dashed;
  }
  .top {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    min-width: 0;
  }
  .points {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: var(--fs-11);
    color: var(--ink-2);
  }
  .title {
    padding: 0;
    border: 0;
    background: transparent;
    font: inherit;
    font-size: var(--fs-13);
    line-height: 1.45;
    color: var(--ink);
    text-align: left;
    overflow-wrap: anywhere;
    cursor: pointer;
  }
  /* The button covers the card: a click anywhere on it opens the task. */
  .title::after {
    content: "";
    position: absolute;
    inset: 0;
    border-radius: var(--radius-8);
  }
  .title:focus-visible {
    outline: none;
  }
  .title:focus-visible::after {
    box-shadow: inset 0 0 0 calc(var(--hairline) * 2) var(--accent);
  }
  .title.closed {
    color: var(--muted);
  }
  .meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-6);
  }
  .meta:empty {
    display: none;
  }
  .warn,
  .blocked,
  .chip,
  .priority {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    height: 18px;
    padding: 0 var(--space-6);
    font-size: var(--fs-11);
  }
  .warn {
    border-radius: var(--radius-pill);
    background: var(--warning-weak);
    color: var(--warning);
  }
  .blocked {
    border-radius: var(--radius-pill);
    background: var(--danger-weak);
    color: var(--danger);
  }
  .chip {
    border-radius: var(--radius-4);
    background: var(--hover);
    color: var(--ink-2);
    font-family: var(--font-mono);
  }
  .priority {
    border-radius: var(--radius-pill);
    background: var(--hover);
    color: var(--ink-2);
    font-weight: 500;
  }
  .priority.high {
    background: var(--warning-weak);
    color: var(--warning);
  }
  .priority.urgent {
    background: var(--danger-weak);
    color: var(--danger);
  }
  .due {
    font-family: var(--font-mono);
    font-size: var(--fs-11);
    color: var(--ink-2);
  }
  .who {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    margin-left: auto;
  }
  .more {
    font-size: var(--fs-11);
    color: var(--muted);
  }
</style>
```

```bash
cd web && pnpm fix && pnpm check
```

Expected: 警告 0（カードを使う `KanbanView` はまだない）。

- [ ] **Step 15: `KanbanView.test.ts` を書く（失敗する）**

`useListScope`・`dnd.ts` を `vi.mock` で差し替え、キャッシュ・Store・スキーマ・API は偽物を渡す（`TreeView.dnd.test.ts` と同じ流儀）。ドラッグは `dnd.ts` の mock が受け取ったオプションを直接呼んで再現する（`onDragStart`・`onOver`・`onDrop`）。

`web/app/kanban/KanbanView.test.ts`:

```ts
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import { closeCreate, createState } from "../create-state.svelte";
import { selection } from "../selection.svelte";
import { schema as baseSchema, task } from "../tree/fixtures";
import type { DragSource } from "./dnd";

type Get<T> = () => T;
const h = vi.hoisted(() => ({
  core: undefined as unknown,
  toast: vi.fn(),
  go: vi.fn(),
  cards: new Map<string, Get<{ ulid: string; lane: string; onDragStart: () => void }>>(),
  cells: new Map<
    string,
    Get<{ onOver: (s: DragSource | null) => void; onDrop: (s: DragSource) => void }>
  >(),
  dragEnd: undefined as undefined | (() => void),
  ls: {} as Record<string, unknown>,
}));

vi.mock("../context", () => ({ core: () => h.core }));
vi.mock("../router", () => ({
  router: { route: { name: "workspace" }, go: h.go, href: (r: { name: string }) => `/${r.name}` },
  onLinkClick: (e: MouseEvent, path: string) => {
    e.preventDefault();
    h.go(path);
  },
}));
vi.mock("../toast.svelte", () => ({ toast: h.toast }));
// Part A's useListScope is checked on its own; the board is given what it resolves.
vi.mock("../list/scope.svelte", () => ({ useListScope: () => h.ls }));
vi.mock("./dnd", () => ({
  cardDnd: (get: Get<{ ulid: string; lane: string; onDragStart: () => void }>) => () => {
    h.cards.set(`${get().ulid}@${get().lane}`, get);
    return () => {};
  },
  cellDnd:
    (get: Get<{ onOver: (s: DragSource | null) => void; onDrop: (s: DragSource) => void }>) =>
    (el: HTMLElement) => {
      h.cells.set(`${el.dataset.column}@${el.dataset.lane}`, get);
      return () => {};
    },
  boardDragEnd: (onEnd: () => void) => () => {
    h.dragEnd = onEnd;
    return () => {};
  },
  boardAutoScroll: () => () => () => {},
}));

import KanbanView from "./KanbanView.svelte";

const ok = (data: unknown) => ({ data, response: new Response(null, { status: 200 }) });

const schema = {
  ...baseSchema,
  statuses: [
    { name: "todo", label: "未着手", category: "todo", default: true },
    { name: "doing", label: "進行中", category: "doing", default: true },
    { name: "review", label: "レビュー", category: "doing" },
    { name: "done", label: "完了", category: "done", default: true },
    { name: "canceled", label: "中止", category: "canceled", default: true },
  ],
  props: { points: { type: "number" }, priority: { type: "enum", values: ["high", "urgent"] } },
  workflow: { transitions: { done: ["todo"] }, required: { done: ["points"] } },
} as unknown as SchemaView;

type Setup = {
  tasks?: TaskView[];
  stores?: string[];
  schemas?: Record<string, SchemaView>;
  blocked?: string[];
  canceled?: string[];
  loading?: boolean;
  noSchema?: boolean;
  listError?: unknown;
  watchers?: string[];
};

function setup(o: Setup = {}) {
  const tasks = o.tasks ?? [
    task("A", "task", null, { id: "REL-1", title: "調べる", status: "todo" }),
    task("B", "task", null, { id: "REL-2", title: "作る", status: "doing", props: { points: 3 } }),
    task("C", "task", null, { id: "REL-3", title: "確かめる", status: "done" }),
  ];
  const byUlid = new Map(tasks.map((t) => [t.ulid, t]));
  const none = () => () => {};
  const api = {
    PATCH: vi.fn(() => ok({ task: byUlid.get("A"), commit: "c1" })),
    GET: vi.fn(() => ok({ watchers: o.watchers ?? [] })),
  };
  const schemas = o.schemas ?? { shared: schema };
  const stores = o.stores ?? ["shared"];
  // The three lists the board asks for: the tasks, the blocked ones, the canceled ones left out.
  const watch = vi.fn((w: { q: string }) => {
    const side = w.q.endsWith("is:blocked")
      ? "blocked"
      : w.q.endsWith(" status:canceled")
        ? "canceled"
        : "main";
    return {
      ulids: () =>
        side === "blocked"
          ? (o.blocked ?? [])
          : side === "canceled"
            ? (o.canceled ?? [])
            : tasks.map((t) => t.ulid),
      error: () => (side === "main" ? (o.listError ?? null) : null),
      loading: () => side === "main" && (o.loading ?? false),
      dispose() {},
    };
  });
  h.core = {
    api,
    cache: {
      task: (u: string) => byUlid.get(u),
      detail: () => undefined,
      watchList: watch,
      applyWrite: vi.fn(),
      subscribe: none,
    },
    stores: {
      stores: () => stores.map((name) => ({ name })),
      tone: () => "purple",
      workspace: () => undefined,
      workspaces: () => [],
      loaded: () => true,
      subscribe: none,
    },
    schema: {
      get: (s: string) => (o.noSchema ? undefined : schemas[s]),
      load: async (s: string) => schemas[s],
      subscribe: none,
    },
  };
  h.ls = {
    ready: true,
    notFound: false,
    apiBase: "workspace:release",
    cliBase: "workspace:release",
    store: "shared",
    workspace: { name: "release" },
    project: undefined,
    saveBase: "workspace:release",
    schemas: () => [],
  };
  return {
    api,
    apply: (h.core as { cache: { applyWrite: ReturnType<typeof vi.fn> } }).cache.applyWrite,
    watch,
  };
}

const SETTINGS = { columns: "status", swimlane: "none" } as const;

function mountView(over: Record<string, unknown> = {}) {
  const onquery = vi.fn();
  const onsettings = vi.fn();
  const r = render(KanbanView, {
    view: { kind: "workspace", store: "shared", ulid: "W1" },
    query: "-status:canceled",
    onquery,
    settings: SETTINGS,
    onsettings,
    ...over,
  });
  return { onquery, onsettings, ...r };
}

const cell = (container: HTMLElement, column: string, lane = "") =>
  container.querySelector(`ul[data-column="${column}"][data-lane="${lane}"]`) as HTMLElement;
const headNames = (container: HTMLElement) =>
  [...container.querySelectorAll(".head .name")].map((e) => e.textContent);
const cellFor = (column: string, lane = "") => h.cells.get(`${column}@${lane}`)?.();

beforeEach(() => {
  cleanup();
  h.toast.mockReset();
  h.go.mockReset();
  h.cards.clear();
  h.cells.clear();
  h.dragEnd = undefined;
  closeCreate();
  selection.set(null);
});

describe("KanbanView: the board", () => {
  it("makes a column of each status and puts the cards in them; the canceled column waits for a task", async () => {
    setup();
    const { container } = mountView();
    await waitFor(() =>
      expect(headNames(container)).toEqual(["未着手", "進行中", "レビュー", "完了"]),
    );
    expect(within(cell(container, "todo")).getByText("調べる")).toBeInTheDocument();
    expect(within(cell(container, "doing")).getByText("作る")).toBeInTheDocument();
    expect(within(cell(container, "done")).getByText("確かめる")).toBeInTheDocument();
    expect(container.querySelector('.head[data-column="doing"] .count')).toHaveTextContent("1");
  });

  it("shows the canceled column once the query lets a canceled task in", async () => {
    setup({ tasks: [task("X", "task", null, { title: "やめた", status: "canceled" })] });
    const { container } = mountView({ query: "" });
    await waitFor(() => expect(headNames(container)).toContain("中止"));
  });

  it("asks for the tasks of the screen with the query typed, and says how many canceled it left out", async () => {
    const { watch } = setup({ canceled: ["X", "Y"] });
    mountView({ query: "-status:canceled assignee:@me" });
    await waitFor(() => expect(screen.getByText(/3 件 · 中止 2 件を除外/)).toBeInTheDocument());
    const qs = watch.mock.calls.map((c) => (c[0] as { q: string }).q);
    expect(qs).toContain("workspace:release -status:canceled assignee:@me");
    expect(qs).toContain("workspace:release assignee:@me status:canceled");
    expect(qs).toContain("workspace:release is:blocked");
  });

  it("makes the columns of the categories", async () => {
    setup();
    const { container } = mountView({ settings: { columns: "category", swimlane: "none" } });
    await waitFor(() => expect(headNames(container)).toEqual(["未着手", "進行中", "完了"]));
    expect(within(cell(container, "doing")).getByText("作る")).toBeInTheDocument();
  });

  it("puts a card with two assignees in the row of each", async () => {
    setup({
      tasks: [
        task("A", "task", null, { title: "二人で", assignees: ["gh/koki", "gh/sato"] }),
        task("B", "task", null, { title: "一人で", assignees: [] }),
      ],
    });
    const { container } = mountView({ settings: { columns: "status", swimlane: "assignee" } });
    await waitFor(() => expect(screen.getAllByText("二人で")).toHaveLength(2));
    expect(within(cell(container, "todo", "gh/koki")).getByText("二人で")).toBeInTheDocument();
    expect(within(cell(container, "todo", "gh/sato")).getByText("二人で")).toBeInTheDocument();
    expect(within(cell(container, "todo", "#none")).getByText("一人で")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /未割り当て/ })).toBeInTheDocument();
  });

  it("opens the task on a click on the card", async () => {
    setup();
    mountView();
    await fireEvent.click(await screen.findByRole("button", { name: "調べる" }));
    expect(h.go).toHaveBeenCalledWith("/task");
  });

  it("says what is wrong with a card, as the server sees it", async () => {
    setup({
      tasks: [
        task("A", "task", null, {
          title: "終わった",
          status: "done",
          problems: [{ code: "required", field: "points", message: "points is required" }],
        }),
      ],
    });
    mountView();
    expect(await screen.findByText("points 未入力")).toBeInTheDocument();
  });

  it("names the blocker on a card that is blocked", async () => {
    setup({
      tasks: [
        task("A", "task", null, {
          title: "待ち",
          blocked_by: [{ ulid: "Z", id: "REL-9", title: "z" }],
        }),
      ],
      blocked: ["A"],
    });
    mountView();
    expect(await screen.findByText("REL-9 待ち")).toBeInTheDocument();
  });

  it("shows the badge of the store on each card only when there are two stores", async () => {
    setup({ stores: ["shared", "work"] });
    const { container } = mountView();
    await waitFor(() => expect(container.querySelectorAll(".badge").length).toBe(3));
    cleanup();
    setup();
    const one = mountView();
    await waitFor(() => expect(headNames(one.container).length).toBeGreaterThan(0));
    expect(one.container.querySelectorAll(".badge").length).toBe(0);
  });

  it("joins the columns of the stores of the cards on a screen of many stores", async () => {
    const work = {
      ...schema,
      statuses: [
        { name: "todo", label: "ToDo", category: "todo" },
        { name: "wip", label: "作業中", category: "doing" },
        { name: "done", label: "完了", category: "done" },
      ],
    } as unknown as SchemaView;
    setup({
      stores: ["shared", "work"],
      schemas: { shared: schema, work },
      tasks: [
        task("A", "task", null, { title: "共有", store: "shared", status: "todo" }),
        task("B", "task", null, { title: "仕事", store: "work", status: "wip" }),
      ],
    });
    h.ls = { ...h.ls, store: undefined, apiBase: "", cliBase: "", workspace: undefined };
    const { container } = mountView({ view: { kind: "all" } });
    await waitFor(() =>
      expect(headNames(container)).toEqual(["未着手", "進行中", "作業中", "レビュー", "完了"]),
    );
    // No fixed part: the command is the query alone (no space in front of it; a term that starts with - goes after --).
    expect(screen.getByText("$ hh ls -- -status:canceled")).toBeInTheDocument();
  });

  it("keeps to the statuses of the project", async () => {
    const withScoped = {
      ...schema,
      scoped: [{ where: "project:P1", statuses: ["todo", "doing", "done"] }],
    } as unknown as SchemaView;
    setup({ schemas: { shared: withScoped } });
    h.ls = { ...h.ls, project: { title: "リリース", ulid: "P1", store: "shared" } };
    const { container } = mountView({ view: { kind: "project", ulid: "P1" } });
    await waitFor(() => expect(headNames(container)).toEqual(["未着手", "進行中", "完了"]));
  });

  it("clears the selection of the screen before, and its own on leaving", () => {
    setup();
    selection.set("old");
    const { unmount } = mountView();
    expect(selection.ulid).toBeNull();
    selection.set("mine");
    unmount();
    expect(selection.ulid).toBeNull();
  });
});

describe("KanbanView: states", () => {
  it("says it is loading", () => {
    setup({ loading: true, tasks: [] });
    mountView();
    expect(screen.getByText("読み込み中…")).toBeInTheDocument();
  });

  it("waits for the schema before it draws the cards", () => {
    setup({ noSchema: true });
    mountView();
    expect(screen.getByText("読み込み中…")).toBeInTheDocument();
  });

  it("offers to make the first task of an empty workspace", async () => {
    setup({ tasks: [] });
    mountView({ query: "-status:canceled" });
    await fireEvent.click(await screen.findByRole("button", { name: /タスクを作成/ }));
    expect(createState.request).toEqual({ kind: "task", parent: undefined });
  });

  it("offers to clear a filter that matches nothing", async () => {
    setup({ tasks: [] });
    const { onquery } = mountView({ query: "assignee:nobody" });
    await fireEvent.click(await screen.findByRole("button", { name: "フィルタを解除" }));
    expect(onquery).toHaveBeenCalledWith("");
  });

  it("says the workspace is not found", () => {
    setup();
    h.ls = { ...h.ls, notFound: true };
    mountView();
    expect(screen.getByText(/が見つかりません/)).toBeInTheDocument();
  });

  it("shows the error of the query in the filter and no board", () => {
    setup({ tasks: [], listError: new Error("bad query") });
    mountView({ query: "progres:x" });
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
});

describe("KanbanView: the settings", () => {
  it("changes the columns and the swimlane from the two menus", async () => {
    setup();
    const { onsettings } = mountView();
    await fireEvent.click(await screen.findByRole("button", { name: /列: ステータス/ }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "カテゴリ" }));
    expect(onsettings).toHaveBeenLastCalledWith({ columns: "category", swimlane: "none" });
    await fireEvent.click(screen.getByRole("button", { name: /スイムレーン: なし/ }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "プロジェクト" }));
    expect(onsettings).toHaveBeenLastCalledWith({ columns: "status", swimlane: "project" });
  });

  it("shows the command of the screen: hh ls, or the one it is given, and the one that saves the settings", async () => {
    setup();
    mountView({ settings: { columns: "category", swimlane: "assignee" } });
    expect(
      await screen.findByText(/\$ hh ls --store shared "workspace:release -status:canceled"/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /hh view save <名前> --layout kanban --kanban-columns category --swimlane assignee/,
      ),
    ).toBeInTheDocument();
    cleanup();
    setup();
    mountView({ cli: "hh view show 今週 --store shared" });
    expect(await screen.findByText(/\$ hh view show 今週 --store shared/)).toBeInTheDocument();
    expect(screen.queryByText(/\$ hh ls/)).toBeNull();
  });

  it("puts the actions it is given next to the controls", async () => {
    setup();
    const { createRawSnippet } = await import("svelte");
    const actions = createRawSnippet(() => ({
      render: () => '<button type="button">ビューを保存</button>',
    }));
    mountView({ actions });
    expect(await screen.findByRole("button", { name: "ビューを保存" })).toBeInTheDocument();
  });
});

describe("KanbanView: the plus of a column", () => {
  it("opens the create dialog in the status of the column", async () => {
    setup();
    mountView();
    await fireEvent.click(await screen.findByRole("button", { name: "レビュー にタスクを追加" }));
    expect(createState.request).toEqual({ kind: "task", status: "review" });
  });

  it("opens it in the category of a column of categories", async () => {
    setup();
    mountView({ settings: { columns: "category", swimlane: "none" } });
    await fireEvent.click(await screen.findByRole("button", { name: "進行中 にタスクを追加" }));
    expect(createState.request).toEqual({ kind: "task", category: "doing" });
  });
});

describe("KanbanView: dragging a card", () => {
  const dragA = { ulid: "A", lane: "" };

  async function ready() {
    const s = setup();
    const v = mountView();
    await screen.findByText("調べる");
    return { ...s, ...v };
  }

  it("changes the status when the card is dropped on another column", async () => {
    const { api, apply } = await ready();
    cellFor("doing")?.onDrop(dragA);
    await waitFor(() => expect(api.PATCH).toHaveBeenCalled());
    expect(api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "A" }, query: { store: "shared" } },
      body: { status: "doing" },
    });
    await waitFor(() =>
      expect(apply).toHaveBeenCalledWith({ task: expect.anything(), commit: "c1" }),
    );
    expect(h.toast).not.toHaveBeenCalled();
  });

  it("does nothing for a drop on the column the card is in", async () => {
    const { api } = await ready();
    cellFor("todo")?.onDrop(dragA);
    expect(api.PATCH).not.toHaveBeenCalled();
    expect(h.toast).not.toHaveBeenCalled();
  });

  it("goes to the default status of the category on a board of categories", async () => {
    const s = setup();
    mountView({ settings: { columns: "category", swimlane: "none" } });
    await screen.findByText("調べる");
    cellFor("doing")?.onDrop(dragA);
    await waitFor(() => expect(s.api.PATCH).toHaveBeenCalled());
    expect((s.api.PATCH.mock.calls[0] as unknown as [string, { body: unknown }])[1].body).toEqual({
      status: "doing",
    });
  });

  it("frames red the columns the workflow shuts, as soon as the drag starts", async () => {
    const { container } = await ready();
    // C is done; the store lets a done task go back to todo and nowhere else.
    h.cards.get("C@")?.().onDragStart();
    await waitFor(() => expect(cell(container, "doing")).toHaveAttribute("data-drop", "reject"));
    expect(cell(container, "review")).toHaveAttribute("data-drop", "reject");
    expect(cell(container, "todo")).not.toHaveAttribute("data-drop");
    h.dragEnd?.();
    await waitFor(() => expect(cell(container, "doing")).not.toHaveAttribute("data-drop"));
  });

  it("frames a column green over it and says why on a column that is shut; the drop shows the reason and changes nothing", async () => {
    const { api, container } = await ready();
    const dragC = { ulid: "C", lane: "" };
    h.cards.get("C@")?.().onDragStart();
    cellFor("todo")?.onOver(dragC);
    await waitFor(() => expect(cell(container, "todo")).toHaveAttribute("data-drop", "ok"));
    cellFor("doing")?.onOver(dragC);
    expect(await screen.findByRole("status")).toHaveTextContent(
      "「完了」から「進行中」には移せません",
    );
    cellFor("doing")?.onDrop(dragC);
    expect(h.toast).toHaveBeenCalledWith({
      message: expect.stringContaining("「完了」から「進行中」には移せません。"),
    });
    expect(api.PATCH).not.toHaveBeenCalled();
    await waitFor(() => expect(screen.queryByRole("status")).toBeNull());
  });

  it("refuses to cross a swimlane", async () => {
    const s = setup({
      tasks: [task("A", "task", null, { title: "二人で", assignees: ["gh/koki", "gh/sato"] })],
    });
    mountView({ settings: { columns: "status", swimlane: "assignee" } });
    await screen.findAllByText("二人で");
    cellFor("doing", "gh/sato")?.onDrop({ ulid: "A", lane: "gh/koki" });
    expect(h.toast).toHaveBeenCalledWith({
      message: expect.stringContaining("担当者の行をまたぐ移動はできません。"),
    });
    expect(s.api.PATCH).not.toHaveBeenCalled();
    cellFor("doing", "gh/koki")?.onDrop({ ulid: "A", lane: "gh/koki" });
    await waitFor(() => expect(s.api.PATCH).toHaveBeenCalledTimes(1));
  });

  it("asks for the required field first and sends it with the status in one call", async () => {
    const { api } = await ready();
    cellFor("done")?.onDrop(dragA);
    expect(api.PATCH).not.toHaveBeenCalled();
    expect(
      await screen.findByRole("dialog", { name: "REL-1 を完了にするには points が必要です" }),
    ).toBeInTheDocument();
    await fireEvent.input(screen.getByRole("spinbutton", { name: "points" }), {
      target: { value: "5" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "完了にする" }));
    await waitFor(() => expect(api.PATCH).toHaveBeenCalledTimes(1));
    expect(api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "A" }, query: { store: "shared" } },
      body: { status: "done", props: { points: 5 } },
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("changes nothing when the dialog is closed", async () => {
    const { api } = await ready();
    cellFor("done")?.onDrop(dragA);
    await fireEvent.click(await screen.findByRole("button", { name: "やめる" }));
    expect(api.PATCH).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("does not ask when the task has the field already", async () => {
    const { api } = await ready();
    cellFor("done")?.onDrop({ ulid: "B", lane: "" });
    await waitFor(() => expect(api.PATCH).toHaveBeenCalled());
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("shows the words of the server when the write is refused", async () => {
    const { api } = await ready();
    const { ApiError } = await import("../../core/api/client");
    api.PATCH.mockImplementation(() => {
      throw new ApiError({ status: 409, type: "store.locked" });
    });
    cellFor("doing")?.onDrop(dragA);
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
    expect(h.toast.mock.calls[0]?.[0].message).toContain("書き込み中");
  });

  describe("a required field only the detail can fill", () => {
    const needy = {
      ...schema,
      workflow: { transitions: {}, required: { done: ["points", "parent", "watchers"] } },
    } as unknown as SchemaView;

    it("lists them all in the dialog, links to the detail, and does not send", async () => {
      const s = setup({ schemas: { shared: needy } });
      mountView();
      await screen.findByText("調べる");
      cellFor("done")?.onDrop(dragA);
      expect(
        await screen.findByRole("dialog", {
          name: "REL-1 を完了にするには points・親・ウォッチャー が必要です",
        }),
      ).toBeInTheDocument();
      // the watchers were not on the list: the full task was read to know
      expect(s.api.GET).toHaveBeenCalledWith("/api/tasks/{id}", {
        params: { path: { id: "A" }, query: { store: "shared" } },
      });
      await fireEvent.input(screen.getByRole("spinbutton", { name: "points" }), {
        target: { value: "5" },
      });
      expect(screen.getByRole("button", { name: "完了にする" })).toBeDisabled();
      await fireEvent.click(screen.getAllByRole("link", { name: "詳細で入力" })[0] as HTMLElement);
      expect(h.go).toHaveBeenCalledWith("/task");
      expect(s.api.PATCH).not.toHaveBeenCalled();
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    });

    it("drops the field the full task has from the list", async () => {
      setup({ schemas: { shared: needy }, watchers: ["gh/koki"] });
      mountView();
      await screen.findByText("調べる");
      cellFor("done")?.onDrop(dragA);
      expect(
        await screen.findByRole("dialog", { name: "REL-1 を完了にするには points・親 が必要です" }),
      ).toBeInTheDocument();
    });

    it("says why when the full task cannot be read, and sends nothing", async () => {
      const s = setup({ schemas: { shared: needy } });
      const { ApiError } = await import("../../core/api/client");
      s.api.GET.mockImplementation(() => {
        throw new ApiError({ status: 409, type: "store.locked" });
      });
      mountView();
      await screen.findByText("調べる");
      cellFor("done")?.onDrop(dragA);
      await waitFor(() => expect(h.toast).toHaveBeenCalled());
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(s.api.PATCH).not.toHaveBeenCalled();
    });
  });
});
```

```bash
cd web && pnpm vitest run app/kanban/KanbanView.test.ts
```

Expected: FAIL（`Failed to resolve import "./KanbanView.svelte"`）。

- [ ] **Step 16: `KanbanView.svelte` を実装する**

仕組み: `useListScope` が解決する base とクエリで `watchList`（Store の指定は画面の Store、`all` は指定なし）を張り、タスクの Store ごとにスキーマを `schema.load` する。`buildBoard` が列と行を作り、`decideDrop` が各セルへの落とし方を決める。スキーマが全部そろうまでは「読み込み中…」を出す（そろう前に描くと、列の和が途中で変わってカードが動く）。読めなかった Store はカードの上に警告を出し、その Store のスキーマなしで描く（`categoryOf` は todo、`decideDrop` は「少し待ってから」）。ドラッグ中は、持っているカードの行にある、移せない列に赤枠を出す。ポインターが乗ったセルは、移せれば青枠、移せなければ赤枠と理由の帯（`role="status"`）を出し、離すと理由をトーストに出して何も変えない。

`web/app/kanban/KanbanView.svelte`:

```svelte
<script lang="ts">
  import { onDestroy, type Snippet } from "svelte";
  import { type ShowView, type TaskView, unwrap } from "../../core/api/client";
  import { cliLs, cliViewSave, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { SchemaView } from "../../core/schema";
  import type { KanbanSettings } from "../../core/views";
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon from "../icons/Icon.svelte";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import ListBar from "../list/ListBar.svelte";
  import type { ListScope } from "../list/scope";
  import { useListScope } from "../list/scope.svelte";
  import { router } from "../router";
  import { selection } from "../selection.svelte";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import { patchTask } from "../tree/actions";
  import { combine, hasTerm, NO_CANCELED, withoutTerm } from "../tree/filter";
  import { countLabel } from "../tree/flatten";
  import { openBlocker } from "../tree/marks";
  import Avatar from "../ui/Avatar.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import Menu from "../ui/Menu.svelte";
  import { buildBoard, type Column, NO_LANE } from "./board";
  import { boardAutoScroll, boardDragEnd, cardDnd, cellDnd, type DragSource } from "./dnd";
  import { type DropDecision, decideDrop, scopedStatusesOf } from "./drop";
  import KanbanCard from "./KanbanCard.svelte";
  import RequiredDialog from "./RequiredDialog.svelte";

  type Props = {
    view: ListScope;
    query: string;
    onquery: (q: string) => void;
    settings: KanbanSettings;
    onsettings: (s: KanbanSettings) => void;
    cli?: string;
    actions?: Snippet;
  };
  // The prop is not called scope: Biome reads that name as the HTML attribute.
  let { view: scope, query, onquery, settings, onsettings, cli, actions }: Props = $props();

  const c = core();
  const ls = useListScope(() => scope);
  // The selection is one for the whole app: a row of another screen is not a card of this.
  selection.set(null);
  onDestroy(() => selection.set(null));
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));

  // --- the tasks -------------------------------------------------------------
  const q = $derived(combine(ls.apiBase, query).trim());
  type ListHandle = ReturnType<typeof c.cache.watchList>;
  let list = $state<ListHandle | null>(null);
  let blockedList = $state<ListHandle | null>(null);
  let canceledList = $state<ListHandle | null>(null);

  $effect(() => {
    if (!ls.ready) {
      list = null;
      return;
    }
    const h = c.cache.watchList({ q, tree: false, store: ls.store });
    list = h;
    return () => h.dispose();
  });
  $effect(() => {
    if (!ls.ready) {
      blockedList = null;
      return;
    }
    const h = c.cache.watchList({
      q: combine(ls.apiBase, "is:blocked").trim(),
      tree: false,
      store: ls.store,
    });
    blockedList = h;
    return () => h.dispose();
  });
  // The default filter leaves the canceled tasks out; the count says how many that is.
  $effect(() => {
    if (!ls.ready || !hasTerm(query, NO_CANCELED)) {
      canceledList = null;
      return;
    }
    const h = c.cache.watchList({
      q: combine(ls.apiBase, `${withoutTerm(query, NO_CANCELED)} status:canceled`).trim(),
      tree: false,
      store: ls.store,
    });
    canceledList = h;
    return () => h.dispose();
  });

  const tasks = $derived.by(() => {
    cacheRev.current;
    return (list?.ulids() ?? []).flatMap((u) => {
      const t = c.cache.task(u);
      return t ? [t] : [];
    });
  });
  const loading = $derived.by(() => {
    cacheRev.current;
    return !ls.ready || (list?.loading() ?? true);
  });
  const listError = $derived.by(() => {
    cacheRev.current;
    return list?.error() ?? null;
  });
  const blocked = $derived.by(() => {
    cacheRev.current;
    return new Set(blockedList?.ulids() ?? []);
  });
  const canceledCount = $derived.by(() => {
    cacheRev.current;
    return canceledList ? canceledList.ulids().length : null;
  });

  // --- the schemas: the store of each task (and of the screen) -------------------
  const storesNeeded = $derived([
    ...new Set([...tasks.map((t) => t.store), ...(ls.store ? [ls.store] : [])]),
  ]);
  let failed = $state<ReadonlySet<string>>(new Set());
  $effect(() => {
    for (const store of storesNeeded) {
      c.schema.load(store).catch(() => {
        failed = new Set([...failed, store]);
      });
    }
  });
  const schemas = $derived.by(() => {
    schemaRev.current;
    const m = new Map<string, SchemaView>();
    for (const s of storesNeeded) {
      const v = c.schema.get(s);
      if (v) m.set(s, v);
    }
    return m;
  });
  const schemaWaiting = $derived(storesNeeded.some((s) => !schemas.has(s) && !failed.has(s)));
  const storeOrder = $derived.by(() => {
    storesRev.current;
    return c.stores.stores().map((s) => s.name);
  });
  // With two stores or more, each card says which store it is of.
  const manyStores = $derived(storeOrder.length >= 2);

  // --- the board -------------------------------------------------------------------
  const scopedStatuses = $derived.by(() => {
    if (scope.kind !== "project" || !ls.store) return undefined;
    const sc = schemas.get(ls.store);
    return sc ? scopedStatusesOf(sc, scope.ulid) : undefined;
  });
  const board = $derived(
    buildBoard(tasks, {
      mode: settings.columns,
      swimlane: settings.swimlane,
      schemas,
      scopedStatuses,
      storeOrder,
      store: ls.store,
    }),
  );
  // How many tasks a column has: a task with two assignees is in two rows but counts once.
  const counts = $derived(
    board.columns.map(
      (_, i) => new Set(board.lanes.flatMap((l) => l.cells[i] ?? []).map((t) => t.ulid)).size,
    ),
  );

  function isClosed(t: TaskView): boolean {
    const category = schemas.get(t.store)?.statuses?.find((s) => s.name === t.status)?.category;
    return category === "done" || category === "canceled";
  }
  const taskOf = (ulid: string) => c.cache.task(ulid);

  function open(ulid: string) {
    router.go(router.href({ name: "task", ulid }));
  }

  // --- drag and drop ---------------------------------------------------------------------
  let dragging = $state<DragSource | null>(null);
  let over = $state<{ column: string; lane: string; decision: DropDecision } | null>(null);
  let required = $state<{
    task: TaskView;
    status: string;
    fields: string[];
    elsewhere: string[];
    schema: SchemaView;
  } | null>(null);

  function decide(
    source: DragSource,
    column: Column,
    lane: string,
    detail?: ShowView,
  ): DropDecision | null {
    const t = taskOf(source.ulid);
    if (!t) return null;
    return decideDrop({
      task: t,
      column,
      sameLane: source.lane === lane,
      swimlane: settings.swimlane,
      mode: settings.columns,
      schema: schemas.get(t.store),
      detail,
    });
  }

  // The columns the dragged card cannot go to, in its own row: they show a red frame at once.
  const forbidden = $derived.by(() => {
    const out = new Set<string>();
    const d = dragging;
    if (!d) return out;
    for (const col of board.columns) {
      const r = decide(d, col, d.lane);
      if (r && !r.ok && !r.silent) out.add(col.key);
    }
    return out;
  });

  function markOf(col: Column, lane: string): "ok" | "reject" | null {
    if (over && over.column === col.key && over.lane === lane) {
      if (over.decision.ok) return "ok";
      return over.decision.silent ? null : "reject";
    }
    return dragging && dragging.lane === lane && forbidden.has(col.key) ? "reject" : null;
  }

  function setOver(col: Column, lane: string, source: DragSource | null) {
    if (!source) {
      if (over?.column === col.key && over.lane === lane) over = null;
      return;
    }
    const decision = decide(source, col, lane);
    over = decision ? { column: col.key, lane, decision } : null;
  }

  // The full task, for the required fields the list does not carry (watchers and repos).
  async function detailOf(t: TaskView): Promise<ShowView> {
    const have = c.cache.detail(t.ulid);
    if (have) return have;
    return unwrap(
      c.api.GET("/api/tasks/{id}", { params: { path: { id: t.ulid }, query: { store: t.store } } }),
    );
  }

  async function dropOn(col: Column, lane: string, source: DragSource) {
    over = null;
    dragging = null;
    const t = taskOf(source.ulid);
    let d = decide(source, col, lane);
    if (!t || !d) return;
    try {
      if (d.ok && d.elsewhere.some((f) => f === "watchers" || f === "repos")) {
        d = decide(source, col, lane, await detailOf(t)) ?? d;
      }
    } catch (err) {
      toast({ message: errorMessage(err) });
      return;
    }
    if (!d.ok) {
      if (!d.silent) toast({ message: `${d.title}。${d.reason}` });
      return;
    }
    const schema = schemas.get(t.store);
    if ((d.fill.length > 0 || d.elsewhere.length > 0) && schema) {
      required = { task: t, status: d.status, fields: d.fill, elsewhere: d.elsewhere, schema };
      return;
    }
    void patchTask(c, t, { status: d.status }).catch((err) =>
      toast({ message: errorMessage(err) }),
    );
  }

  const refusal = $derived(
    over && !over.decision.ok && !over.decision.silent ? over.decision : null,
  );

  // --- the buttons ---------------------------------------------------------------------------
  // A column's "+" opens the create dialog in that status (in that category, by category).
  function addTo(col: Column) {
    openCreate(
      col.status !== null
        ? { kind: "task", status: col.status }
        : { kind: "task", category: col.key },
    );
  }

  let menu = $state<"columns" | "swimlane" | null>(null);
  let columnsButton = $state<HTMLButtonElement | null>(null);
  let swimlaneButton = $state<HTMLButtonElement | null>(null);
  const COLUMN_ITEMS = [
    { value: "status", label: "ステータス" },
    { value: "category", label: "カテゴリ" },
  ];
  const SWIMLANE_ITEMS = [
    { value: "none", label: "なし" },
    { value: "assignee", label: "担当者" },
    { value: "project", label: "プロジェクト" },
  ];
  const labelIn = (items: { value: string; label: string }[], v: string) =>
    items.find((i) => i.value === v)?.label ?? v;

  function pick(value: string) {
    const which = menu;
    menu = null;
    if (which === "columns")
      onsettings({ ...settings, columns: value as KanbanSettings["columns"] });
    if (which === "swimlane") {
      onsettings({ ...settings, swimlane: value as KanbanSettings["swimlane"] });
    }
  }

  // --- reading the screen ------------------------------------------------------------------------
  const lsLine = $derived(
    cli ??
      cliLs(
        combine(ls.cliBase, query).trim(),
        false,
        scope.kind === "workspace" ? ls.store : undefined,
      ),
  );
  // The settings of this screen as the command that saves them in a view.
  const saveLine = $derived(
    cliViewSave({
      name: new Placeholder("名前"),
      body: {
        layout: "kanban",
        kanban_columns: settings.columns,
        swimlane: settings.swimlane,
      },
    }),
  );
  const noTasks = $derived(
    withoutTerm(query, NO_CANCELED).trim() === "" && !(canceledCount && canceledCount > 0),
  );
  const crumbTitle = $derived(
    scope.kind === "project" ? (ls.project?.title ?? "") : (ls.workspace?.name ?? ""),
  );
  const emptyScope = $derived<"project" | "workspace">(
    scope.kind === "project" ? "project" : "workspace",
  );
  // Spread, because Biome reads a prop called scope as the HTML attribute.
  const emptyProps = $derived({ scope: emptyScope, name: crumbTitle });
  const notFoundProps = $derived({
    scope: emptyScope,
    name: scope.kind === "workspace" ? scope.ulid : "",
  });
  const showBoard = $derived(!loading && tasks.length > 0 && !schemaWaiting);
</script>

<section class="kanban" aria-label="カンバン">
  <ListBar
    base={ls.cliBase}
    applied={query}
    count={countLabel(tasks.length, canceledCount)}
    cli={lsLine}
    error={listError}
    onchange={onquery}
    {actions}
  >
    {#snippet controls()}
      <button
        type="button"
        class="ctl"
        bind:this={columnsButton}
        aria-haspopup="menu"
        aria-expanded={menu === "columns"}
        onclick={() => (menu = menu === "columns" ? null : "columns")}
      >
        列: {labelIn(COLUMN_ITEMS, settings.columns)}
        <Icon name="chevron-down" size={12} />
      </button>
      <button
        type="button"
        class="ctl"
        bind:this={swimlaneButton}
        aria-haspopup="menu"
        aria-expanded={menu === "swimlane"}
        onclick={() => (menu = menu === "swimlane" ? null : "swimlane")}
      >
        スイムレーン: {labelIn(SWIMLANE_ITEMS, settings.swimlane)}
        <Icon name="chevron-down" size={12} />
      </button>
    {/snippet}
  </ListBar>

  <div class="body">
    {#if ls.notFound}
      <Empty
        variant="notfound"
        {...notFoundProps}
        onHome={() => router.go(router.href({ name: "home" }))}
      />
    {:else if loading || (tasks.length > 0 && schemaWaiting)}
      <p class="note">読み込み中…</p>
    {:else if tasks.length === 0}
      {#if !listError}
        {#if scope.kind !== "all" && noTasks}
          <Empty
            variant="empty"
            {...emptyProps}
            onCreate={() =>
              openCreate({
                kind: "task",
                parent: scope.kind === "project" ? scope.ulid : undefined,
              })}
          />
        {:else}
          <Empty
            variant="nomatch"
            note={hasTerm(query, NO_CANCELED) ? "中止を除外しています" : undefined}
            onClear={query.trim() === "" ? undefined : () => onquery("")}
          />
        {/if}
      {/if}
    {/if}

    {#if failed.size > 0 && showBoard}
      <p class="note" role="alert">
        {[...failed].join("、")}
        の schema.yaml を読み込めていません。列は読み込めたものだけです
      </p>
    {/if}

    {#if showBoard}
      <section
        class="scroller"
        aria-label="カンバンの盤面"
        {@attach boardAutoScroll()}
        {@attach boardDragEnd(() => {
          dragging = null;
          over = null;
        })}
      >
        <div class="board" style:--cols={board.columns.length}>
          <div class="heads">
            {#each board.columns as col, i (col.key)}
              <div class="head" data-column={col.key}>
                <StatusIcon status={col.key} category={col.category} />
                <span class="name">{col.label}</span>
                <span class="count">{counts[i]}</span>
                {#if !col.unknown}
                  <button
                    type="button"
                    class="add"
                    aria-label="{col.label} にタスクを追加"
                    onclick={() => addTo(col)}
                  >
                    <Icon name="plus" size={14} />
                  </button>
                {/if}
              </div>
            {/each}
          </div>

          {#each board.lanes as lane (lane.key)}
            <section class="lane" aria-label={lane.label || undefined}>
              {#if settings.swimlane !== "none"}
                <h3 class="lane-head">
                  {#if settings.swimlane === "assignee" && lane.key !== NO_LANE}
                    <Avatar name={lane.key} />
                  {/if}
                  <span title={lane.key !== NO_LANE ? lane.key : undefined}>{lane.label}</span>
                  <span class="count">
                    {new Set(lane.cells.flat().map((t) => t.ulid)).size}
                  </span>
                </h3>
              {/if}
              <div class="row">
                {#each board.columns as col, i (col.key)}
                  <ul
                    class="cell"
                    aria-label="{col.label}{lane.label ? ` · ${lane.label}` : ""}"
                    data-column={col.key}
                    data-lane={lane.key}
                    data-drop={markOf(col, lane.key) ?? undefined}
                    {@attach cellDnd(() => ({
                      onOver: (s) => setOver(col, lane.key, s),
                      onDrop: (s) => dropOn(col, lane.key, s),
                    }))}
                  >
                    {#each lane.cells[i] ?? [] as t (t.ulid)}
                      <KanbanCard
                        task={t}
                        schema={schemas.get(t.store)}
                        blockedBy={openBlocker(t, taskOf, isClosed, blocked.has(t.ulid))}
                        tone={manyStores ? c.stores.tone(t.store) : undefined}
                        dragging={dragging?.ulid === t.ulid && dragging.lane === lane.key}
                        onopen={() => open(t.ulid)}
                        dnd={cardDnd(() => ({
                          ulid: t.ulid,
                          lane: lane.key,
                          onDragStart: () => (dragging = { ulid: t.ulid, lane: lane.key }),
                        }))}
                      />
                    {/each}
                  </ul>
                {/each}
              </div>
            </section>
          {/each}
        </div>
      </section>

      {#if refusal && !refusal.ok}
        <aside class="refusal" role="status">
          <strong>{refusal.title}</strong>
          <p>{refusal.reason}</p>
        </aside>
      {/if}
    {/if}
  </div>

  <footer class="keys">
    <span>ドラッグでステータスを変更（遷移の制約に合わない列は赤枠）</span>
    <span class="save"><CliLine command={saveLine} /></span>
  </footer>
</section>

{#if menu === "columns"}
  <Menu
    anchor={columnsButton}
    items={COLUMN_ITEMS}
    label="列"
    selected={settings.columns}
    onselect={pick}
    onclose={() => (menu = null)}
  />
{:else if menu === "swimlane"}
  <Menu
    anchor={swimlaneButton}
    items={SWIMLANE_ITEMS}
    label="スイムレーン"
    selected={settings.swimlane}
    onselect={pick}
    onclose={() => (menu = null)}
  />
{/if}

{#if required}
  <RequiredDialog
    task={required.task}
    status={required.status}
    fields={required.fields}
    elsewhere={required.elsewhere}
    schema={required.schema}
    onsubmit={async (body) => {
      if (required) await patchTask(c, required.task, body);
    }}
    onclose={() => (required = null)}
  />
{/if}

<style>
  .kanban {
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }
  .ctl {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    height: 2.25rem;
    padding: 0 var(--space-10);
    border: var(--hairline) solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink-2);
    font-size: var(--fs-13);
    white-space: nowrap;
    cursor: pointer;
  }
  .ctl:hover {
    background: var(--hover);
  }
  .body {
    position: relative;
    flex: 1 1 auto;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .note {
    margin: 1rem 1.75rem;
    color: var(--muted);
  }
  .scroller {
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto;
    padding: 0 1.75rem 1rem;
  }
  .board {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    width: max-content;
    min-width: 100%;
  }
  .heads,
  .row {
    display: grid;
    grid-template-columns: repeat(var(--cols), 17rem);
    gap: var(--space-10);
  }
  .heads {
    position: sticky;
    top: 0;
    z-index: 2;
    padding: var(--space-4) 0;
    background: var(--surface);
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    min-width: 0;
    padding: 0 var(--space-4);
  }
  .name {
    font-size: var(--fs-13);
    font-weight: 600;
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .count {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .add {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    margin-left: auto;
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--muted);
    cursor: pointer;
  }
  .add:hover {
    background: var(--hover);
    color: var(--ink);
  }
  .lane {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .lane-head {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    margin: 0;
    font-size: var(--fs-13);
    font-weight: 600;
    color: var(--ink-2);
  }
  .cell {
    margin: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    min-height: 5rem;
    padding: var(--space-8);
    border-radius: var(--radius-10);
    background: var(--sunken);
  }
  .cell[data-drop="ok"] {
    background: var(--accent-faint);
    box-shadow: inset 0 0 0 calc(var(--hairline) * 1.5) var(--accent);
  }
  .cell[data-drop="reject"] {
    background: var(--danger-weak);
    box-shadow: inset 0 0 0 calc(var(--hairline) * 1.5) var(--danger);
  }
  .refusal {
    /* Over the board, not in the flow: showing it must not resize the board. */
    position: absolute;
    right: 1.75rem;
    bottom: 0.5rem;
    left: 1.75rem;
    z-index: 3;
    pointer-events: none;
    padding: 0.625rem 0.875rem;
    border: var(--hairline) solid var(--danger);
    border-radius: var(--radius-8);
    background: var(--danger-weak);
    color: var(--ink);
  }
  .refusal strong {
    color: var(--danger);
  }
  .refusal p {
    margin: 0.25rem 0 0;
    font-size: 0.78rem;
    color: var(--ink-2);
  }
  .keys {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.25rem 1.125rem;
    padding: 0.625rem 1.75rem;
    border-top: var(--hairline) solid var(--line);
    color: var(--muted);
    font-size: 0.75rem;
  }
  .save {
    margin-left: auto;
    min-width: 0;
  }
</style>
```

```bash
cd web && pnpm vitest run app/kanban/KanbanView.test.ts
```

Expected: PASS（36 件）。

- [ ] **Step 17: 全体を確かめてコミットする**

```bash
cd web && pnpm fix && pnpm check && pnpm exec biome ci . && pnpm test
```

Expected: `pnpm check` は警告 0、`biome ci` は指摘 0、`pnpm test` は全件 PASS。E2E はこの Task では流さない（画面はまだ App から開けない。Task 7 の結線のあとで、カンバンの移動 E2E を足して流す）。

```bash
git add web/app/kanban
git commit -m "feat(web): カンバンの画面（カード・列とスイムレーン・ドラッグ・必須の項目のダイアログ）" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

**Task 4 で決めたこと**（plan 末尾の「決めたこと」に載せる）:

1. **列の和（D4）の作り方**: ステータスの列は、表示中のタスクの Store と、1 Store の画面ならその Store のスキーマのステータスの和。名前が同じなら1列で、表示名とカテゴリは Store の順で最初に定義した Store のもの。並びは `sort:status` と同じ（カテゴリ → そのスキーマの `statuses` 全体での位置 → Store の順。Store の順は `GET /api/stores`）で、その計算は `web/core/status-order.ts` の1か所にある（`buildBoard` も Task 5 のテーブルの並びもこれを使う）。タスクのない Store の列は出さない。中止のカテゴリの列は、1 Store の画面でも、そのステータスのタスクがあるときだけ。既定のフィルタ `-status:canceled` では中止のタスクが返らないので列は出ず、フィルタ欄を空にすると出る（ツリーの「中止 n 件を除外」と同じ数を出すため、中止だけの問い合わせを1本余分に張る）。
2. **カテゴリの列**: 未着手・進行中・完了は常に出し、中止はそのカテゴリのタスクがあるときだけ。タスクのカテゴリは**そのタスクの Store のスキーマ**で決める。ドロップしたカードは、そのタスクの Store のスキーマの、そのカテゴリの既定のステータス（`default: true`、なければ先頭）になる。そのカテゴリのステータスがない Store のタスクは移せない（理由を出す）。
3. **スイムレーン（細部 37）**: 担当者は名前順（login は `/` のあと）、複数担当のカードは各担当者の行に出す（列の件数は1回と数える）。プロジェクトは届いた順。担当者なし・プロジェクトなしの行は最後で、タスクがなければ行を出さない。スイムレーンなしの行は見出しを出さない。
4. **列の中の順**: 届いた順（サーバーは rank と ULID）。ドラッグで並べ替えない（細部 38）。
5. **ドラッグ（細部 38）**: 移せるかは `decideDrop` が決める。順に、(1) 別の行へのドロップは拒否（担当者・プロジェクトが変わるため。同じ列でも拒否）、(2) 同じ列は黙って何もしない、(3) 列がどのスキーマにもないステータスなら拒否、(4) スキーマが読めていなければ拒否、(5) 移り先がそのタスクの Store のスキーマになければ拒否、(6) 遷移の制約に合わなければ拒否、(7) 通れば必須の項目の空を調べる。拒否は離したときに「<題>。<理由>」のトーストで、データは変えない。ドラッグ中は、持っているカードの行にある拒否される列を赤枠にし、ポインターが乗ったセルは通れば青枠、拒否なら赤枠と理由の帯を出す（ツリーの `refusal` と同じ流儀）。
6. **遷移の制約と必須の項目は、どの画面でもそのタスクの scoped を足す**。D3 の「scoped の `statuses` を使うのはプロジェクトの画面だけ」は、どの列を出すかの話で、遷移と必須はサーバーが検査するものと同じ（Store と scoped の和）にする。どの scoped を足すかは、サーバーが `TaskView.scope`（そのタスクに効く上書きを持つ祖先の ULID。タスク自身がコンテナで上書きを持つならそのタスク自身。書き込みの検査の `schema.For` と同じ祖先の並びから選ぶ。なければ省く）で返し、`drop.ts` は `where === "project:<scope>"` で引く（利用者の決定 2026-11-03）。最初は `TaskView.project`（最も近い kind が `project` の祖先）で引いていたが、サーバーは自分自身がコンテナのタスクと近い祖先（epic など）の上書きを先に見るので、epic の上書きが必須の項目を足すとドロップが 422 になり、プロジェクトが禁じる遷移を epic の上書きが許すと画面が許されている移動を断った。列を決める `scopedStatusesOf` は表示の話なので、見ているプロジェクトの ULID のまま。
7. **必須の項目（細部 39・利用者の決定 2026-11-03: 事前に止めて詳細へ案内する）**: 移る先の必須の項目（Store と scoped の和）が欠けていれば、離したときにダイアログを開き、欠けている項目を全部並べる。担当者・ラベル・スキーマのプロパティ（タスク参照の型を除く）はその場で入力でき、ステータスと一緒に1回の `PATCH`（1コミット）で送る。親・ブロック・ウォッチャー・リポジトリ・タスク参照・スプリント・リリース・リンク（どれが必須になりうるかは `internal/schema/check.go` の `builtinFilled`）は、詳細でしか入力できないので「詳細で入力」のリンク（タスク詳細を開き、ダイアログを閉じる）を出し、移動のボタンを無効にする。詳細で入れたあとにもう一度ドラッグする。サーバーに 422 を起こさせない。ウォッチャーとリポジトリは一覧にないので、必須のときだけ `GET /api/tasks/{id}` でフルのタスクを読んで判定する（読めなければトーストを出して何も送らない）。スプリント・リリース・リンクは API に出ない（フェーズ 6）ので、必須にしている Store では常に「欠けている」扱いになる。送って失敗したらダイアログの中にサーバーの文言を出して開いたままにする。足元は `hh set <ID> status=<…> <項目>=<値>`。
8. **カード（細部 40・41）**: 種類・ID・Store のバッジ（Store が2つ以上のときだけ）・points（`props.points` が数のとき「N pt」）・タイトル・警告（`problems` の先頭か「壊れています」。フロントで必須を判定しない）・ブロック中（ツリーと同じ `is:blocked` の問い合わせと `openBlocker`）・priority・チェックリスト・子（`progress.total`）・期限・担当者（3人まで、残りは「+n」）。PR・WIP 上限・「完了」列の「ほか n 件」は作らない（全件出す）。
9. **「＋」（細部 42）**: 列のステータス（カテゴリ列ではカテゴリ）で `openCreate` する。親とワークスペースは作成ダイアログの既存の規則（`resolveCreateContext`）に任せる。プロジェクトの画面ではそのプロジェクトの下、ワークスペースの画面ではそのワークスペースのルート、ビュー・クエリの画面（`all`）では最初のワークスペースで、2つ以上あれば選ばせる。スキーマにないステータスの列には「＋」を出さない。
10. **仮想化しない**: TanStack Virtual は4d のツリーとテーブル（Task 5）だけ。カンバンは全件を DOM に置く（細部 41 の「全件出す」）。カードが数千になる運用が出たら、そのとき考える。
11. **足元の行（D10・細部 49）**: フィルタ欄の `hh` の行は `cli` があればそれ（ビューの画面は `hh view show …`）、なければ `hh ls <クエリ>`（ワークスペースの画面だけ `--store` を付ける。ツリーと同じ）。盤面の足元には、列とスイムレーンの設定を保存するコマンド `hh view save <名前> --layout kanban --kanban-columns … --swimlane …` を出す。`--query` は付けない（付けないと保存済みの値のままなので、既存のビューの設定を直すコマンドとして正しい。ビューの新規保存のクエリは Task 6 の保存のダイアログが D8 の形で組む）。
12. **読み込み中と空（細部 58）**: ツリーに揃える。スキーマが全部そろうまでは「読み込み中…」（そろう前に描くと列の和が変わってカードが動く）。タスクがなければ、フィルタが空（中止の除外だけ）で `all` でなければ「まだタスクがありません」（プロジェクトの画面では親にプロジェクトを渡して作成）、そうでなければ「一致するタスクはありません」と「フィルタを解除」。クエリの誤りはフィルタ欄の赤字（「もしかして」は Task 7）。ワークスペース・プロジェクトが見つからなければ `Empty` の notfound。
13. **細部 36 を直す（利用者の決定 2026-11-03）**: 「scoped の `statuses` があるプロジェクトの画面ではそれだけ」を、「scoped の `statuses` と、タスクのいる表示外のステータス」に直す。scoped の `statuses` は表示のための絞り込みで、書き込みの検査には使われない（外のステータスのタスクは作れる）ので、そのままではタスクが盤面から消える。タスクのいるステータスの列は残す（scoped の外のものはスキーマの表示名で、どのスキーマにもないステータスは名前のまま末尾に出す。後者はドロップ先にならず「＋」もない）。spec・概要 plan の細部 36 もこの文言に直す。
14. **テストの境目**: `KanbanView.test.ts` は `useListScope` と `dnd.ts` を差し替える。ドラッグは `dnd.ts` が受け取ったオプション（`onDragStart`・`onOver`・`onDrop`）を呼んで再現する。Pragmatic DnD そのものは E2E（Task 7）で確かめる。カードのタイトルは `<button>` で、その上で押して始めるドラッグは、ブラウザによっては始まらないことがある（`draggable` の中のフォーム部品。Pragmatic DnD の文書には記載がなく、Chromium の E2E でだけ確かめる。Playwright は Chromium だけを使う）。Firefox などで困ったら、カードを `<ul role="listbox">` の中の `<li role="option" tabindex="0">` にして、`onclick` と Enter を `li` に付ける作りに替える（重ね貼りの `::after` がなくなる）。

### Task 5: テーブル（列の選択・セルの編集・並び替え・グループ・仮想化）

概要 plan の細部 43〜49（テーブル）と 58（読み込み中と空の状態）を作る。TanStack Table 9 で列・見出し・並び替えの状態・並べ替えた行を持ち、グループの見出しは並べ替えのあとに自前で行の間へ差して平らな配列にし、TanStack Virtual で仮想化する。ビューの保存・レイアウトの切り替え・「ビューを保存」のボタンはこの部品の外（Task 6）。この Task は部品 `TableView` と、その中の純関数（列・並び替えの値・グループ）と、セル `TableCell` だけを作り、画面への入口は出さない（Task 7 が結線する）。

**前提**:
- Task 2 が入っている。`@tanstack/svelte-table`（9.3.1、exact）が `web/package.json` にあり、`web/core/views.ts` の `TableSettings`、`web/app/list/scope.ts` の `ListScope`、`web/app/list/scope.svelte.ts` の `useListScope`、`web/app/list/ListBar.svelte`、`web/core/cli.ts` の `cliViewSave` がある
- Task 4 が入っている。sort:status の順は Task 4 の `web/core/status-order.ts`（`statusRank(schema, status, storeAt)`）を使う。`TreeView`・`KanbanView` そのものには依存しない。`TableView` が使う部 A のものは上の 5 つだけ

**実物を読んで確かめたこと**（コードはこれに合わせてある）:
- TanStack Table 9 は `createTable({ features: tableFeatures({...}), columns, data })` で、並び替えは `rowSortingFeature` と `sortedRowModel: createSortedRowModel()` を `tableFeatures` に入れたときだけ使える。`columns` と `data` はどちらも getter で渡すと追従する（`createTable` が `$effect.pre` で `setOptions` し直す。複製で、列の配列と `data` を差し替えて見出しと行が変わることを確かめた）。並び替えの状態は `state: { get sorting() {…} }` と `onSortingChange` で自分の `$state` に持たせる
- 並び替えの値の比べ方は、列の定義の `sortFn` に `constructSortFn({ sort: (a, b) => … })` を渡せば登録が要らない。`accessorFn` が返す値を `sort` が受ける。`sortUndefined: "last"` で値のない行は昇順でも降順でも最後になる。最初の押しが降順になるのを防ぐには `createTable` の `sortDescFirst: false`（数の列は既定で降順から始まる）。押すごとに 昇順 → 降順 → 解除（クエリの順）
- `getRowId` を渡さないと行の `id` は添字になる。`getRowId: (t) => t.ulid` にする。`table.getRowModel().rows` が並べ替えたあとの行で、`row.original` がタスク
- 行の仮想化は、`@tanstack/svelte-virtual` の `createVirtualizer`（ストア）を `virtualizer.subscribe` で読み、`setOptions` で `count` と `getScrollElement` を渡す流儀（TreeView と同じ）。`estimateSize` と `getItemKey` は最初のオプションに渡し、`setOptions` は前のオプションに重ねる（`@tanstack/svelte-virtual` の `createVirtualizerBase`）。行の高さが 2 種類（行と見出し）あるので、`getItemKey` を渡して、見出しが増えても測った高さが別の行に移らないようにする
- TanStack の「仮想化の例」は `<table>` の要素に `display: grid`／`position: absolute` を当てる。この plan も実際の `<table>`・`<thead>`・`<tbody>`・`<tr>`・`<th scope="col">`・`<td>` を使い、Biome の `useSemanticElements`・`useFocusableInteractive`（`div` に `role="gridcell"` を付けると出る）を避ける。複製で Chromium（Playwright）に出して、見出しの固定、列のそろい、400 行の仮想化、グループの見出しを目で確かめた
- `detail/FieldInput.svelte` は、文字の入力で `onedit`、blur で `oncommit`、選択・日付・チップは選んだ時点で `onedit` と `oncommit` を続けて呼ぶ。`SaveController.edit(ulid, store, field, value)` が 2 秒の待ちを張り、`flush(ulid)` がすぐ送る。つまりセルは `onedit` で `save.edit`、`oncommit` で `save.flush` につなぐだけでよい。`save.editing(ulid, field)` は応答のない編集があるかで、`FieldInput` の `pending` に渡す（入力中に外からの変更で文字が書き換わらない）。`save.state(ulid)` は `idle`／`saving`／`saved`／`error`（`message` つき）。`PatchField` は `title|kind|status|assignees|labels|props.<名前>` など
- セルの種類ごとの入力は 4d のものを使う。ステータスは `statusOptions(schema)`（`prop-rows.ts`）、種類は `canParent`・`parentRule`（`core/schema.ts`、詳細パネルと同じ規則で、親の種類が分からないときは何も除かない）、プロパティは `PropField`（`propEditorOf` の型で選ぶ。`PropRow` を組んで渡す）。`task_ref` のプロパティは詳細パネルでも関連の節が受け持つので、テーブルでは読み取りだけで ID を並べる
- `internal/view` の `Builtins` は `id,title,kind,status,assignee,label,parent,created,updated` の 9 個。`blocked_by` は列に選べない（`hh view save --columns blocked_by` は `view.bad_column`）。`MissingColumns` のコメントのとおり、どのスキーマにもない列は GUI が読み飛ばす
- `GET /api/schema` の `props` はオブジェクトなので定義順が分からない。列の候補は名前順にする（詳細パネルの `propRows` も名前順）
- `ui/Menu.svelte` は選んでも閉じない（閉じるのは呼ぶ側）ので、列の選択のチェックボックスは `item` スニペットで描く複数選択のメニューにできる
- `Empty` の `empty` の文言は「このプロジェクト／このワークスペースの中で `hh add` しても…」なので、全 Store の画面（`view.kind === "all"`）には使わない（その画面は一致なしの `nomatch` だけ）
- `FilterBar`／`ListBar` の `count` は `countLabel(matched, canceledCount)`（`tree/flatten.ts`）。ツリーと同じに、フィルタに `-status:canceled` があるときは中止の件数を別の `watchList` で数える
- `status-order.ts` は Task 4 の Step 3 の実物を複製にそのまま置いて確かめた（この Task では作らない）
- 複製の確認: `pnpm fix`、`pnpm check`（警告 0）、`pnpm exec biome ci --vcs-enabled=false .`、`pnpm vitest run`（94 ファイル 917 件）が通る。全件の 1 回目に `app/tree/TreeView.test.ts` の「lets a late wait for a row give way to a newer one」が 1 件落ちた（`AssertionError: expected null to be 'M199'`、全件を並列（jsdom を 94 回）で流している最中。単独の実行と全件の再実行は通る。この Task の変更とは無関係の既存のタイミングの揺れ）

**Files:**
- Create: `web/app/table/columns.ts`、`web/app/table/columns.test.ts`
- Create: `web/app/table/group.ts`、`web/app/table/group.test.ts`
- Create: `web/app/table/TableCell.svelte`
- Create: `web/app/table/TableView.svelte`、`web/app/table/TableView.test.ts`

**Interfaces:**
- Consumes:
  - Task 2: `ListScope`（`web/app/list/scope.ts`）、`useListScope(get: () => ListScope)` の `ready`・`notFound`・`apiBase`・`cliBase`・`store`・`workspace`・`project`・`saveBase`・`schemas(stores: string[]): SchemaView[]`（読み込み済みのものを `stores` の順で返し、`c.schema` の変化で読み直される）、`ListBar.svelte`（props `{ base; applied; count; cli; error; onchange; controls?; actions? }`）、`TableSettings`（`web/core/views.ts`）、`cliViewSave`・`Placeholder`（`web/core/cli.ts`）
  - Task 4: `web/core/status-order.ts` の `statusRank(schema: SchemaView | undefined, status: string, storeAt: number): number`（`storeAt` は `GET /api/stores` の順での位置。ここでは `ctx.storeOrder.indexOf(t.store)`）
  - 4d の core: `TaskCache.watchList`・`task`・`subscribe`、`SaveController.edit`・`flush`・`editing`・`state`・`subscribe`、`SchemaCache.get`・`load`・`subscribe`、`StoreDirectory.stores`・`tone`・`subscribe`、`canParent`・`parentRule`・`propEditorOf`・`SchemaView`、`PatchField`、`formatDate`、`cliLs`、`TaskView`
  - 4d の app: `core()`、`useSubscribe`、`openCreate`、`router.href`・`router.go`・`onLinkClick`、`Empty`、`FieldInput`・`PropField`・`statusOptions`・`PropRow`・`Option`（`detail/`）、`combine`・`hasTerm`・`withoutTerm`・`NO_CANCELED`（`tree/filter`）、`countLabel`・`VIRTUALIZE_AT`（`tree/flatten`）、`Button`・`CliLine`・`IdLabel`・`StoreBadge`・`Menu`（`ui/`）、`Icon`・`StatusIcon`
- Produces:
  - `web/app/table/columns.ts`: `type ColumnSpec = { id: string; label: string; readonly: boolean; width: number }`（`width` は rem）、`STORE_COLUMN`（`id: "@store"`。複数 Store の画面の先頭の列で、ビューの列ではない）、`isBuiltinColumn(id)`、`columnCandidates(schemas: SchemaView[]): ColumnSpec[]`（組み込み 9 個 → スキーマのプロパティの和を名前順。`blocked_by` と組み込みと同じ名前は候補にしない）、`shownColumns(selected: string[], candidates: ColumnSpec[]): ColumnSpec[]`（保存した順。どの候補にもない名前と重複は出さない）、`toggleColumn(selected, id, on): string[]`（足すときは末尾。出さない名前は残す）、`patchFieldOf(spec): PatchField | null`（読み取りだけの列は null）、`type SortKey`、`type SortCtx = { schemaOf; taskOf; storeOrder }`、`sortKey(t, id, ctx): SortKey`、`compareKeys(a, b): number`
  - `web/app/table/group.ts`: `type GroupBy`（`TableSettings["groupBy"]`）、`ROW_HEIGHT = 40`・`GROUP_HEIGHT = 36`、`type GroupHead`・`type TableItem = GroupHead | { type: "row"; key: string; task: TaskView }`、`groupTasks(tasks, by, ctx): TableItem[]`
  - `web/app/table/TableCell.svelte`: props `{ task; col: ColumnSpec; schema: SchemaView | undefined; tone: StoreTone; pending: boolean; taskOf; onedit: (field: PatchField, value: unknown) => void; oncommit: () => void }`
  - `web/app/table/TableView.svelte`: props `{ view: ListScope; query: string; onquery: (q: string) => void; settings: TableSettings; onsettings: (s: TableSettings) => void; cli?: string; actions?: Snippet }`。画面の状態（`query`・`settings`）は持たない。持つのは見出しの並び替え（保存しない）だけ
  - E2E（Task 7）が使うアクセシブルな名前: セルの入力は `タイトル: <ID>`・`ステータス: <ID>`（選択のボタン）・`担当: <ID>`・`ラベル: <ID>`・`種類: <ID>`・`<プロパティのラベル>: <ID>`、列の見出しは `columnheader`（`aria-sort`）の中のボタン、設定は `グループ: <名前>`・`列 <数>` のボタン（開くメニューは `グループ`・`列`）、保存に失敗した行の `保存に失敗しました。やり直す: <メッセージ>` のボタン

---

- [ ] **Step 1: 列の純関数のテストを書く（失敗する）**

`web/app/table/columns.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { SchemaView } from "../../core/schema";
import { schema as base, task } from "../tree/fixtures";
import {
  columnCandidates,
  compareKeys,
  patchFieldOf,
  type SortCtx,
  shownColumns,
  sortKey,
  toggleColumn,
} from "./columns";

const withProps = (props: Record<string, unknown>): SchemaView =>
  ({ ...base, props }) as unknown as SchemaView;

const shared = withProps({
  due: { type: "date", label: "期限" },
  points: { type: "number" },
  area: { type: "multi_enum", values: ["backend", "frontend"] },
  relates: { type: "task_ref", label: "関連" },
  blocked_by: { type: "task_ref" },
  status: { type: "text" },
});
const me = withProps({
  area: { type: "multi_enum", label: "領域", values: ["x"] },
  team: { type: "text", label: "チーム" },
});

const ids = (list: { id: string }[]) => list.map((c) => c.id);

describe("columnCandidates", () => {
  it("lists the builtin columns, then the properties of the schemas by name", () => {
    expect(ids(columnCandidates([shared]))).toEqual([
      "id",
      "title",
      "kind",
      "status",
      "assignee",
      "label",
      "parent",
      "created",
      "updated",
      "area",
      "due",
      "points",
      "relates",
    ]);
  });

  it("takes the union of several stores and the label of the first store that has the property", () => {
    const list = columnCandidates([shared, me]);
    expect(ids(list).slice(9)).toEqual(["area", "due", "points", "relates", "team"]);
    expect(list.find((c) => c.id === "area")?.label).toBe("area");
    expect(list.find((c) => c.id === "team")?.label).toBe("チーム");
    expect(list.find((c) => c.id === "due")?.label).toBe("期限");
  });

  it("shows ID, parent, created, updated and task references only", () => {
    const readonly = columnCandidates([shared])
      .filter((c) => c.readonly)
      .map((c) => c.id);
    expect(readonly).toEqual(["id", "parent", "created", "updated", "relates"]);
  });

  it("has no property column without a schema", () => {
    expect(columnCandidates([])).toHaveLength(9);
  });
});

describe("shownColumns", () => {
  const candidates = columnCandidates([shared]);

  it("keeps the order of the view and leaves out what no schema defines", () => {
    expect(ids(shownColumns(["title", "nope", "due", "id"], candidates))).toEqual([
      "title",
      "due",
      "id",
    ]);
  });

  it("shows a column once", () => {
    expect(ids(shownColumns(["id", "id"], candidates))).toEqual(["id"]);
  });

  it("leaves out a property when the store that defines it is not shown", () => {
    expect(ids(shownColumns(["id", "team"], columnCandidates([shared])))).toEqual(["id"]);
    expect(ids(shownColumns(["id", "team"], columnCandidates([shared, me])))).toEqual([
      "id",
      "team",
    ]);
  });
});

describe("toggleColumn", () => {
  it("adds at the end and takes out, and keeps the names it does not show", () => {
    expect(toggleColumn(["id", "ghost"], "due", true)).toEqual(["id", "ghost", "due"]);
    expect(toggleColumn(["id", "due", "ghost"], "due", false)).toEqual(["id", "ghost"]);
    expect(toggleColumn(["id", "due"], "due", true)).toEqual(["id", "due"]);
  });
});

describe("patchFieldOf", () => {
  const c = columnCandidates([shared]);
  const of = (id: string) => {
    const spec = c.find((x) => x.id === id);
    return spec ? patchFieldOf(spec) : "missing";
  };
  it("names the field of the PATCH", () => {
    expect(of("title")).toBe("title");
    expect(of("kind")).toBe("kind");
    expect(of("status")).toBe("status");
    expect(of("assignee")).toBe("assignees");
    expect(of("label")).toBe("labels");
    expect(of("points")).toBe("props.points");
  });
  it("has none for what is only shown", () => {
    for (const id of ["id", "parent", "created", "updated", "relates"]) expect(of(id)).toBeNull();
  });
});

describe("sortKey", () => {
  const parent = task("P", "project", null, { id: "REL-1" });
  const ctx: SortCtx = {
    schemaOf: (store) =>
      store === "shared"
        ? ({
            ...shared,
            props: {
              ...shared.props,
              prio: { type: "enum", values: ["low", "mid", "high"] },
            },
          } as SchemaView)
        : undefined,
    taskOf: (u) => (u === "P" ? parent : undefined),
    storeOrder: ["shared", "me"],
  };

  it("reads the builtin fields", () => {
    const t = task("T1", "task", "P", {
      id: "REL-4",
      title: "",
      assignees: ["koki", "tanaka"],
      labels: ["a", "b"],
      created: "2026-11-01T00:00:00Z",
    });
    expect(sortKey(t, "id", ctx)).toBe("REL-4");
    expect(sortKey(t, "title", ctx)).toBeUndefined();
    expect(sortKey(t, "assignee", ctx)).toBe("koki");
    expect(sortKey(t, "label", ctx)).toBe("a,b");
    expect(sortKey(t, "parent", ctx)).toBe("REL-1");
    expect(sortKey(t, "created", ctx)).toBe(Date.parse("2026-11-01T00:00:00Z"));
    expect(sortKey(t, "@store", ctx)).toBe(0);
  });

  it("reads a property by its type", () => {
    const t = task("T1", "task", null, {
      props: { points: 5, done: true, prio: "high", note: "x", empty: "", area: ["a", "b"] },
    });
    expect(sortKey(t, "points", ctx)).toBe(5);
    expect(sortKey(t, "done", ctx)).toBe(1);
    expect(sortKey(t, "prio", ctx)).toBe(2);
    expect(sortKey(t, "note", ctx)).toBe("x");
    expect(sortKey(t, "empty", ctx)).toBeUndefined();
    expect(sortKey(t, "area", ctx)).toBe("a,b");
    expect(sortKey(t, "missing", ctx)).toBeUndefined();
  });

  it("sorts the status by its place in sort:status", () => {
    const t = (status: string, store = "shared") => task("T", "task", null, { status, store });
    const key = (x: ReturnType<typeof t>) => sortKey(x, "status", ctx) as number;
    expect(key(t("backlog"))).toBeLessThan(key(t("todo")));
    expect(key(t("doing"))).toBeLessThan(key(t("done")));
    expect(key(t("todo", "me"))).toBeGreaterThan(key(t("canceled")));
  });
});

describe("compareKeys", () => {
  it("compares numbers as numbers and texts naturally, numbers first", () => {
    expect(compareKeys(2, 10)).toBeLessThan(0);
    expect(compareKeys("REL-2", "REL-10")).toBeLessThan(0);
    expect(compareKeys(5, "a")).toBeLessThan(0);
    expect(compareKeys("a", 5)).toBeGreaterThan(0);
    expect(compareKeys("x", "x")).toBe(0);
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

```bash
cd web && pnpm vitest run app/table/columns.test.ts
```

期待: `./columns` が解決できず失敗する。

- [ ] **Step 3: `columns.ts` を書く**

`web/app/table/columns.ts`:

```ts
import type { TaskView } from "../../core/api/client";
import type { PatchField } from "../../core/save";
import type { SchemaView } from "../../core/schema";
import { statusRank } from "../../core/status-order";

// A column of the table. width is in rem.
export type ColumnSpec = { id: string; label: string; readonly: boolean; width: number };

// The columns a view can name besides the properties of the schema, in the order of
// internal/view Builtins. ID, parent, created and updated are shown, not edited.
const BUILTIN: ColumnSpec[] = [
  { id: "id", label: "ID", readonly: true, width: 6.5 },
  { id: "title", label: "タイトル", readonly: false, width: 22 },
  { id: "kind", label: "種類", readonly: false, width: 8 },
  { id: "status", label: "ステータス", readonly: false, width: 10 },
  { id: "assignee", label: "担当", readonly: false, width: 12 },
  { id: "label", label: "ラベル", readonly: false, width: 12 },
  { id: "parent", label: "親", readonly: true, width: 6.5 },
  { id: "created", label: "作成", readonly: true, width: 6.5 },
  { id: "updated", label: "更新", readonly: true, width: 6.5 },
];

// The first column of a table that lists the tasks of more than one store; it is not a column of a
// view, so it is never saved.
export const STORE_COLUMN: ColumnSpec = { id: "@store", label: "Store", readonly: true, width: 8 };

// isBuiltinColumn is whether the column is a field of a task, not a property of the schema.
export const isBuiltinColumn = (id: string): boolean =>
  id === STORE_COLUMN.id || BUILTIN.some((c) => c.id === id);

// Names of a schema that are not a column: the builtin ones, and blocked_by, which a view cannot
// name (the Relations section of the detail edits it).
const NOT_A_PROP = new Set([...BUILTIN.map((c) => c.id), "blocked_by"]);

function propWidth(def: { type: string; multi?: boolean }): number {
  if (def.multi || def.type === "multi_enum") return 12;
  if (def.type === "number" || def.type === "bool") return 6.5;
  if (def.type === "date") return 8;
  return 9;
}

// columnCandidates are the columns the menu offers: the builtin ones, then the properties of the
// schemas by name. Several stores give the union of their properties, and the label is the one of
// the first store that has the property.
export function columnCandidates(schemas: SchemaView[]): ColumnSpec[] {
  const props = new Map<string, ColumnSpec>();
  for (const sc of schemas) {
    for (const [name, def] of Object.entries(sc.props)) {
      if (NOT_A_PROP.has(name) || props.has(name)) continue;
      props.set(name, {
        id: name,
        label: def.label || name,
        readonly: def.type === "task_ref",
        width: propWidth(def),
      });
    }
  }
  return [...BUILTIN, ...[...props.keys()].sort().map((name) => props.get(name) as ColumnSpec)];
}

// shownColumns are the columns of a view in its order. A name that no schema defines is left out
// (the view keeps it; a store that has the property may be added later), and so is a repeated one.
export function shownColumns(selected: string[], candidates: ColumnSpec[]): ColumnSpec[] {
  const byId = new Map(candidates.map((c) => [c.id, c]));
  const out: ColumnSpec[] = [];
  for (const id of selected) {
    const spec = byId.get(id);
    if (spec && !out.includes(spec)) out.push(spec);
  }
  return out;
}

// toggleColumn adds a column at the end, or takes it out; the names the table does not show stay.
export function toggleColumn(selected: string[], id: string, on: boolean): string[] {
  const rest = selected.filter((c) => c !== id);
  return on ? [...rest, id] : rest;
}

// patchFieldOf is the field a cell edits; null for a column that is only shown.
export function patchFieldOf(spec: ColumnSpec): PatchField | null {
  if (spec.readonly) return null;
  switch (spec.id) {
    case "title":
    case "kind":
    case "status":
      return spec.id;
    case "assignee":
      return "assignees";
    case "label":
      return "labels";
    default:
      return `props.${spec.id}`;
  }
}

// What a column sorts by: a number, a text, or undefined for a task with no value (those go last
// whichever way the column is sorted).
export type SortKey = string | number | undefined;

// What sorting and grouping need to know besides the task.
export type SortCtx = {
  schemaOf: (store: string) => SchemaView | undefined;
  taskOf: (ulid: string) => TaskView | undefined;
  // The names of the stores in the order of GET /api/stores.
  storeOrder: string[];
};

const asKey = (v: string): SortKey => (v === "" ? undefined : v);

function propKey(t: TaskView, name: string, ctx: SortCtx): SortKey {
  const v = (t.props as Record<string, unknown>)[name];
  if (v === undefined || v === null || v === "") return undefined;
  if (Array.isArray(v)) return v.length === 0 ? undefined : v.map(String).join(",");
  if (typeof v === "boolean") return v ? 1 : 0;
  if (typeof v === "number") return v;
  const def = ctx.schemaOf(t.store)?.props[name];
  if (def?.type === "task_ref") return ctx.taskOf(String(v))?.id ?? String(v);
  const values = def?.values;
  if (values && values.length > 0) {
    const at = values.indexOf(String(v));
    return at >= 0 ? at : values.length;
  }
  return String(v);
}

// sortKey is the value the column sorts the task by.
export function sortKey(t: TaskView, id: string, ctx: SortCtx): SortKey {
  switch (id) {
    case "@store": {
      const i = ctx.storeOrder.indexOf(t.store);
      return i < 0 ? ctx.storeOrder.length : i;
    }
    case "id":
      return t.id;
    case "title":
      return asKey(t.title);
    case "kind":
      return t.kind;
    case "status":
      return statusRank(ctx.schemaOf(t.store), t.status, ctx.storeOrder.indexOf(t.store));
    case "assignee":
      return asKey(t.assignees?.[0] ?? "");
    case "label":
      return asKey((t.labels ?? []).join(","));
    case "parent":
      return t.parent ? (ctx.taskOf(t.parent)?.id ?? undefined) : undefined;
    case "created":
      return Date.parse(t.created);
    case "updated":
      return Date.parse(t.updated);
    default:
      return propKey(t, id, ctx);
  }
}

// compareKeys orders numbers as numbers and texts the way a person reads them (REL-2 before
// REL-10); a number comes before a text.
export function compareKeys(a: unknown, b: unknown): number {
  if (typeof a === "number" && typeof b === "number") return a - b;
  if (typeof a === "number") return -1;
  if (typeof b === "number") return 1;
  return String(a).localeCompare(String(b), "ja", { numeric: true });
}
```

- [ ] **Step 4: 通ることを確かめる**

```bash
cd web && pnpm vitest run app/table/columns.test.ts
```

期待: PASS（14 件）。

- [ ] **Step 5: グループのテストを書く（失敗する）**

`web/app/table/group.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { TaskView } from "../../core/api/client";
import { schema as base, task } from "../tree/fixtures";
import type { SortCtx } from "./columns";
import { groupTasks } from "./group";

const parents: Record<string, TaskView> = {
  E1: task("E1", "epic", null, {
    id: "REL-1",
    title: "API v2 への移行",
    progress: { done: 1, total: 5 },
  }),
  E8: task("E8", "epic", null, { id: "REL-8", title: "リリース作業" }),
};
const ctx: SortCtx = {
  schemaOf: () => base,
  taskOf: (u) => parents[u],
  storeOrder: ["shared"],
};

const shape = (items: ReturnType<typeof groupTasks>) =>
  items.map((i) => (i.type === "group" ? `# ${i.label} (${i.count})` : i.task.ulid));

describe("groupTasks", () => {
  const tasks = [
    task("T1", "task", "E1", {
      status: "doing",
      assignees: ["koki"],
      project: { ulid: "P", id: "REL-15", title: "リリース" },
    }),
    task("T2", "task", "E8", { status: "todo", assignees: ["koki", "tanaka"] }),
    task("T3", "task", "E1", { status: "done" }),
    task("T4", "task", null, { status: "todo", assignees: ["abe"] }),
  ];

  it("is the rows as they are with no group", () => {
    expect(shape(groupTasks(tasks, "none", ctx))).toEqual(["T1", "T2", "T3", "T4"]);
  });

  it("groups by parent in the order they appear, with the progress of the parent, and none last", () => {
    const items = groupTasks(tasks, "parent", ctx);
    expect(shape(items)).toEqual([
      "# API v2 への移行 (2)",
      "T1",
      "T3",
      "# リリース作業 (1)",
      "T2",
      "# 親なし (1)",
      "T4",
    ]);
    expect(items[0]).toMatchObject({ id: "REL-1", ulid: "E1", progress: { done: 1, total: 5 } });
    expect(items[3]).toMatchObject({ id: "REL-8", progress: undefined });
  });

  it("names a parent that is not in the cache", () => {
    const items = groupTasks([task("T9", "task", "GONE")], "parent", ctx);
    expect(shape(items)).toEqual(["# 不明な親 (1)", "T9"]);
  });

  it("groups by status in the order of sort:status, with the label of the schema", () => {
    expect(shape(groupTasks(tasks, "status", ctx))).toEqual([
      "# todo (2)",
      "T2",
      "T4",
      "# doing (1)",
      "T1",
      "# done (1)",
      "T3",
    ]);
  });

  it("groups by assignee by name and puts a task in the group of each assignee", () => {
    const items = groupTasks(tasks, "assignee", ctx);
    expect(shape(items)).toEqual([
      "# abe (1)",
      "T4",
      "# koki (2)",
      "T1",
      "T2",
      "# tanaka (1)",
      "T2",
      "# 未割り当て (1)",
      "T3",
    ]);
    const keys = items.map((i) => i.key);
    expect(new Set(keys).size).toBe(keys.length);
  });

  it("groups by project, the tasks outside any project last", () => {
    expect(shape(groupTasks(tasks, "project", ctx))).toEqual([
      "# リリース (1)",
      "T1",
      "# プロジェクトなし (3)",
      "T2",
      "T3",
      "T4",
    ]);
  });
});
```

- [ ] **Step 6: 失敗を確かめる**

```bash
cd web && pnpm vitest run app/table/group.test.ts
```

期待: `./group` が解決できず失敗する。

- [ ] **Step 7: `group.ts` を書く**

`web/app/table/group.ts`:

```ts
import type { TaskView } from "../../core/api/client";
import { statusRank } from "../../core/status-order";
import type { TableSettings } from "../../core/views";
import type { SortCtx } from "./columns";

export type GroupBy = TableSettings["groupBy"];

// The height of a row and of the heading of a group, in px; the CSS uses --row-h and
// --control-h, which have the same sizes.
export const ROW_HEIGHT = 40;
export const GROUP_HEIGHT = 36;

// The heading of a group. id and ulid are those of the parent or the project the group is of;
// none is the group of the tasks that have no parent, assignee or project.
export type GroupHead = {
  type: "group";
  key: string;
  label: string;
  id?: string;
  ulid?: string;
  progress?: { done: number; total: number };
  count: number;
  none: boolean;
};
export type TableItem = GroupHead | { type: "row"; key: string; task: TaskView };

type Bucket = { head: Omit<GroupHead, "count">; tasks: TaskView[] };

const NONE_LABEL: Record<Exclude<GroupBy, "none" | "status">, string> = {
  parent: "親なし",
  assignee: "未割り当て",
  project: "プロジェクトなし",
};

function bucketsOf(t: TaskView, by: Exclude<GroupBy, "none">, ctx: SortCtx): Bucket["head"][] {
  switch (by) {
    case "parent": {
      if (!t.parent) return [{ type: "group", key: "", label: NONE_LABEL.parent, none: true }];
      const p = ctx.taskOf(t.parent);
      return [
        {
          type: "group",
          key: t.parent,
          label: p?.title ?? "不明な親",
          id: p?.id,
          ulid: t.parent,
          progress: p?.progress && p.progress.total > 0 ? p.progress : undefined,
          none: false,
        },
      ];
    }
    case "status": {
      const label = ctx.schemaOf(t.store)?.statuses?.find((s) => s.name === t.status)?.label;
      return [{ type: "group", key: t.status, label: label || t.status, none: false }];
    }
    case "assignee": {
      const names = t.assignees ?? [];
      if (names.length === 0) {
        return [{ type: "group", key: "", label: NONE_LABEL.assignee, none: true }];
      }
      return names.map((n) => ({ type: "group", key: n, label: n, none: false }));
    }
    case "project": {
      if (!t.project) return [{ type: "group", key: "", label: NONE_LABEL.project, none: true }];
      return [
        {
          type: "group",
          key: t.project.ulid,
          label: t.project.title,
          id: t.project.id,
          ulid: t.project.ulid,
          none: false,
        },
      ];
    }
  }
}

// groupTasks lays the tasks out as the table draws them, flat: a heading, then its rows. The
// tasks come in the order the table sorts them. The groups of a parent or a project come in the
// order they first appear; those of a status in the order of sort:status; those of an assignee by
// name. The group of the tasks with none goes last. A task with several assignees is in the group
// of each, as group:assignee of the query has it.
export function groupTasks(tasks: TaskView[], by: GroupBy, ctx: SortCtx): TableItem[] {
  if (by === "none") return tasks.map((task) => ({ type: "row", key: task.ulid, task }));
  const buckets = new Map<string, Bucket>();
  for (const task of tasks) {
    for (const head of bucketsOf(task, by, ctx)) {
      const b = buckets.get(head.key) ?? { head, tasks: [] };
      b.tasks.push(task);
      buckets.set(head.key, b);
    }
  }
  const list = [...buckets.values()];
  if (by === "status") {
    const rank = (b: Bucket) =>
      Math.min(
        ...b.tasks.map((t) =>
          statusRank(ctx.schemaOf(t.store), t.status, ctx.storeOrder.indexOf(t.store)),
        ),
      );
    list.sort((a, b) => rank(a) - rank(b));
  } else if (by === "assignee") {
    list.sort((a, b) => a.head.label.localeCompare(b.head.label, "ja"));
  }
  const ordered = [...list.filter((b) => !b.head.none), ...list.filter((b) => b.head.none)];
  return ordered.flatMap((b) => [
    { ...b.head, count: b.tasks.length },
    ...b.tasks.map((task) => ({
      type: "row" as const,
      key: `${b.head.key}\u0000${task.ulid}`,
      task,
    })),
  ]);
}
```

- [ ] **Step 8: 通ることを確かめる**

```bash
cd web && pnpm vitest run app/table/group.test.ts
```

期待: PASS（6 件）。

- [ ] **Step 9: 画面のテストを書く（失敗する）**

`useListScope`・`../context`・`../router`・`@tanstack/svelte-virtual` は `vi.mock` で差し替える（部 A の実装に依存しないテストにするため。TanStack Table は本物）。core は `watchList`・`task`・`stores`・`schema`・`save` の最小の偽物。セルの編集は `save.edit`・`save.flush` の引数で確かめる。

`web/app/table/TableView.test.ts`:

```ts
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import type { TableSettings } from "../../core/views";
import { closeCreate, createState } from "../create-state.svelte";
import { emitter } from "../test-utils";
import { schema as base, task } from "../tree/fixtures";

const h = vi.hoisted(() => ({
  core: undefined as unknown,
  scope: undefined as unknown,
  go: vi.fn(),
  // Where the window of the virtualizer starts.
  vstart: 0,
}));

vi.mock("../context", () => ({ core: () => h.core }));
vi.mock("../list/scope.svelte", () => ({ useListScope: () => h.scope }));
vi.mock("../router", () => ({
  router: {
    go: h.go,
    href: (r: { name: string; ulid?: string }) => (r.name === "home" ? "/" : `/tasks/${r.ulid}`),
  },
  onLinkClick: (e: MouseEvent, path: string) => {
    e.preventDefault();
    h.go(path);
  },
}));
vi.mock("@tanstack/svelte-virtual", () => ({
  createVirtualizer: () => ({
    subscribe(fn: (v: unknown) => void) {
      const inst = {
        getVirtualItems: () =>
          Array.from({ length: 10 }, (_, i) => {
            const index = h.vstart + i;
            return { index, start: index * 40, size: 40, key: index };
          }),
        getTotalSize: () => 40 * 250,
        setOptions() {},
      };
      fn(inst);
      return () => {};
    },
    setOptions() {},
  }),
}));

import TableView from "./TableView.svelte";

const shared = {
  ...base,
  props: {
    due: { type: "date", label: "期限" },
    points: { type: "number", label: "points" },
  },
} as unknown as SchemaView;
const me = {
  ...base,
  props: { team: { type: "text", label: "チーム" } },
} as unknown as SchemaView;

const SETTINGS: TableSettings = {
  columns: ["id", "title", "status", "assignee", "due"],
  groupBy: "none",
};
const WORKSPACE = { kind: "workspace", store: "shared", ulid: "W1" } as const;

type Over = {
  ulids?: string[];
  stores?: string[];
  scope?: Record<string, unknown>;
  error?: unknown;
  loading?: boolean;
  saveState?: { kind: string; message?: string };
  many?: number;
  canceled?: string[];
};

function setup(over: Over = {}) {
  const tasks = new Map<string, TaskView>([
    [
      "E1",
      task("E1", "epic", null, {
        id: "REL-1",
        title: "API v2 への移行",
        progress: { done: 1, total: 5 },
      }),
    ],
    [
      "T1",
      task("T1", "task", "E1", {
        id: "REL-4",
        title: "移行ガイドを書く",
        status: "todo",
        assignees: ["tanaka"],
        props: { points: 3, due: "2026-11-06" },
      }),
    ],
    [
      "T2",
      task("T2", "task", "E1", {
        id: "REL-2",
        title: "破壊的変更をまとめる",
        status: "doing",
        assignees: ["koki"],
        props: { points: 5 },
      }),
    ],
    ["T3", task("T3", "task", null, { id: "REL-9", title: "負荷試験", status: "backlog" })],
    [
      "M1",
      task("M1", "task", null, {
        id: "ME-1",
        title: "私のタスク",
        store: "me",
        props: { team: "sre" },
      }),
    ],
  ]);
  let ulids = over.ulids ?? ["T1", "T2", "T3"];
  if (over.many) {
    ulids = Array.from({ length: over.many }, (_, i) => {
      tasks.set(`N${i}`, task(`N${i}`, "task", null, { id: `REL-${100 + i}`, title: `many ${i}` }));
      return `N${i}`;
    });
  }
  const ev = emitter();
  const watchList = vi.fn((q: { q: string }) => ({
    ulids: () => (/(^| )status:canceled/.test(q.q) ? (over.canceled ?? []) : ulids),
    error: () => over.error ?? null,
    loading: () => over.loading ?? false,
    dispose() {},
  }));
  const save = {
    edit: vi.fn(),
    flush: vi.fn(async () => {}),
    editing: () => false,
    state: () => over.saveState ?? { kind: "idle" },
    subscribe: ev.subscribe,
  };
  const schemas: Record<string, SchemaView> = { shared, me };
  h.core = {
    cache: { task: (u: string) => tasks.get(u), watchList, subscribe: ev.subscribe },
    stores: {
      stores: () => (over.stores ?? ["shared"]).map((name) => ({ name })),
      tone: (s: string) => (s === "me" ? "orange" : "purple"),
      subscribe: ev.subscribe,
    },
    schema: {
      get: (s: string) => schemas[s],
      load: async (s: string) => schemas[s],
      subscribe: ev.subscribe,
    },
    save,
  };
  h.scope = {
    ready: true,
    notFound: false,
    apiBase: "workspace:release",
    cliBase: "workspace:release",
    store: "shared",
    workspace: { name: "release", store: "shared", ulid: "W1" },
    project: undefined,
    saveBase: "workspace:release",
    schemas: (stores: string[]) =>
      stores.map((s) => schemas[s]).filter((s): s is SchemaView => s !== undefined),
    ...over.scope,
  };
  return { save, watchList };
}

function mount(props: Record<string, unknown> = {}) {
  const onquery = vi.fn();
  const onsettings = vi.fn();
  const r = render(TableView, {
    view: WORKSPACE,
    query: "-status:canceled",
    onquery,
    settings: SETTINGS,
    onsettings,
    ...props,
  });
  return { onquery, onsettings, ...r };
}

// The IDs of the rows in the order they are drawn.
const order = () => screen.getAllByRole("link").map((a) => a.textContent);

beforeEach(() => {
  h.vstart = 0;
  h.go.mockReset();
  closeCreate();
});
afterEach(cleanup);

describe("TableView", () => {
  it("lists the tasks with the columns of the settings and the count", () => {
    setup();
    mount();
    expect(screen.getAllByRole("columnheader").map((c) => c.textContent?.trim())).toEqual([
      "保存の状態",
      "ID",
      "タイトル",
      "ステータス",
      "担当",
      "期限",
    ]);
    expect(order()).toEqual(["REL-4", "REL-2", "REL-9"]);
    expect(screen.getByDisplayValue("移行ガイドを書く")).toBeInTheDocument();
    expect(screen.getByDisplayValue("2026-11-06")).toBeInTheDocument();
    expect(screen.getByText("3 件 · 中止 0 件を除外")).toBeInTheDocument();
  });

  it("reads the tasks with the query of the screen on its store", () => {
    const { watchList } = setup();
    mount();
    expect(watchList).toHaveBeenCalledWith({
      q: "workspace:release -status:canceled",
      tree: false,
      store: "shared",
    });
  });

  it("reads every store when the screen names none, without a leading space", () => {
    const { watchList } = setup({ scope: { apiBase: "", cliBase: "", store: undefined } });
    mount({ view: { kind: "all" }, query: "assignee:@me" });
    expect(watchList).toHaveBeenCalledWith({ q: "assignee:@me", tree: false, store: undefined });
  });

  it("opens the detail from the ID", async () => {
    setup();
    mount();
    await fireEvent.click(screen.getByRole("link", { name: "REL-2" }));
    expect(h.go).toHaveBeenCalledWith("/tasks/T2");
  });

  it("shows the command that lists the rows, or the one the screen is given", () => {
    setup();
    const { unmount } = mount();
    expect(
      screen.getByLabelText(
        'コマンドをコピー: hh ls --store shared "workspace:release -status:canceled"',
      ),
    ).toBeInTheDocument();
    unmount();
    mount({ cli: "hh view show 今週のリリース作業 --store shared" });
    expect(
      screen.getByLabelText("コマンドをコピー: hh view show 今週のリリース作業 --store shared"),
    ).toBeInTheDocument();
  });

  it("shows the command that would save the settings, not to copy until it has a name", () => {
    setup();
    mount({ settings: { columns: ["id", "due"], groupBy: "status" } });
    const line = screen.getByLabelText(
      /^コマンドをコピー: hh view save <名前> --layout table --query .* --columns id,due --group-by status$/,
    );
    expect(line).toBeDisabled();
  });

  describe("editing a cell", () => {
    it("sends the title edited, and flushes when the cell loses the focus", async () => {
      const { save } = setup();
      mount();
      const input = screen.getByRole("textbox", { name: "タイトル: REL-4" });
      await fireEvent.input(input, { target: { value: "移行ガイドを直す" } });
      expect(save.edit).toHaveBeenCalledWith("T1", "shared", "title", "移行ガイドを直す");
      expect(save.flush).not.toHaveBeenCalled();
      await fireEvent.blur(input);
      expect(save.flush).toHaveBeenCalledWith("T1");
    });

    it("sends a property under props.<name>", async () => {
      const { save } = setup();
      mount({ settings: { ...SETTINGS, columns: ["id", "points"] } });
      await fireEvent.input(screen.getByRole("spinbutton", { name: "points: REL-4" }), {
        target: { value: "8" },
      });
      expect(save.edit).toHaveBeenCalledWith("T1", "shared", "props.points", 8);
    });

    it("sends a status chosen from the menu of the store of the row, and flushes at once", async () => {
      const { save } = setup();
      mount();
      await fireEvent.click(screen.getByRole("button", { name: "ステータス: REL-4" }));
      await fireEvent.click(screen.getByRole("menuitem", { name: /doing/ }));
      expect(save.edit).toHaveBeenCalledWith("T1", "shared", "status", "doing");
      expect(save.flush).toHaveBeenCalledWith("T1");
    });

    it("sends the assignees as a list", async () => {
      const { save } = setup();
      mount();
      const input = screen.getByRole("textbox", { name: "担当: REL-9" });
      await fireEvent.input(input, { target: { value: "sato" } });
      await fireEvent.keyDown(input, { key: "Enter" });
      expect(save.edit).toHaveBeenCalledWith("T3", "shared", "assignees", ["sato"]);
    });

    it("only shows the columns that are not edited", () => {
      setup();
      mount({ settings: { ...SETTINGS, columns: ["id", "parent", "created"] } });
      // The parent of REL-4 and REL-2 is REL-1; REL-9 has none.
      expect(screen.getAllByRole("link").map((a) => a.textContent)).toEqual([
        "REL-4",
        "REL-1",
        "REL-2",
        "REL-1",
        "REL-9",
      ]);
      expect(screen.getAllByText("11/01")).toHaveLength(3);
      expect(within(screen.getByRole("table")).queryByRole("textbox")).toBeNull();
    });

    it("marks a row whose save failed, and sends it again", async () => {
      const { save } = setup({ saveState: { kind: "error", message: "値が正しくありません" } });
      mount();
      const retry = screen.getAllByRole("button", { name: /保存に失敗しました。やり直す/ })[0];
      expect(retry).toHaveAttribute("title", "値が正しくありません");
      await fireEvent.click(retry as HTMLElement);
      expect(save.flush).toHaveBeenCalledWith("T1");
    });
  });

  describe("columns", () => {
    it("leaves out a column no schema defines and keeps the order of the view", () => {
      setup();
      mount({ settings: { ...SETTINGS, columns: ["due", "ghost", "id"] } });
      expect(screen.getAllByRole("columnheader").map((c) => c.textContent?.trim())).toEqual([
        "保存の状態",
        "期限",
        "ID",
      ]);
    });

    it("chooses the columns with check marks that keep the menu open", async () => {
      setup();
      const { onsettings } = mount();
      await fireEvent.click(screen.getByRole("button", { name: "列 5" }));
      const menu = screen.getByRole("menu", { name: "列" });
      expect(within(menu).getByRole("menuitem", { name: /^ID 表示中$/ })).toBeInTheDocument();
      expect(within(menu).getByRole("menuitem", { name: /^親 非表示$/ })).toBeInTheDocument();
      // The properties of the schema come after the builtin columns, by name.
      expect(
        within(menu)
          .getAllByRole("menuitem")
          .map((i) => i.textContent?.replace(/表示中|非表示/, "").trim())
          .slice(-2),
      ).toEqual(["期限", "points"]);
      await fireEvent.click(within(menu).getByRole("menuitem", { name: /^親/ }));
      expect(onsettings).toHaveBeenCalledWith({
        ...SETTINGS,
        columns: [...SETTINGS.columns, "parent"],
      });
      expect(screen.getByRole("menu", { name: "列" })).toBeInTheDocument();
      await fireEvent.click(within(menu).getByRole("menuitem", { name: /^期限/ }));
      expect(onsettings).toHaveBeenLastCalledWith({
        ...SETTINGS,
        columns: ["id", "title", "status", "assignee"],
      });
    });

    it("does not take out the last column", async () => {
      setup();
      mount({ settings: { ...SETTINGS, columns: ["id"] } });
      await fireEvent.click(screen.getByRole("button", { name: "列 1" }));
      expect(screen.getByRole("menuitem", { name: /^ID/ })).toBeDisabled();
    });

    it("keeps the names the table does not show when a column is added", async () => {
      setup();
      const { onsettings } = mount({ settings: { ...SETTINGS, columns: ["id", "ghost"] } });
      await fireEvent.click(screen.getByRole("button", { name: "列 1" }));
      await fireEvent.click(screen.getByRole("menuitem", { name: /^タイトル/ }));
      expect(onsettings).toHaveBeenCalledWith({ ...SETTINGS, columns: ["id", "ghost", "title"] });
    });
  });

  describe("several stores", () => {
    const both = {
      ulids: ["T1", "M1"],
      stores: ["shared", "me"],
      scope: { apiBase: "", cliBase: "", store: undefined, saveBase: "" },
    };

    it("tells the store of each row and the properties of each store", () => {
      setup(both);
      mount({
        view: { kind: "all" },
        settings: { ...SETTINGS, columns: ["id", "due", "team"] },
      });
      expect(screen.getAllByRole("columnheader").map((c) => c.textContent?.trim())).toEqual([
        "保存の状態",
        "Store",
        "ID",
        "期限",
        "チーム",
      ]);
      expect(screen.getByTitle("Store: shared")).toBeInTheDocument();
      expect(screen.getByTitle("Store: me")).toBeInTheDocument();
      // A property the store of the row does not define is not editable: 未定義
      const undef = screen.getAllByText("（未定義）");
      expect(undef).toHaveLength(2);
      // Drawn in italics by the class
      for (const u of undef) expect(u).toHaveClass("undef");
      expect(screen.getByDisplayValue("sre")).toBeInTheDocument();
    });

    it("has no Store column for a screen of one store", () => {
      setup({ ulids: ["T1"] });
      mount();
      expect(screen.queryByRole("columnheader", { name: "Store" })).toBeNull();
    });
  });

  describe("sorting by a header", () => {
    it("sorts ascending, then descending, then back to the order of the query", async () => {
      setup();
      mount();
      const id = () => screen.getByRole("columnheader", { name: /^ID/ });
      expect(id()).toHaveAttribute("aria-sort", "none");
      await fireEvent.click(within(id()).getByRole("button"));
      expect(id()).toHaveAttribute("aria-sort", "ascending");
      expect(order()).toEqual(["REL-2", "REL-4", "REL-9"]);
      await fireEvent.click(within(id()).getByRole("button"));
      expect(id()).toHaveAttribute("aria-sort", "descending");
      expect(order()).toEqual(["REL-9", "REL-4", "REL-2"]);
      await fireEvent.click(within(id()).getByRole("button"));
      expect(id()).toHaveAttribute("aria-sort", "none");
      expect(order()).toEqual(["REL-4", "REL-2", "REL-9"]);
    });

    it("sorts the status in the order of the schema and puts the rows with no value last", async () => {
      setup();
      mount({ settings: { ...SETTINGS, columns: ["id", "status", "due"] } });
      await fireEvent.click(
        within(screen.getByRole("columnheader", { name: /^ステータス/ })).getByRole("button"),
      );
      // backlog, todo, doing
      expect(order()).toEqual(["REL-9", "REL-4", "REL-2"]);
      await fireEvent.click(
        within(screen.getByRole("columnheader", { name: /^期限/ })).getByRole("button"),
      );
      expect(order()).toEqual(["REL-4", "REL-2", "REL-9"]);
      await fireEvent.click(
        within(screen.getByRole("columnheader", { name: /^期限/ })).getByRole("button"),
      );
      // Only REL-4 has a due date: it stays first whichever way, the others after it
      expect(order()).toEqual(["REL-4", "REL-2", "REL-9"]);
    });
  });

  describe("groups", () => {
    it("draws a heading before the rows of each parent, with the progress of the parent", () => {
      setup();
      mount({ settings: { ...SETTINGS, groupBy: "parent" } });
      const heads = document.querySelectorAll(".group");
      expect([...heads].map((e) => e.textContent?.replace(/\s+/g, " ").trim())).toEqual([
        "API v2 への移行 REL-1 1/5 完了 2 件",
        "親なし 1 件",
      ]);
      expect(order()).toEqual(["REL-4", "REL-2", "REL-9"]);
    });

    it("chooses the group from a menu", async () => {
      setup();
      const { onsettings } = mount();
      await fireEvent.click(screen.getByRole("button", { name: "グループ: なし" }));
      const menu = screen.getByRole("menu", { name: "グループ" });
      expect(
        within(menu)
          .getAllByRole("menuitem")
          .map((i) => i.textContent?.trim()),
      ).toEqual(["なし", "親", "ステータス", "担当者", "プロジェクト"]);
      await fireEvent.click(within(menu).getByRole("menuitem", { name: "担当者" }));
      expect(onsettings).toHaveBeenCalledWith({ ...SETTINGS, groupBy: "assignee" });
      expect(screen.queryByRole("menu")).toBeNull();
    });
  });

  describe("states", () => {
    it("says it is loading", () => {
      setup({ ulids: [], loading: true });
      mount();
      expect(screen.getByText("読み込み中…")).toBeInTheDocument();
    });

    it("says there is no task yet, and offers to create one", async () => {
      setup({ ulids: [] });
      mount();
      expect(screen.getByText("release にはまだタスクがありません")).toBeInTheDocument();
      await fireEvent.click(screen.getByRole("button", { name: /タスクを作成/ }));
      expect(createState.request).toEqual({ kind: "task", parent: undefined });
    });

    it("says nothing matches, and clears the filter", async () => {
      setup({ ulids: [] });
      const { onquery } = mount({ query: "assignee:@me" });
      expect(screen.getByText("一致するタスクはありません")).toBeInTheDocument();
      await fireEvent.click(screen.getByRole("button", { name: "フィルタを解除" }));
      expect(onquery).toHaveBeenCalledWith("");
    });

    it("says nothing matches, not that there is no task, when only the canceled are left out", () => {
      setup({ ulids: [], canceled: ["X1"] });
      mount();
      expect(screen.getByText("一致するタスクはありません")).toBeInTheDocument();
      expect(screen.getByText("中止を除外しています")).toBeInTheDocument();
    });

    it("has no empty state of its own on a screen of every store", () => {
      setup({ ulids: [], scope: { apiBase: "", cliBase: "", store: undefined } });
      mount({ view: { kind: "all" }, query: "" });
      expect(screen.getByText("一致するタスクはありません")).toBeInTheDocument();
    });

    it("says the workspace is not found", () => {
      setup({ ulids: [], scope: { notFound: true } });
      mount();
      expect(screen.getByText("ワークスペース「W1」が見つかりません")).toBeInTheDocument();
    });

    it("shows why the query failed in the filter bar", () => {
      setup({ ulids: [], error: new Error("boom") });
      mount();
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });

    it("applies the filter that is typed", async () => {
      setup();
      const { onquery } = mount();
      const input = screen.getByLabelText("フィルタ（hh ls と同じクエリ）");
      await fireEvent.input(input, { target: { value: "assignee:@me" } });
      await fireEvent.keyDown(input, { key: "Enter" });
      expect(onquery).toHaveBeenCalledWith("assignee:@me");
    });
  });

  describe("a long list", () => {
    it("draws the rows in the window of the virtualizer only", async () => {
      setup({ many: 250 });
      mount();
      expect(order()).toHaveLength(10);
      expect(order()[0]).toBe("REL-100");
      const row = document.querySelector<HTMLElement>("tr.row");
      expect(row).toHaveStyle({ transform: "translateY(0px)" });
    });
  });

  it("waits for the schema before it draws a property", async () => {
    setup();
    mount({ settings: { ...SETTINGS, columns: ["id", "points"] } });
    await waitFor(() =>
      expect(screen.getByRole("spinbutton", { name: "points: REL-4" })).toHaveValue(3),
    );
  });
});
```

- [ ] **Step 10: 失敗を確かめる**

```bash
cd web && pnpm vitest run app/table/TableView.test.ts
```

期待: `./TableView.svelte` が解決できず失敗する。

- [ ] **Step 11: セル（`TableCell.svelte`）を書く**

列ごとの表示と入力をここに集める。ステータス・種類の選択肢は行の Store のスキーマ（D3）。Store のスキーマにないプロパティは斜体の「（未定義）」で、編集させない。スキーマがまだ届いていないあいだ（`schema` が undefined）のプロパティのセルは空にする。

`web/app/table/TableCell.svelte`:

```svelte
<script lang="ts">
  import type { TaskView } from "../../core/api/client";
  import { formatDate } from "../../core/format";
  import type { PatchField } from "../../core/save";
  import { canParent, parentRule, propEditorOf, type SchemaView } from "../../core/schema";
  import type { StoreTone } from "../../core/stores";
  import FieldInput from "../detail/FieldInput.svelte";
  import type { Option } from "../detail/field-types";
  import PropField from "../detail/PropField.svelte";
  import { type PropRow, statusOptions } from "../detail/prop-rows";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { onLinkClick, router } from "../router";
  import IdLabel from "../ui/IdLabel.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { type ColumnSpec, isBuiltinColumn, patchFieldOf } from "./columns";

  // One cell of the table. The schema is the one of the task's store (undefined until it is read).
  // pending: the field has an edit the server has not answered.
  let {
    task,
    col,
    schema,
    tone,
    pending,
    taskOf,
    onedit,
    oncommit,
  }: {
    task: TaskView;
    col: ColumnSpec;
    schema: SchemaView | undefined;
    tone: StoreTone;
    pending: boolean;
    taskOf: (ulid: string) => TaskView | undefined;
    onedit: (field: PatchField, value: unknown) => void;
    oncommit: () => void;
  } = $props();

  const field = $derived(patchFieldOf(col));
  const edit = (v: unknown) => {
    if (field) onedit(field, v);
  };
  const category = $derived(schema?.statuses?.find((s) => s.name === task.status)?.category);
  // null: no parent; undefined: there is one whose kind is not known, so no kind is ruled out.
  const parentKind = $derived(task.parent ? taskOf(task.parent)?.kind : null);
  const kindOptions = $derived<Option[]>(
    (schema?.kinds ?? []).map((k) => {
      const usable =
        k.name === task.kind ||
        !schema ||
        parentKind === undefined ||
        canParent(schema, k.name, parentKind);
      return {
        value: k.name,
        label: k.label || k.name,
        disabled: !usable,
        title: usable || !schema ? undefined : parentRule(schema, k.name),
      };
    }),
  );
  const parent = $derived(task.parent ? taskOf(task.parent) : undefined);
  const taskHref = (ulid: string) => router.href({ name: "task", ulid });

  // A property of the schema: its definition in the store of the task, or none (the store does not
  // define it: 未定義).
  const def = $derived(isBuiltinColumn(col.id) ? undefined : schema?.props[col.id]);
  const value = $derived((task.props as Record<string, unknown>)[col.id] ?? null);
  const row = $derived<PropRow | null>(
    schema && def
      ? {
          name: col.id,
          label: `${def.label || col.id}: ${task.id}`,
          editor: propEditorOf(schema, col.id),
          multi: def.multi === true,
          options: def.values ?? [],
          value,
        }
      : null,
  );
  const refs = $derived(
    (Array.isArray(value) ? value : value ? [value] : []).map(
      (v) => taskOf(String(v))?.id ?? String(v),
    ),
  );
</script>

{#if col.id === "@store"}
  <StoreBadge store={task.store} {tone} compact />
{:else if col.id === "id"}
  <IdLabel
    id={task.id}
    ulid={task.ulid}
    href={taskHref(task.ulid)}
    onopen={(e) => onLinkClick(e, taskHref(task.ulid))}
  />
{:else if col.id === "title"}
  <FieldInput
    editor="text"
    label="タイトル: {task.id}"
    value={task.title}
    nullable={false}
    {pending}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "kind"}
  <FieldInput
    editor="select"
    label="種類: {task.id}"
    value={task.kind}
    options={kindOptions}
    nullable={false}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "status"}
  <span class="status">
    <StatusIcon status={task.status} {category} />
    <FieldInput
      editor="select"
      label="ステータス: {task.id}"
      value={task.status}
      options={statusOptions(schema)}
      nullable={false}
      onedit={edit}
      {oncommit}
    />
  </span>
{:else if col.id === "assignee"}
  <FieldInput
    editor="chips"
    label="担当: {task.id}"
    value={task.assignees ?? []}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "label"}
  <FieldInput
    editor="chips"
    label="ラベル: {task.id}"
    value={task.labels ?? []}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "parent"}
  {#if parent}
    <IdLabel
      id={parent.id}
      ulid={parent.ulid}
      href={taskHref(parent.ulid)}
      onopen={(e) => onLinkClick(e, taskHref(parent.ulid))}
    />
  {:else}
    <span class="plain">—</span>
  {/if}
{:else if col.id === "created" || col.id === "updated"}
  <span class="plain mono"
    >{formatDate(col.id === "created" ? task.created : task.updated, new Date())}</span
  >
{:else if !schema}
  <span class="plain"></span>
{:else if !row}
  <span class="undef" title="{task.store} の schema.yaml にはないプロパティです">（未定義）</span>
{:else if row.editor === "taskref"}
  <span class="plain mono">{refs.join(", ")}</span>
{:else}
  <PropField {row} value={row.value} {pending} onedit={edit} {oncommit} />
{/if}

<style>
  .status {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    min-width: 0;
  }
  .plain,
  .undef {
    display: block;
    overflow: hidden;
    padding: 0 var(--space-8);
    color: var(--ink-2);
    font-size: var(--fs-13);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .undef {
    font-style: italic;
  }
</style>
```

- [ ] **Step 12: 画面（`TableView.svelte`）を書く**

作りの要点（コードのコメントにもある）:
- 列の定義は `settings.columns`（保存した順）とスキーマの候補から作る。`createTable` の `columns`・`data` は getter で渡す。`accessorFn` は並び替えの値を返し、セルは `TableCell` が描く（`FlexRender` は使わない）
- 並べ替えた行（`table.getRowModel().rows`）に `groupTasks` で見出しを差した平らな配列 `items` を、仮想化（200 を超えたときだけ）または全部描く
- 行の高さは `--row-h`（40px）、見出しは `--control-h`（36px）。`group.ts` の `ROW_HEIGHT`・`GROUP_HEIGHT` と同じ大きさ
- 設定の操作（グループ・列）は `ListBar` の `controls` に置き、メニューで選ぶたびに `onsettings` へ新しい `TableSettings` を渡す。足元の `hh view save <名前> …` の行は名前が空なのでコピーさせない（`CliLine` の `disabled`）
- 保存の状態は左端の細い列に出す（保存中は「…」、失敗は警告のボタンで、押すとそのタスクを `flush` し直す）
- スキーマの読み込みに失敗した Store は、行の下に `<Store> の schema.yaml を読み込めていません` を出す

`web/app/table/TableView.svelte`:

```svelte
<script lang="ts">
  import {
    constructSortFn,
    createSortedRowModel,
    createTable,
    rowSortingFeature,
    type SortingState,
    tableFeatures,
  } from "@tanstack/svelte-table";
  import { createVirtualizer } from "@tanstack/svelte-virtual";
  import type { Snippet } from "svelte";
  import type { TaskView } from "../../core/api/client";
  import { cliLs, cliViewSave, Placeholder } from "../../core/cli";
  import type { PatchField } from "../../core/save";
  import type { TableSettings } from "../../core/views";
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon from "../icons/Icon.svelte";
  import ListBar from "../list/ListBar.svelte";
  import type { ListScope } from "../list/scope";
  import { useListScope } from "../list/scope.svelte";
  import { router } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import { combine, hasTerm, NO_CANCELED, withoutTerm } from "../tree/filter";
  import { countLabel, VIRTUALIZE_AT } from "../tree/flatten";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import Menu, { type MenuItem } from "../ui/Menu.svelte";
  import {
    type ColumnSpec,
    columnCandidates,
    compareKeys,
    patchFieldOf,
    type SortCtx,
    STORE_COLUMN,
    shownColumns,
    sortKey,
    toggleColumn,
  } from "./columns";
  import { GROUP_HEIGHT, type GroupBy, groupTasks, ROW_HEIGHT, type TableItem } from "./group";
  import TableCell from "./TableCell.svelte";

  let {
    view,
    query,
    onquery,
    settings,
    onsettings,
    cli,
    actions,
  }: {
    view: ListScope;
    query: string;
    onquery: (q: string) => void;
    settings: TableSettings;
    onsettings: (s: TableSettings) => void;
    cli?: string;
    actions?: Snippet;
  } = $props();

  const c = core();
  const scope = useListScope(() => view);
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));
  // A fresh object on every change of the SaveController.
  const saves = useSubscribe(c.save, () => ({
    editing: (ulid: string, field: PatchField) => c.save.editing(ulid, field),
    state: (ulid: string) => c.save.state(ulid),
  }));

  // The cells read the cache and the schemas through these, so they follow a change.
  const taskOf = (ulid: string): TaskView | undefined => {
    cacheRev.current;
    return c.cache.task(ulid);
  };
  const schemaOf = (store: string) => {
    schemaRev.current;
    return c.schema.get(store);
  };

  // --- what the table lists ----------------------------------------------------------
  const joinQuery = (base: string, q: string) => combine(base, q).trim();

  type ListHandle = ReturnType<typeof c.cache.watchList>;
  let list = $state<ListHandle | null>(null);
  let canceledList = $state<ListHandle | null>(null);
  $effect(() => {
    if (!scope.ready) {
      list = null;
      return;
    }
    const h = c.cache.watchList({
      q: joinQuery(scope.apiBase, query),
      tree: false,
      store: scope.store,
    });
    list = h;
    return () => h.dispose();
  });
  // The tasks the default filter leaves out, to say how many.
  $effect(() => {
    if (!scope.ready || !hasTerm(query, NO_CANCELED)) {
      canceledList = null;
      return;
    }
    const q = joinQuery(scope.apiBase, `${withoutTerm(query, NO_CANCELED)} status:canceled`);
    const h = c.cache.watchList({ q, tree: false, store: scope.store });
    canceledList = h;
    return () => h.dispose();
  });

  const tasks = $derived.by(() => {
    cacheRev.current;
    return (list?.ulids() ?? [])
      .map((u) => c.cache.task(u))
      .filter((t): t is TaskView => t !== undefined);
  });
  const loading = $derived.by(() => {
    cacheRev.current;
    return !scope.ready || (list?.loading() ?? true);
  });
  const listError = $derived.by(() => {
    cacheRev.current;
    return list?.error() ?? null;
  });
  const canceledCount = $derived.by(() => {
    cacheRev.current;
    return canceledList ? canceledList.ulids().length : null;
  });

  // --- the schemas -----------------------------------------------------------------------
  const storeNames = $derived.by(() => {
    storesRev.current;
    return c.stores.stores().map((s) => s.name);
  });
  // The tasks of more than one store are told apart by their store.
  const showStore = $derived(view.kind === "all" && storeNames.length >= 2);
  const taskStores = $derived([...new Set(tasks.map((t) => t.store))]);
  // The columns on offer are the properties of the stores the screen shows; with no task to tell,
  // every store.
  const shownStores = $derived(
    scope.store ? [scope.store] : taskStores.length > 0 ? taskStores : storeNames,
  );
  const storeKey = $derived([...new Set([...taskStores, ...shownStores])].join("\n"));
  let schemaFailed = $state<string[]>([]);
  $effect(() => {
    for (const store of storeKey.split("\n")) {
      if (store === "") continue;
      c.schema.load(store).then(
        () => (schemaFailed = schemaFailed.filter((s) => s !== store)),
        () =>
          (schemaFailed = schemaFailed.includes(store) ? schemaFailed : [...schemaFailed, store]),
      );
    }
  });
  const schemas = $derived.by(() => {
    schemaRev.current;
    return scope.schemas(shownStores);
  });

  // --- the table -----------------------------------------------------------------------------
  const ctx = $derived.by<SortCtx>(() => {
    cacheRev.current;
    schemaRev.current;
    return {
      schemaOf: (store) => c.schema.get(store),
      taskOf: (ulid) => c.cache.task(ulid),
      storeOrder: storeNames,
    };
  });
  const candidates = $derived(columnCandidates(schemas));
  const shown = $derived(shownColumns(settings.columns, candidates));
  const specs = $derived<ColumnSpec[]>(showStore ? [STORE_COLUMN, ...shown] : shown);

  const features = tableFeatures({ rowSortingFeature, sortedRowModel: createSortedRowModel() });
  const sortFn = constructSortFn({ sort: compareKeys });
  // The accessor of a column gives the value the column sorts by; the cells are drawn from the task.
  const columnDefs = $derived(
    specs.map((spec) => ({
      id: spec.id,
      header: spec.label,
      accessorFn: (t: TaskView) => sortKey(t, spec.id, ctx),
      sortFn,
      sortUndefined: "last" as const,
    })),
  );
  // The order of the headers is not saved: a sort to keep is written in the filter as sort:.
  let sorting = $state<SortingState>([]);
  const table = createTable({
    features,
    // A click sorts ascending first, then descending, then back to the order of the query.
    sortDescFirst: false,
    enableMultiSort: false,
    getRowId: (t: TaskView) => t.ulid,
    get columns() {
      return columnDefs;
    },
    get data() {
      return tasks;
    },
    state: {
      get sorting() {
        return sorting;
      },
    },
    onSortingChange: (u) => {
      sorting = typeof u === "function" ? u(sorting) : u;
    },
  });
  const headers = $derived(table.getHeaderGroups()[0]?.headers ?? []);
  const sorted = $derived(table.getRowModel().rows.map((r) => r.original));

  // The rows and the headings of the groups, flat.
  const items = $derived(groupTasks(sorted, settings.groupBy, ctx));
  const GUTTER = 1.75; // rem, the mark of a save on its way or failed
  const template = $derived(
    `${GUTTER}rem ${specs.map((s) => (s.id === "title" ? `minmax(${s.width}rem, 1fr)` : `${s.width}rem`)).join(" ")}`,
  );
  const minWidth = $derived(GUTTER + specs.reduce((sum, s) => sum + s.width, 0));

  // --- editing a cell --------------------------------------------------------------------------
  function edit(t: TaskView, field: PatchField, value: unknown) {
    c.save.edit(t.ulid, t.store, field, value);
  }
  const commit = (t: TaskView) => void c.save.flush(t.ulid);

  // --- virtual scrolling -------------------------------------------------------------------
  const virtualized = $derived(items.length > VIRTUALIZE_AT);
  let scroller = $state<HTMLDivElement | null>(null);
  type Virtualizer = {
    getVirtualItems(): { index: number; start: number }[];
    getTotalSize(): number;
    setOptions(o: { count: number; getScrollElement: () => HTMLDivElement | null }): void;
  };
  let vinst = $state.raw<Virtualizer | null>(null);
  let windowed = $state<{ index: number; start: number }[]>([]);
  let totalSize = $state(0);
  // The key of an item: the rows keep their elements when the window moves or the list changes.
  function itemKey(i: number): string {
    const it = items[i];
    return it ? (it.type === "row" ? it.key : `#${it.key}`) : `#${i}`;
  }
  const virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: 0,
    getScrollElement: () => scroller,
    estimateSize: (i) => (items[i]?.type === "group" ? GROUP_HEIGHT : ROW_HEIGHT),
    getItemKey: itemKey,
    overscan: 12,
  });
  $effect(() =>
    virtualizer.subscribe((v) => {
      vinst = v as unknown as Virtualizer;
      windowed = v.getVirtualItems();
      totalSize = v.getTotalSize();
    }),
  );
  // The window is only asked for while the list is long.
  $effect(() => {
    vinst?.setOptions({
      count: virtualized ? items.length : 0,
      getScrollElement: () => scroller,
    });
  });

  // --- the controls ------------------------------------------------------------------------------
  const GROUPS: { value: GroupBy; label: string }[] = [
    { value: "none", label: "なし" },
    { value: "parent", label: "親" },
    { value: "status", label: "ステータス" },
    { value: "assignee", label: "担当者" },
    { value: "project", label: "プロジェクト" },
  ];
  const groupLabel = $derived(GROUPS.find((g) => g.value === settings.groupBy)?.label ?? "なし");
  let menu = $state<{ which: "group" | "columns"; anchor: HTMLElement } | null>(null);
  const shownIds = $derived(new Set(shown.map((s) => s.id)));
  const columnItems = $derived<MenuItem[]>(
    candidates.map((s) => {
      const last = shownIds.size === 1 && shownIds.has(s.id);
      return {
        value: s.id,
        label: s.label,
        disabled: last,
        hint: last ? "最後の 1 列は外せません" : undefined,
      };
    }),
  );
  function pickColumn(id: string) {
    onsettings({ ...settings, columns: toggleColumn(settings.columns, id, !shownIds.has(id)) });
  }
  function pickGroup(value: string) {
    menu = null;
    onsettings({ ...settings, groupBy: value as GroupBy });
  }

  // --- reading the screen -----------------------------------------------------------------------
  const lsCli = $derived(
    cliLs(
      joinQuery(scope.cliBase, query),
      false,
      view.kind === "workspace" ? view.store : undefined,
    ),
  );
  // Where the settings of the table would be saved as a view; the name is the user's to give.
  const saveCli = $derived(
    cliViewSave({
      name: new Placeholder("名前"),
      body: {
        layout: "table",
        query: joinQuery(scope.saveBase, query),
        columns: settings.columns,
        group_by: settings.groupBy,
      },
    }),
  );
  const noTasks = $derived(
    withoutTerm(query, NO_CANCELED).trim() === "" && !(canceledCount && canceledCount > 0),
  );
  const crumbTitle = $derived(
    view.kind === "project" ? (scope.project?.title ?? "") : (scope.workspace?.name ?? ""),
  );
  // Spread, because Biome reads a prop called scope as the HTML attribute.
  const emptyProps = $derived({
    scope: view.kind === "workspace" ? ("workspace" as const) : ("project" as const),
    name: crumbTitle,
  });
  const notFoundProps = $derived({
    scope: view.kind === "workspace" ? ("workspace" as const) : ("project" as const),
    name: view.kind === "workspace" ? view.ulid : "",
  });
  const labelOf = (id: string) => specs.find((s) => s.id === id)?.label ?? id;
  const groupMeta = (g: Extract<TableItem, { type: "group" }>) =>
    g.progress ? `${g.progress.done}/${g.progress.total} 完了` : "";
</script>

{#snippet controls()}
  <div class="controls">
    <Button
      size="sm"
      aria-haspopup="menu"
      aria-expanded={menu?.which === "group"}
      onclick={(e) =>
        (menu = menu?.which === "group" ? null : { which: "group", anchor: e.currentTarget })}
    >
      グループ: {groupLabel}
    </Button>
    <Button
      size="sm"
      aria-haspopup="menu"
      aria-expanded={menu?.which === "columns"}
      onclick={(e) =>
        (menu = menu?.which === "columns" ? null : { which: "columns", anchor: e.currentTarget })}
    >
      列 {shown.length}
    </Button>
    <!-- The name is not given yet, so the line is not complete to copy. -->
    <CliLine command={saveCli} disabled />
  </div>
{/snippet}

{#snippet columnItem(it: MenuItem)}
  <span class="check" class:on={shownIds.has(it.value)} aria-hidden="true">
    {#if shownIds.has(it.value)}
      <Icon name="check" size={12} />
    {/if}
  </span>
  <span class="item-label">{it.label}</span>
  <span class="sr">{shownIds.has(it.value) ? "表示中" : "非表示"}</span>
{/snippet}

{#snippet entry(item: TableItem, start?: number)}
  {#if item.type === "group"}
    <tr
      class="group"
      class:placed={start !== undefined}
      style:transform={start === undefined ? undefined : `translateY(${start}px)`}
    >
      <td class="ghead" colspan={specs.length + 1}>
        <span class="gname">{item.label}</span>
        {#if item.id}
          <span class="gid">{item.id}</span>
        {/if}
        {#if groupMeta(item)}
          <span class="gmeta">{groupMeta(item)}</span>
        {/if}
        <span class="gmeta">{item.count} 件</span>
      </td>
    </tr>
  {:else}
    {@const t = item.task}
    {@const st = saves.current.state(t.ulid)}
    <tr
      class="row"
      class:placed={start !== undefined}
      style:transform={start === undefined ? undefined : `translateY(${start}px)`}
      data-ulid={t.ulid}
    >
      <td class="mark">
        {#if st.kind === "saving"}
          <span role="img" aria-label="保存中" title="保存中…">…</span>
        {:else if st.kind === "error"}
          <button
            type="button"
            class="retry"
            aria-label="保存に失敗しました。やり直す: {st.message}"
            title={st.message}
            onclick={() => commit(t)}
          >
            <Icon name="warning" size={14} />
          </button>
        {/if}
      </td>
      {#each specs as spec (spec.id)}
        {@const field = patchFieldOf(spec)}
        <td class="cell" class:link={spec.id === "id" || spec.id === "parent"}>
          <TableCell
            task={t}
            col={spec}
            schema={schemaOf(t.store)}
            tone={c.stores.tone(t.store)}
            pending={field ? saves.current.editing(t.ulid, field) : false}
            {taskOf}
            onedit={(f, v) => edit(t, f, v)}
            oncommit={() => commit(t)}
          />
        </td>
      {/each}
    </tr>
  {/if}
{/snippet}

<section class="table" aria-label="テーブル">
  <ListBar
    base={scope.cliBase}
    applied={query}
    count={countLabel(tasks.length, canceledCount)}
    cli={cli ?? lsCli}
    error={listError}
    onchange={onquery}
    {controls}
    {actions}
  />

  <div class="body">
    <div class="scroller" bind:this={scroller}>
      <div class="inner" style:--cols={template} style:min-width="{minWidth}rem">
        <!-- The table elements are laid out as a grid, so that the columns line up and the
          virtualizer can place a row; they keep their roles. -->
        <table aria-label="タスクの表" aria-colcount={specs.length + 1}>
          <thead>
            <tr class="head">
              <th class="hcell" scope="col"><span class="sr">保存の状態</span></th>
              {#each headers as h (h.id)}
                {@const dir = h.column.getIsSorted()}
                <th
                  class="hcell"
                  scope="col"
                  aria-sort={dir === "asc" ? "ascending" : dir === "desc" ? "descending" : "none"}
                >
                  <button type="button" class="sort" onclick={h.column.getToggleSortingHandler()}>
                    {labelOf(h.column.id)}
                    <span class="dir" aria-hidden="true">
                      {dir === "asc" ? "↑" : dir === "desc" ? "↓" : ""}
                    </span>
                  </button>
                </th>
              {/each}
            </tr>
          </thead>
          {#if virtualized}
            <tbody class="vbox" style:height="{totalSize}px">
              {#each windowed as v (itemKey(v.index))}
                {@const item = items[v.index]}
                {#if item}
                  {@render entry(item, v.start)}
                {/if}
              {/each}
            </tbody>
          {:else}
            <tbody>
              {#each items as item (item.type === "row" ? item.key : `#${item.key}`)}
                {@render entry(item)}
              {/each}
            </tbody>
          {/if}
        </table>

        {#if scope.notFound}
          <Empty
            variant="notfound"
            {...notFoundProps}
            onHome={() => router.go(router.href({ name: "home" }))}
          />
        {:else if loading && items.length === 0}
          <p class="note">読み込み中…</p>
        {:else if tasks.length === 0 && !listError}
          {#if view.kind !== "all" && noTasks}
            <Empty
              variant="empty"
              {...emptyProps}
              onCreate={() =>
                openCreate({
                  kind: "task",
                  parent: view.kind === "project" ? view.ulid : undefined,
                })}
            />
          {:else}
            <Empty
              variant="nomatch"
              note={hasTerm(query, NO_CANCELED) ? "中止を除外しています" : undefined}
              onClear={() => onquery("")}
            />
          {/if}
        {/if}
        {#each schemaFailed as store (store)}
          <p class="note" role="alert">{store} の schema.yaml を読み込めていません</p>
        {/each}
      </div>
    </div>
  </div>

  <footer class="foot">
    <span
      >セルは直接編集できます。入力が 2 秒止まるか、フォーカスが外れたときに 1 操作 1
      コミットで保存します。</span
    >
    <span
      >見出しのクリックの並び替えは保存されません。保存したい並びはフィルタ欄の sort:
      に書きます。</span
    >
  </footer>
</section>

{#if menu?.which === "group"}
  <Menu
    anchor={menu.anchor}
    items={GROUPS}
    label="グループ"
    selected={settings.groupBy}
    onselect={pickGroup}
    onclose={() => (menu = null)}
  />
{:else if menu?.which === "columns"}
  <Menu
    anchor={menu.anchor}
    items={columnItems}
    label="列"
    item={columnItem}
    onselect={pickColumn}
    onclose={() => (menu = null)}
  />
{/if}

<style>
  .table {
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }
  .controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
    padding: 0 var(--gutter) var(--space-8);
  }
  .body {
    position: relative;
    flex: 1 1 auto;
    min-height: 0;
  }
  .scroller {
    box-sizing: border-box;
    height: 100%;
    overflow: auto;
    padding: 0 var(--space-16);
  }
  .inner {
    position: relative;
  }
  /* The table is a grid of rows; every row has the same columns (--cols). */
  table,
  thead,
  tbody {
    display: grid;
    margin: 0;
    padding: 0;
    border-collapse: collapse;
  }
  thead {
    position: sticky;
    top: 0;
    z-index: 4;
    background: var(--surface);
  }
  tr {
    display: grid;
    grid-template-columns: var(--cols);
    align-items: center;
  }
  .head {
    height: var(--control-h);
    border-bottom: var(--hairline) solid var(--line);
  }
  .hcell {
    min-width: 0;
    padding: 0 var(--space-4);
    font-weight: normal;
    text-align: left;
  }
  .sort {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    max-width: 100%;
    padding: 0 var(--space-8);
    border: 0;
    border-radius: var(--radius-4);
    background: transparent;
    color: var(--muted);
    font: inherit;
    font-size: var(--fs-12);
    font-weight: 600;
    cursor: pointer;
  }
  .sort:hover {
    background: var(--hover);
    color: var(--ink);
  }
  .sort:focus-visible {
    outline: 2px solid var(--accent);
  }
  .dir {
    color: var(--accent);
  }
  .vbox {
    position: relative;
    width: 100%;
  }
  /* The virtualizer places a row by its offset. */
  .placed {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
  }
  .row {
    height: var(--row-h);
    border-bottom: var(--hairline) solid var(--line-soft);
  }
  .row:hover {
    background: var(--sunken);
  }
  /* The ID pops its ULID over the rows below it. */
  .row:hover,
  .row:focus-within {
    position: relative;
    z-index: 3;
  }
  .row.placed:hover,
  .row.placed:focus-within {
    position: absolute;
  }
  /* A cell keeps to its row: chips that do not fit are cut off until the cell has the focus. The
    ID and the parent pop their ULID out of the cell. */
  .cell {
    display: flex;
    align-items: center;
    min-width: 0;
    height: var(--row-h);
    padding: 0 var(--space-4);
    overflow: hidden;
  }
  .cell.link,
  .cell:focus-within {
    overflow: visible;
  }
  .cell > :global(*) {
    min-width: 0;
  }
  /* The chips of a cell stay on one line, so that the row keeps its height. */
  .cell :global(.chips) {
    flex-wrap: nowrap;
  }
  .mark {
    display: flex;
    justify-content: center;
    color: var(--muted);
  }
  .retry {
    display: inline-flex;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--danger);
    cursor: pointer;
  }
  .group {
    display: flex;
    align-items: center;
    height: var(--control-h);
    background: var(--sunken);
    border-bottom: var(--hairline) solid var(--line-soft);
  }
  .ghead {
    display: flex;
    align-items: baseline;
    gap: var(--space-8);
    padding: 0 var(--space-12);
    font-size: var(--fs-13);
  }
  .gname {
    font-weight: 600;
    color: var(--ink);
  }
  .gid {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    color: var(--muted);
  }
  .gmeta {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .note {
    margin: var(--space-16) var(--space-12);
    color: var(--muted);
  }
  .foot {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-4) var(--space-16);
    padding: var(--space-10) var(--gutter);
    border-top: var(--hairline) solid var(--line);
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .check {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    border: 1px solid var(--line-input);
    border-radius: var(--radius-4);
    background: var(--surface);
    color: var(--accent-on);
  }
  .check.on {
    border-color: var(--accent);
    background: var(--accent);
  }
  .item-label {
    flex: 1;
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }
</style>
```

- [ ] **Step 13: 通ることを確かめる**

```bash
cd web && pnpm vitest run app/table
```

期待: PASS（`columns.test.ts` 14 件・`group.test.ts` 6 件・`TableView.test.ts` 32 件）。

- [ ] **Step 14: 検証とコミット**

Run: `cd web && pnpm fix && pnpm check && pnpm test`
Expected: 全件 PASS、`svelte-check` の警告 0、`biome` の指摘 0

この Task は画面に入口を出さない。E2E（テーブルのセル編集）は Task 7 の結線のあとで流す。

```bash
git add web/app/table
git commit -m "feat(web): テーブル（列の選択・セルの編集・並び替え・グループ・仮想化）"
```

**この Task で決めたこと**（plan の「決めたこと」へ）:
- **列の候補**: 組み込み 9 個、続けてスキーマのプロパティの和を名前順。ラベルは最初にそのプロパティを持つ Store のもの。`blocked_by` と、組み込みと同じ名前のプロパティは候補にしない。候補にする Store は、1 Store の画面ならその Store、それ以外は表示中のタスクの Store（タスクがまだなければ全 Store）
- **表示する列**: 保存した `table.columns` の順。どの候補にもない名前は出さず、保存した値からは消さない（メニューで列を足すときも残す）。最後の 1 列はメニューで外せない（`columns: []` はサーバーの既定の列になり、画面と食い違うため）。メニューで足した列は末尾に付く（外してから足し直すと末尾へ動く。ドラッグで順を変える操作は作らない）
- **複数 Store**: 全 Store の画面で Store が 2 つ以上あるときは、先頭に Store の列（バッジ。並び替えはできるが、保存する列ではない）を出す。ある Store のスキーマにない列は、その Store のタスクのセルだけ斜体の「（未定義）」
- **セルの編集**: ID・親・作成・更新と `task_ref` のプロパティは読み取りだけ。ステータスの選択肢は行の Store のスキーマの全ステータス（詳細パネルと同じ。scoped の `statuses` を使うのはカンバンだけ）、種類は詳細パネルと同じ `canParent` で置けないものを無効にする
- **並び替え**: 見出しのクリックで 昇順 → 降順 → クエリの順。1 列だけ。保存しない（細部 46）。値のない行は常に最後。ステータスは sort:status の順、ID・タイトルなどの文字は数を数として比べる（`REL-2` は `REL-10` の前）、数・期日は値の順
- **グループ**: 親とプロジェクトは並べ替えのあとの初出の順（「親なし」「プロジェクトなし」は最後）、ステータスは sort:status の順（同じ名前は 1 つ）、担当者は名前順（複数担当のタスクは各担当者のグループに出し、未割り当ては最後）。見出しは、親なら ID・タイトル・進捗（`done/total 完了`）、どれも件数。親がキャッシュにないときは「不明な親」。見出しの折りたたみは作らない
- **仮想化**: 見出しも含めた行が 200 を超えたときだけ（ツリーと同じ `VIRTUALIZE_AT`）。行 40px・見出し 36px
- **保存の状態**: 左端の細い列に、保存中（…）と失敗（警告のボタン。押すと送り直す）を出す。「保存済み」は出さない
- **足元**: `cli` があればそれ、なければ `hh ls …`（ワークスペースの画面だけ `--store`）。設定の隣に `hh view save <名前> --layout table --query … --columns … --group-by …`（D10。名前を入れるまではコピーさせない）。`--query` は `scope.saveBase`（D8 の固定の部分）とフィルタ欄を足したもの
- **件数と空の状態**: ツリーと同じ件数の言い方と中止の件数。タスクがない画面は、ワークスペース・プロジェクトで、絞り込みがなければ `empty`（作成ボタンつき）、そうでなければ `nomatch`（「フィルタを解除」）。全 Store の画面は `nomatch` だけ
- **TanStack Table の持ち分**: 列の定義・見出し・並び替えの状態・並べ替えた行の順。セルとグループの見出しは自前で描く。仮想化とグループは TanStack の feature ではなく、並べ替えのあとの配列に対して行う
- **部 A との境目**: `useListScope().schemas()` は読み込み済みを渡した順に返して `c.schema` の変化で読み直す。`ListBar` は `base` が `""` のとき固定の部分の札を出さない。どちらも Task 2 が対応する（形は Task 2 の実物で確かめ、違えば合わせる）
- **プロジェクトの画面のテーブルのステータスの選択肢**: 全ステータス（詳細パネルと同じ）。scoped の `statuses` で隠したステータスのタスクも直せる
- **列の順の変更**: 「外して足し直す（末尾に動く）」だけ。↑↓の操作は作らない（細部 48 は選択のチェックボックスだけを求めている）
- **sort:status の順**: Task 4 の `core/status-order.ts` を共有する（並び替えの `status` の値とグループの順）
### Task 6: ビューの画面・クエリの画面・ビューのダイアログ・ワークスペースとプロジェクトのカンバン・テーブルのタブの入れ物（細部 50〜54、D1・D6〜D10）

**Files:**
- Modify: `web/app/ui/Tabs.svelte`、`web/app/ui/tabs.test.ts`（タブに `onselect`。押すとその場で動き、移動しない）
- Modify: `web/app/states/Empty.svelte`、`web/app/states/Empty.test.ts`（`notfound` の `scope` に `"view"`）
- Create: `web/app/views/draft.ts`、`draft.test.ts`（レイアウトの名前、設定の要約、ツリーの `sort:`・`group:` の判定、ビューの名前の同一判定、新しいドラフト）
- Create: `web/app/views/dialogs-state.svelte.ts`、`web/app/views/testkit.ts`、`web/app/views/fakes/FakeTree.svelte`・`FakeKanban.svelte`・`FakeTable.svelte`（テストの部品。画面のテストが TreeView・KanbanView・TableView の代わりに置く）
- Create: `web/app/views/ViewSave.svelte`・`ViewRename.svelte`・`ViewDelete.svelte`・`ViewDialogs.svelte` と、それぞれの `*.test.ts`
- Create: `web/app/list/SaveViewButton.svelte`・`ListTab.svelte` と、それぞれの `*.test.ts`
- Create: `web/app/views/QueryPage.svelte`・`ViewPage.svelte` と、それぞれの `*.test.ts`
- この Task では `App.svelte`・`WorkspacePage.svelte`・`WorkspaceHeader.svelte`・サイドバー・パレットを触らない（入口の結線は Task 7）

**Interfaces:**
- Consumes（部 A〜D の最終形。この回のコードは、これと同じ名前と型の最小のスタブで動かして確かめた。実物に替わっても、画面のテストは TreeView・KanbanView・TableView・`useListScope` を `vi.mock` で置き換えるので、実物の中身に依存しない）:
  - `web/app/router.ts`（Task 2）: `Layout`、`Route` の `{ name: "view"; store; view }`・`{ name: "query"; layout; q }`、`router.href(route)`・`router.route`・`router.go(path, { replace? })`・`router.replace(path)`（履歴を積まず、購読者に知らせる）、`hrefOf({ name: "home" })`。パスは `/views/<store>/<名前>`、`/query/<layout>?q=`
  - `web/core/views.ts`（Task 2）: `ViewDraft`・`KanbanSettings`・`TableSettings`・`DEFAULT_KANBAN`・`DEFAULT_TABLE`・`draftOf`・`saveBodyOf`・`sameDraft`・`ViewEntry`・`SavedView`・`ViewSaveBody`、`ViewDirectory` の `list()`・`get(store, name)`・`loaded()`・`subscribe()`・`save(store, name, body)`・`rename(store, name, to)`・`remove(store, name)`（`save`・`rename` は応答の view を返し、一覧にも即座に反映する）。`AppCore.views`
  - `web/core/cli.ts`（Task 2）: `cliViewSave({ name: Arg; store?: string; body: ViewSaveBody })`・`cliViewShow(name, store?)`・`cliViewRm(name, store?)`・`cliViewRename(name, to: Arg, store?)`、`Placeholder`
  - `web/app/list/scope.ts`・`scope.svelte.ts`（Task 2）: `ListScope`、`useListScope(get)` の `ready`・`store`・`saveBase`（D8 の固定の部分。Store が2つ以上なら先頭に `store:<Store> ` が入っている。all は `""`）
  - `web/app/tree/TreeView.svelte`（Task 3）: props `{ view: ListScope; query?: string; onquery?: (q: string) => void; cli?: string; actions?: Snippet }`。**`query` が `undefined` でなければ外から持つ（`""` を含む）**ことを前提にする
  - `web/app/kanban/KanbanView.svelte`（Task 4）・`web/app/table/TableView.svelte`（Task 5）: props `{ view: ListScope; query: string; onquery; settings; onsettings; cli?: string; actions?: Snippet }`
  - 既存: `core()`、`useSubscribe`、`toast`・`toasts`・`resetToasts`、`ui/Dialog.svelte`・`Button.svelte`・`CliLine.svelte`・`Menu.svelte`・`StoreBadge.svelte`・`FormError.svelte`・`Tabs.svelte`、`states/Empty.svelte`、`errorMessage`・`ApiError`、`runUndo`（`app/undo.ts`）、`tree/filter.ts` の `NO_CANCELED`・`combine`・`splitTerms`、`detail/testkit.ts` の `makeCore`、`test-utils.ts` の `renderWithCore`
- Produces（Task 7 が使う）:
  - `web/app/views/dialogs-state.svelte.ts`: `openViewSave(i: { draft: ViewDraft; store?: string })`・`openViewRename(entry: ViewEntry)`・`openViewDelete(entry: ViewEntry)`・`closeViewDialog()`・`viewDialog.current`（`{ kind: "save"; draft; store? } | { kind: "rename"; entry } | { kind: "delete"; entry } | null`）
  - `web/app/views/ViewDialogs.svelte`（props なし。App に1つ、`<WorkspaceDialogs />` の次に置く）
  - `web/app/views/ViewPage.svelte`（`{ store: string; name: string }`）。App は Store と名前をつないだキー（`{#key}`）で包む
  - `web/app/views/QueryPage.svelte`（`{ layout: Layout; q: string }`）。App は**キーで包まない**（レイアウトのタブを替えてもカンバン・テーブルの設定が残る）
  - `web/app/list/ListTab.svelte`（`{ view: ListScope; layout: "kanban" | "table" }`）。WorkspacePage とプロジェクトの画面は、kanban と table を**同じ `<ListTab layout={tab}>` の1か所**で描く（タブを替えてもフィルタが残る）
  - `web/app/list/SaveViewButton.svelte`（`{ view: ListScope; layout: Layout; filter: string; kanban?: KanbanSettings; table?: TableSettings }`）。ツリーのタブの「ビューを保存」は、Task 7 がこれを `TreeView` の `actions` に入れる（`filter` は Task 7 が持つ `query`）
  - `web/app/views/draft.ts`: `LAYOUTS`、`summaryOf(d)`、`treeOrderProblem(d)`、`sameViewName(a, b)`、`newDraft(i)`
  - `web/app/ui/Tabs.svelte`: `TabItem` に `onselect?: () => void`（あれば、押すとその場で `onselect` を呼ぶ。⌘/Ctrl/⇧ を押していれば `href` を新しいタブで開く）。同じ `href` の項目が並んでもよい（key は `label`）
  - `web/app/states/Empty.svelte`: `notfound` の `scope="view"`（「ビュー「<名前>」が見つかりません」）

**この Task で決めたこと**
- 保存済みビューの画面（`ViewPage`）は、保存した値（`draftOf`）の上に「画面の値」（クエリ・カンバンの設定・テーブルの設定）を持つ。レイアウトはいつも保存した値で、タブは `PUT {layout}` だけを送り、未保存の変更は送らない・捨てない（D6）。クエリ・設定のどれかが保存した値と違えば「未保存の変更」と「ビューを更新」（1回の `PUT` に全部。レイアウトは保存した値）を出す。更新の足元に、同じ操作の `hh view save` を出す。ツリーでクエリに `sort:`・`group:` があれば、更新を無効にして理由を出す（細部 54）
- ビューの画面の足元の行は `hh view show <名前> --store <Store>`（D10）。3つの部品の `cli` に渡す。タブの下に、レイアウトを変える `hh view save <名前> --layout … --store …` と「切り替えは保存したビューの layout を変えます（1 コミット）」を出す（キャンバスの Table）
- ビューが見つからない（消された・改名された）ときは、`ViewDirectory` の読み込みが終わっていれば Empty の `notfound`、終わっていなければ「読み込み中…」（D9）
- クエリの画面（`QueryPage`）は、フィルタ欄の変更を `router.replace` で URL の `q` に書く。戻ってきた `q` が今の画面の値と同じなら何もしない（入力中の欄を巻き戻さない）。`q` が外から変わった（戻る・進む）ときは、それに従う。タブは `q` を持ったまま `/query/<layout>` へのリンク。画面の既定のフィルタは足さない（D1・D7）。保存先の Store は決めずに「ビューを保存」のダイアログを開く（既定の Store で始まる）
- `ListTab` は、フィルタ（既定 `-status:canceled`）とカンバン・テーブルの設定を、画面を開いているあいだだけ持つ（D7。localStorage も使わない）。「ビューを保存」のクエリは `SaveViewButton` が `useListScope.saveBase`（`workspace:<名前>` / `project:<ID>`、Store が2つ以上なら `store:<Store>` つき）とフィルタをつないで作り、保存先の既定をその画面の Store にする（D8）
- 保存のダイアログ: 名前、保存先の Store（2つ以上のときだけ選ぶ。既定は渡された Store、なければ `default` の Store、なければ先頭。閲覧のみの Store は選べない）、設定の要約とクエリ、足元の `hh view save`。名前の規則はサーバーの `view.bad_name` などの文言をそのまま出す。保存したら `/views/<store>/<名前>` を開き、トーストを出す（D8）。同じ Store に同じ名前（大文字小文字・正規化の違いを含む。サーバーの `SameName`）があれば、`PUT` が置き換えてしまうので保存せず、理由を出す（細部 53 のとおり別の Store には同じ名前があってよい）
- 改名・削除のダイアログ（細部 52）は `ViewPage` の「…」から。開いているのがそのビューの画面なら、改名は新しい名前の画面へ（履歴を積まない）、削除はホームへ移る。削除のトーストには、ワークスペースの削除と同じく取り消し（`runUndo`、そのビューの Store）を付ける
- 画面のテストは TreeView・KanbanView・TableView を `fakes/` の部品に、`SaveViewButton`・`ListTab` のテストは `useListScope` を値に置き換える。実物の画面の中身は部 B〜D のテストと Task 7 の E2E が確かめる

- [ ] **Step 1: タブの `onselect` と Empty の `view` のテストを書く（失敗する）**

`web/app/ui/tabs.test.ts` の差分:

```diff
--- a/web/app/ui/tabs.test.ts
+++ b/web/app/ui/tabs.test.ts
@@ -1,5 +1,5 @@
 import { cleanup, fireEvent, render, within } from "@testing-library/svelte";
-import { afterEach, beforeEach, describe, expect, it } from "vitest";
+import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
 import Tabs from "./Tabs.svelte";
 
 beforeEach(() => history.replaceState(null, "", "/"));
@@ -46,6 +46,34 @@
     expect(location.pathname).toBe("/workspaces/shared/W1/tree");
   });
 
+  it("onselect のあるタブは、押すとその場で onselect を呼び、移動しない", async () => {
+    const onselect = vi.fn();
+    const same = "/views/shared/weekly";
+    const { getByRole } = render(Tabs, {
+      items: [
+        { label: "ツリー", href: same, current: true },
+        { label: "カンバン", href: same, current: false, onselect },
+      ],
+    });
+    const ev = new MouseEvent("click", { button: 0, bubbles: true, cancelable: true });
+    getByRole("link", { name: "カンバン" }).dispatchEvent(ev);
+    expect(ev.defaultPrevented).toBe(true);
+    expect(onselect).toHaveBeenCalledTimes(1);
+    expect(location.pathname).toBe("/");
+  });
+
+  it("onselect のあるタブも、⌘ を押して押すと新しいタブで href を開く", async () => {
+    const onselect = vi.fn();
+    const open = vi.spyOn(window, "open").mockReturnValue(null);
+    const { getByRole } = render(Tabs, {
+      items: [{ label: "カンバン", href: "/views/shared/weekly", current: false, onselect }],
+    });
+    await fireEvent.click(getByRole("link", { name: "カンバン" }), { metaKey: true });
+    expect(onselect).not.toHaveBeenCalled();
+    expect(open).toHaveBeenCalledWith(`${location.origin}/views/shared/weekly`, "_blank");
+    open.mockRestore();
+  });
+
   it("label でナビゲーションの名前を変えられる", () => {
     const { getByRole } = render(Tabs, { items, label: "プロジェクトのタブ" });
     expect(getByRole("navigation", { name: "プロジェクトのタブ" })).toBeTruthy();
```

`web/app/states/Empty.test.ts` の差分:

```diff
--- a/web/app/states/Empty.test.ts
+++ b/web/app/states/Empty.test.ts
@@ -43,6 +43,15 @@
     expect(getByText("Store「nope」が見つかりません")).toBeTruthy();
   });
 
+  it("notfound: ビューの名前を出す", () => {
+    const { getByText } = renderWithCore(Empty, {
+      variant: "notfound",
+      scope: "view",
+      name: "今週のリリース作業",
+    });
+    expect(getByText("ビュー「今週のリリース作業」が見つかりません")).toBeTruthy();
+  });
+
   it("notfound: プロジェクトは名前を出さない", () => {
     const { getByText } = renderWithCore(Empty, { variant: "notfound", scope: "project" });
     expect(getByText("プロジェクトが見つかりません")).toBeTruthy();
```

Run: `cd web && pnpm vitest run app/ui/tabs.test.ts app/states/Empty.test.ts`
Expected: FAIL（`onselect` のあるタブが `href` へ移動してしまう。`scope="view"` の文言が「ワークスペース「…」」になる）

- [ ] **Step 2: `Tabs.svelte` と `Empty.svelte` を直す**

`web/app/ui/Tabs.svelte` の差分:

```diff
--- a/web/app/ui/Tabs.svelte
+++ b/web/app/ui/Tabs.svelte
@@ -1,7 +1,8 @@
 <script lang="ts" module>
   import type { IconName } from "../icons/Icon.svelte";
 
-  export type TabItem = { label: string; href: string; current: boolean };
+  /** onselect: the tab does something in place (the layout of a saved view); href is where a new tab opens. */
+  export type TabItem = { label: string; href: string; current: boolean; onselect?: () => void };
   /** The tab at the right end (the settings), which can have an icon. */
   export type TabEnd = TabItem & { icon?: IconName };
 </script>
@@ -12,17 +13,27 @@
 
   type Props = { items: TabItem[]; end?: TabEnd; label?: string };
   let { items, end, label = "ビューの切り替え" }: Props = $props();
+
+  function pick(e: MouseEvent, it: TabItem) {
+    const plain = e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
+    if (it.onselect && plain && !e.defaultPrevented) {
+      e.preventDefault();
+      it.onselect();
+      return;
+    }
+    onLinkClick(e, it.href);
+  }
 </script>
 
 <!-- The tabs are links, so the back button, copying the address and ⌘/Ctrl-click work. -->
 <nav class="tabs" aria-label={label}>
-  {#each items as it (it.href)}
+  {#each items as it (it.label)}
     <a
       class="tab"
       class:current={it.current}
       href={it.href}
       aria-current={it.current ? "page" : undefined}
-      onclick={(e) => onLinkClick(e, it.href)}
+      onclick={(e) => pick(e, it)}
       >{it.label}</a
     >
   {/each}
@@ -32,7 +43,7 @@
       class:current={end.current}
       href={end.href}
       aria-current={end.current ? "page" : undefined}
-      onclick={(e) => onLinkClick(e, end.href)}
+      onclick={(e) => pick(e, end)}
     >
       {#if end.icon}
         <Icon name={end.icon} size={14} />
```

`web/app/states/Empty.svelte` の差分:

```diff
--- a/web/app/states/Empty.svelte
+++ b/web/app/states/Empty.svelte
@@ -6,7 +6,7 @@
 
   type Props = {
     variant: "empty" | "nomatch" | "noworkspace" | "notfound";
-    scope?: "project" | "workspace" | "store";
+    scope?: "project" | "workspace" | "store" | "view";
     name?: string;
     note?: string;
     onCreate?: () => void;
@@ -30,7 +30,9 @@
       ? "プロジェクト"
       : scope === "store"
         ? `Store「${name}」`
-        : `ワークスペース「${name}」`,
+        : scope === "view"
+          ? `ビュー「${name}」`
+          : `ワークスペース「${name}」`,
   );
 </script>
 
```

Run: `cd web && pnpm vitest run app/ui app/states`
Expected: PASS（既存のタブのテストも通る。key を `href` から `label` に替えたが、同じタブバーの中で label は重ならない）

- [ ] **Step 3: `draft.ts` のテストを書く（失敗する）**

`web/app/views/draft.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { DEFAULT_KANBAN, DEFAULT_TABLE, type ViewDraft } from "../../core/views";
import { newDraft, sameViewName, summaryOf, treeOrderProblem } from "./draft";

const draft = (over: Partial<ViewDraft> = {}): ViewDraft => ({
  layout: "tree",
  query: "",
  kanban: DEFAULT_KANBAN,
  table: DEFAULT_TABLE,
  ...over,
});

describe("summaryOf", () => {
  it("ツリーは名前だけ", () => {
    expect(summaryOf(draft())).toBe("ツリー");
  });

  it("カンバンは列とスイムレーン", () => {
    expect(summaryOf(draft({ layout: "kanban" }))).toBe(
      "カンバン · 列: ステータス · スイムレーン: なし",
    );
    expect(
      summaryOf(draft({ layout: "kanban", kanban: { columns: "category", swimlane: "assignee" } })),
    ).toBe("カンバン · 列: カテゴリ · スイムレーン: 担当者");
  });

  it("テーブルは列の数とグループ", () => {
    expect(summaryOf(draft({ layout: "table" }))).toBe("テーブル · 列 5 · グループ: なし");
    expect(
      summaryOf(draft({ layout: "table", table: { columns: ["id", "title"], groupBy: "parent" } })),
    ).toBe("テーブル · 列 2 · グループ: 親");
  });
});

describe("treeOrderProblem", () => {
  it("ツリーで sort: か group: があれば、サーバーと同じ理由を返す", () => {
    const why = "ツリーのビューは rank の順に並ぶため、group: と sort: は使えません";
    expect(treeOrderProblem({ layout: "tree", query: "kind:task sort:updated" })).toBe(why);
    expect(treeOrderProblem({ layout: "tree", query: "group:status" })).toBe(why);
  });

  it("ツリーでも、引用符の中の sort: や、ほかの語は問題にしない", () => {
    expect(treeOrderProblem({ layout: "tree", query: 'title:"a sort:b" assignee:@me' })).toBeNull();
    expect(treeOrderProblem({ layout: "tree", query: "" })).toBeNull();
  });

  it("カンバンとテーブルは sort:・group: を持てる", () => {
    expect(treeOrderProblem({ layout: "table", query: "sort:updated" })).toBeNull();
    expect(treeOrderProblem({ layout: "kanban", query: "group:status" })).toBeNull();
  });
});

describe("newDraft", () => {
  it("設定を渡さなければ、レイアウトの既定", () => {
    expect(newDraft({ layout: "table", query: "q" })).toEqual({
      layout: "table",
      query: "q",
      kanban: DEFAULT_KANBAN,
      table: DEFAULT_TABLE,
    });
  });
});

describe("sameViewName", () => {
  it("大文字小文字と Unicode の正規化の違いは同じ名前", () => {
    expect(sameViewName("Weekly", "weekly")).toBe(true);
    expect(sameViewName("が", "か\u3099")).toBe(true);
    expect(sameViewName("weekly", "weekly2")).toBe(false);
  });
});
```

Run: `cd web && pnpm vitest run app/views/draft.test.ts`
Expected: FAIL（`./draft` がない）

- [ ] **Step 4: `draft.ts` を書く**

`web/app/views/draft.ts`:

```ts
import { ApiError } from "../../core/api/client";
import { errorMessage } from "../../core/messages";
import {
  DEFAULT_KANBAN,
  DEFAULT_TABLE,
  type KanbanSettings,
  type TableSettings,
  type ViewDraft,
} from "../../core/views";
import type { Layout } from "../router";
import { splitTerms } from "../tree/filter";

/** The layouts in the order of the tabs. */
export const LAYOUTS: { layout: Layout; label: string }[] = [
  { layout: "tree", label: "ツリー" },
  { layout: "kanban", label: "カンバン" },
  { layout: "table", label: "テーブル" },
];

const KANBAN_COLUMNS: Record<KanbanSettings["columns"], string> = {
  status: "ステータス",
  category: "カテゴリ",
};
const SWIMLANES: Record<KanbanSettings["swimlane"], string> = {
  none: "なし",
  assignee: "担当者",
  project: "プロジェクト",
};
const GROUPS: Record<TableSettings["groupBy"], string> = {
  none: "なし",
  parent: "親",
  status: "ステータス",
  assignee: "担当者",
  project: "プロジェクト",
};

/** The settings of the layout in a line, for the dialog that saves a view (細部 51). */
export function summaryOf(d: ViewDraft): string {
  if (d.layout === "kanban") {
    return `カンバン · 列: ${KANBAN_COLUMNS[d.kanban.columns]} · スイムレーン: ${SWIMLANES[d.kanban.swimlane]}`;
  }
  if (d.layout === "table") {
    return `テーブル · 列 ${d.table.columns.length} · グループ: ${GROUPS[d.table.groupBy]}`;
  }
  return "ツリー";
}

/**
 * Whether two names are one view for the server: names that differ only in case or in Unicode
 * normalization are the same (internal/view SameName), since a file system may not tell them apart.
 */
export function sameViewName(a: string, b: string): boolean {
  return a.normalize("NFC").toLowerCase() === b.normalize("NFC").toLowerCase();
}

const TREE_ORDER = new ApiError({ status: 422, type: "view.tree_order" });

/**
 * Why a tree view cannot keep this query (細部 54): the tree follows rank, so the server refuses a
 * sort: or a group: in it (view.tree_order). Null when nothing is wrong.
 */
export function treeOrderProblem(d: Pick<ViewDraft, "layout" | "query">): string | null {
  if (d.layout !== "tree") return null;
  return splitTerms(d.query).some((t) => /^(sort|group):/.test(t))
    ? errorMessage(TREE_ORDER)
    : null;
}

/** A draft of a new view: the settings that are not given are the defaults of the layout. */
export function newDraft(i: {
  layout: Layout;
  query: string;
  kanban?: KanbanSettings;
  table?: TableSettings;
}): ViewDraft {
  return {
    layout: i.layout,
    query: i.query,
    kanban: i.kanban ?? DEFAULT_KANBAN,
    table: i.table ?? DEFAULT_TABLE,
  };
}
```

Run: `cd web && pnpm vitest run app/views/draft.test.ts`
Expected: PASS

- [ ] **Step 5: ダイアログの状態とテストの部品を作る**

`web/app/views/dialogs-state.svelte.ts`（`workspace/dialogs-state.svelte.ts` と同じ形）:

`web/app/views/dialogs-state.svelte.ts`:

```ts
import type { ViewDraft, ViewEntry } from "../../core/views";

/** The view dialog that is asked for; ViewDialogs draws it. */
export type ViewDialog =
  | { kind: "save"; draft: ViewDraft; store?: string }
  | { kind: "rename"; entry: ViewEntry }
  | { kind: "delete"; entry: ViewEntry };

// raw: the entry is the one the directory holds, not a copy of it.
let current = $state.raw<ViewDialog | null>(null);

export const viewDialog = {
  get current(): ViewDialog | null {
    return current;
  },
};

/** store is the Store the dialog starts with; without it, the default Store. */
export function openViewSave(i: { draft: ViewDraft; store?: string }): void {
  current = { kind: "save", draft: i.draft, store: i.store };
}

export function openViewRename(entry: ViewEntry): void {
  current = { kind: "rename", entry };
}

export function openViewDelete(entry: ViewEntry): void {
  current = { kind: "delete", entry };
}

export function closeViewDialog(): void {
  current = null;
}
```

テストの部品。`testkit.ts` は、メモリ上の `ViewDirectory`（書き込むと一覧を替えて購読者に知らせる）と、`detail/testkit.ts` の `makeCore` に `views` を足して描く `mountWithViews`。`fakes/` は、画面のテストが3つの一覧の代わりに置く部品で、渡された `view`・`query`・`settings`・`cli` を見せ、ボタンで `onquery`・`onsettings` を呼ぶ。

`web/app/views/testkit.ts`:

```ts
import { type Component, flushSync } from "svelte";
import { vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import type { SavedView, ViewEntry, ViewSaveBody } from "../../core/views";
import { makeCore } from "../detail/testkit";
import { renderWithCore } from "../test-utils";

/** A saved view as GET /api/views answers it: the defaults for what the file does not say. */
export function savedView(over: Partial<SavedView> = {}): SavedView {
  return {
    name: "weekly",
    layout: "table",
    query: "workspace:release -status:canceled",
    table: { columns: ["id", "title", "status", "assignee", "due"], group_by: "none" },
    kanban: { columns: "status", swimlane: "none" },
    ...over,
  };
}

export function viewEntry(
  over: Partial<SavedView> = {},
  store = "shared",
  tone: ViewEntry["tone"] = "purple",
): ViewEntry {
  return { store, tone, view: savedView(over) };
}

/** What GET /api/stores answers for one store. */
export function storeItem(name: string, isDefault = false) {
  return {
    name,
    default: isDefault,
    path: `/data/${name}`,
    remotes: [],
    account: "github.com/koki",
  };
}

function applied(v: SavedView, body: ViewSaveBody): SavedView {
  return {
    ...v,
    layout: body.layout ?? v.layout,
    query: body.query ?? v.query,
    table: {
      columns: body.columns ?? v.table.columns,
      group_by: body.group_by ?? v.table.group_by,
    },
    kanban: {
      columns: body.kanban_columns ?? v.kanban.columns,
      swimlane: body.swimlane ?? v.kanban.swimlane,
    },
  };
}

/** A ViewDirectory in memory: a write changes the list and tells the subscribers, as the real one does. */
export function makeViews(initial: ViewEntry[] = []) {
  let items = [...initial];
  const subs = new Set<() => void>();
  const notify = () => {
    for (const fn of [...subs]) fn();
  };
  return {
    list: vi.fn(() => items),
    get: vi.fn((store: string, name: string) =>
      items.find((e) => e.store === store && e.view.name === name),
    ),
    loaded: vi.fn(() => true),
    subscribe: (fn: () => void) => {
      subs.add(fn);
      return () => {
        subs.delete(fn);
      };
    },
    save: vi.fn(async (store: string, name: string, body: ViewSaveBody) => {
      const at = items.find((e) => e.store === store && e.view.name === name);
      const view = applied(at?.view ?? savedView({ name, layout: "tree" }), body);
      const entry: ViewEntry = { store, tone: at?.tone ?? "purple", view };
      items = at ? items.map((e) => (e === at ? entry : e)) : [...items, entry];
      notify();
      return view;
    }),
    rename: vi.fn(async (store: string, name: string, to: string) => {
      const at = items.find((e) => e.store === store && e.view.name === name);
      if (!at) throw new Error("no such view");
      const entry: ViewEntry = { ...at, view: { ...at.view, name: to } };
      items = items.map((e) => (e === at ? entry : e));
      notify();
      return entry.view;
    }),
    remove: vi.fn(async (store: string, name: string) => {
      items = items.filter((e) => !(e.store === store && e.view.name === name));
      notify();
    }),
    /** Replaces the list as an event from outside would. */
    set(next: ViewEntry[]) {
      items = next;
      notify();
    },
  };
}

export type FakeViews = ReturnType<typeof makeViews>;

/** Renders a screen with the fake core of the detail screens plus the views. */
export function mountWithViews<P extends Record<string, unknown>>(
  C: Component<P>,
  props: P,
  opts: {
    views?: FakeViews;
    stores?: string[];
    readOnly?: string[];
    tasks?: TaskView[];
  } = {},
) {
  const k = makeCore({ tasks: opts.tasks });
  const readOnly = opts.readOnly ?? [];
  const items = (opts.stores ?? ["shared"]).map((n, i) => storeItem(n, i === 0));
  const stores = {
    ...k.stores,
    stores: vi.fn(() => items),
    tone: vi.fn((s: string) => (readOnly.includes(s) ? "readonly" : "purple")),
  };
  const views = opts.views ?? makeViews();
  const { api, cache, save, schema, conn } = k;
  const r = renderWithCore(C, props, { api, cache, save, stores, schema, conn, views });
  flushSync();
  return { k, views, stores, ...r };
}
```

`web/app/views/fakes/FakeTree.svelte`:

```svelte
<!-- The tests of the screens that hold a tree stand this in for TreeView: it shows what it was given. -->
<script lang="ts">
  import type { Snippet } from "svelte";
  import type { ListScope } from "../../list/scope";

  type Props = {
    view: ListScope;
    query?: string;
    onquery?: (q: string) => void;
    cli?: string;
    actions?: Snippet;
  };
  let { view, query, onquery, cli, actions }: Props = $props();
</script>

<div data-testid="tree" data-view={JSON.stringify(view)} data-cli={cli}>
  <span data-testid="tree-query">{query}</span>
  <button type="button" onclick={() => onquery?.("sort:title")}>tree-set-sort</button>
  {#if actions}
    {@render actions()}
  {/if}
</div>
```

`web/app/views/fakes/FakeKanban.svelte`:

```svelte
<!-- The tests of the screens that hold a kanban stand this in for KanbanView. -->
<script lang="ts">
  import type { Snippet } from "svelte";
  import type { KanbanSettings } from "../../../core/views";
  import type { ListScope } from "../../list/scope";

  type Props = {
    view: ListScope;
    query: string;
    onquery: (q: string) => void;
    settings: KanbanSettings;
    onsettings: (s: KanbanSettings) => void;
    cli?: string;
    actions?: Snippet;
  };
  let { view, query, onquery, settings, onsettings, cli, actions }: Props = $props();
</script>

<div data-testid="kanban" data-view={JSON.stringify(view)} data-cli={cli}>
  <span data-testid="kanban-query">{query}</span>
  <span data-testid="kanban-settings">{settings.columns}/{settings.swimlane}</span>
  <button type="button" onclick={() => onquery("assignee:@me")}>kanban-set-query</button>
  <button type="button" onclick={() => onsettings({ ...settings, swimlane: "assignee" })}>
    kanban-set-swimlane
  </button>
  {#if actions}
    {@render actions()}
  {/if}
</div>
```

`web/app/views/fakes/FakeTable.svelte`:

```svelte
<!-- The tests of the screens that hold a table stand this in for TableView. -->
<script lang="ts">
  import type { Snippet } from "svelte";
  import type { TableSettings } from "../../../core/views";
  import type { ListScope } from "../../list/scope";

  type Props = {
    view: ListScope;
    query: string;
    onquery: (q: string) => void;
    settings: TableSettings;
    onsettings: (s: TableSettings) => void;
    cli?: string;
    actions?: Snippet;
  };
  let { view, query, onquery, settings, onsettings, cli, actions }: Props = $props();
</script>

<div data-testid="table" data-view={JSON.stringify(view)} data-cli={cli}>
  <span data-testid="table-query">{query}</span>
  <span data-testid="table-settings">{settings.columns.join(",")}/{settings.groupBy}</span>
  <button type="button" onclick={() => onquery("assignee:@me")}>table-set-query</button>
  <button type="button" onclick={() => onsettings({ ...settings, groupBy: "parent" })}>
    table-set-group
  </button>
  {#if actions}
    {@render actions()}
  {/if}
</div>
```


- [ ] **Step 6: 保存のダイアログのテストを書く（失敗する）**

`web/app/views/ViewSave.test.ts`:

```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { cliViewSave } from "../../core/cli";
import { DEFAULT_KANBAN, saveBodyOf, type ViewDraft } from "../../core/views";
import { resetToasts, toasts } from "../toast.svelte";
import { makeViews, mountWithViews, viewEntry } from "./testkit";
import ViewSave from "./ViewSave.svelte";

const table: ViewDraft = {
  layout: "table",
  query: "workspace:release -status:canceled",
  kanban: DEFAULT_KANBAN,
  table: { columns: ["id", "title", "status"], groupBy: "parent" },
};

function setup(
  over: {
    draft?: ViewDraft;
    store?: string;
    stores?: string[];
    readOnly?: string[];
    views?: ReturnType<typeof makeViews>;
  } = {},
) {
  const onclose = vi.fn();
  const r = mountWithViews(
    ViewSave,
    { draft: over.draft ?? table, store: over.store, onclose },
    { stores: over.stores, readOnly: over.readOnly, views: over.views },
  );
  return { onclose, ...r };
}

const typeName = (value: string) =>
  fireEvent.input(screen.getByRole("textbox", { name: "名前" }), { target: { value } });

describe("ViewSave", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/query/table");
  });

  it("名前を入れて保存すると、今の設定をまるごと PUT して、保存したビューを開く", async () => {
    const { onclose, views } = setup();
    expect(screen.getByRole("dialog", { name: "ビューを保存" })).toBeVisible();
    await typeName("weekly");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(views.save).toHaveBeenCalledWith("shared", "weekly", saveBodyOf(table));
    expect(location.pathname).toBe("/views/shared/weekly");
    expect(toasts.list[0]?.message).toBe("ビュー weekly を shared に保存しました");
  });

  it("足元に hh view save を出す。名前が空のあいだは <名前> でコピーできない", async () => {
    setup();
    const cli = () => screen.getByRole("button", { name: /コマンドをコピー/ });
    expect(cli()).toBeDisabled();
    expect(cli()).toHaveTextContent("<名前>");
    await typeName("weekly");
    expect(cli()).toBeEnabled();
    expect(cli()).toHaveTextContent(
      cliViewSave({ name: "weekly", store: "shared", body: saveBodyOf(table) }),
    );
  });

  it("設定の要約とクエリを出す", () => {
    setup();
    expect(screen.getByText("テーブル · 列 3 · グループ: 親")).toBeInTheDocument();
    expect(screen.getByText("workspace:release -status:canceled")).toBeInTheDocument();
  });

  it("名前が空なら保存できない。最初は叱らない", () => {
    const { views } = setup();
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(views.save).not.toHaveBeenCalled();
  });

  it("Store が1つなら保存先の欄を出さず、2つ以上なら選ばせる（既定は default の Store）", async () => {
    const one = setup();
    expect(screen.queryByRole("radiogroup", { name: "保存先" })).toBeNull();
    one.unmount();

    setup({ stores: ["shared", "me"] });
    const group = screen.getByRole("radiogroup", { name: "保存先" });
    expect(group).toBeVisible();
    expect(screen.getByRole("radio", { name: /shared/ })).toBeChecked();
    await fireEvent.click(screen.getByRole("radio", { name: /me/ }));
    await typeName("mine");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(location.pathname).toBe("/views/me/mine"));
  });

  it("画面の Store が渡されていれば、それが最初の選択", () => {
    setup({ stores: ["shared", "me"], store: "me" });
    expect(screen.getByRole("radio", { name: /me/ })).toBeChecked();
  });

  it("閲覧のみの Store は選べない", () => {
    setup({ stores: ["shared", "ro"], readOnly: ["ro"] });
    expect(screen.getByRole("radio", { name: /ro/ })).toBeDisabled();
    expect(screen.getByRole("radio", { name: /shared/ })).toBeEnabled();
  });

  it("同じ Store に同じ名前（大文字小文字の違いも）があれば保存できず、理由を出す", async () => {
    const views = makeViews([viewEntry({ name: "Weekly" })]);
    const { onclose } = setup({ views });
    await typeName("weekly");
    expect(screen.getByText(/Weekly というビューが shared にあります/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toBeDisabled();
    expect(onclose).not.toHaveBeenCalled();
  });

  it("別の Store に同じ名前があってもよい（細部 53）", async () => {
    const views = makeViews([viewEntry({ name: "weekly" }, "other")]);
    setup({ views });
    await typeName("weekly");
    expect(screen.getByRole("button", { name: "保存" })).toBeEnabled();
  });

  it("ツリーで sort: か group: があれば、保存の前に理由を出して保存を無効にする（細部 54）", async () => {
    setup({ draft: { ...table, layout: "tree", query: "kind:task sort:updated" } });
    await typeName("weekly");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "ツリーのビューは rank の順に並ぶため、group: と sort: は使えません",
    );
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
  });

  it("カンバンやテーブルなら sort: があっても保存できる", async () => {
    setup({ draft: { ...table, query: "sort:updated" } });
    await typeName("weekly");
    expect(screen.getByRole("button", { name: "保存" })).toBeEnabled();
  });

  it("サーバーが断ったら、その言葉を出してダイアログを開いたままにする", async () => {
    const views = makeViews();
    const { ApiError } = await import("../../core/api/client");
    views.save.mockRejectedValueOnce(
      new ApiError({ status: 422, type: "view.bad_name", args: { value: "a/b" } }),
    );
    const { onclose } = setup({ views });
    await typeName("a/b");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("「a/b」はビューの名前に使えません");
    expect(onclose).not.toHaveBeenCalled();
    expect(location.pathname).toBe("/query/table");
    expect(toasts.list).toHaveLength(0);
  });

  it("名前の欄で Enter を押すと保存する。変換中の Enter は保存しない", async () => {
    const { onclose, views } = setup();
    await typeName("weekly");
    const box = screen.getByRole("textbox", { name: "名前" });
    await fireEvent.keyDown(box, { key: "Enter", isComposing: true });
    expect(views.save).not.toHaveBeenCalled();
    await fireEvent.keyDown(box, { key: "Enter" });
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(views.save).toHaveBeenCalledTimes(1);
  });

  it("書き込みのあいだは閉じない", async () => {
    const views = makeViews();
    let release: (() => void) | undefined;
    views.save.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          release = () => resolve(viewEntry({ name: "weekly" }).view);
        }),
    );
    const { onclose } = setup({ views });
    await typeName("weekly");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
    expect(onclose).not.toHaveBeenCalled();
    release?.();
    await waitFor(() => expect(onclose).toHaveBeenCalled());
  });
});
```

Run: `cd web && pnpm vitest run app/views/ViewSave.test.ts`
Expected: FAIL（`./ViewSave.svelte` がない）

- [ ] **Step 7: `ViewSave.svelte` を書く**

`web/app/views/ViewSave.svelte`:

```svelte
<script lang="ts">
  import { cliViewSave, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { saveBodyOf, type ViewDraft } from "../../core/views";
  import { core } from "../context";
  import { router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { sameViewName, summaryOf, treeOrderProblem } from "./draft";

  // store is the Store the dialog starts with (the one the screen is about); without it, the default.
  let {
    draft,
    store: asked,
    onclose,
  }: { draft: ViewDraft; store?: string; onclose: () => void } = $props();

  const { stores, views } = core();
  const uid = $props.id();
  const dir = useSubscribe(stores, () => stores.stores());
  const all = useSubscribe(views, () => views.list());

  // The Store picked here, else the one asked for, else the default one, else the first.
  let picked = $state("");
  const store = $derived(
    dir.current.find((s) => s.name === picked)?.name ??
      dir.current.find((s) => s.name === asked)?.name ??
      dir.current.find((s) => s.default)?.name ??
      dir.current[0]?.name ??
      "",
  );

  let name = $state("");
  let touched = $state(false);
  let busy = $state(false);
  let failure = $state<string | null>(null);

  // PUT would replace a view of that name, so a name in use is not saved here (the view's own
  // screen updates it).
  const taken = $derived(
    all.current.find((e) => e.store === store && sameViewName(e.view.name, name)),
  );
  const orderProblem = $derived(treeOrderProblem(draft));
  const nameProblem = $derived(
    taken
      ? `${taken.view.name} というビューが ${store} にあります。置き換えるには、そのビューの画面の「ビューを更新」を使います`
      : null,
  );
  const invalid = $derived(
    store === "" || name === "" || nameProblem !== null || orderProblem !== null,
  );
  // An empty field is not scolded before the first try.
  const nameShown = $derived(name === "" && !touched ? null : nameProblem);

  const cli = $derived(
    cliViewSave({
      name: name === "" ? new Placeholder("名前") : name,
      store,
      body: saveBodyOf(draft),
    }),
  );

  // While the write runs the dialog stays: closing it then would hide its error.
  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    touched = true;
    if (busy || invalid) return;
    busy = true;
    failure = null;
    try {
      const saved = await views.save(store, name, saveBodyOf(draft));
      toast({ message: `ビュー ${saved.name} を ${store} に保存しました` });
      router.go(router.href({ name: "view", store, view: saved.name }));
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    const t = e.target as HTMLInputElement;
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229 && t.type === "text") {
      e.preventDefault();
      void submit();
    }
  }
</script>

<Dialog open title="ビューを保存" width={560} {cli} cliDisabled={invalid} onclose={close}>
  <div class="form" role="presentation" {onkeydown}>
    <div class="field">
      <label class="lbl" for="{uid}-name">名前</label>
      <input
        id="{uid}-name"
        type="text"
        autocomplete="off"
        data-autofocus
        value={name}
        oninput={(e) => (name = e.currentTarget.value)}
        aria-invalid={nameShown ? "true" : undefined}
        aria-describedby="{uid}-name-msg"
      >
      <span id="{uid}-name-msg" class="msg" class:bad={nameShown}>
        {nameShown ?? '1〜64 文字。/ \\ : * ? " < > | は使えません'}
      </span>
    </div>

    {#if dir.current.length >= 2}
      <div class="field" role="radiogroup" aria-label="保存先">
        <span class="lbl">保存先</span>
        <div class="stores">
          {#each dir.current as s (s.name)}
            {@const ro = stores.tone(s.name) === "readonly"}
            <!-- biome-ignore lint/a11y/noLabelWithoutControl: the badge is the text of the label -->
            <label class="choice" class:on={s.name === store} class:off={ro}>
              <input
                type="radio"
                name="{uid}-store"
                value={s.name}
                checked={s.name === store}
                disabled={ro}
                onchange={() => (picked = s.name)}
              >
              <StoreBadge store={s.name} tone={stores.tone(s.name)} />
              {#if ro}
                <span class="note">閲覧のみ</span>
              {/if}
            </label>
          {/each}
        </div>
        <span class="msg">{store} の views/{name === "" ? "<名前>" : name}.yaml に保存します</span>
      </div>
    {:else}
      <p class="msg">{store} の views/{name === "" ? "<名前>" : name}.yaml に保存します</p>
    {/if}

    <dl class="summary">
      <dt>設定</dt>
      <dd>{summaryOf(draft)}</dd>
      <dt>クエリ</dt>
      <dd class="mono">{draft.query === "" ? "（すべてのタスク）" : draft.query}</dd>
    </dl>

    {#if orderProblem}
      <p class="problem" role="alert">{orderProblem}</p>
    {/if}
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy || invalid} onclick={() => void submit()}>
      {busy ? "保存中…" : "保存"}
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
  .lbl {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  input[type="text"] {
    min-height: 40px;
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font-size: var(--fs-14);
  }
  input[type="text"]:focus-visible {
    outline: 2px solid var(--accent);
  }
  input[aria-invalid="true"] {
    border: 1.5px solid var(--danger);
  }
  .stores {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
  }
  .choice {
    display: inline-flex;
    gap: var(--space-8);
    align-items: center;
    padding: var(--space-6) var(--space-10);
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
    cursor: pointer;
  }
  .choice.on {
    border-color: var(--accent);
    background: var(--accent-weak);
  }
  .choice.off {
    cursor: not-allowed;
  }
  .note {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.55;
  }
  .bad {
    color: var(--danger);
  }
  .summary {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: var(--space-6) var(--space-12);
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--sunken);
    font-size: var(--fs-13);
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .problem {
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--warning-weak);
    font-size: var(--fs-13);
    line-height: 1.6;
  }
</style>
```

Run: `cd web && pnpm vitest run app/views/ViewSave.test.ts`
Expected: PASS

- [ ] **Step 8: 改名・削除のダイアログと、まとめ役のテストを書く（失敗する）**

`web/app/views/ViewRename.test.ts`:

```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../core/api/client";
import { cliViewRename } from "../../core/cli";
import { resetToasts, toasts } from "../toast.svelte";
import { makeViews, mountWithViews, viewEntry } from "./testkit";
import ViewRename from "./ViewRename.svelte";

const weekly = viewEntry({ name: "weekly" });

function setup(views = makeViews([weekly, viewEntry({ name: "Sprint" })])) {
  const onclose = vi.fn();
  const r = mountWithViews(ViewRename, { entry: weekly, onclose }, { views });
  return { onclose, ...r };
}

const typeName = (value: string) =>
  fireEvent.input(screen.getByRole("textbox", { name: "新しい名前" }), { target: { value } });

describe("ViewRename", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("今の名前が入っていて、変えるまで改名できない", () => {
    setup();
    expect(screen.getByRole("dialog", { name: "ビュー weekly を改名" })).toBeVisible();
    expect(screen.getByRole("textbox", { name: "新しい名前" })).toHaveValue("weekly");
    expect(screen.getByRole("button", { name: "改名" })).toBeDisabled();
  });

  it("新しい名前で PATCH して、トーストを出して閉じる。足元に hh view rename", async () => {
    const { onclose, views } = setup();
    await typeName("monthly");
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      cliViewRename("weekly", "monthly", "shared"),
    );
    await fireEvent.click(screen.getByRole("button", { name: "改名" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(views.rename).toHaveBeenCalledWith("shared", "weekly", "monthly");
    expect(toasts.list[0]?.message).toBe("ビュー weekly を monthly に改名しました");
  });

  it("開いているのがそのビューの画面なら、新しい名前の画面に移る（履歴は積まない）", async () => {
    history.replaceState(null, "", "/views/shared/weekly");
    const before = history.length;
    const { onclose } = setup();
    await typeName("monthly");
    await fireEvent.click(screen.getByRole("button", { name: "改名" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/views/shared/monthly");
    expect(history.length).toBe(before);
  });

  it("ほかの画面にいれば動かさない", async () => {
    history.replaceState(null, "", "/views/shared/Sprint");
    const { onclose } = setup();
    await typeName("monthly");
    await fireEvent.click(screen.getByRole("button", { name: "改名" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/views/shared/Sprint");
  });

  it("同じ Store の別のビューと重なる名前（大文字小文字の違いも）は断る", async () => {
    setup();
    await typeName("sprint");
    expect(screen.getByText("Sprint というビューが shared にあります")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "改名" })).toBeDisabled();
  });

  it("名前が空なら改名できない", async () => {
    setup();
    await typeName("");
    expect(screen.getByRole("button", { name: "改名" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toBeDisabled();
  });

  it("サーバーが断ったら、その言葉を出して開いたままにする", async () => {
    const views = makeViews([weekly]);
    views.rename.mockRejectedValueOnce(
      new ApiError({ status: 422, type: "view.bad_name", args: { value: "a/b" } }),
    );
    const { onclose } = setup(views);
    await typeName("a/b");
    await fireEvent.click(screen.getByRole("button", { name: "改名" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("「a/b」はビューの名前に使えません");
    expect(onclose).not.toHaveBeenCalled();
    expect(toasts.list).toHaveLength(0);
  });
});
```

`web/app/views/ViewDelete.test.ts`:

```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../core/api/client";
import { cliViewRm } from "../../core/cli";
import { resetToasts, toasts } from "../toast.svelte";
import { runUndo } from "../undo";
import { makeViews, mountWithViews, viewEntry } from "./testkit";
import ViewDelete from "./ViewDelete.svelte";

vi.mock("../undo", () => ({ runUndo: vi.fn(async () => {}) }));

const weekly = viewEntry({ name: "weekly" });

function setup(views = makeViews([weekly])) {
  const onclose = vi.fn();
  const r = mountWithViews(ViewDelete, { entry: weekly, onclose }, { views });
  return { onclose, ...r };
}

describe("ViewDelete", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("確認して消す。足元に hh view rm", async () => {
    const { onclose, views } = setup();
    expect(screen.getByRole("dialog", { name: "ビュー weekly を削除" })).toBeVisible();
    expect(screen.getByText(/shared の views\/weekly.yaml を消します/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      cliViewRm("weekly", "shared"),
    );
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(views.remove).toHaveBeenCalledWith("shared", "weekly");
    expect(toasts.list[0]?.message).toBe("ビュー weekly を削除しました");
  });

  it("トーストの取り消しは、そのビューの Store の直前の操作を戻す", async () => {
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    await toasts.list[0]?.undo?.();
    expect(runUndo).toHaveBeenCalledWith(expect.anything(), "shared");
  });

  it("開いているのがそのビューの画面なら、ホームへ戻る", async () => {
    history.replaceState(null, "", "/views/shared/weekly");
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/");
  });

  it("ほかの画面にいれば動かさない", async () => {
    history.replaceState(null, "", "/query/table");
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/query/table");
  });

  it("サーバーが断ったら、その言葉を出して開いたままにする", async () => {
    const views = makeViews([weekly]);
    views.remove.mockRejectedValueOnce(
      new ApiError({ status: 404, type: "view.not_found", args: { name: "weekly" } }),
    );
    const { onclose } = setup(views);
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("weekly というビューはありません");
    expect(onclose).not.toHaveBeenCalled();
    expect(toasts.list).toHaveLength(0);
  });
});
```

`web/app/views/ViewDialogs.test.ts`:

```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, describe, expect, it } from "vitest";
import { DEFAULT_KANBAN, DEFAULT_TABLE, type ViewDraft } from "../../core/views";
import {
  closeViewDialog,
  openViewDelete,
  openViewRename,
  openViewSave,
  viewDialog,
} from "./dialogs-state.svelte";
import { mountWithViews, viewEntry } from "./testkit";
import ViewDialogs from "./ViewDialogs.svelte";

const draft: ViewDraft = {
  layout: "kanban",
  query: "assignee:@me",
  kanban: DEFAULT_KANBAN,
  table: DEFAULT_TABLE,
};

afterEach(() => closeViewDialog());

describe("ViewDialogs", () => {
  it("何も頼まれていなければ何も出さない", () => {
    mountWithViews(ViewDialogs, {});
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("保存・改名・削除を頼まれた順に出し、閉じるとなくなる", async () => {
    mountWithViews(ViewDialogs, {});
    const entry = viewEntry({ name: "weekly" });

    openViewSave({ draft, store: "shared" });
    expect(await screen.findByRole("dialog", { name: "ビューを保存" })).toBeVisible();
    closeViewDialog();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    openViewRename(entry);
    expect(await screen.findByRole("dialog", { name: "ビュー weekly を改名" })).toBeVisible();

    openViewDelete(entry);
    expect(await screen.findByRole("dialog", { name: "ビュー weekly を削除" })).toBeVisible();
    await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(viewDialog.current).toBeNull();
  });

  it("保存のダイアログは渡された Store で始まる", async () => {
    mountWithViews(ViewDialogs, {}, { stores: ["shared", "me"] });
    openViewSave({ draft, store: "me" });
    expect(await screen.findByRole("radio", { name: /me/ })).toBeChecked();
  });
});
```

Run: `cd web && pnpm vitest run app/views/ViewRename.test.ts app/views/ViewDelete.test.ts app/views/ViewDialogs.test.ts`
Expected: FAIL（3つの `.svelte` がない）

- [ ] **Step 9: `ViewRename.svelte`・`ViewDelete.svelte`・`ViewDialogs.svelte` を書く**

`web/app/views/ViewRename.svelte`:

```svelte
<script lang="ts">
  import { cliViewRename, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { ViewEntry } from "../../core/views";
  import { core } from "../context";
  import { router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import { sameViewName } from "./draft";

  let { entry, onclose }: { entry: ViewEntry; onclose: () => void } = $props();

  const { views } = core();
  const uid = $props.id();
  const all = useSubscribe(views, () => views.list());
  const from = $derived(entry.view.name);

  // null until the user types: the field starts with the name it has.
  let typed = $state<string | null>(null);
  const name = $derived(typed ?? from);
  let touched = $state(false);
  let busy = $state(false);
  let failure = $state<string | null>(null);

  // Another view of the Store with that name (the same name with a different case is this view's
  // own to change, which the server decides).
  const taken = $derived(
    all.current.find(
      (e) => e.store === entry.store && e.view.name !== from && sameViewName(e.view.name, name),
    ),
  );
  const problem = $derived(
    taken ? `${taken.view.name} というビューが ${entry.store} にあります` : null,
  );
  const unchanged = $derived(name === from);
  const invalid = $derived(name === "" || unchanged || problem !== null);
  const shown = $derived(name === "" && !touched ? null : problem);

  const cli = $derived(
    cliViewRename(from, name === "" ? new Placeholder("新しい名前") : name, entry.store),
  );

  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    touched = true;
    if (busy || invalid) return;
    busy = true;
    failure = null;
    try {
      const renamed = await views.rename(entry.store, from, name);
      // The screen of the old name has nothing to show now: it goes to the new one.
      const r = router.route;
      if (r.name === "view" && r.store === entry.store && r.view === from) {
        router.go(router.href({ name: "view", store: entry.store, view: renamed.name }), {
          replace: true,
        });
      }
      toast({ message: `ビュー ${from} を ${renamed.name} に改名しました` });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    const t = e.target as HTMLInputElement;
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229 && t.type === "text") {
      e.preventDefault();
      void submit();
    }
  }
</script>

<Dialog open title="ビュー {from} を改名" width={560} {cli} cliDisabled={invalid} onclose={close}>
  <div class="form" role="presentation" {onkeydown}>
    <div class="field">
      <label class="lbl" for="{uid}-name">新しい名前</label>
      <input
        id="{uid}-name"
        type="text"
        autocomplete="off"
        data-autofocus
        value={name}
        oninput={(e) => (typed = e.currentTarget.value)}
        aria-invalid={shown ? "true" : undefined}
        aria-describedby="{uid}-msg"
      >
      <span id="{uid}-msg" class="msg" class:bad={shown}>
        {shown ?? `${entry.store} の views/${from}.yaml を、新しい名前のファイルに移します`}
      </span>
    </div>
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy || invalid} onclick={() => void submit()}>
      {busy ? "改名中…" : "改名"}
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
  .lbl {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  input[type="text"] {
    min-height: 40px;
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font-size: var(--fs-14);
  }
  input[type="text"]:focus-visible {
    outline: 2px solid var(--accent);
  }
  input[aria-invalid="true"] {
    border: 1.5px solid var(--danger);
  }
  .msg {
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.55;
  }
  .bad {
    color: var(--danger);
  }
</style>
```

`web/app/views/ViewDelete.svelte`:

```svelte
<script lang="ts">
  import { cliViewRm } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { ViewEntry } from "../../core/views";
  import { core } from "../context";
  import { router } from "../router";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import { runUndo } from "../undo";

  let { entry, onclose }: { entry: ViewEntry; onclose: () => void } = $props();

  const c = core();
  const { views } = c;

  let busy = $state(false);
  let failure = $state<string | null>(null);

  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    if (busy) return;
    busy = true;
    failure = null;
    const gone = { store: entry.store, name: entry.view.name };
    try {
      await views.remove(gone.store, gone.name);
      // The screen of the view that is gone has nothing to show.
      const r = router.route;
      if (r.name === "view" && r.store === gone.store && r.view === gone.name) {
        router.go(router.href({ name: "home" }));
      }
      toast({
        message: `ビュー ${gone.name} を削除しました`,
        undo: () => runUndo(c, gone.store),
      });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog
  open
  title="ビュー {entry.view.name} を削除"
  width={560}
  cli={cliViewRm(entry.view.name, entry.store)}
  onclose={close}
>
  <div class="body">
    <p class="note">
      {entry.store}
      の views/{entry.view.name}.yaml
      を消します。ビューが指していたタスクは変わりません。コマンドパレットの「直前の操作を取り消す」（hh
      undo）で元に戻せます。
    </p>
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="danger" disabled={busy} onclick={() => void submit()}>
      {busy ? "削除中…" : "削除"}
    </Button>
  {/snippet}
</Dialog>

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .note {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-13);
    line-height: 1.7;
  }
</style>
```

`web/app/views/ViewDialogs.svelte`:

```svelte
<script lang="ts">
  import { closeViewDialog, viewDialog } from "./dialogs-state.svelte";
  import ViewDelete from "./ViewDelete.svelte";
  import ViewRename from "./ViewRename.svelte";
  import ViewSave from "./ViewSave.svelte";

  const d = $derived(viewDialog.current);
</script>

{#if d?.kind === "save"}
  {#key d}
    <ViewSave draft={d.draft} store={d.store} onclose={closeViewDialog} />
  {/key}
{:else if d?.kind === "rename"}
  {#key d}
    <ViewRename entry={d.entry} onclose={closeViewDialog} />
  {/key}
{:else if d?.kind === "delete"}
  {#key d}
    <ViewDelete entry={d.entry} onclose={closeViewDialog} />
  {/key}
{/if}
```

Run: `cd web && pnpm vitest run app/views`
Expected: PASS

- [ ] **Step 10: 「ビューを保存」のボタンと `ListTab` のテストを書く（失敗する）**

`web/app/list/SaveViewButton.test.ts`:

```ts
import { fireEvent, screen } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DEFAULT_KANBAN, DEFAULT_TABLE } from "../../core/views";
import { closeViewDialog, viewDialog } from "../views/dialogs-state.svelte";
import { mountWithViews } from "../views/testkit";
import SaveViewButton from "./SaveViewButton.svelte";
import type { ListScope } from "./scope";

// What useListScope answers is its own test's business: here it is a value the tests set.
const scope = vi.hoisted(() => ({
  ready: true,
  store: "shared" as string | undefined,
  saveBase: "workspace:release",
}));
vi.mock("./scope.svelte", () => ({ useListScope: () => scope }));

const view: ListScope = { kind: "workspace", store: "shared", ulid: "W1" };

beforeEach(() => {
  scope.ready = true;
  scope.store = "shared";
  scope.saveBase = "workspace:release";
});
afterEach(() => closeViewDialog());

describe("SaveViewButton", () => {
  it("固定の部分とフィルタを保存するクエリにして、その画面の Store で始める（D8）", async () => {
    mountWithViews(SaveViewButton, { view, layout: "kanban", filter: "-status:canceled" });
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toEqual({
      kind: "save",
      draft: {
        layout: "kanban",
        query: "workspace:release -status:canceled",
        kanban: DEFAULT_KANBAN,
        table: DEFAULT_TABLE,
      },
      store: "shared",
    });
  });

  it("固定の部分はそのまま使う（Store が2つ以上のときの store: は固定の部分に入っている）", async () => {
    scope.saveBase = "store:shared workspace:release";
    mountWithViews(SaveViewButton, { view, layout: "table", filter: "" });
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      draft: { layout: "table", query: "store:shared workspace:release" },
      store: "shared",
    });
  });

  it("プロジェクトの画面の固定の部分は project:<ID>", async () => {
    scope.saveBase = "project:REL-15";
    mountWithViews(SaveViewButton, { view, layout: "tree", filter: "kind:task" });
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      draft: { layout: "tree", query: "project:REL-15 kind:task" },
    });
  });

  it("全 Store の画面（固定の部分も Store もない）はフィルタだけで、Store は決めない", async () => {
    scope.saveBase = "";
    scope.store = undefined;
    mountWithViews(
      SaveViewButton,
      { view: { kind: "all" }, layout: "table", filter: "assignee:@me" },
      { stores: ["shared", "me"] },
    );
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toEqual({
      kind: "save",
      draft: expect.objectContaining({ query: "assignee:@me" }),
      store: undefined,
    });
  });

  it("渡された設定をそのまま保存する", async () => {
    mountWithViews(SaveViewButton, {
      view,
      layout: "table",
      filter: "",
      table: { columns: ["id", "title"], groupBy: "status" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      draft: { table: { columns: ["id", "title"], groupBy: "status" } },
    });
  });

  it("画面の対象がまだ読めないあいだは押せない", () => {
    scope.ready = false;
    mountWithViews(SaveViewButton, { view, layout: "table", filter: "" });
    expect(screen.getByRole("button", { name: "ビューを保存" })).toBeDisabled();
  });
});
```

`web/app/list/ListTab.test.ts`:

```ts
import { fireEvent, screen } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { closeViewDialog, viewDialog } from "../views/dialogs-state.svelte";
import { mountWithViews } from "../views/testkit";
import ListTab from "./ListTab.svelte";
import type { ListScope } from "./scope";

// The scope of the screen and the two lists are stood in for: this test is about what ListTab holds.
vi.mock("./scope.svelte", () => ({
  useListScope: () => ({ ready: true, store: "shared", saveBase: "workspace:release" }),
}));
vi.mock("../kanban/KanbanView.svelte", async () => ({
  default: (await import("../views/fakes/FakeKanban.svelte")).default,
}));
vi.mock("../table/TableView.svelte", async () => ({
  default: (await import("../views/fakes/FakeTable.svelte")).default,
}));

const workspace: ListScope = { kind: "workspace", store: "shared", ulid: "W1" };

afterEach(() => closeViewDialog());

describe("ListTab", () => {
  it("カンバンのタブ: フィルタの既定は -status:canceled、設定は既定", () => {
    mountWithViews(ListTab, { view: workspace, layout: "kanban" });
    expect(JSON.parse(screen.getByTestId("kanban").dataset.view ?? "")).toEqual(workspace);
    expect(screen.getByTestId("kanban-query")).toHaveTextContent("-status:canceled");
    expect(screen.getByTestId("kanban-settings")).toHaveTextContent("status/none");
  });

  it("テーブルのタブ: 設定は id,title,status,assignee,due・グループなし", () => {
    mountWithViews(ListTab, { view: workspace, layout: "table" });
    expect(screen.getByTestId("table-query")).toHaveTextContent("-status:canceled");
    expect(screen.getByTestId("table-settings")).toHaveTextContent(
      "id,title,status,assignee,due/none",
    );
  });

  it("フィルタと設定は画面の状態で、保存のダイアログに渡る（D7・D8）", async () => {
    mountWithViews(ListTab, { view: workspace, layout: "kanban" });
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-query" }));
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-swimlane" }));
    expect(screen.getByTestId("kanban-query")).toHaveTextContent("assignee:@me");
    expect(screen.getByTestId("kanban-settings")).toHaveTextContent("status/assignee");
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      kind: "save",
      draft: {
        layout: "kanban",
        query: "workspace:release assignee:@me",
        kanban: { columns: "status", swimlane: "assignee" },
      },
      store: "shared",
    });
  });

  it("カンバンとテーブルのタブを替えても、フィルタは残る", async () => {
    const { rerender } = mountWithViews(ListTab, { view: workspace, layout: "kanban" });
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-query" }));
    await rerender({ view: workspace, layout: "table" });
    expect(screen.queryByTestId("kanban")).toBeNull();
    expect(screen.getByTestId("table-query")).toHaveTextContent("assignee:@me");
    await fireEvent.click(screen.getByRole("button", { name: "table-set-group" }));
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      draft: {
        layout: "table",
        query: "workspace:release assignee:@me",
        table: { groupBy: "parent" },
      },
    });
  });
});
```

Run: `cd web && pnpm vitest run app/list/SaveViewButton.test.ts app/list/ListTab.test.ts`
Expected: FAIL（2つの `.svelte` がない）

- [ ] **Step 11: `SaveViewButton.svelte` と `ListTab.svelte` を書く**

`web/app/list/SaveViewButton.svelte`:

```svelte
<script lang="ts">
  import {
    DEFAULT_KANBAN,
    DEFAULT_TABLE,
    type KanbanSettings,
    type TableSettings,
  } from "../../core/views";
  import type { Layout } from "../router";
  import { combine } from "../tree/filter";
  import Button from "../ui/Button.svelte";
  import { openViewSave } from "../views/dialogs-state.svelte";
  import { newDraft } from "../views/draft";
  import type { ListScope } from "./scope";
  import { useListScope } from "./scope.svelte";

  // The "ビューを保存" button of a screen of a workspace or a project (D8). filter is what the
  // filter field holds; the fixed part of the screen is added here.
  type Props = {
    view: ListScope;
    layout: Layout;
    filter: string;
    kanban?: KanbanSettings;
    table?: TableSettings;
  };
  let { view, layout, filter, kanban = DEFAULT_KANBAN, table = DEFAULT_TABLE }: Props = $props();

  const scope = useListScope(() => view);

  // saveBase is the fixed part of the screen; with two Stores or more it already names the Store (D8).
  // It is empty on the screens of every Store, which combine would leave a space in front of.
  function save() {
    const query = combine(scope.saveBase, filter).trim();
    openViewSave({ draft: newDraft({ layout, query, kanban, table }), store: scope.store });
  }
</script>

<Button size="sm" disabled={!scope.ready} onclick={save}>ビューを保存</Button>
```

`web/app/list/ListTab.svelte`:

```svelte
<script lang="ts">
  import {
    DEFAULT_KANBAN,
    DEFAULT_TABLE,
    type KanbanSettings,
    type TableSettings,
  } from "../../core/views";
  import KanbanView from "../kanban/KanbanView.svelte";
  import TableView from "../table/TableView.svelte";
  import { NO_CANCELED } from "../tree/filter";
  import SaveViewButton from "./SaveViewButton.svelte";
  import type { ListScope } from "./scope";

  // The body of the kanban tab and the table tab of a workspace or a project. What is on the
  // screen (the filter, the columns, the swimlane, the group) is kept here, for as long as the
  // screen is open and not saved (D7): one ListTab serves both tabs, so the filter stays when
  // the tab changes.
  type Props = { view: ListScope; layout: "kanban" | "table" };
  let { view, layout }: Props = $props();

  // The default filter leaves the canceled tasks out, as the tree does.
  let query = $state(NO_CANCELED);
  let kanban = $state.raw<KanbanSettings>(DEFAULT_KANBAN);
  let table = $state.raw<TableSettings>(DEFAULT_TABLE);
</script>

{#snippet save()}
  <SaveViewButton {view} {layout} filter={query} {kanban} {table} />
{/snippet}

{#if layout === "kanban"}
  <KanbanView
    {view}
    {query}
    onquery={(q) => (query = q)}
    settings={kanban}
    onsettings={(s) => (kanban = s)}
    actions={save}
  />
{:else}
  <TableView
    {view}
    {query}
    onquery={(q) => (query = q)}
    settings={table}
    onsettings={(s) => (table = s)}
    actions={save}
  />
{/if}
```

Run: `cd web && pnpm vitest run app/list`
Expected: PASS

- [ ] **Step 12: クエリの画面のテストを書く（失敗する）**

`web/app/views/QueryPage.test.ts`:

```ts
import { fireEvent, screen, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DEFAULT_KANBAN, DEFAULT_TABLE } from "../../core/views";
import type { Layout } from "../router";
import { closeViewDialog, viewDialog } from "./dialogs-state.svelte";
import QueryPage from "./QueryPage.svelte";
import { mountWithViews } from "./testkit";

vi.mock("../tree/TreeView.svelte", async () => ({
  default: (await import("./fakes/FakeTree.svelte")).default,
}));
vi.mock("../kanban/KanbanView.svelte", async () => ({
  default: (await import("./fakes/FakeKanban.svelte")).default,
}));
vi.mock("../table/TableView.svelte", async () => ({
  default: (await import("./fakes/FakeTable.svelte")).default,
}));

const here = () => location.pathname + location.search;

function setup(layout: Layout = "table", q = "assignee:@me") {
  history.replaceState(null, "", `/query/${layout}?q=${encodeURIComponent(q)}`);
  return mountWithViews(QueryPage, { layout, q });
}

const tab = (name: string) =>
  within(screen.getByRole("navigation", { name: "レイアウトの切り替え" })).getByRole("link", {
    name,
  });

describe("QueryPage", () => {
  beforeEach(() => history.replaceState(null, "", "/"));
  afterEach(() => closeViewDialog());

  it("全 Store で評価する画面に、渡されたクエリをそのまま出す（既定の絞り込みは足さない）", () => {
    setup("table", "assignee:@me");
    expect(screen.getByRole("heading", { level: 1, name: "クエリ" })).toBeVisible();
    expect(screen.getByText("保存していないクエリ")).toBeVisible();
    expect(JSON.parse(screen.getByTestId("table").dataset.view ?? "")).toEqual({ kind: "all" });
    expect(screen.getByTestId("table-query")).toHaveTextContent("assignee:@me");
    expect(screen.getByTestId("table-settings")).toHaveTextContent(
      "id,title,status,assignee,due/none",
    );
  });

  it("layout のタブは、クエリを持ったまま /query/<layout> へ移る（保存しない）", async () => {
    setup("table", "assignee:@me");
    expect(tab("テーブル")).toHaveAttribute("aria-current", "page");
    expect(tab("カンバン")).toHaveAttribute("href", "/query/kanban?q=assignee%3A%40me");
    await fireEvent.click(tab("ツリー"));
    expect(here()).toBe("/query/tree?q=assignee%3A%40me");
  });

  it("layout ごとに本文が替わる", () => {
    setup("kanban");
    expect(screen.getByTestId("kanban")).toBeVisible();
    expect(screen.queryByTestId("table")).toBeNull();
  });

  it("ツリーも出る", () => {
    setup("tree", "kind:task");
    expect(screen.getByTestId("tree-query")).toHaveTextContent("kind:task");
  });

  it("フィルタ欄の変更は、履歴を積まずに URL の q に反映する", async () => {
    setup("table", "kind:task");
    const before = history.length;
    await fireEvent.click(screen.getByRole("button", { name: "table-set-query" }));
    expect(here()).toBe("/query/table?q=assignee%3A%40me");
    expect(history.length).toBe(before);
  });

  it("フィルタ欄の変更は、URL が戻ってくる前でも画面の値になり、外から q が変われば従う", async () => {
    const { rerender } = setup("table", "kind:task");
    await fireEvent.click(screen.getByRole("button", { name: "table-set-query" }));
    expect(screen.getByTestId("table-query")).toHaveTextContent("assignee:@me");
    await rerender({ layout: "table", q: "assignee:@me" });
    expect(screen.getByTestId("table-query")).toHaveTextContent("assignee:@me");
    await rerender({ layout: "table", q: "kind:bug" });
    expect(screen.getByTestId("table-query")).toHaveTextContent("kind:bug");
  });

  it("「ビューを保存」は今のレイアウト・クエリ・設定で保存のダイアログを開く（Store は決めない）", async () => {
    setup("table", "assignee:@me");
    await fireEvent.click(screen.getByRole("button", { name: "table-set-group" }));
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toEqual({
      kind: "save",
      draft: {
        layout: "table",
        query: "assignee:@me",
        kanban: DEFAULT_KANBAN,
        table: { ...DEFAULT_TABLE, groupBy: "parent" },
      },
      store: undefined,
    });
  });

  it("カンバンの設定も保存する", async () => {
    setup("kanban", "kind:task");
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-swimlane" }));
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      kind: "save",
      draft: { layout: "kanban", kanban: { columns: "status", swimlane: "assignee" } },
    });
  });

  it("ツリーにも「ビューを保存」がある", async () => {
    setup("tree", "kind:task");
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      kind: "save",
      draft: { layout: "tree", query: "kind:task" },
    });
  });
});
```

Run: `cd web && pnpm vitest run app/views/QueryPage.test.ts`
Expected: FAIL（`./QueryPage.svelte` がない）

- [ ] **Step 13: `QueryPage.svelte` を書く**

`web/app/views/QueryPage.svelte`:

```svelte
<script lang="ts">
  import {
    DEFAULT_KANBAN,
    DEFAULT_TABLE,
    type KanbanSettings,
    type TableSettings,
  } from "../../core/views";
  import KanbanView from "../kanban/KanbanView.svelte";
  import type { ListScope } from "../list/scope";
  import { type Layout, router } from "../router";
  import TableView from "../table/TableView.svelte";
  import TreeView from "../tree/TreeView.svelte";
  import Button from "../ui/Button.svelte";
  import Tabs from "../ui/Tabs.svelte";
  import { openViewSave } from "./dialogs-state.svelte";
  import { LAYOUTS, newDraft } from "./draft";

  // A query that is not saved (D1): it is evaluated in every Store, the layout is a tab, and the
  // query is in the address (?q=), so the address can be copied.
  type Props = { layout: Layout; q: string };
  let { layout, q }: Props = $props();

  // The view of every Store, one object, so the lists below do not start over at each render.
  const everywhere: ListScope = { kind: "all" };

  // The columns and the swimlane are for as long as the screen is open and not saved (D7).
  let kanban = $state.raw<KanbanSettings>(DEFAULT_KANBAN);
  let table = $state.raw<TableSettings>(DEFAULT_TABLE);

  // What the filter field holds until the address says the same: the route that comes back from
  // our own replace is the same text, so the field the user is typing in is not wound back.
  let typed = $state<string | null>(null);
  const query = $derived(typed ?? q);
  $effect(() => {
    q;
    typed = null;
  });

  const at = (l: Layout, text: string) => router.href({ name: "query", layout: l, q: text });
  const items = $derived(
    LAYOUTS.map((t) => ({
      label: t.label,
      href: at(t.layout, query),
      current: t.layout === layout,
    })),
  );
  // The filter writes the address without a new entry in the history.
  function setQuery(next: string) {
    typed = next;
    router.replace(at(layout, next));
  }
  const save = () => openViewSave({ draft: newDraft({ layout, query, kanban, table }) });
</script>

{#snippet saveButton()}
  <Button size="sm" onclick={save}>ビューを保存</Button>
{/snippet}

<div class="page">
  <header class="head">
    <h1>クエリ</h1>
    <span class="eyebrow">保存していないクエリ</span>
    <span class="note">すべての Store で調べます。store: で絞れます</span>
  </header>
  <Tabs {items} label="レイアウトの切り替え" />
  <div class="body">
    {#if layout === "tree"}
      <TreeView view={everywhere} {query} onquery={setQuery} actions={saveButton} />
    {:else if layout === "kanban"}
      <KanbanView
        view={everywhere}
        {query}
        onquery={setQuery}
        settings={kanban}
        onsettings={(s) => (kanban = s)}
        actions={saveButton}
      />
    {:else}
      <TableView
        view={everywhere}
        {query}
        onquery={setQuery}
        settings={table}
        onsettings={(s) => (table = s)}
        actions={saveButton}
      />
    {/if}
  </div>
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--space-10);
    padding: var(--space-16) var(--gutter) 0;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .eyebrow {
    color: var(--label);
    font-size: var(--fs-11);
    font-weight: 600;
    letter-spacing: 0.04em;
  }
  .note {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .body {
    flex: 1 1 auto;
    min-height: 0;
  }
</style>
```

Run: `cd web && pnpm vitest run app/views/QueryPage.test.ts`
Expected: PASS

- [ ] **Step 14: ビューの画面のテストを書く（失敗する）**

`web/app/views/ViewPage.test.ts`:

```ts
import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../core/api/client";
import { cliViewSave, cliViewShow } from "../../core/cli";
import { draftOf, saveBodyOf } from "../../core/views";
import { resetToasts, toasts } from "../toast.svelte";
import { closeViewDialog, viewDialog } from "./dialogs-state.svelte";
import { makeViews, mountWithViews, viewEntry } from "./testkit";
import ViewPage from "./ViewPage.svelte";

vi.mock("../tree/TreeView.svelte", async () => ({
  default: (await import("./fakes/FakeTree.svelte")).default,
}));
vi.mock("../kanban/KanbanView.svelte", async () => ({
  default: (await import("./fakes/FakeKanban.svelte")).default,
}));
vi.mock("../table/TableView.svelte", async () => ({
  default: (await import("./fakes/FakeTable.svelte")).default,
}));

const weekly = viewEntry({ name: "weekly", layout: "table", query: "kind:task" });

function setup(entries = [weekly], store = "shared", name = "weekly") {
  const views = makeViews(entries);
  return mountWithViews(ViewPage, { store, name }, { views });
}

const tab = (name: string) =>
  within(screen.getByRole("navigation", { name: "レイアウトの切り替え" })).getByRole("link", {
    name,
  });

describe("ViewPage", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/views/shared/weekly");
  });
  afterEach(() => closeViewDialog());

  it("見出しに名前・保存先の Store・ファイルの場所を出し、保存した layout で開く", () => {
    setup();
    expect(screen.getByRole("heading", { level: 1, name: "weekly" })).toBeVisible();
    expect(screen.getByTitle("Store: shared")).toBeVisible();
    expect(screen.getByText("shared の views/weekly.yaml")).toBeVisible();
    expect(tab("テーブル")).toHaveAttribute("aria-current", "page");
    expect(tab("ツリー")).not.toHaveAttribute("aria-current");
    expect(screen.getByTestId("table")).toBeVisible();
    expect(screen.queryByTestId("kanban")).toBeNull();
  });

  it("全 Store で評価する画面を、保存したクエリと設定で出し、足元は hh view show", () => {
    setup();
    const t = screen.getByTestId("table");
    expect(JSON.parse(t.dataset.view ?? "")).toEqual({ kind: "all" });
    expect(screen.getByTestId("table-query")).toHaveTextContent("kind:task");
    expect(screen.getByTestId("table-settings")).toHaveTextContent(
      "id,title,status,assignee,due/none",
    );
    expect(t.dataset.cli).toBe(cliViewShow("weekly", "shared"));
  });

  it("layout ごとに本文が替わる", () => {
    setup([viewEntry({ name: "weekly", layout: "kanban" })]);
    expect(screen.getByTestId("kanban")).toBeVisible();
    expect(screen.getByTestId("kanban-settings")).toHaveTextContent("status/none");
  });

  it("ツリーのビューは TreeView にクエリを渡す", () => {
    setup([viewEntry({ name: "weekly", layout: "tree", query: "kind:task" })]);
    expect(screen.getByTestId("tree-query")).toHaveTextContent("kind:task");
  });

  it("タブを押すと layout だけを PUT して、その layout の本文になる（未保存の変更を送らない）", async () => {
    const { views } = setup();
    await fireEvent.click(tab("カンバン"));
    await waitFor(() => expect(screen.getByTestId("kanban")).toBeVisible());
    expect(views.save).toHaveBeenCalledWith("shared", "weekly", { layout: "kanban" });
    expect(tab("カンバン")).toHaveAttribute("aria-current", "page");
    expect(location.pathname).toBe("/views/shared/weekly");
    expect(screen.queryByText("未保存の変更")).toBeNull();
  });

  it("タブの足元に、layout を変える hh view save を出す", () => {
    setup();
    expect(
      screen.getByText("切り替えは保存したビューの layout を変えます（1 コミット）"),
    ).toBeVisible();
    expect(
      screen.getByRole("button", {
        name: `コマンドをコピー: ${cliViewSave({ name: "weekly", store: "shared", body: { layout: "table" } })}`,
      }),
    ).toBeVisible();
  });

  it("切り替えをサーバーが断ったら、その言葉をトーストに出して、layout は変わらない", async () => {
    const { views } = setup([viewEntry({ name: "weekly", layout: "kanban", query: "sort:title" })]);
    views.save.mockRejectedValueOnce(new ApiError({ status: 422, type: "view.tree_order" }));
    await fireEvent.click(tab("ツリー"));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(toasts.list[0]?.message).toBe(
      "ツリーのビューは rank の順に並ぶため、group: と sort: は使えません",
    );
    expect(tab("カンバン")).toHaveAttribute("aria-current", "page");
    expect(screen.getByTestId("kanban")).toBeVisible();
  });

  it("クエリが保存した値と違えば「未保存の変更」と「ビューを更新」を出し、1回の PUT に全部送る", async () => {
    const { views } = setup();
    expect(screen.queryByText("未保存の変更")).toBeNull();
    expect(screen.queryByRole("button", { name: "ビューを更新" })).toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "table-set-group" }));
    await fireEvent.click(screen.getByRole("button", { name: "table-set-query" }));
    expect(screen.getByText("未保存の変更")).toBeVisible();

    const next = {
      ...draftOf(weekly.view),
      table: { columns: ["id", "title", "status", "assignee", "due"], groupBy: "parent" as const },
    };
    await fireEvent.click(screen.getByRole("button", { name: "ビューを更新" }));
    await waitFor(() => expect(screen.queryByText("未保存の変更")).toBeNull());
    expect(views.save).toHaveBeenCalledTimes(1);
    expect(views.save).toHaveBeenCalledWith(
      "shared",
      "weekly",
      saveBodyOf({ ...next, query: "assignee:@me" }),
    );
    expect(toasts.list[0]?.message).toBe("ビュー weekly を更新しました");
    expect(screen.getByTestId("table-settings")).toHaveTextContent("/parent");
  });

  it("設定だけを変えても未保存になる（カンバンのスイムレーン）", async () => {
    setup([viewEntry({ name: "weekly", layout: "kanban" })]);
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-swimlane" }));
    expect(screen.getByText("未保存の変更")).toBeVisible();
    expect(screen.getByTestId("kanban-settings")).toHaveTextContent("status/assignee");
  });

  it("更新の足元に、今の設定を保存する hh view save を出す", async () => {
    setup([viewEntry({ name: "weekly", layout: "kanban", query: "" })]);
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-query" }));
    const d = { ...draftOf(weekly.view), layout: "kanban" as const, query: "assignee:@me" };
    d.kanban = { columns: "status", swimlane: "none" };
    expect(
      screen.getByRole("button", {
        name: `コマンドをコピー: ${cliViewSave({ name: "weekly", store: "shared", body: saveBodyOf(d) })}`,
      }),
    ).toBeVisible();
  });

  it("未保存の変更は、layout を切り替えても残り、更新では保存した layout で送る", async () => {
    const { views } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "table-set-query" }));
    await fireEvent.click(tab("カンバン"));
    await waitFor(() => expect(screen.getByTestId("kanban")).toBeVisible());
    expect(views.save).toHaveBeenLastCalledWith("shared", "weekly", { layout: "kanban" });
    expect(screen.getByTestId("kanban-query")).toHaveTextContent("assignee:@me");
    expect(screen.getByText("未保存の変更")).toBeVisible();
    await fireEvent.click(screen.getByRole("button", { name: "ビューを更新" }));
    await waitFor(() => expect(screen.queryByText("未保存の変更")).toBeNull());
    expect(views.save).toHaveBeenLastCalledWith(
      "shared",
      "weekly",
      saveBodyOf({ ...draftOf(weekly.view), layout: "kanban", query: "assignee:@me" }),
    );
  });

  it("ツリーのビューでクエリに sort: を入れたら、更新を無効にして理由を出す（細部 54）", async () => {
    const { views } = setup([viewEntry({ name: "weekly", layout: "tree", query: "kind:task" })]);
    await fireEvent.click(screen.getByRole("button", { name: "tree-set-sort" }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "ツリーのビューは rank の順に並ぶため、group: と sort: は使えません",
    );
    expect(screen.getByRole("button", { name: "ビューを更新" })).toBeDisabled();
    await fireEvent.click(screen.getByRole("button", { name: "ビューを更新" }));
    expect(views.save).not.toHaveBeenCalled();
  });

  it("更新をサーバーが断ったら、トーストに出して未保存のまま残す", async () => {
    const { views } = setup();
    views.save.mockRejectedValueOnce(
      new ApiError({ status: 422, type: "view.bad_column", args: { column: "x" } }),
    );
    await fireEvent.click(screen.getByRole("button", { name: "table-set-query" }));
    await fireEvent.click(screen.getByRole("button", { name: "ビューを更新" }));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(toasts.list[0]?.message).toBe(
      "列 x は、フィールドでもスキーマのプロパティでもありません",
    );
    expect(screen.getByText("未保存の変更")).toBeVisible();
  });

  it("「…」から改名と削除のダイアログを開く", async () => {
    setup();
    await fireEvent.click(screen.getByRole("button", { name: "ビューの操作（改名・削除）" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "改名…" }));
    expect(viewDialog.current).toMatchObject({ kind: "rename", entry: weekly });

    await fireEvent.click(screen.getByRole("button", { name: "ビューの操作（改名・削除）" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "削除…" }));
    expect(viewDialog.current).toMatchObject({ kind: "delete", entry: weekly });
  });

  it("同じ名前のビューが別の Store にあっても、URL の Store のものを出す（細部 53）", () => {
    setup(
      [
        viewEntry({ name: "weekly", layout: "kanban" }, "other", "teal"),
        viewEntry({ name: "weekly", layout: "table" }, "shared"),
      ],
      "other",
    );
    expect(screen.getByTestId("kanban")).toBeVisible();
    expect(screen.getByText("other の views/weekly.yaml")).toBeVisible();
  });

  it("ビューが見つからなければ（消された・改名された）、見つからないと言う（D9）", async () => {
    const { views } = setup();
    expect(screen.getByTestId("table")).toBeVisible();
    views.set([]);
    expect(await screen.findByText("ビュー「weekly」が見つかりません")).toBeVisible();
    await fireEvent.click(screen.getByRole("button", { name: "ホームへ戻る" }));
    expect(location.pathname).toBe("/");
  });

  it("読み込みが終わるまでは、見つからないと言わない", () => {
    const views = makeViews([]);
    views.loaded.mockReturnValue(false);
    mountWithViews(ViewPage, { store: "shared", name: "weekly" }, { views });
    expect(screen.getByText("読み込み中…")).toBeVisible();
    expect(screen.queryByText(/見つかりません/)).toBeNull();
  });
});
```

Run: `cd web && pnpm vitest run app/views/ViewPage.test.ts`
Expected: FAIL（`./ViewPage.svelte` がない）

- [ ] **Step 15: `ViewPage.svelte` を書く**

`web/app/views/ViewPage.svelte`:

```svelte
<script lang="ts">
  import { cliViewSave, cliViewShow } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import {
    draftOf,
    type KanbanSettings,
    sameDraft,
    saveBodyOf,
    type TableSettings,
    type ViewDraft,
  } from "../../core/views";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import KanbanView from "../kanban/KanbanView.svelte";
  import type { ListScope } from "../list/scope";
  import { type Layout, router } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import TableView from "../table/TableView.svelte";
  import { toast } from "../toast.svelte";
  import TreeView from "../tree/TreeView.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import Menu from "../ui/Menu.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";
  import { openViewDelete, openViewRename } from "./dialogs-state.svelte";
  import { LAYOUTS, treeOrderProblem } from "./draft";

  // A saved view (細部 50): it opens in the layout it was saved with, and the tabs change that
  // layout in place (a PUT of the layout only).
  type Props = { store: string; name: string };
  let { store, name }: Props = $props();

  const { views } = core();
  const found = useSubscribe(views, () => ({
    entry: views.get(store, name),
    loaded: views.loaded(),
  }));
  const entry = $derived(found.current.entry);
  // Spread, because Biome reads a prop called scope as the HTML attribute.
  const notFoundProps = $derived({ scope: "view" as const, name });

  // The view of every Store, one object, so the lists below do not start over at each render.
  const everywhere: ListScope = { kind: "all" };

  // What is saved, and what the screen holds on top of it (the filter, the columns, the swimlane,
  // the group). local is null while the screen is as saved. The layout is always the saved one:
  // a tab changes it in the file at once, and an edit that is not saved does not go with it (D6).
  const saved = $derived(entry ? draftOf(entry.view) : null);
  let local = $state.raw<ViewDraft | null>(null);
  const draft = $derived(saved ? { ...(local ?? saved), layout: saved.layout } : null);
  const dirty = $derived(local !== null && saved !== null && !sameDraft(local, saved));
  const problem = $derived(draft ? treeOrderProblem(draft) : null);

  let busy = $state(false);

  function edit(part: Partial<Pick<ViewDraft, "query" | "kanban" | "table">>) {
    if (draft) local = { ...draft, ...part };
  }

  async function write(body: Parameters<typeof views.save>[2]): Promise<boolean> {
    busy = true;
    try {
      await views.save(store, name, body);
      return true;
    } catch (e) {
      toast({ message: errorMessage(e) });
      return false;
    } finally {
      busy = false;
    }
  }

  async function switchLayout(layout: Layout) {
    if (busy || !saved || layout === saved.layout) return;
    await write({ layout });
  }

  async function update() {
    if (busy || !dirty || !draft || problem !== null) return;
    const sent = local;
    if (await write(saveBodyOf(draft))) {
      // An edit made while the write ran is not lost.
      if (local === sent) local = null;
      toast({ message: `ビュー ${name} を更新しました` });
    }
  }

  const items = $derived(
    LAYOUTS.map((t) => ({
      label: t.label,
      href: router.href({ name: "view", store, view: name }),
      current: t.layout === saved?.layout,
      onselect: () => void switchLayout(t.layout),
    })),
  );

  let more = $state<HTMLElement | null>(null);
  const moreItems = [
    { value: "rename", label: "改名…" },
    { value: "delete", label: "削除…" },
  ];
  function pickMore(value: string) {
    more = null;
    if (!entry) return;
    if (value === "rename") openViewRename(entry);
    else openViewDelete(entry);
  }

  const showCli = $derived(cliViewShow(name, store));
  // The commands of what the buttons do (Foundations: the same operation as a command).
  const updateCli = $derived(draft ? cliViewSave({ name, store, body: saveBodyOf(draft) }) : "");
  const switchCli = $derived(
    saved ? cliViewSave({ name, store, body: { layout: saved.layout } }) : "",
  );
</script>

{#if entry && draft && saved}
  <div class="page">
    <header class="head">
      <div class="title">
        <h1>{entry.view.name}</h1>
        <StoreBadge store={entry.store} tone={entry.tone} />
        <span class="eyebrow">保存済みビュー</span>
        <span class="path">{entry.store} の views/{entry.view.name}.yaml</span>
      </div>
      <div class="actions">
        {#if dirty}
          <span class="dirty" role="status">未保存の変更</span>
          <Button
            variant="primary"
            size="sm"
            disabled={busy || problem !== null}
            onclick={() => void update()}
            >ビューを更新</Button
          >
        {/if}
        <Button
          size="sm"
          aria-label="ビューの操作（改名・削除）"
          aria-haspopup="menu"
          aria-expanded={more !== null}
          onclick={(e) => (more = e.currentTarget)}
        >
          <Icon name="more" size={14} />
        </Button>
        {#if more}
          <Menu
            anchor={more}
            items={moreItems}
            label="ビューの操作"
            onselect={pickMore}
            onclose={() => (more = null)}
          />
        {/if}
      </div>
    </header>
    {#if dirty}
      <div class="update">
        {#if problem}
          <p class="problem" role="alert">{problem}</p>
        {:else}
          <span class="hint">今の設定で保存し直します（1 コミット）</span>
        {/if}
        <CliLine command={updateCli} disabled={problem !== null} />
      </div>
    {/if}
    <Tabs {items} label="レイアウトの切り替え" />
    <div class="switch">
      <span class="hint">切り替えは保存したビューの layout を変えます（1 コミット）</span>
      <CliLine command={switchCli} />
    </div>
    <div class="body">
      {#if saved.layout === "kanban"}
        <KanbanView
          view={everywhere}
          query={draft.query}
          onquery={(q) => edit({ query: q })}
          settings={draft.kanban}
          onsettings={(s: KanbanSettings) => edit({ kanban: s })}
          cli={showCli}
        />
      {:else if saved.layout === "table"}
        <TableView
          view={everywhere}
          query={draft.query}
          onquery={(q) => edit({ query: q })}
          settings={draft.table}
          onsettings={(s: TableSettings) => edit({ table: s })}
          cli={showCli}
        />
      {:else}
        <TreeView
          view={everywhere}
          query={draft.query}
          onquery={(q) => edit({ query: q })}
          cli={showCli}
        />
      {/if}
    </div>
  </div>
{:else if found.current.loaded}
  <Empty
    variant="notfound"
    {...notFoundProps}
    onHome={() => router.go(router.href({ name: "home" }))}
  />
{:else}
  <p class="note">読み込み中…</p>
{/if}

<style>
  .page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-12);
    padding: var(--space-16) var(--gutter) 0;
  }
  .title {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
    min-width: 0;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .eyebrow {
    color: var(--label);
    font-size: var(--fs-11);
    font-weight: 600;
    letter-spacing: 0.04em;
  }
  .path {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-8);
  }
  .dirty {
    color: var(--warning);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  .update,
  .switch {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
    padding: var(--space-8) var(--gutter) 0;
  }
  .hint {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .problem {
    margin: 0;
    padding: var(--space-6) var(--space-10);
    border-radius: var(--radius-8);
    background: var(--warning-weak);
    font-size: var(--fs-13);
  }
  .body {
    flex: 1 1 auto;
    min-height: 0;
  }
  .note {
    margin: var(--space-16) var(--space-12);
    color: var(--muted);
  }
</style>
```

Run: `cd web && pnpm vitest run app/views/ViewPage.test.ts`
Expected: PASS

- [ ] **Step 16: 検証とコミット**

Run（前面。Bash の timeout は 600000）: `cd web && pnpm fix && pnpm check && pnpm lint && pnpm vitest run app/views app/list app/ui app/states`
Expected: すべて通る（`pnpm check` は警告 0、`pnpm lint` は指摘 0）。全件（`pnpm test`）と E2E は Task 7 の結線のあとに流す（この Task の画面は Task 7 まで入口がない）。

```bash
git add web/app/ui/Tabs.svelte web/app/ui/tabs.test.ts web/app/states/Empty.svelte web/app/states/Empty.test.ts web/app/views web/app/list/SaveViewButton.svelte web/app/list/SaveViewButton.test.ts web/app/list/ListTab.svelte web/app/list/ListTab.test.ts
git commit -m "feat(web): ビューの画面・クエリの画面・ビューのダイアログ、カンバン・テーブルのタブの入れ物

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

**確かめたこと（書き手）**: この節のコードは、`web/` の写し（main の 79a5277）に、部 A〜D の名前と型の最小のスタブ（`core/views.ts`、`router.ts` の `Route`・`replace`、`cli.ts` の `cliView*`、`list/scope*`、`AppCore.views`、TreeView・KanbanView・TableView の props）を置いて、`pnpm fix`・`pnpm check`（警告 0）・`pnpm exec biome ci --vcs-enabled=false .`・この Task のテスト（`app/views`・`app/list`・`app/ui`・`app/states`）を通した。スタブの置き換えで落ちた既存のテスト（`router.test.ts` の kanban/table のパス、`core-factory.test.ts` の core の数、TreeView の2つのテスト）は、部 A・B の担当。E2E は流していない。写しで見つけた誤り: `combine("", filter)` は先頭に空白が付くので、全 Store の画面の `saveBase`（`""`）では `.trim()` が要る（`SaveViewButton`）。

### Task 7: マイタスク・もしかして・入口の結線・E2E（細部 55〜58、16・17・34 の 4e-3 分）

Task 2〜6 で作った画面に入口を付ける最後の Task。マイタスクの画面（細部 55〜57）と、クエリの誤りの「もしかして」（細部 58）を作り、ワークスペースとプロジェクトのカンバン・テーブルのタブ、ツリーのタブの「ビューを保存」、サイドバーのマイタスクと保存済みビュー（細部 16・17）、パレットのマイタスク・保存済みビュー・「このクエリで」（細部 34）、App の振り分けをつなぎ、E2E（細部 80 の 4e-3 分）を足す。

**この Task は単独で通る**（`pnpm check`・`pnpm test` が通り、Step 21 で `make e2e` が通る）。コミットは 6 つ（Step 9・15・19・20b・20c・22）。

**Files:**
- Create: `web/app/my/groups.ts`、`groups.test.ts`、`MyTasks.svelte`、`MyTasks.test.ts`（マイタスク）
- Create: `web/app/list/suggest.ts`、`suggest.test.ts`（もしかして）
- Create: `web/app/list/TreeTab.svelte`、`TreeTab.test.ts`（ツリーのタブ。「ビューを保存」を付ける）
- Create: `web/app/project/ProjectHeader.svelte`、`ProjectHeader.test.ts`、`ProjectPage.svelte`、`ProjectPage.test.ts`（プロジェクトの見出しとタブ）
- Create: `web/e2e/kanban.spec.ts`、`table.spec.ts`、`my.spec.ts`、`views.spec.ts`
- Modify: `web/app/tree/FilterBar.svelte`、`FilterBar.test.ts`（もしかして）
- Modify: `web/app/tree/TreeView.svelte`、`TreeView.test.ts`（プロジェクトの見出しとタブを `ProjectHeader` に移す）
- Modify: `web/app/workspace/WorkspaceHeader.svelte`、`WorkspaceHeader.test.ts`、`WorkspacePage.svelte`、`WorkspacePage.test.ts`（カンバン・テーブルのタブ、ツリーのタブを `TreeTab` に）
- Modify: `web/app/sidebar/Sidebar.svelte`、`Sidebar.test.ts`（マイタスクと保存済みビュー）
- Modify: `web/app/palette/search.ts`、`search.test.ts`、`CommandPalette.svelte`、`CommandPalette.test.ts`（マイタスク・保存済みビュー・このクエリで）
- Modify: `web/app/App.svelte`、`App.test.ts`（振り分け、`ViewDialogs`）
- Modify: `web/app/undo.ts`、`undo.test.ts`（保存済みビューの画面の Store）
- Modify: `web/app/test-utils.ts`（`fakeCore` の `cache.watchList`）
- Modify: `web/e2e/routes.spec.ts`、`screens.ts`（タブ、サイドバー、ドラッグの補助）
- Test: 上の各 `*.test.ts`・`*.spec.ts`

**Interfaces:**
- Consumes（部 A〜E の最終形。このコードは、これと同じ名前と型の最小のスタブで動かして確かめた）:
  - Task 2: `web/app/router.ts` の `Route`（`view`・`query`・`my` を含む）、`Layout`、`WorkspaceTab`・`ProjectTab`（`kanban`・`table` を含む）、`router.href(route)`、`hrefOf`。`web/core/views.ts` の `ViewEntry = { store; tone; view }`、`AppCore.views`（`ViewDirectory` の `list()`・`subscribe()`）。`web/app/list/scope.ts` の `ListScope`、`scope.svelte.ts` の `useListScope(get)`（`ready`・`store`・`workspace`・`project`）、`ListBar.svelte`。`FilterBar` は `base` が `""` のとき固定クエリの札を出さない
  - Task 3: `TreeView` の props `{ view: ListScope; query?: string; onquery?: (q: string) => void; cli?: string; actions?: Snippet }`
  - Task 6: `web/app/views/dialogs-state.svelte.ts` の `openViewSave({ draft, store? })`、`web/app/views/draft.ts` の `newDraft({ layout, query, kanban?, table? })`、`web/app/list/SaveViewButton.svelte`（`{ view; layout; filter; kanban?; table? }`）、`ListTab.svelte`（`{ view; layout: "kanban" | "table" }`）、`web/app/views/ViewPage.svelte`（`{ store; name }`）、`QueryPage.svelte`（`{ layout; q }`）、`ViewDialogs.svelte`（props なし）
  - 既存: `core()`、`useSubscribe`、`TaskCache.watchList`（`ulids()`・`loading()`・`error()`・`dispose()`）、`SchemaCache.get/load`、`StoreDirectory.stores()/tone()`、`StoreItem.account`（`host/login`）、`ApiError`（`type`・`detail`）、`errorMessage`、`cliLs`、`formatDate`、`propText`（`tree/marks`）、`splitTerms`・`NO_CANCELED`（`tree/filter`）、`IdLabel`・`StoreBadge`・`Menu`・`Tabs`、`StatusIcon`・`KindIcon`・`Icon`（`checklist`・`filter` を含む）、`states/Empty.svelte`、`renderWithCore`・`fakeCore`、`Hedgehug`（`web/e2e/harness.ts`）・`row`・`toast`（`screens.ts`）
- Produces:
  - `web/app/my/groups.ts`: `MY_QUERY = "assignee:@me -status:done -status:canceled"`、`type MyGroupBy = "due" | "project"`、`type DueBucket = "overdue" | "today" | "week" | "later" | "none"`、`type MyGroup = { key: string; name: string; tone: "alert" | "plain"; tasks: TaskView[] }`、`NO_PROJECT = "プロジェクトなし"`、`dueDay(t: TaskView): number | null`（日付の通し番号）、`bucketOf(day: number | null, today: Date): DueBucket`（週は月曜はじまり）、`groupMyTasks(tasks: TaskView[], by: MyGroupBy, today: Date): MyGroup[]`、`workspaceCount(tasks): number`、`loginsOf(stores: { account?: string }[]): string[]`
  - `web/app/my/MyTasks.svelte`: props なし。見出し・「自分 = …」・フィルタ欄（全体が編集できる。既定は `MY_QUERY`）・件数「n ワークスペース · m 件」・「まとめ方: 期限 / プロジェクト」（`ui/Menu`）・グループごとの表
  - `web/app/list/suggest.ts`: `suggestTerm(error: unknown, schemas: SchemaView[]): string | null`（直した語。例 `"status:in_progress"`）、`replaceTerm(q: string, error: unknown, to: string): string | null`（`q` の誤った語を `to` に替えたクエリ。語が `q` になければ null）
  - `web/app/tree/FilterBar.svelte`: 誤りの下に「<語> のことですか？」（語が名前のボタン。押すと `onchange` に直したクエリを渡す）。`core()` がない場所で描いても動く（出さないだけ）
  - `web/app/list/TreeTab.svelte`: props `{ view: { kind: "workspace"; store; ulid } | { kind: "project"; ulid } }`。フィルタを持ち（既定 `-status:canceled`）、`TreeView` の `actions` に `SaveViewButton`（layout `tree`）を入れる
  - `web/app/project/ProjectHeader.svelte`: props `{ project: TaskView; workspace?: WorkspaceEntry; tab: ProjectTab }`（パンくず、タブ ツリー・カンバン・テーブル）。`ProjectPage.svelte`: props `{ ulid: string; tab: ProjectTab }`
  - `web/app/palette/search.ts`: `CommandEnv` に `saveQuery: (q: string) => void`・`views: ViewEntry[]`・`hrefs.view(store, name)`・`hrefs.query(layout, q)`・`hrefs.my()`、`queryCommands(env, q): PaletteGroup`（「このクエリで」: テーブルで開く、ビューとして保存…）。`CommandItem.icon` に `checklist`・`filter`
  - `web/app/undo.ts`: `routeStore` が `view` の画面の Store を返す
  - 画面の言葉（E2E が使う。`screens.ts` にも書いた）: サイドバーのリンク「マイタスク」（件数つき）、リージョン「保存済みビュー」の各リンク（`title` は「<Store> の views/ に保存」）、ワークスペースとプロジェクトのタブ ナビ「ビューの切り替え」のリンク「カンバン」「テーブル」、マイタスクのリージョン「今日」「今週」「来週以降」「期限切れ」「期限なし」（まとめ方がプロジェクトなら、プロジェクト名と「プロジェクトなし」）、ボタン「まとめ方: …」、誤りの下のボタン（名前は直した語）と「のことですか？」、パレットのグループ「このクエリで」

**この Task で決めたこと**
- **マイタスク**（細部 55〜57）: 問い合わせは Store を指定せずに全 Store へ（`watchList({ q, tree: false })`）。フィルタ欄はクエリ全体を持ち、既定は `MY_QUERY`。サイドバーの件数は、画面のフィルタ欄がどうでも、いつも `MY_QUERY` の件数。「自分 = …」は `GET /api/stores` の `account`（`host/login`）の login を、重ならないように ` ＋ ` でつなぐ（個人・仕事の別名は 4f）。まとめ方は期限（期限切れ・今日・今週・来週以降・期限なし。月曜はじまり、ブラウザのローカルの日付）とプロジェクト（プロジェクトの外は最後）の 2 つで、細部 57 のとおり。キャンバスの「ワークスペース」のまとめ方と、非表示にした Store の注意書き（org-b/ops を非表示にしているため…）は作らない（Store を隠す機能が 4f）。ID は Store の修飾なしで、行の右に Store のバッジ。今日の日付は、タスクかまとめ方が変わるたびに取り直す
- 一致するタスクがないときは `Empty`（nomatch）。「フィルタを解除」は、クエリを最初の状態（`MY_QUERY`）に戻す（フィルタを空にすると全員のタスクになり、マイタスクでなくなるため）。既定のクエリで 0 件のときは解除を出さない
- **もしかして**（細部 58）: サーバーの `query.invalid` は `args` を持たず、原因は英語の `detail`（`term 1 (status:in_progres): unknown status or category "in_progres"`）にしかない。そのため `detail` を読む。直せるのは、ステータス・種類・キー（クエリの組み込みのキーとスキーマのプロパティ）・enum の値の誤り。候補は編集距離 2 以内でいちばん近いもの（同点は先に並ぶほう）で、1〜2 文字の語は丸ごと書き換えになるなら出さない。近い名前がなければ、打った語（3 文字以上）を含む名前（先に並ぶほう）を候補にする。キャンバスの `progres` → `status:in_progress`（距離 4）が出るように、細部 58（編集距離 2 以内）をこう直す（利用者の決定 2026-11-03。概要 plan の細部 58 も同じ文に直す）。スキーマは、誤りがあるときに全 Store の分を読む。画面が固定している部分（`workspace:`・`project:`）の語は直せないので出さない。Store 名の誤りは、名前がスキーマにないので対象外
- **プロジェクトの画面**: 見出し（パンくず・タブ）を `TreeView` から `ProjectHeader` に出し、`ProjectPage` が見出しと本文（ツリー・カンバン・テーブル）を持つ。ワークスペースの `WorkspacePage` と同じ作り。カンバンとテーブルは `ListTab` の 1 か所で描く（タブを替えてもフィルタが残る）。プロジェクトの設定のタブは 4e-4
- **ツリーのタブ**の「ビューを保存」は `TreeTab` が付ける（フィルタを持つのがここだから）。ツリーとカンバン・テーブルのあいだでフィルタは引き継がない（別の部品が持つ）。保存するクエリは `SaveViewButton`（Task 6）が `saveBase` とフィルタから作る
- **パレット**（細部 34）: 「マイタスクを開く」と各保存済みビューの「<名前> を開く」は「開く」のグループの末尾に置く（Store のバッジつき）。「このクエリで」は、クエリのモードで一致するタスクがあり、誤りがないときだけ出す（利用者の決定 2026-11-03）。「テーブルで開く」は `/query/table?q=`（保存しない。D1）、「ビューとして保存…」はテーブルのレイアウトで保存ダイアログを開く（テーブルは `sort:`・`group:` を持つクエリも保存できるため）。キャンバスの「一致する n 件をまとめて更新…」はフェーズ 6
- **undo** のコマンドは、保存済みビューの画面ではそのビューの Store に向ける（`routeStore`）。クエリの画面とマイタスクは Store を決めない
- App の起動時の URL の整え（`router.go(location.pathname, …)`）は、クエリの画面の `?q=` を落としていたので、`location.search` も渡す
- 通知・アクティビティの入口は出さない（4e-4）。出さないことをテストで確かめる

**確かめたこと（書き手）**: この節のコードは、`web/` の写し（main の 79a5277）に、部 A〜E の実物（Task 2 の `router.ts`・`core/views.ts`・`list/scope*`・`ListBar.svelte` と `FilterBar` の札、Task 6 の `views/draft.ts`・`dialogs-state.svelte.ts`・`SaveViewButton.svelte`・`ListTab.svelte`）と、実物の代わりの最小のスタブ（Task 3 の `TreeView` の props、Task 4・5 の `KanbanView`・`TableView`、Task 6 の `ViewPage`・`QueryPage`・`ViewDialogs`）を置いたものに、Step の差分をそのまま当てて、`pnpm fix`・`pnpm check`（警告 0）・`pnpm exec biome ci --vcs-enabled=false .`・`pnpm vitest run`（全件）を通した。全件の1回目と2回目に、この Task と関係のない `app/tree/TreeView.test.ts` の「lets a late wait for a row give way to a newer one」（`AssertionError: expected null to be 'M199'`、1.3 秒）と、2回目に `app/detail/BodyEditor.test.ts` の「offers the people for @ from GET /api/people」（2.7 秒）が落ちた。どちらも単独では通り、3回目の全件も通った（96 ファイルの並列、WSL2、Node 26 の混んだときの時間切れ）。Step 13 の `TreeView.svelte`・`TreeView.test.ts` は、Task 3 の写し（w-B）の最終形のファイルを置いて当て、「変更前」が一字一句合うことと、`pnpm check`・biome・テストが通ることを確かめた。E2E は流していない（Step 21 で流す）。E2E の言葉は、Task 4・5・6 の plan に書かれた目印（盤面・セル・カード・ダイアログの名前）に合わせて書いた。

- [ ] **Step 1: 「もしかして」の純関数のテストを書く（失敗する）**

`web/app/list/suggest.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { ApiError } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import { replaceTerm, suggestTerm } from "./suggest";

const schema = {
  statuses: [
    { name: "backlog", category: "todo" },
    { name: "in_progress", category: "doing" },
    { name: "done", category: "done" },
    { name: "canceled", category: "canceled" },
  ],
  kinds: [{ name: "story" }, { name: "task" }, { name: "bug" }],
  props: {
    priority: { type: "enum", values: ["low", "high", "urgent"] },
    points: { type: "number" },
  },
} as unknown as SchemaView;
// A second store: a status and a property the first one lacks.
const other = {
  statuses: [{ name: "review", category: "doing" }],
  kinds: [{ name: "task" }],
  props: { estimate: { type: "number" } },
} as unknown as SchemaView;

// The server's words are English and cannot be told apart by anything but the detail: that is what
// the query error carries (it has no args).
const bad = (detail: string) => new ApiError({ status: 400, type: "query.invalid", detail });

describe("suggestTerm", () => {
  it("fixes a status that is a letter off", () => {
    const e = bad('term 1 (status:in_progres): unknown status or category "in_progres"');
    expect(suggestTerm(e, [schema])).toBe("status:in_progress");
  });

  it("keeps the minus sign and the other values of the term", () => {
    const e = bad('term 2 (-status:backlgo,done): unknown status or category "backlgo"');
    expect(suggestTerm(e, [schema])).toBe("-status:backlog,done");
  });

  it("offers the categories as statuses, and the statuses of every store", () => {
    expect(
      suggestTerm(bad('term 1 (status:doin): unknown status or category "doin"'), [schema]),
    ).toBe("status:doing");
    expect(
      suggestTerm(bad('term 1 (status:reveiw): unknown status or category "reveiw"'), [
        schema,
        other,
      ]),
    ).toBe("status:review");
  });

  it("fixes a kind", () => {
    expect(suggestTerm(bad('term 1 (kind:stroy): unknown kind "stroy"'), [schema])).toBe(
      "kind:story",
    );
  });

  it("fixes a key: the ones of the query and the properties of the schemas", () => {
    expect(suggestTerm(bad('term 1 (asignee:@me): unknown key "asignee"'), [schema])).toBe(
      "assignee:@me",
    );
    expect(suggestTerm(bad('term 1 (point>=3): unknown key "point"'), [schema])).toBe("points>=3");
    expect(suggestTerm(bad('term 1 (-prioriti:high): unknown key "prioriti"'), [schema])).toBe(
      "-priority:high",
    );
    expect(suggestTerm(bad('term 1 (estimat>2): unknown key "estimat"'), [schema, other])).toBe(
      "estimate>2",
    );
  });

  it("fixes a value of an enum from the list the server gives", () => {
    const e = bad('term 1 (priority:hgh): "hgh" is not one of [low high urgent]');
    expect(suggestTerm(e, [schema])).toBe("priority:high");
  });

  it("takes a word of three letters or more that is part of a name (the example of the design)", () => {
    const e = bad('term 1 (status:progres): unknown status or category "progres"');
    expect(suggestTerm(e, [schema])).toBe("status:in_progress");
    expect(
      suggestTerm(bad('term 1 (status:prog): unknown status or category "prog"'), [schema]),
    ).toBe("status:in_progress");
    // the nearest name still wins over a name that merely holds the word
    expect(
      suggestTerm(bad('term 1 (status:dne): unknown status or category "dne"'), [schema]),
    ).toBe("status:done");
  });

  it("gives nothing when no name is near or holds the word, or the word is too short to tell", () => {
    expect(
      suggestTerm(bad('term 1 (status:zzzz): unknown status or category "zzzz"'), [schema]),
    ).toBeNull();
    expect(suggestTerm(bad('term 1 (x:1): unknown key "x"'), [schema])).toBeNull();
    // two letters are not taken for the part of a name
    expect(
      suggestTerm(bad('term 1 (status:gr): unknown status or category "gr"'), [schema]),
    ).toBeNull();
  });

  it("gives nothing for another error, or for something that is not an error of the query", () => {
    expect(suggestTerm(bad("term 1 (a): unclosed quote"), [schema])).toBeNull();
    expect(
      suggestTerm(new ApiError({ status: 404, type: "task.not_found", detail: "x" }), [schema]),
    ).toBeNull();
    expect(suggestTerm(null, [schema])).toBeNull();
    expect(suggestTerm(new Error("network"), [schema])).toBeNull();
  });

  it("gives nothing while no schema has been read", () => {
    const e = bad('term 1 (status:in_progres): unknown status or category "in_progres"');
    expect(suggestTerm(e, [])).toBeNull();
  });
});

describe("replaceTerm", () => {
  const e = bad('term 2 (status:in_progres): unknown status or category "in_progres"');

  it("puts the fixed term where the wrong one stood", () => {
    expect(replaceTerm("assignee:@me status:in_progres kind:task", e, "status:in_progress")).toBe(
      "assignee:@me status:in_progress kind:task",
    );
  });

  it("gives nothing when the term is not in the part the user types (it is the fixed part)", () => {
    expect(replaceTerm("assignee:@me", e, "status:in_progress")).toBeNull();
  });

  it("changes only the term, not another that holds the same letters", () => {
    expect(replaceTerm("status:in_progres2 status:in_progres", e, "status:in_progress")).toBe(
      "status:in_progres2 status:in_progress",
    );
  });
});
```

Run: `cd web && pnpm vitest run app/list/suggest.test.ts`
Expected: FAIL（`Failed to resolve import "./suggest"`）

- [ ] **Step 2: 「もしかして」を実装する**

`web/app/list/suggest.ts`:

```ts
import type { SchemaView } from "../../core/schema";
import { splitTerms } from "../tree/filter";

// "もしかして": the term of a query the server refused, written again with the name it is
// closest to. The server tells the cause only in English in the detail of query.invalid
// ("term 1 (status:progres): unknown status or category "progres""), so that is what is read.

const CATEGORIES = ["todo", "doing", "done", "canceled"];
// The keys the query reads on its own (internal/query/eval.go, sort.go, mount/query.go).
const QUERY_KEYS = [
  "status",
  "kind",
  "label",
  "assignee",
  "watching",
  "mentions",
  "parent",
  "project",
  "workspace",
  "blocked_by",
  "blocks",
  "is",
  "store",
  "sort",
  "group",
];
const MAX_DISTANCE = 2;
// The shortest word that is taken for the part of a name.
const MIN_CONTAINED = 3;

type Problem = {
  raw: string; // the term as it was typed
  what: "status" | "kind" | "key" | "value";
  word: string; // the name that is wrong
  options?: string[]; // the names the server lists (an enum)
};

const HEAD = /^term \d+ \((.*?)\): (.*)$/s;
const NAMED: [RegExp, "status" | "kind" | "key"][] = [
  [/^unknown status or category "(.*)"$/s, "status"],
  [/^unknown kind "(.*)"$/s, "kind"],
  [/^unknown key "(.*)"$/s, "key"],
];
const ENUM = /^"(.*)" is not one of \[(.*)\]$/s;

function problemOf(error: unknown): Problem | null {
  if (typeof error !== "object" || error === null) return null;
  const e = error as { type?: unknown; detail?: unknown };
  if (e.type !== "query.invalid" || typeof e.detail !== "string") return null;
  const head = HEAD.exec(e.detail);
  if (!head) return null;
  const raw = head[1] ?? "";
  const msg = head[2] ?? "";
  for (const [re, what] of NAMED) {
    const m = re.exec(msg);
    if (m) return { raw, what, word: m[1] ?? "" };
  }
  const m = ENUM.exec(msg);
  if (m) return { raw, what: "value", word: m[1] ?? "", options: (m[2] ?? "").split(" ") };
  return null;
}

// distance is the edit distance (insert, delete, change one letter) of two words.
function distance(a: string, b: string): number {
  let prev = Array.from({ length: b.length + 1 }, (_, j) => j);
  for (let i = 1; i <= a.length; i++) {
    const row = [i];
    for (let j = 1; j <= b.length; j++) {
      const same = a[i - 1] === b[j - 1] ? 0 : 1;
      row.push(Math.min((prev[j] ?? 0) + 1, (row[j - 1] ?? 0) + 1, (prev[j - 1] ?? 0) + same));
    }
    prev = row;
  }
  return prev[b.length] ?? 0;
}

// closest is the name within two letters of the word, the nearest, the first on a tie. A word of
// one or two letters is left alone when the fix would rewrite all of it. With none near, a word of
// three letters or more that is part of a name is that name.
function closest(word: string, names: string[]): string | null {
  const w = word.toLowerCase();
  let best: string | null = null;
  let bestDistance = Infinity;
  for (const name of names) {
    const d = distance(w, name.toLowerCase());
    if (d <= MAX_DISTANCE && d < w.length && d < bestDistance) {
      best = name;
      bestDistance = d;
    }
  }
  if (best !== null || w.length < MIN_CONTAINED) return best;
  // A word that is part of a name ("progres" of "in_progress") is the name, when no name is near.
  return names.find((name) => name.toLowerCase().includes(w)) ?? null;
}

function uniq(names: string[]): string[] {
  return [...new Set(names)];
}

function namesFor(p: Problem, schemas: SchemaView[]): string[] {
  switch (p.what) {
    case "status":
      return uniq([
        ...schemas.flatMap((s) => (s.statuses ?? []).map((x) => x.name)),
        ...CATEGORIES,
      ]);
    case "kind":
      return uniq(schemas.flatMap((s) => (s.kinds ?? []).map((k) => k.name)));
    case "key":
      return uniq([...QUERY_KEYS, ...schemas.flatMap((s) => Object.keys(s.props ?? {}))]);
    case "value":
      return p.options ?? [];
  }
}

// The term with the wrong name changed for the right one.
function fixed(p: Problem, name: string): string {
  if (p.what === "key") {
    const m = /^(-?)([^:<>=]*)(.*)$/s.exec(p.raw);
    return m && m[2] === p.word ? `${m[1]}${name}${m[3]}` : p.raw;
  }
  // The value is after the operator; the key may hold the same letters.
  const at = p.raw.search(/[:<>=]/);
  const rest = p.raw.slice(at + 1);
  return `${p.raw.slice(0, at + 1)}${rest.replace(p.word, name)}`;
}

/**
 * The term the error is about, written with the name it is closest to (within two letters):
 * "status:in_progres" is "status:in_progress". Null when the error is not one of a name, or no
 * name is near.
 */
export function suggestTerm(error: unknown, schemas: SchemaView[]): string | null {
  const p = problemOf(error);
  if (!p) return null;
  const name = closest(p.word, namesFor(p, schemas));
  if (name === null) return null;
  const to = fixed(p, name);
  return to === p.raw ? null : to;
}

/**
 * The query with the term the error is about changed for `to` (what suggestTerm gave). Null when
 * the term is not in q: it is in the part the screen fixes, which the user cannot change.
 */
export function replaceTerm(q: string, error: unknown, to: string): string | null {
  const p = problemOf(error);
  if (!p) return null;
  const terms = splitTerms(q);
  const at = terms.indexOf(p.raw);
  if (at < 0) return null;
  terms[at] = to;
  return terms.join(" ");
}
```

Run: `cd web && pnpm vitest run app/list/suggest.test.ts`
Expected: PASS

- [ ] **Step 3: FilterBar のテストを足す（失敗する）**

`web/app/tree/FilterBar.test.ts` の差分（`@@` の行は目安。該当する箇所に当てる。Task 2 が足した「固定クエリの札」のテストは、そのまま残す）:

```diff
--- a/web/app/tree/FilterBar.test.ts
+++ b/web/app/tree/FilterBar.test.ts
@@ -1,6 +1,31 @@
 import { fireEvent, render } from "@testing-library/svelte";
 import { describe, expect, it, vi } from "vitest";
+import { ApiError } from "../../core/api/client";
+import { renderWithCore } from "../test-utils";
 import FilterBar from "./FilterBar.svelte";
+
+const refused = new ApiError({
+  status: 400,
+  type: "query.invalid",
+  detail: 'term 1 (status:in_progres): unknown status or category "in_progres"',
+});
+const schema = { statuses: [{ name: "in_progress", category: "doing" }], kinds: [], props: {} };
+// The core of a screen that has read the schema of its one store.
+const core = {
+  stores: { stores: () => [{ name: "shared" }], subscribe: () => () => {} },
+  schema: { get: () => schema, load: async () => schema, subscribe: () => () => {} },
+};
+// Lets the schemas be read, so that a test that expects no suggestion has waited for one.
+const settle = () => new Promise((resolve) => setTimeout(resolve, 0));
+const props = (over: Record<string, unknown> = {}) => ({
+  base: "",
+  applied: "status:in_progres",
+  count: "0 件",
+  cli: "hh ls",
+  error: refused,
+  onchange: vi.fn(),
+  ...over,
+});
 
 describe("FilterBar", () => {
   it("does not apply the typed query after it is removed", async () => {
@@ -25,4 +50,45 @@
       vi.useRealTimers();
     }
   });
+
+  it("offers the name that is a letter off, and applies it when pressed", async () => {
+    const p = props();
+    const { getByRole, findByRole, getByText } = renderWithCore(FilterBar, p, core);
+    expect(getByRole("alert")).toHaveTextContent("クエリが正しくありません");
+    // the schemas are read first
+    const fix = await findByRole("button", { name: "status:in_progress" });
+    expect(getByText(/のことですか？/)).toBeTruthy();
+    await fireEvent.click(fix);
+    expect(p.onchange).toHaveBeenCalledWith("status:in_progress");
+  });
+
+  it("keeps the other terms of the query it fixes", async () => {
+    const p = props({ applied: "assignee:@me status:in_progres" });
+    const { findByRole } = renderWithCore(FilterBar, p, core);
+    await fireEvent.click(await findByRole("button", { name: "status:in_progress" }));
+    expect(p.onchange).toHaveBeenCalledWith("assignee:@me status:in_progress");
+  });
+
+  it("offers nothing when the wrong term is in the part the screen fixes", async () => {
+    const { queryByRole } = renderWithCore(
+      FilterBar,
+      props({ base: "workspace:demo", applied: "" }),
+      core,
+    );
+    await settle();
+    expect(queryByRole("button", { name: "status:in_progress" })).toBeNull();
+  });
+
+  it("offers nothing without a core, and still tells the error", async () => {
+    const { getByRole, queryByRole } = render(FilterBar, props());
+    await settle();
+    expect(getByRole("alert")).toBeTruthy();
+    expect(queryByRole("button", { name: "status:in_progress" })).toBeNull();
+  });
+
+  it("offers nothing when there is no error", async () => {
+    const { queryByRole } = renderWithCore(FilterBar, props({ error: null }), core);
+    await settle();
+    expect(queryByRole("button", { name: "status:in_progress" })).toBeNull();
+  });
 });
```

Run: `cd web && pnpm vitest run app/tree/FilterBar.test.ts`
Expected: FAIL（`Unable to find an accessible element with the role "button" and name "status:in_progress"`）

- [ ] **Step 4: FilterBar に「もしかして」を足す**

スキーマは `core()` から読むが、FilterBar は core のない場所（このファイルのテスト）でも描ける作りのまま、`core()` が投げたら出さないだけにする。`web/app/tree/FilterBar.svelte` の差分（Task 2 の「`base` が空なら札を出さない」の変更が入っている前提）:

```diff
--- a/web/app/tree/FilterBar.svelte
+++ b/web/app/tree/FilterBar.svelte
@@ -1,6 +1,9 @@
 <script lang="ts">
   import { errorMessage } from "../../core/messages";
+  import type { SchemaView } from "../../core/schema";
+  import { type AppCore, core } from "../context";
   import Icon from "../icons/Icon.svelte";
+  import { replaceTerm, suggestTerm } from "../list/suggest";
   import CliLine from "../ui/CliLine.svelte";
 
   type Props = {
@@ -37,6 +40,39 @@
   }
 
   const message = $derived(error ? errorMessage(error) : null);
+
+  // "もしかして": the term the server refused, written with the name it is closest to. The names
+  // are in the schemas of the stores, which are read when there is an error to explain. A bar
+  // drawn without the core (it holds the schemas) shows the error and cannot suggest.
+  function appCore(): AppCore | null {
+    try {
+      return core();
+    } catch {
+      return null;
+    }
+  }
+  const c = appCore();
+  let schemas = $state.raw<SchemaView[]>([]);
+  $effect(() => {
+    if (!c || !error) {
+      schemas = [];
+      return;
+    }
+    let live = true;
+    void Promise.all(c.stores.stores().map((s) => c.schema.load(s.name).catch(() => null))).then(
+      (all) => {
+        if (live) schemas = all.filter((s) => s !== null);
+      },
+    );
+    return () => {
+      live = false;
+    };
+  });
+  const fix = $derived.by(() => {
+    const to = error ? suggestTerm(error, schemas) : null;
+    const q = to ? replaceTerm(applied, error, to) : null;
+    return to && q !== null ? { to, q } : null;
+  });
 </script>
 
 <div class="bar">
@@ -63,6 +99,12 @@
     <p class="error" role="alert">
       {message}
     </p>
+    {#if fix}
+      <p class="hint">
+        <button type="button" class="fix" onclick={() => onchange(fix.q)}>{fix.to}</button>
+        のことですか？
+      </p>
+    {/if}
   {/if}
 </div>
 
@@ -117,6 +159,24 @@
     color: var(--danger);
     font-size: 0.78rem;
   }
+  .hint {
+    margin: var(--space-4) 0 0;
+    color: var(--ink-2);
+    font-size: 0.78rem;
+  }
+  .fix {
+    padding: 0.1rem 0.4rem;
+    border: 0;
+    border-radius: var(--radius-4);
+    background: var(--accent-weak);
+    color: var(--accent-ink);
+    font-family: var(--font-mono);
+    font-size: 0.75rem;
+    cursor: pointer;
+  }
+  .fix:hover {
+    background: var(--accent-on);
+  }
   .sr {
     position: absolute;
     width: 1px;
```

Run: `cd web && pnpm vitest run app/tree/FilterBar.test.ts app/list`
Expected: PASS

- [ ] **Step 5: マイタスクの純関数のテストを書く（失敗する）**

`web/app/my/groups.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { TaskView } from "../../core/api/client";
import { task } from "../tree/fixtures";
import {
  bucketOf,
  dueDay,
  groupMyTasks,
  loginsOf,
  MY_QUERY,
  NO_PROJECT,
  workspaceCount,
} from "./groups";

// Wednesday 2026-11-04, at an hour that is neither the start nor the end of the day.
const wed = new Date(2026, 10, 4, 15, 30);

function due(ulid: string, date: string | undefined, extra: Partial<TaskView> = {}): TaskView {
  return task(ulid, "task", null, { props: date ? { due: date } : {}, ...extra });
}

describe("MY_QUERY", () => {
  it("asks for the tasks of the person that are not finished", () => {
    expect(MY_QUERY).toBe("assignee:@me -status:done -status:canceled");
  });
});

describe("dueDay", () => {
  it("reads the day of a date, and null for none or for something that is not a date", () => {
    expect(dueDay(due("A", "2026-11-04"))).toBe(dueDay(due("B", "2026-11-04T23:59:00Z")));
    expect((dueDay(due("A", "2026-11-05")) ?? 0) - (dueDay(due("B", "2026-11-04")) ?? 0)).toBe(1);
    expect(dueDay(due("A", undefined))).toBeNull();
    expect(dueDay(due("A", "next week"))).toBeNull();
  });
});

describe("bucketOf: the week begins on Monday", () => {
  const at = (date: string, now = wed) => bucketOf(dueDay(due("A", date)), now);

  it("tells overdue, today, this week and later from a Wednesday", () => {
    expect(at("2026-11-03")).toBe("overdue");
    expect(at("2026-11-04")).toBe("today");
    expect(at("2026-11-05")).toBe("week");
    expect(at("2026-11-08")).toBe("week"); // Sunday is the last day of this week
    expect(at("2026-11-09")).toBe("later"); // Monday begins the next
    expect(bucketOf(null, wed)).toBe("none");
  });

  it("counts Monday as the first day and Sunday as the last", () => {
    const mon = new Date(2026, 10, 2, 0, 5);
    expect(at("2026-11-02", mon)).toBe("today");
    expect(at("2026-11-01", mon)).toBe("overdue"); // the Sunday before is last week
    expect(at("2026-11-08", mon)).toBe("week");
    expect(at("2026-11-09", mon)).toBe("later");
    const sun = new Date(2026, 10, 8, 23, 59);
    expect(at("2026-11-08", sun)).toBe("today");
    expect(at("2026-11-09", sun)).toBe("later"); // nothing is left of the week after today
  });
});

describe("groupMyTasks by due date", () => {
  it("groups in the order overdue, today, this week, later, none, and leaves empty groups out", () => {
    const tasks = [
      due("N", undefined),
      due("L", "2026-11-20"),
      due("W", "2026-11-06"),
      due("T", "2026-11-04"),
      due("O", "2026-10-30"),
    ];
    const groups = groupMyTasks(tasks, "due", wed);
    expect(groups.map((g) => [g.name, g.tasks.map((t) => t.ulid)])).toEqual([
      ["期限切れ", ["O"]],
      ["今日", ["T"]],
      ["今週", ["W"]],
      ["来週以降", ["L"]],
      ["期限なし", ["N"]],
    ]);
    expect(groups.map((g) => g.tone)).toEqual(["alert", "alert", "plain", "plain", "plain"]);
    expect(groupMyTasks([due("N", undefined)], "due", wed).map((g) => g.name)).toEqual([
      "期限なし",
    ]);
    expect(groupMyTasks([], "due", wed)).toEqual([]);
  });

  it("puts the soonest first within a group, and keeps the order of the ones that are equal", () => {
    const tasks = [due("B", "2026-11-07"), due("C", "2026-11-05"), due("A", "2026-11-05")];
    const [week] = groupMyTasks(tasks, "due", wed);
    expect(week?.tasks.map((t) => t.ulid)).toEqual(["C", "A", "B"]);
  });
});

describe("groupMyTasks by project", () => {
  const p = (ulid: string, title: string) => ({ ulid, id: `P-${ulid}`, title });

  it("groups by project, sorted by name, with the tasks outside any project last", () => {
    const tasks = [
      due("1", undefined),
      due("2", "2026-11-06", { project: p("PB", "ビルド") }),
      due("3", undefined, { project: p("PA", "あいうえお") }),
      due("4", "2026-11-05", { project: p("PB", "ビルド") }),
    ];
    const groups = groupMyTasks(tasks, "project", wed);
    expect(groups.map((g) => [g.name, g.tasks.map((t) => t.ulid)])).toEqual([
      ["あいうえお", ["3"]],
      ["ビルド", ["4", "2"]], // the sooner due date first
      [NO_PROJECT, ["1"]],
    ]);
  });

  it("keeps projects of the same name in two stores apart", () => {
    const tasks = [
      due("1", undefined, { project: p("P1", "release") }),
      due("2", undefined, { project: p("P2", "release") }),
    ];
    expect(groupMyTasks(tasks, "project", wed)).toHaveLength(2);
  });
});

describe("workspaceCount", () => {
  it("counts the workspaces the tasks are in, a workspace of another store apart", () => {
    const ws = (store: string, ulid: string) => ({ store, workspace: { ulid } }) as never;
    const tasks = [
      due("1", undefined, ws("shared", "W1")),
      due("2", undefined, ws("shared", "W1")),
      due("3", undefined, ws("shared", "W2")),
      due("4", undefined, ws("me", "W1")),
    ];
    expect(workspaceCount(tasks)).toBe(3);
    expect(workspaceCount([])).toBe(0);
  });
});

describe("loginsOf", () => {
  it("takes the login after the host, once each, and skips a store with no account", () => {
    expect(
      loginsOf([
        { account: "github.com/alice" },
        { account: "github.com/alice" },
        {},
        { account: "ghe.example.com/alice-corp" },
      ]),
    ).toEqual(["alice", "alice-corp"]);
    expect(loginsOf([])).toEqual([]);
  });
});
```

Run: `cd web && pnpm vitest run app/my/groups.test.ts`
Expected: FAIL（`Failed to resolve import "./groups"`）

- [ ] **Step 6: マイタスクの純関数を実装する**

`web/app/my/groups.ts`:

```ts
import type { TaskView } from "../../core/api/client";
import { propText } from "../tree/marks";

// What the first screen of a person asks for: the tasks given to them that are not finished.
// The filter field may change it; the count of the sidebar always asks this.
export const MY_QUERY = "assignee:@me -status:done -status:canceled";

export type MyGroupBy = "due" | "project";
export type DueBucket = "overdue" | "today" | "week" | "later" | "none";
export type MyGroup = {
  key: string;
  name: string;
  // alert: the due dates that need the eye (overdue, today)
  tone: "alert" | "plain";
  tasks: TaskView[];
};

export const NO_PROJECT = "プロジェクトなし";

const DAY = 86_400_000;
const DATE = /^(\d{4})-(\d{2})-(\d{2})/;

const BUCKETS: { key: DueBucket; name: string; tone: "alert" | "plain" }[] = [
  { key: "overdue", name: "期限切れ", tone: "alert" },
  { key: "today", name: "今日", tone: "alert" },
  { key: "week", name: "今週", tone: "plain" },
  { key: "later", name: "来週以降", tone: "plain" },
  { key: "none", name: "期限なし", tone: "plain" },
];

// The number of a calendar day: the same for every hour of it, and summer time does not move it.
function dayNumber(y: number, m: number, d: number): number {
  return Math.floor(Date.UTC(y, m - 1, d) / DAY);
}

/** The day a task is due as a day number, or null when it has no due date. */
export function dueDay(t: TaskView): number | null {
  const s = propText(t, "due");
  const m = s ? DATE.exec(s) : null;
  return m ? dayNumber(Number(m[1]), Number(m[2]), Number(m[3])) : null;
}

/** Which part of the time a due day is in, by the date of the browser. The week begins on Monday. */
export function bucketOf(day: number | null, today: Date): DueBucket {
  if (day === null) return "none";
  const now = dayNumber(today.getFullYear(), today.getMonth() + 1, today.getDate());
  if (day < now) return "overdue";
  if (day === now) return "today";
  // Sunday is the last day of the week (getDay() is 0), so only the days up to it are "this week".
  const sunday = now + ((7 - today.getDay()) % 7);
  return day <= sunday ? "week" : "later";
}

// By due date, soonest first; a task with none goes last. The sort is stable.
function byDue(a: TaskView, b: TaskView): number {
  const x = dueDay(a);
  const y = dueDay(b);
  if (x === y) return 0;
  if (x === null) return 1;
  if (y === null) return -1;
  return x - y;
}

/** The tasks in groups: by due date (overdue, today, this week, later, none) or by project. */
export function groupMyTasks(tasks: TaskView[], by: MyGroupBy, today: Date): MyGroup[] {
  const sorted = [...tasks].sort(byDue);
  if (by === "due") {
    return BUCKETS.map((b) => ({
      ...b,
      tasks: sorted.filter((t) => bucketOf(dueDay(t), today) === b.key),
    })).filter((g) => g.tasks.length > 0);
  }
  const byProject = new Map<string, MyGroup>();
  for (const t of sorted) {
    const key = t.project?.ulid ?? "";
    let g = byProject.get(key);
    if (!g) {
      g = { key, name: t.project?.title ?? NO_PROJECT, tone: "plain", tasks: [] };
      byProject.set(key, g);
    }
    g.tasks.push(t);
  }
  // Named projects by name; the tasks outside any project last.
  return [...byProject.values()].sort((a, b) =>
    a.key === "" ? 1 : b.key === "" ? -1 : a.name.localeCompare(b.name, "ja"),
  );
}

/** How many workspaces the tasks are in ("3 ワークスペース · 9 件"). */
export function workspaceCount(tasks: TaskView[]): number {
  return new Set(tasks.map((t) => `${t.store}/${t.workspace.ulid}`)).size;
}

/** The logins of the accounts of the stores ("host/login" is what the stores list), once each. */
export function loginsOf(stores: { account?: string }[]): string[] {
  const out: string[] = [];
  for (const s of stores) {
    if (!s.account) continue;
    const login = s.account.slice(s.account.lastIndexOf("/") + 1);
    if (!out.includes(login)) out.push(login);
  }
  return out;
}
```

Run: `cd web && pnpm vitest run app/my/groups.test.ts`
Expected: PASS

- [ ] **Step 7: マイタスクの画面のテストを書く（失敗する）**

`fakeCore` の `cache` は `watchList` を持たなかった（サイドバーの件数が使う）。ほかの画面のテストが `fakeCore` のままでも動くよう、何も見つけない `watchList` を足す。`web/app/test-utils.ts`:

変更前:
```ts
    cache: { subscribe: e.subscribe },
```

変更後:
```ts
    cache: {
      subscribe: e.subscribe,
      // a query that finds nothing
      watchList: () => ({ ulids: () => [], error: () => null, loading: () => false, dispose() {} }),
    },
```

`web/app/my/MyTasks.test.ts`:

```ts
import { cleanup, fireEvent, screen, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, type TaskView } from "../../core/api/client";
import { renderWithCore } from "../test-utils";
import { schema, task } from "../tree/fixtures";
import { MY_QUERY } from "./groups";
import MyTasks from "./MyTasks.svelte";

const project = { ulid: "P1", id: "REL-1", title: "リリース準備 v2.4" };
const t = (ulid: string, over: Partial<TaskView>): TaskView =>
  task(ulid, "task", null, { id: `REL-${ulid}`, title: `title ${ulid}`, ...over });

const tasks = [
  t("1", { props: { due: "2026-11-04" }, project, status: "doing" }),
  t("2", { props: { due: "2026-11-06" }, project }),
  t("3", {
    props: { due: "2026-11-07" },
    store: "me",
    workspace: { ulid: "W9", name: "me", id_prefix: "ME" },
  }),
  t("4", { props: {}, project }),
] as TaskView[];

function mount(over: { list?: () => string[]; error?: ApiError | null; loading?: boolean } = {}) {
  const watchList = vi.fn((_q: { q: string; tree: boolean }) => ({
    ulids: () => (over.list ? over.list() : tasks.map((x) => x.ulid)),
    error: () => over.error ?? null,
    loading: () => over.loading ?? false,
    dispose: vi.fn(),
  }));
  const none = () => () => {};
  const view = renderWithCore(
    MyTasks,
    {},
    {
      cache: {
        subscribe: none,
        watchList,
        task: (u: string) => tasks.find((x) => x.ulid === u),
      },
      stores: {
        stores: () => [
          { name: "shared", account: "github.com/alice" },
          { name: "me", account: "ghe.example.com/alice-corp" },
        ],
        tone: (s: string) => (s === "shared" ? "purple" : "teal"),
        loaded: () => true,
        subscribe: none,
      },
      schema: { get: () => schema, load: async () => schema, subscribe: none },
    },
  );
  return { watchList, ...view };
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 4, 12, 0)); // Wednesday
  history.replaceState(null, "", "/my");
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("MyTasks", () => {
  it("asks for the tasks of the person in every store, the default query", () => {
    const { watchList } = mount();
    expect(watchList).toHaveBeenCalledWith({ q: MY_QUERY, tree: false });
    expect(screen.getByLabelText("フィルタ（hh ls と同じクエリ）")).toHaveValue(MY_QUERY);
  });

  it("names the accounts, and counts the workspaces and the tasks", () => {
    mount();
    expect(screen.getByRole("heading", { level: 1, name: "マイタスク" })).toBeTruthy();
    expect(screen.getByText("自分 = alice ＋ alice-corp")).toBeTruthy();
    expect(screen.getByText("2 ワークスペース · 4 件")).toBeTruthy();
  });

  it("groups by due date: today, this week, none", () => {
    mount();
    const today = screen.getByRole("region", { name: "今日" });
    expect(within(today).getByText("title 1")).toBeTruthy();
    const week = screen.getByRole("region", { name: "今週" });
    expect(within(week).getByText("title 2")).toBeTruthy();
    expect(within(week).getByText("title 3")).toBeTruthy();
    const none = screen.getByRole("region", { name: "期限なし" });
    expect(within(none).getByText("title 4")).toBeTruthy();
    expect(screen.queryByRole("region", { name: "期限切れ" })).toBeNull();
  });

  it("shows the ID without a store, the project, the due date and the badge of the store", () => {
    mount();
    const row = screen.getByRole("row", { name: /title 2/ });
    expect(within(row).getByRole("link", { name: "REL-2" })).toHaveAttribute("href", "/tasks/2");
    expect(within(row).getByRole("link", { name: "title 2" })).toHaveAttribute("href", "/tasks/2");
    expect(within(row).getByText("リリース準備 v2.4")).toBeTruthy();
    expect(within(row).getByText("11/06")).toBeTruthy();
    expect(within(row).getByText("shared")).toBeTruthy();
    expect(within(screen.getByRole("row", { name: /title 3/ })).getByText("me")).toBeTruthy();
  });

  it("groups by project from the menu, the tasks outside any project last", async () => {
    mount();
    await fireEvent.click(screen.getByRole("button", { name: "まとめ方: 期限" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "プロジェクト" }));
    const p = screen.getByRole("region", { name: "リリース準備 v2.4" });
    expect(within(p).getAllByRole("row")).toHaveLength(3);
    const none = screen.getByRole("region", { name: "プロジェクトなし" });
    expect(within(none).getByText("title 3")).toBeTruthy();
    expect(screen.getByRole("button", { name: "まとめ方: プロジェクト" })).toBeTruthy();
  });

  it("asks again with the query typed in the field", async () => {
    const { watchList } = mount();
    const field = screen.getByLabelText("フィルタ（hh ls と同じクエリ）");
    await fireEvent.input(field, { target: { value: "assignee:@me kind:bug" } });
    await fireEvent.keyDown(field, { key: "Enter" });
    expect(watchList).toHaveBeenLastCalledWith({ q: "assignee:@me kind:bug", tree: false });
  });

  it("says there is nothing to do when the default query finds nothing", () => {
    mount({ list: () => [] });
    expect(screen.getByText("一致するタスクはありません")).toBeTruthy();
    expect(screen.getByText("自分が担当で、完了と中止でないタスクはありません")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "フィルタを解除" })).toBeNull();
  });

  it("goes back to the default query from a query that finds nothing", async () => {
    const { watchList } = mount({ list: () => [] });
    const field = screen.getByLabelText("フィルタ（hh ls と同じクエリ）");
    await fireEvent.input(field, { target: { value: "kind:bug" } });
    await fireEvent.keyDown(field, { key: "Enter" });
    await fireEvent.click(screen.getByRole("button", { name: "フィルタを解除" }));
    expect(watchList).toHaveBeenLastCalledWith({ q: MY_QUERY, tree: false });
  });

  it("says the list is being read", () => {
    mount({ loading: true });
    expect(screen.getByText("読み込み中…")).toBeTruthy();
    expect(screen.queryByText("一致するタスクはありません")).toBeNull();
  });

  it("tells why the query does not run, and draws no list", () => {
    mount({ error: new ApiError({ status: 400, type: "account.none" }) });
    expect(screen.getByRole("alert")).toHaveTextContent("アカウントが設定されていません");
    expect(screen.queryByRole("region")).toBeNull();
    expect(screen.queryByText("一致するタスクはありません")).toBeNull();
  });

  it("shows the command that gives the same list", () => {
    mount();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      `hh ls "${MY_QUERY}"`,
    );
  });
});
```

Run: `cd web && pnpm vitest run app/my/MyTasks.test.ts`
Expected: FAIL（`Failed to resolve import "./MyTasks.svelte"`）

- [ ] **Step 8: マイタスクの画面を実装する**

`web/app/my/MyTasks.svelte`:

```svelte
<script lang="ts">
  import { cliLs } from "../../core/cli";
  import { formatDate } from "../../core/format";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import ListBar from "../list/ListBar.svelte";
  import { onLinkClick, router } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import { propText } from "../tree/marks";
  import IdLabel from "../ui/IdLabel.svelte";
  import Menu from "../ui/Menu.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import {
    bucketOf,
    dueDay,
    groupMyTasks,
    loginsOf,
    MY_QUERY,
    type MyGroupBy,
    workspaceCount,
  } from "./groups";

  const c = core();
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));

  // The filter field holds the whole query: nothing on this screen is fixed.
  let applied = $state(MY_QUERY);
  let by = $state<MyGroupBy>("due");

  // Every store, so no store is asked for. The cache asks again at every tasks.changed and resync.
  let list = $state.raw<ReturnType<typeof c.cache.watchList> | null>(null);
  $effect(() => {
    const w = c.cache.watchList({ q: applied, tree: false });
    list = w;
    return () => w.dispose();
  });

  const loading = $derived.by(() => {
    cacheRev.current;
    return list === null || list.loading();
  });
  const error = $derived.by(() => {
    cacheRev.current;
    return list?.error() ?? null;
  });
  const tasks = $derived.by(() => {
    cacheRev.current;
    return (list?.ulids() ?? []).flatMap((u) => c.cache.task(u) ?? []);
  });
  // The date is read again whenever the tasks or the grouping change, so a tab left open over
  // midnight catches up at the next change.
  const grouped = $derived.by(() => {
    const now = new Date();
    return { now, groups: groupMyTasks(tasks, by, now) };
  });

  // The category of a status is in the schema of the store of the task.
  $effect(() => {
    for (const store of new Set(tasks.map((t) => t.store))) {
      c.schema.load(store).catch(() => undefined);
    }
  });
  function categoryOf(store: string, status: string): string | undefined {
    schemaRev.current;
    return c.schema.get(store)?.statuses?.find((s) => s.name === status)?.category;
  }

  const logins = $derived.by(() => {
    storesRev.current;
    return loginsOf(c.stores.stores());
  });
  const count = $derived(
    loading || error ? "" : `${workspaceCount(tasks)} ワークスペース · ${tasks.length} 件`,
  );
  const isDefault = $derived(applied === MY_QUERY);

  const taskHref = (ulid: string) => router.href({ name: "task", ulid });
  function dueText(t: Parameters<typeof propText>[0]): string {
    const d = propText(t, "due");
    return d ? formatDate(d, grouped.now) : "";
  }
  function dueAlert(t: Parameters<typeof propText>[0]): boolean {
    const b = bucketOf(dueDay(t), grouped.now);
    return b === "overdue" || b === "today";
  }

  let groupAnchor = $state<HTMLElement | null>(null);
  const groupItems = [
    { value: "due", label: "期限" },
    { value: "project", label: "プロジェクト" },
  ];
  const groupLabel = $derived(groupItems.find((i) => i.value === by)?.label);
</script>

<div class="my">
  <header class="head">
    <div class="title">
      <Icon name="checklist" size={16} />
      <h1>マイタスク</h1>
    </div>
    {#if logins.length > 0}
      <p class="me">自分 = {logins.join(" ＋ ")}</p>
    {/if}
  </header>

  <ListBar
    base=""
    {applied}
    {count}
    cli={cliLs(applied, false)}
    {error}
    onchange={(q) => (applied = q)}
  >
    {#snippet controls()}
      <button
        type="button"
        class="group-btn"
        aria-haspopup="menu"
        onclick={(e) => (groupAnchor = e.currentTarget)}
      >
        まとめ方: {groupLabel}
      </button>
    {/snippet}
  </ListBar>

  <div class="body">
    {#if error}
      <!-- the bar above tells why -->
    {:else if loading}
      <p class="msg">読み込み中…</p>
    {:else if tasks.length === 0}
      <Empty
        variant="nomatch"
        note={isDefault
          ? "自分が担当で、完了と中止でないタスクはありません"
          : "条件を最初の状態（自分が担当で、完了と中止でない）に戻せます"}
        onClear={isDefault ? undefined : () => (applied = MY_QUERY)}
      />
    {:else}
      {#each grouped.groups as g (g.key)}
        <section class="group" aria-labelledby="my-{g.key}">
          <div class="g-head">
            <h2 id="my-{g.key}" class:alert={g.tone === "alert"}>{g.name}</h2>
            <span class="sub">{g.tasks.length}</span>
          </div>
          <table>
            <tbody>
              {#each g.tasks as t (t.ulid)}
                {@const href = taskHref(t.ulid)}
                <tr>
                  <td class="w-status">
                    <StatusIcon status={t.status} category={categoryOf(t.store, t.status)} />
                  </td>
                  <td class="w-kind"><KindIcon kind={t.kind} /></td>
                  <td class="w-id">
                    <IdLabel id={t.id} ulid={t.ulid} {href} onopen={(e) => onLinkClick(e, href)} />
                  </td>
                  <td class="t-title">
                    <a {href} onclick={(e) => onLinkClick(e, href)}>{t.title}</a>
                  </td>
                  <td class="t-project">{by === "due" ? (t.project?.title ?? "") : ""}</td>
                  <td class="w-due" class:alert={dueAlert(t)}>{dueText(t)}</td>
                  <td class="w-store">
                    <StoreBadge store={t.store} tone={c.stores.tone(t.store)} compact />
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </section>
      {/each}
    {/if}
  </div>
</div>

{#if groupAnchor}
  <Menu
    anchor={groupAnchor}
    label="まとめ方"
    items={groupItems}
    selected={by}
    onselect={(v) => {
      by = v as MyGroupBy;
      groupAnchor = null;
    }}
    onclose={() => (groupAnchor = null)}
  />
{/if}

<style>
  .my {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--space-12);
    padding: var(--space-16) var(--gutter) 0;
  }
  .title {
    display: flex;
    align-items: center;
    gap: var(--space-10);
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .me {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .group-btn {
    min-height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink-2);
    font-size: var(--fs-12);
    white-space: nowrap;
    cursor: pointer;
  }
  .group-btn:hover {
    background: var(--hover);
  }
  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-20);
    padding: var(--space-8) var(--gutter) var(--space-20);
  }
  .msg {
    margin: 0;
    color: var(--muted);
  }
  .g-head {
    display: flex;
    align-items: baseline;
    gap: var(--space-8);
    margin-bottom: var(--space-4);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-13);
    font-weight: 600;
  }
  h2.alert,
  .alert {
    color: var(--danger);
  }
  .sub {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-13);
  }
  td {
    height: var(--row-h);
    padding: 0 var(--space-8);
    border-bottom: var(--hairline) solid var(--line-soft);
  }
  .w-status,
  .w-kind {
    width: 2rem;
  }
  .w-id {
    width: 6.5rem;
  }
  .w-due {
    width: 4.5rem;
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    color: var(--ink-2);
  }
  .w-due.alert {
    color: var(--danger);
    font-weight: 500;
  }
  .w-store {
    width: 9rem;
    text-align: right;
  }
  .t-title {
    min-width: 0;
  }
  .t-title a {
    color: var(--ink);
    text-decoration: none;
  }
  .t-title a:hover {
    text-decoration: underline;
  }
  .t-project {
    max-width: 14rem;
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-12);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
```

Run: `cd web && pnpm vitest run app/my`
Expected: PASS

- [ ] **Step 9: 通して、コミットする**

`FilterBar` は誤りがあるとき `c.stores.stores()` と `c.schema.load` を呼ぶ。`web/app/tree/TreeView.test.ts` の `setup()` が作る core の `stores` に `stores` を足す（誤りのあるクエリを描く「shows the message of query.invalid under the filter」などが `c.stores.stores is not a function` で落ちるため。`loaded` の次の行。Task 3 がこの `setup()` を変えていたら、同じ場所に）:

変更前:
```ts
    stores: {
      loaded: () => true,
      workspace: () =>
```

変更後:
```ts
    stores: {
      loaded: () => true,
      stores: () => [{ name: "shared" }],
      workspace: () =>
```

Run: `cd web && pnpm fix && pnpm check && pnpm exec biome ci --vcs-enabled=false . && pnpm vitest run app/my app/list app/tree`
Expected: 警告 0、指摘 0、すべて PASS

```bash
git add web/app/my web/app/list/suggest.ts web/app/list/suggest.test.ts web/app/tree/FilterBar.svelte web/app/tree/FilterBar.test.ts web/app/tree/TreeView.test.ts web/app/test-utils.ts
git commit -m "feat(web): マイタスクの画面と、クエリの誤りの「もしかして」を足す

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 10: プロジェクトの見出しとページのテストを書く（失敗する）**

`web/app/project/ProjectHeader.test.ts`:

```ts
import { cleanup, fireEvent, screen, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { TaskView } from "../../core/api/client";
import { renderWithCore } from "../test-utils";
import { task } from "../tree/fixtures";
import { entry } from "../workspace/testkit";
import ProjectHeader from "./ProjectHeader.svelte";

const project = task("P1", "project", null, {
  id: "REL-15",
  title: "リリース準備 v2.4",
}) as TaskView;

function mount(props: { tab: "tree" | "kanban" | "table"; withWorkspace?: boolean }) {
  return renderWithCore(
    ProjectHeader,
    { project, workspace: props.withWorkspace === false ? undefined : entry(), tab: props.tab },
    { stores: { tone: () => "purple", subscribe: () => () => {} } },
  );
}

beforeEach(() => history.replaceState(null, "", "/projects/P1"));
afterEach(cleanup);

describe("ProjectHeader", () => {
  it("shows where the project is: the store, the workspace, its name and its ID", () => {
    mount({ tab: "tree" });
    const crumb = screen.getByRole("navigation", { name: "パンくず" });
    expect(within(crumb).getByText("shared")).toBeTruthy();
    expect(within(crumb).getByRole("link", { name: /release/ })).toHaveAttribute(
      "href",
      "/workspaces/shared/W1",
    );
    expect(screen.getByRole("heading", { level: 1, name: "リリース準備 v2.4" })).toBeTruthy();
    expect(within(crumb).getByText("REL-15")).toBeTruthy();
  });

  it("goes to the overview of the workspace through the router", async () => {
    mount({ tab: "tree" });
    await fireEvent.click(screen.getByRole("link", { name: /release/ }));
    expect(location.pathname).toBe("/workspaces/shared/W1");
  });

  it("draws no workspace until it is known", () => {
    mount({ tab: "tree", withWorkspace: false });
    expect(screen.queryByRole("link", { name: /release/ })).toBeNull();
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy();
  });

  it("has the tabs tree, kanban and table as links, the current one marked", () => {
    mount({ tab: "kanban" });
    const links = within(screen.getByRole("navigation", { name: "ビューの切り替え" })).getAllByRole(
      "link",
    );
    expect(links.map((a) => a.textContent?.trim())).toEqual(["ツリー", "カンバン", "テーブル"]);
    expect(links.map((a) => a.getAttribute("href"))).toEqual([
      "/projects/P1",
      "/projects/P1/kanban",
      "/projects/P1/table",
    ]);
    expect(links.map((a) => a.getAttribute("aria-current"))).toEqual([null, "page", null]);
  });

  it("does not offer the settings of a project yet", () => {
    mount({ tab: "tree" });
    expect(screen.queryByRole("link", { name: "設定" })).toBeNull();
  });
});
```

`web/app/project/ProjectPage.test.ts`:

```ts
import { cleanup, screen } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import { renderWithCore } from "../test-utils";
import { task } from "../tree/fixtures";
import { entry } from "../workspace/testkit";
import ProjectPage from "./ProjectPage.svelte";

// The bodies of the tabs have their own tests: here each only says which one is drawn.
vi.mock("../list/TreeTab.svelte", () => ({
  default: (anchor: Comment) => {
    const p = document.createElement("p");
    p.textContent = "tree tab";
    anchor.before(p);
  },
}));
vi.mock("../list/ListTab.svelte", () => ({
  default: (anchor: Comment, props: { layout: string; view: { kind: string; ulid: string } }) => {
    const p = document.createElement("p");
    p.textContent = `${props.layout} tab of ${props.view.kind} ${props.view.ulid}`;
    anchor.before(p);
  },
}));

const project = task("P1", "project", null, {
  id: "REL-15",
  title: "リリース準備 v2.4",
  workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
}) as TaskView;

function mount(tab: "tree" | "kanban" | "table", read = true) {
  const none = () => () => {};
  return renderWithCore(
    ProjectPage,
    { ulid: "P1", tab },
    {
      cache: {
        task: (u: string) => (read && u === "P1" ? project : undefined),
        watchDetail: () => ({ ulid: () => null, error: () => null, dispose() {} }),
        subscribe: none,
      },
      stores: {
        workspaces: () => [entry()],
        workspace: () => entry(),
        loaded: () => true,
        tone: () => "purple",
        subscribe: none,
      },
      schema: { subscribe: none },
    },
  );
}

afterEach(cleanup);

describe("ProjectPage", () => {
  it("draws the heading with the tabs above the tree", () => {
    mount("tree");
    expect(screen.getByRole("heading", { level: 1, name: "リリース準備 v2.4" })).toBeTruthy();
    expect(screen.getByRole("navigation", { name: "ビューの切り替え" })).toBeTruthy();
    expect(screen.getByText("tree tab")).toBeTruthy();
  });

  it.each(["kanban", "table"] as const)("draws the %s tab for the project", (tab) => {
    mount(tab);
    expect(screen.getByText(`${tab} tab of project P1`)).toBeTruthy();
    expect(screen.queryByText("tree tab")).toBeNull();
  });

  it("leaves the body to say the project is missing, and draws no heading until it is read", () => {
    mount("kanban", false);
    expect(screen.queryByRole("heading")).toBeNull();
    expect(screen.getByText("kanban tab of project P1")).toBeTruthy();
  });
});
```

Run: `cd web && pnpm vitest run app/project`
Expected: FAIL（`Failed to resolve import "./ProjectHeader.svelte"`）

- [ ] **Step 11: プロジェクトの見出しとページを実装する**

見出しの作りは `TreeView` のプロジェクトの見出し（パンくず・Store のバッジ・ID）をそのまま移し、タブだけ 3 つにする。

`web/app/project/ProjectHeader.svelte`:

```svelte
<script lang="ts">
  import type { TaskView } from "../../core/api/client";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import { onLinkClick, type ProjectTab, router } from "../router";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";

  // The heading of a project: where it is (Store, workspace), its name and ID, and the tabs.
  // 4e-4 adds the settings of the project at the right end.
  type Props = { project: TaskView; workspace?: WorkspaceEntry; tab: ProjectTab };
  let { project, workspace, tab }: Props = $props();

  const c = core();
  const at = (t: ProjectTab) => router.href({ name: "project", ulid: project.ulid, tab: t });
  const items = $derived([
    { label: "ツリー", href: at("tree"), current: tab === "tree" },
    { label: "カンバン", href: at("kanban"), current: tab === "kanban" },
    { label: "テーブル", href: at("table"), current: tab === "table" },
  ]);
</script>

<header class="head">
  <nav class="crumb" aria-label="パンくず">
    <StoreBadge store={project.store} tone={c.stores.tone(project.store)} />
    {#if workspace}
      {@const wsHref = router.href({
        name: "workspace",
        store: workspace.store,
        ulid: workspace.ulid,
        tab: "overview",
      })}
      <a class="ws" href={wsHref} onclick={(e) => onLinkClick(e, wsHref)}>
        <Icon name="workspace" size={16} />{workspace.name}
      </a>
      <span class="sep" aria-hidden="true">›</span>
    {/if}
    <KindIcon kind="project" />
    <h1>{project.title}</h1>
    <span class="id">{project.id}</span>
  </nav>
</header>
<Tabs {items} />

<style>
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.75rem;
    padding: 1.125rem 1.75rem 0;
  }
  .crumb {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.625rem;
    min-width: 0;
  }
  .ws {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    color: var(--ink-2);
    text-decoration: none;
  }
  .sep {
    color: var(--muted-2);
  }
  h1 {
    margin: 0;
    font-size: 1.19rem;
    font-weight: 600;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    color: var(--muted);
  }
</style>
```

`web/app/project/ProjectPage.svelte`:

```svelte
<script lang="ts">
  import ListTab from "../list/ListTab.svelte";
  import { useListScope } from "../list/scope.svelte";
  import TreeTab from "../list/TreeTab.svelte";
  import type { ProjectTab } from "../router";
  import ProjectHeader from "./ProjectHeader.svelte";

  // The page of a project: the heading with the tabs, and the tab. The tabs read the project
  // themselves and say so when it is missing, so the heading waits for it and nothing more.
  type Props = { ulid: string; tab: ProjectTab };
  let { ulid, tab }: Props = $props();

  const view = $derived({ kind: "project" as const, ulid });
  const scope = useListScope(() => view);
</script>

<div class="page">
  {#if scope.project}
    <ProjectHeader project={scope.project} workspace={scope.workspace} {tab} />
  {/if}
  <div class="body">
    {#if tab === "tree"}
      <TreeTab {view} />
    {:else}
      <ListTab {view} layout={tab} />
    {/if}
  </div>
</div>

<style>
  /* The tree scrolls in its own box, so the page is the height of the window and the body takes
     what the heading leaves (as the page of a workspace does). */
  .page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .body {
    flex: 1 1 auto;
    min-height: 0;
  }
</style>
```

Run: `cd web && pnpm vitest run app/project`
Expected: PASS

- [ ] **Step 12: ツリーのタブのテストを書く（失敗する）→ 実装する**

`web/app/list/TreeTab.test.ts`:

```ts
import { cleanup, fireEvent } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DEFAULT_KANBAN, DEFAULT_TABLE } from "../../core/views";
import { renderWithCore } from "../test-utils";
import TreeTab from "./TreeTab.svelte";

// The tree has its own tests: here it shows the filter it is given and where its actions are.
const { open } = vi.hoisted(() => ({ open: vi.fn() }));
vi.mock("../tree/TreeView.svelte", () => ({
  default: (
    anchor: Comment,
    props: { query: string; onquery: (q: string) => void; actions: (anchor: Comment) => void },
  ) => {
    const p = document.createElement("p");
    p.textContent = `filter: ${props.query}`;
    const b = document.createElement("button");
    b.textContent = "絞り込む";
    b.onclick = () => props.onquery("kind:bug");
    anchor.before(p, b);
    props.actions(anchor);
  },
}));
vi.mock("../views/dialogs-state.svelte", () => ({ openViewSave: open }));

const workspace = {
  store: "shared",
  tone: "purple",
  ulid: "W1",
  name: "release",
  prefix: "REL",
  projects: [],
};
const stores = (count: number) => ({
  workspace: () => workspace,
  workspaces: () => [workspace],
  stores: () => Array.from({ length: count }, (_, i) => ({ name: i === 0 ? "shared" : `s${i}` })),
  loaded: () => true,
  subscribe: () => () => {},
});
const cache = { task: () => undefined, subscribe: () => () => {} };

function mount(count = 1) {
  return renderWithCore(
    TreeTab,
    { view: { kind: "workspace" as const, store: "shared", ulid: "W1" } },
    { stores: stores(count), cache },
  );
}

beforeEach(() => open.mockClear());
afterEach(cleanup);

describe("TreeTab", () => {
  it("gives the tree the filter that leaves the canceled tasks out", () => {
    const { getByText } = mount();
    expect(getByText("filter: -status:canceled")).toBeTruthy();
  });

  it("opens the save dialog with the fixed part and the filter, as a tree", async () => {
    const { getByRole } = mount();
    await fireEvent.click(getByRole("button", { name: "絞り込む" }));
    await fireEvent.click(getByRole("button", { name: "ビューを保存" }));
    expect(open).toHaveBeenCalledWith({
      draft: {
        layout: "tree",
        query: "workspace:release kind:bug",
        kanban: DEFAULT_KANBAN,
        table: DEFAULT_TABLE,
      },
      store: "shared",
    });
  });

  it("cannot save before the workspace is known", () => {
    const { getByRole } = renderWithCore(
      TreeTab,
      { view: { kind: "workspace" as const, store: "shared", ulid: "W1" } },
      { stores: { ...stores(1), workspace: () => undefined, loaded: () => false }, cache },
    );
    expect(getByRole("button", { name: "ビューを保存" })).toBeDisabled();
  });

  it("keeps the view to the store of the screen when two stores are mounted", async () => {
    const { getByRole } = mount(2);
    await fireEvent.click(getByRole("button", { name: "ビューを保存" }));
    expect(open.mock.calls[0]?.[0].draft.query).toBe(
      "store:shared workspace:release -status:canceled",
    );
  });
});
```

Run: `cd web && pnpm vitest run app/list/TreeTab.test.ts`
Expected: FAIL（`Failed to resolve import "./TreeTab.svelte"`）

`web/app/list/TreeTab.svelte`:

```svelte
<script lang="ts">
  import { NO_CANCELED } from "../tree/filter";
  import TreeView from "../tree/TreeView.svelte";
  import SaveViewButton from "./SaveViewButton.svelte";
  import type { ListScope } from "./scope";

  // The tree tab of a workspace or a project. The filter lives here, so that "ビューを保存" can
  // save what the field holds (ListTab does the same for the kanban and the table).
  type Props = { view: Extract<ListScope, { kind: "workspace" | "project" }> };
  let { view }: Props = $props();

  let query = $state(NO_CANCELED);
</script>

<TreeView {view} {query} onquery={(q) => (query = q)}>
  {#snippet actions()}
    <SaveViewButton {view} layout="tree" filter={query} />
  {/snippet}
</TreeView>
```

Run: `cd web && pnpm vitest run app/list/TreeTab.test.ts`
Expected: PASS

- [ ] **Step 13: TreeView からプロジェクトの見出しを外す**

見出しとタブは `ProjectHeader` にあるので、`TreeView` のものを消す。`web/app/tree/TreeView.svelte` の変更（変更前は Task 3 のあとの形。Task 3 は見出しの `store`・`workspace`・`project` を `useListScope` の `ls.store`・`ls.workspace`・`ls.project` に替えている。消すのは `{#if scope.kind === "project"}` の見出しのブロックだけ）:

変更前（テンプレートの先頭）:
```svelte
<section class="tree" aria-label="ツリー">
  <!-- The workspace has its own heading and tabs (WorkspaceHeader), which sit above this view. -->
  {#if scope.kind === "project"}
    <header class="head">
      <nav class="crumb" aria-label="パンくず">
        {#if ls.store}
          <StoreBadge store={ls.store} tone={c.stores.tone(ls.store)} />
        {/if}
        {#if ls.workspace}
          {@const wsHref = router.href({
            name: "workspace",
            store: ls.workspace.store,
            ulid: ls.workspace.ulid,
            tab: "overview",
          })}
          <a class="ws" href={wsHref} onclick={(e) => onLinkClick(e, wsHref)}>
            <Icon name="workspace" size={16} />{ls.workspace.name}
          </a>
          <span class="sep" aria-hidden="true">›</span>
        {/if}
        <KindIcon kind="project" />
        <h1>{crumbTitle}</h1>
        {#if ls.project}
          <span class="id">{ls.project.id}</span>
        {/if}
      </nav>
    </header>
    <Tabs
      items={[
        {
          label: "ツリー",
          href: router.href({ name: "project", ulid: scope.ulid, tab: "tree" }),
          current: true,
        },
      ]}
    />
  {/if}

  <ListBar
```

変更後:
```svelte
<section class="tree" aria-label="ツリー">
  <!-- The heading and the tabs of a workspace (WorkspaceHeader) and of a project (ProjectHeader)
       sit above this view, in their pages. -->
  <ListBar
```

使われなくなった import を消す（`pnpm fix` の noUnusedImports が教える。main では次の 5 つ）:
```ts
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";
```
と、`import { onLinkClick, router } from "../router";` を `import { router } from "../router";` に。

`<style>` から、見出しの規則（`.head`・`.crumb`・`.ws`・`.sep`・`h1`・`.id`）を消す（`ProjectHeader.svelte` に同じものがある）:
```css
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.75rem;
    padding: 1.125rem 1.75rem 0;
  }
  .crumb {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.625rem;
    min-width: 0;
  }
  .ws {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    color: var(--ink-2);
    text-decoration: none;
  }
  .sep {
    color: var(--muted-2);
  }
  h1 {
    margin: 0;
    font-size: 1.19rem;
    font-weight: 600;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    color: var(--muted);
  }
```

`web/app/tree/TreeView.test.ts` の変更。見出しとタブのテスト 2 つ（「opens a link of the breadcrumb through the router」「shows the tab of a project as a link to its tree」。どちらも `ProjectHeader.test.ts` が確かめている）を 1 つにする:

変更前:
```ts
  it("opens a link of the breadcrumb through the router", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(screen.getByRole("link", { name: /release/ }));
    expect(h.go).toHaveBeenCalledWith("/tasks/W1");
  });

  it("shows the tab of a project as a link to its tree", () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const tabs = screen.getByRole("navigation", { name: "ビューの切り替え" });
    // The only tab of a project is its tree (F10).
    expect(within(tabs).getAllByRole("link")).toHaveLength(1);
    expect(within(tabs).getByRole("link", { name: "ツリー" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });
```

変更後:
```ts
  it("leaves the heading, the breadcrumb and the tabs of a project to its page", () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
    expect(screen.queryByRole("navigation", { name: "パンくず" })).toBeNull();
    expect(screen.queryByRole("navigation", { name: "ビューの切り替え" })).toBeNull();
    expect(screen.getByRole("tree")).toBeInTheDocument();
  });
```

`within` が使われなくなるので、import から外す（`import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";`）。

Run: `cd web && pnpm fix && pnpm vitest run app/tree`
Expected: PASS。ほかのテスト（Task 3〜6 のテスト）が、`core` を差し替えていて誤りのあるクエリを描くとき `c.stores.stores is not a function` で落ちたら、そのテストの core の `stores` に `stores: () => []` を足す

- [ ] **Step 14: ワークスペースのタブとページ**

`web/app/workspace/WorkspaceHeader.test.ts`・`WorkspacePage.test.ts` の差分（`@@` の行は目安）:

```diff
--- a/web/app/workspace/WorkspaceHeader.test.ts
+++ b/web/app/workspace/WorkspaceHeader.test.ts
@@ -27,17 +27,46 @@
     expect(getByText("shared")).toBeTruthy();
   });
 
-  it("タブは概要・ツリーと右端の設定。今のタブだけ aria-current", () => {
+  it("タブは概要・ツリー・カンバン・テーブルと右端の設定。今のタブだけ aria-current", () => {
     const { getByRole } = render(WorkspaceHeader, { entry, tab: "tree" });
     const nav = getByRole("navigation", { name: "ビューの切り替え" });
     const links = within(nav).getAllByRole("link");
-    expect(links.map((a) => a.textContent?.trim())).toEqual(["概要", "ツリー", "設定"]);
+    expect(links.map((a) => a.textContent?.trim())).toEqual([
+      "概要",
+      "ツリー",
+      "カンバン",
+      "テーブル",
+      "設定",
+    ]);
     expect(links.map((a) => a.getAttribute("href"))).toEqual([
       "/workspaces/shared/W1",
       "/workspaces/shared/W1/tree",
+      "/workspaces/shared/W1/kanban",
+      "/workspaces/shared/W1/table",
       "/workspaces/shared/W1/settings",
     ]);
-    expect(links.map((a) => a.getAttribute("aria-current"))).toEqual([null, "page", null]);
+    expect(links.map((a) => a.getAttribute("aria-current"))).toEqual([
+      null,
+      "page",
+      null,
+      null,
+      null,
+    ]);
+  });
+
+  it.each([
+    ["kanban", "カンバン"],
+    ["table", "テーブル"],
+  ] as const)("%s のタブのとき「%s」が今のタブ", (tab, label) => {
+    const { getByRole } = render(WorkspaceHeader, { entry, tab });
+    expect(getByRole("link", { name: label }).getAttribute("aria-current")).toBe("page");
+  });
+
+  it("ガント・バックログなど後のフェーズのタブは出さない", () => {
+    const { queryByRole } = render(WorkspaceHeader, { entry, tab: "overview" });
+    for (const name of ["ガント", "バックログ", "スプリント", "リリース", "レポート"]) {
+      expect(queryByRole("link", { name })).toBeNull();
+    }
   });
 
   it("設定のタブのとき設定が今のタブ", () => {
```

```diff
--- a/web/app/workspace/WorkspacePage.test.ts
+++ b/web/app/workspace/WorkspacePage.test.ts
@@ -14,6 +14,20 @@
 }));
 vi.mock("./WorkspaceOverview.svelte", () => ({ default: marker("overview") }));
 vi.mock("./WorkspaceSettings.svelte", () => ({ default: marker("settings") }));
+vi.mock("../list/TreeTab.svelte", () => ({
+  default: (anchor: Comment, props: { view: { ulid: string } }) => {
+    const p = document.createElement("p");
+    p.textContent = `tree tab of ${props.view.ulid}`;
+    anchor.before(p);
+  },
+}));
+vi.mock("../list/ListTab.svelte", () => ({
+  default: (anchor: Comment, props: { layout: string; view: { kind: string; ulid: string } }) => {
+    const p = document.createElement("p");
+    p.textContent = `${props.layout} tab of ${props.view.kind} ${props.view.ulid}`;
+    anchor.before(p);
+  },
+}));
 
 const entry: WorkspaceEntry = {
   store: "shared",
@@ -28,7 +42,7 @@
 };
 
 function mount(
-  tab: "overview" | "tree" | "settings",
+  tab: "overview" | "tree" | "kanban" | "table" | "settings",
   over: { loaded?: boolean; found?: boolean } = {},
 ) {
   const loaded = over.loaded ?? true;
@@ -62,6 +76,23 @@
     expect(getByText("settings of release")).toBeTruthy();
   });
 
+  it("ツリーのタブは見出しの下にツリーを出す", () => {
+    const { getByRole, getByText } = mount("tree");
+    expect(getByRole("heading", { level: 1, name: "release" })).toBeTruthy();
+    expect(getByText("tree tab of W1")).toBeTruthy();
+  });
+
+  it.each(["kanban", "table"] as const)("%s のタブは見出しの下にそのレイアウトを出す", (tab) => {
+    const { getByRole, getByText, queryByText } = mount(tab);
+    expect(getByRole("heading", { level: 1, name: "release" })).toBeTruthy();
+    expect(getByText(`${tab} tab of workspace W1`)).toBeTruthy();
+    expect(queryByText("tree tab of W1")).toBeNull();
+    expect(
+      getByRole("navigation", { name: "ビューの切り替え" }).querySelector("[aria-current]")
+        ?.textContent,
+    ).toContain(tab === "kanban" ? "カンバン" : "テーブル");
+  });
+
   it("概要のタブは見出しごと WorkspaceOverview に任せる", () => {
     const { getByText, queryByRole } = mount("overview");
     expect(getByText("overview of release")).toBeTruthy();
```

Run: `cd web && pnpm vitest run app/workspace/WorkspaceHeader.test.ts app/workspace/WorkspacePage.test.ts`
Expected: FAIL（タブが 3 つしかない、`tree tab of W1` が見つからない）

`web/app/workspace/WorkspaceHeader.svelte`・`WorkspacePage.svelte` の差分:

```diff
--- a/web/app/workspace/WorkspaceHeader.svelte
+++ b/web/app/workspace/WorkspaceHeader.svelte
@@ -11,10 +11,11 @@
 
   const at = (t: WorkspaceTab) =>
     router.href({ name: "workspace", store: entry.store, ulid: entry.ulid, tab: t });
-  // 4e-3 puts the kanban and the table between the tree and the settings.
   const items = $derived([
     { label: "概要", href: at("overview"), current: tab === "overview" },
     { label: "ツリー", href: at("tree"), current: tab === "tree" },
+    { label: "カンバン", href: at("kanban"), current: tab === "kanban" },
+    { label: "テーブル", href: at("table"), current: tab === "table" },
   ]);
   const end = $derived({
     label: "設定",
```

```diff
--- a/web/app/workspace/WorkspacePage.svelte
+++ b/web/app/workspace/WorkspacePage.svelte
@@ -1,9 +1,10 @@
 <script lang="ts">
   import { core } from "../context";
+  import ListTab from "../list/ListTab.svelte";
+  import TreeTab from "../list/TreeTab.svelte";
   import { router, type WorkspaceTab } from "../router";
   import Empty from "../states/Empty.svelte";
   import { useSubscribe } from "../svelte-store";
-  import TreeView from "../tree/TreeView.svelte";
   import WorkspaceHeader from "./WorkspaceHeader.svelte";
   import WorkspaceOverview from "./WorkspaceOverview.svelte";
   import WorkspaceSettings from "./WorkspaceSettings.svelte";
@@ -30,7 +31,9 @@
       <WorkspaceHeader {entry} {tab} />
       <div class="body">
         {#if tab === "tree"}
-          <TreeView view={{ kind: "workspace", store, ulid }} />
+          <TreeTab view={{ kind: "workspace", store, ulid }} />
+        {:else if tab === "kanban" || tab === "table"}
+          <ListTab view={{ kind: "workspace", store, ulid }} layout={tab} />
         {:else}
           <WorkspaceSettings {entry} />
         {/if}
```

Run: `cd web && pnpm fix && pnpm check && pnpm vitest run app/workspace app/project app/list app/tree`
Expected: PASS

- [ ] **Step 15: コミットする**

```bash
git add web/app/project web/app/list/TreeTab.svelte web/app/list/TreeTab.test.ts web/app/tree/TreeView.svelte web/app/tree/TreeView.test.ts web/app/workspace
git commit -m "feat(web): ワークスペースとプロジェクトにカンバン・テーブルのタブを足し、ツリーの「ビューを保存」をつなぐ

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 16: サイドバーのマイタスクと保存済みビュー**

`web/app/sidebar/Sidebar.test.ts` の差分（「4e の入口は出さない」のテストを、通知とアクティビティだけに直す）:

```diff
--- a/web/app/sidebar/Sidebar.test.ts
+++ b/web/app/sidebar/Sidebar.test.ts
@@ -1,6 +1,8 @@
-import { cleanup, fireEvent } from "@testing-library/svelte";
+import { cleanup, fireEvent, within } from "@testing-library/svelte";
 import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
+import { ApiError } from "../../core/api/client";
 import { closeCreate, createState } from "../create-state.svelte";
+import { MY_QUERY } from "../my/groups";
 import { closePalette, palette } from "../palette-state.svelte";
 import { renderWithCore } from "../test-utils";
 import { openWorkspaceDelete } from "../workspace/dialogs-state.svelte";
@@ -32,13 +34,31 @@
   },
 ];
 
-function mount() {
+function mount(over: Record<string, unknown> = {}) {
   return renderWithCore(
     Sidebar,
     {},
-    { stores: { workspaces: () => workspaces, subscribe: () => () => {} } },
+    { stores: { workspaces: () => workspaces, subscribe: () => () => {} }, ...over },
   );
 }
+
+// A cache whose query for the tasks of the person finds these tasks, or does not run.
+function myCache(found: { ulids?: string[]; loading?: boolean; error?: unknown }) {
+  const watchList = vi.fn((_q: { q: string; tree: boolean }) => ({
+    ulids: () => found.ulids ?? [],
+    loading: () => found.loading ?? false,
+    error: () => found.error ?? null,
+    dispose: vi.fn(),
+  }));
+  return { watchList, cache: { subscribe: () => () => {}, watchList } };
+}
+
+const savedViews = (...entries: [store: string, tone: string, name: string][]) => ({
+  list: () => entries.map(([store, tone, name]) => ({ store, tone, view: { name } })),
+  get: () => undefined,
+  loaded: () => true,
+  subscribe: () => () => {},
+});
 
 beforeEach(() => {
   vi.clearAllMocks();
@@ -58,11 +78,108 @@
     expect(getByText(location.host)).toBeTruthy();
   });
 
-  it("4e の入口（マイタスク・通知・アクティビティ・保存済みビュー）は出さない", () => {
+  it("通知とアクティビティの入口は 4e-4 まで出さない", () => {
     const { queryByText } = mount();
-    for (const t of ["マイタスク", "通知", "アクティビティ", "保存済みビュー"]) {
+    for (const t of ["通知", "アクティビティ"]) {
       expect(queryByText(t)).toBeNull();
     }
+  });
+
+  it("マイタスクは /my を開く行。今の画面なら aria-current", async () => {
+    const { getByRole } = mount();
+    const link = getByRole("link", { name: /マイタスク/ });
+    expect(link.getAttribute("href")).toBe("/my");
+    expect(link.getAttribute("aria-current")).toBeNull();
+    await fireEvent.click(link);
+    expect(location.pathname).toBe("/my");
+    cleanup();
+    const again = mount();
+    expect(again.getByRole("link", { name: /マイタスク/ }).getAttribute("aria-current")).toBe(
+      "page",
+    );
+  });
+
+  it("マイタスクの件数は、マイタスクの画面の既定のクエリを全 Store に問い合わせた数", () => {
+    const { watchList, cache } = myCache({ ulids: ["A", "B", "C"] });
+    const { getByRole } = mount({ cache });
+    expect(watchList).toHaveBeenCalledWith({ q: MY_QUERY, tree: false });
+    expect(getByRole("link", { name: /マイタスク/ })).toHaveTextContent("マイタスク 3");
+  });
+
+  it("読み込み中と、クエリが走らないとき（アカウントがない）は件数を出さない", () => {
+    const loading = mount({ cache: myCache({ loading: true, ulids: ["A"] }).cache });
+    expect(loading.getByRole("link", { name: /マイタスク/ })).toHaveTextContent(/^マイタスク$/);
+    cleanup();
+    const failed = mount({
+      cache: myCache({ error: new ApiError({ status: 400, type: "account.none" }) }).cache,
+    });
+    expect(failed.getByRole("link", { name: /マイタスク/ })).toHaveTextContent(/^マイタスク$/);
+  });
+
+  it("保存済みビューがなければ、その見出しも出さない", () => {
+    const { queryByRole, queryByText } = mount();
+    expect(queryByRole("region", { name: "保存済みビュー" })).toBeNull();
+    expect(queryByText("保存済みビュー")).toBeNull();
+  });
+
+  it("保存済みビューを並べ、/views/<Store>/<名前> を開く。ツールチップは保存先の Store", async () => {
+    const views = savedViews(
+      ["shared", "purple", "今週のリリース作業"],
+      ["backend", "orange", "ブロック中"],
+      ["org-b/ops", "teal", "a/b を含む名前"],
+    );
+    const { getByRole } = mount({ views });
+    const region = getByRole("region", { name: "保存済みビュー" });
+    const links = within(region).getAllByRole("link");
+    expect(links.map((a) => a.textContent?.trim())).toEqual([
+      "今週のリリース作業",
+      "ブロック中",
+      "a/b を含む名前",
+    ]);
+    expect(links.map((a) => a.getAttribute("href"))).toEqual([
+      "/views/shared/%E4%BB%8A%E9%80%B1%E3%81%AE%E3%83%AA%E3%83%AA%E3%83%BC%E3%82%B9%E4%BD%9C%E6%A5%AD",
+      "/views/backend/%E3%83%96%E3%83%AD%E3%83%83%E3%82%AF%E4%B8%AD",
+      "/views/org-b%2Fops/a%2Fb%20%E3%82%92%E5%90%AB%E3%82%80%E5%90%8D%E5%89%8D",
+    ]);
+    expect(links.map((a) => a.getAttribute("title"))).toEqual([
+      "shared の views/ に保存",
+      "backend の views/ に保存",
+      "org-b/ops の views/ に保存",
+    ]);
+    // the colour of the store is the dot (the store's token), not a hex
+    expect(links[0]?.querySelector(".dot")?.getAttribute("style")).toBe(
+      "background: var(--store-purple-dot);",
+    );
+    await fireEvent.click(links[1] as HTMLElement);
+    expect(location.pathname).toBe("/views/backend/%E3%83%96%E3%83%AD%E3%83%83%E3%82%AF%E4%B8%AD");
+  });
+
+  it("同じ名前のビューは、Store の色の点とツールチップで見分ける", () => {
+    const views = savedViews(
+      ["shared", "purple", "ブロック中"],
+      ["backend", "orange", "ブロック中"],
+    );
+    const { getByRole } = mount({ views });
+    const links = within(getByRole("region", { name: "保存済みビュー" })).getAllByRole("link");
+    expect(links.map((a) => a.getAttribute("title"))).toEqual([
+      "shared の views/ に保存",
+      "backend の views/ に保存",
+    ]);
+    expect(links.map((a) => a.querySelector(".dot")?.getAttribute("style"))).toEqual([
+      "background: var(--store-purple-dot);",
+      "background: var(--store-orange-dot);",
+    ]);
+  });
+
+  it("開いているビューだけ aria-current にする", () => {
+    history.replaceState(null, "", "/views/backend/%E3%83%96%E3%83%AD%E3%83%83%E3%82%AF%E4%B8%AD");
+    const views = savedViews(
+      ["shared", "purple", "ブロック中"],
+      ["backend", "orange", "ブロック中"],
+    );
+    const { getByRole } = mount({ views });
+    const links = within(getByRole("region", { name: "保存済みビュー" })).getAllByRole("link");
+    expect(links.map((a) => a.getAttribute("aria-current"))).toEqual([null, "page"]);
   });
 
   it("右上の歯車は設定（/settings）を開く", async () => {
```

Run: `cd web && pnpm vitest run app/sidebar/Sidebar.test.ts`
Expected: FAIL（`Unable to find an accessible element with the role "link" and name "/マイタスク/"`）

`web/app/sidebar/Sidebar.svelte` の差分:

```diff
--- a/web/app/sidebar/Sidebar.svelte
+++ b/web/app/sidebar/Sidebar.svelte
@@ -6,6 +6,7 @@
   import Icon from "../icons/Icon.svelte";
   import KindIcon from "../icons/KindIcon.svelte";
   import { kbdLabel, registerShortcut } from "../keys";
+  import { MY_QUERY } from "../my/groups";
   import { openPalette } from "../palette-state.svelte";
   import { hrefOf, onLinkClick, router } from "../router";
   import { useSubscribe } from "../svelte-store";
@@ -14,11 +15,28 @@
   import { openWorkspaceDelete } from "../workspace/dialogs-state.svelte";
   import SyncCard from "./SyncCard.svelte";
 
-  const { stores } = core();
+  const { stores, cache, views } = core();
   const workspaces = useSubscribe(stores, () => stores.workspaces());
+  const savedViews = useSubscribe(views, () => views.list());
+  const cacheRev = useSubscribe(cache, () => ({}));
   const route = $derived(router.route);
   // /settings names no Store: the settings go to the first one.
   const settingsHref = hrefOf({ name: "settings", page: "workspaces", store: "" });
+  const myHref = hrefOf({ name: "my" });
+
+  // The count beside "マイタスク" is the number the screen shows with its own query (細部 55). It
+  // is left out while the list is read, and when the query does not run (no account).
+  let myList = $state.raw<ReturnType<typeof cache.watchList> | null>(null);
+  $effect(() => {
+    const w = cache.watchList({ q: MY_QUERY, tree: false });
+    myList = w;
+    return () => w.dispose();
+  });
+  const myCount = $derived.by(() => {
+    cacheRev.current;
+    if (!myList || myList.loading() || myList.error()) return null;
+    return myList.ulids().length;
+  });
 
   onMount(() =>
     registerShortcut("mod+k", () => {
@@ -75,6 +93,46 @@
     <span class="ph">ID・検索・コマンド</span>
     <kbd>{kbdLabel("mod+k")}</kbd>
   </button>
+
+  <div class="places">
+    <a
+      href={myHref}
+      class="nav-link"
+      class:active={route.name === "my"}
+      aria-current={route.name === "my" ? "page" : undefined}
+      onclick={(e) => onLinkClick(e, myHref)}
+    >
+      <Icon name="checklist" size={14} />
+      <span class="nav-text">マイタスク</span>
+      {#if myCount !== null}
+        <span class="count">{myCount}</span>
+      {/if}
+    </a>
+  </div>
+
+  {#if savedViews.current.length > 0}
+    <section class="views" aria-label="保存済みビュー">
+      <div class="ws-head">
+        <span class="ws-title">保存済みビュー</span>
+      </div>
+      {#each savedViews.current as v (`${v.store}/${v.view.name}`)}
+        {@const vhref = hrefOf({ name: "view", store: v.store, view: v.view.name })}
+        {@const vhere =
+          route.name === "view" && route.store === v.store && route.view === v.view.name}
+        <a
+          href={vhref}
+          class="view"
+          class:active={vhere}
+          aria-current={vhere ? "page" : undefined}
+          title="{v.store} の views/ に保存"
+          onclick={(e) => onLinkClick(e, vhref)}
+        >
+          <span class="dot" style:background="var(--store-{v.tone}-dot)" aria-hidden="true"></span>
+          <span class="p-name">{v.view.name}</span>
+        </a>
+      {/each}
+    </section>
+  {/if}
 
   <section class="ws-section" aria-label="ワークスペース">
     <div class="ws-head">
@@ -293,7 +351,9 @@
     background: var(--hover);
   }
   .ws-link,
-  .project {
+  .project,
+  .nav-link,
+  .view {
     display: flex;
     align-items: center;
     gap: var(--space-8);
@@ -302,17 +362,43 @@
     color: var(--ink-2);
     text-decoration: none;
   }
+  .nav-link {
+    min-height: var(--row-h);
+  }
+  .nav-text {
+    flex: 1;
+  }
+  .count {
+    color: var(--label);
+    font-family: var(--font-mono);
+    font-size: var(--fs-11);
+  }
+  .places,
+  .views {
+    display: flex;
+    flex-direction: column;
+    gap: 1px;
+  }
+  .dot {
+    flex: 0 0 auto;
+    width: 8px;
+    height: 8px;
+    border-radius: 2px;
+  }
   .ws-link {
     flex: 1;
     min-width: 0;
     min-height: var(--row-h);
     padding-right: var(--control-h-sm);
   }
-  .project {
+  .project,
+  .view {
     min-height: 30px;
   }
   .ws-link:hover,
-  .project:hover {
+  .project:hover,
+  .nav-link:hover,
+  .view:hover {
     background: var(--hover);
   }
   .active,
```

Run: `cd web && pnpm fix && pnpm vitest run app/sidebar`
Expected: PASS

- [ ] **Step 17: パレットのマイタスク・保存済みビュー・このクエリで**

`web/app/palette/search.test.ts`・`CommandPalette.test.ts` の差分:

```diff
--- a/web/app/palette/search.test.ts
+++ b/web/app/palette/search.test.ts
@@ -6,6 +6,7 @@
   buildCommands,
   type CommandEnv,
   detectMode,
+  queryCommands,
   type SearchDeps,
   searchTasks,
   taskCommands,
@@ -191,7 +192,9 @@
     createProject: vi.fn(),
     addWorkspace: vi.fn(),
     undo: vi.fn(),
+    saveQuery: vi.fn(),
     workspaces: [ws],
+    views: [],
     storeNames: ["shared"],
     go: vi.fn(),
     hrefs: {
@@ -200,6 +203,9 @@
       settingsWorkspaces: (s) => `/settings/workspaces/${s}`,
       project: (u) => `/projects/${u}`,
       task: (u) => `/tasks/${u}`,
+      view: (s, n) => `/views/${s}/${encodeURIComponent(n)}`,
+      query: (l, q) => `/query/${l}?q=${encodeURIComponent(q)}`,
+      my: () => "/my",
     },
     selected: null,
     recent: [],
@@ -233,6 +239,7 @@
       "ワークスペースの設定を開く",
       "リリース準備 v2.4 を開く",
       "設定のワークスペース一覧を開く",
+      "マイタスクを開く",
     ]);
   });
 
@@ -330,10 +337,76 @@
     ).toEqual(["設定のワークスペース一覧を開く|shared", "設定のワークスペース一覧を開く|empty"]);
   });
 
+  it("opens the tasks of the person, and each saved view with the store it is in", () => {
+    const views = [
+      { store: "shared", tone: "purple" as const, view: { name: "今週のリリース作業" } },
+      { store: "backend", tone: "orange" as const, view: { name: "ブロック中" } },
+    ] as CommandEnv["views"];
+    const e = env({ views });
+    const gs = buildCommands(e, ">");
+    const opens = gs.find((g) => g.name === "開く")?.items ?? [];
+    expect(
+      opens.slice(-3).map((i) => (i.type === "command" ? [i.label, i.store, i.href] : [])),
+    ).toEqual([
+      ["マイタスクを開く", undefined, "/my"],
+      [
+        "今週のリリース作業 を開く",
+        "shared",
+        "/views/shared/%E4%BB%8A%E9%80%B1%E3%81%AE%E3%83%AA%E3%83%AA%E3%83%BC%E3%82%B9%E4%BD%9C%E6%A5%AD",
+      ],
+      [
+        "ブロック中 を開く",
+        "backend",
+        "/views/backend/%E3%83%96%E3%83%AD%E3%83%83%E3%82%AF%E4%B8%AD",
+      ],
+    ]);
+    const view = opens.at(-2);
+    if (view?.type === "command") view.run();
+    expect(e.go).toHaveBeenCalledWith(
+      "/views/shared/%E4%BB%8A%E9%80%B1%E3%81%AE%E3%83%AA%E3%83%AA%E3%83%BC%E3%82%B9%E4%BD%9C%E6%A5%AD",
+    );
+  });
+
+  it("narrows the places to open by the word, the saved views too", () => {
+    const views = [
+      { store: "backend", tone: "orange" as const, view: { name: "ブロック中" } },
+    ] as CommandEnv["views"];
+    const gs = buildCommands(env({ views }), ">ブロック");
+    expect(gs.map((g) => g.name)).toEqual(["開く"]);
+    expect(gs[0]?.items.map((i) => (i.type === "command" ? i.label : ""))).toEqual([
+      "ブロック中 を開く",
+    ]);
+    expect(
+      buildCommands(env(), ">マイ")[0]?.items.map((i) => (i.type === "command" ? i.label : "")),
+    ).toEqual(["マイタスクを開く"]);
+  });
+
   it("adds the commands for the selected task", () => {
     const t = task("T1", "task", null, { id: "REL-6" });
     const gs = buildCommands(env({ selected: t }), ">");
     expect(gs.map((g) => g.name)).toEqual(["コマンド", "開く", "REL-6 に対して"]);
+  });
+});
+
+describe("queryCommands", () => {
+  it("opens the query as a table without saving, and saves it as a view", () => {
+    const e = env();
+    const g = queryCommands(e, "status:doing assignee:@me");
+    expect(g.name).toBe("このクエリで");
+    expect(g.items.map((i) => (i.type === "command" ? i.label : ""))).toEqual([
+      "テーブルで開く",
+      "ビューとして保存…",
+    ]);
+    const [open, save] = g.items;
+    expect(open).toMatchObject({
+      href: "/query/table?q=status%3Adoing%20assignee%3A%40me",
+    });
+    if (open?.type === "command") open.run();
+    expect(e.go).toHaveBeenCalledWith("/query/table?q=status%3Adoing%20assignee%3A%40me");
+    if (save?.type === "command") save.run();
+    expect(e.saveQuery).toHaveBeenCalledWith("status:doing assignee:@me");
+    // a command of the query is not one of the bulk update, which is for phase 6
+    expect(g.items.some((i) => i.type === "command" && /まとめて更新/.test(i.label))).toBe(false);
   });
 });
 
```

```diff
--- a/web/app/palette/CommandPalette.test.ts
+++ b/web/app/palette/CommandPalette.test.ts
@@ -13,6 +13,7 @@
   runUndo: vi.fn(async () => {}),
   addWorkspace: vi.fn(),
   moveWorkspace: vi.fn(),
+  saveView: vi.fn(),
 }));
 
 vi.mock("../context", () => ({ core: () => h.core }));
@@ -22,16 +23,31 @@
       return h.route;
     },
     go: h.go,
-    href: (r: { name: string; ulid?: string; store?: string; tab?: string }) =>
+    href: (r: {
+      name: string;
+      ulid?: string;
+      store?: string;
+      tab?: string;
+      view?: string;
+      layout?: string;
+      q?: string;
+    }) =>
       r.name === "task"
         ? `/tasks/${r.ulid}`
         : r.name === "project"
           ? `/projects/${r.ulid}`
           : r.name === "settings"
             ? `/settings/workspaces/${r.store}`
-            : `/workspaces/${r.store}/${r.ulid}${r.tab === "overview" ? "" : `/${r.tab}`}`,
+            : r.name === "view"
+              ? `/views/${r.store}/${r.view}`
+              : r.name === "query"
+                ? `/query/${r.layout}?q=${r.q}`
+                : r.name === "my"
+                  ? "/my"
+                  : `/workspaces/${r.store}/${r.ulid}${r.tab === "overview" ? "" : `/${r.tab}`}`,
   },
 }));
+vi.mock("../views/dialogs-state.svelte", () => ({ openViewSave: h.saveView }));
 vi.mock("../toast.svelte", () => ({ toast: h.toast }));
 vi.mock("../undo", async (orig) => ({
   ...(await orig<typeof import("../undo")>()),
@@ -50,7 +66,7 @@
   title: "Web の API クライアントを v2 に更新",
 });
 
-function setup() {
+function setup(found: unknown[] = []) {
   const none = () => () => {};
   const ws = {
     store: "shared",
@@ -86,7 +102,7 @@
             response: new Response(null, { status: 400 }),
           };
         }
-        return ok({ tasks: [] });
+        return ok({ tasks: found });
       },
     ),
     POST: vi.fn(async () => ok([])),
@@ -109,6 +125,10 @@
       subscribe: none,
     },
     schema: { get: () => schema, load: async () => schema, subscribe: none },
+    views: {
+      list: () => [{ store: "backend", tone: "orange", view: { name: "ブロック中" } }],
+      subscribe: none,
+    },
   };
   return { api };
 }
@@ -121,6 +141,7 @@
   h.runUndo.mockClear();
   h.addWorkspace.mockClear();
   h.moveWorkspace.mockClear();
+  h.saveView.mockClear();
   localStorage.clear();
   closePalette();
   closeCreate();
@@ -292,6 +313,59 @@
     await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "foo:bar" } });
     expect(await screen.findByRole("alert")).toHaveTextContent("term 1 (foo:bar): unknown key");
     expect(screen.getByText("クエリ", { selector: "[data-mode]" })).toBeInTheDocument();
+  });
+
+  it("opens the tasks of the person and a saved view from the commands", async () => {
+    setup();
+    render(CommandPalette);
+    openPalette();
+    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">開く" } });
+    await fireEvent.click(await screen.findByRole("option", { name: /マイタスクを開く/ }));
+    expect(h.go).toHaveBeenCalledWith("/my");
+    openPalette();
+    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">ブロック" } });
+    await fireEvent.click(await screen.findByRole("option", { name: /ブロック中 を開く/ }));
+    expect(h.go).toHaveBeenLastCalledWith("/views/backend/ブロック中");
+  });
+
+  it("offers to open a query that found tasks as a table, and to save it as a view", async () => {
+    setup([rel6]);
+    render(CommandPalette);
+    openPalette();
+    await fireEvent.input(await screen.findByRole("combobox"), {
+      target: { value: "status:doing assignee:@me" },
+    });
+    expect(await screen.findByRole("group", { name: "このクエリで" })).toBeInTheDocument();
+    await fireEvent.click(screen.getByRole("option", { name: /ビューとして保存/ }));
+    expect(h.saveView).toHaveBeenCalledWith({
+      draft: {
+        layout: "table",
+        query: "status:doing assignee:@me",
+        kanban: { columns: "status", swimlane: "none" },
+        table: { columns: ["id", "title", "status", "assignee", "due"], groupBy: "none" },
+      },
+    });
+    expect(palette.open).toBe(false);
+    openPalette();
+    await fireEvent.input(await screen.findByRole("combobox"), {
+      target: { value: "status:doing assignee:@me" },
+    });
+    await fireEvent.click(await screen.findByRole("option", { name: /テーブルで開く/ }));
+    expect(h.go).toHaveBeenCalledWith("/query/table?q=status:doing assignee:@me");
+  });
+
+  it("offers nothing for a query that found nothing or does not run", async () => {
+    setup();
+    render(CommandPalette);
+    openPalette();
+    await fireEvent.input(await screen.findByRole("combobox"), {
+      target: { value: "status:doing" },
+    });
+    expect(await screen.findByText("一致するものがありません")).toBeInTheDocument();
+    expect(screen.queryByRole("group", { name: "このクエリで" })).toBeNull();
+    await fireEvent.input(box(), { target: { value: "foo:bar" } });
+    expect(await screen.findByRole("alert")).toBeInTheDocument();
+    expect(screen.queryByRole("group", { name: "このクエリで" })).toBeNull();
   });
 
   it("offers the commands for the selected task", async () => {
```

Run: `cd web && pnpm vitest run app/palette`
Expected: FAIL（`queryCommands is not a function` など）

`web/app/palette/search.ts`・`CommandPalette.svelte` の差分:

```diff
--- a/web/app/palette/search.ts
+++ b/web/app/palette/search.ts
@@ -1,7 +1,9 @@
 import { type Api, type TaskView, unwrap } from "../../core/api/client";
 import { searchTasks as findTasks } from "../../core/search";
 import type { WorkspaceEntry } from "../../core/stores";
+import type { ViewEntry } from "../../core/views";
 import { kbdLabel } from "../keys";
+import type { Layout } from "../router";
 
 export type PaletteMode = "id" | "query" | "command";
 
@@ -37,7 +39,16 @@
   type: "command";
   key: string;
   label: string;
-  icon: "sync" | "plus" | "status" | "folder" | "workspace" | "settings" | "undo";
+  icon:
+    | "sync"
+    | "plus"
+    | "status"
+    | "folder"
+    | "workspace"
+    | "settings"
+    | "undo"
+    | "checklist"
+    | "filter";
   kbd?: string;
   note?: string;
   store?: string;
@@ -141,7 +152,9 @@
   createProject: () => void;
   addWorkspace: () => void;
   undo: () => void;
+  saveQuery: (q: string) => void; // opens the dialog that saves the query as a view
   workspaces: WorkspaceEntry[];
+  views: ViewEntry[]; // the saved views of every store
   storeNames: string[]; // every mounted store, also one with no workspace
   go: (path: string) => void;
   hrefs: {
@@ -150,6 +163,9 @@
     settingsWorkspaces: (store: string) => string;
     project: (ulid: string) => string;
     task: (ulid: string) => string;
+    view: (store: string, name: string) => string;
+    query: (layout: Layout, q: string) => string;
+    my: () => string;
   };
   selected: TaskView | null; // the task the commands for a task are about
   recent: TaskView[]; // the tasks opened last, the latest first
@@ -205,6 +221,33 @@
   };
 }
 
+// queryCommands are what the palette offers for the query that was typed: the screen of the
+// query as a table (nothing is saved), and the dialog that saves it as a view. The bulk update
+// of the matches the design shows is not made (phase 6).
+export function queryCommands(env: CommandEnv, q: string): PaletteGroup {
+  const href = env.hrefs.query("table", q);
+  return {
+    name: "このクエリで",
+    items: [
+      {
+        type: "command",
+        key: "query-table",
+        label: "テーブルで開く",
+        icon: "filter",
+        href,
+        run: () => env.go(href),
+      },
+      {
+        type: "command",
+        key: "query-save",
+        label: "ビューとして保存…",
+        icon: "filter",
+        run: () => env.saveQuery(q),
+      },
+    ],
+  };
+}
+
 // buildCommands lists the commands, then where to go, then the commands for
 // the selected task. text is the input, with or without the leading >; a word
 // in it narrows the list by its label and note.
@@ -252,6 +295,29 @@
       run: env.undo,
     },
   ];
+
+  const myHref = env.hrefs.my();
+  const my: CommandItem = {
+    type: "command",
+    key: "open:my",
+    label: "マイタスクを開く",
+    icon: "checklist",
+    href: myHref,
+    run: () => env.go(myHref),
+  };
+  const saved: CommandItem[] = env.views.map((v) => {
+    const href = env.hrefs.view(v.store, v.view.name);
+    return {
+      type: "command",
+      key: `open:view:${v.store}:${v.view.name}`,
+      label: `${v.view.name} を開く`,
+      icon: "filter",
+      note: "保存済みビュー",
+      store: v.store,
+      href,
+      run: () => env.go(href),
+    };
+  });
 
   const places: CommandItem[] = env.workspaces.flatMap((w) => {
     const wsHref = env.hrefs.workspace(w.store, w.ulid);
@@ -312,7 +378,7 @@
   const groups: PaletteGroup[] = [
     { name: "コマンド", items: commands.filter(keep) },
     { name: "最近開いたもの", items: recent },
-    { name: "開く", items: [...places, ...lists].filter(keep) },
+    { name: "開く", items: [...places, ...lists, my, ...saved].filter(keep) },
   ];
   if (env.selected) {
     const g = taskCommands(env, env.selected);
```

```diff
--- a/web/app/palette/CommandPalette.svelte
+++ b/web/app/palette/CommandPalette.svelte
@@ -16,6 +16,8 @@
   import { patchTask } from "../tree/actions";
   import StoreBadge from "../ui/StoreBadge.svelte";
   import { routeStore, runUndo } from "../undo";
+  import { openViewSave } from "../views/dialogs-state.svelte";
+  import { newDraft } from "../views/draft";
   import { openWorkspaceAdd, openWorkspaceMove } from "../workspace/dialogs-state.svelte";
   import { fetchMissing, readRecent, rememberTask } from "./recent";
   import {
@@ -24,6 +26,7 @@
     detectMode,
     type PaletteGroup,
     type PaletteItem,
+    queryCommands,
     searchTasks,
     taskCommands,
   } from "./search";
@@ -32,6 +35,7 @@
   const cacheRev = useSubscribe(c.cache, () => ({}));
   const storesRev = useSubscribe(c.stores, () => ({}));
   const schemaRev = useSubscribe(c.schema, () => ({}));
+  const viewsRev = useSubscribe(c.views, () => ({}));
 
   let input = $state("");
   let active = $state(0);
@@ -132,12 +136,16 @@
   const env = $derived.by<CommandEnv>(() => {
     cacheRev.current;
     storesRev.current;
+    viewsRev.current;
     const sel = router.route.name === "task" ? router.route.ulid : selection.ulid;
     return {
       syncNow: () => void runSync(c.api),
       createTask: (parent) => openCreate({ kind: "task", parent }),
       createProject: () => openCreate({ kind: "project" }),
       addWorkspace: () => openWorkspaceAdd(),
+      // A query typed in the palette is saved as a table, which holds any query (a tree cannot sort).
+      saveQuery: (query) => openViewSave({ draft: newDraft({ layout: "table", query }) }),
+      views: c.views.list(),
       undo: () =>
         void runUndo(
           c,
@@ -154,6 +162,9 @@
         settingsWorkspaces: (store) => router.href({ name: "settings", page: "workspaces", store }),
         project: (ulid) => router.href({ name: "project", ulid, tab: "tree" }),
         task: (ulid) => router.href({ name: "task", ulid }),
+        view: (store, view) => router.href({ name: "view", store, view }),
+        query: (layout, q) => router.href({ name: "query", layout, q }),
+        my: () => router.href({ name: "my" }),
       },
       selected: sel ? (c.cache.task(sel) ?? null) : null,
       recent: recentUlids.flatMap((u) => c.cache.task(u) ?? []),
@@ -188,6 +199,10 @@
     if (picking) return statusGroups(picking);
     if (mode === "command" || input.trim() === "") return buildCommands(env, input);
     const out = [...found.groups];
+    // A query that found tasks can be opened as a table or saved as a view.
+    if (mode === "query" && !found.error && found.groups.length > 0) {
+      out.push(queryCommands(env, input.trim()));
+    }
     const first = found.groups.flatMap((g) => g.items).find((i) => i.type === "task");
     if (mode === "id" && first?.type === "task") {
       const t = c.cache.task(first.ulid);
```

Run: `cd web && pnpm fix && pnpm check && pnpm vitest run app/palette`
Expected: PASS

- [ ] **Step 18: undo の Store と App の振り分け**

`web/app/undo.test.ts`・`App.test.ts` の差分:

```diff
--- a/web/app/undo.test.ts
+++ b/web/app/undo.test.ts
@@ -69,6 +69,14 @@
     expect(routeStore({ name: "task", ulid: "T1" }, taskOf)).toBe("backend");
   });
 
+  it("takes the store a saved view is saved in, none for the screens of every store", () => {
+    expect(routeStore({ name: "view", store: "org-b/ops", view: "weekly" }, taskOf)).toBe(
+      "org-b/ops",
+    );
+    expect(routeStore({ name: "query", layout: "table", q: "kind:bug" }, taskOf)).toBeUndefined();
+    expect(routeStore({ name: "my" }, taskOf)).toBeUndefined();
+  });
+
   it("takes the store of the first workspace at home, which shows its overview", () => {
     expect(routeStore({ name: "home" }, taskOf, { store: "shared" })).toBe("shared");
     expect(routeStore({ name: "home" }, taskOf)).toBeUndefined();
```

```diff
--- a/web/app/App.test.ts
+++ b/web/app/App.test.ts
@@ -2,6 +2,30 @@
 import { flushSync } from "svelte";
 import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
 import App from "./App.svelte";
+
+// The screens of the new routes have their own tests: here each only says which one is drawn,
+// and with what.
+vi.mock("./views/ViewPage.svelte", () => ({
+  default: (anchor: Comment, props: { store: string; name: string }) => {
+    const p = document.createElement("p");
+    p.textContent = `view page ${props.store}/${props.name}`;
+    anchor.before(p);
+  },
+}));
+vi.mock("./views/QueryPage.svelte", () => ({
+  default: (anchor: Comment, props: { layout: string; q: string }) => {
+    const p = document.createElement("p");
+    p.textContent = `query page ${props.layout} ${props.q}`;
+    anchor.before(p);
+  },
+}));
+vi.mock("./list/ListTab.svelte", () => ({
+  default: (anchor: Comment, props: { layout: string; view: { ulid: string } }) => {
+    const p = document.createElement("p");
+    p.textContent = `${props.layout} tab of ${props.view.ulid}`;
+    anchor.before(p);
+  },
+}));
 
 beforeEach(() => {
   vi.stubGlobal("fetch", () => new Promise(() => {}));
@@ -32,6 +56,43 @@
     expect(location.pathname).toBe("/settings/workspaces/shared");
   });
 
+  it("/my はマイタスクの画面", () => {
+    history.replaceState(null, "", "/my#token=tkn");
+    const { getByRole } = render(App);
+    flushSync();
+    expect(getByRole("heading", { level: 1, name: "マイタスク" })).toBeTruthy();
+    expect(location.pathname).toBe("/my");
+  });
+
+  it("/views/<Store>/<名前> はビューの画面。名前は URL から戻して渡す", () => {
+    history.replaceState(null, "", "/views/org-b%2Fops/%E4%BB%8A%E9%80%B1#token=tkn");
+    const { getByText } = render(App);
+    flushSync();
+    expect(getByText("view page org-b/ops/今週")).toBeTruthy();
+  });
+
+  it("/query/<レイアウト>?q= はクエリの画面。q を残す", () => {
+    history.replaceState(null, "", "/query/table?q=status%3Adoing%20assignee%3A%40me#token=tkn");
+    const { getByText } = render(App);
+    flushSync();
+    expect(getByText("query page table status:doing assignee:@me")).toBeTruthy();
+    expect(location.pathname + location.search).toBe(
+      "/query/table?q=status%3Adoing%20assignee%3A%40me",
+    );
+  });
+
+  it("プロジェクトのカンバン・テーブルのタブは、そのレイアウトの画面", () => {
+    history.replaceState(null, "", "/projects/01HPR/kanban#token=tkn");
+    const first = render(App);
+    flushSync();
+    expect(first.getByText("kanban tab of 01HPR")).toBeTruthy();
+    first.unmount();
+    history.replaceState(null, "", "/projects/01HPR/table#token=tkn");
+    const second = render(App);
+    flushSync();
+    expect(second.getByText("table tab of 01HPR")).toBeTruthy();
+  });
+
   it("設定以外の画面ではサイドバーを出す", () => {
     history.replaceState(null, "", "/tasks/01HTK#token=tkn");
     const { container } = render(App);
```

Run: `cd web && pnpm vitest run app/undo.test.ts app/App.test.ts`
Expected: FAIL

`web/app/undo.ts`・`App.svelte` の差分:

```diff
--- a/web/app/undo.ts
+++ b/web/app/undo.ts
@@ -20,9 +20,10 @@
     : `${what}を取り消しました`;
 }
 
-// routeStore is the store the screen is about: the store of a workspace, of the settings page, or
-// of the project or task it shows (the cache may not have it yet). Home shows the overview of the
-// first workspace, so it is that one's store. Elsewhere there is none.
+// routeStore is the store the screen is about: the store of a workspace, of the settings page, of a
+// saved view, or of the project or task it shows (the cache may not have it yet). Home shows the
+// overview of the first workspace, so it is that one's store. Elsewhere there is none (the query
+// screen and the tasks of the person list every store).
 export function routeStore(
   route: Route,
   taskOf: (ulid: string) => { store: string } | undefined,
@@ -30,6 +31,7 @@
 ): string | undefined {
   if (route.name === "workspace") return route.store;
   if (route.name === "settings") return route.store || undefined;
+  if (route.name === "view") return route.store;
   if (route.name === "home") return first?.store;
   if (route.name === "project" || route.name === "task") return taskOf(route.ulid)?.store;
   return undefined;
```

```diff
--- a/web/app/App.svelte
+++ b/web/app/App.svelte
@@ -6,7 +6,9 @@
   import { createCore } from "./core-factory";
   import CreateHost from "./create/CreateHost.svelte";
   import TaskDetail from "./detail/TaskDetail.svelte";
+  import MyTasks from "./my/MyTasks.svelte";
   import CommandPalette from "./palette/CommandPalette.svelte";
+  import ProjectPage from "./project/ProjectPage.svelte";
   import { router } from "./router";
   import SettingsRoute from "./settings/SettingsRoute.svelte";
   import Sidebar from "./sidebar/Sidebar.svelte";
@@ -16,8 +18,10 @@
   import NoToken from "./states/NoToken.svelte";
   import Onboarding from "./states/Onboarding.svelte";
   import { useSubscribe } from "./svelte-store";
-  import TreeView from "./tree/TreeView.svelte";
   import Toasts from "./ui/Toasts.svelte";
+  import QueryPage from "./views/QueryPage.svelte";
+  import ViewDialogs from "./views/ViewDialogs.svelte";
+  import ViewPage from "./views/ViewPage.svelte";
   import WorkspaceDialogs from "./workspace/WorkspaceDialogs.svelte";
   import WorkspacePage from "./workspace/WorkspacePage.svelte";
 
@@ -53,7 +57,8 @@
   });
 
   onMount(() => {
-    router.go(location.pathname, { replace: true });
+    // The address is made ours (an unknown path goes to "/"); the query screen keeps its ?q=.
+    router.go(location.pathname + location.search, { replace: true });
     if (token) c.conn.start();
     return () => c.conn.stop();
   });
@@ -73,12 +78,20 @@
         {/key}
       {:else if route.name === "project"}
         {#key route.ulid}
-          <TreeView view={{ kind: "project", ulid: route.ulid }} />
+          <ProjectPage ulid={route.ulid} tab={route.tab} />
         {/key}
       {:else if route.name === "task"}
         {#key route.ulid}
           <TaskDetail ulid={route.ulid} />
         {/key}
+      {:else if route.name === "view"}
+        {#key `${route.store}/${route.view}`}
+          <ViewPage store={route.store} name={route.view} />
+        {/key}
+      {:else if route.name === "query"}
+        <QueryPage layout={route.layout} q={route.q} />
+      {:else if route.name === "my"}
+        <MyTasks />
       {:else if route.name === "settings"}
         <SettingsRoute page={route} />
       {:else if route.name === "home" && dir.current.first}
@@ -116,6 +129,7 @@
 {/if}
 <CreateHost />
 <WorkspaceDialogs />
+<ViewDialogs />
 <CommandPalette />
 <Toasts />
 
```

Run: `cd web && pnpm fix && pnpm check && pnpm exec biome ci --vcs-enabled=false . && pnpm vitest run`
Expected: 警告 0、指摘 0、すべて PASS。全件が混んで落ちたら（上の「確かめたこと」の 2 件のような時間切れ）、落ちたテストの出力と実行条件を残し、そのファイルだけをもう一度流して通ることを確かめる

- [ ] **Step 19: コミットする**

```bash
git add web/app
git commit -m "feat(web): サイドバー・パレット・App にマイタスクと保存済みビューとクエリの画面をつなぐ

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 20: E2E を足す**

目印は `web/e2e/screens.ts` の冒頭に書いた。カンバンの盤面・セル・カード・必須の項目のダイアログ（Task 4）、テーブルのセルの名前（Task 5）、ビューの画面・ダイアログの名前（Task 6）は、それぞれの Task の plan の「E2E が使う目印」と同じ言葉を使っている。実物の言葉が違っていたら、spec と `screens.ts` を実物に合わせて直す（画面の言葉は変えない）。

`web/e2e/screens.ts` と `routes.spec.ts` の差分:

```diff
--- a/web/e2e/screens.ts
+++ b/web/e2e/screens.ts
@@ -25,6 +25,19 @@
 //               タスク" (name them exactly: the second name starts with the first), whose rows are
 //               named by the ID and the title; the buttons "タスクを作成" and "プロジェクトを作成"
 //   Undo        the palette command "直前の操作を取り消す"; the toast says "<ID> の変更を取り消しました"
+//   Kanban      the board is the section "カンバンの盤面": a list for each column, ul[data-column=
+//               "<status or category>"], whose cards are li[data-ulid="<ULID>"]; a card is dragged
+//               onto a list (dragCard); a status that needs a field opens the dialog
+//               "<ID> を<ステータス>にするには <項目> が必要です"
+//   Table       a row is a set of inputs named by the column and the ID: the textbox "タイトル: <ID>",
+//               the button "ステータス: <ID>"; the filter field is the textbox
+//               "フィルタ（hh ls と同じクエリ）" on every list screen
+//   Views       /views/<Store>/<名前> (heading <名前>, navigation "レイアウトの切り替え", the status
+//               "未保存の変更" and the button "ビューを更新", the button "ビューの操作（改名・削除）"),
+//               /query/<layout>?q= (heading "クエリ"), the sidebar region "保存済みビュー", the
+//               dialogs "ビューを保存" / "ビュー <名前> を改名" / "ビュー <名前> を削除"
+//   My tasks    /my (heading "マイタスク"); the groups are regions named "今日", "今週", ...; the sidebar
+//               link "マイタスク" carries the count
 
 //
 // Exceptions that hold on to a CSS class, because no role or text tells them apart: change them
@@ -70,6 +83,11 @@
   return page.getByRole("status", { name: "保存の状態" }).filter({ hasText: /^保存済み/ });
 }
 
+/** Drags a card of the kanban onto the list of another column (the middle of the list). */
+export async function dragCard(page: Page, card: Locator, cell: Locator): Promise<void> {
+  await dragRow(page, card, cell, "child");
+}
+
 /**
  * Drags the row `from` onto the row `to`: onto its upper quarter ("before"),
  * its lower quarter ("after") or its middle ("child"). `hold` runs while the
```

```diff
--- a/web/e2e/routes.spec.ts
+++ b/web/e2e/routes.spec.ts
@@ -1,8 +1,10 @@
 // Screens this spec relies on:
 //   - "/" is the overview of the first workspace: the heading (level 1) is the workspace name, and
-//     the navigation "ビューの切り替え" holds the links 概要, ツリー and 設定; the current one has
-//     aria-current="page". The tabs are links (the address, back and reload keep the tab)
-//   - the sidebar ("メインナビゲーション") has the link "設定" (the gear, /settings)
+//     the navigation "ビューの切り替え" holds the links 概要, ツリー, カンバン, テーブル and 設定; the
+//     current one has aria-current="page". The tabs are links (the address, back and reload keep the
+//     tab). A project has the same navigation with the links ツリー, カンバン and テーブル
+//   - the sidebar ("メインナビゲーション") has the link "設定" (the gear, /settings) and the link
+//     "マイタスク" (/my); it has no entry for the notifications and the activity yet
 //   - for each workspace the sidebar has the button "<名前> の操作（設定を開く・削除）" that opens
 //     the menu "ワークスペースの操作" with 設定を開く and 削除…
 //   - the settings take the window: the navigation "設定のナビゲーション" (with the link
@@ -101,3 +103,52 @@
   await expect(page).toHaveURL(/\/settings\/workspaces\/demo$/);
   await expect(page.getByRole("navigation", { name: "設定のナビゲーション" })).toBeVisible();
 });
+
+test("the kanban and the table are tabs of a workspace and of a project", async ({ page, hh }) => {
+  const project = hh.add("Release", "--kind", "project");
+  const child = hh.add("Ship it", "--parent", project.id);
+  const board = page.locator('section[aria-label="カンバンの盤面"]');
+  await hh.open(page);
+
+  await expect(tabs(page).getByRole("link")).toHaveText([
+    "概要",
+    "ツリー",
+    "カンバン",
+    "テーブル",
+    "設定",
+  ]);
+  await tabs(page).getByRole("link", { name: "カンバン" }).click();
+  await expect(page).toHaveURL(/\/workspaces\/demo\/[^/]+\/kanban$/);
+  await expect(board).toBeVisible();
+  await expect(heading(page)).toBeVisible();
+  await tabs(page).getByRole("link", { name: "テーブル" }).click();
+  await expect(page).toHaveURL(/\/workspaces\/demo\/[^/]+\/table$/);
+  await expect(page.getByRole("textbox", { name: `タイトル: ${child.id}` })).toBeVisible();
+
+  await hh.open(page, `/projects/${project.ulid}`);
+  await expect(page.getByRole("heading", { level: 1, name: "Release" })).toBeVisible();
+  await expect(tabs(page).getByRole("link")).toHaveText(["ツリー", "カンバン", "テーブル"]);
+  await expect(row(page, "Ship it")).toBeVisible();
+  await tabs(page).getByRole("link", { name: "カンバン" }).click();
+  await expect(page).toHaveURL(new RegExp(`/projects/${project.ulid}/kanban$`));
+  await expect(board.locator(`li[data-ulid="${child.ulid}"]`)).toBeVisible();
+  await tabs(page).getByRole("link", { name: "テーブル" }).click();
+  await expect(page).toHaveURL(new RegExp(`/projects/${project.ulid}/table$`));
+  await expect(page.getByRole("textbox", { name: `タイトル: ${child.id}` })).toBeVisible();
+  await tabs(page).getByRole("link", { name: "ツリー" }).click();
+  await expect(page).toHaveURL(new RegExp(`/projects/${project.ulid}$`));
+  await expect(row(page, "Ship it")).toBeVisible();
+});
+
+test("the sidebar has the tasks of the person and none of the screens of 4e-4", async ({
+  page,
+  hh,
+}) => {
+  await hh.open(page);
+  await expect(sidebar(page).getByRole("link", { name: /マイタスク/ })).toHaveAttribute(
+    "href",
+    "/my",
+  );
+  await expect(sidebar(page).getByText("通知")).toHaveCount(0);
+  await expect(sidebar(page).getByText("アクティビティ")).toHaveCount(0);
+});
```

`web/e2e/kanban.spec.ts`:

```ts
// Screens this spec relies on (see screens.ts for the shared list):
//   - /workspaces/<store>/<ulid>/kanban and /projects/<ulid>/kanban show the board
//     (section "カンバンの盤面"): one list for each column, ul[data-column="<status>"], whose cards are
//     li[data-ulid="<ULID>"]; a card is dragged onto the list of another column to change its status
//   - a status that needs a field (the workflow of the project) asks for it first, in the dialog
//     "<ID> を<ステータス>にするには <項目> が必要です": the field is labelled by its name, the button
//     "<ステータス>にする" is disabled until the field is filled, and "やめる" closes it
import type { Page } from "@playwright/test";
import { expect, type Hedgehug, test } from "./harness";
import { dragCard } from "./screens";

// The path of a tab of the first workspace of the first store.
async function tab(hh: Hedgehug, name: string): Promise<string> {
  return (await hh.treePath()).replace(/\/tree$/, `/${name}`);
}

const cell = (page: Page, column: string) => page.locator(`ul[data-column="${column}"]`);
const card = (page: Page, ulid: string) => page.locator(`li[data-ulid="${ulid}"]`);
const commits = (hh: Hedgehug) => hh.hh("log").trim().split("\n").length;

test("dragging a card to another column changes its status", async ({ page, hh }) => {
  const alpha = hh.add("Alpha");
  hh.add("Bravo");
  await hh.open(page, await tab(hh, "kanban"));
  await expect(card(page, alpha.ulid)).toBeVisible();
  await expect(cell(page, "todo").locator(`li[data-ulid="${alpha.ulid}"]`)).toBeVisible();

  const before = commits(hh);
  await dragCard(page, card(page, alpha.ulid), cell(page, "in_progress"));
  await expect(cell(page, "in_progress").locator(`li[data-ulid="${alpha.ulid}"]`)).toBeVisible();
  await expect.poll(() => hh.show(alpha.id).status).toBe("in_progress");
  expect(commits(hh)).toBe(before + 1);
});

test("a status that needs a field asks for it, and sets both in one commit", async ({
  page,
  hh,
}) => {
  const project = hh.add("Release", "--kind", "project");
  const ship = hh.add("Ship it", "--parent", project.id);
  hh.hh("schema", "scope", "set", project.id, "--required", "in_progress=points");
  await hh.open(page, `/projects/${project.ulid}/kanban`);
  await expect(card(page, ship.ulid)).toBeVisible();

  const before = commits(hh);
  await dragCard(page, card(page, ship.ulid), cell(page, "in_progress"));
  const dialog = page.getByRole("dialog", {
    name: `${ship.id} を進行中にするには points が必要です`,
  });
  await expect(dialog).toBeVisible();
  // nothing is written until the field is given
  expect(hh.show(ship.id).status).toBe("todo");
  const go = dialog.getByRole("button", { name: "進行中にする" });
  await expect(go).toBeDisabled();

  await dialog.getByLabel("points", { exact: true }).fill("5");
  await expect(go).toBeEnabled();
  await go.click();
  await expect(dialog).toBeHidden();
  await expect(cell(page, "in_progress").locator(`li[data-ulid="${ship.ulid}"]`)).toBeVisible();

  const after = hh.show(ship.id);
  expect(after.status).toBe("in_progress");
  expect(after.props.points).toBe("5");
  expect(commits(hh)).toBe(before + 1);
});

test("closing the dialog of a missing field leaves the task where it was", async ({ page, hh }) => {
  const project = hh.add("Release", "--kind", "project");
  const ship = hh.add("Ship it", "--parent", project.id);
  hh.hh("schema", "scope", "set", project.id, "--required", "in_progress=points");
  await hh.open(page, `/projects/${project.ulid}/kanban`);

  const before = commits(hh);
  await dragCard(page, card(page, ship.ulid), cell(page, "in_progress"));
  const dialog = page.getByRole("dialog", {
    name: `${ship.id} を進行中にするには points が必要です`,
  });
  await dialog.getByRole("button", { name: "やめる" }).click();
  await expect(dialog).toBeHidden();
  await expect(cell(page, "todo").locator(`li[data-ulid="${ship.ulid}"]`)).toBeVisible();
  expect(hh.show(ship.id).status).toBe("todo");
  expect(commits(hh)).toBe(before);
});
```

`web/e2e/table.spec.ts`:

```ts
// Screens this spec relies on (see screens.ts for the shared list):
//   - /workspaces/<store>/<ulid>/table shows a row for each task, whose cells are inputs named by
//     the column and the ID: the textbox "タイトル: <ID>", the button "ステータス: <ID>" (it opens a
//     menu of the statuses), and so on. A cell saves 2 seconds after the last key or when it loses
//     the focus, as the detail does
import { expect, type Hedgehug, test } from "./harness";

async function tab(hh: Hedgehug, name: string): Promise<string> {
  return (await hh.treePath()).replace(/\/tree$/, `/${name}`);
}

test("editing the title of a cell saves the task when the cell loses the focus", async ({
  page,
  hh,
}) => {
  const alpha = hh.add("Old title");
  const other = hh.add("Other");
  await hh.open(page, await tab(hh, "table"));

  const title = page.getByRole("textbox", { name: `タイトル: ${alpha.id}` });
  await expect(title).toHaveValue("Old title");
  await title.fill("New title");
  await title.blur();
  await expect.poll(() => hh.show(alpha.id).title).toBe("New title");
  expect(hh.show(other.id).title).toBe("Other");
});

test("the status cell chooses a status from a menu", async ({ page, hh }) => {
  const alpha = hh.add("Alpha");
  await hh.open(page, await tab(hh, "table"));

  await page.getByRole("button", { name: `ステータス: ${alpha.id}` }).click();
  await page.getByRole("menuitem", { name: /進行中/ }).click();
  await expect.poll(() => hh.show(alpha.id).status).toBe("in_progress");
});

test("the filter field narrows the rows", async ({ page, hh }) => {
  const alpha = hh.add("Alpha");
  const bravo = hh.add("Bravo", "--status", "in_progress");
  await hh.open(page, await tab(hh, "table"));
  await expect(page.getByRole("textbox", { name: `タイトル: ${alpha.id}` })).toBeVisible();

  const filter = page.getByRole("textbox", { name: "フィルタ（hh ls と同じクエリ）" });
  await filter.fill("status:in_progress");
  await filter.press("Enter");
  await expect(page.getByRole("textbox", { name: `タイトル: ${bravo.id}` })).toBeVisible();
  await expect(page.getByRole("textbox", { name: `タイトル: ${alpha.id}` })).toHaveCount(0);
});
```

`web/e2e/my.spec.ts`:

```ts
// Screens this spec relies on (see screens.ts for the shared list):
//   - /my is the tasks of the person: the heading "マイタスク", the line "自分 = <login>", the filter
//     field (its default is assignee:@me -status:done -status:canceled), the count "<n> ワークスペース ·
//     <m> 件", and the regions named by their group: "期限切れ", "今日", "今週", "来週以降", "期限なし"
//     (by due date) or the name of the project, "プロジェクトなし" (by project; the button "まとめ方: …"
//     opens the menu 期限 / プロジェクト). A row shows the ID and the title as links, and the Store
//   - the sidebar ("メインナビゲーション") has the link "マイタスク" with the count of the same query
//   - a name of the query the server refuses ("status:in_progres") is explained, and the button named
//     by the right term ("status:in_progress") puts it in the field
import type { Page } from "@playwright/test";
import { expect, test } from "./harness";

const sidebar = (page: Page) => page.getByRole("navigation", { name: "メインナビゲーション" });
const filter = (page: Page) =>
  page.getByRole("textbox", { name: "フィルタ（hh ls と同じクエリ）" });

// The assignee is the ID of the account of the tests (host/login): the query's @me stands for it.
const ME = "github.com/e2e";
const ymd = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

test("the tasks of the person are grouped by due date, and the sidebar counts them", async ({
  page,
  hh,
}) => {
  const today = new Date();
  const later = new Date(today.getFullYear(), today.getMonth(), today.getDate() + 10);
  const dueToday = hh.add("Due today", "-p", `assignees=${ME}`, "-p", `due=${ymd(today)}`);
  const dueLater = hh.add("Due later", "-p", `assignees=${ME}`, "-p", `due=${ymd(later)}`);
  const noDue = hh.add("No due date", "-p", `assignees=${ME}`);
  const finished = hh.add("Finished", "-p", `assignees=${ME}`);
  hh.hh("set", finished.id, "status=done");
  hh.add("Somebody else's");

  await hh.open(page);
  await sidebar(page)
    .getByRole("link", { name: /マイタスク/ })
    .click();
  await expect(page).toHaveURL(`${hh.origin}/my`);
  await expect(page.getByRole("heading", { level: 1, name: "マイタスク" })).toBeVisible();
  await expect(page.getByText("自分 = e2e")).toBeVisible();
  await expect(filter(page)).toHaveValue("assignee:@me -status:done -status:canceled");
  await expect(page.getByText("1 ワークスペース · 3 件")).toBeVisible();
  // the sidebar asks the same query
  await expect(sidebar(page).getByRole("link", { name: /マイタスク/ })).toContainText("3");

  const group = (name: string) => page.getByRole("region", { name, exact: true });
  await expect(group("今日").getByRole("row", { name: new RegExp(dueToday.id) })).toContainText(
    "Due today",
  );
  await expect(group("来週以降").getByRole("row", { name: new RegExp(dueLater.id) })).toContainText(
    "Due later",
  );
  await expect(group("期限なし").getByRole("row", { name: new RegExp(noDue.id) })).toContainText(
    "No due date",
  );
  await expect(page.getByText("Finished")).toHaveCount(0);
  await expect(page.getByText("Somebody else's")).toHaveCount(0);
  // the store is told on every row
  await expect(group("今日").getByRole("row").first()).toContainText("demo");

  // a task opens from its title
  await group("今日").getByRole("link", { name: "Due today" }).click();
  await expect(page).toHaveURL(new RegExp(`/tasks/${dueToday.ulid}$`));
});

test("the tasks are grouped by project from the menu", async ({ page, hh }) => {
  const project = hh.add("Release", "--kind", "project");
  hh.add("In the project", "-p", `assignees=${ME}`, "--parent", project.id);
  hh.add("Outside", "-p", `assignees=${ME}`);
  await hh.open(page, "/my");

  await page.getByRole("button", { name: "まとめ方: 期限" }).click();
  await page.getByRole("menuitem", { name: "プロジェクト" }).click();
  await expect(
    page.getByRole("region", { name: "Release", exact: true }).getByText("In the project"),
  ).toBeVisible();
  await expect(
    page.getByRole("region", { name: "プロジェクトなし" }).getByText("Outside"),
  ).toBeVisible();
});

test("a task that gets an assignee from the CLI appears and the count follows", async ({
  page,
  hh,
}) => {
  const t = hh.add("Not mine yet");
  await hh.open(page, "/my");
  await expect(page.getByText("一致するタスクはありません")).toBeVisible();

  hh.hh("set", t.id, `assignees=${ME}`);
  await expect(page.getByRole("link", { name: "Not mine yet" })).toBeVisible();
  await expect(sidebar(page).getByRole("link", { name: /マイタスク/ })).toContainText("1");
});

test("a query that names a status wrong offers the right one", async ({ page, hh }) => {
  const busy = hh.add("In progress", "-p", `assignees=${ME}`, "--status", "in_progress");
  hh.add("Waiting its turn", "-p", `assignees=${ME}`);
  await hh.open(page, "/my");
  await expect(page.getByRole("link", { name: "Waiting its turn" })).toBeVisible();

  await filter(page).fill("assignee:@me status:in_progres");
  await filter(page).press("Enter");
  await expect(page.getByRole("alert")).toContainText("クエリが正しくありません");
  const fix = page.getByRole("button", { name: "status:in_progress" });
  await expect(fix).toBeVisible();
  await expect(page.getByText("のことですか？")).toBeVisible();

  await fix.click();
  await expect(filter(page)).toHaveValue("assignee:@me status:in_progress");
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.getByRole("link", { name: busy.title })).toBeVisible();
  await expect(page.getByRole("link", { name: "Waiting its turn" })).toHaveCount(0);
});
```

`web/e2e/views.spec.ts`:

```ts
// Screens this spec relies on (see screens.ts for the shared list):
//   - the tab "ツリー" of a workspace has the button "ビューを保存"; it opens the dialog "ビューを保存"
//     (the textbox "名前", the buttons "保存" and "キャンセル", the line of the command hh view save).
//     Saving shows the toast "ビュー <名前> を <Store> に保存しました" and opens /views/<Store>/<名前>
//   - /views/<Store>/<名前> has the heading (level 1) <名前>, the navigation "レイアウトの切り替え"
//     (the links ツリー, カンバン, テーブル; the one of the saved layout is aria-current="page"; a
//     click saves the layout and keeps the address), the board "カンバンの盤面" of the kanban, the
//     table with the textboxes "タイトル: <ID>", the status "未保存の変更" with the button
//     "ビューを更新" once the filter differs from the saved query, and the button "ビューの操作
//     （改名・削除）" whose menu has 改名… and 削除…
//   - rename: the dialog "ビュー <名前> を改名" (the textbox "新しい名前", the button "改名", the toast
//     "ビュー <a> を <b> に改名しました"); delete: the dialog "ビュー <名前> を削除" (the button
//     "削除", the toast "ビュー <名前> を削除しました" with the button "取り消し")
//   - /query/<layout>?q= is the heading "クエリ" with the same navigation and the button
//     "ビューを保存"; the filter field writes the query into the address, and the tabs keep it
//   - the sidebar has the region "保存済みビュー" with a link for each view (its title says where it
//     is saved: "<Store> の views/ に保存"); the palette lists the views as "<名前> を開く", and a
//     query that found tasks has the group "このクエリで" (テーブルで開く, ビューとして保存…)
import type { Page } from "@playwright/test";
import { expect, test } from "./harness";
import { row, toast } from "./screens";

type SavedView = { name: string; layout: string; query: string };

const sidebar = (page: Page) => page.getByRole("navigation", { name: "メインナビゲーション" });
const saved = (page: Page) => sidebar(page).getByRole("region", { name: "保存済みビュー" });
const layouts = (page: Page) => page.getByRole("navigation", { name: "レイアウトの切り替え" });
const filter = (page: Page) =>
  page.getByRole("textbox", { name: "フィルタ（hh ls と同じクエリ）" });
const board = (page: Page) => page.locator('section[aria-label="カンバンの盤面"]');
const palette = (page: Page) => page.getByRole("dialog", { name: "ID・検索・コマンド" });

test("a tree saved as a view opens, changes its layout, is updated, renamed and deleted", async ({
  page,
  hh,
}) => {
  const doing = hh.add("Doing it", "--status", "in_progress");
  hh.add("Not yet");
  const views = () => hh.json<SavedView[]>("view", "list");

  await hh.openTree(page);
  await filter(page).fill("status:in_progress");
  await filter(page).press("Enter");
  await expect(row(page, "Doing it")).toBeVisible();
  await expect(row(page, "Not yet")).toHaveCount(0);

  // save what the tree shows
  await page.getByRole("button", { name: "ビューを保存" }).click();
  const save = page.getByRole("dialog", { name: "ビューを保存" });
  await save.getByRole("textbox", { name: "名前" }).fill("doing");
  await expect(save.getByRole("button", { name: /コマンドをコピー/ })).toContainText(
    "hh view save doing",
  );
  await save.getByRole("button", { name: "保存", exact: true }).click();
  await expect(toast(page, "ビュー doing を demo に保存しました")).toBeVisible();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/doing`);
  expect(views()).toMatchObject([
    { name: "doing", layout: "tree", query: "workspace:demo status:in_progress" },
  ]);

  // it opens as saved: the tasks of the query, the sidebar marks it
  await expect(page.getByRole("heading", { level: 1, name: "doing" })).toBeVisible();
  await expect(row(page, "Doing it")).toBeVisible();
  await expect(row(page, "Not yet")).toHaveCount(0);
  await expect(saved(page).getByRole("link", { name: "doing" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(saved(page).getByRole("link", { name: "doing" })).toHaveAttribute(
    "title",
    "demo の views/ に保存",
  );

  // the tabs change the layout of the saved view, and the address stays
  await layouts(page).getByRole("link", { name: "カンバン" }).click();
  await expect.poll(() => views()[0]?.layout).toBe("kanban");
  await expect(board(page)).toBeVisible();
  await expect(board(page).locator(`li[data-ulid="${doing.ulid}"]`)).toBeVisible();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/doing`);
  await page.reload();
  await expect(board(page)).toBeVisible();

  await layouts(page).getByRole("link", { name: "テーブル" }).click();
  await expect.poll(() => views()[0]?.layout).toBe("table");
  await expect(page.getByRole("textbox", { name: `タイトル: ${doing.id}` })).toBeVisible();

  // a changed filter is not saved until the view is updated
  await expect(page.getByText("未保存の変更")).toHaveCount(0);
  // the field of a saved view holds the whole query: nothing is fixed on a screen of every store
  await expect(filter(page)).toHaveValue("workspace:demo status:in_progress");
  await filter(page).fill("workspace:demo status:in_progress kind:task");
  await filter(page).press("Enter");
  await expect(page.getByText("未保存の変更")).toBeVisible();
  expect(views()[0]?.query).toBe("workspace:demo status:in_progress");
  await page.getByRole("button", { name: "ビューを更新" }).click();
  await expect.poll(() => views()[0]?.query).toBe("workspace:demo status:in_progress kind:task");
  await expect(page.getByText("未保存の変更")).toHaveCount(0);

  // rename
  await page.getByRole("button", { name: "ビューの操作（改名・削除）" }).click();
  await page.getByRole("menuitem", { name: "改名…" }).click();
  const rename = page.getByRole("dialog", { name: "ビュー doing を改名" });
  await rename.getByRole("textbox", { name: "新しい名前" }).fill("busy");
  await rename.getByRole("button", { name: "改名", exact: true }).click();
  await expect(toast(page, "ビュー doing を busy に改名しました")).toBeVisible();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/busy`);
  expect(views().map((v) => v.name)).toEqual(["busy"]);
  await expect(saved(page).getByRole("link", { name: "busy" })).toBeVisible();

  // delete, and take it back (at once: the undo comes before the poll of the server has seen the
  // delete, so no views.changed comes, and the tab reads the views again after the undo itself)
  await page.getByRole("button", { name: "ビューの操作（改名・削除）" }).click();
  await page.getByRole("menuitem", { name: "削除…" }).click();
  const remove = page.getByRole("dialog", { name: "ビュー busy を削除" });
  await remove.getByRole("button", { name: "削除", exact: true }).click();
  const gone = toast(page, "ビュー busy を削除しました");
  await expect(gone).toBeVisible();
  expect(views()).toEqual([]);
  await expect(saved(page)).toHaveCount(0);
  await expect(page).toHaveURL(`${hh.origin}/`);

  await gone.getByRole("button", { name: "取り消し" }).click();
  await expect.poll(() => views().map((v) => v.name)).toEqual(["busy"]);
  await expect(saved(page).getByRole("link", { name: "busy" })).toBeVisible();
});

test("a view the CLI saves appears in the sidebar and the palette, and opens", async ({
  page,
  hh,
}) => {
  const alpha = hh.add("Alpha");
  hh.add("Bravo", "--status", "in_progress");
  await hh.open(page);
  await expect(saved(page)).toHaveCount(0);

  hh.hh("view", "save", "from-cli", "--query", "status:todo", "--layout", "table");
  const link = saved(page).getByRole("link", { name: "from-cli" });
  await expect(link).toBeVisible();

  await link.click();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/from-cli`);
  await expect(page.getByRole("textbox", { name: `タイトル: ${alpha.id}` })).toBeVisible();
  await expect(page.getByRole("textbox", { name: /^タイトル: / })).toHaveCount(1);

  // and the palette opens it from anywhere
  await page.goBack();
  await expect(page).toHaveURL(`${hh.origin}/`);
  await page.keyboard.press("ControlOrMeta+k");
  const box = palette(page).getByRole("combobox");
  await box.fill(">from-cli");
  await expect(palette(page).getByRole("option", { name: /from-cli を開く/ })).toBeVisible();
  await box.press("Enter");
  await expect(page).toHaveURL(`${hh.origin}/views/demo/from-cli`);
});

test("a query from the palette opens as a table, changes layout without saving, and is saved", async ({
  page,
  hh,
}) => {
  const alpha = hh.add("Alpha");
  const bravo = hh.add("Bravo", "--status", "in_progress");
  const views = () => hh.json<SavedView[]>("view", "list");
  await hh.open(page);

  await page.keyboard.press("ControlOrMeta+k");
  const box = palette(page).getByRole("combobox");
  await box.fill("status:in_progress");
  await page.getByRole("option", { name: /テーブルで開く/ }).click();
  await expect(page).toHaveURL(/\/query\/table\?q=status%3Ain_progress$/);
  await expect(page.getByRole("heading", { level: 1, name: "クエリ" })).toBeVisible();
  await expect(page.getByRole("textbox", { name: `タイトル: ${bravo.id}` })).toBeVisible();
  await expect(page.getByRole("textbox", { name: `タイトル: ${alpha.id}` })).toHaveCount(0);

  // the tab changes the layout and keeps the query; nothing is saved
  await layouts(page).getByRole("link", { name: "カンバン" }).click();
  await expect(page).toHaveURL(/\/query\/kanban\?q=status%3Ain_progress$/);
  await expect(board(page).locator(`li[data-ulid="${bravo.ulid}"]`)).toBeVisible();
  expect(views()).toEqual([]);

  // the field writes the query into the address
  await filter(page).fill("status:todo");
  await filter(page).press("Enter");
  await expect(page).toHaveURL(/\/query\/kanban\?q=status%3Atodo$/);
  await expect(board(page).locator(`li[data-ulid="${alpha.ulid}"]`)).toBeVisible();

  // and the screen saves it as a view in the layout it shows
  await page.getByRole("button", { name: "ビューを保存" }).click();
  const save = page.getByRole("dialog", { name: "ビューを保存" });
  await save.getByRole("textbox", { name: "名前" }).fill("todo-board");
  await save.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/todo-board`);
  expect(views()).toMatchObject([{ name: "todo-board", layout: "kanban", query: "status:todo" }]);
});

test("the palette saves the query it was given as a view", async ({ page, hh }) => {
  hh.add("Alpha");
  await hh.open(page);

  await page.keyboard.press("ControlOrMeta+k");
  await palette(page).getByRole("combobox").fill("status:todo");
  await page.getByRole("option", { name: /ビューとして保存/ }).click();
  const save = page.getByRole("dialog", { name: "ビューを保存" });
  await expect(save).toContainText("status:todo");
  await save.getByRole("textbox", { name: "名前" }).fill("todo-table");
  await save.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/todo-table`);
  expect(hh.json<SavedView[]>("view", "list")).toMatchObject([
    { name: "todo-table", layout: "table", query: "status:todo" },
  ]);
});
```

- [ ] **Step 20b: `useListScope` の `apiBase`・`cliBase` を文字列の `$derived` にする（Step 21 の E2E で見つかった）**

`apiBase`・`cliBase` は `base`（`workspace` から作る新しいオブジェクト）を読んで文字列を返すだけの getter だったので、これを読む `$effect`（ツリーの `watchTree`）は、タスクを作って Store が読み直され `workspace` が同じ内容の新しいオブジェクトになるたびに走り直し、ツリーを読み込み直して行を作り直していた（`tree.spec.ts` の「Esc closes the input even when the task is still being made」「a click on a row closes the input, also while the task is being made」が、行のフォーカスを失って落ちた。main の 79a5277 では通る）。文字列の `$derived` にすれば、中身が同じなら効果は走り直さない。

`web/app/list/scope.svelte.test.ts` の差分（先に足して、失敗を確かめる）:

```diff
--- a/web/app/list/scope.svelte.test.ts
+++ b/web/app/list/scope.svelte.test.ts
@@ -150,6 +150,27 @@ describe("useListScope: a workspace", () => {
     stop();
   });
 
+  it("tells an effect that reads the query only when the query changes, not when the workspace is read again", () => {
+    const { w, stores } = setup();
+    let runs = 0;
+    const stop = $effect.root(() => {
+      const s = useListScope(() => ({ kind: "workspace", store: "shared", ulid: "W1" }));
+      $effect(() => {
+        // What the tree reads to start its query: reading it again would reload the tree.
+        const read = [s.ready, s.apiBase, s.cliBase, s.store];
+        if (read.length > 0) runs++;
+      });
+    });
+    flushSync();
+    const before = runs;
+    // The stores are read again (a task was made): an equal workspace, a new object.
+    w.workspaces = [entry()];
+    stores.emit();
+    flushSync();
+    expect(runs).toBe(before);
+    stop();
+  });
+
   it("follows the scope it is given", () => {
     setup({ workspaces: [entry(), entry({ ulid: "W2", name: "web" })] });
     let current = $state<ListScope>({ kind: "workspace", store: "shared", ulid: "W1" });
```

Run: `cd web && pnpm vitest run app/list/scope.svelte.test.ts`
Expected: FAIL（`expected 2 to be 1`）

`web/app/list/scope.svelte.ts` の差分:

```diff
--- a/web/app/list/scope.svelte.ts
+++ b/web/app/list/scope.svelte.ts
@@ -68,6 +68,10 @@ export function useListScope(get: () => ListScope) {
     return null;
   });
   const ready = $derived(get().kind === "all" || base !== null);
+  // Strings, so an effect that reads them runs again when the query changes, not each time the
+  // workspace is read again (a new object with the same name).
+  const apiBase = $derived(base ? baseQuery(base, "api") : "");
+  const cliBase = $derived(base ? baseQuery(base, "cli") : "");
   const manyStores = $derived.by(() => {
     storesRev.current;
     return c.stores.stores().length >= 2;
@@ -88,11 +92,11 @@ export function useListScope(get: () => ListScope) {
     },
     /** The fixed part of the query for the API (the project by ULID); "" for every store or while not ready. */
     get apiBase(): string {
-      return base ? baseQuery(base, "api") : "";
+      return apiBase;
     },
     /** The fixed part as the user would type it in hh (the project by ID); "" for every store or while not ready. */
     get cliBase(): string {
-      return base ? baseQuery(base, "cli") : "";
+      return cliBase;
     },
     /** The store of the workspace or project; undefined for every store. */
     get store(): string | undefined {
```

Run: `cd web && pnpm fix && pnpm check && pnpm vitest run app/list app/tree app/kanban app/table`
Expected: PASS

```bash
git add web/app/list/scope.svelte.ts web/app/list/scope.svelte.test.ts
git commit -m "fix(web): ツリーが、同じ内容のワークスペースを読み直しただけで読み込み直さないようにする

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 20c: ビューの書き込みを取り消したら、ビューの一覧を読み直す（Step 21 の E2E で見つかった）**

サーバーはビューの変更を 300ms ごとのポーリングで `views.changed` として知らせる。ビューを削除してすぐ（ポーリングが削除を見る前に）取り消すと、2 回のポーリングのあいだで木が同じに戻るので何も知らせない。タブの `ViewDirectory` は削除の応答で一覧から消しているので、取り消したビューがサイドバーに戻らない（`views.spec.ts` の「a tree saved as a view …」の末尾が `--repeat-each=12` で 12 回中 8 回落ちた）。`runUndo` が取り消した操作がビュー（`op` が `view`）なら、その Store のビューを読み直す。E2E は取り消しの前に待たない（待つとこの不具合を隠す）。

テストを先に足して、失敗を確かめる（`dir.refresh is not a function` と、`refresh` が呼ばれない）:

```diff
diff --git a/web/app/undo.test.ts b/web/app/undo.test.ts
index b710945..8bd5f9b 100644
--- a/web/app/undo.test.ts
+++ b/web/app/undo.test.ts
@@ -98,8 +98,12 @@ describe("runUndo", () => {
   function setup(route: Parameters<typeof fakeApi>[0]) {
     const { api, calls } = fakeApi(route);
     const noteOwnCommit = vi.fn();
-    const c = { api, cache: { noteOwnCommit } } as unknown as Pick<AppCore, "api" | "cache">;
-    return { c, calls, noteOwnCommit };
+    const refresh = vi.fn();
+    const c = { api, cache: { noteOwnCommit }, views: { refresh } } as unknown as Pick<
+      AppCore,
+      "api" | "cache" | "views"
+    >;
+    return { c, calls, noteOwnCommit, refresh };
   }
 
   it("posts to /api/undo with the store, remembers the commit and says what it did", async () => {
@@ -112,6 +116,22 @@ describe("runUndo", () => {
     expect(toasts.list.map((t) => t.message)).toEqual(["REL-6 の変更を取り消しました"]);
   });
 
+  it("reads the views of the store again after it takes back a change of a view", async () => {
+    // The server tells views.changed from its poll: a delete taken back before the poll saw it
+    // leaves the tree as it was, so nothing is told, and the list would keep the view out.
+    const { c, refresh } = setup(() =>
+      result({ undone: { ...result().undone, op: "view", task: null, id: null, store: "work" } }),
+    );
+    await runUndo(c, "work");
+    expect(refresh).toHaveBeenCalledWith("work");
+  });
+
+  it("does not read the views again after it takes back another operation", async () => {
+    const { c, refresh } = setup(() => result());
+    await runUndo(c, "shared");
+    expect(refresh).not.toHaveBeenCalled();
+  });
+
   it("sends no store when there is none", async () => {
     const { c, calls } = setup(() => result());
     await runUndo(c);
diff --git a/web/core/views.test.ts b/web/core/views.test.ts
index 5378aa3..dc88f45 100644
--- a/web/core/views.test.ts
+++ b/web/core/views.test.ts
@@ -213,6 +213,20 @@ describe("ViewDirectory", () => {
     expect(views("nobody")).toHaveLength(0);
   });
 
+  it("refresh fetches the store again, and not one it does not know", async () => {
+    // An undo that the poll of the server missed (a delete taken back within one poll) leaves no
+    // views.changed: the screen asks for the views itself.
+    const { dir, w, views } = setup({ shared: [], personal: [] });
+    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
+    w.shared = [view("weekly")];
+    dir.refresh("shared");
+    await vi.waitFor(() => expect(names(dir)).toEqual(["shared/weekly"]));
+    expect(views("personal")).toHaveLength(1);
+    dir.refresh("nobody");
+    await settle();
+    expect(views("nobody")).toHaveLength(0);
+  });
+
   it("fetches every store again at resync, and keeps the entries that did not change", async () => {
     const { dir, conn, views } = setup({ shared: [view("weekly")], personal: [view("mine")] });
     await vi.waitFor(() => expect(dir.loaded()).toBe(true));
```

Run: `cd web && pnpm vitest run core/views.test.ts app/undo.test.ts`
Expected: 2 件が FAIL

実装:

```diff
diff --git a/web/app/undo.ts b/web/app/undo.ts
index 6cf39a7..7a8fc89 100644
--- a/web/app/undo.ts
+++ b/web/app/undo.ts
@@ -41,11 +41,17 @@ export function routeStore(
 // went. It does not ask first: the spec holds that an undo is not hard to take back. The store is
 // the one the screen is about; without it the server picks one as the CLI does, which asks the
 // user to choose when there are several.
-export async function runUndo(c: Pick<AppCore, "api" | "cache">, store?: string): Promise<void> {
+export async function runUndo(
+  c: Pick<AppCore, "api" | "cache" | "views">,
+  store?: string,
+): Promise<void> {
   try {
     const r = await unwrap(c.api.POST("/api/undo", { params: { query: { store } } }));
     // The commit is this tab's own: its tasks.changed is not shown as a change from outside.
     c.cache.noteOwnCommit(r.commit);
+    // The server tells views.changed from its poll, which misses a write taken back before it
+    // looked (the store is as it was): the list of views would keep what this tab wrote.
+    if (r.undone.op === "view") c.views.refresh(r.undone.store);
     toast({ message: undoMessage(r) });
   } catch (e) {
     toast({ message: errorMessage(e) });
diff --git a/web/core/views.ts b/web/core/views.ts
index a624886..06693d9 100644
--- a/web/core/views.ts
+++ b/web/core/views.ts
@@ -143,6 +143,12 @@ export class ViewDirectory {
     return () => this.subs.delete(fn);
   }
 
+  // refresh reads the views of a store again: for a change the server does not tell (an undo of a
+  // write that its poll did not see leaves the store as it was).
+  refresh(store: string): void {
+    if (this.names.includes(store)) void this.fetch(store);
+  }
+
   // save creates the view, or changes the settings body holds (PUT: what is not sent stays).
   async save(store: string, name: string, body: ViewSaveBody): Promise<SavedView> {
     const r = await unwrap(
```

Run: `cd web && pnpm fix && pnpm check && pnpm vitest run core/views.test.ts app/undo.test.ts`
Expected: PASS（`pnpm check` は警告 0）

```bash
git add web/core/views.ts web/core/views.test.ts web/app/undo.ts web/app/undo.test.ts
git commit -m "fix(web): ビューの書き込みを取り消したら、ビューの一覧を読み直す

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 21: E2E を流す**

Run（前面で、Bash の timeout は 600000）: `make e2e`
Expected: 既存の spec の全件と、新しい 4 つの spec（`kanban`・`table`・`my`・`views`）と `routes.spec.ts` の追加分が PASS

落ちたら、失敗の出力（エラーとスクリーンショットかトレース）と実行条件（並列数、`CI` の有無）を残してから直す。実物の言葉と spec の言葉が違うだけなら spec を直す。ドラッグが動かないときは、`dragCard`（`screens.ts`）の移動の刻みを `tree.spec.ts` の `dragRow` と同じにしてあるので、カンバンの `dnd.ts` が受け取るイベントを確かめる。

`web/e2e/create.spec.ts` の「C on a task detail makes a child of that task」は、詳細が読み込まれる前に `c` を押すと親のないダイアログが開く（全件の流しで負荷がかかると落ちる）。`c` の前に待つ:

変更前:
```ts
  await hh.open(page, `/tasks/${parent.ulid}`);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText(parent.id, { exact: true })).toBeVisible();
```

変更後:
```ts
  await hh.open(page, `/tasks/${parent.ulid}`);
  // C makes a child of the task the screen holds: wait until the detail has read it
  await expect(page.getByRole("textbox", { name: "タイトル" })).toHaveValue("Parent task");
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText(parent.id, { exact: true })).toBeVisible();
```

`web/e2e/newtab.spec.ts` の「Ctrl+click on a sidebar link opens a tab that keeps the token」は、macOS の Chromium では Ctrl+クリックが右クリック（contextmenu）になって新しいタブが開かず落ちる（main の 79a5277 でも同じ）。アプリは ⌘ でも Ctrl でも新しいタブで開く（`router.ts` の `metaKey || ctrlKey`）ので、OS に合わせて押す:

変更前:
```ts
test("Ctrl+click on a sidebar link opens a tab that keeps the token", async ({
```
```ts
    link.click({ modifiers: ["Control"] }),
```

変更後:
```ts
test("Ctrl+click (Cmd+click on macOS) on a sidebar link opens a tab that keeps the token", async ({
```
```ts
    link.click({ modifiers: ["ControlOrMeta"] }),
```

- [ ] **Step 22: 全体を確かめて、コミットする**

Run: `cd web && pnpm fix && pnpm check && pnpm exec biome ci --vcs-enabled=false . && pnpm test`
Expected: すべて通る

```bash
git add web/e2e
git commit -m "test(e2e): カンバンの移動、テーブルのセル編集、保存済みビュー、クエリの画面、マイタスクを足す

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

## 決めたこと

この plan を組むときに決めた点。D1・D2 と「利用者の決定」は 2026-11-03 に利用者が決めた。ほかは親設計（Claude Docs の Hedgehug 設計ドキュメント）から導いたものと、plan の書き手の推奨を controller が採ったもの。概要 plan の細部と spec に反映する（細部の番号は概要 plan を指す）。

親設計の拠り所: 「ビューは常に統合する」「統合ビュー（マイタスク、Store をまたぐクエリ）」、クエリは既定で全マウント Store（`store:` で絞る）、「Store をまたぐ統合ビューはカテゴリで揃える」「sort:status はカテゴリの順で Store をまたいで比べ、同じなら各スキーマでの位置、それも同じなら Store の順」「複数のプロジェクトのタスクが並ぶビュー（マイタスク、Store をまたぐクエリ）は Store の定義で表示する」「マイタスク: 全 Store から」「タスクごとに、Store の色とバッジを常に表示する」「別の Store のタスクの下へ移すなど、Store をまたぐ参照になる操作は、ドラッグの時点で弾いて理由を表示する」。

1. **D1（利用者）保存していないクエリの画面**: `/query/<layout>?q=<クエリ>`（layout は tree・kanban・table）。全 Store で評価し、レイアウトはタブで切り替える（URL を替えるだけで保存しない）。「ビューを保存」を置く。router はこのルートだけ `?q=` を残す。フィルタ欄の変更は `router.replace` で URL の q に反映し、戻ってきた q が画面の値と同じなら何もしない。パレットの「このクエリで: テーブルで開く」はここを開く
2. **D2（利用者）`hh view show` の評価範囲**: `--store` はビューを読む場所としてだけ使い、クエリは常に全マウント Store で評価する（絞るのはクエリの `store:`）。GUI の保存済みビューも全 Store で評価し、足元の `hh view show <名前> --store <Store>` と結果が一致する。親設計の「保存済みビュー」に1行足す
3. **D3 scoped を使う画面**: scoped の `statuses` を使うのはプロジェクトの画面だけ。ビュー・クエリ・マイタスク・ワークスペースの画面は Store の定義
4. **D4 Store をまたぐカンバンの status 列**: 表示中のタスクの Store のスキーマのステータスの和を、`web/core/status-order.ts` の順（カテゴリ → スキーマでの位置 → Store の順）に並べる。同じ名前は1列（表示名は最初の Store のもの）。1 Store の画面ではスキーマの全ステータス（中止のカテゴリはタスクがあるときだけ）。テーブルの sort:status とグループの順も同じ関数を使う
5. **D5 Store をまたぐツリー**: 根を Store の順に並べ、Store が2つ以上なら行に Store のバッジ。種類・親の規則とキー操作は行の Store のスキーマ。下に追加・C は選んだ行の Store とワークスペース。行がないときの作成は作成ダイアログ。Store をまたぐドロップの拒否の文言は 4d のまま（「置けません」の既存の文言とテスト）
6. **D6 未保存の変更**: ビューの画面では、クエリ・カンバンの設定・テーブルの設定のどれかが保存した値と違えば「未保存の変更」の印と「ビューを更新」（1回の `PATCH /api/views/{name}`。更新だけで作らず、404 `view.not_found` なら notfound）。比べるクエリは前後の空白を除くだけ（語の順や引用符は正規化しない）。レイアウトの切り替えはその場で `PATCH {layout}`（利用者の決定、2026-11-03。下の「組み上げで直した点」）
7. **D7 ビューでない画面の設定**: 列・スイムレーン・テーブルの列・グループは画面を開いているあいだだけ（保存しない、localStorage も使わない）。初期値はビューの既定。フィルタの既定はツリーと同じ `-status:canceled`。TreeView は外から `query` を渡されたら `""` でもそのまま使う。ワークスペースとプロジェクトのフィルタは、タブを持つページ（`WorkspacePage`・`ProjectPage`）が1つ持ち、ツリー・カンバン・テーブルのタブをまたいで残る（ページを開いているあいだだけ。保存しない）
8. **D8 ワークスペース・プロジェクトから保存するクエリ**: 固定の部分（`workspace:<名前>`・`project:<ID>`。CLI の形）＋フィルタ欄。マウントされた Store が2つ以上なら先頭に `store:<Store>`。保存先の既定はその画面の Store。保存したら `/views/<store>/<名前>` を開く
9. **D9 ビューの URL**: `/views/<store>/<名前>`（`encodeURIComponent`）。開いているビューが消えた・改名されたら Empty の notfound
10. **D10 ビューの足元**: `hh view show <名前> --store <Store>`。テーブル・カンバンの設定の近くに細部 49 の `hh view save …`
11. **（利用者）プロジェクトのカンバンの表示外のステータス**: scoped の `statuses` に加え、タスクのいる表示外のステータス（スキーマにないものも）の列を出し、タスクを隠さない。細部 36 を直す
12. **（利用者）必須の項目**: 移る先の必須の項目（Store と scoped の和）が欠けていれば、ダイアログに全部並べる。担当者・ラベル・プロパティはその場で入力し、ほか（親・ブロッカー・ウォッチャー・リポジトリ・タスク参照・スプリント・リリース・リンク）は「詳細で入力」へ案内して移動を無効にする（サーバーの 422 を起こさない）。ウォッチャーとリポジトリは必須のときだけ `GET /api/tasks/{id}` で確かめる。スプリント・リリース・リンクは API にまだない（5・6）ので、それを必須にした Store ではその状態へカンバンから移せない（CLI でも埋められないため、その時点では想定どおり）。細部 39 を直す
13. **（利用者）「もしかして」**: 編集距離 2 以内の名前、なければ打った語（3 文字以上）を含む名前。キャンバスの `progres` → `in_progress` が出る。細部 58 を直す
14. ビューを保存するとき、同じ Store に同じ名前のビューがあれば保存を無効にして理由を出す（`PUT` は黙って置き換えるため。更新はビューの画面の「ビューを更新」）
15. パレットの「ビューとして保存…」はテーブルの layout で開き、「このクエリで」のグループは一致があるときだけ出す
16. プロジェクトの見出し（パンくずとタブ）は Task 7 で `ProjectHeader` に切り出し、ツリー・カンバン・テーブルのタブで共有する
17. カンバンのカードのキーボード操作（S でステータスなど）は作らない（spec のキーボードにない）。カードは Tab で止まり Enter で詳細
18. テーブル: プロジェクトの画面でもステータスのセルの選択肢は全ステータス（詳細と同じ）。列の順の変更は「外して足し直す」（末尾に付く）だけ
19. `router.replace` は購読者に通知する。`useListScope` は router を import しない（テストのモックのため）。`ListBar`・`FilterBar` は固定の部分が空なら札を出さない。`tree/filter.ts` の `combine("", q)` は先頭に空白を付けない（Task 3 で直す）
20. `hh view show` の直しは、`viewShow` の中で `a.ss` を `--store` なしのセッションに差し替えてクエリを評価する（ops に全 Store 版を足して二重にしない）
21. **既存テストの負荷下の時間切れ**（Task 3 の最初の Step）: 全件を同時に何本も流すと、実時間に頼るテスト（TreeView の「lets a late wait for a row give way to a newer one」、BodyEditor の「offers the people for @ from GET /api/people」）が `waitFor` の1秒で切れ、重いテスト（TaskDetail 系・CreateTask の各ファイルの最初のテスト。単独で約 0.5 秒）が既定の 5 秒で切れた。前の2つは偽のタイマーで時間を進める形に直し、`vitest.config.ts` の `testTimeout` を 20 秒にする。全件 3 本を同時に 3 回で、直す前は 9 本中 7 本が落ち、直したあとは 9 本とも通る。製品のコードの不具合ではない

## 組み上げで直した点

- Task 5 Step 9: `TableView.test.ts` の「a long list」が `document.querySelector<HTMLElement>(".row")` で最初の `.row` を取ると、`ListBar`（Task 2 の実物）の中の `<div class="row">`（フィルタ欄）に当たり、`toHaveStyle({ transform: "translateY(0px)" })` が落ちた（`expect(element).toHaveStyle()`、受け取った側の transform が空。`pnpm vitest run app/table` で単独実行）。`tr.row` に直した（Step 9 のテストと、このときの本文）。直したあとは `app/table` 52 件（columns 14・group 6・TableView 32）と全件 105 ファイル 1083 件が通る。他の差分はなし（`@tanstack/svelte-table` 9.3.1 の API は plan のコードのまま `svelte-check` 0 エラー 0 警告）
- Task 7 Step 9: Step 9 の `pnpm vitest run app/my app/list app/tree` で `app/tree/TreeView.test.ts` の「shows the message of query.invalid under the filter」「shows no empty state when the query is wrong」の 2 件が落ちた（`TypeError: c.stores.stores is not a function`、`FilterBar.svelte` の `$effect`。`pnpm vitest run app/tree/TreeView.test.ts -t "query.invalid|wrong"` で単独でも）。`TreeView.test.ts` の `setup()` の core の `stores` に `stores: () => [{ name: "shared" }]` を足す直しは Step 13 にあったので、Step 9 に移し（コミットにも `TreeView.test.ts` を足した）、Step 13 からは消した。直したあとは `app/my app/list app/tree` 20 ファイル 234 件が通る
- Task 7 Step 21: `kanban.spec.ts` の「a status that needs a field …」「closing the dialog of a missing field …」が、`done` の列がページの外（x=1714、幅 1440）にあるためドロップが届かず、ダイアログが出ないまま落ちた（`getByRole('dialog', { name: 'DM-2 を完了にするには points が必要です' })` が見つからない、`pnpm exec playwright test e2e/kanban.spec.ts` で単独でも、1440x900）。必須の項目を `done` ではなく、ページ内に収まる `in_progress` に付けるように直した（`--required in_progress=points`、`cell(page, "in_progress")`、ダイアログの名前「進行中にするには」、ボタン「進行中にする」、`status` は `in_progress`）
- Task 7 Step 21: 同じ spec で、`points`（number のプロパティ）が `hh show` の JSON では文字列 `"5"` で返る（CLI の `hh set DM-2 points=5` でも同じ）ため、`expect(after.props.points).toBe(5)` が `Received: "5"` で落ちた。`toBe("5")` に直した
- Task 7 Step 21: `views.spec.ts` の「a tree saved as a view …」が、末尾の削除の取り消しのあとサイドバーに `busy` が戻らず落ちた（`getByRole('link', { name: 'busy' })` が見つからない。`--repeat-each=12` で 12 回中 8 回、単独の 1 回は通る）。サーバーはビューの変更をポーリング（300ms ごと）で知らせるので、削除を見る前に取り消すと、2 回のポーリングのあいだに差がなくなり何も知らせない（取り消しの前に `waitForResponse` で 1 秒待つと 12 回とも通った）。製品の不具合なので、E2E で待たずに、取り消したあとにタブがビューを読み直すように直した（Step 20c を新設。E2E は取り消しの前に待たない）。直したあとは `views.spec.ts` を `--repeat-each=15` で 60 件すべて通り、直しを外すと `--repeat-each=10` で 1 回落ちる
- Task 7 Step 20b（新設）: `tree.spec.ts` の「Esc closes the input even when the task is still being made, and the row keys work」「a click on a row closes the input, also while the task is being made」が `expect(row "Alpha").toBeFocused()` で落ちた（`Received: inactive`、`pnpm exec playwright test e2e/tree.spec.ts --workers=1`。main の 79a5277 では通る）。原因は Task 2 の `useListScope` の `apiBase`・`cliBase` が `$derived` でない getter で、`base`（`workspace` から作る新しいオブジェクト）を読んでいたため、タスクを作って Store が読み直され `workspace` が同じ内容の新しいオブジェクトになるたびに、これを読む `TreeView` の `watchTree` の `$effect` が走り直し、ツリーが「読み込み中…」を挟んで行を作り直していたこと。`scope.svelte.ts` に文字列の `$derived` の `apiBase`・`cliBase` を足し、`scope.svelte.test.ts` に「ワークスペースを読み直しても効果が走り直さない」テストを足した（直す前は `expected 2 to be 1` で落ちる）。直したあと `tree.spec.ts` は全件通る
- Task 7 Step 21: `make e2e` の 2 回の結果は、`newtab.spec.ts` の「Ctrl+click on a sidebar link …」だけが落ちる（ほかは 82 件 PASS）。macOS の Chromium では Ctrl+クリックが右クリック（contextmenu）になり新しいタブが開かないため（`modifiers: ["Meta"]` に替えると通る）で、plan の変更とは無関係（main の 79a5277 でも同じに落ちる）。`modifiers: ["ControlOrMeta"]` にして macOS でも通るようにした（Step 21 に差分を足した）。`create.spec.ts` の「C on a task detail makes a child of that task」が全件の 1 回目に 1 度だけ落ちた（`dialog.getByText("DM-1")` が見つからない。詳細が読み込まれる前に `c` を押す既存のテストの競合で、単独の `--repeat-each=6` は 36 件通り、全件の 2 回目も通った）。`c` を押す前に、詳細のタイトル欄が `Parent task` になるのを待つように `create.spec.ts` を直した（Step 21 の最後に差分を足した）
- 全体の `make e2e`（3 回）: 1 回目に `live.spec.ts` の「a task added in the page does not get the mark」が1度だけ落ちた（`expect(row "Made here").not.toContainText("CLI で更新 · 今")`、`Received string: "   DM-2  Made here   CLI で更新 · 今 …"`。単独の `--repeat-each=30` は 30 件通り、2・3 回目の全件も通る）。原因はコードから読んだもので、4e-3 の変更とは関係がない: サーバーの `headMoved`（`internal/server/poll.go`）は 300ms ごとのポーリングで、前の HEAD から今の HEAD までの差を1つの `tasks.changed`（`commit` は今の HEAD）にまとめて知らせる。負荷でページの作成と CLI の `hh set` が同じポーリングの間に入ると、ページの作成したタスクも CLI のコミットの変更として届き、`TaskCache` が「CLI で更新」の印を付ける。`poll.go`・`cache.ts` は 4e-3 で触らず、main でも同じ作り。直し方（コミットごとにイベントを分けるか、イベントにコミットごとの ULID を持たせるか）はイベントの形に関わるので、この plan では直さない。利用者の決定（2026-11-03）で main への別の直し（PR #42。first parent でたどれるコミットを1つずつ知らせる）にした。テストは待ちを足して隠さない
- Task 6 のレビュー後の変更（利用者の決定、2026-11-03）: ビューの画面のレイアウト切り替えと「ビューを更新」は `PUT` ではなく `PATCH /api/views/{name}` を使う。`PUT` は無いビューを作るため、ほかのクライアントが改名・削除したあとの数百ミリ秒（`views.changed` が届くまで）に押すと、古い名前のビューを作り直してしまう。そこで `PATCH` に、`name`（改名）に加えて `PUT` と同じ `layout`・`query`・`columns`・`group_by`・`kanban_columns`・`swimlane` を任意で受けさせた（ops の `ViewUpdate`・`ViewPatch`）。送ったものだけを変え、変えた結果を保存と同じに検証し（`view.tree_order` など）、ビューが無ければ作らずに 404（`view.not_found`）にする。改名と設定を同時に送ると 400（`view.rename_with_settings`。1操作1コミットを保つため）。web は `ViewDirectory.update` を足し、404 ならその Store の一覧からビューを外すので、画面は「見つかりません」になる。CLI の `hh view save` は変えない
- Task 4（レビューでの直し）: カンバンの事前の判定が `TaskView.project` で scoped を引いていたため、サーバー（`schema.For`: 自分がコンテナなら自分、近い祖先の順）と食い違った。epic の上書きが必須の項目を足すと 422、プロジェクトが禁じる遷移を epic の上書きが許すと許されている移動を断る。利用者の決定（2026-11-03）で、サーバーが `TaskView.scope` に効いている上書きの祖先の ULID を返すようにし（`ops.scopeOf`、`Schema.ScopeOf`。`api/openapi.yaml` と `web/core/api/schema.d.ts` は `go generate` と `pnpm gen` で更新）、`drop.ts` はそれで引く。あわせて、ドロップを直列にし（古い取得の結果と、画面が消えたあとのトースト・状態を捨てる）、読めなかったスキーマの警告は読めたら消え、列の「＋」は `status` と一緒に `category` を渡し、カードの詳細の行は中身があるときだけ出し、担当者の行は表示名で並べ、リテラルの値をトークンに替えた（`24px`・`18px` に合うトークンは `tokens.css` になく、ほかの部品も px で書いているのでそのまま）。
- Task 2 のレビュー後の変更（d7436b0）: `ViewDirectory` は、書き込みと重なって始まった取得の結果を捨てると一覧が古いままになるため、書き込みのあとに読み直し、Store がマウントから外れても取得の通し番号（seq）を残す（付け直したときに古い取得を受け取らない）。クエリの URL の `'` は `URLSearchParams` で組み、`encodeURIComponent` の結果と食い違わないようにした。
- Task 3 のレビュー後の変更（d717a41）: スキーマを待つキーは、押した行が選ばれたままのときだけ動かす（待つあいだに別の行へ移ると、別の行に効いてしまうため）。下の入力は、ワークスペースを読み終えるまで使えないようにし、そのときの Store・ワークスペースを引く。
- Task 4 のレビュー後の変更（8f106fd、87a69a0、7f1a148）: サーバーの `scope` の祖先は `Index.Ancestors` でインデックスの検査と同じ歩き方に揃え、読めない祖先で止める（8f106fd）。web の `TaskCache` はスキーマの変更（`schema.changed`）でタスクを取り直し、ドロップの失敗で列を止めず（87a69a0）、ドロップを直列にして古い取得の結果と画面が消えたあとのトースト・状態を捨て、読めたスキーマの警告を消し、列の「＋」は `status` と一緒に `category` を渡し、カードの詳細の行（meta row）は中身があるときだけ出し、スイムレーンの担当者は表示する名前で並べ、同じならキーで並べる（7f1a148）。
- Task 5 のレビュー後の変更: 編集中の行は並びとグループに固定し、クエリから外れてもフォーカスが外れるまで元の位置に残す（af4b511、2111fd1）。サーバーが拒んだ編集は `SaveController.takeRefused` で取り下げてトーストに理由を出し（9232bfe、af4b511）、入力から出たときは取り下げた編集の値をサーバーの値に戻す。`FieldInput` のフォーカスは `$state` にする（2e637ed）。親の行は個別の `watchDetail` ではなく、クエリと同じ `watchTree(withoutOrder(q))` 1 本で読み、親を出すときだけ開く（0986588）。条件は宣言順に揃え（e79a717）、`sort:`・`group:` を外したクエリで読み、ツリーが答えるまで新しい行の親を読み込み中にする（b969d3a）。`Menu` の項目を `menuitemcheckbox` にでき、列メニューのチェックの状態をスクリーンリーダーに渡す（cee50c6）。行に `aria-rowcount`・`aria-rowindex` を付けた（5006d77）。読めたスキーマの警告は消し、外した列の並び替えを持ち越さない（5112227）。
- Task 6 のレビュー後の変更: `ViewSave` の保存先は書き込める Store だけにし、ビューを読めていない間は保存しない。失敗は `failed()` で出す（67e5cac）。改名のとき、開いているビューの未保存の変更を新しい名前の画面へ持ち越す（2de26fe）。ツリーにできない保存済みビュー（クエリに `sort:`・`group:` がある）はツリーのタブを無効にして理由を伝え、カンバン・テーブルの `saved` prop で設定を保存するコマンドにビューの名前と Store を出す（e554641、26f8043）。更新の 404 でビューを外すのは `view.not_found` のときだけにした（26f8043）。
- Task 7 のレビュー後の変更（5af6e54）: 「もしかして」は誤った値だけを直し、ほかの語は触らない。パレットの「このクエリで」は調べ終えたクエリにだけ出す（`foundFor`）。プロジェクトの見出しは読み込み前も出す（題は空、バッジと ID はなし。タブは ULID だけで描けるので遅れて出ない）。
- 最終レビューでの直し（利用者の決定、2026-11-03）: (1) 文書の `PUT` を、現在の `PATCH /api/views/{name}` に合わせた（概要 plan の細部 48・50、D6）。(2) カンバン・テーブルの「ビューとして保存」のコマンドは、保存済みビューなら `--query` なし（コピーできる）、そうでなければ `--query` つき（コピー不可）に揃えた（細部 49。1a272bb）。(3) テーブルの「一致なし」は、クエリが空なら解除の操作を出さない（ツリー・カンバンと同じ。1a272bb）。(4) `view.*` の偽のエラーはサーバーと同じ 400 にし、`PUT` と書いたままのコメントを `PATCH` にした（da83e9c）。(5) カテゴリの定義（型・一覧・ラベル・`categoryOf`）を `core/categories.ts` にまとめ、`status-order.ts` の `STATUS_CATEGORIES`、`workspace/counts.ts`、`list/suggest.ts` の重複と `board.ts` の `categoryIn` を消した（8c55aff）。(6) `defaultStatus` は `defaultStatusIn` を呼ぶだけなので消した（2119e8a）。(7) `TableView` のスコープの名前をツリー・カンバンに揃え（prop `view` を `scope`、`useListScope` の結果を `ls`）、空の画面・見つからない画面に渡す値は `useListScope` の `crumbTitle`・`emptyProps`・`notFoundProps` に集めた（c730209、1a272bb）。(8) フィルタをタブを持つページ（`WorkspacePage`・`ProjectPage`）に置き、ツリー・カンバン・テーブルのタブをまたいで残す（D7）。これまでは `TreeTab` と `ListTab` が別々に持ち、ツリーとほかのタブを行き来すると `-status:canceled` に戻っていた（67b2e7c。E2E は `table.spec.ts`）。(9) `PATCH /api/views/{name}` の operationId を、改名だけではなくなったので `rename-view` から `update-view` にし、`api/openapi.yaml` と `web/core/api/schema.d.ts` を作り直した（c69837a）。
