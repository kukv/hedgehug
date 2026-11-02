# フェーズ 4e-2: ワークスペースと骨組み Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ワークスペースの概要と設定、ワークスペースの追加・改名・接頭辞の変更・削除、別のワークスペースへの移動、設定の画面の骨組み（ルートとタブ、設定のナビ、サイドバーの入口）、コマンドパレットのコマンド、undo を GUI で動かす。

**Architecture:** フェーズ 4e の4回のうちの2回目。決定事項と spec にない細部は概要 `docs/superpowers/plans/2026-11-03-phase4e-screens.md` にあり（この回は細部 13〜35）、各 Task はそこを前提にする（細部の番号はそこを指す）。web だけの回で、Go は 4e-1 で入った API（`WorkspaceView.deleted`・`prefix_set`、`workspace.not_empty` の `args.tasks`、`TaskView.project`、`POST /api/tasks/{id}/move` の `workspace` と応答の `detached`、`POST /api/undo`）を使う。フロントとバックは REST で分け（`web/core` はフレームワークに依存しない）、画面は Svelte 5 の runes。Task 1 でルートとタブ・見出し・設定のレイアウトを作り、Task 2 のダイアログと設定の画面、Task 3 の概要・パレット・undo を作り、Task 4 で Task 1 が用意した場所に結線する。

**Tech Stack:** TypeScript、Svelte 5（runes）、Vite、Vitest と @testing-library/svelte、Playwright（`web/e2e`、`harness.ts`）、Biome、svelte-check。

**Spec:** `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（「画面と範囲」）と、概要 `docs/superpowers/plans/2026-11-03-phase4e-screens.md`、4d の概要 `docs/superpowers/plans/2026-11-02-phase4d-web.md`。

**この回の Task:** Task 1（ルートとタブ・見出し・設定のレイアウトとナビ・サイドバーの歯車・App の振り分け）、Task 2（ワークスペースのダイアログと、設定のタブ・設定のワークスペース一覧。2a・2b の2コミット）、Task 3（ワークスペースの概要・パレットのコマンド・undo・削除の取り消し）、Task 4（結線とサイドバーのワークスペースの行の「…」）。番号の順に実行する（Task 2 と 3 は Task 4 の結線まで画面に出ないので、E2E の全件は Task 4 で流す）。末尾の「決めたこと」に、この plan を組むときに決めた点をまとめた。

**着手の前に:** main を取り込み、各 Task の Files にある関数・型がまだその場所と名前であることを確かめる（動いていれば、コードをその時点の形に合わせ直す。API の形と画面の言葉は変えない）。Task 1・2 のコードは、4e-1 マージ後の `web/`（main の 0dffaec）の写しに Step の差分を当てて、`pnpm fix`・`svelte-check`・`biome ci`・`vitest run` を通したもの（Task 1 は E2E も Chromium で通した）。Task 3 のコードは、Task 1・2 のファイルの代わりに同じ props のスタブを置いて確かめたもの。Task 4 は、Task 2・3 の実物の代わりに同じ props の空の部品を置いて確かめたもの。いずれも、この plan を組むときに足した部分（接頭辞を変えた日の表示 `prefixSet`、パレットの「別のワークスペースへ移動…」、削除のトーストの取り消し、サイドバーの「…」の Task 4 への移し替え）は、まだ実行していない（名前が違えば実在のものに合わせる）。

## Global Constraints

4d の概要 plan と 4e の概要 plan の Global Constraints をすべて引き継ぐ。

- 画面の文言はすべて日本語。コードの識別子・コメントは英語（既存のコードと同じ）
- コンポーネントは色・寸法の 16 進数や px を直接書かず、`web/app/styles/tokens.css` の CSS 変数を使う（Foundations: 「色はトークン名で実装し、画面に 16 進数を直書きしない」）
- ステータスは色だけでなく形で区別する（Foundations のステータスの形）
- 行とカードは ID だけを出し、ULID は ID にカーソルを合わせたときに出してコピーできるようにする。タスク詳細では ULID を常に出す。「キー未確定」・短縮 ID・「次の同期で確定」は作らない
- フォントは `@fontsource` で同梱し、外部（Google Fonts など）に通信しない
- ダークモードは作らない
- キーボード: ⌘K コマンドパレット、C タスクを作成、⇧⌘S 今すぐ同期、Enter 下に追加（ツリー）、Tab / ⇧Tab 字下げ・戻す、⌥↑↓ 並べ替え、Space 完了、S ステータス、K 種類、⌘↵ 詳細を開く・作成。Windows と Linux では ⌘ を Ctrl にする
- 保存は入力が 2 秒止まったとき、またはフォーカスが外れたときに `PATCH`。1操作1コミット
- トーストは左下に 6 秒、取り消し付きは 10 秒。取り消しにくい操作だけ確認ダイアログ。undo は確認しない
- ダイアログとフィルタ欄の足元には同じ操作の `hh` コマンドを出し、コピーできるようにする
- `svelte-check` は警告ゼロ、`biome ci` は指摘ゼロ
- spec・設計と違う挙動にしたくなったら、実装を止めて報告する（自分で決めない）
- Go のテストは `-race` を含めて通す。Go のジョブは `web/dist` なし（`.gitkeep` だけ）で通る（この回は Go を触らない）
- まだない画面への入口は出さない（4e の回の中でも、その画面を作る回まで）。この回で出さないのは、マイタスク・通知・アクティビティ・保存済みビューのサイドバーの行、カンバン・テーブルのタブ、プロジェクトの設定タブ、設定の同期と保守・スキーマ。後のフェーズの画面（ガント・バックログ・スプリント・リリース・レポート、アカウント・Store・自動化の設定）のタブと項目も出さない。出さないことはテストで確かめる
- キャンバスの CLI の表記が実際のコマンドと違うときは実際のコマンドに合わせる（例: `hh workspace prefix` → `hh workspace set-prefix`、`hh purge --moved` は存在しない）
- コミットのメッセージの末尾は `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`

---
### Task 1: ルートとタブ・見出し・設定のレイアウトとナビ・サイドバーの入口・App の振り分け

4e-2 の骨組み。ルートに「タブ」と「設定のページ」を足し、ワークスペースの見出し（`WorkspaceHeader`）とタブ（`ui/Tabs`）、設定の左ナビと全体のレイアウト、サイドバーの入口（歯車）、App の振り分けを作る。概要（Task 3）と、ワークスペースの設定・ダイアログ（Task 2）の画面は、この Task が用意した場所に、Task 4 が結線する。4d の画面（ツリー・プロジェクト・タスク詳細）は新しいルートに合わせて直す。

**この Task は単独で通る**（`pnpm check`・`pnpm test`・`make e2e` が通り、コミットできる）。概要・ワークスペースの設定・設定のワークスペース一覧の画面（Task 2・3）は、まだ App につながない（概要のタブは見出しだけ、設定のタブは見出しだけ、設定のページは本文が空）。つなぐのは Task 4。

**Files:**
- Create: `web/app/ui/Tabs.svelte`、`web/app/ui/tabs.test.ts`
- Create: `web/app/workspace/WorkspaceHeader.svelte`、`WorkspaceHeader.test.ts`、`WorkspacePage.svelte`、`WorkspacePage.test.ts`
- Create: `web/app/settings/SettingsLayout.svelte`、`SettingsNav.svelte`、`SettingsRoute.svelte`、`settings.test.ts`
- Create: `web/e2e/routes.spec.ts`
- Modify: `web/app/router.ts`、`router.test.ts`（タブと設定のルート）
- Modify: `web/core/stores.ts`、`stores.test.ts`（`WorkspaceEntry` に `tasks`・`deleted`・`prefixSet`）
- Modify: `web/app/tree/TreeView.svelte`、`TreeView.test.ts`（ワークスペースの見出しとタブを外し、プロジェクトのタブを `Tabs` に）
- Modify: `web/app/sidebar/Sidebar.svelte`、`Sidebar.test.ts`（歯車）
- Modify: `web/app/App.svelte`、`App.test.ts`、`gates.test.ts`（振り分け、設定のあいだのレイアウト）
- Modify（`tab` を足すだけ）: `web/app/detail/TaskDetail.svelte`、`web/app/create/CreateStore.svelte`、`CreateProject.svelte`、`context.test.ts`、`web/app/palette/CommandPalette.svelte`、`search.test.ts`、`web/app/detail/testkit.ts`
- Modify: `web/e2e/harness.ts`（`get`・`treePath`・`openTree`）、`tree.spec.ts`、`live.spec.ts`、`create.spec.ts`、`newtab.spec.ts`（ツリーの URL に合わせる）
- Test: 上の各 `*.test.ts`・`routes.spec.ts`

**Interfaces:**
- Consumes（4d の最終形）: `core()`・`AppCore`、`useSubscribe`、`renderWithCore`・`fakeCore`（`test-utils.ts`）、`router.route`・`router.go`・`router.href`・`onLinkClick`・`hrefOf`、`StoreDirectory.stores()/workspaces()/workspace(store, ulid)/loaded()/tone(store)`、`unwrap(api.GET("/api/health"))`（`HealthOutputBody`: `{ pid: number; version: string }`）、`Icon`（`settings`・`more`・`plus`・`workspace`）、`StoreBadge`、`Menu`（`anchor`・`items`・`label`・`onselect`・`onclose`）、`Empty`（`notfound`）、`TreeView`（`view`）、`Hedgehug`（`web/e2e/harness.ts`）・`row`（`screens.ts`）
- Produces:
  - `web/app/router.ts`（Task 2・3 と以降の回が使う）:
    ```ts
    export type WorkspaceTab = "overview" | "tree" | "settings";   // 4e-3 で kanban・table を足す
    export type ProjectTab = "tree";                               // 4e-3 で kanban・table、4e-4 で settings
    export type SettingsPage = { page: "workspaces"; store: string }; // 4e-4 で general・schema。store は "" なら「最初の Store」
    export type Route =
      | { name: "home" }
      | { name: "workspace"; store: string; ulid: string; tab: WorkspaceTab }
      | { name: "project"; ulid: string; tab: ProjectTab }
      | { name: "task"; ulid: string }
      | ({ name: "settings" } & SettingsPage);
    ```
    パスは `/workspaces/<store>/<ulid>`（overview）・`…/tree`・`…/settings`、`/projects/<ulid>`（`/projects/<ulid>/tree` も同じ）、`/settings/workspaces/<store>`、`/settings`（`store: ""`）。知らないタブ・パスは home
  - `web/core/stores.ts`: `WorkspaceEntry` に `tasks: number`（削除済みを除く）・`deleted: number`（削除済み）・`prefixSet: string`（接頭辞を最後に変えた時刻。`WorkspaceView.prefix_set` の ISO 日時）
  - `web/app/ui/Tabs.svelte`: props `{ items: TabItem[]; end?: TabEnd; label?: string }`。`export type TabItem = { label: string; href: string; current: boolean }`、`export type TabEnd = TabItem & { icon?: IconName }`（`import type { TabItem, TabEnd } from "../ui/Tabs.svelte"`）。タブは `<a href>`（`onLinkClick`）で、`current` のものに `aria-current="page"`。`label` は `<nav>` の名前（既定は「ビューの切り替え」）。横の余白（`--gutter`）と下線を持つ
  - `web/app/workspace/WorkspaceHeader.svelte`: props `{ entry: WorkspaceEntry; tab: WorkspaceTab; actions?: Snippet }`。Store のバッジ・アイコン・`<h1>` の名前・接頭辞・「ワークスペース」と、`Tabs`（概要・ツリー、右端に設定）。`actions` は見出しの右。横の余白は `--gutter`（本文の余白は各画面が `--gutter` で持つ）
  - `web/app/workspace/WorkspacePage.svelte`: props `{ store: string; ulid: string; tab: WorkspaceTab }`。`StoreDirectory` から `entry` を引き、概要は `WorkspaceOverview`（見出しごと）、ツリーと設定は `WorkspaceHeader` の下に `TreeView`／`WorkspaceSettings`。なければ `Empty`（notfound）、読み込み前は「読み込み中…」
  - `web/app/settings/SettingsLayout.svelte`: props `{ page: SettingsPage; children?: Snippet }`（左に `SettingsNav`、右に本文の `<section aria-label="設定の本文">`。本文の見出しは持たない）。`web/app/settings/SettingsNav.svelte`: props `{ page: SettingsPage }`（`<nav aria-label="設定のナビゲーション">`。Store が 2 つ以上なら「設定する Store」の選択、脚注に版）。`web/app/settings/SettingsRoute.svelte`: props `{ page: SettingsPage }`（`/settings` を最初の Store に置き換え、`SettingsLayout` の中に設定のページを置く）
  - `web/e2e/harness.ts`: `Hedgehug.get<T>(path)`、`treePath()`（最初の Store の最初のワークスペースのツリーのパス）、`openTree(page)`
  - 画面の言葉（E2E が使う）: サイドバーの歯車は link「設定」。ワークスペースのタブは nav「ビューの切り替え」の link「概要」「ツリー」「設定」。設定のナビは nav「設定のナビゲーション」

**この Task で決めたこと**
- ワークスペースへのリンク（サイドバーの行・タスク詳細のパンくず・パレットの「<名前> を開く」・プロジェクトの見出しのリンク）は概要を開く（以前はツリー）。ツリーへは見出しのタブか、概要の「ツリーで開く」。Store を作った直後だけは、最初のタスクを足す行のあるツリーへ行く
- `/` は最初のワークスペースの概要で、URL は `/` のまま（リダイレクトしない）
- ワークスペースの見出しとタブは `WorkspaceHeader` が持ち、ワークスペースの `TreeView` は自分の見出しとタブを出さない。プロジェクトの見出し（ワークスペース › プロジェクトのパンくず、Store のバッジ、概要へのリンク）は 4d の `TreeView` のままで、タブだけを `Tabs` に替える（細部 26）
- 設定のあいだは、サイドバーを `hidden` にして設定のナビに替える（キャンバスの「Hedgehug に戻る」）。サイドバーは外さない。アプリのキー（⌘K・⇧⌘S）をサイドバーと同期カードが登録しているため
- 歯車は `/settings` を開く（概要 plan の細部 18 の `/settings/general` は 4e-4 で「同期と保守」ができてから）
- 4e-2 で出さない入口（マイタスク・通知・アクティビティ・保存済みビュー、カンバン・テーブルのタブ、プロジェクトの設定タブ、設定の同期と保守・スキーマ）は、テストで「出さない」ことを確かめる

**確かめたこと（この Task の書き手）**: この節のコードは、4e-1 マージ後の `web/`（main の 0dffaec）の写しに Step の差分をそのまま当てて、`pnpm fix`・`svelte-check --fail-on-warnings`・`biome ci`・`vitest run` を通した。E2E は、その写しを `vite build` して Go のバイナリに埋め込み、既存 spec の全件と `routes.spec.ts` の 5 本を Chromium で通した。`prefixSet` の足しは、この plan を組むときに足したもので、写しでは通していない（`stores.ts` の 1 項目と、型を付けたフィクスチャへの 1 行ずつ）。


- [ ] **Step 1: ルートの失敗するテストを書く**

`web/app/router.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
    ["/workspaces/shared/01HWS", { name: "workspace", store: "shared", ulid: "01HWS" }],
    ["/workspaces/org-b%2Fops/01HWS", { name: "workspace", store: "org-b/ops", ulid: "01HWS" }],
    ["/projects/01HPR", { name: "project", ulid: "01HPR" }],
    ["/tasks/01HTK/", { name: "task", ulid: "01HTK" }],
    ["/nothing/here", { name: "home" }],
    ["/workspaces/shared", { name: "home" }],
```

変更後:
```ts
    [
      "/workspaces/shared/01HWS",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "overview" },
    ],
    [
      "/workspaces/shared/01HWS/",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "overview" },
    ],
    [
      "/workspaces/shared/01HWS/tree",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "tree" },
    ],
    [
      "/workspaces/shared/01HWS/settings",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "settings" },
    ],
    [
      "/workspaces/org-b%2Fops/01HWS/tree",
      { name: "workspace", store: "org-b/ops", ulid: "01HWS", tab: "tree" },
    ],
    ["/workspaces/shared/01HWS/kanban", { name: "home" }],
    ["/workspaces/shared/01HWS/tree/extra", { name: "home" }],
    ["/projects/01HPR", { name: "project", ulid: "01HPR", tab: "tree" }],
    ["/projects/01HPR/tree", { name: "project", ulid: "01HPR", tab: "tree" }],
    ["/projects/01HPR/kanban", { name: "home" }],
    ["/projects/01HPR/settings", { name: "home" }],
    ["/tasks/01HTK/", { name: "task", ulid: "01HTK" }],
    ["/settings", { name: "settings", page: "workspaces", store: "" }],
    ["/settings/workspaces/shared", { name: "settings", page: "workspaces", store: "shared" }],
    [
      "/settings/workspaces/org-b%2Fops",
      { name: "settings", page: "workspaces", store: "org-b/ops" },
    ],
    ["/settings/general", { name: "home" }],
    ["/nothing/here", { name: "home" }],
    ["/workspaces/shared", { name: "home" }],
```

変更前:
```ts
    const r = { name: "workspace", store: "org-b/ops", ulid: "01HWS" } as const;
    expect(hrefOf(r)).toBe("/workspaces/org-b%2Fops/01HWS");
    expect(parseRoute(hrefOf(r))).toEqual(r);
    expect(hrefOf({ name: "home" })).toBe("/");
    expect(hrefOf({ name: "task", ulid: "01HTK" })).toBe("/tasks/01HTK");
  });
```

変更後:
```ts
    const r = { name: "workspace", store: "org-b/ops", ulid: "01HWS", tab: "overview" } as const;
    expect(hrefOf(r)).toBe("/workspaces/org-b%2Fops/01HWS");
    expect(parseRoute(hrefOf(r))).toEqual(r);
    expect(hrefOf({ name: "home" })).toBe("/");
    expect(hrefOf({ name: "task", ulid: "01HTK" })).toBe("/tasks/01HTK");
  });

  it("タブはパスの末尾に付ける。概要は付けない", () => {
    const ws = { name: "workspace", store: "shared", ulid: "01HWS" } as const;
    expect(hrefOf({ ...ws, tab: "tree" })).toBe("/workspaces/shared/01HWS/tree");
    expect(hrefOf({ ...ws, tab: "settings" })).toBe("/workspaces/shared/01HWS/settings");
    expect(hrefOf({ name: "project", ulid: "01HPR", tab: "tree" })).toBe("/projects/01HPR");
    for (const r of [
      { ...ws, tab: "tree" },
      { ...ws, tab: "settings" },
      { name: "project", ulid: "01HPR", tab: "tree" },
      { name: "settings", page: "workspaces", store: "org-b/ops" },
      { name: "settings", page: "workspaces", store: "" },
    ] as const) {
      expect(parseRoute(hrefOf(r))).toEqual(r);
    }
  });

  it("設定のパスは Store 名を符号化し、Store がなければ /settings", () => {
    expect(hrefOf({ name: "settings", page: "workspaces", store: "org-b/ops" })).toBe(
      "/settings/workspaces/org-b%2Fops",
    );
    expect(hrefOf({ name: "settings", page: "workspaces", store: "" })).toBe("/settings");
  });
```

変更前:
```ts
    expect(router.route).toEqual({ name: "project", ulid: "01HPR" });
  });
```

変更後:
```ts
    expect(router.route).toEqual({ name: "project", ulid: "01HPR", tab: "tree" });
  });

  it("別名のパスは正準の形に直して履歴に積む", () => {
    router.go("/projects/01HPR/tree");
    expect(location.pathname).toBe("/projects/01HPR");
    router.go("/workspaces/shared/01HWS/tree");
    expect(router.route).toEqual({
      name: "workspace",
      store: "shared",
      ulid: "01HWS",
      tab: "tree",
    });
  });
```

Run: `cd web && pnpm vitest run app/router.test.ts`
Expected: FAIL（`tab` のないルートが返る。`/settings` のルートがない）

- [ ] **Step 2: router.ts を書く**

タブは URL に持つ（細部 13）。概要は `/workspaces/<store>/<ULID>`（タブなし）で、`/tree`・`/settings` を付ける。4e-2 で受け付けないタブ・パス（`/kanban` など）は home に落とす。プロジェクトは 4e-2 ではツリーだけで、`/projects/<ULID>` と `/projects/<ULID>/tree` は同じ画面（`router.go` は前者の形に直す）。`/settings` は Store を名指さないので `store: ""` のルートにし、画面が最初の Store に置き換える（Step 9）。

`web/app/router.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
export type Route =
  | { name: "home" }
  | { name: "workspace"; store: string; ulid: string }
  | { name: "project"; ulid: string }
  | { name: "task"; ulid: string };
```

変更後:
```ts
// The tabs the screens have so far; 4e-3 adds kanban and table, 4e-4 the settings of a project.
export type WorkspaceTab = "overview" | "tree" | "settings";
export type ProjectTab = "tree";
// store is "" for /settings, which names no Store: the screen goes to the first one.
export type SettingsPage = { page: "workspaces"; store: string };

export type Route =
  | { name: "home" }
  | { name: "workspace"; store: string; ulid: string; tab: WorkspaceTab }
  | { name: "project"; ulid: string; tab: ProjectTab }
  | { name: "task"; ulid: string }
  | ({ name: "settings" } & SettingsPage);
```

変更前:
```ts
  const [head, a, b] = seg;
  if (head === undefined) return { name: "home" };
  if (head === "workspaces" && seg.length === 3 && a && b) {
    return { name: "workspace", store: a, ulid: b };
  }
  if (head === "projects" && seg.length === 2 && a) return { name: "project", ulid: a };
  if (head === "tasks" && seg.length === 2 && a) return { name: "task", ulid: a };
  return null;
```

変更後:
```ts
  const [head, a, b, c] = seg;
  if (head === undefined) return { name: "home" };
  if (head === "workspaces" && a && b) {
    if (seg.length === 3) return { name: "workspace", store: a, ulid: b, tab: "overview" };
    if (seg.length === 4 && (c === "tree" || c === "settings")) {
      return { name: "workspace", store: a, ulid: b, tab: c };
    }
    return null;
  }
  if (head === "projects" && a && (seg.length === 2 || (seg.length === 3 && b === "tree"))) {
    return { name: "project", ulid: a, tab: "tree" };
  }
  if (head === "tasks" && seg.length === 2 && a) return { name: "task", ulid: a };
  if (head === "settings" && seg.length === 1) {
    return { name: "settings", page: "workspaces", store: "" };
  }
  if (head === "settings" && seg.length === 3 && a === "workspaces" && b) {
    return { name: "settings", page: "workspaces", store: b };
  }
  return null;
```

変更前:
```ts
    case "workspace":
      return `/workspaces/${encodeURIComponent(r.store)}/${encodeURIComponent(r.ulid)}`;
    case "project":
      return `/projects/${encodeURIComponent(r.ulid)}`;
    case "task":
      return `/tasks/${encodeURIComponent(r.ulid)}`;
```

変更後:
```ts
    case "workspace": {
      const base = `/workspaces/${encodeURIComponent(r.store)}/${encodeURIComponent(r.ulid)}`;
      return r.tab === "overview" ? base : `${base}/${r.tab}`;
    }
    case "project":
      return `/projects/${encodeURIComponent(r.ulid)}`;
    case "task":
      return `/tasks/${encodeURIComponent(r.ulid)}`;
    case "settings":
      return r.store === "" ? "/settings" : `/settings/workspaces/${encodeURIComponent(r.store)}`;
```

Run: `cd web && pnpm vitest run app/router.test.ts`
Expected: PASS（`pnpm check` は Step 3 の直しが済むまで通らない）

- [ ] **Step 3: `tab` のなかったルートを作っている箇所を直す**

`workspace` と `project` のルートは `tab` が必須になった。ワークスペースへのリンクは概要（`overview`）を指す（サイドバー・タスク詳細のパンくず・パレットの「<名前> を開く」。以前はツリーだった。ツリーへは見出しのタブか、概要の「ツリーで開く」）。Store を作った直後だけは、最初のタスクを足す行のあるツリーへ行く（`e2e/onboarding.spec.ts` がそう期待している）。

`web/app/detail/TaskDetail.svelte` を変える（`pnpm fix` が整形を直す）:

変更前（2 か所すべて）:
```svelte
ulid: s.workspace.ulid })
```

変更後:
```svelte
ulid: s.workspace.ulid, tab: "overview" })
```

`web/app/create/CreateStore.svelte` を変える（`pnpm fix` が整形を直す）:

変更前:
```svelte
      router.go(hrefOf({ name: "workspace", store: r.name, ulid: r.workspace.ulid }));
```

変更後:
```svelte
      // The tree, where the first task is added.
      router.go(hrefOf({ name: "workspace", store: r.name, ulid: r.workspace.ulid, tab: "tree" }));
```

`web/app/create/CreateStore.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前（2 か所すべて）:
```ts
expect(location.pathname).toBe("/workspaces/me/01HWSME");
```

変更後:
```ts
expect(location.pathname).toBe("/workspaces/me/01HWSME/tree");
```

`web/app/create/CreateProject.svelte` を変える（`pnpm fix` が整形を直す）:

変更前:
```svelte
ulid: r.task.ulid }) }
```

変更後:
```svelte
ulid: r.task.ulid, tab: "tree" }) }
```

`web/app/palette/CommandPalette.svelte` を変える（`pnpm fix` が整形を直す）:

変更前:
```svelte
workspace: (store, ulid) => router.href({ name: "workspace", store, ulid }),
```

変更後:
```svelte
workspace: (store, ulid) => router.href({ name: "workspace", store, ulid, tab: "overview" }),
```

変更前:
```svelte
project: (ulid) => router.href({ name: "project", ulid }),
```

変更後:
```svelte
project: (ulid) => router.href({ name: "project", ulid, tab: "tree" }),
```

`web/app/create/context.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前（3 か所すべて）:
```ts
route: { name: "project", ulid: "UP" }
```

変更後:
```ts
route: { name: "project", ulid: "UP", tab: "tree" }
```

変更前（2 か所すべて）:
```ts
{ name: "workspace", store: "shared", ulid: "W2" }
```

変更後:
```ts
{ name: "workspace", store: "shared", ulid: "W2", tab: "tree" }
```

- [ ] **Step 4: `WorkspaceEntry` に件数と接頭辞を変えた日を足す（テストが先）**

概要の件数（細部 20）・設定の削除の可否（細部 25）・設定の一覧（細部 28）は `WorkspaceView` の `tasks`（削除済みを除く）と `deleted`（削除済み）を使う。`StoreDirectory` が持つ `WorkspaceEntry` にこの 2 つと、`prefix_set`（接頭辞を最後に変えた日。設定のタブと一覧が出す。細部 24 の前提「`prefix_set` が API にない」は誤りで、`WorkspaceView.prefix_set` は必須の項目）を `prefixSet` として足し、変わったら購読者に知らせる（`sameEntry` に入れる）。

`web/core/stores.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
  prefix_set: "2026-11-01T00:00:00Z",
  tasks: 0,
});
```

変更後:
```ts
  prefix_set: "2026-11-01T00:00:00Z",
  tasks: 0,
  deleted: 0,
});
```

変更前:
```ts
  it("follows the projects through the task cache", async () => {
```

変更後:
```ts
  it("carries the counts of the tasks and of the deleted ones and the day of the prefix, and tells when they change", async () => {
    const first = baseWorld();
    first.spaces.shared = [{ ...ws("W1", "api", "API"), tasks: 5, deleted: 2 }];
    const { dir, conn, world } = setup(first);
    await vi.waitFor(() => expect(dir.workspace("shared", "W1")?.tasks).toBe(5));
    expect(dir.workspace("shared", "W1")?.deleted).toBe(2);
    expect(dir.workspace("shared", "W1")?.prefixSet).toBe("2026-11-01T00:00:00Z");

    const fn = vi.fn();
    dir.subscribe(fn);
    world.spaces.shared = [
      { ...ws("W1", "api", "API"), tasks: 6, deleted: 3, prefix_set: "2026-11-02T00:00:00Z" },
    ];
    conn.emit("tasks.changed", changed("shared", [], "c1"));
    await vi.waitFor(() => expect(dir.workspace("shared", "W1")?.tasks).toBe(6));
    expect(dir.workspace("shared", "W1")?.deleted).toBe(3);
    expect(dir.workspace("shared", "W1")?.prefixSet).toBe("2026-11-02T00:00:00Z");
    expect(fn).toHaveBeenCalled();
  });

  it("follows the projects through the task cache", async () => {
```

Run: `cd web && pnpm vitest run core/stores.test.ts`
Expected: FAIL（`tasks`・`deleted`・`prefixSet` が `WorkspaceEntry` にない）

`web/core/stores.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
  prefix: string;
  projects: TaskView[];
};
```

変更後:
```ts
  prefix: string;
  // tasks counts the tasks that are not deleted; deleted, the ones that are.
  tasks: number;
  deleted: number;
  // prefixSet is when the prefix was last set (an ISO time).
  prefixSet: string;
  projects: TaskView[];
};
```

変更前:
```ts
    a.prefix === b.prefix &&
```

変更後:
```ts
    a.prefix === b.prefix &&
    a.tasks === b.tasks &&
    a.deleted === b.deleted &&
    a.prefixSet === b.prefixSet &&
```

変更前:
```ts
          prefix: w.id_prefix,
          projects:
```

変更後:
```ts
          prefix: w.id_prefix,
          tasks: w.tasks,
          deleted: w.deleted,
          prefixSet: w.prefix_set,
          projects:
```

型が `WorkspaceEntry` のテストの部品も直す（型を付けていない他のテストの部品は、この 3 項目がなくても通る）。Task 2・3 が足す新しいテストの部品は、自分で 3 項目を持つ:

`web/app/detail/testkit.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
  { store: "shared", tone: "purple", ulid: "W1", name: "release", prefix: "REL", projects: [] },
  { store: "shared", tone: "purple", ulid: "W2", name: "shared", prefix: "SH", projects: [] },
```

変更後:
```ts
  {
    store: "shared",
    tone: "purple",
    ulid: "W1",
    name: "release",
    prefix: "REL",
    tasks: 24,
    deleted: 1,
    prefixSet: "2026-10-29T12:00:00Z",
    projects: [],
  },
  {
    store: "shared",
    tone: "purple",
    ulid: "W2",
    name: "shared",
    prefix: "SH",
    tasks: 0,
    deleted: 0,
    prefixSet: "2026-10-29T12:00:00Z",
    projects: [],
  },
```

`web/app/palette/search.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
  prefix: "REL",
  projects: [task("P1"
```

変更後:
```ts
  prefix: "REL",
  tasks: 24,
  deleted: 1,
  prefixSet: "2026-10-29T12:00:00Z",
  projects: [task("P1"
```

Run: `cd web && pnpm vitest run core/stores.test.ts`
Expected: PASS

- [ ] **Step 5: タブ（`ui/Tabs.svelte`）**

タブはボタンではなくリンクにする（細部 13: 戻る・リンクのコピー・⌘↵ の新しいタブが効くように）。クリックは `onLinkClick` で `router.go` に回す。右端の項目（設定）は `margin-left: auto`。横の余白は `--gutter`（ツリーの見出しと同じ 28px）で、この余白と下線は `Tabs` が持つので、置く側は包まない。

`web/app/ui/tabs.test.ts` を作る:
```ts
import { cleanup, fireEvent, render, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import Tabs from "./Tabs.svelte";

beforeEach(() => history.replaceState(null, "", "/"));
afterEach(cleanup);

const items = [
  { label: "概要", href: "/workspaces/shared/W1", current: false },
  { label: "ツリー", href: "/workspaces/shared/W1/tree", current: true },
];

describe("Tabs", () => {
  it("タブはリンクで、今のタブだけ aria-current=page", () => {
    const { getByRole } = render(Tabs, { items });
    const nav = getByRole("navigation", { name: "ビューの切り替え" });
    const tree = within(nav).getByRole("link", { name: "ツリー" });
    expect(tree.getAttribute("href")).toBe("/workspaces/shared/W1/tree");
    expect(tree.getAttribute("aria-current")).toBe("page");
    expect(within(nav).getByRole("link", { name: "概要" }).getAttribute("aria-current")).toBeNull();
  });

  it("右端の項目を最後に出し、アイコンをつけられる", () => {
    const { getAllByRole, container } = render(Tabs, {
      items,
      end: {
        label: "設定",
        href: "/workspaces/shared/W1/settings",
        current: false,
        icon: "settings",
      },
    });
    const links = getAllByRole("link");
    expect(links.map((a) => a.textContent?.trim())).toEqual(["概要", "ツリー", "設定"]);
    expect(links[2]?.querySelector("svg")).toBeTruthy();
    expect(container.querySelectorAll("svg")).toHaveLength(1);
  });

  it("押すと router で移動する（ページは読み込み直さない）", async () => {
    const { getByRole } = render(Tabs, { items });
    const ev = new MouseEvent("click", { button: 0, bubbles: true, cancelable: true });
    getByRole("link", { name: "概要" }).dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
    expect(location.pathname).toBe("/workspaces/shared/W1");
    await fireEvent.click(getByRole("link", { name: "ツリー" }));
    expect(location.pathname).toBe("/workspaces/shared/W1/tree");
  });

  it("label でナビゲーションの名前を変えられる", () => {
    const { getByRole } = render(Tabs, { items, label: "プロジェクトのタブ" });
    expect(getByRole("navigation", { name: "プロジェクトのタブ" })).toBeTruthy();
  });
});
```

`web/app/ui/Tabs.svelte` を作る:
```svelte
<script lang="ts" module>
  import type { IconName } from "../icons/Icon.svelte";

  export type TabItem = { label: string; href: string; current: boolean };
  /** The tab at the right end (the settings), which can have an icon. */
  export type TabEnd = TabItem & { icon?: IconName };
</script>

<script lang="ts">
  import Icon from "../icons/Icon.svelte";
  import { onLinkClick } from "../router";

  type Props = { items: TabItem[]; end?: TabEnd; label?: string };
  let { items, end, label = "ビューの切り替え" }: Props = $props();
</script>

<!-- The tabs are links, so the back button, copying the address and ⌘/Ctrl-click work. -->
<nav class="tabs" aria-label={label}>
  {#each items as it (it.href)}
    <a
      class="tab"
      class:current={it.current}
      href={it.href}
      aria-current={it.current ? "page" : undefined}
      onclick={(e) => onLinkClick(e, it.href)}
      >{it.label}</a
    >
  {/each}
  {#if end}
    <a
      class="tab end"
      class:current={end.current}
      href={end.href}
      aria-current={end.current ? "page" : undefined}
      onclick={(e) => onLinkClick(e, end.href)}
    >
      {#if end.icon}
        <Icon name={end.icon} size={14} />
      {/if}
      {end.label}
    </a>
  {/if}
</nav>

<style>
  .tabs {
    display: flex;
    gap: var(--space-4);
    padding: var(--space-10) var(--gutter) 0;
    border-bottom: var(--hairline) solid var(--line);
  }
  .tab {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    min-height: 2.25rem;
    padding: 0 var(--space-12);
    margin-bottom: calc(var(--hairline) * -1);
    border-bottom: 2px solid transparent;
    color: var(--muted);
    font-size: var(--fs-13);
    text-decoration: none;
  }
  .tab:hover {
    color: var(--ink);
  }
  .tab.current {
    border-bottom-color: var(--ink);
    color: var(--ink);
    font-weight: 600;
  }
  .end {
    margin-left: auto;
  }
</style>
```

Run: `cd web && pnpm fix && pnpm vitest run app/ui/tabs.test.ts`
Expected: PASS

- [ ] **Step 6: ワークスペースの見出し（`workspace/WorkspaceHeader.svelte`）**

Store のバッジ・アイコン・名前・接頭辞・「ワークスペース」の見出しと、概要・ツリーのタブ、右端の設定。`actions` は見出しの右（Task 3 の概要が「タスク」「プロジェクト」の作成ボタンを入れる）。タブのリンク先は `router.href` で作る。カンバンとテーブルは 4e-3 でツリーと設定の間に足す。

`web/app/workspace/WorkspaceHeader.test.ts` を作る:
```ts
import { cleanup, render, within } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { afterEach, describe, expect, it } from "vitest";
import type { WorkspaceEntry } from "../../core/stores";
import WorkspaceHeader from "./WorkspaceHeader.svelte";

afterEach(cleanup);

const entry: WorkspaceEntry = {
  store: "shared",
  tone: "purple",
  ulid: "W1",
  name: "release",
  prefix: "REL",
  tasks: 24,
  deleted: 1,
  prefixSet: "2026-10-29T12:00:00Z",
  projects: [],
};

describe("WorkspaceHeader", () => {
  it("Store のバッジ・名前・接頭辞・見出しを出す", () => {
    const { getByRole, getByText } = render(WorkspaceHeader, { entry, tab: "overview" });
    expect(getByRole("heading", { level: 1, name: "release" })).toBeTruthy();
    expect(getByText("REL")).toBeTruthy();
    expect(getByText("ワークスペース")).toBeTruthy();
    expect(getByText("shared")).toBeTruthy();
  });

  it("タブは概要・ツリーと右端の設定。今のタブだけ aria-current", () => {
    const { getByRole } = render(WorkspaceHeader, { entry, tab: "tree" });
    const nav = getByRole("navigation", { name: "ビューの切り替え" });
    const links = within(nav).getAllByRole("link");
    expect(links.map((a) => a.textContent?.trim())).toEqual(["概要", "ツリー", "設定"]);
    expect(links.map((a) => a.getAttribute("href"))).toEqual([
      "/workspaces/shared/W1",
      "/workspaces/shared/W1/tree",
      "/workspaces/shared/W1/settings",
    ]);
    expect(links.map((a) => a.getAttribute("aria-current"))).toEqual([null, "page", null]);
  });

  it("設定のタブのとき設定が今のタブ", () => {
    const { getByRole } = render(WorkspaceHeader, { entry, tab: "settings" });
    expect(getByRole("link", { name: "設定" }).getAttribute("aria-current")).toBe("page");
  });

  it("actions を見出しの右に出す", () => {
    const actions = createRawSnippet(() => ({ render: () => "<button>タスク</button>" }));
    const { getByRole } = render(WorkspaceHeader, { entry, tab: "overview", actions });
    expect(getByRole("button", { name: "タスク" })).toBeTruthy();
  });
});
```

`web/app/workspace/WorkspaceHeader.svelte` を作る:
```svelte
<script lang="ts">
  import type { Snippet } from "svelte";
  import type { WorkspaceEntry } from "../../core/stores";
  import Icon from "../icons/Icon.svelte";
  import { router, type WorkspaceTab } from "../router";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";

  type Props = { entry: WorkspaceEntry; tab: WorkspaceTab; actions?: Snippet };
  let { entry, tab, actions }: Props = $props();

  const at = (t: WorkspaceTab) =>
    router.href({ name: "workspace", store: entry.store, ulid: entry.ulid, tab: t });
  // 4e-3 puts the kanban and the table between the tree and the settings.
  const items = $derived([
    { label: "概要", href: at("overview"), current: tab === "overview" },
    { label: "ツリー", href: at("tree"), current: tab === "tree" },
  ]);
  const end = $derived({
    label: "設定",
    href: at("settings"),
    current: tab === "settings",
    icon: "settings" as const,
  });
</script>

<header class="head">
  <div class="title">
    <StoreBadge store={entry.store} tone={entry.tone} />
    <Icon name="workspace" size={16} />
    <h1>{entry.name}</h1>
    <span class="prefix" title="ID の接頭辞">{entry.prefix}</span>
    <span class="eyebrow">ワークスペース</span>
  </div>
  {#if actions}
    <div class="actions">{@render actions()}</div>
  {/if}
</header>
<Tabs {items} {end} />

<style>
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
  .prefix {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .eyebrow {
    color: var(--label);
    font-size: var(--fs-11);
    font-weight: 600;
    letter-spacing: 0.04em;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-8);
  }
</style>
```

Run: `cd web && pnpm fix && pnpm vitest run app/workspace/WorkspaceHeader.test.ts`
Expected: PASS

- [ ] **Step 7: TreeView の見出しを直す（細部 26）**

ワークスペースの見出しとタブは `WorkspaceHeader` が持つので、ワークスペースの `TreeView` は自分の見出しとタブを出さない（出すと二重になる）。プロジェクトの `TreeView` は、ワークスペース › プロジェクトのパンくず・Store のバッジ・ワークスペースの概要へのリンク（`tab: "overview"`）を今のまま持ち、1 つだけのタブを `Tabs`（`/projects/<ULID>` へのリンク）に替える。ツリーの `aria-label="<名前> のタスク"` は E2E が使うので変えない。

先にテストを足す。

`web/app/tree/TreeView.test.ts` を変える（既存の `within` を足した import と、テスト 2 本の追加。`pnpm fix` が整形を直す）:

変更前:
```ts
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";
```

変更後:
```ts
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";
```

変更前:
```ts
  it("writes the keys of the footer as the platform does", () => {
```

変更後:
```ts
  it("shows the tab of a project as a link to its tree", () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const tabs = screen.getByRole("navigation", { name: "ビューの切り替え" });
    expect(within(tabs).getByRole("link", { name: "ツリー" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("leaves the heading and the tabs of a workspace to its page", () => {
    setup();
    render(TreeView, { view: { kind: "workspace", store: "shared", ulid: "W1" } });
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
    expect(screen.queryByRole("navigation", { name: "ビューの切り替え" })).toBeNull();
    expect(screen.getByRole("tree", { name: "release のタスク" })).toBeInTheDocument();
  });

  it("writes the keys of the footer as the platform does", () => {
```

Run: `cd web && pnpm vitest run app/tree/TreeView.test.ts`
Expected: FAIL（ワークスペースの TreeView が見出しを出している）

`web/app/tree/TreeView.svelte` を変える（3 か所。3 つ目は未使用になった `.tabs`・`.tab` の CSS を消す。`svelte-check` が未使用の CSS を警告するため）:

変更前:
```svelte
  import StoreBadge from "../ui/StoreBadge.svelte";
```

変更後:
```svelte
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";
```

変更前:
```svelte
  <header class="head">
    <nav class="crumb" aria-label="パンくず">
      {#if store}
        <StoreBadge {store} tone={c.stores.tone(store)} />
      {/if}
      {#if scope.kind === "project"}
        {#if workspace}
          {@const wsHref = router.href({
            name: "workspace",
            store: workspace.store,
            ulid: workspace.ulid,
          })}
          <a class="ws" href={wsHref} onclick={(e) => onLinkClick(e, wsHref)}>
            <Icon name="workspace" size={16} />{workspace.name}
          </a>
          <span class="sep" aria-hidden="true">›</span>
        {/if}
        <KindIcon kind="project" />
        <h1>{crumbTitle}</h1>
        {#if project}
          <span class="id">{project.id}</span>
        {/if}
      {:else}
        <Icon name="workspace" size={16} />
        <h1>{crumbTitle}</h1>
        {#if workspace}
          <span class="id">{workspace.prefix}</span>
        {/if}
      {/if}
    </nav>
  </header>

  <nav class="tabs" aria-label="ビューの切り替え">
    <span class="tab" aria-current="page">ツリー</span>
  </nav>
```

変更後:
```svelte
  <!-- The workspace has its own heading and tabs (WorkspaceHeader), which sit above this view. -->
  {#if scope.kind === "project"}
    <header class="head">
      <nav class="crumb" aria-label="パンくず">
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
          <span class="sep" aria-hidden="true">›</span>
        {/if}
        <KindIcon kind="project" />
        <h1>{crumbTitle}</h1>
        {#if project}
          <span class="id">{project.id}</span>
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
```

変更前:
```svelte
  .tabs {
    display: flex;
    gap: 0.25rem;
    padding: 0.625rem 1.75rem 0;
    border-bottom: var(--hairline) solid var(--line);
  }
  .tab {
    display: inline-flex;
    align-items: center;
    min-height: 2.25rem;
    padding: 0 0.75rem;
    margin-bottom: calc(var(--hairline) * -1);
    border-bottom: 2px solid var(--ink);
    color: var(--ink);
    font-weight: 600;
  }
```

変更後:
```svelte

```

Run: `cd web && pnpm fix && pnpm vitest run app/tree`
Expected: PASS（`TreeView.dnd.test.ts` などは見出しを見ていないので変わらない）

- [ ] **Step 8: ワークスペースの画面（`workspace/WorkspacePage.svelte`）**

App のワークスペースのルート（`/workspaces/<store>/<ULID>[/tab]` と `/`）が置く 1 枚。`StoreDirectory` から `entry` を引き、見出しとタブ（`WorkspaceHeader`）の下にタブの本文を置く。ワークスペースが見つからなければ 4d の `Empty`（notfound）、Store を読み込むまでは「読み込み中…」。見出しを誰が描くかは、タブで分ける。ツリーと設定のタブは、このページが `WorkspaceHeader` を描いて、その下に本文（ツリーは `TreeView`）を置く。概要のタブは、Task 3 の `WorkspaceOverview` が自分で `WorkspaceHeader`（`actions` に「タスク」「プロジェクト」の作成ボタン）を描く。そのためこの Task では、概要のタブは見出しだけを描き、Task 4 の Step 14 で `WorkspaceOverview` に替える。ツリーは自分の箱の中でスクロールするので、ページは窓の高さの縦並びにし、本文が見出しの残りを取る（`.main` は高さが決まっていて、4d の `TreeView` の `height: 100%` はそれに乗っている）。

`web/app/workspace/WorkspacePage.test.ts` を作る:
```ts
import { cleanup } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { WorkspaceEntry } from "../../core/stores";
import { renderWithCore } from "../test-utils";
import WorkspacePage from "./WorkspacePage.svelte";

const entry: WorkspaceEntry = {
  store: "shared",
  tone: "purple",
  ulid: "W1",
  name: "release",
  prefix: "REL",
  tasks: 24,
  deleted: 1,
  prefixSet: "2026-10-29T12:00:00Z",
  projects: [],
};

function mount(
  tab: "overview" | "tree" | "settings",
  over: { loaded?: boolean; found?: boolean } = {},
) {
  const loaded = over.loaded ?? true;
  const found = over.found ?? true;
  return renderWithCore(
    WorkspacePage,
    { store: "shared", ulid: "W1", tab },
    {
      stores: {
        workspace: (s: string, u: string) =>
          found && s === "shared" && u === "W1" ? entry : undefined,
        loaded: () => loaded,
        tone: () => "purple",
        subscribe: () => () => {},
      },
    },
  );
}

beforeEach(() => history.replaceState(null, "", "/"));
afterEach(cleanup);

describe("WorkspacePage", () => {
  it.each(["overview", "settings"] as const)("%s のタブでも見出しとタブを出す", (tab) => {
    const { getByRole } = mount(tab);
    expect(getByRole("heading", { level: 1, name: "release" })).toBeTruthy();
    expect(
      getByRole("navigation", { name: "ビューの切り替え" }).querySelector("[aria-current]")
        ?.textContent,
    ).toContain(tab === "overview" ? "概要" : "設定");
  });

  it("ワークスペースがないときは、そう言ってホームへ戻れる", () => {
    const { getByText, getByRole } = mount("overview", { found: false });
    expect(getByText("ワークスペース「W1」が見つかりません")).toBeTruthy();
    getByRole("button", { name: "ホームへ戻る" }).click();
    expect(location.pathname).toBe("/");
  });

  it("Store を読み込むまでは読み込み中", () => {
    const { getByText, queryByRole } = mount("overview", { loaded: false, found: false });
    expect(getByText("読み込み中…")).toBeTruthy();
    expect(queryByRole("heading")).toBeNull();
  });
});
```

`web/app/workspace/WorkspacePage.svelte` を作る:
```svelte
<script lang="ts">
  import { core } from "../context";
  import { router, type WorkspaceTab } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import TreeView from "../tree/TreeView.svelte";
  import WorkspaceHeader from "./WorkspaceHeader.svelte";

  type Props = { store: string; ulid: string; tab: WorkspaceTab };
  let { store, ulid, tab }: Props = $props();

  const c = core();
  const found = useSubscribe(c.stores, () => ({
    entry: c.stores.workspace(store, ulid),
    loaded: c.stores.loaded(),
  }));
  const entry = $derived(found.current.entry);
  // Spread, because Biome reads a prop called scope as the HTML attribute.
  const notFoundProps = $derived({ scope: "workspace" as const, name: ulid });
</script>

{#if entry}
  <div class="page">
    {#if tab === "overview"}
      <!-- The overview draws its own heading, which has the buttons that make a task or a project. -->
      <WorkspaceHeader {entry} tab="overview" />
    {:else}
      <WorkspaceHeader {entry} {tab} />
      <div class="body">
        {#if tab === "tree"}
          <TreeView view={{ kind: "workspace", store, ulid }} />
        {/if}
      </div>
    {/if}
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
  /* The tree scrolls in its own box, so the page is the height of the window and the body takes
     what the heading leaves. */
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
  .note {
    margin: var(--space-16) var(--space-12);
    color: var(--muted);
  }
</style>
```

Run: `cd web && pnpm fix && pnpm vitest run app/workspace`
Expected: PASS

- [ ] **Step 9: 設定のレイアウトとナビ（`settings/`）**

設定は窓全体で 1 つのレイアウト（細部 29）。左が `SettingsNav`、右が本文。`SettingsNav` は 4e-2 では「Store ごとの設定」の「ワークスペース」だけ（細部 27。「このマシン」の同期と保守は 4e-4、アカウント・Store・自動化は出さない）。Store が 2 つ以上のときだけ「設定する Store」の選択を出し（細部 15）、選んだ Store は URL（`/settings/workspaces/<store>`）に持つ。脚注は `GET /api/health` の版だけで、読めなければ出さない。`SettingsRoute` は `/settings`（`store: ""`）を最初の Store のパスに置き換える（履歴を積まない）。

`web/app/settings/settings.test.ts` を作る:
```ts
import { cleanup, fireEvent } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithCore } from "../test-utils";
import SettingsLayout from "./SettingsLayout.svelte";
import SettingsNav from "./SettingsNav.svelte";
import SettingsRoute from "./SettingsRoute.svelte";

const health = (version: string) => ({
  data: { pid: 1, version },
  response: new Response(null, { status: 200 }),
});

function core(names: string[], get = vi.fn(async () => health("0.9.0"))) {
  return {
    api: { GET: get },
    stores: {
      stores: () => names.map((name) => ({ name })),
      subscribe: () => () => {},
    },
  };
}

const page = { page: "workspaces", store: "shared" } as const;

beforeEach(() => history.replaceState(null, "", "/settings/workspaces/shared"));
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("SettingsNav", () => {
  it("ワークスペースの項目を今のページとして出し、戻るリンクがある", () => {
    const { getByRole } = renderWithCore(SettingsNav, { page }, core(["shared"]));
    const item = getByRole("link", { name: /ワークスペース/ });
    expect(item.getAttribute("href")).toBe("/settings/workspaces/shared");
    expect(item.getAttribute("aria-current")).toBe("page");
    expect(getByRole("link", { name: /Hedgehug に戻る/ }).getAttribute("href")).toBe("/");
    expect(getByRole("heading", { name: "設定" })).toBeTruthy();
  });

  it("まだない画面の項目（同期と保守・スキーマ・アカウントなど）は出さない", () => {
    const { queryByText } = renderWithCore(SettingsNav, { page }, core(["shared"]));
    for (const t of ["同期と保守", "スキーマ", "アカウント", "自動化", "マウント中の Store"]) {
      expect(queryByText(new RegExp(t))).toBeNull();
    }
  });

  it("Store が 1 つなら選択欄を出さない", () => {
    const { queryByLabelText } = renderWithCore(SettingsNav, { page }, core(["shared"]));
    expect(queryByLabelText("設定する Store")).toBeNull();
  });

  it("Store が 2 つ以上なら選択欄を出し、選ぶと Store を URL に持って移る", async () => {
    const { getByLabelText } = renderWithCore(SettingsNav, { page }, core(["shared", "org-b/ops"]));
    const select = getByLabelText("設定する Store") as HTMLSelectElement;
    expect(select.value).toBe("shared");
    expect([...select.options].map((o) => o.value)).toEqual(["shared", "org-b/ops"]);
    await fireEvent.change(select, { target: { value: "org-b/ops" } });
    expect(location.pathname).toBe("/settings/workspaces/org-b%2Fops");
  });

  it("脚注に GET /api/health の版を出す。読めなければ出さない", async () => {
    const ok = renderWithCore(SettingsNav, { page }, core(["shared"]));
    expect(await ok.findByText("Hedgehug 0.9.0")).toBeTruthy();
    cleanup();
    const bad = renderWithCore(
      SettingsNav,
      { page },
      core(
        ["shared"],
        vi.fn(async () => Promise.reject(new Error("down"))),
      ),
    );
    await vi.waitFor(() => expect(bad.core.api.GET).toHaveBeenCalled());
    expect(bad.queryByText(/^Hedgehug \d/)).toBeNull();
  });
});

describe("SettingsLayout", () => {
  it("左にナビ、右に本文を出す", () => {
    const children = createRawSnippet(() => ({ render: () => "<p>本文</p>" }));
    const { getByRole, getByText } = renderWithCore(
      SettingsLayout,
      { page, children },
      core(["shared"]),
    );
    expect(getByRole("navigation", { name: "設定のナビゲーション" })).toBeTruthy();
    expect(getByRole("region", { name: "設定の本文" })).toContainElement(getByText("本文"));
  });
});

describe("SettingsRoute", () => {
  it("Store がある /settings/workspaces/<store> はレイアウトを出し、移動しない", () => {
    const { getByRole } = renderWithCore(SettingsRoute, { page }, core(["shared", "me"]));
    expect(getByRole("navigation", { name: "設定のナビゲーション" })).toBeTruthy();
    expect(location.pathname).toBe("/settings/workspaces/shared");
  });

  it("/settings は最初の Store のワークスペースに置き換える（履歴を積まない）", async () => {
    history.replaceState(null, "", "/settings");
    const before = history.length;
    const { queryByRole } = renderWithCore(
      SettingsRoute,
      { page: { page: "workspaces", store: "" } },
      core(["me", "shared"]),
    );
    await vi.waitFor(() => expect(location.pathname).toBe("/settings/workspaces/me"));
    expect(history.length).toBe(before);
    // while it has no Store the layout is not drawn
    expect(queryByRole("navigation", { name: "設定のナビゲーション" })).toBeNull();
  });

  it("Store がまだ読めていない間は /settings のまま待つ", async () => {
    history.replaceState(null, "", "/settings");
    renderWithCore(SettingsRoute, { page: { page: "workspaces", store: "" } }, core([]));
    await new Promise((r) => setTimeout(r, 20));
    expect(location.pathname).toBe("/settings");
  });
});
```

`web/app/settings/SettingsNav.svelte` を作る:
```svelte
<script lang="ts">
  import { onMount } from "svelte";
  import { unwrap } from "../../core/api/client";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import { hrefOf, onLinkClick, router, type SettingsPage } from "../router";
  import { useSubscribe } from "../svelte-store";

  let { page }: { page: SettingsPage } = $props();

  const { api, stores } = core();
  const names = useSubscribe(stores, () => stores.stores().map((s) => s.name));
  let version = $state<string | null>(null);

  // The version is a footnote: when it cannot be read, the footnote is not shown.
  onMount(() => {
    let gone = false;
    unwrap(api.GET("/api/health"))
      .then((h) => {
        if (!gone) version = h.version;
      })
      .catch(() => undefined);
    return () => {
      gone = true;
    };
  });

  const itemHref = $derived(hrefOf({ name: "settings", page: "workspaces", store: page.store }));
  const homeHref = hrefOf({ name: "home" });

  function pick(store: string) {
    router.go(hrefOf({ name: "settings", page: page.page, store }));
  }
</script>

<nav class="nav" aria-label="設定のナビゲーション">
  <a class="back" href={homeHref} onclick={(e) => onLinkClick(e, homeHref)}>
    <span aria-hidden="true">←</span>
    Hedgehug に戻る
  </a>
  <h2>設定</h2>

  <section class="group" aria-label="Store ごとの設定">
    <span class="cap">Store ごとの設定</span>
    {#if names.current.length >= 2}
      <label class="store">
        <span class="label">設定する Store</span>
        <select value={page.store} onchange={(e) => pick(e.currentTarget.value)}>
          {#each names.current as n (n)}
            <option value={n}>{n}</option>
          {/each}
        </select>
      </label>
    {/if}
    <a
      class="item"
      class:on={page.page === "workspaces"}
      href={itemHref}
      aria-current={page.page === "workspaces" ? "page" : undefined}
      onclick={(e) => onLinkClick(e, itemHref)}
    >
      <Icon name="workspace" size={14} />ワークスペース
    </a>
    <p class="note">
      Store ごとの設定は、その Store を使う全員に共有されます（git に書かれます）。
    </p>
  </section>

  {#if version}
    <p class="foot">Hedgehug {version}</p>
  {/if}
</nav>

<style>
  .nav {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    min-height: 100%;
    padding: 14px var(--space-10);
  }
  .back {
    display: inline-flex;
    gap: var(--space-6);
    padding: 0 var(--space-8);
    color: var(--ink-2);
    font-size: var(--fs-13);
    text-decoration: none;
  }
  .back:hover {
    color: var(--ink);
  }
  h2 {
    margin: 0;
    padding: 0 var(--space-8);
    font-size: var(--fs-17);
    font-weight: 600;
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .cap {
    padding: 0 var(--space-8) var(--space-4);
    color: var(--label);
    font-size: var(--fs-11);
    font-weight: 600;
    letter-spacing: 0.04em;
  }
  .store {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: 0 var(--space-8) var(--space-8);
  }
  .label {
    color: var(--label);
    font-size: var(--fs-11);
  }
  select {
    min-height: var(--control-h);
    padding: 0 var(--space-8);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-size: var(--fs-13);
  }
  .item {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    min-height: var(--row-h);
    padding: 0 var(--space-8);
    border-radius: var(--radius-6);
    color: var(--ink-2);
    text-decoration: none;
  }
  .item:hover {
    background: var(--hover);
  }
  .item.on,
  .item.on:hover {
    background: var(--accent-weak);
    color: var(--accent-ink);
    font-weight: 600;
  }
  .note {
    margin: var(--space-8) 0 0;
    padding: 0 var(--space-8);
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .foot {
    margin: auto 0 0;
    padding: 0 var(--space-8);
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
  }
</style>
```

`web/app/settings/SettingsLayout.svelte` を作る:
```svelte
<script lang="ts">
  import type { Snippet } from "svelte";
  import type { SettingsPage } from "../router";
  import SettingsNav from "./SettingsNav.svelte";

  let { page, children }: { page: SettingsPage; children?: Snippet } = $props();
</script>

<!-- The whole window is one layout: the nav on the left takes the place of the main sidebar. -->
<div class="settings">
  <aside class="nav">
    <SettingsNav {page} />
  </aside>
  <section class="content" aria-label="設定の本文">
    {@render children?.()}
  </section>
</div>

<style>
  .settings {
    display: grid;
    grid-template-columns: var(--sidebar-w) minmax(0, 1fr);
    height: 100%;
    min-height: 0;
  }
  .nav {
    overflow-y: auto;
    border-right: 1px solid var(--line);
    background: var(--sidebar);
  }
  .content {
    min-width: 0;
    min-height: 0;
    overflow: auto;
    padding: var(--space-20) var(--gutter);
  }
  @media (max-width: 819px) {
    .settings {
      grid-template-columns: minmax(0, 1fr);
      height: auto;
    }
    .nav {
      border-right: 0;
      border-bottom: 1px solid var(--line);
    }
  }
</style>
```

`web/app/settings/SettingsRoute.svelte` を作る:
```svelte
<script lang="ts">
  import { core } from "../context";
  import { hrefOf, router, type SettingsPage } from "../router";
  import { useSubscribe } from "../svelte-store";
  import SettingsLayout from "./SettingsLayout.svelte";

  let { page }: { page: SettingsPage } = $props();

  const c = core();
  const first = useSubscribe(c.stores, () => c.stores.stores()[0]?.name ?? null);

  // /settings names no Store: it is replaced by the first one, once the Stores are read.
  $effect(() => {
    if (page.store !== "" || first.current === null) return;
    router.go(hrefOf({ name: "settings", page: page.page, store: first.current }), {
      replace: true,
    });
  });
</script>

{#if page.store !== ""}
  <SettingsLayout {page} />
{/if}
```

Run: `cd web && pnpm fix && pnpm vitest run app/settings`
Expected: PASS

- [ ] **Step 10: サイドバーに設定の入口（右上の歯車）**

歯車は `/settings` を開く（細部 18。アカウントの設定は 4f）。ワークスペースの行は概要を開き、どのタブにいても今いる行として示す。

`web/app/sidebar/Sidebar.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
  it("ワークスペース › プロジェクトの入れ子と進捗を出す", () => {
```

変更後:
```ts
  it("右上の歯車は設定（/settings）を開く", async () => {
    const { getByRole } = mount();
    const gear = getByRole("link", { name: "設定" });
    expect(gear.getAttribute("href")).toBe("/settings");
    expect(gear.getAttribute("aria-current")).toBeNull();
    await fireEvent.click(gear);
    expect(location.pathname).toBe("/settings");
  });

  it("設定を開いている間は歯車を aria-current にする", () => {
    history.replaceState(null, "", "/settings/workspaces/shared");
    const { getByRole } = mount();
    expect(getByRole("link", { name: "設定" }).getAttribute("aria-current")).toBe("page");
  });

  it("ワークスペースはどのタブにいても今いる行にする", () => {
    history.replaceState(null, "", "/workspaces/shared/01WSREL/tree");
    const { getByRole } = mount();
    expect(getByRole("link", { name: /release/ }).getAttribute("aria-current")).toBe("page");
  });

  it("ワークスペース › プロジェクトの入れ子と進捗を出す", () => {
```

`web/app/sidebar/Sidebar.svelte` を変える（`pnpm fix` が整形を直す）:

変更前:
```svelte
  const route = $derived(router.route);
```

変更後:
```svelte
  const route = $derived(router.route);
  // /settings names no Store: the settings go to the first one.
  const settingsHref = hrefOf({ name: "settings", page: "workspaces", store: "" });
```

変更前:
```svelte
    <span class="host">{location.host}</span>
  </div>
```

変更後:
```svelte
    <span class="host">{location.host}</span>
    <a
      class="gear"
      href={settingsHref}
      aria-label="設定"
      aria-current={route.name === "settings" ? "page" : undefined}
      onclick={(e) => onLinkClick(e, settingsHref)}
    >
      <Icon name="settings" size={16} />
    </a>
  </div>
```

変更前:
```svelte
hrefOf({ name: "workspace", store: w.store, ulid: w.ulid })
```

変更後:
```svelte
hrefOf({ name: "workspace", store: w.store, ulid: w.ulid, tab: "overview" })
```

変更前:
```svelte
hrefOf({ name: "project", ulid: p.ulid })
```

変更後:
```svelte
hrefOf({ name: "project", ulid: p.ulid, tab: "tree" })
```

変更前:
```svelte
  .search {
    display: flex;
```

変更後:
```svelte
  .gear {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--control-h-sm);
    height: var(--control-h-sm);
    margin-left: auto;
    border-radius: var(--radius-6);
    color: var(--label);
  }
  .gear:hover {
    background: var(--hover);
  }
  .search {
    display: flex;
```

Run: `cd web && pnpm fix && pnpm vitest run app/sidebar`
Expected: PASS

- [ ] **Step 11: App のルートの振り分け**

ワークスペースのルートと `/`（最初のワークスペースの概要。URL は `/` のまま）は `WorkspacePage`、設定のルートは `SettingsRoute`。設定のルートのあいだは、サイドバーを隠して（`hidden`）設定のナビに替え、`.main` の余白をなくす。サイドバーは外さず隠すだけにするのは、アプリのキー（⌘K・⇧⌘S）をサイドバーと同期カードが登録しているため。ワークスペースの画面は `store/ulid` で `{#key}` し、タブを替えても見出しは作り直さない（タブの本文だけが替わる）。

`web/app/App.svelte` を変える（`pnpm fix` が整形を直す）:

変更前:
```svelte
  import Sidebar from "./sidebar/Sidebar.svelte";
```

変更後:
```svelte
  import SettingsRoute from "./settings/SettingsRoute.svelte";
  import Sidebar from "./sidebar/Sidebar.svelte";
```

変更前:
```svelte
  import Toasts from "./ui/Toasts.svelte";
```

変更後:
```svelte
  import Toasts from "./ui/Toasts.svelte";
  import WorkspacePage from "./workspace/WorkspacePage.svelte";
```

変更前:
```svelte
  const noToken = $derived(
```

変更後:
```svelte
  const route = $derived(router.route);
  // The settings take the whole window: their nav stands where the sidebar stands. The sidebar
  // stays mounted, because the keys of the app (⌘K, ⇧⌘S) are registered by it.
  const inSettings = $derived(route.name === "settings");
  const noToken = $derived(
```

変更前:
```svelte
<div class="app" inert={blocked}>
```

変更後:
```svelte
<div class="app" class:settings={inSettings} inert={blocked}>
```

変更前:
```svelte
  <aside class="side">
```

変更後:
```svelte
  <aside class="side" hidden={inSettings}>
```

変更前:
```svelte
    <main class="main" id="main">
```

変更後:
```svelte
    <main class="main" class:flush={inSettings} id="main">
```

変更前:
```svelte
      {#if router.route.name === "workspace"}
        {#key `${router.route.store}/${router.route.ulid}`}
          <TreeView
            view={{ kind: "workspace", store: router.route.store, ulid: router.route.ulid }}
          />
        {/key}
      {:else if router.route.name === "project"}
        {#key router.route.ulid}
          <TreeView view={{ kind: "project", ulid: router.route.ulid }} />
        {/key}
      {:else if router.route.name === "task"}
        {#key router.route.ulid}
          <TaskDetail ulid={router.route.ulid} />
        {/key}
      {:else if router.route.name === "home" && dir.current.first}
        <!-- "/" is the tree of the first workspace -->
        {#key `${dir.current.first.store}/${dir.current.first.ulid}`}
          <TreeView view={{ kind: "workspace", ...dir.current.first }} />
        {/key}
```

変更後:
```svelte
      {#if route.name === "workspace"}
        {#key `${route.store}/${route.ulid}`}
          <WorkspacePage store={route.store} ulid={route.ulid} tab={route.tab} />
        {/key}
      {:else if route.name === "project"}
        {#key route.ulid}
          <TreeView view={{ kind: "project", ulid: route.ulid }} />
        {/key}
      {:else if route.name === "task"}
        {#key route.ulid}
          <TaskDetail ulid={route.ulid} />
        {/key}
      {:else if route.name === "settings"}
        <SettingsRoute page={route} />
      {:else if route.name === "home" && dir.current.first}
        <!-- "/" is the overview of the first workspace -->
        {#key `${dir.current.first.store}/${dir.current.first.ulid}`}
          <WorkspacePage
            store={dir.current.first.store}
            ulid={dir.current.first.ulid}
            tab="overview"
          />
        {/key}
```

変更前:
```svelte
{:else if router.route.name === "home" && dir.current.loaded
```

変更後:
```svelte
{:else if route.name === "home" && dir.current.loaded
```

変更前:
```svelte
  .body {
    display: flex;
```

変更後:
```svelte
  /* The settings have their own nav in place of the sidebar. */
  .app.settings {
    grid-template-columns: minmax(0, 1fr);
  }
  .body {
    display: flex;
```

変更前:
```svelte
  @media (max-width: 819px) {
    .app {
```

変更後:
```svelte
  .main.flush {
    padding: 0;
  }
  @media (max-width: 819px) {
    .app {
```

`web/app/App.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
    expect(sessionStorage.getItem("hedgehug.token")).toBe("tkn");
  });
```

変更後:
```ts
    expect(sessionStorage.getItem("hedgehug.token")).toBe("tkn");
  });

  it("設定の画面ではサイドバーを隠してナビに替える（サイドバーは残す）", () => {
    history.replaceState(null, "", "/settings/workspaces/shared#token=tkn");
    const { container } = render(App);
    flushSync();
    expect(container.querySelector("aside.side")).toHaveAttribute("hidden");
    expect(container.querySelector("main")).toHaveClass("flush");
    expect(location.pathname).toBe("/settings/workspaces/shared");
  });

  it("設定以外の画面ではサイドバーを出す", () => {
    history.replaceState(null, "", "/tasks/01HTK#token=tkn");
    const { container } = render(App);
    flushSync();
    expect(container.querySelector("aside.side")).not.toHaveAttribute("hidden");
  });
```

`/` が概要になったので、4d のゲートのテスト「ホーム / は最初のワークスペースのツリーを開く」を直す（`tasks` の問い合わせではなく、概要の見出しとタブを見る）。`web/app/gates.test.ts` を変える:

`web/app/gates.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
import { cleanup, render, waitFor } from "@testing-library/svelte";
```

変更後:
```ts
import { cleanup, render } from "@testing-library/svelte";
```

変更前:
```ts
it("ホーム / は最初のワークスペースのツリーを開く", async () => {
```

変更後:
```ts
it("ホーム / は最初のワークスペースの概要を開く（URL は / のまま）", async () => {
```

変更前:
```ts
    const asked: string[] = [];
    vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      asked.push(url);
      if (url.includes("/api/events")) return new Promise(() => {});
      if (url.includes("/api/workspaces")) {
```

変更後:
```ts
    vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url.includes("/api/events")) return new Promise(() => {});
      if (url.includes("/api/workspaces")) {
```

変更前:
```ts
            prefix_set: "2026-11-02T00:00:00Z",
            tasks: 0,
          },
        ]);
      }
      if (url.includes("/api/stores")) {
        return json([{ name: "me", path: "/p", remotes: [] }]);
      }
      return new Promise(() => {});
    });
    render(App);
    await waitFor(() =>
      expect(asked.some((u) => u.includes("/api/tasks") && u.includes("workspace"))).toBe(true),
    );
  });
```

変更後:
```ts
            prefix_set: "2026-11-02T00:00:00Z",
            tasks: 0,
            deleted: 0,
          },
        ]);
      }
      if (url.includes("/api/stores")) {
        return json([{ name: "me", path: "/p", remotes: [] }]);
      }
      return new Promise(() => {});
    });
    const { findByRole, getByRole } = render(App);
    expect(await findByRole("heading", { level: 1, name: "me" })).toBeTruthy();
    expect(
      getByRole("navigation", { name: "ビューの切り替え" }).querySelector("[aria-current]")
        ?.textContent,
    ).toContain("概要");
    expect(location.pathname).toBe("/");
  });
```

Run: `cd web && pnpm fix && pnpm check && pnpm test`
Expected: PASS（vitest 全件、svelte-check 警告 0、biome 指摘 0）

- [ ] **Step 12: E2E を新しいルートに合わせる**

`hh.open(page)` が開く `/` は概要になった。ツリーの行を探す spec は、最初のワークスペースのツリーのタブを開く `hh.openTree(page)` に替える。ハーネスは API から最初の Store と最初のワークスペースを引いてパスを作る（Store の名前を spec に書かない）。

`web/e2e/harness.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
  add(title: string, ...args: string[]): TaskJson {
```

変更後:
```ts
  /** GETs the API, as the page does. */
  async get<T>(path: string): Promise<T> {
    const r = await fetch(`${this.origin}${path}`, {
      headers: { Authorization: `Bearer ${this.token}` },
    });
    if (!r.ok) throw new Error(`GET ${path}: ${r.status} ${await r.text()}`);
    return (await r.json()) as T;
  }

  /** The path of the tree tab of the first workspace of the first store, which "/" is the overview of. */
  async treePath(): Promise<string> {
    const stores = await this.get<{ name: string }[]>("/api/stores");
    const store = stores[0]?.name;
    if (!store) throw new Error("hh ui has no store");
    const spaces = await this.get<{ ulid: string }[]>(
      `/api/workspaces?store=${encodeURIComponent(store)}`,
    );
    const space = spaces[0];
    if (!space) throw new Error(`the store ${store} has no workspace`);
    return `/workspaces/${encodeURIComponent(store)}/${space.ulid}/tree`;
  }

  /** Opens the tree tab of the first workspace, and waits until the page is live. */
  async openTree(page: Page): Promise<void> {
    await this.open(page, await this.treePath());
  }

  add(title: string, ...args: string[]): TaskJson {
```

`web/e2e/tree.spec.ts` を変える。`await hh.open(page);`（21 か所すべて）を `await hh.openTree(page);` にし、冒頭のコメントを直す:

`web/e2e/tree.spec.ts` を変える（コメントの直し）:

変更前:
```ts
//   - "/" is the tree of the first workspace; the filter box "フィルタ（hh ls と同じクエリ）"
//     starts as "-status:canceled" and the foot of the tree shows the `hh ls` of the filter
```

変更後:
```ts
//   - the tree is the "ツリー" tab of the first workspace, /workspaces/<store>/<ulid>/tree (the
//     harness opens it with hh.openTree; "/" is the overview); the filter box
//     "フィルタ（hh ls と同じクエリ）" starts as "-status:canceled" and the foot of the tree
//     shows the `hh ls` of the filter
```

`web/e2e/live.spec.ts` を変える（行を探す 4 本。サイドバーを見る 1 本と詳細を開く spec は `/` のままでよい）:

変更前:
```ts
hh.add("Already here");
  await hh.open(page);
```

変更後:
```ts
hh.add("Already here");
  await hh.openTree(page);
```

変更前:
```ts
const t = hh.add("Old title");
  await hh.open(page);
```

変更後:
```ts
const t = hh.add("Old title");
  await hh.openTree(page);
```

変更前:
```ts
const t = hh.add("Doomed");
  hh.add("Stays");
  await hh.open(page);
```

変更後:
```ts
const t = hh.add("Doomed");
  hh.add("Stays");
  await hh.openTree(page);
```

変更前:
```ts
const other = hh.add("Someone else's");
  await hh.open(page);
```

変更後:
```ts
const other = hh.add("Someone else's");
  await hh.openTree(page);
```

`web/e2e/create.spec.ts` を変える（行を探す 3 本）:

変更前:
```ts
  await hh.open(page);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("親: なし")).toBeVisible();
```

変更後:
```ts
  await hh.openTree(page);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("親: なし")).toBeVisible();
```

変更前:
```ts
test("Ctrl+Enter creates", async ({ page, hh }) => {
  await hh.open(page);
```

変更後:
```ts
test("Ctrl+Enter creates", async ({ page, hh }) => {
  await hh.openTree(page);
```

変更前:
```ts
  const task = hh.add("A plain task", "--parent", project.id);
  await hh.open(page);
```

変更後:
```ts
  const task = hh.add("A plain task", "--parent", project.id);
  await hh.openTree(page);
```

`web/e2e/newtab.spec.ts` を変える（サイドバーのワークスペースのリンクは概要を開くので、開いた先の見出しで確かめる）:

変更前:
```ts
  await expect(tab.getByRole("tree", { name: "demo のタスク" })).toBeVisible();
```

変更後:
```ts
  await expect(tab.getByRole("heading", { level: 1, name: "demo" })).toBeVisible();
```

`web/e2e/routes.spec.ts` を作る（この回のルート・タブ・設定の入口の spec。Task 2・3 の画面の spec は別のファイルに置く）:
```ts
// Screens this spec relies on:
//   - "/" is the overview of the first workspace: the heading (level 1) is the workspace name, and
//     the navigation "ビューの切り替え" holds the links 概要, ツリー and 設定; the current one has
//     aria-current="page". The tabs are links (the address, back and reload keep the tab)
//   - the sidebar ("メインナビゲーション") has the link "設定" (the gear, /settings)
//   - the settings take the window: the navigation "設定のナビゲーション" (with the link
//     "Hedgehug に戻る" and the version) is shown in place of the sidebar
import type { Page } from "@playwright/test";
import { expect, test } from "./harness";
import { row } from "./screens";

const tabs = (page: Page) => page.getByRole("navigation", { name: "ビューの切り替え" });
const sidebar = (page: Page) => page.getByRole("navigation", { name: "メインナビゲーション" });
const heading = (page: Page) => page.getByRole("heading", { level: 1, name: "demo" });

test("the tabs of a workspace are links: back, forward and reload keep the tab", async ({
  page,
  hh,
}) => {
  hh.add("Alpha");
  await hh.open(page);
  await expect(heading(page)).toBeVisible();
  await expect(tabs(page).getByRole("link", { name: "概要" })).toHaveAttribute(
    "aria-current",
    "page",
  );

  await tabs(page).getByRole("link", { name: "ツリー" }).click();
  await expect(page).toHaveURL(/\/workspaces\/demo\/[^/]+\/tree$/);
  await expect(row(page, "Alpha")).toBeVisible();
  // The heading stays above the tree.
  await expect(heading(page)).toBeVisible();
  await expect(tabs(page).getByRole("link", { name: "ツリー" })).toHaveAttribute(
    "aria-current",
    "page",
  );

  await page.goBack();
  await expect(page).toHaveURL(`${hh.origin}/`);
  await expect(tabs(page).getByRole("link", { name: "概要" })).toHaveAttribute(
    "aria-current",
    "page",
  );

  await page.goForward();
  await expect(row(page, "Alpha")).toBeVisible();
  await page.reload();
  await expect(row(page, "Alpha")).toBeVisible();
  await expect(tabs(page).getByRole("link", { name: "ツリー" })).toHaveAttribute(
    "aria-current",
    "page",
  );
});

test("the settings tab is the right end of the tabs", async ({ page, hh }) => {
  await hh.open(page);
  await tabs(page).getByRole("link", { name: "設定" }).click();
  await expect(page).toHaveURL(/\/workspaces\/demo\/[^/]+\/settings$/);
  await expect(tabs(page).getByRole("link", { name: "設定" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(heading(page)).toBeVisible();
});

test("/settings goes to the workspaces of the first store, with its own nav in place of the sidebar", async ({
  page,
  hh,
}) => {
  await hh.open(page, "/settings");
  await expect(page).toHaveURL(/\/settings\/workspaces\/demo$/);
  const nav = page.getByRole("navigation", { name: "設定のナビゲーション" });
  await expect(nav).toBeVisible();
  await expect(nav.getByRole("link", { name: /ワークスペース/ })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(nav.getByText(/^Hedgehug \S+$/)).toBeVisible();
  await expect(sidebar(page)).toBeHidden();

  await nav.getByRole("link", { name: /Hedgehug に戻る/ }).click();
  await expect(sidebar(page)).toBeVisible();
  await expect(heading(page)).toBeVisible();
});

test("the gear of the sidebar opens the settings", async ({ page, hh }) => {
  await hh.open(page);
  await sidebar(page).getByRole("link", { name: "設定", exact: true }).click();
  await expect(page).toHaveURL(/\/settings\/workspaces\/demo$/);
  await expect(page.getByRole("navigation", { name: "設定のナビゲーション" })).toBeVisible();
});
```

Run: `cd web && pnpm fix && pnpm check && pnpm test`、続けて `make e2e`（リポジトリのルートで。`web/e2e` の spec が全部通る）
Expected: PASS（`make e2e` は `playwright install chromium` のあとに `pnpm e2e`）

- [ ] **Step 13: コミットする**

```bash
git add web/app web/core web/e2e
git commit -m "feat(web): ワークスペースと設定のルート・タブ・見出し・設定のナビを足す"
```

---

### Task 2a: ワークスペースのダイアログ（追加・接頭辞の変更・削除・別のワークスペースへの移動）

**Files:**
- Modify: `web/core/cli.ts`、`web/core/cli.test.ts`（`cliWorkspaceRename`・`cliWorkspaceSetPrefix`・`cliWorkspaceRm`、`cliMv` の `workspace`）
- Create: `web/core/workspace.ts`、`web/core/workspace.test.ts`（名前・接頭辞の検査、削除できない理由、移動で外れたものの文）
- Create: `web/app/workspace/dialogs-state.svelte.ts`、`move-menu.ts`、`move-menu.test.ts`、`testkit.ts`
- Create: `web/app/workspace/WorkspaceAdd.svelte`、`PrefixChange.svelte`、`WorkspaceDelete.svelte`、`WorkspaceMove.svelte`、`WorkspaceDialogs.svelte` と、それぞれの `*.test.ts`（`WorkspaceDialogs.test.ts` を含む）
- Modify: `web/app/detail/TaskDetail.svelte`（ヘッダーの「その他の操作」）、`web/app/tree/TreeRow.svelte`・`TreeView.svelte`（行の「メニュー」ボタン）、`web/app/tree/TreeView.test.ts`
- Create: `web/app/detail/TaskDetail.more.test.ts`
- Modify: `web/app/App.svelte`（`<WorkspaceDialogs />` を `<CreateHost />` の次に1つ置く）

**Interfaces:**
- Consumes（Task 1 の最終形。この回のコードはこれが入っている前提で動かして確かめた）:
  - `web/app/router.ts` の `Route`（`{ name: "workspace"; store; ulid; tab: "overview" | "tree" | "settings" }`）、`router.href(route)`・`router.route`・`router.go(path)`・`hrefOf({ name: "home" })`。パスは概要が `/workspaces/<store>/<ulid>`、設定タブが `/workspaces/<store>/<ulid>/settings`
  - `web/core/stores.ts` の `WorkspaceEntry`（`store`・`tone`・`ulid`・`name`・`prefix`・`projects`・`tasks`・`deleted`。`tasks` は削除済みを除いた件数、`deleted` は削除済みの件数、`prefixSet` は接頭辞を最後に変えた時刻の ISO 日時）。Task 1 は `WorkspaceEntry` の型を付けたテストのフィクスチャ（`web/app/detail/testkit.ts` の `workspaces`、`app/palette/search.test.ts`）にも `tasks`・`deleted`・`prefixSet` を足している（足さないと `pnpm check` が通らない。型を付けていない配列はこの 3 項目がなくても通る）
  - 既存: `core()`（`api`・`cache`・`stores`）、`useSubscribe`、`toast`・`toasts`・`resetToasts`、`ui/Dialog.svelte`・`Button.svelte`・`Menu.svelte`・`CliLine.svelte`・`StoreBadge.svelte`、`errorMessage`、`unwrap`、`TaskCache.applyWrite`・`noteOwnCommit`、`StoreDirectory.stores()`・`workspaces()`・`tone()`・`reload()`、`detail/testkit.ts` の `makeCore`・`mount`・`ok`・`fail`・`taskView`
  - API（`api/openapi.yaml` で確かめた）: `POST /api/workspaces?store=`（本文 `{name, prefix}`、201 で `WorkspaceWrite`）、`PATCH /api/workspaces/{id}?store=`（本文は `{name}` か `{prefix}`、両方は 400 `workspace.one_change`。応答の `ids` は接頭辞の変更で振り直した `{old, new}`）、`DELETE /api/workspaces/{id}?store=`、`POST /api/tasks/{id}/move?store=`（本文 `{workspace}`。応答 `TaskWrite` の `detached` は `{task, id, sprint?, releases?}` の配列）。`{id}` は ULID を渡す
  - エラーの Code と文言（`web/core/messages.ts` にすべてある。足さない）: `workspace.bad_name`・`workspace.bad_prefix`・`workspace.taken`（名前・接頭辞が同じ Store で重なる）・`workspace.prefix_taken`（別の Store と重なる）・`workspace.not_empty`（`args.tasks`）・`workspace.one_change`・`workspace.not_found`・`move.invalid`・`task.deleted`
- Produces:
  - `web/core/cli.ts`: `cliWorkspaceRename({ workspace: string; name: Arg; store: string })`、`cliWorkspaceSetPrefix({ workspace: string; prefix: Arg; store: string })`、`cliWorkspaceRm({ workspace: string; store: string })`、`cliMv(id, { …; workspace?: Arg })`（`Arg = string | Placeholder`）
  - `web/core/workspace.ts`: `nameProblem(name, store, all, self?)`・`prefixProblem(prefix, store, all, self?)`（理由の文字列か `null`。`all` は `{store, ulid, name, prefix}` の配列、`self` は改名・接頭辞の変更で自分を数えないための ULID）、`removeBlocker({tasks, deleted})`（削除できない理由か `null`）、`detachedText(detached)`、`NAME_RULE`・`PREFIX_RULE`
  - `web/app/workspace/dialogs-state.svelte.ts`: `openWorkspaceAdd(store?: string)`、`openWorkspaceDelete(entry: WorkspaceEntry)`、`openPrefixChange(entry: WorkspaceEntry)`、`openWorkspaceMove(task: MoveTask)`（`MoveTask` は `TaskView` の `ulid`・`id`・`title`・`store`・`workspace` だけの `Pick`。`TaskView` も `ShowView` もそのまま渡せる）、`closeWorkspaceDialog()`、`workspaceDialog.current`
  - `web/app/workspace/WorkspaceDialogs.svelte`（props なし。App に1つ）、`WorkspaceAdd.svelte`（`{store?, onclose}`）、`PrefixChange.svelte`・`WorkspaceDelete.svelte`（`{entry, onclose}`）、`WorkspaceMove.svelte`（`{task, onclose}`）
  - `web/app/workspace/move-menu.ts`: `MOVE_VALUE`、`moveWorkspaceItem(task, entries): MenuItem`（ツリーの行とタスク詳細が同じ項目を使う）
  - `web/app/workspace/testkit.ts`: `entry(over?)`（`WorkspaceEntry`）、`storeItem(name, isDefault?)`、`written(workspace?, extra?)`（`WorkspaceWrite`）。Task 2b のテストも使う。Task 3 のテストも `entry` を使う

**この Task で決めたこと**
- 名前の規則は Go の `model.ValidName`（`^[a-z0-9][a-z0-9_-]*$`）、接頭辞は `model.ValidIDPrefix`（`^[A-Z][A-Z0-9]{1,9}$`）と同じ。同じ Store で名前・接頭辞が重なる（名前は大文字小文字を区別しない）、別の Store の接頭辞と重なる、はフォームの横に理由を出して追加・変更のボタンを無効にする。サーバーの拒否（`workspace.taken` など）は保険として、ダイアログの中に赤い帯で出す
- 追加の Store の選択は、Store が2つ以上のときだけ出す。初期値は呼び出し元が渡した Store、なければ `default` の Store、なければ最初の Store
- 接頭辞の変更の「配下の n 件」は `tasks + deleted`（細部 24）。キャンバスの「RLS-1 から順に振ります」は、新しい接頭辞に使った番号があると成り立たない（`usedNumbers`）ので書かない。コミットは自分のものなので `cache.noteOwnCommit(r.commit)` で行に「CLI で更新」の印を付けない
- 削除の確認は、タスク（削除済みを含む）が1件もないときだけ開く（キャンバスの Dialogs: 「残っていれば設定タブで理由を出す」）。`openWorkspaceDelete(entry)` は残りがあれば確認を開かずにそのワークスペースの設定タブへ移る（サイドバーの「削除…」から呼ばれても理由が見える）。ダイアログ自体も、開いたあとにタスクが足されたときのために理由を出して「削除」を無効にする。サーバーの 409 `workspace.not_empty` はさらにその保険。削除したワークスペースの画面にいたら `/` へ移る。トーストの取り消し（`runUndo`）は、`runUndo` を作る Task 3 の Step 9 がこのダイアログに足す（この Task では付けない。パレットの「直前の操作を取り消す」で戻せる旨はダイアログに書く）
- 移動の移り先は同じ Store のワークスペースだけ（細部 31）。今のワークスペースは選べない印で出し、最初の移り先を選んでおく。説明の3行（ルートに置く／ID の振り直し／スプリント・リリースが外れる）は静的。移したあとの `detached` は ID ごとに「<ID> は スプリント <名前> と リリース <名前> から外れました」でトーストに足す。足元は `hh mv <ID> --workspace <名前>`（サーバーが親を外すので `--root` は付けない。実際の CLI で確かめた）
- 「別のワークスペースへ移動…」は、タスク詳細のヘッダーに足した「その他の操作」（「…」）のメニューと、ツリーの行の右端に足した「メニュー」ボタンのメニューの2か所。4d のツリーには行のメニューがなかった（S・K のメニューはキーで開く）ので、行のボタンは新しく足した（Foundations に行のメニューの決まりはない。行内の小さいボタンの大きさ 28px で、行にカーソルを合わせたとき・選んだときに見える。キーは割り当てない。spec のキーボードにないため）。Store にほかのワークスペースがなければ項目は無効で、理由を項目に出す。行のメニューはスキーマを読み込んだあとに開く（S・K のメニューと同じ）
- テストのフィクスチャ `entry()` は `tasks: 24, deleted: 1`（合わせて 25）。ダイアログは開いている間も `stores.workspaces()` から今の値を引き直す（件数が変わる、消える）

- [ ] **Step 1: 失敗するテストを書く**

`web/core/cli.test.ts` の import に3つ足す。

```ts
// import { … } from "./cli" の中、cliWorkspaceAdd の次に
  cliWorkspaceRename,
  cliWorkspaceRm,
  cliWorkspaceSetPrefix,
```

`it("writes the others", …)` の直前に足す。

```ts
  it("writes hh mv --workspace, and hh workspace rename, set-prefix and rm", () => {
    expect(cliMv("REL-6", { workspace: "shared" })).toBe("hh mv REL-6 --workspace shared");
    expect(cliMv("REL-6", { workspace: new Placeholder("ワークスペース") })).toBe(
      "hh mv REL-6 --workspace <ワークスペース>",
    );
    expect(cliWorkspaceRename({ workspace: "release", name: "releases", store: "shared" })).toBe(
      "hh workspace rename release releases --store shared",
    );
    expect(
      cliWorkspaceRename({
        workspace: "release",
        name: new Placeholder("新しい名前"),
        store: "shared",
      }),
    ).toBe("hh workspace rename release <新しい名前> --store shared");
    expect(cliWorkspaceSetPrefix({ workspace: "release", prefix: "RLS", store: "shared" })).toBe(
      "hh workspace set-prefix release RLS --store shared",
    );
    expect(cliWorkspaceRm({ workspace: "sandbox", store: "shared" })).toBe(
      "hh workspace rm sandbox --store shared",
    );
  });
```

新しいファイル `web/core/workspace.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import {
  detachedText,
  NAME_RULE,
  nameProblem,
  PREFIX_RULE,
  prefixProblem,
  removeBlocker,
} from "./workspace";

const all = [
  { store: "shared", ulid: "W1", name: "release", prefix: "REL" },
  { store: "shared", ulid: "W2", name: "shared", prefix: "SH" },
  { store: "backend", ulid: "B1", name: "api", prefix: "API" },
];

describe("nameProblem", () => {
  it("accepts a free name", () => {
    expect(nameProblem("ops", "shared", all)).toBeNull();
    expect(nameProblem("ops-2_b", "shared", all)).toBeNull();
  });

  it("asks for a name, and refuses what the server refuses", () => {
    expect(nameProblem("", "shared", all)).toBe("名前を入れてください");
    for (const bad of ["Ops", "-ops", "o ps", "運用"]) {
      expect(nameProblem(bad, "shared", all)).toBe(NAME_RULE);
    }
  });

  it("refuses the name of a workspace of the same store, but not of another store", () => {
    expect(nameProblem("shared", "shared", all)).toBe(
      "shared は同じ Store のワークスペースが使っています",
    );
    expect(nameProblem("api", "shared", all)).toBeNull();
  });

  it("does not count the workspace that is renamed", () => {
    expect(nameProblem("release", "shared", all, "W1")).toBeNull();
    expect(nameProblem("shared", "shared", all, "W1")).not.toBeNull();
  });
});

describe("prefixProblem", () => {
  it("accepts a free prefix", () => {
    expect(prefixProblem("OPS", "shared", all)).toBeNull();
    expect(prefixProblem("A1", "shared", all)).toBeNull();
  });

  it("asks for a prefix, and refuses what the server refuses", () => {
    expect(prefixProblem("", "shared", all)).toBe("接頭辞を入れてください");
    for (const bad of ["O", "ops", "1OP", "ABCDEFGHIJK", "OP-S"]) {
      expect(prefixProblem(bad, "shared", all)).toBe(PREFIX_RULE);
    }
  });

  it("says which workspace has the prefix, in the same store or in another", () => {
    expect(prefixProblem("SH", "shared", all)).toBe(
      "SH は同じ Store のワークスペース shared が使っています",
    );
    expect(prefixProblem("API", "shared", all)).toBe(
      "API はマウント中の backend のワークスペース api が使っています。別の接頭辞にしてください",
    );
  });

  it("does not count the workspace whose prefix is changed", () => {
    expect(prefixProblem("REL", "shared", all, "W1")).toBeNull();
  });
});

describe("removeBlocker", () => {
  it("lets an empty workspace go", () => {
    expect(removeBlocker({ tasks: 0, deleted: 0 })).toBeNull();
  });

  it("counts the deleted tasks, which still hold the workspace", () => {
    expect(removeBlocker({ tasks: 24, deleted: 1 })).toBe(
      "タスク 25 件（削除済み 1 件を含む）が残っているため削除できません。別のワークスペースへ移すか完全に削除すると消せます",
    );
    expect(removeBlocker({ tasks: 2, deleted: 0 })).toBe(
      "タスク 2 件が残っているため削除できません。別のワークスペースへ移すか完全に削除すると消せます",
    );
    expect(removeBlocker({ tasks: 0, deleted: 3 })).toContain("タスク 3 件（削除済み 3 件を含む）");
  });
});

describe("detachedText", () => {
  it("says nothing when nothing was taken off", () => {
    expect(detachedText(null)).toBe("");
    expect(detachedText([])).toBe("");
    expect(detachedText([{ task: "U1", id: "SH-1" }])).toBe("");
  });

  it("names the sprint and the releases of each task", () => {
    expect(
      detachedText([
        { task: "U1", id: "SH-12", sprint: "Sprint 44", releases: ["v2.4", "v2.5"] },
        { task: "U2", id: "SH-13", releases: ["v2.4"] },
      ]),
    ).toBe(
      "SH-12 はスプリント Sprint 44 とリリース v2.4、v2.5 から外れました。SH-13 はリリース v2.4 から外れました",
    );
  });
});
```

新しいファイル `web/app/workspace/testkit.ts`（Task 2b・Task 3 のテストも使う）:

```ts
import type { WorkspaceEntry } from "../../core/stores";

/** A workspace of the directory, for the tests of the workspace screens. */
export function entry(over: Partial<WorkspaceEntry> = {}): WorkspaceEntry {
  return {
    store: "shared",
    tone: "purple",
    ulid: "W1",
    name: "release",
    prefix: "REL",
    projects: [],
    tasks: 24,
    deleted: 1,
    prefixSet: "2026-10-29T12:00:00Z",
    ...over,
  };
}

/** What GET /api/stores answers for one store. */
export function storeItem(name: string, isDefault = false) {
  return { name, default: isDefault, path: `/data/${name}`, remotes: [] };
}

/** The answer of a write to a workspace (WorkspaceWrite). */
export function written(
  w: Partial<{ ulid: string; name: string; id_prefix: string }> = {},
  extra: { commit?: string | null; ids?: { old: string; new: string }[] } = {},
) {
  return {
    workspace: {
      ulid: "W1",
      name: "release",
      id_prefix: "REL",
      prefix_set: "2026-11-03T00:00:00Z",
      tasks: 0,
      deleted: 0,
      ...w,
    },
    commit: "c1",
    ...extra,
  };
}
```

新しいファイル `web/app/workspace/move-menu.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { MOVE_VALUE, moveWorkspaceItem } from "./move-menu";
import { entry } from "./testkit";

const task = { store: "shared", workspace: { ulid: "W1", name: "release", id_prefix: "REL" } };

describe("moveWorkspaceItem", () => {
  it("is on when the store has another workspace", () => {
    const item = moveWorkspaceItem(task, [entry(), entry({ ulid: "W2", name: "shared" })]);
    expect(item).toMatchObject({
      value: MOVE_VALUE,
      label: "別のワークスペースへ移動…",
      disabled: false,
    });
    expect(item.hint).toBeUndefined();
  });

  it("is off, and says why, when the only other workspaces are in other stores", () => {
    const item = moveWorkspaceItem(task, [entry(), entry({ store: "backend", ulid: "B1" })]);
    expect(item.disabled).toBe(true);
    expect(item.hint).toBe("この Store にはほかのワークスペースがありません");
  });
});
```

新しいファイル `web/app/workspace/WorkspaceAdd.test.ts`:

```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fail, makeCore, mount, ok } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import { entry, storeItem, written } from "./testkit";
import WorkspaceAdd from "./WorkspaceAdd.svelte";

const spaces = [
  entry(),
  entry({ ulid: "W2", name: "shared", prefix: "SH" }),
  entry({ store: "backend", ulid: "B1", name: "api", prefix: "API" }),
];

function setup(opts: { two?: boolean; store?: string } = {}) {
  const k = makeCore({ workspaces: spaces });
  k.stores.stores.mockReturnValue(
    (opts.two
      ? [storeItem("shared", true), storeItem("backend")]
      : [storeItem("shared", true)]) as never,
  );
  k.api.POST.mockImplementation(() =>
    ok(written({ ulid: "W9", name: "ops", id_prefix: "OPS" }, { commit: "c1" })),
  );
  const onclose = vi.fn();
  return { k, onclose, ...mount(WorkspaceAdd, { store: opts.store, onclose }, k) };
}

async function fillIn(name: string, prefix: string) {
  await fireEvent.input(screen.getByRole("textbox", { name: "名前" }), { target: { value: name } });
  await fireEvent.input(screen.getByRole("textbox", { name: "ID の接頭辞" }), {
    target: { value: prefix },
  });
}

describe("WorkspaceAdd", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("adds the workspace to the store and says where to open it", async () => {
    const { k, onclose } = setup();
    expect(screen.getByRole("dialog", { name: "shared にワークスペースを追加" })).toBeVisible();
    expect(screen.queryByRole("radiogroup", { name: "Store" })).toBeNull();
    await fillIn("ops", "ops"); // the prefix is typed in capitals
    expect(screen.getByRole("textbox", { name: "ID の接頭辞" })).toHaveValue("OPS");
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh workspace add ops --prefix OPS --store shared",
    );
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.POST).toHaveBeenCalledWith("/api/workspaces", {
      params: { query: { store: "shared" } },
      body: { name: "ops", prefix: "OPS" },
    });
    expect(k.stores.reload).toHaveBeenCalled();
    expect(toasts.list[0]).toMatchObject({
      message: "ワークスペース ops（OPS）を shared に追加しました",
      link: { label: "開く", href: "/workspaces/shared/W9" },
    });
  });

  it("cannot add until both fields are good, and says why next to the field", async () => {
    const { k } = setup();
    const add = screen.getByRole("button", { name: "追加" });
    expect(add).toBeDisabled();
    // nothing is scolded before the first try
    expect(screen.queryByText("名前を入れてください")).toBeNull();

    await fillIn("Ops", "SH");
    expect(screen.getByText(/名前は小文字の英数字と/)).toBeInTheDocument();
    expect(
      screen.getByText("SH は同じ Store のワークスペース shared が使っています"),
    ).toBeInTheDocument();
    await fillIn("ops", "API");
    expect(
      screen.getByText(
        "API はマウント中の backend のワークスペース api が使っています。別の接頭辞にしてください",
      ),
    ).toBeInTheDocument();
    expect(add).toBeDisabled();
    expect(k.api.POST).not.toHaveBeenCalled();
  });

  it("shows the command with <名前> and cannot copy it until the fields are good", () => {
    setup();
    const cmd = screen.getByRole("button", { name: /コマンドをコピー/ });
    expect(cmd).toHaveTextContent("hh workspace add <名前> --prefix <接頭辞> --store shared");
    expect(cmd).toBeDisabled();
  });

  it("asks for the Store only when there are two, and starts with the one asked for", async () => {
    const { k } = setup({ two: true, store: "backend" });
    expect(screen.getByRole("dialog", { name: "backend にワークスペースを追加" })).toBeVisible();
    expect(screen.getByRole("radio", { name: "backend" })).toBeChecked();
    // the prefix of api is taken in backend, but not in shared
    await fillIn("ops", "API");
    expect(screen.getByRole("button", { name: "追加" })).toBeDisabled();
    await fireEvent.click(screen.getByRole("radio", { name: "shared" }));
    expect(screen.getByRole("dialog", { name: "shared にワークスペースを追加" })).toBeVisible();
    expect(
      screen.getByText(
        "API はマウント中の backend のワークスペース api が使っています。別の接頭辞にしてください",
      ),
    ).toBeInTheDocument();
    await fillIn("ops", "OPS");
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    await waitFor(() => expect(k.api.POST).toHaveBeenCalled());
    expect(k.api.POST.mock.calls[0]?.[1]).toMatchObject({ params: { query: { store: "shared" } } });
  });

  it("starts with the default Store when none is asked for", () => {
    setup({ two: true });
    expect(screen.getByRole("radio", { name: "shared" })).toBeChecked();
  });

  it("keeps the dialog open and shows the words of the server when it refuses", async () => {
    const { k, onclose } = setup();
    k.api.POST.mockImplementation(() => fail(409, "workspace.prefix_taken", { prefix: "OPS" }));
    await fillIn("ops", "OPS");
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "ID の接頭辞 OPS はすでに使われています",
    );
    expect(onclose).not.toHaveBeenCalled();
    expect(toasts.list).toHaveLength(0);
  });

  it("adds with Enter in a field, but not while an IME composes", async () => {
    const { k } = setup();
    await fillIn("ops", "OPS");
    const field = screen.getByRole("textbox", { name: "名前" });
    await fireEvent.keyDown(field, { key: "Enter", isComposing: true });
    await fireEvent.keyDown(field, { key: "Enter", keyCode: 229 });
    expect(k.api.POST).not.toHaveBeenCalled();
    await fireEvent.keyDown(field, { key: "Enter" });
    await waitFor(() => expect(k.api.POST).toHaveBeenCalledTimes(1));
  });

  it("focuses the name as soon as the dialog opens", async () => {
    setup();
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.getByRole("textbox", { name: "名前" })).toHaveFocus();
  });
});
```

新しいファイル `web/app/workspace/PrefixChange.test.ts`:

```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fail, makeCore, mount, ok } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import PrefixChange from "./PrefixChange.svelte";
import { entry, written } from "./testkit";

const release = entry(); // 24 tasks and 1 deleted: 25 are renumbered
const others = [
  release,
  entry({ ulid: "W2", name: "shared", prefix: "SH" }),
  entry({ store: "backend", ulid: "B1", name: "api", prefix: "API" }),
];

function setup(target = release) {
  const k = makeCore({ workspaces: [target, ...others.slice(1)] });
  k.api.PATCH.mockImplementation(() =>
    ok(
      written(
        { id_prefix: "RLS" },
        {
          commit: "c2",
          ids: [
            { old: "REL-1", new: "RLS-1" },
            { old: "REL-2", new: "RLS-2" },
          ],
        },
      ),
    ),
  );
  const onclose = vi.fn();
  return { k, onclose, ...mount(PrefixChange, { entry: target, onclose }, k) };
}

const prefixField = () => screen.getByRole("textbox", { name: "新しい接頭辞" });

describe("PrefixChange", () => {
  beforeEach(() => resetToasts());

  it("says how many tasks are renumbered, the deleted ones included", () => {
    setup();
    expect(screen.getByRole("dialog", { name: "release の接頭辞を変更" })).toBeVisible();
    expect(
      screen.getByText(/配下の 25 件（削除済みを含む）の ID を新しい接頭辞で振り直します/),
    ).toBeInTheDocument();
    expect(screen.getByText(/古い ID は former_ids に残る/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "25 件を振り直す" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh workspace set-prefix release <接頭辞> --store shared",
    );
  });

  it("changes the prefix in one write and says how many ids changed", async () => {
    const { k, onclose } = setup();
    await fireEvent.input(prefixField(), { target: { value: "rls" } });
    expect(prefixField()).toHaveValue("RLS");
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh workspace set-prefix release RLS --store shared",
    );
    await fireEvent.click(screen.getByRole("button", { name: "25 件を振り直す" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.PATCH).toHaveBeenCalledWith("/api/workspaces/{id}", {
      params: { path: { id: "W1" }, query: { store: "shared" } },
      body: { prefix: "RLS" },
    });
    // the tasks.changed of this commit is ours: the rows must not read "CLI で更新"
    expect(k.cache.noteOwnCommit).toHaveBeenCalledWith("c2");
    expect(k.stores.reload).toHaveBeenCalled();
    expect(toasts.list[0]?.message).toBe(
      "release の接頭辞を REL から RLS に変えました（2 件の ID を振り直しました）",
    );
  });

  it("refuses the current prefix and the prefixes that are taken", async () => {
    const { k } = setup();
    const go = screen.getByRole("button", { name: "25 件を振り直す" });
    await fireEvent.input(prefixField(), { target: { value: "REL" } });
    expect(screen.getByText("今と同じ接頭辞です")).toBeInTheDocument();
    await fireEvent.input(prefixField(), { target: { value: "SH" } });
    expect(
      screen.getByText("SH は同じ Store のワークスペース shared が使っています"),
    ).toBeInTheDocument();
    await fireEvent.input(prefixField(), { target: { value: "API" } });
    expect(screen.getByText(/API はマウント中の backend のワークスペース api/)).toBeInTheDocument();
    await fireEvent.input(prefixField(), { target: { value: "r" } });
    expect(screen.getByText(/2〜10 文字/, { selector: ".bad" })).toBeInTheDocument();
    expect(go).toBeDisabled();
    await fireEvent.keyDown(prefixField(), { key: "Enter" });
    expect(k.api.PATCH).not.toHaveBeenCalled();
  });

  it("has a plain button when the workspace has no task", () => {
    setup(entry({ tasks: 0, deleted: 0 }));
    expect(
      screen.getByText("配下にタスクはありません。接頭辞だけが変わります。"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "変更" })).toBeDisabled();
  });

  it("keeps the dialog open and shows the words of the server when it refuses", async () => {
    const { k, onclose } = setup();
    k.api.PATCH.mockImplementation(() => fail(409, "workspace.prefix_taken", { prefix: "OPS" }));
    await fireEvent.input(prefixField(), { target: { value: "OPS" } });
    await fireEvent.keyDown(prefixField(), { key: "Enter" });
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "ID の接頭辞 OPS はすでに使われています",
    );
    expect(onclose).not.toHaveBeenCalled();
    expect(k.cache.noteOwnCommit).not.toHaveBeenCalled();
  });

  it("counts the tasks as they are now, not as they were when the dialog opened", async () => {
    const { k } = setup();
    const now = [{ ...release, tasks: 30, deleted: 2 }, ...others.slice(1)];
    k.stores.workspaces.mockReturnValue(now);
    k.notify();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "32 件を振り直す" })).toBeInTheDocument(),
    );
  });
});
```

新しいファイル `web/app/workspace/WorkspaceDelete.test.ts`:

```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fail, makeCore, mount, ok } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import { entry, written } from "./testkit";
import WorkspaceDelete from "./WorkspaceDelete.svelte";

const sandbox = entry({ ulid: "W3", name: "sandbox", prefix: "SBX", tasks: 0, deleted: 0 });

function setup(target = sandbox) {
  const k = makeCore({ workspaces: [entry(), target] });
  k.api.DELETE.mockImplementation(() =>
    ok(written({ ulid: "W3", name: "sandbox", id_prefix: "SBX" })),
  );
  const onclose = vi.fn();
  return { k, onclose, ...mount(WorkspaceDelete, { entry: target, onclose }, k) };
}

describe("WorkspaceDelete", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("removes an empty workspace", async () => {
    const { k, onclose } = setup();
    expect(screen.getByRole("dialog", { name: "ワークスペース sandbox を削除" })).toBeVisible();
    expect(screen.getByText(/にはタスクがありません/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh workspace rm sandbox --store shared",
    );
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/workspaces/{id}", {
      params: { path: { id: "W3" }, query: { store: "shared" } },
    });
    expect(k.stores.reload).toHaveBeenCalled();
    expect(toasts.list[0]?.message).toBe("ワークスペース sandbox を削除しました");
  });

  it("leaves the screen of the workspace that is removed", async () => {
    history.replaceState(null, "", "/workspaces/shared/W3/settings");
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/");
  });

  it("stays on the screen of another workspace", async () => {
    history.replaceState(null, "", "/workspaces/shared/W1/settings");
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/workspaces/shared/W1/settings");
  });

  it("says why it cannot be removed while tasks remain, the deleted ones included", async () => {
    const { k } = setup(
      entry({ ulid: "W3", name: "sandbox", prefix: "SBX", tasks: 24, deleted: 1 }),
    );
    expect(
      screen.getByText(
        "タスク 25 件（削除済み 1 件を含む）が残っているため削除できません。別のワークスペースへ移すか完全に削除すると消せます",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "削除" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toBeDisabled();
    expect(k.api.DELETE).not.toHaveBeenCalled();
  });

  it("is blocked by a task that arrives while the dialog is open", async () => {
    const { k } = setup();
    k.stores.workspaces.mockReturnValue([entry(), { ...sandbox, tasks: 1 }]);
    k.notify();
    await waitFor(() => expect(screen.getByRole("button", { name: "削除" })).toBeDisabled());
    expect(screen.getByText(/タスク 1 件が残っているため削除できません/)).toBeInTheDocument();
  });

  it("keeps the dialog open and shows the words of the server when it refuses (409 as a safety net)", async () => {
    const { k, onclose } = setup();
    k.api.DELETE.mockImplementation(() => fail(409, "workspace.not_empty", { tasks: 2 }));
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "ワークスペースにタスクが 2 件（削除済みを含む）残っています",
    );
    expect(onclose).not.toHaveBeenCalled();
    expect(toasts.list).toHaveLength(0);
  });
});
```

新しいファイル `web/app/workspace/WorkspaceMove.test.ts`:

```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fail, makeCore, mount, ok, taskView } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import { entry } from "./testkit";
import WorkspaceMove from "./WorkspaceMove.svelte";

const task = taskView({
  ulid: "U6",
  id: "REL-6",
  title: "Web の API クライアントを v2 に更新",
  store: "shared",
});
const moved = taskView({
  ulid: "U6",
  id: "SH-12",
  title: task.title,
  workspace: { ulid: "W2", name: "shared", id_prefix: "SH" },
});

const spaces = [
  entry(),
  entry({ ulid: "W2", name: "shared", prefix: "SH" }),
  entry({ ulid: "W3", name: "ops", prefix: "OPS" }),
  entry({ store: "backend", ulid: "B1", name: "api", prefix: "API" }),
];

function setup(list = spaces) {
  const k = makeCore({ workspaces: list });
  k.api.POST.mockImplementation(() => ok({ task: moved, commit: "c3", detached: null }));
  const onclose = vi.fn();
  return { k, onclose, ...mount(WorkspaceMove, { task, onclose }, k) };
}

describe("WorkspaceMove", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("lists the workspaces of the store of the task, and the one it is in is not a choice", () => {
    setup();
    expect(
      screen.getByRole("dialog", { name: "REL-6 Web の API クライアントを v2 に更新 を移動" }),
    ).toBeVisible();
    const group = screen.getByRole("radiogroup", { name: "移り先のワークスペース" });
    expect(group).toHaveTextContent("release（REL） — 今のワークスペース");
    expect(screen.getByRole("radio", { name: /^release/ })).toBeDisabled();
    expect(screen.getByRole("radio", { name: "shared（SH）" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "ops（OPS）" })).not.toBeChecked();
    expect(screen.queryByRole("radio", { name: /api/ })).toBeNull(); // another store
  });

  it("explains the move, with the prefix of the workspace picked", async () => {
    setup();
    expect(
      screen.getByText("移り先のルートに置きます。配下のタスクも一緒に移ります"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("ID を SH の番号に振り直します（REL-6 は former_ids に残ります）"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("スプリント・リリースが移り先のものでなければ外れます"),
    ).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("radio", { name: "ops（OPS）" }));
    expect(
      screen.getByText("ID を OPS の番号に振り直します（REL-6 は former_ids に残ります）"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh mv REL-6 --workspace ops",
    );
  });

  it("moves the task and opens the way to it in the toast", async () => {
    const { k, onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "移動" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/move", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
      body: { workspace: "shared" },
    });
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ task: moved, commit: "c3" });
    expect(toasts.list[0]).toMatchObject({
      message: "REL-6 を shared に移しました（今の ID は SH-12）",
      link: { label: "開く", href: "/tasks/U6" },
    });
  });

  it("names what the move took off the task", async () => {
    const { k, onclose } = setup();
    k.api.POST.mockImplementation(() =>
      ok({
        task: moved,
        commit: "c3",
        detached: [{ task: "U6", id: "SH-12", sprint: "Sprint 44", releases: ["v2.4"] }],
      }),
    );
    await fireEvent.click(screen.getByRole("button", { name: "移動" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(toasts.list[0]?.message).toBe(
      "REL-6 を shared に移しました（今の ID は SH-12）。SH-12 はスプリント Sprint 44 とリリース v2.4 から外れました",
    );
  });

  it("has nowhere to go when the store has no other workspace", () => {
    setup([entry(), entry({ store: "backend", ulid: "B1", name: "api", prefix: "API" })]);
    expect(
      screen.getByText("この Store（shared）にはほかのワークスペースがありません。"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("radiogroup")).toBeNull();
    expect(screen.getByRole("button", { name: "移動" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toBeDisabled();
  });

  it("keeps the dialog open and shows the words of the server when it refuses", async () => {
    const { k, onclose } = setup();
    k.api.POST.mockImplementation(() => fail(400, "move.invalid", {}, "task is deleted"));
    await fireEvent.click(screen.getByRole("button", { name: "移動" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "ここには移せません: task is deleted",
    );
    expect(onclose).not.toHaveBeenCalled();
    expect(k.cache.applyWrite).not.toHaveBeenCalled();
  });
});
```

新しいファイル `web/app/workspace/WorkspaceDialogs.test.ts`:

```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { flushSync } from "svelte";
import { afterEach, describe, expect, it } from "vitest";
import { makeCore, mount, taskView } from "../detail/testkit";
import {
  closeWorkspaceDialog,
  openPrefixChange,
  openWorkspaceAdd,
  openWorkspaceDelete,
  openWorkspaceMove,
  workspaceDialog,
} from "./dialogs-state.svelte";
import { entry } from "./testkit";
import WorkspaceDialogs from "./WorkspaceDialogs.svelte";

function setup() {
  const k = makeCore({
    workspaces: [entry(), entry({ ulid: "W2", name: "shared", prefix: "SH" })],
  });
  return mount(WorkspaceDialogs, {}, k);
}

describe("WorkspaceDialogs", () => {
  afterEach(() => closeWorkspaceDialog());

  it("draws nothing until a dialog is asked for", () => {
    setup();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(workspaceDialog.current).toBeNull();
  });

  it("draws the dialog that is asked for", () => {
    setup();
    openWorkspaceAdd("shared");
    flushSync();
    expect(screen.getByRole("dialog", { name: "shared にワークスペースを追加" })).toBeVisible();
    openPrefixChange(entry());
    flushSync();
    expect(screen.getByRole("dialog", { name: "release の接頭辞を変更" })).toBeVisible();
    openWorkspaceDelete(entry({ tasks: 0, deleted: 0 }));
    flushSync();
    expect(screen.getByRole("dialog", { name: "ワークスペース release を削除" })).toBeVisible();
    openWorkspaceMove(taskView({ id: "REL-6", title: "t" }));
    flushSync();
    expect(screen.getByRole("dialog", { name: "REL-6 t を移動" })).toBeVisible();
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
  });

  it("opens no delete confirmation for a workspace with tasks: its settings tab says why", () => {
    history.replaceState(null, "", "/");
    setup();
    openWorkspaceDelete(entry()); // 24 tasks and 1 deleted
    flushSync();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(workspaceDialog.current).toBeNull();
    expect(location.pathname).toBe("/workspaces/shared/W1/settings");
  });

  it("closes with Esc and with the close button", async () => {
    setup();
    openWorkspaceAdd();
    flushSync();
    await fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    openWorkspaceAdd();
    flushSync();
    await fireEvent.click(screen.getByRole("button", { name: "閉じる" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("does not close when the dim background is clicked", async () => {
    setup();
    openWorkspaceAdd();
    flushSync();
    const overlay = document.querySelector(".overlay") as HTMLElement;
    await fireEvent.click(overlay);
    expect(screen.getByRole("dialog")).toBeVisible();
  });
});
```

新しいファイル `web/app/detail/TaskDetail.more.test.ts`:

```ts
import { fireEvent, screen } from "@testing-library/svelte";
import { afterEach, beforeAll, describe, expect, it } from "vitest";
import { closeWorkspaceDialog, workspaceDialog } from "../workspace/dialogs-state.svelte";
import TaskDetail from "./TaskDetail.svelte";
import { makeCore, mount, showView, stubLayout, workspaces } from "./testkit";

const U = "01JBZ8K3QW7M2X9V4T6R5N8P0A";
const show = showView({
  ulid: U,
  id: "REL-6",
  title: "Web の API クライアントを v2 に更新",
  workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
});

describe("TaskDetail: the … menu", () => {
  beforeAll(stubLayout);
  afterEach(() => closeWorkspaceDialog());

  it("opens the move dialog for this task", async () => {
    const k = makeCore({ show });
    mount(TaskDetail, { ulid: U }, k);
    await fireEvent.click(screen.getByRole("button", { name: "その他の操作" }));
    await fireEvent.click(
      await screen.findByRole("menuitem", { name: /別のワークスペースへ移動/ }),
    );
    expect(screen.queryByRole("menu")).toBeNull();
    expect(workspaceDialog.current).toMatchObject({ kind: "move", task: { ulid: U, id: "REL-6" } });
  });

  it("turns the item off, and says why, when the store has no other workspace", async () => {
    const k = makeCore({ show, workspaces: [workspaces[0] as (typeof workspaces)[number]] });
    mount(TaskDetail, { ulid: U }, k);
    await fireEvent.click(screen.getByRole("button", { name: "その他の操作" }));
    const item = await screen.findByRole("menuitem", { name: /別のワークスペースへ移動/ });
    expect(item).toBeDisabled();
    expect(item).toHaveTextContent("この Store にはほかのワークスペースがありません");
  });

  it("has no menu on a deleted task", () => {
    const k = makeCore({ show: { ...show, deleted_at: "2026-11-03T10:00:00Z" } });
    mount(TaskDetail, { ulid: U }, k);
    expect(screen.queryByRole("button", { name: "その他の操作" })).toBeNull();
  });
});
```

`web/app/tree/TreeView.test.ts` に足す。import（`import { emitter } from "../test-utils";` の次）:

```ts
import { closeWorkspaceDialog, workspaceDialog } from "../workspace/dialogs-state.svelte";
```

テスト（`it("says why Space does nothing when the schema has no done status", …)` の直前）:

```ts
  it("opens the move dialog from the … menu of a row", async () => {
    const { core } = setup();
    const w = (ulid: string, name: string, prefix: string) => ({
      store: "shared",
      tone: "purple",
      ulid,
      name,
      prefix,
      projects: [],
      tasks: 1,
      deleted: 0,
      prefixSet: "2026-10-29T12:00:00Z",
    });
    core.stores.workspaces = () => [w("W1", "release", "REL"), w("W2", "shared", "SH")] as never;
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(
      await screen.findByRole("button", { name: "メニュー: Web のクライアントを更新" }),
    );
    await fireEvent.click(
      await screen.findByRole("menuitem", { name: /別のワークスペースへ移動/ }),
    );
    expect(workspaceDialog.current).toMatchObject({ kind: "move", task: { ulid: "T3" } });
    closeWorkspaceDialog();
  });

  it("turns the move item off when the store has no other workspace", async () => {
    setup(); // one workspace only
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(
      await screen.findByRole("button", { name: "メニュー: Web のクライアントを更新" }),
    );
    expect(
      await screen.findByRole("menuitem", { name: /別のワークスペースへ移動/ }),
    ).toBeDisabled();
    expect(workspaceDialog.current).toBeNull();
  });
```

- [ ] **Step 2: 失敗を確かめる**

```bash
cd web && pnpm test core/cli.test.ts core/workspace.test.ts app/workspace app/detail/TaskDetail.more.test.ts app/tree/TreeView.test.ts
```

期待: `./workspace`・`./dialogs-state.svelte`・`./move-menu` などが解決できない、`cliWorkspaceRename is not a function` などで失敗する。

- [ ] **Step 3: 実装する**

`web/core/cli.ts` の `cliMv` を直す（`Arg` は同じファイルにある）。

```diff
-  to: { parent?: string; root?: boolean; before?: string; after?: string },
+  to: { parent?: string; root?: boolean; before?: string; after?: string; workspace?: Arg },
 ): string {
-  const a = ["mv", id];
+  const a: Arg[] = ["mv", id];
   if (to.parent) a.push("--parent", to.parent);
+  if (to.workspace) a.push("--workspace", to.workspace);
   if (to.root) a.push("--root");
```

`cliStoreInit` の直前に足す。

```ts
// cliWorkspaceRename is hh workspace rename <workspace> <name>.
export function cliWorkspaceRename(i: { workspace: string; name: Arg; store: string }): string {
  return withPositionals(
    ["workspace", "rename"],
    [i.workspace, i.name],
    ["--store", i.store],
    true,
  );
}

// cliWorkspaceSetPrefix is hh workspace set-prefix <workspace> <P>.
export function cliWorkspaceSetPrefix(i: {
  workspace: string;
  prefix: Arg;
  store: string;
}): string {
  return withPositionals(
    ["workspace", "set-prefix"],
    [i.workspace, i.prefix],
    ["--store", i.store],
    true,
  );
}

export const cliWorkspaceRm = (i: { workspace: string; store: string }): string =>
  cliLine(["workspace", "rm", i.workspace, "--store", i.store]);
```

新しいファイル `web/core/workspace.ts`:

```ts
import type { Schemas } from "./api/client";
import type { WorkspaceEntry } from "./stores";

// The same rules as ValidName and ValidIDPrefix in internal/model.
const NAME = /^[a-z0-9][a-z0-9_-]*$/;
const PREFIX = /^[A-Z][A-Z0-9]{1,9}$/;

export const NAME_RULE = "名前は小文字の英数字と - _ で、英数字から始めてください";
export const PREFIX_RULE = "接頭辞は英大文字で始まる 2〜10 文字の英大文字と数字です";

type Known = Pick<WorkspaceEntry, "store" | "ulid" | "name" | "prefix">;

// nameProblem says why name cannot name a workspace of store, or null. self is the workspace that is
// being renamed: its own name does not count as taken. The server checks the same (workspace.bad_name,
// workspace.taken); this is for the words next to the field.
export function nameProblem(
  name: string,
  store: string,
  all: Known[],
  self?: string,
): string | null {
  if (name === "") return "名前を入れてください";
  if (!NAME.test(name)) return NAME_RULE;
  const lower = name.toLowerCase();
  const taken = all.some(
    (w) => w.store === store && w.ulid !== self && w.name.toLowerCase() === lower,
  );
  return taken ? `${name} は同じ Store のワークスペースが使っています` : null;
}

// prefixProblem is nameProblem for the id prefix. The prefixes of the other stores that are mounted
// count too (workspace.prefix_taken).
export function prefixProblem(
  prefix: string,
  store: string,
  all: Known[],
  self?: string,
): string | null {
  if (prefix === "") return "接頭辞を入れてください";
  if (!PREFIX.test(prefix)) return PREFIX_RULE;
  const same = all.find((w) => w.store === store && w.ulid !== self && w.prefix === prefix);
  if (same) return `${prefix} は同じ Store のワークスペース ${same.name} が使っています`;
  const other = all.find((w) => w.store !== store && w.prefix === prefix);
  if (other) {
    return `${prefix} はマウント中の ${other.store} のワークスペース ${other.name} が使っています。別の接頭辞にしてください`;
  }
  return null;
}

// removeBlocker is why a workspace cannot be removed, or null when it can: a workspace goes only when
// it has no task at all, deleted ones included (the server answers 409 workspace.not_empty otherwise).
export function removeBlocker(w: Pick<WorkspaceEntry, "tasks" | "deleted">): string | null {
  const n = w.tasks + w.deleted;
  if (n === 0) return null;
  const deleted = w.deleted > 0 ? `（削除済み ${w.deleted} 件を含む）` : "";
  return `タスク ${n} 件${deleted}が残っているため削除できません。別のワークスペースへ移すか完全に削除すると消せます`;
}

// detachedText says what a move took off the tasks (the sprint and the releases that are not of the
// new workspace); "" when nothing.
export function detachedText(ds: Schemas["Detached"][] | null | undefined): string {
  return (ds ?? [])
    .map((d) => {
      const parts: string[] = [];
      if (d.sprint) parts.push(`スプリント ${d.sprint}`);
      if (d.releases && d.releases.length > 0) parts.push(`リリース ${d.releases.join("、")}`);
      return parts.length > 0 ? `${d.id} は${parts.join(" と")} から外れました` : "";
    })
    .filter((t) => t !== "")
    .join("。");
}
```

新しいファイル `web/app/workspace/dialogs-state.svelte.ts`:

```ts
import type { TaskView } from "../../core/api/client";
import type { WorkspaceEntry } from "../../core/stores";
import { removeBlocker } from "../../core/workspace";
import { router } from "../router";

/** What a move needs of a task: a TaskView has all of it. */
export type MoveTask = Pick<TaskView, "ulid" | "id" | "title" | "store" | "workspace">;

/** The workspace dialog that is asked for; WorkspaceDialogs draws it. */
export type WorkspaceDialog =
  | { kind: "add"; store?: string }
  | { kind: "delete"; entry: WorkspaceEntry }
  | { kind: "prefix"; entry: WorkspaceEntry }
  | { kind: "move"; task: MoveTask };

// raw: the entries and tasks are the ones the directory and the cache hold, not copies of them.
let current = $state.raw<WorkspaceDialog | null>(null);

export const workspaceDialog = {
  get current(): WorkspaceDialog | null {
    return current;
  },
};

/** store is the Store the dialog starts with; without it, the default Store. */
export function openWorkspaceAdd(store?: string): void {
  current = { kind: "add", store };
}

/**
 * The confirmation opens only for a workspace with no task at all (Dialogs). With tasks left it says
 * why on the settings tab of the workspace, so that is where it goes.
 */
export function openWorkspaceDelete(entry: WorkspaceEntry): void {
  if (removeBlocker(entry) !== null) {
    router.go(
      router.href({ name: "workspace", store: entry.store, ulid: entry.ulid, tab: "settings" }),
    );
    return;
  }
  current = { kind: "delete", entry };
}

export function openPrefixChange(entry: WorkspaceEntry): void {
  current = { kind: "prefix", entry };
}

export function openWorkspaceMove(task: MoveTask): void {
  current = { kind: "move", task };
}

export function closeWorkspaceDialog(): void {
  current = null;
}
```

新しいファイル `web/app/workspace/move-menu.ts`:

```ts
import type { TaskView } from "../../core/api/client";
import type { WorkspaceEntry } from "../../core/stores";
import type { MenuItem } from "../ui/Menu.svelte";

/** The value of the menu item that moves a task to another workspace. */
export const MOVE_VALUE = "move-workspace";

/**
 * The menu item "別のワークスペースへ移動…" of the task detail and of a row of the tree. A task goes to
 * another workspace of its own Store only, so the item is off when the Store has no other one.
 */
export function moveWorkspaceItem(
  task: Pick<TaskView, "store" | "workspace">,
  entries: WorkspaceEntry[],
): MenuItem {
  const others = entries.filter((w) => w.store === task.store && w.ulid !== task.workspace.ulid);
  return {
    value: MOVE_VALUE,
    label: "別のワークスペースへ移動…",
    disabled: others.length === 0,
    hint: others.length === 0 ? "この Store にはほかのワークスペースがありません" : undefined,
  };
}
```

新しいファイル `web/app/workspace/WorkspaceAdd.svelte`:

```svelte
<script lang="ts">
  import { unwrap } from "../../core/api/client";
  import { cliWorkspaceAdd, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { nameProblem, prefixProblem } from "../../core/workspace";
  import { core } from "../context";
  import { router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";

  // store is the Store the dialog starts with (the settings page of that Store opened it).
  let { store: asked, onclose }: { store?: string; onclose: () => void } = $props();

  const { api, stores } = core();
  const uid = $props.id();
  const dir = useSubscribe(stores, () => ({
    items: stores.stores(),
    workspaces: stores.workspaces(),
  }));

  // The Store picked here, else the one asked for, else the default one, else the first.
  let picked = $state("");
  const store = $derived(
    dir.current.items.find((s) => s.name === picked)?.name ??
      dir.current.items.find((s) => s.name === asked)?.name ??
      dir.current.items.find((s) => s.default)?.name ??
      dir.current.items[0]?.name ??
      "",
  );

  let name = $state("");
  let prefix = $state("");
  let touched = $state(false);
  let busy = $state(false);
  let failure = $state<string | null>(null);

  const nameError = $derived(nameProblem(name, store, dir.current.workspaces));
  const prefixError = $derived(prefixProblem(prefix, store, dir.current.workspaces));
  const invalid = $derived(store === "" || nameError !== null || prefixError !== null);
  // An empty field is not scolded before the first try.
  const nameShown = $derived(name === "" && !touched ? null : nameError);
  const prefixShown = $derived(prefix === "" && !touched ? null : prefixError);

  const cli = $derived(
    cliWorkspaceAdd({
      name: name === "" ? new Placeholder("名前") : name,
      prefix: prefix === "" ? new Placeholder("接頭辞") : prefix,
      store,
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
      const r = await unwrap(
        api.POST("/api/workspaces", { params: { query: { store } }, body: { name, prefix } }),
      );
      // The workspace exists now; a failed reload must not read as a failed add (the SSE catches up).
      await stores.reload().catch(() => undefined);
      toast({
        message: `ワークスペース ${r.workspace.name}（${r.workspace.id_prefix}）を ${store} に追加しました`,
        link: {
          label: "開く",
          href: router.href({
            name: "workspace",
            store,
            ulid: r.workspace.ulid,
            tab: "overview",
          }),
        },
      });
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

<Dialog
  open
  title={store === "" ? "ワークスペースを追加" : `${store} にワークスペースを追加`}
  width={560}
  {cli}
  cliDisabled={invalid}
  onclose={close}
>
  <div class="form" role="presentation" {onkeydown}>
    {#if dir.current.items.length >= 2}
      <div class="field" role="radiogroup" aria-label="Store">
        <span class="lbl">Store</span>
        <div class="stores">
          {#each dir.current.items as s (s.name)}
            <!-- biome-ignore lint/a11y/noLabelWithoutControl: the badge is the text of the label -->
            <label class="choice" class:on={s.name === store}>
              <input
                type="radio"
                name="{uid}-store"
                value={s.name}
                checked={s.name === store}
                onchange={() => (picked = s.name)}
              >
              <StoreBadge store={s.name} tone={stores.tone(s.name)} />
            </label>
          {/each}
        </div>
      </div>
    {/if}

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
        {nameShown ?? "小文字の英数字と - _ が使えます。同じ Store の名前とは重ねられません"}
      </span>
    </div>

    <div class="field">
      <label class="lbl" for="{uid}-prefix">ID の接頭辞</label>
      <input
        id="{uid}-prefix"
        class="mono"
        type="text"
        autocomplete="off"
        value={prefix}
        oninput={(e) => (prefix = e.currentTarget.value.toUpperCase())}
        aria-invalid={prefixShown ? "true" : undefined}
        aria-describedby="{uid}-prefix-msg"
      >
      <span id="{uid}-prefix-msg" class="msg" class:bad={prefixShown}>
        {prefixShown ??
          "接頭辞は英大文字で始まる 2〜10 文字の英大文字と数字です。同じ Store とマウント中の Store のワークスペースとは重ねられません"}
      </span>
    </div>

    {#if failure}
      <p class="failure" role="alert">{failure}</p>
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy || invalid} onclick={() => void submit()}>
      {busy ? "追加中…" : "追加"}
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
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-13);
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
  .msg {
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.55;
  }
  .bad {
    color: var(--danger);
  }
  .failure {
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--danger-weak);
    color: var(--danger-ink);
    line-height: 1.6;
  }
</style>
```

新しいファイル `web/app/workspace/PrefixChange.svelte`:

```svelte
<script lang="ts">
  import { unwrap } from "../../core/api/client";
  import { cliWorkspaceSetPrefix, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { WorkspaceEntry } from "../../core/stores";
  import { prefixProblem } from "../../core/workspace";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";

  let { entry, onclose }: { entry: WorkspaceEntry; onclose: () => void } = $props();

  const { api, cache, stores } = core();
  const uid = $props.id();
  const all = useSubscribe(stores, () => stores.workspaces());
  // The counts move while the dialog is open: read the workspace as it is now.
  const w = $derived(
    all.current.find((x) => x.store === entry.store && x.ulid === entry.ulid) ?? entry,
  );
  // The tasks that are renumbered: the deleted ones too.
  const count = $derived(w.tasks + w.deleted);

  let prefix = $state("");
  let busy = $state(false);
  let failure = $state<string | null>(null);

  const problem = $derived(
    prefix === w.prefix
      ? "今と同じ接頭辞です"
      : prefixProblem(prefix, w.store, all.current, w.ulid),
  );
  // An empty field is not scolded.
  const shown = $derived(prefix === "" ? null : problem);

  const cli = $derived(
    cliWorkspaceSetPrefix({
      workspace: w.name,
      prefix: prefix === "" ? new Placeholder("接頭辞") : prefix,
      store: w.store,
    }),
  );

  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    if (busy || problem !== null) return;
    busy = true;
    failure = null;
    const from = w.prefix;
    try {
      const r = await unwrap(
        api.PATCH("/api/workspaces/{id}", {
          params: { path: { id: w.ulid }, query: { store: w.store } },
          body: { prefix },
        }),
      );
      // The commit renumbers the tasks: its tasks.changed is this tab's own.
      cache.noteOwnCommit(r.commit);
      await stores.reload().catch(() => undefined);
      const n = r.ids?.length ?? 0;
      toast({
        message: `${w.name} の接頭辞を ${from} から ${prefix} に変えました${
          n > 0 ? `（${n} 件の ID を振り直しました）` : ""
        }`,
      });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229) {
      e.preventDefault();
      void submit();
    }
  }
</script>

<Dialog
  open
  title="{w.name} の接頭辞を変更"
  width={560}
  {cli}
  cliDisabled={problem !== null}
  onclose={close}
>
  <div class="form">
    <div class="now">
      <span class="lbl">今</span>
      <span class="mono">{w.prefix}</span>
    </div>

    <div class="field">
      <label class="lbl" for="{uid}-prefix">新しい接頭辞</label>
      <input
        id="{uid}-prefix"
        class="mono"
        type="text"
        autocomplete="off"
        data-autofocus
        value={prefix}
        oninput={(e) => (prefix = e.currentTarget.value.toUpperCase())}
        {onkeydown}
        aria-invalid={shown ? "true" : undefined}
        aria-describedby="{uid}-msg"
      >
      <span id="{uid}-msg" class="msg" class:bad={shown}>
        {shown ?? "接頭辞は英大文字で始まる 2〜10 文字の英大文字と数字です"}
      </span>
    </div>

    {#if count > 0}
      <p class="note">
        配下の {count} 件（削除済みを含む）の ID を新しい接頭辞で振り直します（1 コミット）。
        古い ID は former_ids に残るので、検索やリンクでは引き続き引けます。
      </p>
    {:else}
      <p class="note">配下にタスクはありません。接頭辞だけが変わります。</p>
    {/if}

    {#if failure}
      <p class="failure" role="alert">{failure}</p>
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy || problem !== null} onclick={() => void submit()}>
      {busy ? "変更中…" : count > 0 ? `${count} 件を振り直す` : "変更"}
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }
  .now {
    display: flex;
    gap: var(--space-12);
    align-items: baseline;
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
  input {
    min-height: 40px;
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font-size: var(--fs-14);
  }
  input:focus-visible {
    outline: 2px solid var(--accent);
  }
  input[aria-invalid="true"] {
    border: 1.5px solid var(--danger);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-13);
  }
  .msg {
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.55;
  }
  .bad {
    color: var(--danger);
  }
  .note {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-13);
    line-height: 1.7;
  }
  .failure {
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--danger-weak);
    color: var(--danger-ink);
    line-height: 1.6;
  }
</style>
```

新しいファイル `web/app/workspace/WorkspaceDelete.svelte`:

```svelte
<script lang="ts">
  import { unwrap } from "../../core/api/client";
  import { cliWorkspaceRm } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { WorkspaceEntry } from "../../core/stores";
  import { removeBlocker } from "../../core/workspace";
  import { core } from "../context";
  import { hrefOf, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";

  let { entry, onclose }: { entry: WorkspaceEntry; onclose: () => void } = $props();

  const { api, stores } = core();
  const all = useSubscribe(stores, () => stores.workspaces());
  // The counts move while the dialog is open: read the workspace as it is now.
  const w = $derived(
    all.current.find((x) => x.store === entry.store && x.ulid === entry.ulid) ?? entry,
  );
  // A task added since the dialog opened keeps the workspace (the server answers 409 if this is late).
  const blocker = $derived(removeBlocker(w));

  let busy = $state(false);
  let failure = $state<string | null>(null);

  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    if (busy || blocker !== null) return;
    busy = true;
    failure = null;
    const gone = { ...w };
    try {
      await unwrap(
        api.DELETE("/api/workspaces/{id}", {
          params: { path: { id: gone.ulid }, query: { store: gone.store } },
        }),
      );
      await stores.reload().catch(() => undefined);
      // The screen of the workspace that is gone has nothing to show.
      const r = router.route;
      if (r.name === "workspace" && r.store === gone.store && r.ulid === gone.ulid) {
        router.go(hrefOf({ name: "home" }));
      }
      toast({ message: `ワークスペース ${gone.name} を削除しました` });
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
  title="ワークスペース {w.name} を削除"
  width={560}
  cli={cliWorkspaceRm({ workspace: w.name, store: w.store })}
  cliDisabled={blocker !== null}
  onclose={close}
>
  <div class="body">
    {#if blocker}
      <p class="blocked" role="note">{blocker}</p>
    {:else}
      <p class="note">
        {w.name}（<span class="mono">{w.prefix}</span>）にはタスクがありません。削除すると接頭辞
        <span class="mono">{w.prefix}</span>
        は空き、ほかのワークスペースで使えるようになります。コマンドパレットの「直前の操作を取り消す」（hh
        undo）で元に戻せます。
      </p>
    {/if}
    {#if failure}
      <p class="failure" role="alert">{failure}</p>
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="danger" disabled={busy || blocker !== null} onclick={() => void submit()}>
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
  .note,
  .blocked {
    margin: 0;
    font-size: var(--fs-13);
    line-height: 1.7;
  }
  .note {
    color: var(--ink-2);
  }
  .blocked {
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--warning-weak);
    color: var(--ink);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .failure {
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--danger-weak);
    color: var(--danger-ink);
    line-height: 1.6;
  }
</style>
```

新しいファイル `web/app/workspace/WorkspaceMove.svelte`:

```svelte
<script lang="ts">
  import { unwrap } from "../../core/api/client";
  import { cliMv, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { detachedText } from "../../core/workspace";
  import { core } from "../context";
  import { router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import type { MoveTask } from "./dialogs-state.svelte";

  let { task, onclose }: { task: MoveTask; onclose: () => void } = $props();

  const { api, cache, stores } = core();
  const uid = $props.id();
  const all = useSubscribe(stores, () => stores.workspaces());

  // A task goes to another workspace of its own Store: another Store is a different move.
  const choices = $derived(all.current.filter((w) => w.store === task.store));
  const here = $derived(task.workspace.ulid);
  let picked = $state("");
  const target = $derived(
    choices.find((w) => w.ulid === picked && w.ulid !== here) ??
      choices.find((w) => w.ulid !== here),
  );

  let busy = $state(false);
  let failure = $state<string | null>(null);

  const cli = $derived(
    cliMv(task.id, { workspace: target ? target.name : new Placeholder("ワークスペース") }),
  );

  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    if (busy || !target) return;
    busy = true;
    failure = null;
    const to = target;
    try {
      const r = await unwrap(
        api.POST("/api/tasks/{id}/move", {
          params: { path: { id: task.ulid }, query: { store: task.store } },
          body: { workspace: to.name },
        }),
      );
      cache.applyWrite({ task: r.task, commit: r.commit });
      const lost = detachedText(r.detached);
      toast({
        message: `${task.id} を ${to.name} に移しました（今の ID は ${r.task.id}）${lost ? `。${lost}` : ""}`,
        link: { label: "開く", href: router.href({ name: "task", ulid: task.ulid }) },
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
  title="{task.id} {task.title} を移動"
  width={620}
  {cli}
  cliDisabled={!target}
  onclose={close}
>
  <div class="form">
    {#if target}
      <div class="field" role="radiogroup" aria-label="移り先のワークスペース">
        <span class="lbl">移り先のワークスペース</span>
        {#each choices as w (w.ulid)}
          <label class="choice" class:on={w.ulid === target.ulid} class:here={w.ulid === here}>
            <input
              type="radio"
              name="{uid}-to"
              value={w.ulid}
              checked={w.ulid === target.ulid}
              disabled={w.ulid === here}
              onchange={() => (picked = w.ulid)}
            >
            <span>{w.name}（{w.prefix}）{w.ulid === here ? " — 今のワークスペース" : ""}</span>
          </label>
        {/each}
      </div>

      <ul class="notes">
        <li>移り先のルートに置きます。配下のタスクも一緒に移ります</li>
        <li>ID を {target.prefix} の番号に振り直します（{task.id} は former_ids に残ります）</li>
        <li>スプリント・リリースが移り先のものでなければ外れます</li>
      </ul>
    {:else}
      <p class="none">この Store（{task.store}）にはほかのワークスペースがありません。</p>
    {/if}

    {#if failure}
      <p class="failure" role="alert">{failure}</p>
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy || !target} onclick={() => void submit()}>
      {busy ? "移動中…" : "移動"}
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
  .choice {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    padding: var(--space-6) var(--space-10);
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
    font-size: var(--fs-13);
    cursor: pointer;
  }
  .choice.on {
    border-color: var(--accent);
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .choice.here {
    color: var(--muted);
    cursor: default;
  }
  .notes {
    margin: 0;
    padding-left: var(--space-20);
    color: var(--ink-2);
    font-size: var(--fs-13);
    line-height: 1.8;
  }
  .none {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-13);
  }
  .failure {
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--danger-weak);
    color: var(--danger-ink);
    line-height: 1.6;
  }
</style>
```

新しいファイル `web/app/workspace/WorkspaceDialogs.svelte`（4d の `CreateHost` と同じ形。開くたびに作り直す）:

```svelte
<script lang="ts">
  import { closeWorkspaceDialog, workspaceDialog } from "./dialogs-state.svelte";
  import PrefixChange from "./PrefixChange.svelte";
  import WorkspaceAdd from "./WorkspaceAdd.svelte";
  import WorkspaceDelete from "./WorkspaceDelete.svelte";
  import WorkspaceMove from "./WorkspaceMove.svelte";

  const d = $derived(workspaceDialog.current);
</script>

{#if d?.kind === "add"}
  {#key d}
    <WorkspaceAdd store={d.store} onclose={closeWorkspaceDialog} />
  {/key}
{:else if d?.kind === "delete"}
  {#key d}
    <WorkspaceDelete entry={d.entry} onclose={closeWorkspaceDialog} />
  {/key}
{:else if d?.kind === "prefix"}
  {#key d}
    <PrefixChange entry={d.entry} onclose={closeWorkspaceDialog} />
  {/key}
{:else if d?.kind === "move"}
  {#key d}
    <WorkspaceMove task={d.task} onclose={closeWorkspaceDialog} />
  {/key}
{/if}
```

- [ ] **Step 4: 入口を足す（タスク詳細・ツリーの行・App）**

`web/app/detail/TaskDetail.svelte`（4か所）。

```diff
@@
   import { errorMessage } from "../../core/messages";
   import { core } from "../context";
+  import Icon from "../icons/Icon.svelte";
   import { onLinkClick, router } from "../router";
   import { useSubscribe } from "../svelte-store";
@@
   import CliLine from "../ui/CliLine.svelte";
   import { copyText } from "../ui/clipboard";
+  import Menu from "../ui/Menu.svelte";
   import StoreBadge from "../ui/StoreBadge.svelte";
+  import { openWorkspaceMove } from "../workspace/dialogs-state.svelte";
+  import { MOVE_VALUE, moveWorkspaceItem } from "../workspace/move-menu";
   import BodyEditor from "./BodyEditor.svelte";
   import Checklist from "./Checklist.svelte";
@@
   const locked = $derived(s ? s.deleted_at != null || !!s.broken : false);
 
+  // The "…" menu of the header: the button it hangs from while it is open.
+  const spaces = useSubscribe(stores, () => stores.workspaces());
+  let more = $state<HTMLElement | null>(null);
+  const moreItems = $derived(s ? [moveWorkspaceItem(s, spaces.current)] : []);
+  function pickMore(value: string) {
+    const task = s;
+    more = null;
+    if (task && value === MOVE_VALUE) openWorkspaceMove(task);
+  }
+
   $effect(() => {
     const store = s?.store;
@@
         >
         {#if s.deleted_at == null && !s.broken}
+          <Button
+            size="sm"
+            aria-label="その他の操作"
+            aria-haspopup="menu"
+            aria-expanded={more !== null}
+            onclick={(e) => (more = e.currentTarget)}
+          >
+            <Icon name="more" size={14} />
+          </Button>
           <DeleteDialogs show={s} variant="delete" />
+          {#if more}
+            <Menu
+              anchor={more}
+              items={moreItems}
+              label="その他の操作"
+              onselect={pickMore}
+              onclose={() => (more = null)}
+            />
+          {/if}
         {/if}
       </div>
```

`web/app/tree/TreeRow.svelte`（props・分割代入・ボタン・スタイルの4か所）。

```diff
@@
     ontoggle: () => void;
     onopen: () => void;
+    onmore?: () => void; // the "…" button: the menu of the row
     dnd?: Attachment<HTMLElement>; // makes the row draggable and a drop target
     dragging?: boolean; // this row is being dragged
@@
     ontoggle,
     onopen,
+    onmore,
     dnd,
     dragging = false,
@@
       {/if}
     </span>
+    {#if onmore}
+      <button
+        type="button"
+        class="more"
+        tabindex="-1"
+        aria-label="メニュー: {task.title}"
+        aria-haspopup="menu"
+        onclick={(e) => {
+          e.stopPropagation();
+          onmore();
+        }}
+        ondblclick={(e) => e.stopPropagation()}
+        onkeydown={(e) => e.stopPropagation()}
+      >
+        <Icon name="more" size={14} />
+      </button>
+    {/if}
   </span>
 </div>
@@
     justify-content: center;
   }
+  .more {
+    flex: 0 0 auto;
+    display: inline-flex;
+    align-items: center;
+    justify-content: center;
+    width: 1.75rem;
+    height: 1.75rem;
+    border: 0;
+    border-radius: var(--radius-4);
+    background: transparent;
+    color: var(--muted);
+    opacity: 0;
+    cursor: pointer;
+  }
+  .row:hover .more,
+  .row.selected .more,
+  .more:focus-visible {
+    opacity: 1;
+  }
+  .more:hover {
+    background: var(--hover);
+  }
 </style>
```

`web/app/tree/TreeView.svelte`（5か所。`storesRev` は既にある）。

```diff
@@
   import Menu from "../ui/Menu.svelte";
   import StoreBadge from "../ui/StoreBadge.svelte";
+  import { openWorkspaceMove } from "../workspace/dialogs-state.svelte";
+  import { MOVE_VALUE, moveWorkspaceItem } from "../workspace/move-menu";
   import AddRow from "./AddRow.svelte";
   import { createBelow, moveTask, patchTask } from "./actions";
@@
 
   // --- keys ------------------------------------------------------------------------
-  let menu = $state<{ which: "status" | "kind"; ulid: string } | null>(null);
+  // "row" is the "…" button of a row; S and K open the other two.
+  let menu = $state<{ which: "status" | "kind" | "row"; ulid: string } | null>(null);
 
   function actionOf(k: KeyInput): KeyAction {
@@
   const menuItems = $derived.by(() => {
     if (!menu || !menuTask || !schema) return [];
+    if (menu.which === "row") {
+      storesRev.current;
+      return [moveWorkspaceItem(menuTask, c.stores.workspaces())];
+    }
     if (menu.which === "status") {
       return (schema.statuses ?? []).map((s) => ({ value: s.name, label: s.label || s.name }));
@@
     menu = null;
     if (!t || !which) return;
+    if (which === "row") {
+      if (value === MOVE_VALUE) openWorkspaceMove(t);
+      return;
+    }
     try {
       await patchTask(c, t, which === "status" ? { status: value } : { kind: value });
@@
             ontoggle={() => toggle(item.row.ulid, !item.row.expanded)}
             onopen={() => open(item.row.ulid)}
+            onmore={() => (menu = { which: "row", ulid: item.row.ulid })}
             dragging={dragging === item.row.ulid}
             dropMark={dropMarkOf(item.row.ulid)}
```

`web/app/App.svelte`: import に `import WorkspaceDialogs from "./workspace/WorkspaceDialogs.svelte";` を足し、テンプレートの `<CreateHost />` の次の行に `<WorkspaceDialogs />` を足す。

- [ ] **Step 5: 通ることを確かめる**

```bash
cd web && pnpm fix && pnpm check && pnpm test
```

期待: `biome` の指摘ゼロ、`svelte-check` の警告ゼロ、全テスト成功。

- [ ] **Step 6: コミット**

```bash
git add web/core web/app/workspace web/app/detail web/app/tree web/app/App.svelte
git commit -m "feat(web): ワークスペースの追加・接頭辞の変更・削除・移動のダイアログ"
```

---

### Task 2b: ワークスペースの設定タブ・設定のワークスペース一覧・E2E

**Files:**
- Create: `web/app/workspace/WorkspaceSettings.svelte`、`WorkspaceSettings.test.ts`
- Create: `web/app/settings/SettingsWorkspaces.svelte`、`SettingsWorkspaces.test.ts`、`project-counts.ts`、`project-counts.test.ts`
- Create: `web/e2e/workspace.spec.ts`

**Interfaces:**
- Consumes:
  - Task 2a の `openPrefixChange`・`openWorkspaceDelete`・`openWorkspaceAdd`、`nameProblem`・`removeBlocker`、`cliWorkspaceRename`・`cliWorkspaceRm`、`testkit.ts` の `entry`・`storeItem`・`written`
  - Task 1 の `WorkspaceEntry`、`router.href({ name: "workspace", store, ulid, tab: "settings" })`、`onLinkClick`。Task 1 の `WorkspaceHeader`（見出しと「設定」タブ）の下に `WorkspaceSettings` を置き、`SettingsLayout`（左に SettingsNav、右に本文）の中に `SettingsWorkspaces` を置く。`SettingsLayout` は本文の見出しを持たない（`SettingsWorkspaces` が「ワークスペース」の見出しと Store のバッジを描く）
  - この Task は App を触らない。`WorkspaceSettings` を `WorkspacePage.svelte` の設定のタブに、`SettingsWorkspaces` を `SettingsRoute.svelte` の本文につなぐのは Task 4 の Step 14（App のワークスペースごとの `{#key}` は Task 1 の `WorkspacePage` の呼び出しに既にある）。このため、この Task の `workspace.spec.ts` は Task 4 のあとで通る（Task 4 の Step 16 で全 spec を流す）
  - `TaskCache.watchList({ q, tree, store })` の `ulids()`・`loading()`・`dispose()`、`SchemaCache.get`・`load`、`TaskCache.task(ulid)`
  - `web/e2e/harness.ts` の `Hedgehug`（`hh`・`json`・`add`・`show`・`open`）、`web/e2e/screens.ts` の `row`・`toast`
- Produces:
  - `web/app/workspace/WorkspaceSettings.svelte`: props `{ entry: WorkspaceEntry }`
  - `web/app/settings/SettingsWorkspaces.svelte`: props `{ store: string }`
  - `web/app/settings/project-counts.ts`: `countProjects(tasks, schema): Map<string, number>`（ワークスペースの ULID ごとの、role が container の種類のタスクの数）

**この Task で決めたこと**
- WorkspaceSettings の名前は入力欄と「保存」（Enter でも保存）。保存した名前が変わる（CLI の改名、自分の保存）と欄を合わせ、件数だけが変わったときは打ちかけの文字を残す。理由の出し方は追加のダイアログと同じ（赤字と赤枠、保存を無効）。足元は `hh workspace rename <名前> <新しい名前> --store <Store>`（入力が正しくなければ `<新しい名前>`、コピーは無効）
- 接頭辞は今の値、「このワークスペースのタスクは REL-1, REL-2 … と振ります」、配下の件数（`tasks + deleted`）、「接頭辞を変更…」。キャンバスどおり「最後に変えたのは <日付>」を同じ行の「·」のあとに出す（`entry.prefixSet` を `core/format.ts` の `formatDate(iso, now)` で整える。細部 24 の前提「`prefix_set` が API にない」は誤りで、`WorkspaceView.prefix_set` は必須の項目。`WorkspaceEntry.prefixSet` は Task 1 の Step 4 が足す）。設定のワークスペース一覧にも、キャンバスどおり「接頭辞を最後に変えた日」の列を、タスクの列の次に足す（日付は Mono）
- 削除は、残り（`tasks + deleted > 0`）があれば「削除」を無効にして理由（`removeBlocker`）を出す（細部 25）
- SettingsWorkspaces のプロジェクト数は、その Store の全タスク（`watchList({ q: "", tree: false, store })`）を数える。タスクかスキーマがまだなら「…」。タスク数の列は `tasks + deleted`（キャンバスの「タスク（削除済みを含む）」）。「マウント中のほかの Store が使っている接頭辞」は `stores.workspaces()` の他の Store から作る
- E2E は harness の `demo`（接頭辞 `DM`）の Store に、必要なワークスペースを `hh workspace add` で足して使う。サイドバーの「…」とパレットから開く経路は Task 4・3 の E2E が見る

- [ ] **Step 1: 失敗するテストを書く**

新しいファイル `web/app/settings/project-counts.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { schemaView, taskView } from "../detail/testkit";
import { countProjects } from "./project-counts";

const ws = (ulid: string) => ({ ulid, name: ulid, id_prefix: ulid });

describe("countProjects", () => {
  it("counts the tasks whose kind is a container, per workspace", () => {
    const tasks = [
      taskView({ ulid: "A", kind: "project", workspace: ws("W1") }),
      taskView({ ulid: "B", kind: "epic", workspace: ws("W1") }),
      taskView({ ulid: "C", kind: "task", workspace: ws("W1") }),
      taskView({ ulid: "D", kind: "project", workspace: ws("W2") }),
      taskView({ ulid: "E", kind: "bug", workspace: ws("W2") }),
    ];
    const counts = countProjects(tasks, schemaView);
    expect(counts.get("W1")).toBe(2);
    expect(counts.get("W2")).toBe(1);
    expect(counts.get("W3")).toBeUndefined();
  });

  it("counts nothing without the schema", () => {
    expect(countProjects([taskView({ kind: "project" })], undefined).size).toBe(0);
  });
});
```

新しいファイル `web/app/settings/SettingsWorkspaces.test.ts`:

```ts
import { fireEvent, screen, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { makeCore, mount, taskView } from "../detail/testkit";
import { formatDate } from "../../core/format";
import { closeWorkspaceDialog, workspaceDialog } from "../workspace/dialogs-state.svelte";
import { entry } from "../workspace/testkit";
import SettingsWorkspaces from "./SettingsWorkspaces.svelte";

const ws = (ulid: string) => ({ ulid, name: ulid, id_prefix: ulid });
const spaces = [
  entry({ ulid: "W1", name: "release", prefix: "REL", tasks: 24, deleted: 1 }),
  entry({ ulid: "W2", name: "shared", prefix: "SH", tasks: 9, deleted: 0 }),
  entry({ store: "backend", ulid: "B1", name: "api", prefix: "API", tasks: 3, deleted: 0 }),
];
const tasks = [
  taskView({ ulid: "P1", kind: "project", workspace: ws("W1") }),
  taskView({ ulid: "P2", kind: "project", workspace: ws("W1") }),
  taskView({ ulid: "T1", kind: "task", workspace: ws("W1") }),
  taskView({ ulid: "T2", kind: "task", workspace: ws("W2") }),
];

function setup(opts: { loading?: boolean; noSchema?: boolean } = {}) {
  const k = makeCore({ workspaces: spaces, tasks });
  const watchList = vi.fn(() => ({
    ulids: () => tasks.map((t) => t.ulid),
    error: () => null,
    loading: () => opts.loading ?? false,
    dispose: vi.fn(),
  }));
  (k.cache as unknown as { watchList: typeof watchList }).watchList = watchList;
  if (opts.noSchema) k.schema.get.mockReturnValue(undefined as never);
  return { k, watchList, ...mount(SettingsWorkspaces, { store: "shared" }, k) };
}

describe("SettingsWorkspaces", () => {
  beforeEach(() => history.replaceState(null, "", "/settings/workspaces/shared"));
  afterEach(() => closeWorkspaceDialog());

  it("lists the workspaces of the store with their prefix, projects, tasks (deleted ones included) and the day the prefix was last changed", () => {
    setup();
    const rows = screen.getAllByRole("row");
    expect(rows).toHaveLength(3); // the header and the two workspaces of shared
    expect(
      within(rows[0] as HTMLElement)
        .getAllByRole("columnheader")
        .map((h) => h.textContent?.trim()),
    ).toEqual([
      "名前",
      "ID の接頭辞",
      "プロジェクト",
      "タスク（削除済みを含む）",
      "接頭辞を最後に変えた日",
      "操作",
    ]);
    const release = within(rows[1] as HTMLElement);
    expect(release.getByRole("rowheader")).toHaveTextContent("release");
    expect(release.getAllByRole("cell").map((c) => c.textContent?.trim())).toEqual([
      "REL",
      "2",
      "25",
      formatDate("2026-10-29T12:00:00Z", new Date()),
      "設定",
    ]);
    const shared = within(rows[2] as HTMLElement);
    expect(shared.getAllByRole("cell").map((c) => c.textContent?.trim())).toEqual([
      "SH",
      "0",
      "9",
      formatDate("2026-10-29T12:00:00Z", new Date()),
      "設定",
    ]);
    expect(screen.queryByText("api")).toBeNull(); // a workspace of another store
  });

  it("reads the tasks of this store only", () => {
    const { watchList } = setup();
    expect(watchList).toHaveBeenCalledWith({ q: "", tree: false, store: "shared" });
  });

  it("shows … for the project counts until the tasks and the schema are here", () => {
    setup({ loading: true });
    expect(screen.getAllByRole("cell", { name: "…" })).toHaveLength(2);
  });

  it("waits for the schema too", () => {
    setup({ noSchema: true });
    expect(screen.getAllByRole("cell", { name: "…" })).toHaveLength(2);
  });

  it("links each row to the settings tab of the workspace", async () => {
    setup();
    const link = screen.getByRole("link", { name: "release の設定" });
    expect(link).toHaveAttribute("href", "/workspaces/shared/W1/settings");
    await fireEvent.click(link);
    expect(location.pathname).toBe("/workspaces/shared/W1/settings");
  });

  it("opens the add dialog for this store", async () => {
    setup();
    await fireEvent.click(screen.getByRole("button", { name: "ワークスペースを追加" }));
    expect(workspaceDialog.current).toEqual({ kind: "add", store: "shared" });
  });

  it("shows the rule, the prefixes of the other stores, and the command", () => {
    setup();
    expect(
      screen.getByText(/接頭辞は英大文字で始まる 2〜10 文字の英大文字と数字です/),
    ).toBeInTheDocument();
    expect(screen.getByText(/マウント中のほかの Store が使っている接頭辞:/)).toHaveTextContent(
      "API（backend）",
    );
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh workspace ls --store shared",
    );
  });
});
```

新しいファイル `web/app/workspace/WorkspaceSettings.test.ts`:

```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { formatDate } from "../../core/format";
import { fail, makeCore, mount, ok } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import { closeWorkspaceDialog, workspaceDialog } from "./dialogs-state.svelte";
import { entry, written } from "./testkit";
import WorkspaceSettings from "./WorkspaceSettings.svelte";

const release = entry(); // 24 tasks and 1 deleted
const shared = entry({ ulid: "W2", name: "shared", prefix: "SH", tasks: 9, deleted: 0 });

function setup(target = release) {
  const k = makeCore({ workspaces: [target, shared] });
  k.api.PATCH.mockImplementation(() => ok(written({ name: "releases" }, { commit: "c1" })));
  return { k, ...mount(WorkspaceSettings, { entry: target }, k) };
}

const nameField = () => screen.getByRole("textbox", { name: "名前" });
const saveButton = () => screen.getByRole("button", { name: "保存" });

describe("WorkspaceSettings", () => {
  beforeEach(() => resetToasts());
  afterEach(() => closeWorkspaceDialog());

  it("shows the name as saved: nothing to save", () => {
    setup();
    expect(nameField()).toHaveValue("release");
    expect(saveButton()).toBeDisabled();
    expect(screen.getByText("タスクの ID は変わりません")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /コマンドをコピー/ })[0]).toHaveTextContent(
      "hh workspace rename release <新しい名前> --store shared",
    );
  });

  it("renames in one write", async () => {
    const { k } = setup();
    await fireEvent.input(nameField(), { target: { value: "releases" } });
    expect(
      screen.getByText("名前だけが変わります。接頭辞 REL とタスクの ID はそのままです"),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /コマンドをコピー/ })[0]).toHaveTextContent(
      "hh workspace rename release releases --store shared",
    );
    await fireEvent.click(saveButton());
    await waitFor(() => expect(k.api.PATCH).toHaveBeenCalled());
    expect(k.api.PATCH).toHaveBeenCalledWith("/api/workspaces/{id}", {
      params: { path: { id: "W1" }, query: { store: "shared" } },
      body: { name: "releases" },
    });
    expect(k.stores.reload).toHaveBeenCalled();
    await waitFor(() =>
      expect(toasts.list[0]?.message).toBe(
        "ワークスペースの名前を release から releases に変えました",
      ),
    );
  });

  it("saves with Enter, but not while an IME composes", async () => {
    const { k } = setup();
    await fireEvent.input(nameField(), { target: { value: "releases" } });
    await fireEvent.keyDown(nameField(), { key: "Enter", isComposing: true });
    expect(k.api.PATCH).not.toHaveBeenCalled();
    await fireEvent.keyDown(nameField(), { key: "Enter" });
    await waitFor(() => expect(k.api.PATCH).toHaveBeenCalledTimes(1));
  });

  it("refuses a name another workspace of the store has, in red, and cannot save", async () => {
    const { k } = setup();
    await fireEvent.input(nameField(), { target: { value: "shared" } });
    const msg = screen.getByText("shared は同じ Store のワークスペースが使っています");
    expect(msg).toHaveClass("bad");
    expect(nameField()).toHaveAttribute("aria-invalid", "true");
    expect(saveButton()).toBeDisabled();
    await fireEvent.keyDown(nameField(), { key: "Enter" });
    expect(k.api.PATCH).not.toHaveBeenCalled();
    expect(screen.getAllByRole("button", { name: /コマンドをコピー/ })[0]).toBeDisabled();
  });

  it("refuses a name that is not one", async () => {
    setup();
    await fireEvent.input(nameField(), { target: { value: "Release" } });
    expect(screen.getByText(/名前は小文字の英数字と/)).toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
  });

  it("shows the words of the server when it refuses", async () => {
    const { k } = setup();
    k.api.PATCH.mockImplementation(() =>
      fail(409, "workspace.taken", {}, "a workspace named x exists"),
    );
    await fireEvent.input(nameField(), { target: { value: "x" } });
    await fireEvent.click(saveButton());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "そのワークスペースの名前はすでに使われています",
    );
  });

  it("keeps what is typed when only the counts change, and follows a rename made elsewhere", async () => {
    const { rerender } = setup();
    await fireEvent.input(nameField(), { target: { value: "typing" } });
    await rerender({ entry: { ...release, tasks: 30 } });
    expect(nameField()).toHaveValue("typing");
    await rerender({ entry: { ...release, name: "renamed" } });
    await waitFor(() => expect(nameField()).toHaveValue("renamed"));
  });

  it("shows the prefix, when it was last changed, how many tasks a change renumbers, and opens the dialog", async () => {
    setup();
    expect(screen.getByText("REL", { selector: ".prefix" })).toBeInTheDocument();
    expect(screen.getByText(/REL-1, REL-2 … と振ります/)).toBeInTheDocument();
    expect(
      screen.getByText(`最後に変えたのは ${formatDate(release.prefixSet, new Date())}`, {
        exact: false,
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/配下の 25 件（削除済みを含む）の ID を新しい接頭辞で振り直します/),
    ).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "接頭辞を変更…" }));
    expect(workspaceDialog.current).toMatchObject({ kind: "prefix", entry: { ulid: "W1" } });
  });

  it("shows the ULID and copies it", async () => {
    setup(entry({ ulid: "01JC0A1B2C3D4E5F6G7H8J9K0M" }));
    expect(screen.getByText("01JC0A1B2C3D4E5F6G7H8J9K0M")).toBeInTheDocument();
    expect(screen.getByText(/CLI では名前・接頭辞・ULID のどれでも指せます/)).toBeInTheDocument();
    const writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    await fireEvent.click(screen.getByRole("button", { name: "ULID をコピー" }));
    await waitFor(() => expect(toasts.list[0]?.message).toBe("ULID をコピーしました"));
    expect(writeText).toHaveBeenCalledWith("01JC0A1B2C3D4E5F6G7H8J9K0M");
  });

  it("cannot delete while tasks remain, the deleted ones included, and says so", () => {
    setup();
    expect(
      screen.getByText(
        "タスク 25 件（削除済み 1 件を含む）が残っているため削除できません。別のワークスペースへ移すか完全に削除すると消せます",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "削除" })).toBeDisabled();
  });

  it("opens the delete dialog for an empty workspace", async () => {
    setup(entry({ tasks: 0, deleted: 0 }));
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(workspaceDialog.current).toMatchObject({ kind: "delete", entry: { ulid: "W1" } });
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

```bash
cd web && pnpm test app/settings app/workspace/WorkspaceSettings.test.ts
```

期待: `./project-counts`・`./SettingsWorkspaces.svelte`・`./WorkspaceSettings.svelte` が解決できず失敗する。

- [ ] **Step 3: 実装する**

新しいファイル `web/app/settings/project-counts.ts`:

```ts
import type { TaskView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";

/**
 * How many projects each workspace has, by ULID of the workspace: the tasks whose kind has the role
 * container (a project, an epic …). Without the schema nothing is known, so every count is 0.
 */
export function countProjects(
  tasks: Pick<TaskView, "kind" | "workspace">[],
  schema: SchemaView | undefined,
): Map<string, number> {
  const containers = new Set(
    (schema?.kinds ?? []).filter((k) => k.role === "container").map((k) => k.name),
  );
  const out = new Map<string, number>();
  for (const t of tasks) {
    if (containers.has(t.kind)) out.set(t.workspace.ulid, (out.get(t.workspace.ulid) ?? 0) + 1);
  }
  return out;
}
```

新しいファイル `web/app/settings/SettingsWorkspaces.svelte`:

```svelte
<script lang="ts">
  import type { TaskView } from "../../core/api/client";
  import { cliLine } from "../../core/cli";
  import { formatDate } from "../../core/format";
  import { core } from "../context";
  import { onLinkClick, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { openWorkspaceAdd } from "../workspace/dialogs-state.svelte";
  import { countProjects } from "./project-counts";

  let { store }: { store: string } = $props();

  const { cache, schema, stores } = core();
  const dir = useSubscribe(stores, () => ({
    workspaces: stores.workspaces(),
    tone: stores.tone(store),
  }));
  const rows = $derived(dir.current.workspaces.filter((w) => w.store === store));
  const now = new Date();
  // The prefixes the other stores use: a prefix cannot be shared with them.
  const elsewhere = $derived(dir.current.workspaces.filter((w) => w.store !== store));

  // The tasks of the store, to count the projects of each workspace.
  let list = $state<ReturnType<typeof cache.watchList> | null>(null);
  $effect(() => {
    const h = cache.watchList({ q: "", tree: false, store });
    list = h;
    return () => h.dispose();
  });
  $effect(() => {
    void schema.load(store).catch(() => undefined);
  });
  const rev = useSubscribe(cache, () => ({}));
  const sc = useSubscribe(schema, () => schema.get(store));

  // null while the tasks or the schema are not here yet
  const projects = $derived.by(() => {
    rev.current;
    const s = sc.current;
    if (!list || list.loading() || !s) return null;
    const tasks = list
      .ulids()
      .map((u) => cache.task(u))
      .filter((t): t is TaskView => t !== undefined);
    return countProjects(tasks, s);
  });
</script>

<div class="page">
  <header class="head">
    <h1>ワークスペース</h1>
    <StoreBadge {store} tone={dir.current.tone} />
    <span class="spacer"></span>
    <Button variant="primary" onclick={() => openWorkspaceAdd(store)}>ワークスペースを追加</Button>
  </header>
  <p class="lead">
    タスクを分ける単位で、それぞれが ID の接頭辞を持ちます。{store}
    を使う全員に共有されます（<span class="mono">workspaces/</span>
    への 1 コミット）。
  </p>

  <table>
    <thead>
      <tr>
        <th scope="col">名前</th>
        <th scope="col">ID の接頭辞</th>
        <th scope="col" class="num">プロジェクト</th>
        <th scope="col" class="num">タスク（削除済みを含む）</th>
        <th scope="col">接頭辞を最後に変えた日</th>
        <th scope="col"><span class="sr">操作</span></th>
      </tr>
    </thead>
    <tbody>
      {#each rows as w (w.ulid)}
        {@const href = router.href({ name: "workspace", store, ulid: w.ulid, tab: "settings" })}
        <tr>
          <th scope="row">{w.name}</th>
          <td class="mono">{w.prefix}</td>
          <td class="num">{projects ? (projects.get(w.ulid) ?? 0) : "…"}</td>
          <td class="num">{w.tasks + w.deleted}</td>
          <td class="mono">{formatDate(w.prefixSet, now)}</td>
          <td class="end">
            <a {href} aria-label="{w.name} の設定" onclick={(e) => onLinkClick(e, href)}>設定</a>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>

  <p class="rule">
    接頭辞は英大文字で始まる 2〜10 文字の英大文字と数字です。同じ Store とマウント中の Store
    のワークスペースとは重ねられません。
  </p>
  {#if elsewhere.length > 0}
    <p class="rule">
      マウント中のほかの Store が使っている接頭辞:
      {#each elsewhere as w, i (`${w.store}/${w.ulid}`)}
        {i > 0 ? " · " : ""}<span class="mono">{w.prefix}</span>（{w.store}）
      {/each}
    </p>
  {/if}
  <CliLine command={cliLine(["workspace", "ls", "--store", store])} />
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    max-width: 920px;
  }
  .head {
    display: flex;
    gap: var(--space-10);
    align-items: center;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
  }
  .spacer {
    flex: 1;
  }
  .lead,
  .rule {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.7;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-13);
  }
  th,
  td {
    padding: var(--space-10) var(--space-12);
    border-bottom: 1px solid var(--line);
    text-align: left;
  }
  thead th {
    color: var(--muted);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  tbody th {
    font-weight: 600;
  }
  .num {
    text-align: right;
  }
  .end {
    text-align: right;
  }
  .mono {
    font-family: var(--font-mono);
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  a {
    color: var(--accent);
  }
</style>
```

新しいファイル `web/app/workspace/WorkspaceSettings.svelte`:

```svelte
<script lang="ts">
  import { untrack } from "svelte";
  import { unwrap } from "../../core/api/client";
  import { cliWorkspaceRename, cliWorkspaceRm, Placeholder } from "../../core/cli";
  import { formatDate } from "../../core/format";
  import { errorMessage } from "../../core/messages";
  import type { WorkspaceEntry } from "../../core/stores";
  import { nameProblem, removeBlocker } from "../../core/workspace";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import { copyText } from "../ui/clipboard";
  import { openPrefixChange, openWorkspaceDelete } from "./dialogs-state.svelte";

  let { entry }: { entry: WorkspaceEntry } = $props();

  const { api, stores } = core();
  const uid = $props.id();
  const all = useSubscribe(stores, () => stores.workspaces());

  // The name in the field. It follows the saved name when that changes (a rename from the CLI, or the
  // save below), and is left alone while anything else about the workspace changes.
  const saved = $derived(entry.name);
  let draft = $state(untrack(() => entry.name));
  $effect(() => {
    draft = saved;
  });

  let busy = $state(false);
  let failure = $state<string | null>(null);

  const unchanged = $derived(draft === saved);
  const problem = $derived(
    unchanged ? null : nameProblem(draft, entry.store, all.current, entry.ulid),
  );
  const count = $derived(entry.tasks + entry.deleted);
  // The day the prefix was last set, as a date (this year without the year).
  const prefixDay = $derived(formatDate(entry.prefixSet, new Date()));
  const blocker = $derived(removeBlocker(entry));

  const renameCli = $derived(
    cliWorkspaceRename({
      workspace: saved,
      name: unchanged || problem ? new Placeholder("新しい名前") : draft,
      store: entry.store,
    }),
  );

  async function save() {
    if (busy || unchanged || problem !== null) return;
    busy = true;
    failure = null;
    const from = saved;
    try {
      const r = await unwrap(
        api.PATCH("/api/workspaces/{id}", {
          params: { path: { id: entry.ulid }, query: { store: entry.store } },
          body: { name: draft },
        }),
      );
      await stores.reload().catch(() => undefined);
      toast({ message: `ワークスペースの名前を ${from} から ${r.workspace.name} に変えました` });
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229) {
      e.preventDefault();
      void save();
    }
  }

  async function copyUlid() {
    toast({
      message: (await copyText(entry.ulid)) ? "ULID をコピーしました" : "コピーできませんでした",
    });
  }
</script>

<div class="settings">
  <section class="block" aria-labelledby="{uid}-name-h">
    <h2 id="{uid}-name-h">名前</h2>
    <div class="row">
      <input
        id="{uid}-name"
        type="text"
        autocomplete="off"
        aria-label="名前"
        value={draft}
        oninput={(e) => (draft = e.currentTarget.value)}
        {onkeydown}
        aria-invalid={problem ? "true" : undefined}
        aria-describedby="{uid}-name-msg"
      >
      <Button
        variant="primary"
        disabled={busy || unchanged || problem !== null}
        onclick={() => void save()}
      >
        保存
      </Button>
    </div>
    <p id="{uid}-name-msg" class="msg" class:bad={problem}>
      {#if problem}
        {problem}
      {:else if unchanged}
        タスクの ID は変わりません
      {:else}
        名前だけが変わります。接頭辞 {entry.prefix} とタスクの ID はそのままです
      {/if}
    </p>
    {#if failure}
      <p class="failure" role="alert">{failure}</p>
    {/if}
    <CliLine command={renameCli} disabled={unchanged || problem !== null} />
  </section>

  <section class="block" aria-labelledby="{uid}-prefix-h">
    <h2 id="{uid}-prefix-h">ID の接頭辞</h2>
    <div class="row">
      <span class="prefix mono">{entry.prefix}</span>
      <Button onclick={() => openPrefixChange(entry)}>接頭辞を変更…</Button>
    </div>
    <p class="msg">
      このワークスペースのタスクは {entry.prefix}-1, {entry.prefix}-2 … と振ります · 最後に変えたのは {prefixDay}
    </p>
    <p class="msg">
      変えると、配下の {count} 件（削除済みを含む）の ID を新しい接頭辞で振り直します（1
      コミット）。古い ID でも引き続き引けます。
    </p>
  </section>

  <section class="block" aria-labelledby="{uid}-ulid-h">
    <h2 id="{uid}-ulid-h">ULID</h2>
    <div class="row">
      <span class="mono">{entry.ulid}</span>
      <button type="button" class="copy" aria-label="ULID をコピー" onclick={copyUlid}>
        コピー
      </button>
    </div>
    <p class="msg">
      名前や接頭辞を変えても変わりません。CLI では名前・接頭辞・ULID のどれでも指せます
    </p>
  </section>

  <section class="block" aria-labelledby="{uid}-rm-h">
    <h2 id="{uid}-rm-h">ワークスペースを削除</h2>
    {#if blocker}
      <p class="msg" role="note">{blocker}</p>
    {:else}
      <p class="msg">タスクがないので削除できます。削除すると接頭辞 {entry.prefix} は空きます。</p>
    {/if}
    <div class="row">
      <Button
        variant="danger"
        disabled={blocker !== null}
        onclick={() => openWorkspaceDelete(entry)}
      >
        削除
      </Button>
    </div>
    <CliLine command={cliWorkspaceRm({ workspace: entry.name, store: entry.store })} />
  </section>
</div>

<style>
  .settings {
    display: flex;
    flex-direction: column;
    max-width: 760px;
  }
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-20) 0;
    border-bottom: 1px solid var(--line);
  }
  .block:last-child {
    border-bottom: 0;
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .row {
    display: flex;
    gap: var(--space-8);
    align-items: center;
  }
  input {
    flex: 1;
    max-width: 360px;
    min-height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  input:focus-visible {
    outline: 2px solid var(--accent);
  }
  input[aria-invalid="true"] {
    border: 1.5px solid var(--danger);
  }
  .prefix {
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .mono {
    font-family: var(--font-mono);
  }
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.7;
  }
  .bad {
    color: var(--danger);
  }
  .copy {
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    font-size: var(--fs-12);
    cursor: pointer;
  }
  .failure {
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--danger-weak);
    color: var(--danger-ink);
    font-size: var(--fs-13);
    line-height: 1.6;
  }
</style>
```

- [ ] **Step 4: E2E を書く**

新しいファイル `web/e2e/workspace.spec.ts`。`hh workspace ls --json`・`hh show <ULID> --json` の形と、`hh show DM-1`（旧 ID）が接頭辞の変更のあとも引けることは、手元の `hh` で確かめた。

```ts
// Screens this spec relies on (see screens.ts for the shared list):
//   - /settings/workspaces/<store> lists the workspaces of the Store in a table (one row each, with
//     the prefix, the projects, the tasks and the day the prefix was last changed) and has the button "ワークスペースを追加", which opens
//     the dialog named "<store> にワークスペースを追加" (the textboxes "名前" and "ID の接頭辞", the
//     buttons "追加" and "キャンセル")
//   - /workspaces/<store>/<ulid>/settings has the textbox "名前" with the button "保存", the prefix
//     with the button "接頭辞を変更…" (dialog "<name> の接頭辞を変更": the textbox "新しい接頭辞", the
//     button "<n> 件を振り直す"), and "削除" (dialog "ワークスペース <name> を削除", button "削除")
//   - the detail of a task has the button "その他の操作", whose menu has "別のワークスペースへ移動…"; a
//     row of the tree has the button "メニュー: <title>" with the same item. Both open the dialog
//     "<ID> <title> を移動": the radio group "移り先のワークスペース" (a radio is named
//     "<name>（<prefix>）", the current workspace adds " — 今のワークスペース" and is disabled) and the
//     button "移動"
//   - the toasts say: "ワークスペース <name>（<prefix>）を <store> に追加しました",
//     "ワークスペースの名前を <a> から <b> に変えました",
//     "<name> の接頭辞を <a> から <b> に変えました（<n> 件の ID を振り直しました）",
//     "ワークスペース <name> を削除しました", "<ID> を <name> に移しました（今の ID は <ID>）"
import { expect, type Hedgehug, test } from "./harness";
import { row, toast } from "./screens";

type Ws = { ulid: string; name: string; id_prefix: string; tasks: number };

function workspaceOf(hh: Hedgehug, name: string): Ws {
  const w = hh.json<Ws[]>("workspace", "ls").find((x) => x.name === name);
  if (!w) throw new Error(`no workspace ${name}`);
  return w;
}

const names = (hh: Hedgehug) => hh.json<Ws[]>("workspace", "ls").map((w) => w.name);

test("the settings list adds a workspace and refuses the names and prefixes that are taken", async ({
  page,
  hh,
}) => {
  await hh.open(page, "/settings/workspaces/demo");
  await expect(page.getByRole("row", { name: /demo/ })).toContainText("DM");

  await page.getByRole("button", { name: "ワークスペースを追加" }).click();
  const dialog = page.getByRole("dialog", { name: "demo にワークスペースを追加" });
  const name = dialog.getByRole("textbox", { name: "名前" });
  const prefix = dialog.getByRole("textbox", { name: "ID の接頭辞" });
  const add = dialog.getByRole("button", { name: "追加", exact: true });
  await expect(add).toBeDisabled();

  await name.fill("Ops");
  await expect(dialog).toContainText("名前は小文字の英数字と - _ で、英数字から始めてください");
  await name.fill("demo");
  await expect(dialog).toContainText("demo は同じ Store のワークスペースが使っています");
  await name.fill("ops");
  await prefix.fill("dm"); // typed in lower case: the field turns it into capitals
  await expect(prefix).toHaveValue("DM");
  await expect(dialog).toContainText("DM は同じ Store のワークスペース demo が使っています");
  await expect(add).toBeDisabled();

  await prefix.fill("OPS");
  await expect(dialog.getByRole("button", { name: /コマンドをコピー/ })).toContainText(
    "hh workspace add ops --prefix OPS --store demo",
  );
  await expect(add).toBeEnabled();
  await add.click();

  await expect(toast(page, "ワークスペース ops（OPS）を demo に追加しました")).toBeVisible();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("row", { name: /ops/ })).toContainText("OPS");
  expect(names(hh)).toEqual(["demo", "ops"]);
});

test("the settings tab renames the workspace, and refuses a name another workspace has", async ({
  page,
  hh,
}) => {
  hh.hh("workspace", "add", "other", "--prefix", "OT");
  const demo = workspaceOf(hh, "demo");
  await hh.open(page, `/workspaces/demo/${demo.ulid}/settings`);

  const name = page.getByRole("textbox", { name: "名前" });
  const save = page.getByRole("button", { name: "保存" });
  await expect(name).toHaveValue("demo");
  await expect(save).toBeDisabled();
  await expect(page.getByText("タスクの ID は変わりません")).toBeVisible();

  await name.fill("other");
  await expect(page.getByText("other は同じ Store のワークスペースが使っています")).toBeVisible();
  await expect(save).toBeDisabled();

  await name.fill("platform");
  await expect(
    page.getByText("名前だけが変わります。接頭辞 DM とタスクの ID はそのままです"),
  ).toBeVisible();
  await save.click();

  await expect(toast(page, "ワークスペースの名前を demo から platform に変えました")).toBeVisible();
  await expect(save).toBeDisabled();
  expect(names(hh)).toEqual(["other", "platform"]);
});

test("changing the prefix renumbers the tasks, and the old IDs still find them", async ({
  page,
  hh,
}) => {
  const first = hh.add("First");
  const second = hh.add("Second");
  const demo = workspaceOf(hh, "demo");
  await hh.open(page, `/workspaces/demo/${demo.ulid}/settings`);

  await page.getByRole("button", { name: "接頭辞を変更…" }).click();
  const dialog = page.getByRole("dialog", { name: "demo の接頭辞を変更" });
  await expect(dialog).toContainText(
    "配下の 2 件（削除済みを含む）の ID を新しい接頭辞で振り直します",
  );
  const field = dialog.getByRole("textbox", { name: "新しい接頭辞" });
  const go = dialog.getByRole("button", { name: "2 件を振り直す" });
  await expect(go).toBeDisabled();

  await field.fill("dm");
  await expect(dialog).toContainText("今と同じ接頭辞です");
  await expect(go).toBeDisabled();
  await field.fill("rls");
  await expect(dialog.getByRole("button", { name: /コマンドをコピー/ })).toContainText(
    "hh workspace set-prefix demo RLS --store demo",
  );
  await go.click();

  await expect(
    toast(page, "demo の接頭辞を DM から RLS に変えました（2 件の ID を振り直しました）"),
  ).toBeVisible();
  await expect(page.getByText("RLS-1, RLS-2 … と振ります")).toBeVisible();
  expect(hh.show(first.ulid).id).toBe("RLS-1");
  expect(hh.show(second.ulid).id).toBe("RLS-2");
  expect(hh.show("DM-1").ulid).toBe(first.ulid); // the old ID still finds the task
});

test("a workspace with tasks, deleted ones included, cannot be removed; an empty one can", async ({
  page,
  hh,
}) => {
  hh.add("Kept");
  const gone = hh.add("Removed");
  hh.hh("rm", gone.id);
  hh.hh("workspace", "add", "sandbox", "--prefix", "SBX");
  const demo = workspaceOf(hh, "demo");
  const sandbox = workspaceOf(hh, "sandbox");

  await hh.open(page, `/workspaces/demo/${demo.ulid}/settings`);
  await expect(
    page.getByText("タスク 2 件（削除済み 1 件を含む）が残っているため削除できません"),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "削除", exact: true })).toBeDisabled();

  await hh.open(page, `/workspaces/demo/${sandbox.ulid}/settings`);
  await page.getByRole("button", { name: "削除", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "ワークスペース sandbox を削除" });
  await expect(dialog).toContainText("にはタスクがありません");
  await dialog.getByRole("button", { name: "削除", exact: true }).click();

  await expect(toast(page, "ワークスペース sandbox を削除しました")).toBeVisible();
  await expect(dialog).toBeHidden();
  await expect(page).toHaveURL(`${hh.origin}/`); // the screen of the workspace that is gone is left
  expect(names(hh)).toEqual(["demo"]);
});

test("a task moves to another workspace from the … menu of its detail", async ({ page, hh }) => {
  const t = hh.add("Move me");
  hh.hh("workspace", "add", "ops", "--prefix", "OPS");
  await hh.open(page, `/tasks/${t.ulid}`);

  await page.getByRole("button", { name: "その他の操作" }).click();
  await page.getByRole("menuitem", { name: /別のワークスペースへ移動/ }).click();
  const dialog = page.getByRole("dialog", { name: /Move me を移動$/ });
  await expect(dialog.getByRole("radio", { name: /^demo/ })).toBeDisabled();
  await expect(dialog.getByRole("radio", { name: "ops（OPS）" })).toBeChecked();
  await expect(dialog).toContainText(
    "ID を OPS の番号に振り直します（DM-1 は former_ids に残ります）",
  );
  await expect(dialog.getByRole("button", { name: /コマンドをコピー/ })).toContainText(
    "hh mv DM-1 --workspace ops",
  );
  await dialog.getByRole("button", { name: "移動", exact: true }).click();

  await expect(toast(page, "DM-1 を ops に移しました（今の ID は OPS-1）")).toBeVisible();
  const moved = hh.json<{ id: string; former_ids: string[]; workspace: { name: string } }>(
    "show",
    t.ulid,
  );
  expect(moved.id).toBe("OPS-1");
  expect(moved.workspace.name).toBe("ops");
  expect(moved.former_ids).toContain("DM-1");
  await expect(page.getByRole("navigation", { name: "パンくず" })).toContainText("ops");
});

test("a task moves to another workspace from the … menu of a row of the tree", async ({
  page,
  hh,
}) => {
  const t = hh.add("Move from the tree");
  hh.hh("workspace", "add", "ops", "--prefix", "OPS");
  const demo = workspaceOf(hh, "demo");
  await hh.open(page, `/workspaces/demo/${demo.ulid}/tree`);

  await row(page, "Move from the tree").click();
  await page.getByRole("button", { name: "メニュー: Move from the tree" }).click();
  await page.getByRole("menuitem", { name: /別のワークスペースへ移動/ }).click();
  const dialog = page.getByRole("dialog", { name: /Move from the tree を移動$/ });
  await dialog.getByRole("button", { name: "移動", exact: true }).click();

  await expect(toast(page, "DM-1 を ops に移しました（今の ID は OPS-1）")).toBeVisible();
  await expect(row(page, "Move from the tree")).toBeHidden(); // it left this workspace
  expect(hh.json<{ workspace: { name: string } }>("show", t.ulid).workspace.name).toBe("ops");
});
```

- [ ] **Step 5: 通ることを確かめる**

```bash
cd web && pnpm fix && pnpm check && pnpm test
```

期待: `biome` の指摘ゼロ、`svelte-check` の警告ゼロ、全テスト成功。

`make e2e` はここでは流さない。設定のタブと設定の一覧の画面は、Task 4 の Step 14 がつなぐまで App に出ない。新しい6本の E2E は、Task 4 の Step 16 で全 spec と一緒に流す。そこで落ちるときは、spec 先頭のコメントの「画面の言葉」と、Task 1 の見出し・タブが同じ言葉かをまず確かめる。

- [ ] **Step 6: コミット**

```bash
git add web/app/workspace web/app/settings web/e2e/workspace.spec.ts
git commit -m "feat(web): ワークスペースの設定タブと設定のワークスペース一覧、E2E"
```

---

### Task 3: ワークスペースの概要、パレットのコマンド、undo、削除の取り消し

概要 plan の細部 20〜22（ワークスペースの概要）、34（パレットのコマンドのうち 4e-2 にある画面と操作）、35（undo）を作る。この Task は 4 つに分かれる。(a) 件数の数え方 `counts.ts` と概要の画面 `WorkspaceOverview.svelte`、(b) undo の `undo.ts`、(c) パレットのコマンド（ワークスペースの設定を開く・ワークスペースを追加・直前の操作を取り消す・選んだタスクの「別のワークスペースへ移動…」）、(d) Task 2 のワークスペースの削除のトーストに取り消しを付ける。最後に E2E を足す。

**前提**:
- Task 1（ルートとタブ、`WorkspaceHeader`、`WorkspaceEntry.tasks/deleted/prefixSet`）と Task 2（ワークスペースのダイアログ）が入っている。このコードの `router.href` は `tab` つきの Route を使い、`WorkspaceEntry` は `tasks`・`deleted`・`prefixSet` を持つ。Step 7 のパレットは Task 2 の `web/app/workspace/dialogs-state.svelte.ts`（`openWorkspaceAdd`）を、Step 8 は `openWorkspaceMove` を、Step 9 は Task 2 の `WorkspaceDelete.svelte` を使う
- 概要の画面は `WorkspacePage.svelte` の概要の分岐から `<WorkspaceOverview {entry} />` で呼ばれる。そのつなぎは Task 4 の Step 14 が行う。この Task は `App.svelte` も `WorkspacePage.svelte` も触らない。このため `overview.spec.ts` と、`undo.spec.ts` のワークスペースの削除の取り消しの spec は、Task 4 のあとで通る（Task 4 の Step 16 で全 spec を流す）

**実物を読んで確かめたこと**（コードはこれに合わせてある）:
- `TaskCache.watchList(q: ListQuery)` の `ListQuery` は `{ q: string; tree: boolean; store?: string }`（`tree` は必須）。返す `ulids()`・`loading()`・`error()`・`dispose()`。キャッシュは `tasks.changed` と `resync` のたびに、画面にある問い合わせを全部取り直す。だから概要が自分で SSE を聞く必要はなく、`watchList` で持てば件数と一覧が追従する
- `TaskView.progress`（`{done, total}`）は一覧の応答にサーバーが入れる。プロジェクトの進捗はそれを使う（フロントで子孫を数えない）。数えるのは work の役割で、中止を除く
- `workspace:` の項は名前・接頭辞・ULID のどれも解決する（`index.ResolveWorkspace`）。名前が別のワークスペースの接頭辞と重なると曖昧になるので、API には ULID（`workspace:<ULID>`）を送る。足元の `hh ls` は人が打つので 4d のツリーと同じ名前（`baseQuery` の `workspace:<名前>`）にする。細部 20 の「`workspace:<名前>`」からの違いはこの 1 点だけ
- 概要から開く作成が必ずこのワークスペースになるのは、`selection`（選択行）を書くのが `TreeView` だけで、`TreeView` は作られるときと壊れるときに `selection.set(null)` するため。概要へ来た時点で選択は空になる（`grep -rn "selection.set(" web/app` で確かめた）
- 問い合わせは削除済みを返さない（`is:deleted` を書かない限り）。概要の件数が `WorkspaceView.tasks` と同じになる
- `POST /api/undo` の応答は `UndoWrite`（`undone: LogView`、`commit: string | null`、`kept`）。取り消せるものがないときは 404 `undo.nothing`、取り消せない操作は 409 `undo.blocked`（`args.op`）。どちらも `errorMessage` に文言がある。`ops.undoSkip` が merge・repair・setup・undo・init を飛ばすので、undo を続けて押すと 1 つずつ前の操作に戻る
- 作成ダイアログの入れ先は `resolveCreateContext` が決める。`workspace` ルートで選択行がなければ、そのワークスペースのルート。概要は選択を持たないので、概要から開いた作成は自然にこのワークスペースに固定される（`openCreate` に入れ先を渡す口は足さない）
- ステータスのカテゴリは `todo`・`doing`・`done`・`canceled`。表示名は細部 36 と同じ「未着手」「進行中」「完了」「中止」

**Files:**
- Create: `web/app/workspace/counts.ts`、`web/app/workspace/counts.test.ts`
- Create: `web/app/workspace/WorkspaceOverview.svelte`、`web/app/workspace/WorkspaceOverview.test.ts`
- Create: `web/app/undo.ts`、`web/app/undo.test.ts`
- Modify: `web/app/icons/Icon.svelte`（アイコン `undo`）
- Modify: `web/app/palette/search.ts`（`CommandEnv`・`buildCommands`）、`web/app/palette/search.test.ts`
- Modify: `web/app/palette/CommandPalette.svelte`、`web/app/palette/CommandPalette.test.ts`
- Modify: `web/app/workspace/WorkspaceDelete.svelte`、`WorkspaceDelete.test.ts`（Task 2 のダイアログ。成功のトーストに取り消し）
- Create: `web/e2e/overview.spec.ts`、`web/e2e/undo.spec.ts`
- Modify: `web/e2e/screens.ts`（先頭のコメントに概要と undo の画面を足す）

**Interfaces:**
- Consumes:
  - Task 1: `WorkspaceHeader.svelte`（props `entry: WorkspaceEntry`、`tab: WorkspaceTab`、`actions?: Snippet`）。`Route` の `{ name: "workspace"; store; ulid; tab }`・`{ name: "project"; ulid; tab }`・`{ name: "settings"; page: "workspaces"; store }`。`WorkspaceEntry` の `tasks: number; deleted: number; prefixSet: string`
  - Task 2: `openWorkspaceAdd(store?: string): void`・`openWorkspaceMove(task: MoveTask): void`（`web/app/workspace/dialogs-state.svelte.ts`。`MoveTask` は `TaskView` の `ulid`・`id`・`title`・`store`・`workspace` の `Pick`）、`web/app/workspace/WorkspaceDelete.svelte`（props `{ entry, onclose }`。成功のトーストを出す）と `WorkspaceDelete.test.ts`、`web/app/workspace/testkit.ts` の `entry`
  - 4d の core: `TaskCache.watchList`・`task`・`noteOwnCommit`・`subscribe`、`SchemaCache.get`・`load`、`Api`・`unwrap`・`Schemas["UndoWrite"]`、`errorMessage`、`cliLs`、`formatDate`
  - 4d の app: `core()`（`AppCore`）、`useSubscribe`、`openCreate`、`router.href`・`onLinkClick`、`toast`、`opLabel`（`detail/history-labels`）、`baseQuery`（`tree/filter`）、`propText`（`tree/marks`）、`Button`・`CliLine`・`IdLabel`・`Avatar`、`Icon`・`KindIcon`・`StatusIcon`、`Empty`
- Produces:
  - `web/app/workspace/counts.ts`: `type Category = "todo" | "doing" | "done" | "canceled"`、`CATEGORIES`、`CATEGORY_LABEL`、`type Counts = { total: number; projects: number; byCategory: Record<Category, number> }`、`categoryOf(schema: SchemaView, status: string): Category`、`countByCategory(tasks: readonly TaskView[], schema: SchemaView): Counts`、`projectsOf(tasks): TaskView[]`、`rootsOf(tasks): TaskView[]`
  - `web/app/workspace/WorkspaceOverview.svelte`: props `{ entry: WorkspaceEntry }`。`WorkspaceHeader` を自分で描き（`tab="overview"`、`actions` に「タスクを作成」「プロジェクトを作成」のボタン）、その下に件数・プロジェクト・プロジェクトの外のタスク・足元の `hh ls` を出す
  - `web/app/undo.ts`: `runUndo(c: Pick<AppCore, "api" | "cache">, store?: string): Promise<void>`、`undoMessage(r: Schemas["UndoWrite"]): string`、`routeStore(route: Route, taskOf: (ulid: string) => { store: string } | undefined): string | undefined`。`store` は部 A の取り決めの `runUndo(core)` に足した省略できる引数
  - `web/app/palette/search.ts`: `CommandEnv` に `addWorkspace: () => void`・`undo: () => void`・`moveTask: (t: TaskView) => void`・`hrefs.workspaceSettings(store, ulid)`・`hrefs.settingsWorkspaces(store)`。`CommandItem.icon` に `"settings" | "undo"`。コマンドのキーは `cmd:workspace-add`・`cmd:undo`・`task-move:<ULID>`（選んだタスクの「別のワークスペースへ移動…」）・`open:ws-settings:<store>:<ulid>`・`open:settings-workspaces:<store>`
  - `Icon` の名前 `undo`
  - Task 2 の `WorkspaceDelete.svelte` の変更: 削除が済んだトーストに `undo: () => runUndo(c, gone.store)` を付ける（`WorkspaceDelete` の props は変わらない）

---

- [ ] **Step 1: 件数の数え方のテストを書く（失敗する）**

`web/app/workspace/counts.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { schema, task } from "../tree/fixtures";
import { categoryOf, countByCategory, projectsOf, rootsOf } from "./counts";

describe("countByCategory", () => {
  it("counts every task by the category of its status, projects included", () => {
    const tasks = [
      task("P1", "project", null, { status: "doing" }),
      task("T1", "task", "P1", { status: "doing" }),
      task("T2", "task", "P1", { status: "todo" }),
      task("T3", "task", null, { status: "backlog" }),
      task("T4", "task", null, { status: "done" }),
      task("T5", "task", null, { status: "canceled" }),
    ];
    expect(countByCategory(tasks, schema)).toEqual({
      total: 6,
      projects: 1,
      byCategory: { todo: 2, doing: 2, done: 1, canceled: 1 },
    });
  });

  it("counts a status the schema does not list as todo, so that the categories add up", () => {
    const c = countByCategory([task("T1", "task", null, { status: "gone" })], schema);
    expect(c.byCategory).toEqual({ todo: 1, doing: 0, done: 0, canceled: 0 });
    expect(categoryOf(schema, "gone")).toBe("todo");
  });

  it("counts nothing for no tasks", () => {
    expect(countByCategory([], schema)).toEqual({
      total: 0,
      projects: 0,
      byCategory: { todo: 0, doing: 0, done: 0, canceled: 0 },
    });
  });
});

describe("projectsOf and rootsOf", () => {
  const tasks = [
    task("T9", "task", null, { rank: "c" }),
    task("P2", "project", null, { rank: "b" }),
    task("T1", "task", "P2", { rank: "a" }),
    task("P1", "project", null, { rank: "a" }),
    task("E1", "epic", null, { rank: "a" }),
  ];

  it("lists the projects in rank order", () => {
    expect(projectsOf(tasks).map((t) => t.ulid)).toEqual(["P1", "P2"]);
  });

  it("lists the tasks at the root that are not projects, in rank order", () => {
    expect(rootsOf(tasks).map((t) => t.ulid)).toEqual(["E1", "T9"]);
  });

  it("does not change the list it is given", () => {
    const before = tasks.map((t) => t.ulid);
    projectsOf(tasks);
    rootsOf(tasks);
    expect(tasks.map((t) => t.ulid)).toEqual(before);
  });
});
```

Run: `cd web && pnpm exec vitest run app/workspace/counts.test.ts`
Expected: FAIL（`./counts` が見つからない）

- [ ] **Step 2: `counts.ts` を書く**

状態の名前がスキーマにないタスク（`problems` に `unknown_status` が出る）は「未着手」に数える。そうしないと帯の合計が件数と合わなくなる。

`web/app/workspace/counts.ts`:

```ts
import type { TaskView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";

export type Category = "todo" | "doing" | "done" | "canceled";

// The order the band and its legend show the categories in.
export const CATEGORIES: readonly Category[] = ["todo", "doing", "done", "canceled"];

export const CATEGORY_LABEL: Record<Category, string> = {
  todo: "未着手",
  doing: "進行中",
  done: "完了",
  canceled: "中止",
};

export type Counts = {
  total: number;
  projects: number;
  byCategory: Record<Category, number>;
};

// categoryOf is the category of a status. A status the schema does not list
// (the task has a problem that says so) counts as todo, so that the categories
// always add up to the total.
export function categoryOf(schema: SchemaView, status: string): Category {
  const c = schema.statuses?.find((s) => s.name === status)?.category;
  return c === "doing" || c === "done" || c === "canceled" ? c : "todo";
}

// countByCategory counts the tasks of a workspace as the band shows them:
// all of them (projects too), by the category of their status. projects is how many
// of them are of the project kind, which is part of total.
export function countByCategory(tasks: readonly TaskView[], schema: SchemaView): Counts {
  const byCategory: Record<Category, number> = { todo: 0, doing: 0, done: 0, canceled: 0 };
  let projects = 0;
  for (const t of tasks) {
    byCategory[categoryOf(schema, t.status)]++;
    if (t.kind === "project") projects++;
  }
  return { total: tasks.length, projects, byCategory };
}

const byRank = (a: TaskView, b: TaskView): number =>
  a.rank < b.rank ? -1 : a.rank > b.rank ? 1 : a.ulid < b.ulid ? -1 : a.ulid > b.ulid ? 1 : 0;

// projectsOf lists the projects, in rank order.
export function projectsOf(tasks: readonly TaskView[]): TaskView[] {
  return tasks.filter((t) => t.kind === "project").sort(byRank);
}

// rootsOf lists the tasks put straight at the root of the workspace, outside
// every project, in rank order.
export function rootsOf(tasks: readonly TaskView[]): TaskView[] {
  return tasks.filter((t) => !t.parent && t.kind !== "project").sort(byRank);
}
```

Run: `cd web && pnpm exec vitest run app/workspace/counts.test.ts`
Expected: PASS（6 テスト）

- [ ] **Step 3: 概要の画面のテストを書く（失敗する）**

実物の `TaskCache` と `testing.ts` の `FakeConn`・`fakeApi` を使う。`tasks.changed` を `conn.emit` で流して、件数と一覧が変わることを確かめる（概要が追従する根拠はキャッシュの取り直しなので、フェイクのキャッシュでは意味がない）。

`web/app/workspace/WorkspaceOverview.test.ts`:

```ts
import { cleanup, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import { TaskCache } from "../../core/cache";
import { formatDate } from "../../core/format";
import type { WorkspaceEntry } from "../../core/stores";
import { changed, FakeConn, fakeApi, problem, tv } from "../../core/testing";
import { closeCreate, createState } from "../create-state.svelte";
import { router } from "../router";
import { renderWithCore } from "../test-utils";
import { schema } from "../tree/fixtures";
import WorkspaceOverview from "./WorkspaceOverview.svelte";

afterEach(() => {
  cleanup();
  closeCreate();
});

// The entry the directory holds: its count is older than the list, to show that the page counts the list.
const entry: WorkspaceEntry = {
  store: "shared",
  tone: "purple",
  ulid: "W1",
  name: "release",
  prefix: "REL",
  projects: [],
  tasks: 3,
  deleted: 0,
  prefixSet: "2026-10-29T12:00:00Z",
};

const year = new Date().getFullYear();
const project = tv("P1", "リリース準備 v2.4", {
  id: "REL-15",
  kind: "project",
  status: "doing",
  rank: "a",
  assignees: ["sato"],
  props: { due: `${year}-11-12` },
  progress: { done: 3, total: 14 },
});
const child = tv("C1", "子のタスク", { id: "REL-16", parent: "P1", status: "doing" });
const rootLate = tv("R2", "手順書を見直す", { id: "REL-18", rank: "c", status: "todo" });
const rootEarly = tv("R1", "CI を整理", {
  id: "REL-22",
  rank: "b",
  status: "doing",
  assignees: ["tanaka"],
  props: { due: `${year}-11-08` },
});
const done = tv("D1", "終わったもの", { id: "REL-3", rank: "d", status: "done" });
const canceled = tv("X1", "やめたもの", { id: "REL-4", rank: "e", status: "canceled" });
const all = [project, child, rootLate, rootEarly, done, canceled];

function setup(initial: TaskView[], route?: (path: string) => unknown) {
  const rows = { current: initial };
  const { api, calls } = fakeApi((r) => {
    const own = route?.(r.path);
    if (own !== undefined) return own;
    return { tasks: rows.current };
  });
  const conn = new FakeConn();
  const cache = new TaskCache({ api, conn: conn.asConnection() });
  const none = () => () => {};
  const r = renderWithCore(
    WorkspaceOverview,
    { entry },
    { cache, schema: { get: () => schema, load: async () => schema, subscribe: none } },
  );
  return { ...r, rows, conn, cache, calls };
}

describe("WorkspaceOverview", () => {
  it("asks for the tasks of the workspace by its ULID, in its store", async () => {
    const { calls } = setup(all);
    await screen.findByText("6 件 · プロジェクト 1");
    const q = calls.find((c) => c.path === "/api/tasks");
    expect(q?.query.get("q")).toBe("workspace:W1");
    expect(q?.query.get("store")).toBe("shared");
  });

  it("counts every task and the projects, and the categories", async () => {
    setup(all);
    expect(await screen.findByText("6 件 · プロジェクト 1")).toBeInTheDocument();
    expect(screen.getByText("削除済みは含めません")).toBeInTheDocument();
    expect(screen.getByText("未着手 1")).toBeInTheDocument();
    expect(screen.getByText("進行中 3")).toBeInTheDocument();
    expect(screen.getByText("完了 1")).toBeInTheDocument();
    expect(screen.getByText("中止 1")).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "未着手 1 · 進行中 3 · 完了 1 · 中止 1" }),
    ).toBeInTheDocument();
  });

  it("lists the projects with progress, due date and assignee", async () => {
    setup(all);
    const section = await screen.findByRole("region", { name: "プロジェクト" });
    const row = within(section).getByRole("row", { name: /REL-15/ });
    expect(within(row).getByText("3/14")).toBeInTheDocument();
    expect(within(row).getByText(formatDate(`${year}-11-12`, new Date()))).toBeInTheDocument();
    expect(within(row).getByRole("img", { name: "sato" })).toBeInTheDocument();
    const href = router.href({ name: "project", ulid: "P1", tab: "tree" });
    expect(within(row).getByRole("link", { name: "リリース準備 v2.4" })).toHaveAttribute(
      "href",
      href,
    );
  });

  it("lists the tasks outside the projects in rank order, without the children of a project", async () => {
    setup(all);
    const section = await screen.findByRole("region", { name: "プロジェクトの外のタスク" });
    const ids = within(section)
      .getAllByRole("row")
      .slice(1)
      .map((r) => r.textContent ?? "");
    expect(ids).toHaveLength(4);
    expect(ids[0]).toContain("REL-22");
    expect(ids[1]).toContain("REL-18");
    expect(ids[2]).toContain("REL-3");
    expect(ids[3]).toContain("REL-4");
    expect(within(section).queryByText("子のタスク")).toBeNull();
    const tree = router.href({ name: "workspace", store: "shared", ulid: "W1", tab: "tree" });
    expect(within(section).getByRole("link", { name: "ツリーで開く" })).toHaveAttribute(
      "href",
      tree,
    );
  });

  it("follows tasks.changed: a task added from outside changes the counts and the list", async () => {
    const { rows, conn } = setup(all);
    await screen.findByText("6 件 · プロジェクト 1");

    rows.current = [
      ...all,
      tv("N1", "あとから足した", { id: "REL-30", rank: "f", status: "doing" }),
    ];
    conn.emit("tasks.changed", changed("shared", ["N1"], "c1"));

    expect(await screen.findByText("7 件 · プロジェクト 1")).toBeInTheDocument();
    expect(screen.getByText("進行中 4")).toBeInTheDocument();
    expect(screen.getByText("あとから足した")).toBeInTheDocument();
  });

  it("follows tasks.changed: a project that is gone leaves the list", async () => {
    const { rows, conn } = setup(all);
    await screen.findByRole("row", { name: /REL-15/ });

    rows.current = all.filter((t) => t.ulid !== "P1" && t.ulid !== "C1");
    conn.emit("tasks.changed", changed("shared", [], "c2", "rm", ["P1", "C1"]));

    expect(await screen.findByText("4 件 · プロジェクト 0")).toBeInTheDocument();
    expect(screen.getByText("プロジェクトはまだありません")).toBeInTheDocument();
  });

  it("keeps the query when only the entry changes (a rename, a new count)", async () => {
    const { cache, rerender, calls } = setup(all);
    const watch = vi.spyOn(cache, "watchList");
    await screen.findByText("6 件 · プロジェクト 1");
    const fetched = calls.filter((c) => c.path === "/api/tasks").length;

    await rerender({ entry: { ...entry, name: "release-2", tasks: 9 } });

    expect(screen.getByRole("heading", { name: /release-2/ })).toBeInTheDocument();
    expect(watch).not.toHaveBeenCalled();
    expect(calls.filter((c) => c.path === "/api/tasks")).toHaveLength(fetched);
  });

  it("opens the create dialogs from the buttons", async () => {
    setup(all);
    await screen.findByText("6 件 · プロジェクト 1");
    screen.getByRole("button", { name: "タスクを作成" }).click();
    expect(createState.request).toEqual({ kind: "task" });
    screen.getByRole("button", { name: "プロジェクトを作成" }).click();
    expect(createState.request).toEqual({ kind: "project" });
  });

  it("shows the hh ls line for the workspace", async () => {
    setup(all);
    await screen.findByText("6 件 · プロジェクト 1");
    const line = screen.getByRole("button", { name: /^コマンドをコピー: hh ls/ });
    expect(line.getAttribute("aria-label")).toContain("workspace:release");
    expect(line.getAttribute("aria-label")).toContain("--tree");
    expect(line.getAttribute("aria-label")).toContain("--store shared");
  });

  it("says so when the workspace has no task", async () => {
    setup([]);
    expect(await screen.findByText("release にはまだタスクがありません")).toBeInTheDocument();
  });

  it("shows why the tasks could not be read", async () => {
    setup([], () => problem(500, "internal"));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText("読み込み中…")).toBeNull());
  });
});
```

Run: `cd web && pnpm exec vitest run app/workspace/WorkspaceOverview.test.ts`
Expected: FAIL（`./WorkspaceOverview.svelte` が見つからない）

- [ ] **Step 4: `WorkspaceOverview.svelte` を書く**

読み方の要点:
- 問い合わせは `$effect` の中で `watchList` を取り、片付けで `dispose()`。依存は `store` と `ulid`（`$derived` の値）だけにする。`entry` そのものを読むと、改名や件数の変化で新しい `entry` が来るたびに問い合わせを捨てて取り直し、画面が「読み込み中」に戻ってしまう（テスト「keeps the query when only the entry changes」が見ている）
- 一覧が来るまでの件数は `entry.tasks`（ディレクトリが持つ値）。来たら一覧の数に切り替える。帯はスキーマと一覧がそろってから出す
- 読み込みの失敗は `role="alert"` で `errorMessage` を出す。キャッシュは次の `tasks.changed` か resync で取り直す
- 表の ID は 4d の `IdLabel`（ULID はカーソルを合わせたときだけ）。ID とタイトルは詳細（タスク）か `/projects/<ULID>`（プロジェクト）へのリンク。`⌘`/Ctrl つきのクリックは `onLinkClick` が新しいタブで開く
- 色は `tokens.css` の変数だけ。カテゴリの色は Foundations のステータスの色に合わせる（未着手 `--muted`、進行中 `--accent`、完了 `--success`、中止 `--muted-2`）。帯の幅は件数を `flex-grow` にして比で出す（数値であって色や寸法ではない）

`web/app/workspace/WorkspaceOverview.svelte`:

```svelte
<script lang="ts">
  import { cliLs } from "../../core/cli";
  import { formatDate } from "../../core/format";
  import { errorMessage } from "../../core/messages";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { onLinkClick, router } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import { baseQuery } from "../tree/filter";
  import { propText } from "../tree/marks";
  import Avatar from "../ui/Avatar.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import IdLabel from "../ui/IdLabel.svelte";
  import { CATEGORIES, CATEGORY_LABEL, countByCategory, projectsOf, rootsOf } from "./counts";
  import WorkspaceHeader from "./WorkspaceHeader.svelte";

  let { entry }: { entry: WorkspaceEntry } = $props();

  const c = core();
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));

  // The effects below depend on these two only: a rename or a new count makes a new entry, and
  // that must not drop the query and start it again (the screen would flash "loading").
  const store = $derived(entry.store);
  const ulid = $derived(entry.ulid);

  // The tasks of the workspace, deleted ones not included. The cache fetches the query again at
  // every tasks.changed and resync, so the counts and both lists follow the store. The ULID names
  // the workspace without the doubt a name that is the prefix of another has.
  let list = $state.raw<ReturnType<typeof c.cache.watchList> | null>(null);
  $effect(() => {
    const w = c.cache.watchList({ q: `workspace:${ulid}`, tree: false, store });
    list = w;
    return () => w.dispose();
  });

  // The categories need the schema of the store; the band waits for it.
  $effect(() => {
    void c.schema.load(store).catch(() => undefined);
  });
  const schema = $derived.by(() => {
    schemaRev.current;
    return c.schema.get(store);
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

  const projects = $derived(projectsOf(tasks));
  const roots = $derived(rootsOf(tasks));
  const counts = $derived(schema && !loading && !error ? countByCategory(tasks, schema) : null);
  // Until the list has come, the count of the workspace is the one the store directory has.
  const total = $derived(loading || error ? entry.tasks : tasks.length);
  const statusCategory = (status: string) =>
    schema?.statuses?.find((s) => s.name === status)?.category;

  // Not written as scope="..." in the markup: Biome reads that name as the HTML attribute.
  const emptyProps = $derived({ scope: "workspace" as const, name: entry.name });
  const treeHref = $derived(router.href({ name: "workspace", store, ulid, tab: "tree" }));
  const projectHref = (u: string) => router.href({ name: "project", ulid: u, tab: "tree" });
  const taskHref = (u: string) => router.href({ name: "task", ulid: u });
  const today = new Date();
  const dueOf = (t: Parameters<typeof propText>[0]) => {
    const d = propText(t, "due");
    return d ? formatDate(d, today) : "—";
  };
  const percent = (p: { done: number; total: number }) =>
    p.total > 0 ? Math.round((p.done / p.total) * 100) : 0;
</script>

<WorkspaceHeader {entry} tab="overview">
  {#snippet actions()}
    <Button size="sm" aria-label="タスクを作成" onclick={() => openCreate({ kind: "task" })}>
      <Icon name="plus" size={14} />タスク
    </Button>
    <Button
      size="sm"
      aria-label="プロジェクトを作成"
      onclick={() => openCreate({ kind: "project" })}
    >
      <Icon name="folder-plus" size={14} />プロジェクト
    </Button>
  {/snippet}
</WorkspaceHeader>

<div class="overview">
  {#if error}
    <p class="msg error" role="alert">{errorMessage(error)}</p>
  {:else if !loading && tasks.length === 0}
    <Empty variant="empty" {...emptyProps} onCreate={() => openCreate({ kind: "task" })} />
  {:else}
    <section class="counts" aria-label="件数">
      <p class="total">{total} 件{loading ? "" : ` · プロジェクト ${projects.length}`}</p>
      <p class="note">削除済みは含めません</p>
      {#if counts}
        <div
          class="band"
          role="img"
          aria-label={CATEGORIES.map((k) => `${CATEGORY_LABEL[k]} ${counts.byCategory[k]}`).join(
            " · ",
          )}
        >
          {#each CATEGORIES as k (k)}
            {#if counts.byCategory[k] > 0}
              <span class="seg" data-category={k} style:flex-grow={counts.byCategory[k]}></span>
            {/if}
          {/each}
        </div>
        <ul class="legend">
          {#each CATEGORIES as k (k)}
            <li data-category={k}>
              <span class="dot"></span>{CATEGORY_LABEL[k]} {counts.byCategory[k]}
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    {#if loading}
      <p class="msg">読み込み中…</p>
    {:else}
      <section aria-labelledby="ov-projects">
        <div class="sec-head">
          <h2 id="ov-projects">プロジェクト</h2>
          <span class="sub">{projects.length} 件</span>
        </div>
        {#if projects.length === 0}
          <p class="none">プロジェクトはまだありません</p>
        {:else}
          <table>
            <thead>
              <tr>
                <th class="w-id">ID</th>
                <th>タイトル</th>
                <th class="w-progress">進捗</th>
                <th class="w-due">期限</th>
                <th class="w-who">担当</th>
              </tr>
            </thead>
            <tbody>
              {#each projects as p (p.ulid)}
                {@const href = projectHref(p.ulid)}
                <tr>
                  <td>
                    <IdLabel id={p.id} ulid={p.ulid} {href} onopen={(e) => onLinkClick(e, href)} />
                  </td>
                  <td>
                    <div class="title">
                      <KindIcon kind={p.kind} />
                      <a {href} onclick={(e) => onLinkClick(e, href)}>{p.title}</a>
                    </div>
                  </td>
                  <td>
                    {#if p.progress}
                      <span class="bar" aria-hidden="true"
                        ><span class="fill" style:width="{percent(p.progress)}%"></span></span
                      >
                      <span class="frac">{p.progress.done}/{p.progress.total}</span>
                    {:else}
                      <span class="frac">—</span>
                    {/if}
                  </td>
                  <td class="due">{dueOf(p)}</td>
                  <td class="who">
                    {#if p.assignees?.[0]}
                      <Avatar name={p.assignees[0]} />
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </section>

      <section aria-labelledby="ov-roots">
        <div class="sec-head">
          <h2 id="ov-roots">プロジェクトの外のタスク</h2>
          <span class="sub">ルートに直接置いたタスク · rank 順 · {roots.length} 件</span>
          <a class="open-tree" href={treeHref} onclick={(e) => onLinkClick(e, treeHref)}
            >ツリーで開く</a
          >
        </div>
        {#if roots.length === 0}
          <p class="none">プロジェクトの外のタスクはありません</p>
        {:else}
          <table>
            <thead>
              <tr>
                <th class="w-status">ステータス</th>
                <th class="w-kind">種類</th>
                <th class="w-id">ID</th>
                <th>タイトル</th>
                <th class="w-due">期限</th>
                <th class="w-who">担当</th>
              </tr>
            </thead>
            <tbody>
              {#each roots as t (t.ulid)}
                {@const href = taskHref(t.ulid)}
                <tr>
                  <td><StatusIcon status={t.status} category={statusCategory(t.status)} /></td>
                  <td><KindIcon kind={t.kind} /></td>
                  <td>
                    <IdLabel id={t.id} ulid={t.ulid} {href} onopen={(e) => onLinkClick(e, href)} />
                  </td>
                  <td>
                    <div class="title">
                      <a {href} onclick={(e) => onLinkClick(e, href)}>{t.title}</a>
                    </div>
                  </td>
                  <td class="due">{dueOf(t)}</td>
                  <td class="who">
                    {#if t.assignees?.[0]}
                      <Avatar name={t.assignees[0]} />
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </section>
    {/if}
  {/if}

  <footer class="foot">
    <CliLine
      command={cliLs(baseQuery({ kind: "workspace", name: entry.name }, "cli"), true, store)}
    />
  </footer>
</div>

<style>
  .overview {
    display: flex;
    flex-direction: column;
    gap: var(--space-28);
    padding-top: var(--space-20);
  }
  .counts {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .total {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .band {
    display: flex;
    height: 8px;
    overflow: hidden;
    border-radius: var(--radius-pill);
    background: var(--sunken);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: var(--radius-pill);
  }
  .seg {
    flex-basis: 0;
    min-width: 4px;
  }
  .seg[data-category="todo"],
  li[data-category="todo"] .dot {
    background: var(--muted);
  }
  .seg[data-category="doing"],
  li[data-category="doing"] .dot {
    background: var(--accent);
  }
  .seg[data-category="done"],
  li[data-category="done"] .dot {
    background: var(--success);
  }
  .seg[data-category="canceled"],
  li[data-category="canceled"] .dot {
    background: var(--muted-2);
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-16);
    margin: 0;
    padding: 0;
    list-style: none;
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .legend li {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
  }
  .sec-head {
    display: flex;
    align-items: baseline;
    gap: var(--space-12);
    margin-bottom: var(--space-8);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .sub {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .open-tree {
    margin-left: auto;
    color: var(--accent);
    font-size: var(--fs-12);
    text-decoration: none;
  }
  .open-tree:hover {
    text-decoration: underline;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-13);
  }
  th {
    padding: var(--space-6) var(--space-8);
    border-bottom: var(--hairline) solid var(--line);
    color: var(--muted);
    font-size: var(--fs-11);
    font-weight: 500;
    text-align: left;
  }
  td {
    height: var(--row-h);
    padding: 0 var(--space-8);
    border-bottom: var(--hairline) solid var(--line-soft);
  }
  .title {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    min-width: 0;
  }
  .title a {
    overflow: hidden;
    color: var(--ink);
    text-decoration: none;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title a:hover {
    text-decoration: underline;
  }
  .w-id {
    width: 6rem;
  }
  .w-status,
  .w-kind {
    width: 4.5rem;
  }
  .w-progress {
    width: 11rem;
  }
  .w-due {
    width: 4.5rem;
  }
  .w-who {
    width: 3.5rem;
  }
  .bar {
    display: inline-block;
    width: 5rem;
    height: 6px;
    margin-right: var(--space-8);
    overflow: hidden;
    border-radius: var(--radius-pill);
    background: var(--sunken);
    vertical-align: middle;
  }
  .fill {
    display: block;
    height: 100%;
    background: var(--accent);
  }
  .frac,
  .due {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    color: var(--ink-2);
  }
  .none,
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
  .msg.error {
    color: var(--danger);
  }
  .foot {
    margin-top: auto;
  }
</style>
```

Run: `cd web && pnpm exec vitest run app/workspace`
Expected: PASS（`counts.test.ts` の 6 と `WorkspaceOverview.test.ts` の 11）

- [ ] **Step 5: undo のテストを書く（失敗する）**

`web/app/undo.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Schemas } from "../core/api/client";
import { fakeApi, problem } from "../core/testing";
import type { AppCore } from "./context";
import { resetToasts, toasts } from "./toast.svelte";
import { routeStore, runUndo, undoMessage } from "./undo";

type UndoWrite = Schemas["UndoWrite"];

function result(over: Partial<UndoWrite> = {}): UndoWrite {
  return {
    commit: "c2",
    kept: [],
    undone: {
      hash: "c1",
      date: "2026-11-03T10:00:00Z",
      author: { name: "e2e", email: "e2e@example.com" },
      store: "shared",
      op: "set",
      task: "T1",
      id: "REL-6",
      subject: "set REL-6",
      trailers: [],
    },
    ...over,
  };
}

beforeEach(() => resetToasts());

describe("undoMessage", () => {
  it("names the task and the operation", () => {
    expect(undoMessage(result())).toBe("REL-6 の変更を取り消しました");
  });

  it("leaves out the task when the operation is not about one", () => {
    const r = result();
    expect(undoMessage({ ...r, undone: { ...r.undone, id: null, op: "workspace" } })).toBe(
      "ワークスペースの変更を取り消しました",
    );
  });

  it("says how many values it kept because others changed them later", () => {
    const kept = [
      { path: "t", field: "status", author: "tanaka" },
      { path: "t", field: "title", author: "sato" },
    ];
    expect(undoMessage(result({ kept }))).toBe(
      "REL-6 の変更を取り消しました · あとで別の人が変えた 2 件の値は残しました",
    );
  });

  it("says when nothing could be taken back", () => {
    const kept = [{ path: "t", field: "status", author: "tanaka" }];
    expect(undoMessage(result({ commit: null, kept }))).toBe(
      "REL-6 の変更は、変えた値がすべてあとで別の人に変えられていたため、何も戻しませんでした",
    );
  });
});

describe("routeStore", () => {
  const taskOf = (u: string) => (u === "T1" ? { store: "backend" } : undefined);

  it("takes the store of a workspace, and of the project or task shown", () => {
    expect(
      routeStore({ name: "workspace", store: "shared", ulid: "W1", tab: "overview" }, taskOf),
    ).toBe("shared");
    expect(routeStore({ name: "project", ulid: "T1", tab: "tree" }, taskOf)).toBe("backend");
    expect(routeStore({ name: "task", ulid: "T1" }, taskOf)).toBe("backend");
  });

  it("has none elsewhere, or for a task the cache does not hold", () => {
    expect(routeStore({ name: "home" }, taskOf)).toBeUndefined();
    expect(routeStore({ name: "task", ulid: "NOPE" }, taskOf)).toBeUndefined();
  });
});

describe("runUndo", () => {
  function setup(route: Parameters<typeof fakeApi>[0]) {
    const { api, calls } = fakeApi(route);
    const noteOwnCommit = vi.fn();
    const c = { api, cache: { noteOwnCommit } } as unknown as Pick<AppCore, "api" | "cache">;
    return { c, calls, noteOwnCommit };
  }

  it("posts to /api/undo with the store, remembers the commit and says what it did", async () => {
    const { c, calls, noteOwnCommit } = setup(() => result());
    await runUndo(c, "shared");
    expect(calls).toHaveLength(1);
    expect(calls[0]).toMatchObject({ method: "POST", path: "/api/undo" });
    expect(calls[0]?.query.get("store")).toBe("shared");
    expect(noteOwnCommit).toHaveBeenCalledWith("c2");
    expect(toasts.list.map((t) => t.message)).toEqual(["REL-6 の変更を取り消しました"]);
  });

  it("sends no store when there is none", async () => {
    const { c, calls } = setup(() => result());
    await runUndo(c);
    expect(calls[0]?.query.has("store")).toBe(false);
  });

  it("shows nothing to undo as the message of the server", async () => {
    const { c, noteOwnCommit } = setup(() => problem(404, "undo.nothing"));
    await runUndo(c);
    expect(toasts.list.map((t) => t.message)).toEqual(["取り消せる操作がありません"]);
    expect(noteOwnCommit).not.toHaveBeenCalled();
  });

  it("shows why it is refused", async () => {
    const { c } = setup(() => problem(409, "undo.blocked", { op: "purge" }));
    await runUndo(c);
    expect(toasts.list.map((t) => t.message)).toEqual(["完全に消したものは取り消せません"]);
  });
});
```

Run: `cd web && pnpm exec vitest run app/undo.test.ts`
Expected: FAIL（`./undo` が見つからない）

- [ ] **Step 6: `undo.ts` を書く**

確認のダイアログは出さない（spec の決定）。結果はトーストだけで、取り消しの印は付けない（undo の undo はない。続けて押すと 1 つ前の操作に戻る）。`commit` は自分のコミットとして `noteOwnCommit` に渡し、そのイベントを「CLI で更新 · 今」の印にしない。

`web/app/undo.ts`:

```ts
import { type Schemas, unwrap } from "../core/api/client";
import { errorMessage } from "../core/messages";
import type { AppCore } from "./context";
import { opLabel } from "./detail/history-labels";
import type { Route } from "./router";
import { toast } from "./toast.svelte";

type UndoWrite = Schemas["UndoWrite"];

// undoMessage says what POST /api/undo did: the operation taken back, and what it kept because
// others changed it after. commit is null when every field it changed had been changed since.
export function undoMessage(r: UndoWrite): string {
  const what = `${r.undone.id ? `${r.undone.id} の` : ""}${opLabel(r.undone.op)}`;
  const kept = r.kept?.length ?? 0;
  if (r.commit === null) {
    return `${what}は、変えた値がすべてあとで別の人に変えられていたため、何も戻しませんでした`;
  }
  return kept > 0
    ? `${what}を取り消しました · あとで別の人が変えた ${kept} 件の値は残しました`
    : `${what}を取り消しました`;
}

// routeStore is the store the screen is about: the store of a workspace, or of the project or
// task it shows (the cache may not have it yet). Elsewhere there is none.
export function routeStore(
  route: Route,
  taskOf: (ulid: string) => { store: string } | undefined,
): string | undefined {
  if (route.name === "workspace") return route.store;
  if (route.name === "project" || route.name === "task") return taskOf(route.ulid)?.store;
  return undefined;
}

// runUndo takes back the last operation of the user (POST /api/undo, as hh undo) and says how it
// went. It does not ask first: the spec holds that an undo is not hard to take back. The store is
// the one the screen is about; without it the server picks one as the CLI does, which asks the
// user to choose when there are several.
export async function runUndo(c: Pick<AppCore, "api" | "cache">, store?: string): Promise<void> {
  try {
    const r = await unwrap(c.api.POST("/api/undo", { params: { query: { store } } }));
    // The commit is this tab's own: its tasks.changed is not shown as a change from outside.
    c.cache.noteOwnCommit(r.commit);
    toast({ message: undoMessage(r) });
  } catch (e) {
    toast({ message: errorMessage(e) });
  }
}
```

Run: `cd web && pnpm exec vitest run app/undo.test.ts`
Expected: PASS（10 テスト）

- [ ] **Step 7: パレットのコマンドを足す**

4e-2 にある画面と操作のコマンドだけを足す。

| コマンド | 置き場 | 動き |
|---|---|---|
| 「ワークスペースを追加」 | 「コマンド」 | `openWorkspaceAdd()`（Task 2） |
| 「直前の操作を取り消す」 | 「コマンド」 | `runUndo(c, routeStore(...))`。ワークスペースの画面ならその Store、プロジェクトとタスクの画面ならそのタスクの Store、ほかは Store を渡さない |
| 「ワークスペースの設定を開く」 | 「開く」（ワークスペースごと。注にワークスペースの名前と Store のバッジ） | `/workspaces/<store>/<ulid>/settings` |
| 「設定のワークスペース一覧を開く」 | 「開く」（Store ごと） | `/settings/workspaces/<store>` |

「設定を開く」で探す利用者がワークスペースのタブと設定の一覧の両方に行けるよう、設定のコマンドは 2 つとも足す。選んだタスクの「別のワークスペースへ移動…」は Step 8 で足す。undo は「今の画面の Store」（`routeStore`）を渡す。

同期と保守・スキーマ・doctor・マイタスク・通知・アクティビティ・保存済みビューのコマンドは、その画面を作る回（4e-3・4e-4）で足す。

先にテストを直す。`search.test.ts` と `CommandPalette.test.ts` の差分（コマンドの一覧が変わるので、既存の期待も直す）:

```diff
--- a/web/app/palette/search.test.ts
+++ b/web/app/palette/search.test.ts
@@ -186,10 +186,14 @@
     syncNow: vi.fn(),
     createTask: vi.fn(),
     createProject: vi.fn(),
+    addWorkspace: vi.fn(),
+    undo: vi.fn(),
     workspaces: [ws],
     go: vi.fn(),
     hrefs: {
       workspace: (s, u) => `/workspaces/${s}/${u}`,
+      workspaceSettings: (s, u) => `/workspaces/${s}/${u}/settings`,
+      settingsWorkspaces: (s) => `/settings/workspaces/${s}`,
       project: (u) => `/projects/${u}`,
       task: (u) => `/tasks/${u}`,
     },
@@ -218,8 +222,12 @@
       "今すぐ同期する",
       "タスクを作成",
       "プロジェクトを作成",
+      "ワークスペースを追加",
+      "直前の操作を取り消す",
       "release を開く",
+      "ワークスペースの設定を開く",
       "リリース準備 v2.4 を開く",
+      "設定のワークスペース一覧を開く",
     ]);
   });
 
@@ -255,10 +263,59 @@
     const sync = gs[0]?.items[0];
     if (sync?.type === "command") sync.run();
     expect(e.syncNow).toHaveBeenCalled();
-    const proj = gs[1]?.items[1];
+    const proj = gs[1]?.items[2];
     expect(proj).toMatchObject({ href: "/projects/P1", store: "shared" });
     if (proj?.type === "command") proj.run();
     expect(e.go).toHaveBeenCalledWith("/projects/P1");
+  });
+
+  it("adds a workspace and takes back the last operation", () => {
+    const e = env();
+    const items = buildCommands(e, ">")[0]?.items ?? [];
+    const add = items.find((i) => i.key === "cmd:workspace-add");
+    const undo = items.find((i) => i.key === "cmd:undo");
+    expect(add).toMatchObject({ label: "ワークスペースを追加", icon: "plus" });
+    expect(undo).toMatchObject({ label: "直前の操作を取り消す", icon: "undo" });
+    if (add?.type === "command") add.run();
+    expect(e.addWorkspace).toHaveBeenCalledTimes(1);
+    if (undo?.type === "command") undo.run();
+    expect(e.undo).toHaveBeenCalledTimes(1);
+    expect(labels(buildCommands(e, ">取り消"))).toEqual(["直前の操作を取り消す"]);
+    expect(labels(buildCommands(e, ">追加"))).toEqual(["ワークスペースを追加"]);
+  });
+
+  it("opens the settings of a workspace, named by the workspace, and the list of a store", () => {
+    const e = env();
+    const gs = buildCommands(e, ">設定");
+    expect(gs.map((g) => g.name)).toEqual(["開く"]);
+    const [settings, list] = gs[0]?.items ?? [];
+    expect(settings).toMatchObject({
+      label: "ワークスペースの設定を開く",
+      note: "release",
+      store: "shared",
+      href: "/workspaces/shared/W1/settings",
+    });
+    expect(list).toMatchObject({
+      label: "設定のワークスペース一覧を開く",
+      store: "shared",
+      href: "/settings/workspaces/shared",
+    });
+    if (settings?.type === "command") settings.run();
+    expect(e.go).toHaveBeenCalledWith("/workspaces/shared/W1/settings");
+    if (list?.type === "command") list.run();
+    expect(e.go).toHaveBeenCalledWith("/settings/workspaces/shared");
+  });
+
+  it("lists the settings of a workspace for each workspace, the list once for a store", () => {
+    const second = { ...ws, ulid: "W2", name: "ops", prefix: "OPS", projects: [] };
+    const gs = buildCommands(env({ workspaces: [ws, second] }), ">設定");
+    expect(
+      gs[0]?.items.map((i) => (i.type === "command" ? `${i.label}|${i.note ?? ""}` : "")),
+    ).toEqual([
+      "ワークスペースの設定を開く|release",
+      "ワークスペースの設定を開く|ops",
+      "設定のワークスペース一覧を開く|",
+    ]);
   });
 
   it("adds the commands for the selected task", () => {
```

```diff
--- a/web/app/palette/CommandPalette.test.ts
+++ b/web/app/palette/CommandPalette.test.ts
@@ -7,9 +7,11 @@
 
 const h = vi.hoisted(() => ({
   core: undefined as unknown,
-  route: { name: "home" } as { name: string; ulid?: string },
+  route: { name: "home" } as { name: string; ulid?: string; store?: string },
   go: vi.fn(),
   toast: vi.fn(),
+  runUndo: vi.fn(async () => {}),
+  addWorkspace: vi.fn(),
 }));
 
 vi.mock("../context", () => ({ core: () => h.core }));
@@ -19,15 +21,22 @@
       return h.route;
     },
     go: h.go,
-    href: (r: { name: string; ulid?: string; store?: string }) =>
+    href: (r: { name: string; ulid?: string; store?: string; tab?: string }) =>
       r.name === "task"
         ? `/tasks/${r.ulid}`
         : r.name === "project"
           ? `/projects/${r.ulid}`
-          : `/workspaces/${r.store}/${r.ulid}`,
+          : r.name === "settings"
+            ? `/settings/workspaces/${r.store}`
+            : `/workspaces/${r.store}/${r.ulid}${r.tab === "overview" ? "" : `/${r.tab}`}`,
   },
 }));
 vi.mock("../toast.svelte", () => ({ toast: h.toast }));
+vi.mock("../undo", async (orig) => ({
+  ...(await orig<typeof import("../undo")>()),
+  runUndo: h.runUndo,
+}));
+vi.mock("../workspace/dialogs-state.svelte", () => ({ openWorkspaceAdd: h.addWorkspace }));
 
 import CommandPalette from "./CommandPalette.svelte";
 
@@ -82,7 +91,8 @@
   h.core = {
     api,
     cache: {
-      task: (u: string) => (u === "U6" ? rel6 : undefined),
+      task: (u: string) =>
+        u === "U6" ? rel6 : u === "U9" ? { ...rel6, ulid: "U9", store: "backend" } : undefined,
       applyWrite: vi.fn(),
       epochNow: () => 0,
       prime: vi.fn(),
@@ -104,6 +114,8 @@
 beforeEach(() => {
   h.go.mockReset();
   h.toast.mockReset();
+  h.runUndo.mockClear();
+  h.addWorkspace.mockClear();
   localStorage.clear();
   closePalette();
   closeCreate();
@@ -146,6 +158,62 @@
     expect(screen.getByText("コマンド", { selector: "[data-mode]" })).toBeInTheDocument();
     await fireEvent.keyDown(box(), { key: "Enter" });
     await waitFor(() => expect(api.POST).toHaveBeenCalledWith("/api/sync"));
+    expect(palette.open).toBe(false);
+  });
+
+  it("adds a workspace from the command", async () => {
+    setup();
+    render(CommandPalette);
+    openPalette();
+    await fireEvent.input(await screen.findByRole("combobox"), {
+      target: { value: ">ワークスペースを追加" },
+    });
+    await screen.findByRole("option", { name: /ワークスペースを追加/ });
+    await fireEvent.keyDown(box(), { key: "Enter" });
+    expect(h.addWorkspace).toHaveBeenCalledTimes(1);
+    expect(palette.open).toBe(false);
+  });
+
+  it("takes back the last operation without asking, in the store of the workspace on screen", async () => {
+    const { api } = setup();
+    h.route = { name: "workspace", store: "backend", ulid: "W9" };
+    render(CommandPalette);
+    openPalette();
+    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">取り消" } });
+    await screen.findByRole("option", { name: /直前の操作を取り消す/ });
+    await fireEvent.keyDown(box(), { key: "Enter" });
+    expect(h.runUndo).toHaveBeenCalledTimes(1);
+    expect(h.runUndo).toHaveBeenCalledWith(expect.objectContaining({ api }), "backend");
+    expect(palette.open).toBe(false);
+    expect(screen.queryByRole("dialog")).toBeNull();
+  });
+
+  it("takes back in the store of the task on screen, and in none elsewhere", async () => {
+    setup();
+    h.route = { name: "task", ulid: "U9" };
+    render(CommandPalette);
+    openPalette();
+    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">取り消" } });
+    await fireEvent.keyDown(box(), { key: "Enter" });
+    expect(h.runUndo).toHaveBeenLastCalledWith(expect.anything(), "backend");
+
+    closePalette();
+    h.route = { name: "home" };
+    openPalette();
+    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">取り消" } });
+    await fireEvent.keyDown(box(), { key: "Enter" });
+    expect(h.runUndo).toHaveBeenLastCalledWith(expect.anything(), undefined);
+  });
+
+  it("opens the settings of the workspace", async () => {
+    setup();
+    render(CommandPalette);
+    openPalette();
+    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">設定" } });
+    await fireEvent.click(
+      await screen.findByRole("option", { name: /ワークスペースの設定を開く/ }),
+    );
+    expect(h.go).toHaveBeenCalledWith("/workspaces/shared/W1/settings");
     expect(palette.open).toBe(false);
   });
 
```

Run: `cd web && pnpm exec vitest run app/palette`
Expected: FAIL（コマンドの一覧の期待が合わず、「ワークスペースを追加」「直前の操作を取り消す」「ワークスペースの設定を開く」が見つからない）

なお、`WorkspaceEntry` を作っている既存のフィクスチャへの `tasks`・`deleted`・`prefixSet` の追加は Task 1 の Step 4 が持つので、この差分には含めない。

次に実装。`Icon.svelte`（アイコン `undo`。24 グリッド・線の太さ 2）、`search.ts`、`CommandPalette.svelte`。

**Task 1 との境目**: 下の `CommandPalette.svelte` の差分のうち `hrefs.workspace` と `hrefs.project` の 2 行（`tab: "overview"`・`tab: "tree"` を足す行）は、Route の型を変える Task 1 が先に直している。この Task で足すのは `addWorkspace`・`undo`・`workspaceSettings`・`settingsWorkspaces` と、`../undo`・`../workspace/dialogs-state.svelte` の 2 つの import だけ（差分は main に対して取ったので、その 2 行は適用済みなら読み飛ばす）。

```diff
--- a/web/app/icons/Icon.svelte
+++ b/web/app/icons/Icon.svelte
@@ -20,7 +20,8 @@
     | "filter"
     | "forbid"
     | "checklist"
-    | "status";
+    | "status"
+    | "undo";
 
   type Def = { box: 16 | 24; w: number; d: string };
 
@@ -70,6 +71,7 @@
     filter: { box: 24, w: 2, d: "M3 5h18l-7 8v6l-4-2v-4z" },
     forbid: { box: 16, w: 1.4, d: "M8 2.5a5.5 5.5 0 1 1 0 11a5.5 5.5 0 0 1 0-11zM4.1 4.1l7.8 7.8" },
     checklist: { box: 24, w: 2, d: "M4 6l2 2 3-3.5M4 14l2 2 3-3.5M13 7h7M13 15h7" },
+    undo: { box: 24, w: 2, d: "M9 14L4 9l5-5M4 9h10.5a5.5 5.5 0 0 1 0 11H11" },
     status: {
       box: 16,
       w: 1.4,
```

```diff
--- a/web/app/palette/search.ts
+++ b/web/app/palette/search.ts
@@ -37,7 +37,7 @@
   type: "command";
   key: string;
   label: string;
-  icon: "sync" | "plus" | "status" | "folder" | "workspace";
+  icon: "sync" | "plus" | "status" | "folder" | "workspace" | "settings" | "undo";
   kbd?: string;
   note?: string;
   store?: string;
@@ -139,10 +139,14 @@
   syncNow: () => void;
   createTask: (parent?: string) => void;
   createProject: () => void;
+  addWorkspace: () => void;
+  undo: () => void;
   workspaces: WorkspaceEntry[];
   go: (path: string) => void;
   hrefs: {
     workspace: (store: string, ulid: string) => string;
+    workspaceSettings: (store: string, ulid: string) => string;
+    settingsWorkspaces: (store: string) => string;
     project: (ulid: string) => string;
     task: (ulid: string) => string;
   };
@@ -218,6 +222,20 @@
       icon: "plus",
       run: env.createProject,
     },
+    {
+      type: "command",
+      key: "cmd:workspace-add",
+      label: "ワークスペースを追加",
+      icon: "plus",
+      run: env.addWorkspace,
+    },
+    {
+      type: "command",
+      key: "cmd:undo",
+      label: "直前の操作を取り消す",
+      icon: "undo",
+      run: env.undo,
+    },
   ];
 
   const places: CommandItem[] = env.workspaces.flatMap((w) => {
@@ -230,6 +248,17 @@
       store: w.store,
       href: wsHref,
       run: () => env.go(wsHref),
+    };
+    const settingsHref = env.hrefs.workspaceSettings(w.store, w.ulid);
+    const settings: CommandItem = {
+      type: "command",
+      key: `open:ws-settings:${w.store}:${w.ulid}`,
+      label: "ワークスペースの設定を開く",
+      icon: "settings",
+      note: w.name,
+      store: w.store,
+      href: settingsHref,
+      run: () => env.go(settingsHref),
     };
     const projects = w.projects.map((p): CommandItem => {
       const href = env.hrefs.project(p.ulid);
@@ -244,7 +273,21 @@
         run: () => env.go(href),
       };
     });
-    return [ws, ...projects];
+    return [ws, settings, ...projects];
+  });
+
+  // The page that lists the workspaces of a store, in the settings.
+  const lists: CommandItem[] = [...new Set(env.workspaces.map((w) => w.store))].map((store) => {
+    const href = env.hrefs.settingsWorkspaces(store);
+    return {
+      type: "command",
+      key: `open:settings-workspaces:${store}`,
+      label: "設定のワークスペース一覧を開く",
+      icon: "settings",
+      store,
+      href,
+      run: () => env.go(href),
+    };
   });
 
   const recent = env.recent
@@ -254,7 +297,7 @@
   const groups: PaletteGroup[] = [
     { name: "コマンド", items: commands.filter(keep) },
     { name: "最近開いたもの", items: recent },
-    { name: "開く", items: places.filter(keep) },
+    { name: "開く", items: [...places, ...lists].filter(keep) },
   ];
   if (env.selected) {
     const g = taskCommands(env, env.selected);
```

```diff
--- a/web/app/palette/CommandPalette.svelte
+++ b/web/app/palette/CommandPalette.svelte
@@ -15,6 +15,8 @@
   import { toast } from "../toast.svelte";
   import { patchTask } from "../tree/actions";
   import StoreBadge from "../ui/StoreBadge.svelte";
+  import { routeStore, runUndo } from "../undo";
+  import { openWorkspaceAdd } from "../workspace/dialogs-state.svelte";
   import { fetchMissing, readRecent, rememberTask } from "./recent";
   import {
     buildCommands,
@@ -135,11 +137,21 @@
       syncNow: () => void runSync(c.api),
       createTask: (parent) => openCreate({ kind: "task", parent }),
       createProject: () => openCreate({ kind: "project" }),
+      addWorkspace: () => openWorkspaceAdd(),
+      undo: () =>
+        void runUndo(
+          c,
+          routeStore(router.route, (u) => c.cache.task(u)),
+        ),
       workspaces: c.stores.workspaces(),
       go: (path) => router.go(path),
       hrefs: {
-        workspace: (store, ulid) => router.href({ name: "workspace", store, ulid }),
-        project: (ulid) => router.href({ name: "project", ulid }),
+        workspace: (store, ulid) =>
+          router.href({ name: "workspace", store, ulid, tab: "overview" }),
+        workspaceSettings: (store, ulid) =>
+          router.href({ name: "workspace", store, ulid, tab: "settings" }),
+        settingsWorkspaces: (store) => router.href({ name: "settings", page: "workspaces", store }),
+        project: (ulid) => router.href({ name: "project", ulid, tab: "tree" }),
         task: (ulid) => router.href({ name: "task", ulid }),
       },
       selected: sel ? (c.cache.task(sel) ?? null) : null,
```

Run: `cd web && pnpm exec vitest run app/palette app/undo.test.ts app/workspace`
Expected: PASS

- [ ] **Step 8: パレットの選んだタスクのコマンドに「別のワークスペースへ移動…」を足す**

選んだタスク（タスク詳細を開いているか、ツリーで選んでいるもの）のコマンドの組「<ID> に対して」に、「別のワークスペースへ移動…」を足す。Task 2 の `openWorkspaceMove(task)` を呼ぶ（移動のダイアログは Task 2。パレットは閉じて、ダイアログが開く）。キーは `task-move:<ULID>`、アイコンは `workspace`、組の中の並びは「詳細を開く」の次（最後）。

先にテストを直す。`web/app/palette/search.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
    pickStatus: vi.fn(),
    ...over,
```

変更後:
```ts
    pickStatus: vi.fn(),
    moveTask: vi.fn(),
    ...over,
```

変更前:
```ts
    const [status, child, detail] = g.items;
```

変更後:
```ts
    const [status, child, detail, move] = g.items;
    expect(g.items).toHaveLength(4);
    expect(move).toMatchObject({
      key: "task-move:T1",
      label: "別のワークスペースへ移動…",
      icon: "workspace",
    });
    if (move?.type === "command") move.run();
    expect(e.moveTask).toHaveBeenCalledWith(t);
```

`web/app/palette/CommandPalette.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
  addWorkspace: vi.fn(),
}));
```

変更後:
```ts
  addWorkspace: vi.fn(),
  moveWorkspace: vi.fn(),
}));
```

変更前:
```ts
vi.mock("../workspace/dialogs-state.svelte", () => ({ openWorkspaceAdd: h.addWorkspace }));
```

変更後:
```ts
vi.mock("../workspace/dialogs-state.svelte", () => ({
  openWorkspaceAdd: h.addWorkspace,
  openWorkspaceMove: h.moveWorkspace,
}));
```

変更前:
```ts
  h.addWorkspace.mockClear();
```

変更後:
```ts
  h.addWorkspace.mockClear();
  h.moveWorkspace.mockClear();
```

変更前:
```ts
  it("does nothing on Enter or the arrows while the IME is composing", async () => {
```

変更後:
```ts
  it("opens the move dialog for the selected task from the command", async () => {
    setup();
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">移動" } });
    await fireEvent.click(await screen.findByRole("option", { name: /別のワークスペースへ移動/ }));
    expect(h.moveWorkspace).toHaveBeenCalledTimes(1);
    expect(h.moveWorkspace).toHaveBeenCalledWith(expect.objectContaining({ ulid: "U6" }));
    expect(palette.open).toBe(false);
  });

  it("does nothing on Enter or the arrows while the IME is composing", async () => {
```

Run: `cd web && pnpm exec vitest run app/palette`
Expected: FAIL（`moveTask` が `CommandEnv` になく、「別のワークスペースへ移動…」が見つからない）

実装。`web/app/palette/search.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
  pickStatus: (t: TaskView) => void;
};
```

変更後:
```ts
  pickStatus: (t: TaskView) => void;
  moveTask: (t: TaskView) => void;
};
```

変更前:
```ts
        kbd: kbdLabel("mod+enter"),
        href,
        run: () => env.go(href),
      },
```

変更後:
```ts
        kbd: kbdLabel("mod+enter"),
        href,
        run: () => env.go(href),
      },
      {
        type: "command",
        key: `task-move:${t.ulid}`,
        label: "別のワークスペースへ移動…",
        icon: "workspace",
        run: () => env.moveTask(t),
      },
```

`web/app/palette/CommandPalette.svelte` を変える（`pnpm fix` が整形を直す）:

変更前:
```svelte
  import { openWorkspaceAdd } from "../workspace/dialogs-state.svelte";
```

変更後:
```svelte
  import { openWorkspaceAdd, openWorkspaceMove } from "../workspace/dialogs-state.svelte";
```

変更前:
```svelte
      pickStatus,
    };
```

変更後:
```svelte
      pickStatus,
      moveTask: (t) => openWorkspaceMove(t),
    };
```

`openWorkspaceMove` の引数 `MoveTask` は `TaskView` の `ulid`・`id`・`title`・`store`・`workspace` だけの `Pick` なので、`TaskView` をそのまま渡せる。

Run: `cd web && pnpm exec vitest run app/palette`
Expected: PASS

- [ ] **Step 9: ワークスペースの削除のトーストに取り消しを付ける（Task 2 の `WorkspaceDelete` を直す）**

4d のタスクの削除は、トーストに取り消し（10 秒）を付けている。ワークスペースの削除もそろえる。取り消しは、この Task の `runUndo`（確認なし）を、削除した Store を渡して呼ぶ。Task 2 は `runUndo` より前に入るので、ここで足す。

`web/app/workspace/WorkspaceDelete.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
import { resetToasts, toasts } from "../toast.svelte";
import { entry, written } from "./testkit";
import WorkspaceDelete from "./WorkspaceDelete.svelte";
```

変更後:
```ts
import { resetToasts, toasts } from "../toast.svelte";
import { runUndo } from "../undo";
import { entry, written } from "./testkit";
import WorkspaceDelete from "./WorkspaceDelete.svelte";

vi.mock("../undo", () => ({ runUndo: vi.fn(async () => {}) }));
```

変更前:
```ts
  it("leaves the screen of the workspace that is removed", async () => {
```

変更後:
```ts
  it("puts an undo on the toast, which takes the removal back in the store of the workspace", async () => {
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    const undo = toasts.list[0]?.undo;
    expect(undo).toBeTypeOf("function");
    await undo?.();
    expect(runUndo).toHaveBeenCalledTimes(1);
    expect(runUndo).toHaveBeenCalledWith(expect.anything(), "shared");
  });

  it("leaves the screen of the workspace that is removed", async () => {
```

Run: `cd web && pnpm exec vitest run app/workspace/WorkspaceDelete.test.ts`
Expected: FAIL（トーストに `undo` がない）

`web/app/workspace/WorkspaceDelete.svelte` を変える（`pnpm fix` が整形を直す）:

変更前:
```svelte
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
```

変更後:
```svelte
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import { runUndo } from "../undo";
```

変更前:
```svelte
  const { api, stores } = core();
```

変更後:
```svelte
  const c = core();
  const { api, stores } = c;
```

変更前:
```svelte
      toast({ message: `ワークスペース ${gone.name} を削除しました` });
```

変更後:
```svelte
      toast({
        message: `ワークスペース ${gone.name} を削除しました`,
        undo: () => runUndo(c, gone.store),
      });
```

`WorkspaceDelete.svelte` の本文の「コマンドパレットの「直前の操作を取り消す」（hh undo）で元に戻せます。」はそのままでよい（トーストの取り消しは 10 秒で消えるので、あとからはパレットで戻す）。

Run: `cd web && pnpm exec vitest run app/workspace`
Expected: PASS

- [ ] **Step 10: E2E を足す**

概要は CLI の変更に追従すること（件数・プロジェクトの進捗・一覧）、作成ボタンの入れ先、undo がパレットから確認なしで効くことを見る。概要の URL は `/workspaces/demo/<ULID>`。ULID は `hh workspace ls --json` から取る（`/` が概要かどうかは Task 1 の決めごとなので、この spec は `/` に頼らない）。

`web/e2e/screens.ts` の先頭のコメントの `Detail` の項の最後の行のあとに、次を足す:

```ts
//               property is an input labelled by its name ("due")
//   Overview    /workspaces/<store>/<ulid> shows "<n> 件 · プロジェクト <m>", the legend "未着手 <n>"
//               ... "中止 <n>" (exact text), and the regions "プロジェクト" and "プロジェクトの外の
//               タスク" (name them exactly: the second name starts with the first), whose rows are
//               named by the ID and the title; the buttons "タスクを作成" and "プロジェクトを作成"
//   Undo        the palette command "直前の操作を取り消す"; the toast says "<ID> の変更を取り消しました"
```

`web/e2e/overview.spec.ts`:

```ts
// Screens this spec relies on (see screens.ts for the shared list):
//   - /workspaces/<store>/<ulid> is the overview of a workspace: the text "<n> 件 · プロジェクト <m>"
//     (every task that is not deleted, projects included), the legend "未着手 <n>", "進行中 <n>",
//     "完了 <n>", "中止 <n>", and two regions: "プロジェクト" (a row for each project: its ID, title
//     and progress "<done>/<total>") and "プロジェクトの外のタスク" (the tasks at the root that are
//     not projects, in rank order; the link "ツリーで開く" goes to the tree tab of the workspace)
//   - the buttons "タスクを作成" and "プロジェクトを作成" open the create dialogs for this workspace
//   - the overview follows the store: a change the CLI makes changes the counts and the lists
import { expect, test } from "./harness";
import { createDialog, createReady, toast } from "./screens";

type Workspace = { name: string; ulid: string };

test("the overview counts the tasks by category and lists the projects and the loose tasks", async ({
  page,
  hh,
}) => {
  const project = hh.add("Release prep", "--kind", "project");
  const notes = hh.add("Write notes", "--parent", project.id);
  hh.add("Cut the tag", "--parent", project.id);
  hh.hh("set", notes.id, "status=done");
  const loose = hh.add("Loose task");
  const busy = hh.add("Busy alone");
  hh.hh("set", busy.id, "status=in_progress");
  const [ws] = hh.json<Workspace[]>("workspace", "ls");
  if (!ws) throw new Error("the store has no workspace");

  await hh.open(page, `/workspaces/demo/${ws.ulid}`);

  // project, two children and two loose tasks
  await expect(page.getByText("5 件 · プロジェクト 1")).toBeVisible();
  await expect(page.getByText("未着手 3", { exact: true })).toBeVisible();
  await expect(page.getByText("進行中 1", { exact: true })).toBeVisible();
  await expect(page.getByText("完了 1", { exact: true })).toBeVisible();
  await expect(page.getByText("中止 0", { exact: true })).toBeVisible();

  const projects = page.getByRole("region", { name: "プロジェクト", exact: true });
  const projectRow = projects.getByRole("row", { name: new RegExp(project.id) });
  await expect(projectRow).toContainText("Release prep");
  await expect(projectRow).toContainText("1/2");

  const loosely = page.getByRole("region", { name: "プロジェクトの外のタスク" });
  await expect(loosely.getByRole("row", { name: new RegExp(loose.id) })).toContainText(
    "Loose task",
  );
  await expect(loosely.getByRole("row", { name: new RegExp(busy.id) })).toBeVisible();
  // the children of the project are not at the root
  await expect(loosely.getByText("Write notes")).toHaveCount(0);

  await loosely.getByRole("link", { name: "ツリーで開く" }).click();
  await expect(page).toHaveURL(new RegExp(`/workspaces/demo/${ws.ulid}/tree$`));
});

test("the overview follows what the CLI changes", async ({ page, hh }) => {
  const project = hh.add("Release prep", "--kind", "project");
  const first = hh.add("First", "--parent", project.id);
  hh.add("Second", "--parent", project.id);
  const [ws] = hh.json<Workspace[]>("workspace", "ls");
  if (!ws) throw new Error("the store has no workspace");

  await hh.open(page, `/workspaces/demo/${ws.ulid}`);
  await expect(page.getByText("3 件 · プロジェクト 1")).toBeVisible();
  const projects = page.getByRole("region", { name: "プロジェクト", exact: true });
  await expect(projects.getByRole("row", { name: new RegExp(project.id) })).toContainText("0/2");

  hh.hh("set", first.id, "status=done");
  await expect(projects.getByRole("row", { name: new RegExp(project.id) })).toContainText("1/2");
  await expect(page.getByText("完了 1", { exact: true })).toBeVisible();

  const added = hh.add("Added from the terminal");
  await expect(page.getByText("4 件 · プロジェクト 1")).toBeVisible();
  const loosely = page.getByRole("region", { name: "プロジェクトの外のタスク" });
  await expect(loosely.getByRole("row", { name: new RegExp(added.id) })).toBeVisible();

  hh.hh("rm", added.id);
  await expect(page.getByText("3 件 · プロジェクト 1")).toBeVisible();
  await expect(loosely.getByRole("row", { name: new RegExp(added.id) })).toHaveCount(0);
});

test("the create button makes a task in this workspace, and it shows in the list", async ({
  page,
  hh,
}) => {
  hh.add("Already here");
  const [ws] = hh.json<Workspace[]>("workspace", "ls");
  if (!ws) throw new Error("the store has no workspace");

  await hh.open(page, `/workspaces/demo/${ws.ulid}`);
  await expect(page.getByText("1 件 · プロジェクト 0")).toBeVisible();
  await page.getByRole("button", { name: "タスクを作成", exact: true }).click();
  const dialog = createDialog(page);
  await expect(dialog).toBeVisible();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("From the overview");
  await createReady(dialog);
  await dialog.getByRole("button", { name: /^作成/ }).click();

  await expect(toast(page, "DM-2 を作成しました")).toBeVisible();
  await expect(page.getByText("2 件 · プロジェクト 0")).toBeVisible();
  const loosely = page.getByRole("region", { name: "プロジェクトの外のタスク" });
  await expect(loosely.getByRole("row", { name: /DM-2/ })).toContainText("From the overview");
  expect(hh.show("DM-2").title).toBe("From the overview");
});

test("a workspace with no task says so", async ({ page, hh }) => {
  const [ws] = hh.json<Workspace[]>("workspace", "ls");
  if (!ws) throw new Error("the store has no workspace");

  await hh.open(page, `/workspaces/demo/${ws.ulid}`);
  await expect(page.getByText("demo にはまだタスクがありません")).toBeVisible();

  hh.add("The first one");
  await expect(page.getByText("1 件 · プロジェクト 0")).toBeVisible();
});
```

`web/e2e/undo.spec.ts`:

```ts
// Screens this spec relies on (see screens.ts for the shared list):
//   - the palette (Ctrl+K) lists the command "直前の操作を取り消す" when the input starts with >;
//     Enter runs it without asking, and the toast says "<ID> の変更を取り消しました" (the
//     operation is named as the History tab of the detail names it: 変更, 作成 ...; the ID is
//     left out for a commit that is not about a task)
//   - the settings tab of a workspace (/workspaces/<store>/<ulid>/settings) has the button "削除", whose
//     dialog "ワークスペース <name> を削除" has the button "削除"; the toast "ワークスペース <name> を
//     削除しました" has the button "取り消し", which takes the removal back
import type { Page } from "@playwright/test";
import { expect, test } from "./harness";
import { properties, toast } from "./screens";

async function runUndoFromPalette(page: Page): Promise<void> {
  await page.keyboard.press("ControlOrMeta+k");
  const palette = page.getByRole("dialog", { name: "ID・検索・コマンド" });
  await palette.getByRole("combobox").fill(">取り消");
  await expect(palette.getByRole("option", { name: /直前の操作を取り消す/ })).toBeVisible();
  await page.keyboard.press("Enter");
  await expect(palette).toBeHidden();
}

test("the palette takes back my last operation without asking", async ({ page, hh }) => {
  const t = hh.add("Undo me");
  hh.hh("set", t.id, "status=done");
  expect(hh.show(t.id).status).toBe("done");
  await hh.open(page, `/tasks/${t.ulid}`);
  const status = properties(page).getByRole("button", { name: "ステータス" });
  await expect(status).toContainText("完了");

  await runUndoFromPalette(page);

  await expect(toast(page, /変更を取り消しました/)).toBeVisible();
  // no dialog asked first
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect.poll(() => hh.show(t.id).status).toBe("todo");
  // the detail follows, and the page does not show its own undo as a change from outside
  await expect(status).toContainText("未着手");
  await expect(page.getByText("CLI で更新 · 今")).toHaveCount(0);
});

test("a second undo takes back the operation before", async ({ page, hh }) => {
  const t = hh.add("Twice");
  hh.hh("set", t.id, "status=in_progress");
  hh.hh("set", t.id, "status=done");
  await hh.open(page, `/tasks/${t.ulid}`);

  await runUndoFromPalette(page);
  await expect.poll(() => hh.show(t.id).status).toBe("in_progress");
  await runUndoFromPalette(page);
  await expect.poll(() => hh.show(t.id).status).toBe("todo");
});

test("the toast of a workspace removal takes it back", async ({ page, hh }) => {
  hh.hh("workspace", "add", "sandbox", "--prefix", "SBX");
  const list = () => hh.json<{ ulid: string; name: string }[]>("workspace", "ls");
  const sandbox = list().find((w) => w.name === "sandbox");
  if (!sandbox) throw new Error("no workspace sandbox");

  await hh.open(page, `/workspaces/demo/${sandbox.ulid}/settings`);
  await page.getByRole("button", { name: "削除", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "ワークスペース sandbox を削除" });
  await dialog.getByRole("button", { name: "削除", exact: true }).click();

  const removed = toast(page, "ワークスペース sandbox を削除しました");
  await expect(removed).toBeVisible();
  expect(list().map((w) => w.name)).not.toContain("sandbox");
  await removed.getByRole("button", { name: "取り消し" }).click();

  await expect(toast(page, /取り消しました/)).toBeVisible();
  await expect.poll(() => list().map((w) => w.name)).toContain("sandbox");
});
```

`/変更を取り消しました/` の正規表現は、実装の段階で実物のトーストの文言を確かめて、ID 入り（`<ID> の変更を取り消しました`）に絞ってよい。

- [ ] **Step 11: 検証とコミット**

Run: `cd web && pnpm fix && pnpm check && pnpm test`
Expected: 全件 PASS、`svelte-check` の警告 0、`biome` の指摘 0

`make e2e` はここでは流さない。`overview.spec.ts` の 4 件と `undo.spec.ts` のワークスペースの削除の取り消しの 1 件は、概要と設定のタブを画面につなぐ Task 4 のあとで通る。Task 4 の Step 16 で全 spec（`overview.spec.ts` の 4 件と `undo.spec.ts` の 3 件を含む）を流す。既存の spec が落ちるときは、Task 1 が URL を直し忘れている（`/` が概要になったため、ツリーを前提にした spec は `/tree` 付きの URL へ）

```bash
git add web/app/workspace/counts.ts web/app/workspace/counts.test.ts \
  web/app/workspace/WorkspaceOverview.svelte web/app/workspace/WorkspaceOverview.test.ts \
  web/app/undo.ts web/app/undo.test.ts web/app/icons/Icon.svelte \
  web/app/workspace/WorkspaceDelete.svelte web/app/workspace/WorkspaceDelete.test.ts \
  web/app/palette web/e2e/overview.spec.ts web/e2e/undo.spec.ts web/e2e/screens.ts
git commit -m "feat(web): ワークスペースの概要・パレットのコマンド・undo・削除の取り消し"
```

---

### Task 4: 結線（ワークスペースと設定の本文、サイドバーのワークスペースの行の「…」）

Task 1 が用意した場所に、Task 3 の概要、Task 2 の設定のタブと設定のワークスペース一覧をつなぎ、サイドバーのワークスペースの行に「…」のメニューを足す。Task 1・2・3 が済んでから行う（この Task の import は Task 2・3 のファイルを指す）。この Task で `make e2e` の全 spec（Task 2 の `workspace.spec.ts`、Task 3 の `overview.spec.ts`・`undo.spec.ts` を含む）が通るようになる。

**Files:**
- Modify: `web/app/workspace/WorkspacePage.svelte`、`WorkspacePage.test.ts`（概要は `WorkspaceOverview`、設定のタブは `WorkspaceSettings`）
- Modify: `web/app/settings/SettingsRoute.svelte`、`settings.test.ts`（本文に `SettingsWorkspaces`）
- Modify: `web/app/sidebar/Sidebar.svelte`、`Sidebar.test.ts`（ワークスペースの行の「…」）
- Modify: `web/e2e/routes.spec.ts`（「…」の spec）

**Interfaces:**
- Consumes:
  - Task 1: `WorkspacePage.svelte`（props `{ store: string; ulid: string; tab: WorkspaceTab }`）、`SettingsRoute.svelte`（props `{ page: SettingsPage }`）、`SettingsLayout.svelte`、`WorkspaceHeader.svelte`、`Sidebar.svelte`、`ui/Menu`（`anchor`・`items`・`label`・`onselect`・`onclose`）、`Icon`（`more`）、`router.go`・`hrefOf`、`web/e2e/routes.spec.ts` の `sidebar(page)`
  - Task 2: `web/app/workspace/WorkspaceSettings.svelte`（props `{ entry: WorkspaceEntry }`）、`web/app/settings/SettingsWorkspaces.svelte`（props `{ store: string }`）、`web/app/workspace/dialogs-state.svelte.ts` の `openWorkspaceDelete(entry: WorkspaceEntry)`
  - Task 3: `web/app/workspace/WorkspaceOverview.svelte`（props `{ entry: WorkspaceEntry }`。見出しの `WorkspaceHeader`（`tab="overview"`・`actions` つき）を自分で描く）
- Produces: 新しいファイル・公開する名前はない。画面の言葉（E2E が使う）: ワークスペースの行の「…」は button「<名前> の操作（設定を開く・削除）」、そのメニューは menu「ワークスペースの操作」（項目「設定を開く」「削除…」）

- [ ] **Step 14: ワークスペースの画面と設定の画面に本文をつなぐ**

概要のタブは Task 3 の `WorkspaceOverview`（見出しごと）、設定のタブは見出しの下に Task 2 の `WorkspaceSettings`、設定のページは Task 2 の `SettingsWorkspaces`。App には印を置かず、この Step が `WorkspacePage.svelte` と `SettingsRoute.svelte` に直接つなぐ。

`web/app/workspace/WorkspacePage.svelte` を変える（`pnpm fix` が整形を直す）:

変更前:
```svelte
  import WorkspaceHeader from "./WorkspaceHeader.svelte";
```

変更後:
```svelte
  import WorkspaceHeader from "./WorkspaceHeader.svelte";
  import WorkspaceOverview from "./WorkspaceOverview.svelte";
  import WorkspaceSettings from "./WorkspaceSettings.svelte";
```

変更前:
```svelte
      <WorkspaceHeader {entry} tab="overview" />
```

変更後:
```svelte
      <WorkspaceOverview {entry} />
```

変更前:
```svelte
        {#if tab === "tree"}
          <TreeView view={{ kind: "workspace", store, ulid }} />
        {/if}
```

変更後:
```svelte
        {#if tab === "tree"}
          <TreeView view={{ kind: "workspace", store, ulid }} />
        {:else}
          <WorkspaceSettings {entry} />
        {/if}
```

`WorkspacePage` のテストは、2 つの本文を「どちらが描かれたか」だけを言う部品に替える（本文のテストは Task 2・3 にある）。概要は見出しごと `WorkspaceOverview` が描くので、ページの見出しのテストは設定のタブだけにする。`web/app/workspace/WorkspacePage.test.ts` を変える:

`web/app/workspace/WorkspacePage.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
import { afterEach, beforeEach, describe, expect, it } from "vitest";
```

変更後:
```ts
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
```

変更前:
```ts
import WorkspacePage from "./WorkspacePage.svelte";
```

変更後:
```ts
import WorkspacePage from "./WorkspacePage.svelte";

// The bodies of the tabs have their own tests: here each only says which one is drawn.
const { marker } = vi.hoisted(() => ({
  marker: (what: string) => (anchor: Comment, props: { entry: { name: string } }) => {
    const p = document.createElement("p");
    p.textContent = `${what} of ${props.entry.name}`;
    anchor.before(p);
  },
}));
vi.mock("./WorkspaceOverview.svelte", () => ({ default: marker("overview") }));
vi.mock("./WorkspaceSettings.svelte", () => ({ default: marker("settings") }));
```

変更前:
```ts
  it.each(["overview", "settings"] as const)("%s のタブでも見出しとタブを出す", (tab) => {
    const { getByRole } = mount(tab);
    expect(getByRole("heading", { level: 1, name: "release" })).toBeTruthy();
    expect(
      getByRole("navigation", { name: "ビューの切り替え" }).querySelector("[aria-current]")
        ?.textContent,
    ).toContain(tab === "overview" ? "概要" : "設定");
  });
```

変更後:
```ts
  it("設定のタブは見出しとタブの下に設定を出す", () => {
    const { getByRole, getByText } = mount("settings");
    expect(getByRole("heading", { level: 1, name: "release" })).toBeTruthy();
    expect(
      getByRole("navigation", { name: "ビューの切り替え" }).querySelector("[aria-current]")
        ?.textContent,
    ).toContain("設定");
    expect(getByText("settings of release")).toBeTruthy();
  });

  it("概要のタブは見出しごと WorkspaceOverview に任せる", () => {
    const { getByText, queryByRole } = mount("overview");
    expect(getByText("overview of release")).toBeTruthy();
    expect(queryByRole("heading", { level: 1 })).toBeNull(); // the page draws none of its own
  });
```

`web/app/settings/SettingsRoute.svelte` を変える（`pnpm fix` が整形を直す）:

変更前:
```svelte
  import SettingsLayout from "./SettingsLayout.svelte";
```

変更後:
```svelte
  import SettingsLayout from "./SettingsLayout.svelte";
  import SettingsWorkspaces from "./SettingsWorkspaces.svelte";
```

変更前:
```svelte
  <SettingsLayout {page} />
```

変更後:
```svelte
  <SettingsLayout {page}>
    <SettingsWorkspaces store={page.store} />
  </SettingsLayout>
```

`web/app/settings/settings.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
import SettingsRoute from "./SettingsRoute.svelte";
```

変更後:
```ts
import SettingsRoute from "./SettingsRoute.svelte";

// SettingsWorkspaces has its own test: here it only says it was drawn, and for which Store.
vi.mock("./SettingsWorkspaces.svelte", () => ({
  default: (anchor: Comment, props: { store: string }) => {
    const p = document.createElement("p");
    p.textContent = `workspaces of ${props.store}`;
    anchor.before(p);
  },
}));
```

変更前:
```ts
  it("/settings は最初の Store
```

変更後:
```ts
  it("本文にその Store のワークスペースの一覧を出す", () => {
    const { getByRole, getByText } = renderWithCore(
      SettingsRoute,
      { page },
      core(["shared", "me"]),
    );
    expect(getByRole("region", { name: "設定の本文" })).toContainElement(
      getByText("workspaces of shared"),
    );
  });

  it("/settings は最初の Store
```

Run: `cd web && pnpm fix && pnpm vitest run app/workspace app/settings`
Expected: PASS

- [ ] **Step 15: サイドバーのワークスペースの行の「…」**

行の「…」（行に載せたとき・フォーカスしたとき・メニューを開いている間だけ見える）は `ui/Menu` で「設定を開く」「削除…」の 2 つ。「削除…」は Task 2 の `openWorkspaceDelete(entry)` を呼ぶ（残りがあれば、確認を開かずにそのワークスペースの設定タブへ移る）。接頭辞の変更は設定のタブだけに置く（細部 16）。見出しの「＋」は触らない（キャンバスどおり、4d のプロジェクトの追加のまま）。ワークスペースの追加の入口は、設定のワークスペース一覧（Task 2）とコマンドパレット（Task 3）にある。

`web/app/sidebar/Sidebar.test.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
import { renderWithCore } from "../test-utils";
import Sidebar from "./Sidebar.svelte";
```

変更後:
```ts
import { renderWithCore } from "../test-utils";
import { openWorkspaceDelete } from "../workspace/dialogs-state.svelte";
import Sidebar from "./Sidebar.svelte";

vi.mock("../workspace/dialogs-state.svelte", () => ({
  openWorkspaceDelete: vi.fn(),
}));
```

変更前:
```ts
beforeEach(() => {
  history.replaceState(null, "", "/");
  closePalette();
  closeCreate();
```

変更後:
```ts
beforeEach(() => {
  vi.clearAllMocks();
  history.replaceState(null, "", "/");
  closePalette();
  closeCreate();
```

変更前:
```ts
    expect(createState.request).toEqual({ kind: "project" });
  });
});
```

変更後:
```ts
    expect(createState.request).toEqual({ kind: "project" });
  });

  it("ワークスペースの「…」は設定を開く・削除…の 2 つを出す", async () => {
    const { getByRole, getAllByRole } = mount();
    await fireEvent.click(getByRole("button", { name: "release の操作（設定を開く・削除）" }));
    expect(getByRole("menu", { name: "ワークスペースの操作" })).toBeTruthy();
    expect(getAllByRole("menuitem").map((i) => i.textContent?.trim())).toEqual([
      "設定を開く",
      "削除…",
    ]);
  });

  it("「設定を開く」はそのワークスペースの設定のタブへ移る", async () => {
    const { getByRole, queryByRole } = mount();
    await fireEvent.click(getByRole("button", { name: "release の操作（設定を開く・削除）" }));
    await fireEvent.click(getByRole("menuitem", { name: "設定を開く" }));
    expect(location.pathname).toBe("/workspaces/shared/01WSREL/settings");
    expect(queryByRole("menu")).toBeNull();
  });

  it("「削除…」は削除の確認を開く（ワークスペースを渡す）", async () => {
    const { getByRole, queryByRole } = mount();
    await fireEvent.click(getByRole("button", { name: "ops の操作（設定を開く・削除）" }));
    await fireEvent.click(getByRole("menuitem", { name: "削除…" }));
    expect(openWorkspaceDelete).toHaveBeenCalledWith(workspaces[1]);
    expect(location.pathname).toBe("/");
    expect(queryByRole("menu")).toBeNull();
  });
});
```

`web/app/sidebar/Sidebar.svelte` を変える（`pnpm fix` が整形を直す）:

変更前:
```svelte
  import { onMount } from "svelte";
  import { core } from "../context";
```

変更後:
```svelte
  import { onMount } from "svelte";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
```

変更前:
```svelte
  import StoreBadge from "../ui/StoreBadge.svelte";
  import SyncCard from "./SyncCard.svelte";
```

変更後:
```svelte
  import Menu from "../ui/Menu.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { openWorkspaceDelete } from "../workspace/dialogs-state.svelte";
  import SyncCard from "./SyncCard.svelte";
```

変更前:
```svelte
  function workspaceActive(
```

変更後:
```svelte
  // The menu of the "…" of a workspace row: the workspace, and the button it opens from.
  let menu = $state<{ entry: WorkspaceEntry; anchor: HTMLElement } | null>(null);
  const menuItems = [
    { value: "settings", label: "設定を開く" },
    { value: "delete", label: "削除…" },
  ];

  function pickMenu(value: string) {
    const m = menu;
    menu = null;
    if (!m) return;
    if (value === "settings") {
      router.go(
        hrefOf({ name: "workspace", store: m.entry.store, ulid: m.entry.ulid, tab: "settings" }),
      );
    } else {
      openWorkspaceDelete(m.entry);
    }
  }

  function workspaceActive(
```

変更前:
```svelte
      <div class="ws">
        <a
          {href}
          class="ws-link"
          class:active={wsHere}
          aria-current={wsHere ? "page" : undefined}
          onclick={(e) => onLinkClick(e, href)}
        >
          <Icon name="workspace" size={13} />
          <span class="ws-text">
            <span class="ws-name">{w.name}</span>
            <span class="ws-meta">
              <StoreBadge store={w.store} tone={w.tone} compact />
              <span class="prefix" title="ID の接頭辞">{w.prefix}</span>
            </span>
          </span>
        </a>
```

変更後:
```svelte
      <div class="ws">
        <div class="ws-row" class:open={menu?.entry === w}>
          <a
            {href}
            class="ws-link"
            class:active={wsHere}
            aria-current={wsHere ? "page" : undefined}
            onclick={(e) => onLinkClick(e, href)}
          >
            <Icon name="workspace" size={13} />
            <span class="ws-text">
              <span class="ws-name">{w.name}</span>
              <span class="ws-meta">
                <StoreBadge store={w.store} tone={w.tone} compact />
                <span class="prefix" title="ID の接頭辞">{w.prefix}</span>
              </span>
            </span>
          </a>
          <button
            type="button"
            class="more"
            aria-label="{w.name} の操作（設定を開く・削除）"
            aria-haspopup="menu"
            onclick={(e) => (menu = { entry: w, anchor: e.currentTarget })}
          >
            <Icon name="more" size={14} />
          </button>
        </div>
```

変更前:
```svelte
  <div class="sync"><SyncCard /></div>
</nav>
```

変更後:
```svelte
  <div class="sync"><SyncCard /></div>
</nav>

{#if menu}
  <Menu
    anchor={menu.anchor}
    label="ワークスペースの操作"
    items={menuItems}
    onselect={pickMenu}
    onclose={() => (menu = null)}
  />
{/if}
```

変更前:
```svelte
  .ws-link,
  .project {
```

変更後:
```svelte
  .ws-row {
    display: flex;
    align-items: center;
  }
  /* The "…" shows when the row is pointed at or has the focus, or while its menu is open. */
  .more {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: 0 0 auto;
    width: var(--control-h-sm);
    height: var(--control-h-sm);
    margin-left: calc(var(--control-h-sm) * -1);
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--label);
    opacity: 0;
    cursor: pointer;
  }
  .ws-row:hover .more,
  .ws-row:focus-within .more,
  .ws-row.open .more {
    opacity: 1;
  }
  .more:hover {
    background: var(--hover);
  }
  .ws-link,
  .project {
```

変更前:
```svelte
  .ws-link {
    min-height: var(--row-h);
  }
```

変更後:
```svelte
  .ws-link {
    flex: 1;
    min-width: 0;
    min-height: var(--row-h);
  }
```

Run: `cd web && pnpm fix && pnpm vitest run app/sidebar`
Expected: PASS

- [ ] **Step 16: 「…」の E2E を足す**

`web/e2e/routes.spec.ts` を変える（冒頭のコメントと、「…」の spec の追加）:

変更前:
```ts
//   - the sidebar ("メインナビゲーション") has the link "設定" (the gear, /settings)
//   - the settings take the window
```

変更後:
```ts
//   - the sidebar ("メインナビゲーション") has the link "設定" (the gear, /settings) and, for each
//     workspace, the button "<名前> の操作（設定を開く・削除）" that opens the menu
//     "ワークスペースの操作" with 設定を開く and 削除…
//   - the settings take the window
```

変更前:
```ts
test("/settings goes to the workspaces of the first store, with its own nav in place of the sidebar", async ({
```

変更後:
```ts
test("the … of a workspace row opens its settings", async ({ page, hh }) => {
  await hh.open(page);
  await sidebar(page).getByRole("button", { name: "demo の操作（設定を開く・削除）" }).click();
  const menu = page.getByRole("menu", { name: "ワークスペースの操作" });
  await expect(menu.getByRole("menuitem")).toHaveText(["設定を開く", "削除…"]);
  await menu.getByRole("menuitem", { name: "設定を開く" }).click();
  await expect(page).toHaveURL(/\/workspaces\/demo\/[^/]+\/settings$/);
  await expect(menu).toHaveCount(0);
});

test("/settings goes to the workspaces of the first store, with its own nav in place of the sidebar", async ({
```

Run: `cd web && pnpm fix && pnpm check && pnpm test`、続けて `make e2e`（リポジトリのルートで。ここで初めて、Task 2 の `workspace.spec.ts` と Task 3 の `overview.spec.ts`・`undo.spec.ts` のワークスペースの削除の取り消しが画面につながって通る。既存の spec と `routes.spec.ts` も含めて全件）
Expected: PASS

- [ ] **Step 17: コミットする**

```bash
git add web/app web/e2e
git commit -m "feat(web): サイドバーのワークスペースの「…」、ワークスペースと設定の本文をつなぐ"
```


---

## 決めたこと

この plan を組むときに決めた点。利用者の決定は（利用者）と付けた。

1. **見出しを誰が描くか。** 概要は Task 3 の `WorkspaceOverview` が自分で `WorkspaceHeader` を描く（`actions` に作成ボタン）。ツリーと設定のタブは Task 1 の `WorkspacePage` が `WorkspaceHeader` を描いて、その下に本文を置く
2. **App への組み込み。** Task 4 の Step 14 が `WorkspacePage.svelte` と `SettingsRoute.svelte` に直接つなぐ。App に印（HTML コメント）は置かず、Task 2 は印を前提にしない。Task 2・3 の E2E は Task 4 のあとで通る
3. **歯車は `/settings`。** 最初の Store のワークスペースの設定へ置き換える。概要 plan の細部 18 の `/settings/general` は、「同期と保守」ができる 4e-4 で変える
4. **設定のあいだのサイドバー。** サイドバーは `hidden` にして、設定のナビに替える（サイドバーは外さない。アプリのキーを登録しているため）
5. **サイドバーの「＋」は、キャンバスどおりプロジェクトの追加のまま（利用者）。** 4d の作成のプロジェクトのダイアログを開く（4d の `openCreate({ kind: "project" })` のまま、触らない）。ワークスペースの追加の入口は、設定のワークスペース一覧のボタンとコマンドパレットの「ワークスペースを追加」
6. **ワークスペースへのリンクは概要。** Store を作った直後だけは、最初のタスクを足す行のあるツリー
7. **undo は今の画面の Store を渡す。** ワークスペースの画面ならその Store、設定の画面（`/settings/...`）ならその URL の Store（`/settings` のように名前がなければ渡さない）、`/` ならそこに概要が出る最初のワークスペースの Store、プロジェクトとタスクの画面ならそのタスクの Store、ほかは渡さない。パレットの「設定を開く」は、ワークスペースの設定タブと設定のワークスペース一覧の両方。概要の問い合わせは `workspace:<ULID>`（足元の `hh ls` の行は名前と `--store`）
8. **接頭辞を最後に変えた日を出す（利用者）。** 概要 plan の細部 24 の前提「`prefix_set` が API にない」は誤りで、`WorkspaceView.prefix_set` がある。`WorkspaceEntry.prefixSet` を Task 1 が足し、Task 2 が設定のタブ（「最後に変えたのは <日付>」）と設定のワークスペース一覧（列）に出す。日付は `core/format.ts` の `formatDate`
9. **「別のワークスペースへ移動…」の入口（利用者）。** タスク詳細の見出しの「…」（その他の操作）とツリーの行の「…」（Task 2 が新設）、加えてパレットの選んだタスクのコマンド（Task 3 の Step 8）
10. **ワークスペースの削除。** 残り（削除済みを含む）があれば確認を開かずに設定タブへ移る（Task 2）。削除が済んだトーストには取り消し（`runUndo`）を付ける（利用者）。4d のタスクの削除と揃えるため。Task 2 は Task 3 の `runUndo` より先なので、トーストの取り消しは Task 3 の Step 9 で足す
11. **テストの部品。** `web/app/workspace/testkit.ts`（Task 2a）を Task 2b・3 が使う。`WorkspaceEntry` を作る既存の型付きフィクスチャへの `tasks`・`deleted`・`prefixSet` の追加は Task 1 が持つ。Task 2・3 が足す新しいフィクスチャは、自分で 3 項目を持つ
12. **undo の E2E の正規表現。** `undo.spec.ts` の `/変更を取り消しました/` は、実装の段階で実物のトーストの文言を確かめて、ID 入りに絞ってよい（Task 3 の Step 10）
13. **実行の順。** 番号の順（1 → 2 → 3 → 4）。画面につなぐ結線は、Task 2・3 のファイルを import するので Task 4 として独立させた

## 実装で plan から変えた点（2026-11-03）

本文のコードと食い違っていても、次が正。

- **F1（利用者の決定）: プロジェクトの数え方。** 設定のワークスペース一覧の「プロジェクト」列は `WorkspaceEntry.projects` の件数（kind が `project`）。`project-counts.ts` とそのテスト、そのための `watchList`・`schema.load`・「…」表示は作らない。本文の `project-counts` の記述（4360・4373・4384・4700・4739 行付近）は使わない。概要 plan の細部 28 の文言も直した。
- **F2: 移動の API。** `WorkspaceMove` は `body: { workspace: to.ulid }`（ULID）を送る。足元の `hh mv … --workspace <名前>` は名前のまま。
- **F3: 余白。** 概要の `.overview` に `padding: var(--space-20) var(--gutter) 0`、`WorkspaceSettings` の `.settings` に `padding: 0 var(--gutter)`。
- **F4: 凡例。** 概要のカテゴリの凡例の点は `StatusIcon`（形）。帯は `role="img"` と `aria-label`。
- **F5: 寸法。** 色は 16 進数・rgb を書かずトークンを使う。罫線の 1〜2px、列幅、max-width は plan どおり（4d の R9 と同じ）。
- **F6: 重複の整理。** ダイアログの失敗の帯は `ui/FormError.svelte`。`WorkspaceMove` の選択肢は `create/WorkspaceChoice.svelte`（`isDisabled`・`note`・`showStore`・`label` を足した）。開いている間の件数は `core/workspace.ts` の `currentEntry`。busy/failure と IME 判定つき Enter はダイアログごと。
- **F7: 途中のコミット。** Task 1〜3 のあいだは本文が空の画面を許し、入口の判定は最終の差分で行った。
- **F8: E2E。** Task 2b・3 の E2E は Task 4 の結線のあとに流した。
- **F9〜F13: 機械的な読み替え。** 検証は `pnpm fix && pnpm check && pnpm lint` と対象の vitest。プロジェクトのタブは「ツリー」1つ（F10）。行の「…」は `schema` があるときだけ `onmore` を渡し、スキーマ前は同じ幅の `.more-slot`（F11）。`CreateStore.test.ts` を Task 1 に含めた（F12）。undo の E2E の正規表現は ID 入りの文言（`${id} の変更を取り消しました`）に絞った。
- **Ruling: 見出しの作り直し。** `WorkspacePage` の見出しは、概要と ツリー/設定の切り替えで作り直される。決めたこと 1（概要は `WorkspaceOverview` が自分で見出しを描く）を保つため直さない。状態を持たないので見た目も動作も変わらない。
- **Ruling: 余白。** `.main` と見出しの `--gutter` が二重になるのは 4d のツリーと同じ構造で、退行ではないため変えない（利用者の決定、2026-11-03）。
- **Task 2b: 存在しない Store の設定 URL。** `/settings/workspaces/<store>` の Store が Store 一覧になければ、レイアウトを出さず `states/Empty` の `notfound`（`scope="store"`、「Store「x」が見つかりません」）とホームへの戻りを出す。Store 一覧を読み終えるまでは何も言わない。4d-3 の存在しないワークスペースと同じ形。
- **Task 4: `.foot` と `.more-slot`。** `.overview` に `flex: 1 1 auto` を足し、足元の `hh ls` が窓の下端に来るようにした。TreeRow の「…」はスキーマが来ると列が 1.75rem 増えて期限・担当がずれたため、スキーマ前は同じ幅の `.more-slot`（`aria-hidden`）を置く。幅は `.more` と共通の規則に1つにまとめた。

### 最終レビューで直した点

- **`/` と設定の画面の undo（利用者の決定）。** `routeStore` に `first` を足し、`/` は最初のワークスペースの Store、設定の画面は `page.store` を渡す（決めたこと 7 を上のとおり直した）。Store が複数あっても失敗しない。
- 概要でスキーマを読めないときは、帯の場所に「スキーマを読めないため、カテゴリ別の件数を出せません」と理由を出す。
- 設定のワークスペース一覧は、0 件のとき「この Store にはワークスペースがありません」の行を出す。`prefix_set` がゼロ値（year 1）のときは日付の代わりに「—」（`core/workspace.ts` の `prefixDay`）。
- ワークスペース設定の削除の `CliLine` は、削除できないとき無効。
- サイドバーの「…」は `.ws-link` に右の余白を足して長い名前の省略記号と重ならないようにし、開いている印は `store` と `ulid` で比べる（再読み込みで外れない）。
- `SettingsLayout` の外側の `<aside>` を `<div>` にした（ナビの `<nav>` がランドマーク）。
- 移動のダイアログの「今のワークスペース」はキャッシュの今の値を読む。
- パレットの「別のワークスペースへ移動…」は、同じ Store にほかのワークスペースがないとき出さない。「設定のワークスペース一覧を開く」は Store の名前の一覧（`CommandEnv.storeNames`）から作り、ワークスペースが 0 の Store にも出る。
- ワークスペース削除の取り消しの E2E は、サイドバーに行が戻ることも確かめる。
- Go のテストを1本足した: ワークスペースの削除を undo で戻す単体テスト（`internal/ops/undo_test.go`、コードは変えていない）。plan の「この回は Go を触らない」の例外で、利用者の決定。
