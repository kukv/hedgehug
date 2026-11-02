# フェーズ 4d-4: タスク詳細・作成・コマンドパレット Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** タスク詳細、作成ダイアログ、コマンドパレットを作り、4d-3 で外していた入口（C キー、⌘K、行から詳細を開く操作など）を足し、残りの E2E と仕上げで 4d の完了条件（ブラウザでツリーと詳細からタスクを作成・編集でき、CLI の変更が即時に反映される）を満たす。

**Architecture:** フェーズ 4d の4回のうちの1回。決定事項・spec にない細部・ファイル構成・共通の型は概要 `docs/superpowers/plans/2026-11-02-phase4d-web.md` にあり、各 Task はそこを前提にする（細部の番号はそこを指す）。Task の番号は 4d 全体の通し番号。

**Tech Stack:** 概要のとおり。

**Spec:** `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（「4d・4e フロントエンド」「画面と範囲」「4c サーバー」）と、概要 `docs/superpowers/plans/2026-11-02-phase4d-web.md`。

**この回の Task:** Task 11、12、13、14、16b、17

**着手の前に:** 4d-3 がマージ済みであること。

## Global Constraints

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
- Go のテストは `-race` を含めて通す。Go のジョブは `web/dist` なし（`.gitkeep` だけ）で通る

---

## Task 11〜13 の前提

Task 7（`ui/*`・`tokens.css`・`test-utils.ts`）と Task 5・6（core）の最終形を使う。この節のコードは、Task 2・3・4・5・6・7・8・15 を置いた作業ディレクトリ（`svelte-check --fail-on-warnings`・`vitest run`・`biome ci`・`vite build`）で動かして確かめたもの（`pnpm fix` の後の形）。

**使う名前（Task 7 の表のとおり）**: `Button`（`variant`・`size`・`HTMLButtonAttributes`）、`StoreBadge`（`store`・`tone`）、`Avatar`（`name`）、`CliLine`（`command`）、`Dialog`（`open`・`title`・`width`・`cli`・`onclose`・`children`・`footer`）、`Menu`（`anchor`・`items`・`label`・`selected`・`onselect`・`onclose`）、`StatusIcon`・`KindIcon`・`Icon`、`router.href`・`router.go`・`onLinkClick`、`toast`・`toasts`・`undoToast`・`resetToasts`、`selection`、`registerShortcut`・`kbdLabel`・`modKey`、`copyText`、トークンは `--space-*`・`--radius-*`・`--fs-*`・`--control-h*`・`--gutter`・`--hairline`・`--line-soft` など。

**core の挙動への前提（Task 5・6 で確かめる）**
- `SaveController.editing(ulid, field)` は、未送信・送信中・失敗で残っている間は true（本文のエディタが、保存の途中で古い本文に戻らないための前提）
- `TaskCache.applyWrite({ task | tasks, ulids, commit })` は応答のタスクをキャッシュに入れ、`commit` を自分のものとして覚える。コメントのように `TaskView` を返さない書き込みは `noteOwnCommit(commit)` だけ呼び、詳細は `tasks.changed`（自分のコミットも届く）で取り直されるのを待つ
- `PeopleCache` は `AppCore` の6つに入っていない。Task 11 の `peopleOf(core)` が接続ごとに1つ作る

**テストの書き方**: Task 7 の `renderWithCore` で `core()` を差し替え、`router`・`toast`・`selection` は本物を使う（URL は `history.replaceState`、トーストは `toasts.list`、選択は `selection.set`）。`api` は `testkit.ts` の `makeCore()` の `vi.fn` で、`ok(...)`・`fail(...)` を返す。

---

### Task 11: タスク詳細（プロパティ・本文・チェックリスト・ウォッチ）

**Files:**
- Modify: `web/package.json`、`web/pnpm-lock.yaml`（`@codemirror/autocomplete` を足す）
- Create: `web/app/detail/testkit.ts`（Task 11〜13 のテストが使うフェイクとデータ）
- Create: `web/app/detail/field-types.ts`、`checklist-items.ts`、`lookup.ts`、`prop-rows.ts`、`watch.ts`、`people.ts`、`mentions.ts`
- Create: `web/app/detail/ChipsField.svelte`、`FieldInput.svelte`、`TaskPicker.svelte`、`Checklist.svelte`、`BodyEditor.svelte`、`PropsPanel.svelte`、`TaskDetail.svelte`
- Modify: `web/app/App.svelte`（task ルート）
- Modify: `web/app/router.ts`、`router.test.ts`（`/tasks/<ULID>` のルート）
- Modify: `web/app/tree/keyboard.ts`、`keyboard.test.ts`、`TreeRow.svelte`、`TreeView.svelte`、`TreeView.test.ts`（ツリーの行から詳細を開く）
- Test: `web/app/detail/` の同名の `*.test.ts`（下の Step 1 の13本）

**Interfaces:**
- Consumes（Task 3〜7 の最終形）: `core()`・`AppCore`、`renderWithCore`（`test-utils.ts`）、`useSubscribe`、`router.href`・`onLinkClick`、`toast`・`toasts`・`resetToasts`、`selection`、`kbdLabel`・`modKey`、`Button`・`Dialog`・`Menu`・`StoreBadge`・`Avatar`・`CliLine`・`Icon`・`StatusIcon`・`KindIcon`、`copyText`、`unwrap`・`ApiError`・`ShowView`・`TaskView`・`Schemas`、`errorMessage`、`cliLine`、`formatDateTime`、`SchemaView`・`propEditorOf`・`canParent`・`parentRule`・`PropEditor`、`searchTasks`・`matchPeople`・`PeopleCache`、`SaveController.edit/flush/state/editing`、`TaskCache.watchDetail/detail/task/applyWrite`
- Produces（Task 12・13 が使う）:
  - `testkit.ts`: `makeCore(opts)`（`api`・`cache`・`save`・`stores`・`schema`・`conn` のフェイク。メソッドは `vi.fn`）、`mount(Component, props, k)`（`renderWithCore` で描画して effect を流す）、`schemaView`・`showView(over)`・`taskView(over)`・`workspaces`、`ok(data)`・`fail(status, type, args?)`（`api` の応答）、`viewOf(container, n?)`・`typeInto(container, text, n?)`（CodeMirror を触る）、`stubLayout()`
  - `lookup.ts`: `type Crumb = { ulid; id; title; kind; status; parent: string | null; store }`、`type Lookup`、`createLookup({ api, cache, store })`、`ancestorChain(start, lookup, limit?)`（ルート側が先頭）、`collapseCrumbs(chain)`（4件以上は先頭・省略（null）・末尾2件）
  - `prop-rows.ts`: `propRows(sc, values): PropRow[]`（`task_ref`・`status`・`kind`・`blocked_by` を除く。スキーマにないキーは `editor: null`）
  - `FieldInput.svelte`: props `{ editor: "text" | "number" | "date" | "select" | "chips"; value; label; options?: Option[]; nullable?; pending?; readonly?; size?; placeholder?; transform?; invalidMessage?; hintFor?; onedit(v); oncommit() }`。`select` は `ui/Menu` を開くボタン（アクセシブルネームは `label`、中身は今の選択）。`text`・`number` は入力のたびに `onedit`、blur で `oncommit`。`date`・`select`・`chips` は `onedit` の直後に `oncommit`
  - `ChipsField.svelte`（チップの入力。`transform` で正規化・拒否）、`TaskPicker.svelte`（props `{ store; exclude?: string[]; label?; placeholder?; debounceMs?; onpick(t) }`。`searchTasks` で Store の中を探す）
  - `BodyEditor.svelte`: CodeMirror 6 の Markdown エディタ。props `{ value; store; label?; placeholder?; minHeight?; pending?; readonly?; onedit(text); oncommit?(); onsubmit?() }`、メソッド `getText()`・`setText(text)`・`focus()`。本文・コメント・作成の本文が共通で使い、`@`（人）と `#`（タスク）の補完を持つ。`value` に入れた文字列は、フォーカスがなく `pending` でないときだけエディタに反映する（呼び出し側は `onedit` で受けた文字列を `value` に戻す）
  - `mentions.ts`: `mentionExtension(store, deps)`・`mentionSource`・`triggerOf`。`people.ts`: `peopleOf(core)`（`PeopleCache` は `AppCore` の6つに入っていないので、接続ごとに1つここで作る）
  - `Checklist.svelte` の `ontoggle(line: number)`、`checklist-items.ts` の `checklistItems(body)`・`toggleChecklistLine(body, line)`
  - `TaskDetail.svelte` の差し込み位置（Task 12 が置き換える HTML コメント）: `<!-- delete-action -->`（ヘッダーの削除ボタン）、`<!-- deleted-actions -->`（削除済みの帯の復元・完全削除）、`<!-- detail-sections -->`（本文側の関連・コメント・履歴）

**この Task で決めたこと**
- 本文とコメントのエディタは CodeMirror 6。`@`・`#` の補完は `@codemirror/autocomplete` の `autocompletion({ override })` で、`@` は `PeopleCache`（`GET /api/people?store=`）の名前を `matchPeople` で絞り、`#` は `searchTasks` の結果（ID と、詳細にタイトル）を出す。単語の途中の `@`・`#`（メールアドレスなど）は補完しない
- ウォッチ中かどうかは `isWatching(watchers, account)`: `GET /stores` の `account` は、その Store のアカウントの ID（`host/login`。`ops.StoreView.Account` は `Account.ID()`、Task 1 のテスト `TestStoresAccountIsHostSlashLogin` が固定している）で、`ShowView.watchers` と同じ形なので、完全一致で見る。ウォッチ・ウォッチ解除を押した結果は、タスクの `watchers` が変わるまで画面に残す
- `FieldInput` の `select` は `ui/Menu`。選べない種類（親に置けない）は `disabled` で、理由は項目の `hint` に出す

- [ ] **Step 1: 失敗するテストを書く**

`web/app/detail/testkit.ts`:
```ts
import { EditorView } from "codemirror";
import { type Component, flushSync } from "svelte";
import { vi } from "vitest";
import type { Schemas, ShowView, TaskView } from "../../core/api/client";
import type { WorkspaceEntry } from "../../core/stores";
import { renderWithCore } from "../test-utils";

export type SchemaView = Schemas["SchemaView"];

export const schemaView = {
  statuses: [
    { name: "todo", label: "未着手", category: "todo", default: true },
    { name: "in_progress", label: "進行中", category: "doing" },
    { name: "waiting", label: "待機中", category: "doing" },
    { name: "done", label: "完了", category: "done", default: true },
    { name: "canceled", label: "中止", category: "canceled" },
  ],
  kinds: [
    { name: "project", label: "プロジェクト", role: "container", parents: ["root"] },
    { name: "epic", label: "エピック", role: "container", parents: ["root", "project"] },
    { name: "story", label: "ストーリー", role: "work", parents: null },
    { name: "task", label: "タスク", role: "work", parents: null, default: true },
    { name: "bug", label: "バグ", role: "work", parents: null },
  ],
  props: {
    priority: { type: "enum", values: ["low", "mid", "high"] },
    start: { type: "date" },
    due: { type: "date" },
    points: { type: "number" },
    area: { type: "text" },
    relates: { type: "task_ref", multi: true, symmetric: true, label: "関連" },
    duplicates: { type: "task_ref", label: "重複している", inverse: "重複されている" },
  },
  workflow: {},
  scoped: [],
} as unknown as SchemaView;

const WS = { ulid: "W1", name: "release", id_prefix: "REL" };

export function taskView(over: Partial<TaskView> = {}): TaskView {
  return {
    ulid: "U0",
    id: "REL-0",
    store: "shared",
    workspace: WS,
    kind: "task",
    title: "task",
    parent: null,
    rank: "a0",
    status: "todo",
    assignees: [],
    labels: [],
    blocked_by: [],
    props: {},
    created: "2026-11-01T00:00:00Z",
    updated: "2026-11-01T00:00:00Z",
    deleted_at: null,
    problems: [],
    ...over,
  } as TaskView;
}

export function showView(over: Partial<ShowView> = {}): ShowView {
  return {
    ...taskView(),
    body: "",
    watchers: [],
    repos: [],
    comments: [],
    reverse: {},
    referenced_by: [],
    history: [],
    ...over,
  } as ShowView;
}

/** What an Api call answers with on success (the shape unwrap reads). */
export function ok<T>(data: T) {
  return Promise.resolve({ data, response: new Response(null, { status: 200 }) });
}

/** What an Api call answers with when the server refuses (problem+json). */
export function fail(
  status: number,
  type: string,
  args: Record<string, unknown> = {},
  detail = "",
) {
  return Promise.resolve({
    error: { type, title: "", status, detail, args },
    response: new Response(null, { status }),
  });
}

export const workspaces: WorkspaceEntry[] = [
  { store: "shared", tone: "purple", ulid: "W1", name: "release", prefix: "REL", projects: [] },
  { store: "shared", tone: "purple", ulid: "W2", name: "shared", prefix: "SH", projects: [] },
];

export type FakeCoreOptions = {
  show?: ShowView;
  tasks?: TaskView[];
  account?: string;
  workspaces?: WorkspaceEntry[];
  loadError?: Error | null;
};

/** Fakes of the core objects; every method the screens call is a vi.fn. */
export function makeCore(opts: FakeCoreOptions = {}) {
  const tasks = opts.tasks ?? [];
  const listeners = new Set<() => void>();
  const subscribe = (fn: () => void) => {
    listeners.add(fn);
    return () => listeners.delete(fn);
  };
  const ws = opts.workspaces ?? workspaces;
  const api = { GET: vi.fn(), POST: vi.fn(), PATCH: vi.fn(), PUT: vi.fn(), DELETE: vi.fn() };
  const cache = {
    task: vi.fn((u: string) => tasks.find((t) => t.ulid === u)),
    detail: vi.fn(() => opts.show),
    live: vi.fn(() => undefined),
    watchDetail: vi.fn((u: string) => ({
      ulid: () => u,
      error: () => opts.loadError ?? null,
      dispose: vi.fn(),
    })),
    applyWrite: vi.fn(),
    noteOwnCommit: vi.fn(),
    subscribe,
  };
  const save = {
    edit: vi.fn(),
    flush: vi.fn(async () => {}),
    state: vi.fn(
      () => ({ kind: "idle" }) as { kind: string; pendingPush?: boolean; message?: string },
    ),
    editing: vi.fn(() => false),
    subscribe,
  };
  const stores = {
    stores: vi.fn(() => [
      {
        name: "shared",
        default: true,
        path: "/s",
        remotes: [],
        account: opts.account ?? "github.com/koki",
      },
    ]),
    tone: vi.fn(() => "purple"),
    workspaces: vi.fn(() => ws),
    workspace: vi.fn((s: string, u: string) => ws.find((w) => w.store === s && w.ulid === u)),
    loaded: vi.fn(() => true),
    reload: vi.fn(async () => {}),
    subscribe,
  };
  const schema = { get: vi.fn(() => schemaView), load: vi.fn(async () => schemaView), subscribe };
  const conn = { state: "open", subscribe, on: vi.fn(() => () => {}) };
  return {
    api,
    cache,
    save,
    stores,
    schema,
    conn,
    notify: () => {
      for (const fn of listeners) fn();
    },
  };
}

export type FakeCore = ReturnType<typeof makeCore>;

/** Renders a screen with the fakes as its core and runs its effects. */
export function mount<P extends Record<string, unknown>>(C: Component<P>, props: P, k: FakeCore) {
  const { api, cache, save, stores, schema, conn } = k;
  const r = renderWithCore(C, props, { api, cache, save, stores, schema, conn });
  flushSync();
  return r;
}

/** The CodeMirror view of the n-th editor in the container. */
export function viewOf(container: HTMLElement, n = 0): EditorView {
  const dom = container.querySelectorAll<HTMLElement>(".cm-editor")[n];
  const view = dom ? EditorView.findFromDOM(dom) : null;
  if (!view) throw new Error(`no editor #${n}`);
  return view;
}

/** Types into the n-th CodeMirror editor of the container, as the user would. */
export function typeInto(container: HTMLElement, text: string, n = 0): void {
  const view = viewOf(container, n);
  view.dispatch({
    changes: { from: 0, to: view.state.doc.length, insert: text },
    selection: { anchor: text.length },
  });
}

/** jsdom has no layout; CodeMirror measures ranges, so give it empty answers. */
export function stubLayout() {
  const rect = {
    x: 0,
    y: 0,
    width: 0,
    height: 0,
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    toJSON: () => ({}),
  } as DOMRect;
  Range.prototype.getBoundingClientRect = () => rect;
  Range.prototype.getClientRects = () =>
    ({ length: 0, item: () => null, [Symbol.iterator]: function* () {} }) as unknown as DOMRectList;
  document.elementFromPoint = () => null;
}
```

`web/app/detail/checklist-items.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { checklistItems, toggleChecklistLine } from "./checklist-items";

describe("checklistItems", () => {
  it("lists the checkbox lines with their line numbers", () => {
    const body = "intro\n- [ ] a\n* [x] b\n+ [X] c\n1. [ ] d\n2) [x] e\n- plain\n";
    expect(checklistItems(body)).toEqual([
      { line: 1, checked: false, text: "a" },
      { line: 2, checked: true, text: "b" },
      { line: 3, checked: true, text: "c" },
      { line: 4, checked: false, text: "d" },
      { line: 5, checked: true, text: "e" },
    ]);
  });

  it("counts nested items and ignores things that are not checkboxes", () => {
    expect(
      checklistItems("- [ ] a\n  - [x] b\n- [y] c\n- [] d\n- [ x] e\n- [x]f\n").map((i) => i.line),
    ).toEqual([0, 1]);
  });

  it("skips fenced code, and an unclosed fence hides the rest", () => {
    const body = "- [ ] a\n```\n- [x] in code\n```\n~~~md\n- [ ] in code\n~~~\n- [x] b\n";
    expect(checklistItems(body).map((i) => i.line)).toEqual([0, 7]);
    expect(checklistItems("- [x] a\n```\n- [ ] b\n").map((i) => i.line)).toEqual([0]);
    expect(checklistItems("````\n```\n- [ ] in code\n````\n- [ ] a\n").map((i) => i.line)).toEqual([
      4,
    ]);
  });

  it("reads CRLF bodies", () => {
    expect(checklistItems("- [ ] a\r\n- [x] b\r\n")).toEqual([
      { line: 0, checked: false, text: "a" },
      { line: 1, checked: true, text: "b" },
    ]);
  });
});

describe("toggleChecklistLine", () => {
  it("flips only the given line, even when two lines read the same", () => {
    const body = "- [ ] same\n- [ ] same\n";
    expect(toggleChecklistLine(body, 1)).toBe("- [ ] same\n- [x] same\n");
    expect(toggleChecklistLine("- [x] a\n", 0)).toBe("- [ ] a\n");
    expect(toggleChecklistLine("- [X] a\n", 0)).toBe("- [ ] a\n");
  });

  it("keeps CRLF and leaves the body alone when the line is not a checkbox", () => {
    expect(toggleChecklistLine("- [ ] a\r\n- [ ] b\r\n", 0)).toBe("- [x] a\r\n- [ ] b\r\n");
    expect(toggleChecklistLine("text [ ] here\n- [ ] a\n", 0)).toBe("text [ ] here\n- [ ] a\n");
    expect(toggleChecklistLine("```\n- [ ] a\n```\n", 1)).toBe("```\n- [ ] a\n```\n");
    expect(toggleChecklistLine("- [ ] a\n", 9)).toBe("- [ ] a\n");
  });
});
```

`web/app/detail/lookup.test.ts`:
```ts
import { describe, expect, it, vi } from "vitest";
import { ancestorChain, type Crumb, collapseCrumbs, createLookup } from "./lookup";
import { makeCore, ok, showView, taskView } from "./testkit";

describe("createLookup", () => {
  it("answers from the cache without a request", async () => {
    const k = makeCore({ tasks: [taskView({ ulid: "U1", id: "REL-1", title: "one" })] });
    const lookup = createLookup({ api: k.api as never, cache: k.cache as never, store: "shared" });
    expect((await lookup("U1"))?.id).toBe("REL-1");
    expect(k.api.GET).not.toHaveBeenCalled();
  });

  it("fetches a task the cache lacks once for concurrent calls, and gives undefined on an error", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation(() => ok(showView({ ulid: "U9", id: "REL-9", title: "nine" })));
    const lookup = createLookup({ api: k.api as never, cache: k.cache as never, store: "shared" });
    const [a, b] = await Promise.all([lookup("REL-9"), lookup("REL-9")]);
    expect(a?.ulid).toBe("U9");
    expect(b).toBe(a);
    expect(k.api.GET).toHaveBeenCalledTimes(1);
    k.api.GET.mockImplementation(() => Promise.reject(new Error("down")));
    expect(await lookup("REL-10")).toBeUndefined();
  });
});

describe("ancestorChain", () => {
  const crumb = (ulid: string, parent: string | null): Crumb => ({
    ulid,
    id: ulid,
    title: ulid,
    kind: "task",
    status: "todo",
    parent,
    store: "shared",
  });
  const all: Record<string, Crumb> = {
    C: crumb("C", "B"),
    B: crumb("B", "A"),
    A: crumb("A", null),
  };

  it("returns the ancestors with the root first", async () => {
    const lookup = vi.fn(async (u: string) => all[u]);
    expect((await ancestorChain("C", lookup)).map((c) => c.ulid)).toEqual(["A", "B", "C"]);
    expect(await ancestorChain(null, lookup)).toEqual([]);
  });

  it("stops at a task it cannot read and at the limit", async () => {
    expect(
      (await ancestorChain("C", async (u) => (u === "A" ? undefined : all[u]))).map((c) => c.ulid),
    ).toEqual(["B", "C"]);
    const loop: Record<string, Crumb> = { X: crumb("X", "X") };
    expect(await ancestorChain("X", async (u) => loop[u], 3)).toHaveLength(3);
  });
});

describe("collapseCrumbs", () => {
  it("keeps three or fewer and collapses the middle of longer chains", () => {
    expect(collapseCrumbs([1, 2, 3])).toEqual([1, 2, 3]);
    expect(collapseCrumbs([1, 2, 3, 4, 5])).toEqual([1, null, 4, 5]);
  });
});
```

`web/app/detail/prop-rows.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { propRows } from "./prop-rows";
import { schemaView } from "./testkit";

describe("propRows", () => {
  it("lists the editable properties of the schema by name, with their editors and values", () => {
    const rows = propRows(schemaView, { due: "2026-11-07", points: 5 });
    expect(rows.map((r) => r.name)).toEqual(["area", "due", "points", "priority", "start"]);
    const due = rows.find((r) => r.name === "due");
    expect(due).toMatchObject({ editor: "date", value: "2026-11-07", label: "due" });
    expect(rows.find((r) => r.name === "points")).toMatchObject({ editor: "number", value: 5 });
    expect(rows.find((r) => r.name === "priority")).toMatchObject({
      editor: "enum",
      options: ["low", "mid", "high"],
      multi: false,
    });
  });

  it("leaves out task references, which the relations section shows", () => {
    expect(propRows(schemaView, {}).map((r) => r.name)).not.toContain("relates");
  });

  it("appends keys the schema does not define, read-only", () => {
    const rows = propRows(schemaView, { team: "web" });
    expect(rows.at(-1)).toMatchObject({ name: "team", editor: null, value: "web" });
  });
});
```

`web/app/detail/watch.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { isWatching } from "./watch";

describe("isWatching", () => {
  it("matches the account as host/login, as the watchers are written", () => {
    expect(isWatching(["sato", "github.com/koki"], "github.com/koki")).toBe(true);
  });

  it("does not match a login alone, or another login that ends the same", () => {
    expect(isWatching(["github.com/koki"], "koki")).toBe(false);
    expect(isWatching(["github.com/nonkoki"], "github.com/koki")).toBe(false);
    expect(isWatching(["sato"], "github.com/koki")).toBe(false);
  });

  it("never matches when the store has no account", () => {
    expect(isWatching(["", "sato"], "")).toBe(false);
  });
});
```

`web/app/detail/mentions.test.ts`:
```ts
import { CompletionContext } from "@codemirror/autocomplete";
import { EditorState } from "@codemirror/state";
import { describe, expect, it, vi } from "vitest";
import type { MentionDeps } from "./mentions";
import { mentionSource, triggerOf } from "./mentions";
import { taskView } from "./testkit";

const noDeps: MentionDeps = { people: vi.fn(async () => []), search: vi.fn(async () => []) };

const at = (doc: string, deps: MentionDeps = noDeps) =>
  mentionSource(
    "shared",
    deps,
  )(new CompletionContext(EditorState.create({ doc }), doc.length, false));

describe("triggerOf", () => {
  it("reads @ and # that start a word", () => {
    expect(triggerOf("@")).toEqual({ sigil: "@", typed: "" });
    expect(triggerOf("了解です。@suz")).toEqual({ sigil: "@", typed: "suz" });
    expect(triggerOf("see #REL-4")).toEqual({ sigil: "#", typed: "REL-4" });
    expect(triggerOf("(#REL")).toEqual({ sigil: "#", typed: "REL" });
  });

  it("ignores a sigil inside a word and text that has ended the word", () => {
    expect(triggerOf("me@example")).toBeNull();
    expect(triggerOf("a#b")).toBeNull();
    expect(triggerOf("@suzuki done")).toBeNull();
    expect(triggerOf("plain")).toBeNull();
  });
});

describe("mentionSource", () => {
  it("offers the people whose names match what follows @", async () => {
    const people = vi.fn(async () => ["suzuki", "suzumura", "tanaka"]);
    const r = await at("確認は @suz", { ...noDeps, people });
    expect(people).toHaveBeenCalledWith("shared");
    expect(r?.from).toBe(5);
    expect(r?.options.map((o) => [o.label, o.apply])).toEqual([
      ["suzuki", "suzuki "],
      ["suzumura", "suzumura "],
    ]);
  });

  it("offers the tasks the search finds for #, with the title as the detail", async () => {
    const search = vi.fn(async () => [taskView({ id: "REL-4", title: "移行ガイドを書く" })]);
    const r = await at("#REL", { ...noDeps, search });
    expect(search).toHaveBeenCalledWith("REL", "shared");
    expect(r?.options[0]).toMatchObject({
      label: "REL-4",
      detail: "移行ガイドを書く",
      apply: "REL-4 ",
    });
  });

  it("asks nothing for a bare # and offers nothing without a trigger", async () => {
    const search = vi.fn(async () => []);
    expect(await at("#", { ...noDeps, search })).toBeNull();
    expect(search).not.toHaveBeenCalled();
    expect(await at("hello")).toBeNull();
  });
});
```

`web/app/detail/ChipsField.test.ts`:
```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import ChipsField from "./ChipsField.svelte";

describe("ChipsField", () => {
  it("adds a value with Enter and on blur, and ignores duplicates", async () => {
    const onchange = vi.fn();
    render(ChipsField, { values: ["koki"], label: "担当者", onchange });
    const input = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(input, { target: { value: "sato" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(onchange).toHaveBeenLastCalledWith(["koki", "sato"]);
    await fireEvent.input(input, { target: { value: "koki" } });
    await fireEvent.blur(input);
    expect(onchange).toHaveBeenCalledTimes(1);
  });

  it("removes a chip with its button and with Backspace on an empty input", async () => {
    const onchange = vi.fn();
    render(ChipsField, { values: ["a", "b"], label: "ラベル", onchange });
    await fireEvent.click(screen.getByRole("button", { name: "b を外す" }));
    expect(onchange).toHaveBeenLastCalledWith(["a"]);
    await fireEvent.keyDown(screen.getByRole("textbox", { name: "ラベル" }), { key: "Backspace" });
    expect(onchange).toHaveBeenLastCalledWith(["a"]);
  });

  it("rejects text that transform refuses and shows the message and the hint", async () => {
    const onchange = vi.fn();
    render(ChipsField, {
      values: [],
      label: "リポジトリ",
      onchange,
      transform: (t: string) => (t.includes("/") ? t.toLowerCase() : null),
      invalidMessage: "読めません",
      hintFor: (t: string) => (t ? `Enter で ${t} として追加します` : null),
    });
    const input = screen.getByRole("textbox", { name: "リポジトリ" });
    await fireEvent.input(input, { target: { value: "nope" } });
    expect(screen.getByText("Enter で nope として追加します")).toBeInTheDocument();
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(screen.getByRole("alert")).toHaveTextContent("読めません");
    expect(onchange).not.toHaveBeenCalled();
    await fireEvent.input(input, { target: { value: "Org/Repo" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(onchange).toHaveBeenCalledWith(["org/repo"]);
  });
});
```

`web/app/detail/FieldInput.test.ts`:
```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import FieldInput from "./FieldInput.svelte";

describe("FieldInput", () => {
  it("text: edits on every input and commits on blur", async () => {
    const onedit = vi.fn();
    const oncommit = vi.fn();
    render(FieldInput, { editor: "text", value: "old", label: "area", onedit, oncommit });
    const input = screen.getByRole("textbox", { name: "area" });
    expect(input).toHaveValue("old");
    await fireEvent.input(input, { target: { value: "new" } });
    expect(onedit).toHaveBeenLastCalledWith("new");
    expect(oncommit).not.toHaveBeenCalled();
    await fireEvent.blur(input);
    expect(oncommit).toHaveBeenCalledTimes(1);
  });

  it("text: an empty value is null for a property and kept for a title", async () => {
    const onedit = vi.fn();
    const { unmount } = render(FieldInput, {
      editor: "text",
      value: "x",
      label: "area",
      onedit,
      oncommit: vi.fn(),
    });
    await fireEvent.input(screen.getByRole("textbox", { name: "area" }), { target: { value: "" } });
    expect(onedit).toHaveBeenLastCalledWith(null);
    unmount();
    render(FieldInput, {
      editor: "text",
      value: "x",
      label: "タイトル",
      nullable: false,
      onedit,
      oncommit: vi.fn(),
    });
    await fireEvent.input(screen.getByRole("textbox", { name: "タイトル" }), {
      target: { value: "" },
    });
    expect(onedit).toHaveBeenLastCalledWith("");
  });

  it("does not overwrite what is typed while a save is pending, and takes the new value afterwards", async () => {
    const { rerender } = render(FieldInput, {
      editor: "text",
      value: "a",
      label: "area",
      pending: false,
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    const input = screen.getByRole("textbox", { name: "area" });
    await fireEvent.input(input, { target: { value: "typed" } });
    await rerender({ value: "from outside", pending: true });
    expect(input).toHaveValue("typed");
    await rerender({ value: "from outside", pending: false });
    expect(input).toHaveValue("from outside");
  });

  it("number: sends a number, or null for an empty input", async () => {
    const onedit = vi.fn();
    render(FieldInput, {
      editor: "number",
      value: 3,
      label: "points",
      onedit,
      oncommit: vi.fn(),
    });
    const input = screen.getByRole("spinbutton", { name: "points" });
    await fireEvent.input(input, { target: { value: "8" } });
    expect(onedit).toHaveBeenLastCalledWith(8);
    await fireEvent.input(input, { target: { value: "" } });
    expect(onedit).toHaveBeenLastCalledWith(null);
  });

  it("date commits right after the edit", async () => {
    const calls: string[] = [];
    const onedit = vi.fn((v: unknown) => calls.push(`edit:${String(v)}`));
    const oncommit = vi.fn(() => calls.push("commit"));
    render(FieldInput, { editor: "date", value: "2026-11-07", label: "due", onedit, oncommit });
    await fireEvent.change(screen.getByLabelText("due"), { target: { value: "2026-11-09" } });
    expect(calls).toEqual(["edit:2026-11-09", "commit"]);
  });

  it("select: shows the current choice, opens a menu and commits the chosen one", async () => {
    const calls: string[] = [];
    const onedit = vi.fn((v: unknown) => calls.push(`edit:${String(v)}`));
    const oncommit = vi.fn(() => calls.push("commit"));
    render(FieldInput, {
      editor: "select",
      value: "todo",
      label: "ステータス",
      options: [
        { value: "todo", label: "未着手" },
        { value: "done", label: "完了" },
      ],
      nullable: false,
      onedit,
      oncommit,
    });
    const button = screen.getByRole("button", { name: "ステータス" });
    expect(button).toHaveTextContent("未着手");
    await fireEvent.click(button);
    expect(screen.getByRole("menu", { name: "ステータス" })).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("menuitem", { name: "完了" }));
    expect(calls).toEqual(["edit:done", "commit"]);
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("select: a nullable select has a choice that clears the value", async () => {
    const onedit = vi.fn();
    render(FieldInput, {
      editor: "select",
      value: "mid",
      label: "priority",
      options: [{ value: "mid", label: "mid" }],
      onedit,
      oncommit: vi.fn(),
    });
    await fireEvent.click(screen.getByRole("button", { name: "priority" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "—" }));
    expect(onedit).toHaveBeenLastCalledWith(null);
  });

  it("select: a disabled choice cannot be taken", async () => {
    const onedit = vi.fn();
    render(FieldInput, {
      editor: "select",
      value: "task",
      label: "種類",
      nullable: false,
      options: [
        { value: "task", label: "タスク" },
        { value: "project", label: "プロジェクト", disabled: true, title: "ルートにだけ置けます" },
      ],
      onedit,
      oncommit: vi.fn(),
    });
    await fireEvent.click(screen.getByRole("button", { name: "種類" }));
    expect(screen.getByRole("menuitem", { name: /^プロジェクト/ })).toBeDisabled();
    expect(screen.getByText("ルートにだけ置けます")).toBeInTheDocument();
  });

  it("readonly shows the value without editing", () => {
    const { unmount } = render(FieldInput, {
      editor: "text",
      value: "fixed",
      label: "area",
      readonly: true,
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    expect(screen.getByRole("textbox", { name: "area" })).toHaveAttribute("readonly");
    unmount();
    render(FieldInput, {
      editor: "select",
      value: "a",
      label: "s",
      options: [{ value: "a", label: "A" }],
      readonly: true,
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    expect(screen.getByRole("button", { name: "s" })).toBeDisabled();
  });
});
```

`web/app/detail/TaskPicker.test.ts`:
```ts
import { fireEvent, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import TaskPicker from "./TaskPicker.svelte";
import { makeCore, mount, ok, taskView } from "./testkit";

describe("TaskPicker", () => {
  it("searches the store by the typed words and picks a result with a click", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation(() =>
      ok({
        tasks: [
          taskView({ ulid: "U1", id: "REL-8", title: "リリース作業" }),
          taskView({ ulid: "U2", id: "REL-9", title: "別の作業" }),
        ],
      }),
    );
    const onpick = vi.fn();
    mount(TaskPicker, { store: "shared", debounceMs: 0, onpick }, k);
    await fireEvent.input(screen.getByRole("combobox", { name: "タスクを検索" }), {
      target: { value: "REL" },
    });
    const option = await screen.findByRole("option", { name: /REL-8/ });
    expect(k.api.GET).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { q: '"REL"', store: "shared" } },
    });
    await fireEvent.click(option);
    expect(onpick).toHaveBeenCalledWith(expect.objectContaining({ ulid: "U1" }));
  });

  it("leaves out excluded tasks and picks the active one with Enter", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation(() =>
      ok({
        tasks: [
          taskView({ ulid: "U1", id: "REL-1", title: "self" }),
          taskView({ ulid: "U2", id: "REL-2", title: "other" }),
        ],
      }),
    );
    const onpick = vi.fn();
    mount(TaskPicker, { store: "shared", exclude: ["U1"], debounceMs: 0, onpick }, k);
    const input = screen.getByRole("combobox", { name: "タスクを検索" });
    await fireEvent.input(input, { target: { value: "REL" } });
    await screen.findByRole("option", { name: /REL-2/ });
    expect(screen.queryByRole("option", { name: /REL-1/ })).toBeNull();
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(onpick).toHaveBeenCalledWith(expect.objectContaining({ ulid: "U2" }));
  });
});
```

`web/app/detail/Checklist.test.ts`:
```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Checklist from "./Checklist.svelte";

const body = "intro\n- [x] 生成コード\n- [ ] 認証ヘッダ\n- [ ] ページング\n";

describe("Checklist", () => {
  it("shows the count and one checkbox per item", () => {
    render(Checklist, { body, ontoggle: vi.fn() });
    expect(screen.getByRole("heading", { name: /チェックリスト/ })).toHaveTextContent("1/3");
    expect(screen.getAllByRole("checkbox")).toHaveLength(3);
    expect(screen.getByRole("checkbox", { name: "生成コード" })).toBeChecked();
  });

  it("reports the line of the item that was toggled", async () => {
    const ontoggle = vi.fn();
    render(Checklist, { body, ontoggle });
    await fireEvent.click(screen.getByRole("checkbox", { name: "ページング" }));
    expect(ontoggle).toHaveBeenCalledWith(3);
  });

  it("renders nothing for a body without checkboxes, and disables them when read-only", () => {
    const { container, unmount } = render(Checklist, { body: "plain", ontoggle: vi.fn() });
    expect(container.querySelector("section")).toBeNull();
    unmount();
    render(Checklist, { body, readonly: true, ontoggle: vi.fn() });
    expect(screen.getByRole("checkbox", { name: "生成コード" })).toBeDisabled();
  });
});
```

`web/app/detail/BodyEditor.test.ts`:
```ts
import { beforeAll, describe, expect, it, vi } from "vitest";
import BodyEditor from "./BodyEditor.svelte";
import { makeCore, mount, stubLayout, viewOf } from "./testkit";

const props = (over: Record<string, unknown> = {}) => ({
  value: "# title",
  store: "shared",
  pending: false,
  onedit: vi.fn(),
  oncommit: vi.fn(),
  ...over,
});

describe("BodyEditor", () => {
  beforeAll(stubLayout);

  it("starts with the value, labelled for assistive technology", () => {
    const { container } = mount(BodyEditor, props(), makeCore());
    expect(viewOf(container).state.doc.toString()).toBe("# title");
    expect(container.querySelector('[aria-label="本文"]')).not.toBeNull();
  });

  it("reports edits made by the user", () => {
    const onedit = vi.fn();
    const { container } = mount(BodyEditor, props({ value: "a", onedit }), makeCore());
    viewOf(container).dispatch({ changes: { from: 1, insert: "b" } });
    expect(onedit).toHaveBeenLastCalledWith("ab");
  });

  it("commits when the editor loses focus", () => {
    const oncommit = vi.fn();
    const { container } = mount(BodyEditor, props({ oncommit }), makeCore());
    viewOf(container).contentDOM.dispatchEvent(new FocusEvent("blur"));
    expect(oncommit).toHaveBeenCalledTimes(1);
  });

  it("takes a change from outside unless a save is pending, without reporting it as an edit", async () => {
    const onedit = vi.fn();
    const { container, rerender } = mount(BodyEditor, props({ value: "one", onedit }), makeCore());
    const view = viewOf(container);
    await rerender({ value: "two", pending: true });
    expect(view.state.doc.toString()).toBe("one");
    await rerender({ value: "two", pending: false });
    expect(view.state.doc.toString()).toBe("two");
    expect(onedit).not.toHaveBeenCalled();
  });

  it("setText and getText read and replace the document without an edit", () => {
    const onedit = vi.fn();
    const { container, component } = mount(
      BodyEditor,
      props({ value: "- [ ] a", onedit }),
      makeCore(),
    );
    const editor = component as unknown as { getText(): string; setText(t: string): void };
    expect(editor.getText()).toBe("- [ ] a");
    editor.setText("- [x] a");
    expect(viewOf(container).state.doc.toString()).toBe("- [x] a");
    expect(onedit).not.toHaveBeenCalled();
  });

  it("does not accept edits when read-only", () => {
    const { container } = mount(BodyEditor, props({ readonly: true }), makeCore());
    expect(viewOf(container).state.readOnly).toBe(true);
  });

  it("offers the people for @ from GET /api/people", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation(() =>
      Promise.resolve({ data: ["suzuki", "tanaka"], response: new Response() }),
    );
    const { container } = mount(BodyEditor, props({ value: "" }), k);
    const view = viewOf(container);
    view.dispatch({
      changes: { from: 0, insert: "@su" },
      selection: { anchor: 3 },
      userEvent: "input.type",
    });
    await vi.waitFor(() =>
      expect(k.api.GET).toHaveBeenCalledWith("/api/people", {
        params: { query: { store: "shared" } },
      }),
    );
  });
});
```

`web/app/detail/PropsPanel.test.ts`:
```ts
import { fireEvent, screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import PropsPanel from "./PropsPanel.svelte";
import { makeCore, mount, showView } from "./testkit";

const show = showView({
  ulid: "U6",
  id: "REL-6",
  store: "shared",
  kind: "task",
  status: "in_progress",
  assignees: ["sato"],
  labels: ["frontend"],
  watchers: ["koki", "sato"],
  props: { priority: "mid", due: "2026-11-07", points: 5, team: "web" },
});

describe("PropsPanel", () => {
  it("shows the built-in fields and the properties of the schema", () => {
    mount(PropsPanel, { show }, makeCore({ show }));
    expect(screen.getByRole("button", { name: "ステータス" })).toHaveTextContent("進行中 · doing");
    expect(screen.getByRole("button", { name: "種類" })).toHaveTextContent("タスク");
    expect(screen.getByRole("button", { name: "priority" })).toHaveTextContent("mid");
    expect(screen.getByLabelText("due")).toHaveValue("2026-11-07");
    expect(screen.getByRole("spinbutton", { name: "points" })).toHaveValue(5);
    expect(screen.getByText("shared")).toBeInTheDocument();
  });

  it("shows a property the schema does not define as read-only, in italics", () => {
    mount(PropsPanel, { show }, makeCore({ show }));
    expect(screen.getByText("team（未定義）")).toHaveClass("undef");
    expect(screen.getByText("web")).toBeInTheDocument();
  });

  it("edits through the SaveController and flushes for discrete choices", async () => {
    const k = makeCore({ show });
    mount(PropsPanel, { show }, k);
    await fireEvent.click(screen.getByRole("button", { name: "ステータス" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "完了 · done" }));
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "status", "done");
    expect(k.save.flush).toHaveBeenCalledWith("U6");
    await fireEvent.change(screen.getByLabelText("due"), { target: { value: "2026-11-09" } });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "props.due", "2026-11-09");
  });

  it("edits a number while typing and commits on blur", async () => {
    const k = makeCore({ show });
    mount(PropsPanel, { show }, k);
    const points = screen.getByRole("spinbutton", { name: "points" });
    await fireEvent.input(points, { target: { value: "8" } });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "props.points", 8);
    expect(k.save.flush).not.toHaveBeenCalled();
    await fireEvent.blur(points);
    expect(k.save.flush).toHaveBeenCalledWith("U6");
  });

  it("adds an assignee and a label", async () => {
    const k = makeCore({ show });
    mount(PropsPanel, { show }, k);
    const assignee = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(assignee, { target: { value: "koki" } });
    await fireEvent.keyDown(assignee, { key: "Enter" });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "assignees", ["sato", "koki"]);
    const label = screen.getByRole("textbox", { name: "ラベル" });
    await fireEvent.input(label, { target: { value: "v2" } });
    await fireEvent.keyDown(label, { key: "Enter" });
    expect(k.save.edit).toHaveBeenCalledWith("U6", "shared", "labels", ["frontend", "v2"]);
  });

  it("does not offer a kind the parent cannot hold", async () => {
    const child = showView({ ...show, parent: "UP" });
    const k = makeCore({ show: child });
    k.cache.task.mockImplementation((u: string) =>
      u === "UP" ? ({ ulid: "UP", kind: "task" } as never) : undefined,
    );
    mount(PropsPanel, { show: child }, k);
    await fireEvent.click(screen.getByRole("button", { name: "種類" }));
    expect(screen.getByRole("menuitem", { name: /^プロジェクト/ })).toBeDisabled();
  });

  it("does not edit when read-only", () => {
    mount(PropsPanel, { show, readonly: true }, makeCore({ show }));
    expect(screen.getByRole("button", { name: "ステータス" })).toBeDisabled();
    expect(screen.getByLabelText("due")).toHaveAttribute("readonly");
  });

  it("lists the watchers", () => {
    mount(PropsPanel, { show }, makeCore({ show }));
    expect(screen.getByRole("list", { name: "ウォッチャー" })).toHaveTextContent("koki");
  });
});
```

`web/app/detail/TaskDetail.test.ts`:
```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeAll, beforeEach, describe, expect, it } from "vitest";
import { resetToasts, toasts } from "../toast.svelte";
import TaskDetail from "./TaskDetail.svelte";
import { fail, makeCore, mount, ok, showView, stubLayout, taskView } from "./testkit";

const U = "01JBZ8K3QW7M2X9V4T6R5N8P0A";
const base = showView({
  ulid: U,
  id: "REL-6",
  store: "shared",
  kind: "task",
  title: "Web の API クライアントを v2 に更新",
  former_ids: ["SH-17"],
  status: "in_progress",
  parent: "UP",
  body: "- [x] a\n- [ ] b\n",
  workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
  watchers: ["sato"],
});
const parent = taskView({
  ulid: "UP",
  id: "REL-1",
  title: "API v2 への移行",
  kind: "project",
  parent: null,
});

function setup(over: Partial<typeof base> = {}, tasks = [parent]) {
  const show = { ...base, ...over };
  const k = makeCore({ show, account: "github.com/koki", tasks });
  return { k, ...mount(TaskDetail, { ulid: U }, k) };
}

describe("TaskDetail", () => {
  beforeAll(stubLayout);
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("shows the kind and ID, the former ID, the ULID and the title", () => {
    setup();
    expect(screen.getByText(/^タスク ·/)).toHaveTextContent("タスク · REL-6");
    expect(screen.getByText(/SH-17/)).toBeInTheDocument();
    expect(screen.getByText(U)).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "タイトル" })).toHaveValue(
      "Web の API クライアントを v2 に更新",
    );
  });

  it("shows the breadcrumb with the store, the workspace, the ancestors and the ID", async () => {
    setup();
    const nav = screen.getByRole("navigation", { name: "パンくず" });
    expect(nav).toHaveTextContent("shared");
    expect(nav).toHaveTextContent("release");
    await waitFor(() => expect(nav).toHaveTextContent("REL-1 API v2 への移行"));
    expect(nav).toHaveTextContent("REL-6");
  });

  it("edits the title in place through the SaveController", async () => {
    const { k } = setup();
    const title = screen.getByRole("textbox", { name: "タイトル" });
    await fireEvent.input(title, { target: { value: "新しい題" } });
    expect(k.save.edit).toHaveBeenCalledWith(U, "shared", "title", "新しい題");
    await fireEvent.blur(title);
    expect(k.save.flush).toHaveBeenCalledWith(U);
  });

  it("shows the save state, and a retry for a failure", async () => {
    const k = makeCore({ show: base, tasks: [parent] });
    k.save.state.mockReturnValue({ kind: "saved", pendingPush: true });
    const { unmount } = mount(TaskDetail, { ulid: U }, k);
    expect(screen.getByRole("status", { name: "保存の状態" })).toHaveTextContent(
      "保存済み · push 待ち",
    );
    unmount();
    k.save.state.mockReturnValue({ kind: "error", message: "保存できませんでした" });
    mount(TaskDetail, { ulid: U }, k);
    await fireEvent.click(screen.getByRole("button", { name: "やり直す" }));
    expect(k.save.flush).toHaveBeenCalledWith(U);
  });

  it("toggles the watch: PUT when not watching, DELETE when watching, and the button follows", async () => {
    const { k } = setup();
    k.api.PUT.mockImplementation(() => ok({ task: taskView({ ulid: U }), commit: "c1" }));
    k.api.DELETE.mockImplementation(() => ok({ task: taskView({ ulid: U }), commit: "c2" }));
    await fireEvent.click(screen.getByRole("button", { name: "ウォッチ" }));
    expect(k.api.PUT).toHaveBeenCalledWith("/api/tasks/{id}/watchers/me", {
      params: { path: { id: U }, query: { store: "shared" } },
    });
    await waitFor(() =>
      expect(k.cache.applyWrite).toHaveBeenCalledWith({ task: expect.anything(), commit: "c1" }),
    );
    await fireEvent.click(await screen.findByRole("button", { name: "ウォッチ中" }));
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}/watchers/me", {
      params: { path: { id: U }, query: { store: "shared" } },
    });
  });

  it("knows it is watching from the watchers, written as host/login", () => {
    setup({ watchers: ["sato", "github.com/koki"] });
    expect(screen.getByRole("button", { name: "ウォッチ中" })).toBeInTheDocument();
  });

  it("shows an error toast when the watch request fails", async () => {
    const { k } = setup();
    k.api.PUT.mockImplementation(() => fail(409, "store.read_only", { store: "shared" }));
    await fireEvent.click(screen.getByRole("button", { name: "ウォッチ" }));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
  });

  it("warns that a blocker is not done yet, and does not when it is done", async () => {
    const blocked = {
      blocked_by: [{ ulid: "UB", id: "REL-2", title: "API の破壊的変更をまとめる" }],
    };
    const open = taskView({
      ulid: "UB",
      id: "REL-2",
      title: "API の破壊的変更をまとめる",
      status: "in_progress",
    });
    const { unmount } = setup(blocked, [parent, open]);
    expect(await screen.findByText(/にブロックされています/)).toHaveTextContent("REL-2");
    unmount();
    setup(blocked, [parent, { ...open, status: "done" }]);
    await waitFor(() => expect(screen.queryByText(/にブロックされています/)).toBeNull());
  });

  it("toggles a checklist item by rewriting the body and flushing at once", async () => {
    const { k } = setup();
    await fireEvent.click(screen.getByRole("checkbox", { name: "b" }));
    expect(k.save.edit).toHaveBeenCalledWith(U, "shared", "body", "- [x] a\n- [x] b\n");
    expect(k.save.flush).toHaveBeenCalledWith(U);
  });

  it("shows a deleted task read-only with its banner", () => {
    setup({ deleted_at: "2026-11-03T10:00:00Z" });
    expect(screen.getByRole("status", { name: "削除済み" })).toHaveTextContent("削除済み");
    expect(screen.getByRole("textbox", { name: "タイトル" })).toHaveAttribute("readonly");
  });

  it("shows a broken file with the hh doctor command and no editors", () => {
    setup({ broken: "yaml: line 3: bad", problems: [] });
    expect(screen.getByText(/hh doctor で確かめてください/)).toBeInTheDocument();
    expect(screen.getByText("$ hh doctor")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "タイトル" })).toBeNull();
  });

  it("shows the load error", async () => {
    const k = makeCore({ show: undefined, loadError: new Error("x") });
    mount(TaskDetail, { ulid: U }, k);
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm vitest run app/detail`
Expected: FAIL（`./checklist-items`・`./lookup`・`./prop-rows`・`./watch`・`./mentions` と各 `.svelte` が存在しない）

- [ ] **Step 3: 実装する**

依存を足す（`codemirror` が内部で使っている版と同じ。Renovate が追う。細部 1 の一覧にも足す）:
```bash
cd web && pnpm add @codemirror/autocomplete@6.20.4
```
`package.json` の `dependencies` に `"@codemirror/autocomplete": "6.20.4"` が exact で入る。

`web/app/detail/field-types.ts`:
```ts
export type Option = { value: string; label: string; disabled?: boolean; title?: string };
```

`web/app/detail/checklist-items.ts`:
```ts
export type CheckItem = { line: number; checked: boolean; text: string };

// A list item that starts with a checkbox: a bullet or a number, a space, [ ], [x] or [X], then a space or the end.
const checkItem = /^[ \t]*(?:[-*+]|[0-9]{1,9}[.)])[ \t]+\[([ xX])\](?:[ \t]|$)(.*)$/;
// Three or more backticks or tildes after at most three spaces open or close a fenced code block.
const fence = /^ {0,3}(`{3,}|~{3,})/;

export function checklistItems(body: string): CheckItem[] {
  const items: CheckItem[] = [];
  let open = "";
  body.split("\n").forEach((raw, i) => {
    const line = raw.endsWith("\r") ? raw.slice(0, -1) : raw;
    const f = fence.exec(line);
    if (f) {
      const mark = f[1] ?? "";
      if (open === "") {
        open = mark;
      } else if (
        mark.charAt(0) === open.charAt(0) &&
        mark.length >= open.length &&
        line.slice(f[0].length).trim() === ""
      ) {
        open = "";
      }
      return;
    }
    if (open !== "") return;
    const m = checkItem.exec(line);
    if (m) items.push({ line: i, checked: m[1] !== " ", text: (m[2] ?? "").trim() });
  });
  return items;
}

// toggleChecklistLine flips the checkbox on the given line (0-based). A line that is not a checkbox item leaves the body as it is.
export function toggleChecklistLine(body: string, line: number): string {
  if (!checklistItems(body).some((it) => it.line === line)) return body;
  const lines = body.split("\n");
  lines[line] = (lines[line] ?? "").replace(/\[([ xX])\]/, (_m, c: string) =>
    c === " " ? "[x]" : "[ ]",
  );
  return lines.join("\n");
}
```

`web/app/detail/lookup.ts`:
```ts
import { type Api, type TaskView, unwrap } from "../../core/api/client";
import type { TaskCache } from "../../core/cache";

export type Crumb = {
  ulid: string;
  id: string;
  title: string;
  kind: string;
  status: string;
  parent: string | null;
  store: string;
};
export type Lookup = (idOrUlid: string) => Promise<Crumb | undefined>;

function crumbOf(
  t: Pick<TaskView, "ulid" | "id" | "title" | "kind" | "status" | "parent" | "store">,
): Crumb {
  return {
    ulid: t.ulid,
    id: t.id,
    title: t.title,
    kind: t.kind,
    status: t.status,
    parent: t.parent,
    store: t.store,
  };
}

// createLookup finds a task by ID or ULID: the cache first, then GET /tasks/{id}. Requests for the same key that
// are in flight at once share one request; nothing is kept afterwards, so a title that changed is read again.
export function createLookup(opts: {
  api: Api;
  cache: Pick<TaskCache, "task">;
  store: string;
}): Lookup {
  const inflight = new Map<string, Promise<Crumb | undefined>>();
  return (key) => {
    const hit = opts.cache.task(key);
    if (hit) return Promise.resolve(crumbOf(hit));
    let p = inflight.get(key);
    if (!p) {
      p = unwrap(
        opts.api.GET("/api/tasks/{id}", {
          params: { path: { id: key }, query: { store: opts.store } },
        }),
      )
        .then(crumbOf, () => undefined)
        .finally(() => inflight.delete(key));
      inflight.set(key, p);
    }
    return p;
  };
}

// ancestorChain walks up from start (a parent ULID) and returns the ancestors, the root first. It stops at a
// task it cannot read and after limit steps (a parent cycle cannot loop forever).
export async function ancestorChain(
  start: string | null,
  lookup: Lookup,
  limit = 16,
): Promise<Crumb[]> {
  const chain: Crumb[] = [];
  let cur = start;
  while (cur && chain.length < limit) {
    const c = await lookup(cur);
    if (!c) break;
    chain.unshift(c);
    cur = c.parent;
  }
  return chain;
}

// collapseCrumbs shortens a long chain for the breadcrumb: the first, an ellipsis (null) and the last two.
export function collapseCrumbs<T>(chain: T[]): (T | null)[] {
  if (chain.length <= 3) return chain;
  return [chain[0] as T, null, ...chain.slice(-2)];
}
```

`web/app/detail/prop-rows.ts`:
```ts
import type { SchemaView } from "../../core/schema";
import { type PropEditor, propEditorOf } from "../../core/schema";

export type PropRow = {
  name: string;
  label: string;
  editor: PropEditor | null;
  multi: boolean;
  options: string[];
  value: unknown;
};

const builtIn = new Set(["status", "kind", "blocked_by"]);

// propRows lists the properties the detail panel edits: those the schema defines (by name; GET /schema gives a map,
// so the order is the name order), then the keys the task has that the schema does not (read-only, editor null).
// Task references are not here: the relations section shows them.
export function propRows(sc: SchemaView, values: Record<string, unknown>): PropRow[] {
  const rows: PropRow[] = [];
  for (const name of Object.keys(sc.props).sort()) {
    if (builtIn.has(name)) continue;
    const editor = propEditorOf(sc, name);
    if (editor === "taskref") continue;
    const def = sc.props[name];
    if (!def) continue;
    rows.push({
      name,
      label: def.label || name,
      editor,
      multi: def.multi === true,
      options: def.values ?? [],
      value: values[name] ?? null,
    });
  }
  for (const name of Object.keys(values)) {
    if (name in sc.props || builtIn.has(name)) continue;
    rows.push({ name, label: name, editor: null, multi: false, options: [], value: values[name] });
  }
  return rows;
}
```

`web/app/detail/watch.ts`:
```ts
/**
 * isWatching tells whether the account is among the watchers of a task. GET /stores gives the
 * account of a store as host/login (the ID of the account that writes it), and the watchers of
 * a task are written the same way, so the two are compared as they are.
 */
export function isWatching(watchers: readonly string[], account: string): boolean {
  return account !== "" && watchers.includes(account);
}
```

`web/app/detail/people.ts`:
```ts
import { PeopleCache } from "../../core/people";
import type { AppCore } from "../context";

const caches = new WeakMap<object, PeopleCache>();

/**
 * The PeopleCache of the app. It is not one of the objects of AppCore, so it is made once per
 * connection, here.
 */
export function peopleOf(c: Pick<AppCore, "api" | "conn">): PeopleCache {
  let p = caches.get(c.conn);
  if (!p) {
    p = new PeopleCache({ api: c.api, conn: c.conn });
    caches.set(c.conn, p);
  }
  return p;
}
```

`web/app/detail/mentions.ts`:
```ts
import {
  autocompletion,
  type CompletionContext,
  type CompletionResult,
  type CompletionSource,
} from "@codemirror/autocomplete";
import type { Extension } from "@codemirror/state";
import type { TaskView } from "../../core/api/client";
import { matchPeople } from "../../core/people";

export type MentionDeps = {
  /** The names @ offers in the store. */
  people: (store: string) => Promise<string[]>;
  /** The tasks whose ID or title has the words, in the store. */
  search: (text: string, store: string) => Promise<TaskView[]>;
};

/**
 * triggerOf reads the text before the cursor: an @ or a # that starts a word, and what was typed
 * after it. An @ or # inside a word (an e-mail address, a#b) triggers nothing.
 */
export function triggerOf(before: string): { sigil: "@" | "#"; typed: string } | null {
  const m = /(?:^|[\s(（、。])([@#])([^\s@#]*)$/.exec(before);
  if (!m) return null;
  return { sigil: m[1] as "@" | "#", typed: m[2] ?? "" };
}

/** The completion source for @ (people) and # (tasks) in a Markdown text. */
export function mentionSource(store: string, deps: MentionDeps): CompletionSource {
  return async (ctx: CompletionContext): Promise<CompletionResult | null> => {
    const line = ctx.state.doc.lineAt(ctx.pos);
    const t = triggerOf(line.text.slice(0, ctx.pos - line.from));
    if (!t) return null;
    const from = ctx.pos - t.typed.length;
    if (t.sigil === "@") {
      const names = matchPeople(await deps.people(store), t.typed);
      if (ctx.aborted) return null;
      return {
        from,
        filter: false,
        options: names.map((n) => ({ label: n, apply: `${n} `, type: "text" })),
      };
    }
    if (t.typed === "") return null;
    const found = await deps.search(t.typed, store);
    if (ctx.aborted) return null;
    return {
      from,
      filter: false,
      options: found.map((x) => ({
        label: x.id,
        detail: x.title,
        apply: `${x.id} `,
        type: "text",
      })),
    };
  };
}

/** The autocompletion extension that the body and the comments share. */
export function mentionExtension(store: string, deps: MentionDeps): Extension {
  return autocompletion({
    override: [mentionSource(store, deps)],
    icons: false,
  });
}
```

`web/app/detail/ChipsField.svelte`:
```svelte
<script lang="ts">
  let {
    values,
    label,
    options = [],
    placeholder = "",
    readonly = false,
    transform,
    invalidMessage = "",
    hintFor,
    onchange,
  }: {
    values: string[];
    label: string;
    options?: string[];
    placeholder?: string;
    readonly?: boolean;
    transform?: (text: string) => string | null;
    invalidMessage?: string;
    hintFor?: (text: string) => string | null;
    onchange: (values: string[]) => void;
  } = $props();

  let text = $state("");
  let error = $state("");
  const listId = `chips-${Math.random().toString(36).slice(2)}`;
  const hint = $derived(text.trim() === "" ? null : (hintFor?.(text.trim()) ?? null));

  function add() {
    const t = text.trim();
    if (t === "") return;
    const v = transform ? transform(t) : t;
    if (v === null) {
      error = invalidMessage;
      return;
    }
    error = "";
    text = "";
    if (!values.includes(v)) onchange([...values, v]);
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      add();
    } else if (e.key === "Backspace" && text === "" && values.length > 0) {
      onchange(values.slice(0, -1));
    }
  }
</script>

<div class="chips" class:readonly>
  {#each values as v (v)}
    <span class="chip">
      {v}
      {#if !readonly}
        <button
          type="button"
          aria-label="{v} を外す"
          onclick={() => onchange(values.filter((x) => x !== v))}
        >
          ×
        </button>
      {/if}
    </span>
  {/each}
  {#if !readonly}
    <input
      type="text"
      aria-label={label}
      {placeholder}
      list={options.length > 0 ? listId : undefined}
      bind:value={text}
      oninput={() => (error = "")}
      {onkeydown}
      onblur={add}
    >
    {#if options.length > 0}
      <datalist id={listId}>
        {#each options as o (o)}
          <option value={o}></option>
        {/each}
      </datalist>
    {/if}
  {/if}
</div>
{#if hint}
  <p class="hint">{hint}</p>
{/if}
{#if error}
  <p class="error" role="alert">{error}</p>
{/if}

<style>
  .chips {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-4);
    min-height: var(--control-h-sm);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    padding: 0 var(--space-6);
    border-radius: var(--radius-4);
    background: var(--sunken);
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .chip button {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    padding: 0;
  }
  input {
    flex: 1;
    min-width: 6em;
    height: var(--control-h-sm);
    border: 0;
    background: transparent;
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  input:focus-visible {
    outline: 2px solid var(--accent);
    border-radius: var(--radius-4);
  }
  .hint {
    margin: var(--space-4) 0 0;
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .error {
    margin: var(--space-4) 0 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
```

`web/app/detail/FieldInput.svelte`:
```svelte
<script lang="ts">
  import { untrack } from "svelte";
  import Icon from "../icons/Icon.svelte";
  import Menu from "../ui/Menu.svelte";
  import ChipsField from "./ChipsField.svelte";
  import type { Option } from "./field-types";

  let {
    editor,
    value,
    label,
    options = [],
    nullable = true,
    pending = false,
    readonly = false,
    size = "normal",
    placeholder = "",
    transform,
    invalidMessage = "",
    hintFor,
    onedit,
    oncommit,
  }: {
    editor: "text" | "number" | "date" | "select" | "chips";
    value: unknown;
    label: string;
    options?: Option[];
    nullable?: boolean;
    pending?: boolean;
    readonly?: boolean;
    size?: "normal" | "title";
    placeholder?: string;
    transform?: (text: string) => string | null;
    invalidMessage?: string;
    hintFor?: (text: string) => string | null;
    onedit: (v: unknown) => void;
    oncommit: () => void;
  } = $props();

  const asText = (v: unknown) => (v === null || v === undefined ? "" : String(v));

  // The text the user is typing. An outside change is taken only when nothing is pending and the input has no focus.
  let local = $state(untrack(() => asText(value)));
  let focused = false;
  $effect(() => {
    const v = asText(value);
    if (!focused && !pending) local = v;
  });

  function oninput(e: Event & { currentTarget: HTMLInputElement }) {
    local = e.currentTarget.value;
    if (editor === "number") {
      if (local.trim() === "") onedit(null);
      else if (!Number.isNaN(Number(local))) onedit(Number(local));
      return;
    }
    onedit(local === "" && nullable ? null : local);
  }

  function ondate(e: Event & { currentTarget: HTMLInputElement }) {
    const v = e.currentTarget.value;
    onedit(v === "" ? null : v);
    oncommit();
  }

  // The menu of a select: the button shows the current choice and opens ui/Menu under itself.
  let trigger = $state<HTMLButtonElement | null>(null);
  let open = $state(false);
  const items = $derived([
    ...(nullable ? [{ value: "", label: "—" }] : []),
    ...options.map((o) => ({
      value: o.value,
      label: o.label,
      disabled: o.disabled,
      hint: o.title,
    })),
  ]);
  const current = $derived(items.find((o) => o.value === asText(value))?.label ?? asText(value));

  function choose(v: string) {
    open = false;
    trigger?.focus();
    onedit(v === "" ? null : v);
    oncommit();
  }

  function onchips(list: string[]) {
    onedit(list);
    oncommit();
  }
</script>

{#if editor === "select"}
  <button
    type="button"
    class="pick"
    bind:this={trigger}
    aria-label={label}
    aria-haspopup="menu"
    aria-expanded={open}
    disabled={readonly}
    onclick={() => (open = !open)}
  >
    <span class="cur">{current || "—"}</span>
    {#if !readonly}
      <Icon name="chevron-down" size={12} />
    {/if}
  </button>
  {#if open}
    <Menu
      anchor={trigger}
      {items}
      {label}
      selected={asText(value)}
      onselect={choose}
      onclose={() => {
        open = false;
        trigger?.focus();
      }}
    />
  {/if}
{:else if editor === "chips"}
  <ChipsField
    values={Array.isArray(value) ? (value as string[]) : value ? [String(value)] : []}
    {label}
    options={options.map((o) => o.value)}
    {placeholder}
    {readonly}
    {transform}
    {invalidMessage}
    {hintFor}
    onchange={onchips}
  />
{:else if editor === "date"}
  <input type="date" aria-label={label} value={asText(value)} {readonly} onchange={ondate}>
{:else}
  <input
    type={editor === "number" ? "number" : "text"}
    class:title={size === "title"}
    aria-label={label}
    {placeholder}
    {readonly}
    value={local}
    {oninput}
    onfocus={() => (focused = true)}
    onblur={() => {
      focused = false;
      oncommit();
    }}
  >
{/if}

<style>
  input,
  .pick {
    width: 100%;
    height: var(--control-h-sm);
    padding: 0 var(--space-8);
    border: 1px solid transparent;
    border-radius: var(--radius-6);
    background: transparent;
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  .pick {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-6);
    text-align: left;
    cursor: pointer;
  }
  .cur {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  input:hover:not([readonly]),
  .pick:hover:not(:disabled) {
    border-color: var(--line-input);
  }
  input:focus-visible,
  .pick:focus-visible {
    outline: 2px solid var(--accent);
    background: var(--surface);
  }
  input[readonly],
  .pick:disabled {
    color: var(--ink-2);
    cursor: default;
  }
  .title {
    height: auto;
    padding: var(--space-4) var(--space-8);
    font-size: var(--fs-19);
    font-weight: 600;
  }
</style>
```

`web/app/detail/TaskPicker.svelte`:
```svelte
<script lang="ts">
  import { onDestroy } from "svelte";
  import type { TaskView } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import { searchTasks } from "../../core/search";
  import { core } from "../context";

  let {
    store,
    exclude = [],
    placeholder = "ID かタイトルで検索",
    label = "タスクを検索",
    debounceMs = 150,
    onpick,
  }: {
    store: string;
    /** ULIDs that are not offered (the task itself, what is already added). */
    exclude?: string[];
    placeholder?: string;
    label?: string;
    debounceMs?: number;
    onpick: (t: TaskView) => void;
  } = $props();

  const { api } = core();
  const listId = $props.id();
  let text = $state("");
  let results = $state<TaskView[]>([]);
  let active = $state(0);
  let error = $state("");
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;
  onDestroy(() => clearTimeout(timer));

  function oninput() {
    clearTimeout(timer);
    const q = text.trim();
    if (q === "") {
      seq++;
      results = [];
      return;
    }
    timer = setTimeout(() => void search(q), debounceMs);
  }

  async function search(q: string) {
    const n = ++seq;
    try {
      const found = await searchTasks(api, q, { store, limit: 20 });
      if (n !== seq) return;
      results = found.filter((t) => !exclude.includes(t.ulid)).slice(0, 8);
      active = 0;
      error = "";
    } catch (e) {
      if (n === seq) error = errorMessage(e);
    }
  }

  function pick(t: TaskView) {
    text = "";
    results = [];
    onpick(t);
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === "ArrowDown" && results.length > 0) {
      e.preventDefault();
      active = (active + 1) % results.length;
    } else if (e.key === "ArrowUp" && results.length > 0) {
      e.preventDefault();
      active = (active - 1 + results.length) % results.length;
    } else if (e.key === "Enter" && results[active]) {
      e.preventDefault();
      pick(results[active] as TaskView);
    } else if (e.key === "Escape" && results.length > 0) {
      e.stopPropagation();
      results = [];
    }
  }
</script>

<div class="picker">
  <input
    type="text"
    role="combobox"
    aria-label={label}
    aria-expanded={results.length > 0}
    aria-controls={listId}
    autocomplete="off"
    {placeholder}
    bind:value={text}
    {oninput}
    {onkeydown}
  >
  {#if results.length > 0}
    <div id={listId} class="list" role="listbox" aria-label={label}>
      {#each results as t, i (t.ulid)}
        <button
          type="button"
          role="option"
          aria-selected={i === active}
          class:active={i === active}
          onclick={() => pick(t)}
        >
          <span class="id">{t.id}</span>
          <span class="title">{t.title}</span>
        </button>
      {/each}
    </div>
  {/if}
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
</div>

<style>
  .picker {
    position: relative;
  }
  input {
    width: 100%;
    height: var(--control-h);
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
  .list {
    position: absolute;
    z-index: 5;
    inset-inline: 0;
    margin-top: var(--space-4);
    padding: var(--space-4);
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
    background: var(--surface);
    box-shadow: var(--shadow-popover);
    display: grid;
  }
  .list button {
    display: flex;
    gap: var(--space-8);
    align-items: baseline;
    padding: var(--space-6) var(--space-8);
    border: 0;
    border-radius: var(--radius-6);
    background: none;
    font: inherit;
    font-size: var(--fs-13);
    text-align: left;
    color: var(--ink);
    cursor: pointer;
  }
  .list button.active {
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .id {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    color: var(--muted);
  }
  .error {
    margin: var(--space-4) 0 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
```

`web/app/detail/Checklist.svelte`:
```svelte
<script lang="ts">
  import { checklistItems } from "./checklist-items";

  let {
    body,
    readonly = false,
    ontoggle,
  }: { body: string; readonly?: boolean; ontoggle: (line: number) => void } = $props();

  const items = $derived(checklistItems(body));
  const done = $derived(items.filter((i) => i.checked).length);
</script>

{#if items.length > 0}
  <section aria-labelledby="checklist-h">
    <h2 id="checklist-h">チェックリスト <span class="count">{done}/{items.length}</span></h2>
    <ul>
      {#each items as it (it.line)}
        <li>
          <label class:done={it.checked}>
            <input
              type="checkbox"
              checked={it.checked}
              disabled={readonly}
              onchange={() => ontoggle(it.line)}
            >
            <span>{it.text}</span>
          </label>
        </li>
      {/each}
    </ul>
    <p class="hint">
      チェックはここで切り替えられます。別の行どうしなら、同時に切り替えても衝突しません。
    </p>
  </section>
{/if}

<style>
  h2 {
    margin: 0 0 var(--space-8);
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .count {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    color: var(--muted);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: var(--space-4);
  }
  label {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  label.done span {
    color: var(--muted);
    text-decoration: line-through;
  }
  .hint {
    margin: var(--space-8) 0 0;
    font-size: var(--fs-12);
    color: var(--muted);
  }
</style>
```

`web/app/detail/BodyEditor.svelte`:
```svelte
<script lang="ts">
  import { markdown } from "@codemirror/lang-markdown";
  import { Compartment, EditorState, Prec, Transaction } from "@codemirror/state";
  import { keymap, placeholder as placeholderExt } from "@codemirror/view";
  import { EditorView, minimalSetup } from "codemirror";
  import { untrack } from "svelte";
  import { searchTasks } from "../../core/search";
  import { core } from "../context";
  import { mentionExtension } from "./mentions";
  import { peopleOf } from "./people";

  let {
    value,
    store,
    label = "本文",
    placeholder = "Markdown で書けます。@ で人、# でタスク",
    minHeight = "8em",
    pending = false,
    readonly = false,
    onedit,
    oncommit,
    onsubmit,
  }: {
    value: string;
    /** The store whose people and tasks @ and # offer. */
    store: string;
    label?: string;
    placeholder?: string;
    minHeight?: string;
    /** A save of this text is not answered yet: a change from outside waits. */
    pending?: boolean;
    readonly?: boolean;
    onedit: (text: string) => void;
    oncommit?: () => void;
    /** ⌘↵ / Ctrl+↵. Without it the key adds a blank line. */
    onsubmit?: () => void;
  } = $props();

  const c = core();
  let host: HTMLDivElement;
  let view: EditorView | undefined;
  let applying = false;
  const access = new Compartment();

  const theme = EditorView.theme({
    "&": { backgroundColor: "var(--surface)", color: "var(--ink)", fontSize: "var(--fs-13)" },
    ".cm-content": {
      fontFamily: "var(--font-sans)",
      padding: "var(--space-8) 0",
      minHeight: "var(--hh-min-height)",
    },
    "&.cm-focused": { outline: "2px solid var(--accent)" },
    ".cm-cursor": { borderLeftColor: "var(--ink)" },
    ".cm-tooltip.cm-tooltip-autocomplete": {
      border: "1px solid var(--line)",
      borderRadius: "var(--radius-8)",
      backgroundColor: "var(--surface)",
      boxShadow: "var(--shadow-popover)",
    },
    ".cm-tooltip-autocomplete ul li[aria-selected]": {
      backgroundColor: "var(--accent-weak)",
      color: "var(--accent-ink)",
    },
    ".cm-completionDetail": { color: "var(--muted)", marginLeft: "var(--space-8)" },
  });

  function replace(text: string) {
    if (!view) return;
    const doc = view.state.doc.toString();
    if (doc === text) return;
    applying = true;
    view.dispatch({
      changes: { from: 0, to: doc.length, insert: text },
      annotations: Transaction.addToHistory.of(false),
    });
    applying = false;
  }

  // A change from outside is taken only when the editor has no focus and no save is pending.
  function sync() {
    if (!view || view.hasFocus || pending) return;
    replace(value);
  }

  export function getText(): string {
    return view?.state.doc.toString() ?? value;
  }

  /** Replaces the text without reporting it as an edit (the caller sends the change itself). */
  export function setText(text: string) {
    replace(text);
  }

  export function focus() {
    view?.focus();
  }

  $effect(() => {
    const v = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: untrack(() => value),
        extensions: [
          Prec.highest(
            keymap.of([
              {
                key: "Mod-Enter",
                run: () => {
                  if (!onsubmit) return false;
                  onsubmit();
                  return true;
                },
              },
            ]),
          ),
          minimalSetup,
          markdown(),
          EditorView.lineWrapping,
          placeholderExt(untrack(() => placeholder)),
          theme,
          EditorView.contentAttributes.of({ "aria-label": untrack(() => label) }),
          mentionExtension(
            untrack(() => store),
            {
              people: (s) => peopleOf(c).load(s),
              search: (text, s) => searchTasks(c.api, text, { store: s }),
            },
          ),
          access.of([]),
          EditorView.updateListener.of((u) => {
            if (u.docChanged && !applying) onedit(u.state.doc.toString());
          }),
          EditorView.domEventHandlers({
            blur: () => {
              oncommit?.();
              queueMicrotask(sync);
              return false;
            },
          }),
        ],
      }),
    });
    view = v;
    return () => {
      v.destroy();
      view = undefined;
    };
  });

  $effect(() => {
    view?.dispatch({
      effects: access.reconfigure([
        EditorView.editable.of(!readonly),
        EditorState.readOnly.of(readonly),
      ]),
    });
  });

  $effect(() => {
    void value;
    void pending;
    sync();
  });
</script>

<div class="editor" style:--hh-min-height={minHeight} bind:this={host}></div>

<style>
  .editor {
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    padding: 0 var(--space-12);
  }
</style>
```

`web/app/detail/PropsPanel.svelte`:
```svelte
<script lang="ts">
  import type { ShowView } from "../../core/api/client";
  import type { PatchField } from "../../core/save";
  import { canParent, parentRule } from "../../core/schema";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import Avatar from "../ui/Avatar.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import FieldInput from "./FieldInput.svelte";
  import type { Option } from "./field-types";
  import { propRows } from "./prop-rows";

  let { show, readonly = false }: { show: ShowView; readonly?: boolean } = $props();

  const { cache, save, schema, stores } = core();
  const sc = useSubscribe(schema, () => schema.get(show.store));
  // Whether a field has a save that the server has not answered; a fresh object on every change of the SaveController.
  const pend = useSubscribe(save, () => ({
    editing: (f: PatchField) => save.editing(show.ulid, f),
  }));
  $effect(() => {
    void schema.load(show.store);
  });

  const parentKind = $derived(show.parent ? (cache.task(show.parent)?.kind ?? null) : null);
  const statusOptions = $derived<Option[]>(
    (sc.current?.statuses ?? []).map((s) => ({
      value: s.name,
      label: `${s.label || s.name} · ${s.category}`,
    })),
  );
  const kindOptions = $derived<Option[]>(
    (sc.current?.kinds ?? []).map((k) => {
      const current = sc.current;
      const usable = k.name === show.kind || !current || canParent(current, k.name, parentKind);
      return {
        value: k.name,
        label: k.label || k.name,
        disabled: !usable,
        title: usable || !current ? undefined : parentRule(current, k.name),
      };
    }),
  );
  const boolOptions: Option[] = [
    { value: "true", label: "はい" },
    { value: "false", label: "いいえ" },
  ];
  const rows = $derived(sc.current ? propRows(sc.current, show.props) : []);
  const showRepos = $derived(
    (show.repos ?? []).length > 0 ||
      sc.current?.kinds?.find((k) => k.name === show.kind)?.role === "container",
  );

  function edit(field: PatchField, value: unknown) {
    save.edit(show.ulid, show.store, field, value);
  }
  const commit = () => void save.flush(show.ulid);
  const text = (v: unknown) => (typeof v === "string" ? v : JSON.stringify(v));
</script>

<dl>
  <dt>Store</dt>
  <dd><StoreBadge store={show.store} tone={stores.tone(show.store)} /></dd>

  <dt>種類</dt>
  <dd>
    <FieldInput
      editor="select"
      label="種類"
      value={show.kind}
      options={kindOptions}
      nullable={false}
      {readonly}
      onedit={(v) => edit("kind", v)}
      oncommit={commit}
    />
  </dd>

  <dt>ステータス</dt>
  <dd>
    <FieldInput
      editor="select"
      label="ステータス"
      value={show.status}
      options={statusOptions}
      nullable={false}
      {readonly}
      onedit={(v) => edit("status", v)}
      oncommit={commit}
    />
  </dd>

  <dt>担当者</dt>
  <dd>
    <FieldInput
      editor="chips"
      label="担当者"
      value={show.assignees ?? []}
      {readonly}
      onedit={(v) => edit("assignees", v)}
      oncommit={commit}
    />
  </dd>

  <dt>ラベル</dt>
  <dd>
    <FieldInput
      editor="chips"
      label="ラベル"
      value={show.labels ?? []}
      {readonly}
      onedit={(v) => edit("labels", v)}
      oncommit={commit}
    />
  </dd>

  {#each rows as r (r.name)}
    {#if r.editor === null}
      <dt class="undef">{r.label}（未定義）</dt>
      <dd class="undef">{text(r.value)}</dd>
    {:else}
      <dt>{r.label}</dt>
      <dd>
        {#if r.editor === "enum" && !r.multi}
          <FieldInput
            editor="select"
            label={r.label}
            value={r.value}
            {readonly}
            options={r.options.map((o) => ({ value: o, label: o }))}
            onedit={(v) => edit(`props.${r.name}`, v)}
            oncommit={commit}
          />
        {:else if r.editor === "enum" ||
          r.editor === "multi_enum" ||
          (r.editor === "user" && r.multi)}
          <FieldInput
            editor="chips"
            label={r.label}
            value={r.value}
            {readonly}
            options={r.options.map((o) => ({ value: o, label: o }))}
            onedit={(v) => edit(`props.${r.name}`, v)}
            oncommit={commit}
          />
        {:else if r.editor === "bool"}
          <FieldInput
            editor="select"
            label={r.label}
            value={r.value}
            {readonly}
            options={boolOptions}
            onedit={(v) => edit(`props.${r.name}`, v === null ? null : v === "true")}
            oncommit={commit}
          />
        {:else if r.editor === "date"}
          <FieldInput
            editor="date"
            label={r.label}
            value={r.value}
            {readonly}
            onedit={(v) => edit(`props.${r.name}`, v)}
            oncommit={commit}
          />
        {:else if r.editor === "number"}
          <FieldInput
            editor="number"
            label={r.label}
            value={r.value}
            {readonly}
            pending={pend.current.editing(`props.${r.name}`)}
            onedit={(v) => edit(`props.${r.name}`, v)}
            oncommit={commit}
          />
        {:else}
          <FieldInput
            editor="text"
            label={r.label}
            value={r.value}
            {readonly}
            pending={pend.current.editing(`props.${r.name}`)}
            onedit={(v) => edit(`props.${r.name}`, v)}
            oncommit={commit}
          />
        {/if}
      </dd>
    {/if}
  {/each}

  {#if showRepos}
    <dt>リポジトリ</dt>
    <dd>
      <FieldInput
        editor="chips"
        label="リポジトリ"
        value={show.repos ?? []}
        {readonly}
        onedit={(v) => edit("repos", v)}
        oncommit={commit}
      />
    </dd>
  {/if}
</dl>

<section class="watchers">
  <h3>ウォッチャー</h3>
  <ul aria-label="ウォッチャー">
    {#each show.watchers ?? [] as w (w)}
      <li><Avatar name={w} /><span>{w}</span></li>
    {/each}
  </ul>
  <p class="hint">作成者・担当者・メンションで自動追加</p>
</section>

<style>
  dl {
    display: grid;
    grid-template-columns: 7.5em minmax(0, 1fr);
    gap: var(--space-6) var(--space-8);
    align-items: center;
    margin: 0;
  }
  dt {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  dd {
    margin: 0;
    min-width: 0;
    font-size: var(--fs-13);
  }
  .undef {
    font-style: italic;
    color: var(--ink-2);
  }
  .watchers {
    margin-top: var(--space-16);
  }
  h3 {
    margin: 0 0 var(--space-6);
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--muted);
  }
  ul {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-6);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    font-size: var(--fs-12);
  }
  .hint {
    margin: var(--space-6) 0 0;
    font-size: var(--fs-12);
    color: var(--muted);
  }
</style>
```

`web/app/detail/TaskDetail.svelte`:
```svelte
<script lang="ts">
  import { untrack } from "svelte";
  import { type ShowView, unwrap } from "../../core/api/client";
  import { cliLine } from "../../core/cli";
  import { formatDateTime } from "../../core/format";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { onLinkClick, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import { copyText } from "../ui/clipboard";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import BodyEditor from "./BodyEditor.svelte";
  import Checklist from "./Checklist.svelte";
  import { toggleChecklistLine } from "./checklist-items";
  import FieldInput from "./FieldInput.svelte";
  import { ancestorChain, type Crumb, collapseCrumbs, createLookup } from "./lookup";
  import PropsPanel from "./PropsPanel.svelte";
  import { isWatching } from "./watch";

  let { ulid }: { ulid: string } = $props();

  const { api, cache, save, schema, stores } = core();

  // Keep the task on screen watched while this route is open.
  let handle = $state<ReturnType<typeof cache.watchDetail> | null>(null);
  $effect(() => {
    const w = cache.watchDetail(ulid);
    handle = w;
    return () => w.dispose();
  });

  const shown = useSubscribe(cache, () => cache.detail(ulid));
  const loadError = useSubscribe(cache, () => handle?.error() ?? null);
  const saveState = useSubscribe(save, () => save.state(ulid));
  const bodyPending = useSubscribe(save, () => save.editing(ulid, "body"));
  const titlePending = useSubscribe(save, () => save.editing(ulid, "title"));
  const sc = useSubscribe(schema, () => {
    const d = cache.detail(ulid);
    return d ? schema.get(d.store) : undefined;
  });
  const s = $derived<ShowView | undefined>(shown.current);
  const locked = $derived(s ? s.deleted_at != null || !!s.broken : false);

  $effect(() => {
    const store = s?.store;
    if (store) void schema.load(store);
  });

  // Ancestors for the breadcrumb, and the blockers that are not done yet.
  let chain = $state<Crumb[]>([]);
  let blockers = $state<Crumb[]>([]);
  $effect(() => {
    const cur = s;
    if (!cur) return;
    const lookup = createLookup({ api, cache, store: cur.store });
    const categories = new Map((sc.current?.statuses ?? []).map((st) => [st.name, st.category]));
    let live = true;
    void ancestorChain(cur.parent, lookup).then((c) => {
      if (live) chain = c;
    });
    void Promise.all((cur.blocked_by ?? []).map((r) => lookup(r.ulid))).then((cs) => {
      if (!live) return;
      blockers = cs.filter((c): c is Crumb => {
        if (!c) return false;
        const cat = categories.get(c.status);
        return cat !== "done" && cat !== "canceled";
      });
    });
    return () => {
      live = false;
    };
  });

  const crumbs = $derived(collapseCrumbs(chain));
  const kindLabel = $derived(
    sc.current?.kinds?.find((k) => k.name === s?.kind)?.label || s?.kind || "",
  );
  const statusLabel = (name: string) =>
    sc.current?.statuses?.find((x) => x.name === name)?.label || name;

  // Watching: the account of the store (host/login) against the watchers; what the user just did stands until the task says the same.
  const account = $derived(stores.stores().find((x) => x.name === s?.store)?.account ?? "");
  const watchingNow = $derived(isWatching(s?.watchers ?? [], account));
  let watchOverride = $state<boolean | null>(null);
  $effect(() => {
    void watchingNow;
    watchOverride = null;
  });
  const watching = $derived(watchOverride ?? watchingNow);

  const saveText = $derived.by(() => {
    const st = saveState.current;
    if (st.kind === "saving") return "保存中…";
    if (st.kind === "saved") return st.pendingPush ? "保存済み · push 待ち" : "保存済み";
    if (st.kind === "error") return st.message;
    return "";
  });

  // The body as the editor has it, for the checklist while a save is pending.
  let liveBody = $state<string | null>(null);
  $effect(() => {
    void s?.body;
    untrack(() => {
      liveBody = null;
    });
  });
  let editor = $state<{ getText(): string; setText(t: string): void } | undefined>();

  function onBodyEdit(text: string) {
    liveBody = text;
    if (s) save.edit(ulid, s.store, "body", text);
  }

  async function toggleCheck(line: number) {
    if (!s) return;
    const cur = editor?.getText() ?? s.body;
    const next = toggleChecklistLine(cur, line);
    if (next === cur) return;
    editor?.setText(next);
    liveBody = next;
    save.edit(ulid, s.store, "body", next);
    await save.flush(ulid);
  }

  async function toggleWatch() {
    if (!s) return;
    const params = { params: { path: { id: s.ulid }, query: { store: s.store } } };
    const next = !watching;
    try {
      const r = await unwrap(
        next
          ? api.PUT("/api/tasks/{id}/watchers/me", params)
          : api.DELETE("/api/tasks/{id}/watchers/me", params),
      );
      cache.applyWrite({ task: r.task, commit: r.commit });
      watchOverride = next;
    } catch (e) {
      toast({ message: errorMessage(e) });
    }
  }

  async function copyUlid() {
    if (!s) return;
    toast({
      message: (await copyText(s.ulid)) ? "ULID をコピーしました" : "コピーできませんでした",
    });
  }
</script>

{#if loadError.current}
  <p class="state" role="alert">{errorMessage(loadError.current)}</p>
{:else if !s}
  <p class="state">読み込み中…</p>
{:else}
  <div class="page">
    <header class="top">
      <nav aria-label="パンくず" class="crumbs">
        <StoreBadge store={s.store} tone={stores.tone(s.store)} />
        <span class="sep">›</span>
        <a
          href={router.href({ name: "workspace", store: s.store, ulid: s.workspace.ulid })}
          onclick={(e) =>
            onLinkClick(
              e,
              router.href({ name: "workspace", store: s.store, ulid: s.workspace.ulid }),
            )}
          >{s.workspace.name}</a
        >
        {#each crumbs as c, i (c ? c.ulid : `gap-${i}`)}
          <span class="sep">›</span>
          {#if c}
            <a
              href={router.href({ name: "task", ulid: c.ulid })}
              onclick={(e) => onLinkClick(e, router.href({ name: "task", ulid: c.ulid }))}
              ><span class="mono">{c.id}</span> {c.title}</a
            >
          {:else}
            <span>…</span>
          {/if}
        {/each}
        <span class="sep">›</span>
        <span class="mono current" aria-current="page">{s.id}</span>
      </nav>
      <div class="actions">
        <span class="save" role="status" aria-label="保存の状態" data-kind={saveState.current.kind}
          >{saveText}</span
        >
        {#if saveState.current.kind === "error"}
          <Button size="sm" onclick={() => void save.flush(ulid)}>やり直す</Button>
        {/if}
        <Button size="sm" onclick={toggleWatch} disabled={s.deleted_at != null}
          >{watching ? "ウォッチ中" : "ウォッチ"}</Button
        >
        <!-- delete-action -->
      </div>
    </header>

    <div class="heading">
      <p class="eyebrow">
        <span>{kindLabel} · <span class="mono">{s.id}</span></span>
        {#if (s.former_ids ?? []).length > 0}
          <span class="muted">旧 <span class="mono">{(s.former_ids ?? []).join("、")}</span></span>
        {/if}
        <span class="muted">·</span>
        <span class="mono ulid">{s.ulid}</span>
        <button type="button" class="copy" aria-label="ULID をコピー" onclick={copyUlid}>
          コピー
        </button>
      </p>
      {#if !s.broken}
        <FieldInput
          editor="text"
          size="title"
          label="タイトル"
          value={s.title}
          nullable={false}
          pending={titlePending.current}
          readonly={locked}
          onedit={(v) => save.edit(ulid, s.store, "title", v)}
          oncommit={() => void save.flush(ulid)}
        />
      {:else}
        <h1 class="broken-title">{s.title || s.id}</h1>
      {/if}
    </div>

    {#if s.deleted_at != null}
      <div class="band deleted" role="status" aria-label="削除済み">
        <span>削除済み（{formatDateTime(s.deleted_at, new Date())}）。編集はできません。</span>
        <!-- deleted-actions -->
      </div>
    {/if}

    {#if blockers.length > 0}
      <div class="band blocked" role="status" aria-label="ブロック">
        <span>
          {#each blockers as b, i (b.ulid)}
            {i > 0 ? "、" : ""}<span class="mono">{b.id}</span>
            {b.title}（{statusLabel(b.status)}）
          {/each}
          にブロックされています。{blockers.length === 1
            ? blockers[0]?.id
            : "すべて"}が完了か中止になれば自動で解除されます。
        </span>
      </div>
    {/if}

    {#if s.broken}
      <div class="band broken" role="alert">
        <p>このタスクのファイルを読めません: {s.broken}</p>
        <p>hh doctor で確かめてください。</p>
        <CliLine command={cliLine(["doctor"])} />
      </div>
    {:else}
      {#if (s.problems ?? []).length > 0}
        <div class="band broken" role="alert">
          <ul>
            {#each s.problems ?? [] as p (p.code + p.field)}
              <li>{p.message}</li>
            {/each}
          </ul>
          <p>hh doctor で確かめてください。</p>
          <CliLine command={cliLine(["doctor"])} />
        </div>
      {/if}

      <div class="columns">
        <article>
          <section aria-labelledby="body-h">
            <div class="sec-head">
              <h2 id="body-h">本文</h2>
              <span class="muted">Markdown</span>
            </div>
            <BodyEditor
              bind:this={editor}
              value={s.body}
              store={s.store}
              pending={bodyPending.current}
              readonly={locked}
              onedit={onBodyEdit}
              oncommit={() => void save.flush(ulid)}
            />
          </section>
          <Checklist body={liveBody ?? s.body} readonly={locked} ontoggle={toggleCheck} />
          <!-- detail-sections -->
        </article>
        <aside aria-label="プロパティ">
          <PropsPanel show={s} readonly={locked} />
        </aside>
      </div>
    {/if}
  </div>
{/if}

<style>
  .page {
    display: grid;
    gap: var(--space-16);
  }
  .state {
    color: var(--muted);
  }
  .top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-12);
    flex-wrap: wrap;
  }
  .crumbs {
    display: flex;
    align-items: center;
    gap: var(--space-6);
    flex-wrap: wrap;
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .crumbs a {
    color: var(--ink-2);
    text-decoration: none;
  }
  .crumbs a:hover {
    text-decoration: underline;
  }
  .current {
    color: var(--ink);
  }
  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-8);
  }
  .save {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .save[data-kind="error"] {
    color: var(--danger);
  }
  .eyebrow {
    display: flex;
    gap: var(--space-8);
    align-items: baseline;
    flex-wrap: wrap;
    margin: 0 0 var(--space-4);
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .mono,
  .ulid {
    font-family: var(--font-mono);
  }
  .copy {
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    font-size: var(--fs-12);
    cursor: pointer;
  }
  .broken-title {
    margin: 0;
    font-size: var(--fs-19);
  }
  .band {
    display: flex;
    gap: var(--space-12);
    align-items: center;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    font-size: var(--fs-13);
  }
  .band.deleted {
    background: var(--sunken);
    color: var(--ink-2);
  }
  .band.blocked {
    background: var(--danger-weak);
    color: var(--danger);
  }
  .band.broken {
    display: block;
    background: var(--warning-weak);
    color: var(--warning);
  }
  .band p {
    margin: 0 0 var(--space-6);
  }
  .columns {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 20rem;
    gap: var(--gutter);
    align-items: start;
  }
  article {
    display: grid;
    gap: var(--space-20);
    min-width: 0;
  }
  aside {
    padding: var(--space-16);
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-12);
    background: var(--surface);
  }
  .sec-head {
    display: flex;
    gap: var(--space-8);
    align-items: baseline;
    margin-bottom: var(--space-8);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .muted {
    color: var(--muted);
  }
  @media (max-width: 960px) {
    .columns {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
```

`web/app/App.svelte` の変更。Task 7 が `<!-- routes -->` に分岐を足す場所を空けてあるので、そこに task の分岐を足す（Task 9 のツリーの分岐も同じ場所に足す）。次の `{#if}` を `<main>` の中の `<!-- routes -->` の直前に置く。`CreateHost`（Task 13）はここでは足さない:
```svelte
<script lang="ts">
  // 既存の import に足す
  import TaskDetail from "./detail/TaskDetail.svelte";
</script>

{#if router.route.name === "task"}
  {#key router.route.ulid}
    <TaskDetail ulid={router.route.ulid} />
  {/key}
{/if}
```
（`{#key}` は別のタスクへ移ったとき、編集中の状態を持ち越さないため。）

- [ ] **Step 4: ツリーの行から詳細を開く操作を足す**

4d-3 のルーターは `/tasks/<ULID>` をルートに持たない（細部 17 の「知らないパスは `/` へ」）。ここで足す。4d-3 のツリー（Task 9）は、開く先の詳細がまだないので、⌘↵・ダブルクリック・フッターの「⌘↵ 詳細を開く」を持たない。詳細ができたので足す。`App.svelte` の task ルート（`TaskDetail`）は Step 3 で足す。

`web/app/tree/keyboard.ts` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```ts
  | { type: "add-below"; after: string; parent: string | null }
```

変更後:
```ts
  | { type: "add-below"; after: string; parent: string | null }
  | { type: "open-detail"; ulid: string }
```

変更前:
```ts
      if (k.mod || k.shift || k.alt) return NONE;
```

変更後:
```ts
      if (k.mod) return { type: "open-detail", ulid: row.ulid };
      if (k.shift || k.alt) return NONE;
```

`web/app/tree/keyboard.test.ts` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```ts
  it("does nothing with the modifier: the detail is another screen", () => {
    expect(keyAction(key("Enter", { mod: true }), ctx("T1"))).toEqual({ type: "none" });
  });
```

変更後:
```ts
  it("opens the detail with the modifier", () => {
    expect(keyAction(key("Enter", { mod: true }), ctx("T1"))).toEqual({
      type: "open-detail",
      ulid: "T1",
    });
  });
```

`web/app/tree/TreeRow.svelte` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```svelte
    ontoggle: () => void;
```

変更後:
```svelte
    ontoggle: () => void;
    onopen: () => void;
```

変更前:
```svelte
    ontoggle,
```

変更後:
```svelte
    ontoggle,
    onopen,
```

変更前:
```svelte
  onclick={onselect}
```

変更後:
```svelte
  onclick={onselect}
  ondblclick={onopen}
```

`web/app/tree/TreeView.svelte` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```svelte
  function toggle(ulid: string, openIt: boolean) {
```

変更後:
```svelte
  function open(ulid: string) {
    router.go(router.href({ name: "task", ulid }));
  }
  function toggle(ulid: string, openIt: boolean) {
```

変更前:
```svelte
        case "toggle":
          toggle(a.ulid, a.open);
          break;
```

変更後:
```svelte
        case "toggle":
          toggle(a.ulid, a.open);
          break;
        case "open-detail":
          open(a.ulid);
          break;
```

変更前:
```svelte
          ontoggle={() => toggle(item.row.ulid, !item.row.expanded)}
```

変更後:
```svelte
          ontoggle={() => toggle(item.row.ulid, !item.row.expanded)}
          onopen={() => open(item.row.ulid)}
```

変更前:
```svelte
    <span><kbd>K</kbd> 種類</span>
```

変更後:
```svelte
    <span><kbd>K</kbd> 種類</span>
    <span><kbd>⌘↵</kbd> 詳細を開く</span>
```

`web/app/tree/TreeView.test.ts` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```ts
  it("selects a row on click", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(screen.getByText("Web のクライアントを更新"));
    expect(selection.ulid).toBe("T3");
  });
```

変更後:
```ts
  it("selects a row on click and opens the detail on double click", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(screen.getByText("Web のクライアントを更新"));
    expect(selection.ulid).toBe("T3");
    await fireEvent.dblClick(screen.getByText("Web のクライアントを更新"));
    expect(h.go).toHaveBeenCalledWith("/tasks/T3");
  });
```

`web/app/router.ts` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```ts
  | { name: "project"; ulid: string };
```

変更後:
```ts
  | { name: "project"; ulid: string }
  | { name: "task"; ulid: string };
```

変更前:
```ts
  if (head === "projects" && seg.length === 2 && a) return { name: "project", ulid: a };
```

変更後:
```ts
  if (head === "projects" && seg.length === 2 && a) return { name: "project", ulid: a };
  if (head === "tasks" && seg.length === 2 && a) return { name: "task", ulid: a };
```

変更前:
```ts
    case "project":
      return `/projects/${encodeURIComponent(r.ulid)}`;
```

変更後:
```ts
    case "project":
      return `/projects/${encodeURIComponent(r.ulid)}`;
    case "task":
      return `/tasks/${encodeURIComponent(r.ulid)}`;
```

`web/app/router.test.ts` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```ts
    ["/tasks/01HTK/", { name: "home" }], // the detail is another screen
```

変更後:
```ts
    ["/tasks/01HTK/", { name: "task", ulid: "01HTK" }],
```

変更前:
```ts
    expect(hrefOf({ name: "home" })).toBe("/");
```

変更後:
```ts
    expect(hrefOf({ name: "home" })).toBe("/");
    expect(hrefOf({ name: "task", ulid: "01HTK" })).toBe("/tasks/01HTK");
```

- [ ] **Step 5: 通ることを確かめる**

Run: `cd web && pnpm fix && pnpm vitest run app/detail app/tree && pnpm check && pnpm lint`
Expected: PASS（vitest 全件、svelte-check 警告 0、biome 指摘 0）。`pnpm fix` は import の並びと整形を直す。

- [ ] **Step 6: コミット**

```bash
git add web/package.json web/pnpm-lock.yaml web/app/detail web/app/tree web/app/router.ts web/app/router.test.ts web/app/App.svelte
git commit -m "feat(web): タスク詳細のプロパティ・本文・チェックリスト・ウォッチ"
```

---

### Task 12: タスク詳細（関連・コメント・履歴・削除と完全削除）

**Files:**
- Create: `web/app/detail/relation-groups.ts`（関連の組み立てと PATCH の本文。純粋）、`history-labels.ts`（op の日本語）、`delete-actions.ts`（削除・復元・完全削除の API 呼び出しと配下の収集）
- Create: `web/app/detail/Relations.svelte`、`Comments.svelte`、`History.svelte`、`DeleteDialogs.svelte`
- Modify: `web/app/detail/TaskDetail.svelte`（Task 11 の差し込み位置3か所と import）
- Test: `web/app/detail/` の同名の `*.test.ts` と `TaskDetail.sections.test.ts`

**Interfaces:**
- Consumes: Task 11 の `testkit.ts`・`createLookup`・`FieldInput`・`TaskPicker`・`BodyEditor`、Task 3〜7 の `core()`・`unwrap`・`ApiError`・`errorMessage`・`cliRm`・`cliRestoreAt`・`cliPurge`・`formatDateTime`・`showAt`・`diffForRestore`・`RestoreChange`（`core/history.ts`）・`toast`・`undoToast`・`router`・`Dialog`・`kbdLabel`、`TaskCache.applyWrite/noteOwnCommit`
- Produces:
  - `relation-groups.ts`: `type RelItem`・`type RelGroup`、`buildGroups(show, sc, lookup): Promise<RelGroup[]>`、`relationKinds(sc)`、`addRefBody(show, sc, kind, ulid)`・`removeRefBody(show, sc, kind, raw)`（`PATCH` の本文）
  - `history-labels.ts`: `opLabel(op: string | null): string`
  - `delete-actions.ts`: `type Target = { ulid; id; title }`、`collectDescendants(api, store, ulid)`、`deleteTask`・`restoreTask`・`probePurge`・`purgeTask`・`purgeTargetsOf(e)`
  - 各コンポーネント: `Relations`・`Comments`・`History` は props `{ show: ShowView; readonly?: boolean }`、`DeleteDialogs` は `{ show: ShowView; variant: "delete" | "deleted" }`

**この Task で決めたこと**
- コメントの入力と編集は `BodyEditor`（`@`・`#` の補完つき）。⌘↵ で送る。コメントは書き込みのあと `show.updated` が変わる（SSE で詳細が取り直される）まで、返ってきた `CommentView` を重ねて表示する
- 「この時点に戻す」: 行を押すと `showAt`（`GET /api/tasks/{id}/history/{commit}`）でその時点のタスクを読み、`diffForRestore` で変わる項目だけを「今 → 戻したあと」の表にして確認する。何も変わらなければ「この時点と同じです」を出して戻すボタンを出さない。そのコミットにタスクがなければ（`restore_at.not_found`）エラーを出す。戻したら応答の `note` をトーストに出す
- 完全削除: ダイアログを開いたときに `DELETE …?purge=true`（`confirm` なし）を送って 409 `purge.unconfirmed` の対象を受け取り、並べる。Store 名を入力するまで送れない。送り直しでも 409 なら新しい対象に差し替えて入力を空に戻す（2026-11-03: main の決定 4ea022c で完全削除を GUI と API から外した。末尾「実装で plan から変えた点」の利用者の決定 7）
- 関連の種類を選ぶメニューは `FieldInput` の `select`（`ui/Menu`）

- [ ] **Step 1: 失敗するテストを書く**

`web/app/detail/relation-groups.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import type { Crumb, Lookup } from "./lookup";
import { addRefBody, buildGroups, relationKinds, removeRefBody } from "./relation-groups";
import { schemaView, showView } from "./testkit";

const crumbs: Record<string, Crumb> = {
  UB: {
    ulid: "UB",
    id: "REL-2",
    title: "破壊的変更",
    kind: "task",
    status: "waiting",
    parent: null,
    store: "shared",
  },
  UR: {
    ulid: "UR",
    id: "REL-7",
    title: "スキーマ再生成",
    kind: "task",
    status: "in_progress",
    parent: null,
    store: "shared",
  },
  UD: {
    ulid: "UD",
    id: "SH-26",
    title: "対応",
    kind: "task",
    status: "canceled",
    parent: null,
    store: "shared",
  },
  UK: {
    ulid: "UK",
    id: "REL-4",
    title: "移行ガイド",
    kind: "task",
    status: "todo",
    parent: null,
    store: "shared",
  },
};
const lookup: Lookup = async (k) => crumbs[k];

const show = showView({
  ulid: "U6",
  id: "REL-6",
  blocked_by: [{ ulid: "UB", id: "REL-2", title: "破壊的変更" }],
  props: { relates: ["UR"], duplicates: "UD" },
  reverse: { duplicates: [{ ulid: "UD", id: "SH-26", title: "対応" }] },
  referenced_by: [{ ulid: "UK", id: "REL-4", title: "移行ガイド" }],
});

describe("buildGroups", () => {
  it("makes a group for blocked_by, each task_ref property, the reverse side and the backlinks", async () => {
    const groups = await buildGroups(show, schemaView, lookup);
    expect(groups.map((g) => g.key)).toEqual([
      "blocked_by",
      "duplicates",
      "relates",
      "reverse:duplicates",
      "referenced_by",
    ]);
    const blocked = groups[0];
    expect(blocked).toMatchObject({
      label: "ブロックされている",
      note: "blocked_by",
      removable: true,
    });
    expect(blocked?.items[0]).toMatchObject({ raw: "UB", id: "REL-2", status: "waiting" });
    expect(groups.find((g) => g.key === "relates")).toMatchObject({
      label: "関連",
      note: "relates（双方向）",
    });
    expect(groups.find((g) => g.key === "reverse:duplicates")).toMatchObject({
      label: "重複されている",
      removable: false,
    });
    expect(groups.at(-1)).toMatchObject({
      label: "バックリンク",
      note: "本文・コメントの #REL-6",
      removable: false,
    });
  });

  it("falls back to the reference itself when the task cannot be read", async () => {
    const groups = await buildGroups(show, schemaView, async () => undefined);
    expect(groups[0]?.items[0]).toMatchObject({ id: "REL-2", title: "破壊的変更", status: null });
    expect(groups.find((g) => g.key === "relates")?.items[0]).toMatchObject({
      raw: "UR",
      id: "UR",
    });
  });

  it("leaves out empty groups", async () => {
    expect(await buildGroups(showView(), schemaView, lookup)).toEqual([]);
  });
});

describe("relationKinds", () => {
  it("offers blocked_by and the task_ref properties", () => {
    expect(relationKinds(schemaView).map((k) => k.key)).toEqual([
      "blocked_by",
      "duplicates",
      "relates",
    ]);
  });
});

describe("addRefBody / removeRefBody", () => {
  it("adds to blocked_by by ULID", () => {
    expect(addRefBody(show, schemaView, "blocked_by", "UX")).toEqual({ blocked_by: ["UB", "UX"] });
    expect(removeRefBody(show, schemaView, "blocked_by", "UB")).toEqual({ blocked_by: [] });
  });
  it("adds to a multiple property as a list and to a single one as a value", () => {
    expect(addRefBody(show, schemaView, "relates", "UX")).toEqual({
      props: { relates: ["UR", "UX"] },
    });
    expect(addRefBody(show, schemaView, "duplicates", "UX")).toEqual({
      props: { duplicates: "UX" },
    });
  });
  it("removes from a list, and clears a single value with null", () => {
    expect(removeRefBody(show, schemaView, "relates", "UR")).toEqual({ props: { relates: [] } });
    expect(removeRefBody(show, schemaView, "duplicates", "UD")).toEqual({
      props: { duplicates: null },
    });
  });
});
```

`web/app/detail/history-labels.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { opLabel } from "./history-labels";

describe("opLabel", () => {
  it("names the operations", () => {
    expect(opLabel("add")).toBe("作成");
    expect(opLabel("set")).toBe("変更");
    expect(opLabel("mv")).toBe("移動");
    expect(opLabel("rm")).toBe("削除");
    expect(opLabel("restore")).toBe("復元");
    expect(opLabel("merge")).toBe("同期でマージ");
  });
  it("passes an unknown op through and names a missing one", () => {
    expect(opLabel("frobnicate")).toBe("frobnicate");
    expect(opLabel(null)).toBe("変更");
  });
});
```

`web/app/detail/delete-actions.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { ApiError } from "../../core/api/client";
import {
  collectDescendants,
  deleteTask,
  probePurge,
  purgeTargetsOf,
  purgeTask,
  restoreTask,
} from "./delete-actions";
import { fail, makeCore, ok, taskView } from "./testkit";

const t = { ulid: "U6", store: "shared" };

describe("collectDescendants", () => {
  it("walks the children level by level", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation((_p: string, o: { params: { query: { q: string } } }) => {
      const q = o.params.query.q;
      if (q === "parent:U6")
        return ok({ tasks: [taskView({ ulid: "A" }), taskView({ ulid: "B" })] });
      if (q === "parent:A") return ok({ tasks: [taskView({ ulid: "C" })] });
      return ok({ tasks: [] });
    });
    const ds = await collectDescendants(k.api as never, "shared", "U6");
    expect(ds.map((d) => d.ulid)).toEqual(["A", "B", "C"]);
  });
});

describe("deleteTask / restoreTask", () => {
  it("sends DELETE and puts the affected tasks into the cache", async () => {
    const k = makeCore();
    k.api.DELETE.mockImplementation(() => ok({ commit: "c1", tasks: [taskView({ ulid: "U6" })] }));
    await deleteTask({ api: k.api as never, cache: k.cache }, t);
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
    });
    expect(k.cache.applyWrite).toHaveBeenCalledWith({
      tasks: [expect.objectContaining({ ulid: "U6" })],
      commit: "c1",
    });
  });
  it("sends POST restore", async () => {
    const k = makeCore();
    k.api.POST.mockImplementation(() => ok({ commit: "c2", tasks: [] }));
    await restoreTask({ api: k.api as never, cache: k.cache }, t);
    expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/restore", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
    });
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ tasks: [], commit: "c2" });
  });
});

describe("purge", () => {
  const targets = [
    { ulid: "U6", id: "REL-6", title: "x" },
    { ulid: "U7", id: "REL-7", title: "y" },
  ];

  it("purgeTargetsOf reads the targets of a purge.unconfirmed problem only", async () => {
    const k = makeCore();
    k.api.DELETE.mockImplementation(() => fail(409, "purge.unconfirmed", { targets }));
    const err = await deleteTask({ api: k.api as never, cache: k.cache }, t).catch(
      (e: unknown) => e,
    );
    expect(err).toBeInstanceOf(ApiError);
    expect(purgeTargetsOf(err)).toEqual(targets);
    expect(purgeTargetsOf(new Error("x"))).toBeNull();
    k.api.DELETE.mockImplementation(() => fail(404, "task.not_found"));
    expect(
      purgeTargetsOf(
        await deleteTask({ api: k.api as never, cache: k.cache }, t).catch((e: unknown) => e),
      ),
    ).toBeNull();
  });

  it("probePurge asks without a confirmation and returns what must be confirmed", async () => {
    const k = makeCore();
    k.api.DELETE.mockImplementation(() => fail(409, "purge.unconfirmed", { targets }));
    expect(await probePurge({ api: k.api as never, cache: k.cache }, t)).toEqual(targets);
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "U6" }, query: { store: "shared", purge: true } },
    });
  });

  it("probePurge rethrows other errors", async () => {
    const k = makeCore();
    k.api.DELETE.mockImplementation(() => fail(404, "task.not_found"));
    await expect(probePurge({ api: k.api as never, cache: k.cache }, t)).rejects.toBeInstanceOf(
      ApiError,
    );
  });

  it("purgeTask sends the confirmed ULIDs and tells the cache which tasks are gone", async () => {
    const k = makeCore();
    k.api.DELETE.mockImplementation(() => ok({ commit: "c3", ulids: ["U6", "U7"] }));
    await purgeTask({ api: k.api as never, cache: k.cache }, t, ["U6", "U7"]);
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "U6" }, query: { store: "shared", purge: true, confirm: "U6,U7" } },
    });
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ ulids: ["U6", "U7"], commit: "c3" });
  });
});
```

`web/app/detail/Relations.test.ts`:
```ts
import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import Relations from "./Relations.svelte";
import { fail, makeCore, mount, ok, showView, taskView } from "./testkit";

const blocker = taskView({ ulid: "UB", id: "REL-2", title: "破壊的変更", status: "waiting" });
const show = showView({
  ulid: "U6",
  id: "REL-6",
  store: "shared",
  blocked_by: [{ ulid: "UB", id: "REL-2", title: "破壊的変更" }],
  props: {},
});

const setup = (readonly = false) => {
  const k = makeCore({ show, tasks: [blocker] });
  return { k, ...mount(Relations, { show, readonly }, k) };
};

describe("Relations", () => {
  it("says that only tasks of the same store can be referred to, and groups the relations", async () => {
    setup();
    expect(screen.getByText("同じ Store（shared）のタスクだけを参照できます")).toBeInTheDocument();
    const group = await screen.findByRole("group", { name: "ブロックされている" });
    expect(within(group).getByText("REL-2")).toBeInTheDocument();
    expect(within(group).getByText("破壊的変更")).toBeInTheDocument();
  });

  it("removes a blocker with a PATCH of the rest", async () => {
    const { k } = setup();
    k.api.PATCH.mockImplementation(() => ok({ task: taskView({ ulid: "U6" }), commit: "c1" }));
    await fireEvent.click(await screen.findByRole("button", { name: "REL-2 を外す" }));
    expect(k.api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
      body: { blocked_by: [] },
    });
    await waitFor(() =>
      expect(k.cache.applyWrite).toHaveBeenCalledWith({ task: expect.anything(), commit: "c1" }),
    );
  });

  it("adds a relation: pick the kind, search, pick a task", async () => {
    const { k } = setup();
    k.api.GET.mockImplementation(() =>
      ok({ tasks: [taskView({ ulid: "UN", id: "REL-9", title: "新しい関連" })] }),
    );
    k.api.PATCH.mockImplementation(() => ok({ task: taskView({ ulid: "U6" }), commit: "c2" }));
    await fireEvent.click(screen.getByRole("button", { name: "関連を追加" }));
    await fireEvent.click(screen.getByRole("button", { name: "関連の種類" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "関連" }));
    await fireEvent.input(screen.getByRole("combobox", { name: "タスクを検索" }), {
      target: { value: "REL-9" },
    });
    await fireEvent.click(await screen.findByRole("option", { name: /REL-9/ }));
    await waitFor(() =>
      expect(k.api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
        params: { path: { id: "U6" }, query: { store: "shared" } },
        body: { props: { relates: ["UN"] } },
      }),
    );
  });

  it("shows the server's error, such as ref.cross_store, in the section", async () => {
    const { k } = setup();
    k.api.GET.mockImplementation(() =>
      ok({ tasks: [taskView({ ulid: "UN", id: "REL-9", title: "x" })] }),
    );
    k.api.PATCH.mockImplementation(() =>
      fail(422, "ref.cross_store", { store: "other", id: "X-1" }),
    );
    await fireEvent.click(screen.getByRole("button", { name: "関連を追加" }));
    await fireEvent.input(screen.getByRole("combobox", { name: "タスクを検索" }), {
      target: { value: "X" },
    });
    await fireEvent.click(await screen.findByRole("option", { name: /REL-9/ }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("hides the add and remove controls when read-only", async () => {
    setup(true);
    await screen.findByRole("group", { name: "ブロックされている" });
    expect(screen.queryByRole("button", { name: "関連を追加" })).toBeNull();
    expect(screen.queryByRole("button", { name: "REL-2 を外す" })).toBeNull();
  });
});
```

`web/app/detail/Comments.test.ts`:
```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { resetToasts, toasts } from "../toast.svelte";
import Comments from "./Comments.svelte";
import { fail, makeCore, mount, ok, showView, stubLayout, typeInto, viewOf } from "./testkit";

const c1 = {
  ulid: "C1",
  author: "tanaka",
  created: "2026-11-04T10:12:00Z",
  body: "ガイドでは「カーソル」で統一します。",
};
const show = showView({
  ulid: "U6",
  id: "REL-6",
  store: "shared",
  comments: [c1],
  updated: "2026-11-04T10:12:00Z",
});

const setup = (readonly = false) => {
  const k = makeCore({ show });
  return { k, ...mount(Comments, { show, readonly }, k) };
};

describe("Comments", () => {
  beforeAll(stubLayout);
  beforeEach(() => resetToasts());

  it("lists the comments with their author", () => {
    setup();
    expect(screen.getByRole("heading", { name: /コメント/ })).toHaveTextContent("1");
    expect(screen.getByText("ガイドでは「カーソル」で統一します。")).toBeInTheDocument();
    expect(screen.getByText("tanaka")).toBeInTheDocument();
  });

  it("posts a comment, shows it at once and tells the cache the commit is its own", async () => {
    const { k, container } = setup();
    k.api.POST.mockImplementation(() =>
      ok({
        comment: { ulid: "C2", author: "koki", created: "2026-11-04T11:00:00Z", body: "了解です" },
        commit: "c1",
      }),
    );
    typeInto(container, "了解です");
    await fireEvent.click(await screen.findByRole("button", { name: "コメント" }));
    expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/comments", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
      body: { body: "了解です" },
    });
    expect(await screen.findByText("了解です")).toBeInTheDocument();
    expect(k.cache.noteOwnCommit).toHaveBeenCalledWith("c1");
    await waitFor(() => expect(viewOf(container).state.doc.toString()).toBe(""));
  });

  it("cannot post an empty comment", () => {
    setup();
    expect(screen.getByRole("button", { name: "コメント" })).toBeDisabled();
  });

  it("offers the people for @ in the comment, from GET /api/people", async () => {
    const { k, container } = setup();
    k.api.GET.mockImplementation(() => ok(["suzuki", "suzumura"]));
    const view = viewOf(container);
    view.dispatch({
      changes: { from: 0, insert: "@suz" },
      selection: { anchor: 4 },
      userEvent: "input.type",
    });
    await vi.waitFor(() =>
      expect(k.api.GET).toHaveBeenCalledWith("/api/people", {
        params: { query: { store: "shared" } },
      }),
    );
  });

  it("edits a comment", async () => {
    const { k, container } = setup();
    k.api.PATCH.mockImplementation(() =>
      ok({ comment: { ...c1, body: "直しました" }, commit: "c2" }),
    );
    await fireEvent.click(screen.getByRole("button", { name: "編集" }));
    typeInto(container, "直しました", 0);
    await fireEvent.click(await screen.findByRole("button", { name: "保存" }));
    expect(k.api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}/comments/{cid}", {
      params: { path: { id: "U6", cid: "C1" }, query: { store: "shared" } },
      body: { body: "直しました" },
    });
    expect(await screen.findByText("直しました")).toBeInTheDocument();
  });

  it("shows comment.not_author under the comment when someone else's comment is edited", async () => {
    const { k } = setup();
    k.api.PATCH.mockImplementation(() => fail(409, "comment.not_author", { author: "tanaka" }));
    await fireEvent.click(screen.getByRole("button", { name: "編集" }));
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("deletes a comment and says so", async () => {
    const { k } = setup();
    k.api.DELETE.mockImplementation(() => ok({ comment: c1, commit: "c3" }));
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}/comments/{cid}", {
      params: { path: { id: "U6", cid: "C1" }, query: { store: "shared" } },
    });
    await waitFor(() =>
      expect(screen.queryByText("ガイドでは「カーソル」で統一します。")).toBeNull(),
    );
    expect(toasts.list[0]?.message).toBe("コメントを削除しました");
  });

  it("has no form when read-only", () => {
    setup(true);
    expect(screen.queryByRole("button", { name: "コメント" })).toBeNull();
    expect(screen.queryByRole("button", { name: "編集" })).toBeNull();
  });
});
```

`web/app/detail/History.test.ts`:
```ts
import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { beforeEach, describe, expect, it } from "vitest";
import { resetToasts, toasts } from "../toast.svelte";
import History from "./History.svelte";
import { fail, makeCore, mount, ok, showView, taskView } from "./testkit";

const log = (hash: string, op: string, date: string, subject: string) => ({
  hash,
  date,
  op,
  subject,
  task: "U6",
  id: "REL-6",
  store: "shared",
  author: { name: "koki", email: "k@example.com" },
});
const logs = [
  log("bbbbbbb1", "set", "2026-11-04T11:42:00Z", "set REL-6"),
  log("aaaaaaa1", "add", "2026-11-03T13:58:00Z", "add REL-6"),
];
const show = showView({
  ulid: "U6",
  id: "REL-6",
  store: "shared",
  title: "今の題",
  status: "in_progress",
  updated: "2026-11-04T11:42:00Z",
});
const then = showView({ ...show, title: "元の題", status: "todo" });

// GET history answers the list, GET history/{commit} the task at that time.
function setup(readonly = false, at = then) {
  const k = makeCore({ show });
  k.api.GET.mockImplementation((path: string) =>
    path === "/api/tasks/{id}/history/{commit}" ? ok(at) : ok(logs),
  );
  return { k, ...mount(History, { show, readonly }, k) };
}

describe("History", () => {
  beforeEach(() => resetToasts());

  it("loads the commits of the task, newest first, with the operation in Japanese", async () => {
    const { k } = setup();
    expect(await screen.findByText(/koki · 変更/)).toBeInTheDocument();
    expect(k.api.GET).toHaveBeenCalledWith("/api/tasks/{id}/history", {
      params: { path: { id: "U6" }, query: { store: "shared", limit: 50 } },
    });
    const rows = screen.getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("Hedgehug-Op: set");
    expect(rows[0]).toHaveTextContent("bbbbbbb");
    expect(rows[1]).toHaveTextContent("作成");
  });

  it("offers 'この時点に戻す' on every commit but the newest", async () => {
    setup();
    await screen.findByText(/koki · 変更/);
    expect(screen.getAllByRole("button", { name: "この時点に戻す" })).toHaveLength(1);
  });

  it("lists what changes, then restores at the commit and shows the note of the response", async () => {
    const { k } = setup();
    k.api.POST.mockImplementation(() =>
      ok({ task: taskView({ ulid: "U6" }), commit: "c9", note: "2 項目を戻しました" }),
    );
    await fireEvent.click(await screen.findByRole("button", { name: "この時点に戻す" }));
    const dialog = await screen.findByRole("dialog");
    expect(k.api.GET).toHaveBeenCalledWith("/api/tasks/{id}/history/{commit}", {
      params: { path: { id: "U6", commit: "aaaaaaa1" }, query: { store: "shared" } },
    });
    const table = await within(dialog).findByRole("table");
    expect(
      within(table).getByRole("row", { name: /タイトル.*今の題.*元の題/ }),
    ).toBeInTheDocument();
    expect(
      within(table).getByRole("row", { name: /ステータス.*in_progress.*todo/ }),
    ).toBeInTheDocument();
    await fireEvent.click(within(dialog).getByRole("button", { name: "2 項目を戻す" }));
    await waitFor(() =>
      expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/history/{commit}/restore", {
        params: { path: { id: "U6", commit: "aaaaaaa1" }, query: { store: "shared" } },
      }),
    );
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ task: expect.anything(), commit: "c9" });
    expect(toasts.list[0]?.message).toBe("2 項目を戻しました");
  });

  it("says so, and offers no restore, when the task is the same as at that time", async () => {
    setup(false, show);
    await fireEvent.click(await screen.findByRole("button", { name: "この時点に戻す" }));
    const dialog = await screen.findByRole("dialog");
    expect(await within(dialog).findByText(/この時点と同じです/)).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: /項目を戻す/ })).toBeNull();
  });

  it("shows the error when the task did not exist at that time", async () => {
    const { k } = setup();
    k.api.GET.mockImplementation((path: string) =>
      path === "/api/tasks/{id}/history/{commit}"
        ? fail(404, "restore_at.not_found", { at: "aaaaaaa1" })
        : ok(logs),
    );
    await fireEvent.click(await screen.findByRole("button", { name: "この時点に戻す" }));
    const dialog = await screen.findByRole("dialog");
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: /項目を戻す/ })).toBeNull();
  });

  it("does not offer the restore for a deleted task", async () => {
    setup(true);
    await screen.findByText(/koki · 変更/);
    expect(screen.queryByRole("button", { name: "この時点に戻す" })).toBeNull();
  });
});
```

`web/app/detail/DeleteDialogs.test.ts`:
```ts
import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { beforeEach, describe, expect, it } from "vitest";
import { resetToasts, toasts, undoToast } from "../toast.svelte";
import DeleteDialogs from "./DeleteDialogs.svelte";
import { fail, makeCore, mount, ok, showView, taskView } from "./testkit";

const show = showView({ ulid: "U6", id: "REL-6", store: "shared", title: "リリース作業" });

describe("DeleteDialogs (variant delete)", () => {
  beforeEach(() => resetToasts());

  it("lists the descendants that go with it and deletes with an undo toast", async () => {
    const k = makeCore({ show });
    k.api.GET.mockImplementation((_p: string, o: { params: { query: { q: string } } }) =>
      o.params.query.q === "parent:U6"
        ? ok({
            tasks: [
              taskView({ ulid: "A", id: "REL-9", title: "負荷試験" }),
              taskView({ ulid: "B", id: "REL-10", title: "502" }),
            ],
          })
        : ok({ tasks: [] }),
    );
    k.api.DELETE.mockImplementation(() => ok({ commit: "c1", tasks: [taskView({ ulid: "U6" })] }));
    k.api.POST.mockImplementation(() => ok({ commit: "c2", tasks: [taskView({ ulid: "U6" })] }));
    mount(DeleteDialogs, { show, variant: "delete" }, k);
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    const dialog = await screen.findByRole("dialog");
    expect(await within(dialog).findByText(/配下の 2 件も一緒に削除します/)).toBeInTheDocument();
    expect(within(dialog).getByText("REL-9")).toBeInTheDocument();
    await fireEvent.click(within(dialog).getByRole("button", { name: "3 件を削除" }));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    const t = toasts.list[0];
    expect(t?.message).toContain("リリース作業");
    await undoToast(t?.id ?? 0);
    expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/restore", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
    });
  });

  it("deletes a task without children with a plain button", async () => {
    const k = makeCore({ show });
    k.api.GET.mockImplementation(() => ok({ tasks: [] }));
    mount(DeleteDialogs, { show, variant: "delete" }, k);
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    const dialog = await screen.findByRole("dialog");
    const del = await within(dialog).findByRole("button", { name: "削除" });
    await waitFor(() => expect(del).toBeEnabled());
  });
});

describe("DeleteDialogs (variant deleted)", () => {
  const gone = showView({ ...show, deleted_at: "2026-11-03T10:00:00Z" });
  const targets = [
    { ulid: "U6", id: "REL-6", title: "リリース作業" },
    { ulid: "U7", id: "REL-7", title: "子" },
  ];

  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/tasks/U6");
  });

  it("restores", async () => {
    const k = makeCore({ show: gone });
    k.api.POST.mockImplementation(() => ok({ commit: "c1", tasks: [taskView({ ulid: "U6" })] }));
    mount(DeleteDialogs, { show: gone, variant: "deleted" }, k);
    await fireEvent.click(screen.getByRole("button", { name: "復元" }));
    await waitFor(() =>
      expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/restore", expect.anything()),
    );
    await waitFor(() => expect(toasts.list[0]?.message).toBe("REL-6 を復元しました"));
  });

  it("purges only after the targets are listed and the store name is typed", async () => {
    const k = makeCore({ show: gone });
    k.api.DELETE.mockImplementationOnce(() =>
      fail(409, "purge.unconfirmed", { targets }),
    ).mockImplementationOnce(() => ok({ commit: "c3", ulids: ["U6", "U7"] }));
    mount(DeleteDialogs, { show: gone, variant: "deleted" }, k);
    await fireEvent.click(screen.getByRole("button", { name: "完全に削除" }));
    const dialog = await screen.findByRole("dialog");
    expect(await within(dialog).findByText("REL-7")).toBeInTheDocument();
    const go = within(dialog).getByRole("button", { name: "完全に削除" });
    expect(go).toBeDisabled();
    await fireEvent.input(within(dialog).getByRole("textbox", { name: "Store 名" }), {
      target: { value: "shared" },
    });
    expect(go).toBeEnabled();
    await fireEvent.click(go);
    await waitFor(() =>
      expect(k.api.DELETE).toHaveBeenLastCalledWith("/api/tasks/{id}", {
        params: { path: { id: "U6" }, query: { store: "shared", purge: true, confirm: "U6,U7" } },
      }),
    );
    await waitFor(() => expect(location.pathname).toBe("/"));
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ ulids: ["U6", "U7"], commit: "c3" });
  });

  it("replaces the targets and clears the input when the confirmation is refused again", async () => {
    const k = makeCore({ show: gone });
    const changed = [...targets, { ulid: "U8", id: "REL-8", title: "増えた子" }];
    k.api.DELETE.mockImplementationOnce(() =>
      fail(409, "purge.unconfirmed", { targets }),
    ).mockImplementationOnce(() => fail(409, "purge.unconfirmed", { targets: changed }));
    mount(DeleteDialogs, { show: gone, variant: "deleted" }, k);
    await fireEvent.click(screen.getByRole("button", { name: "完全に削除" }));
    const dialog = await screen.findByRole("dialog");
    await within(dialog).findByText("REL-7");
    const box = within(dialog).getByRole("textbox", { name: "Store 名" });
    await fireEvent.input(box, { target: { value: "shared" } });
    await fireEvent.click(within(dialog).getByRole("button", { name: "完全に削除" }));
    expect(await within(dialog).findByText("REL-8")).toBeInTheDocument();
    expect(box).toHaveValue("");
    expect(within(dialog).getByRole("alert")).toHaveTextContent("対象が変わりました");
  });
});
```

`web/app/detail/TaskDetail.sections.test.ts`:
```ts
import { screen } from "@testing-library/svelte";
import { beforeAll, describe, expect, it } from "vitest";
import TaskDetail from "./TaskDetail.svelte";
import { makeCore, mount, ok, showView, stubLayout } from "./testkit";

const U = "01JBZ8K3QW7M2X9V4T6R5N8P0A";
const base = showView({ ulid: U, id: "REL-6", store: "shared", title: "t", parent: null });

function setup(over: Partial<typeof base> = {}) {
  const k = makeCore({ show: { ...base, ...over } });
  k.api.GET.mockImplementation(() => ok([]));
  return mount(TaskDetail, { ulid: U }, k);
}

describe("TaskDetail sections", () => {
  beforeAll(stubLayout);

  it("has relations, comments, history and the delete button for a live task", async () => {
    setup();
    expect(await screen.findByRole("heading", { name: "関連" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /コメント/ })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "履歴" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "削除" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "復元" })).toBeNull();
  });

  it("has restore and purge instead of delete for a deleted task", async () => {
    setup({ deleted_at: "2026-11-03T10:00:00Z" });
    expect(await screen.findByRole("button", { name: "復元" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "完全に削除" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "削除" })).toBeNull();
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm vitest run app/detail`
Expected: FAIL（`./relation-groups`・`./history-labels`・`./delete-actions` と `Relations`・`Comments`・`History`・`DeleteDialogs` の `.svelte` が存在しない）

- [ ] **Step 3: 実装する**

`web/app/detail/relation-groups.ts`:
```ts
import type { ShowView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import type { Lookup } from "./lookup";

export type RelItem = {
  raw: string;
  ulid: string;
  id: string;
  title: string;
  status: string | null;
};
export type RelGroup = {
  key: string;
  label: string;
  note: string;
  items: RelItem[];
  removable: boolean;
};
export type RefPatch = { blocked_by?: string[]; props?: Record<string, unknown> };

function asList(v: unknown): string[] {
  if (Array.isArray(v)) return v.filter((x): x is string => typeof x === "string");
  return typeof v === "string" && v !== "" ? [v] : [];
}

// buildGroups lists the relations of a task: blocked_by, each task_ref property, their reverse sides
// (labelled with the property's inverse) and the backlinks. A task that cannot be read keeps the
// reference as it is stored; empty groups are left out.
export async function buildGroups(
  show: ShowView,
  sc: SchemaView,
  lookup: Lookup,
): Promise<RelGroup[]> {
  const item = async (
    raw: string,
    ref?: { ulid: string; id: string; title: string },
  ): Promise<RelItem> => {
    const c = await lookup(ref?.ulid ?? raw);
    return {
      raw,
      ulid: c?.ulid ?? ref?.ulid ?? raw,
      id: c?.id ?? ref?.id ?? raw,
      title: c?.title ?? ref?.title ?? "",
      status: c?.status ?? null,
    };
  };
  const groups: RelGroup[] = [];

  const blockers = show.blocked_by ?? [];
  if (blockers.length > 0) {
    groups.push({
      key: "blocked_by",
      label: "ブロックされている",
      note: "blocked_by",
      removable: true,
      items: await Promise.all(blockers.map((r) => item(r.ulid, r))),
    });
  }
  for (const name of Object.keys(sc.props).sort()) {
    const def = sc.props[name];
    if (def?.type !== "task_ref") continue;
    const raws = asList(show.props[name]);
    if (raws.length === 0) continue;
    groups.push({
      key: name,
      label: def.label || name,
      note: name + (def.symmetric ? "（双方向）" : ""),
      removable: true,
      items: await Promise.all(raws.map((r) => item(r))),
    });
  }
  for (const name of Object.keys(show.reverse ?? {}).sort()) {
    const refs = show.reverse[name] ?? [];
    if (refs.length === 0) continue;
    const def = sc.props[name];
    groups.push({
      key: `reverse:${name}`,
      label: def?.inverse || `${def?.label || name}（逆）`,
      note: `${name} の逆方向`,
      removable: false,
      items: await Promise.all(refs.map((r) => item(r.ulid, r))),
    });
  }
  const back = show.referenced_by ?? [];
  if (back.length > 0) {
    groups.push({
      key: "referenced_by",
      label: "バックリンク",
      note: `本文・コメントの #${show.id}`,
      removable: false,
      items: await Promise.all(back.map((r) => item(r.ulid, r))),
    });
  }
  return groups;
}

// relationKinds is what "関連を追加" can add: blocked_by and the task_ref properties.
export function relationKinds(sc: SchemaView): { key: string; label: string }[] {
  const kinds = [{ key: "blocked_by", label: "ブロックされている" }];
  for (const name of Object.keys(sc.props).sort()) {
    const def = sc.props[name];
    if (def?.type === "task_ref") kinds.push({ key: name, label: def.label || name });
  }
  return kinds;
}

export function addRefBody(show: ShowView, sc: SchemaView, kind: string, ulid: string): RefPatch {
  if (kind === "blocked_by") {
    const cur = (show.blocked_by ?? []).map((r) => r.ulid);
    return { blocked_by: cur.includes(ulid) ? cur : [...cur, ulid] };
  }
  if (sc.props[kind]?.multi) {
    const cur = asList(show.props[kind]);
    return { props: { [kind]: cur.includes(ulid) ? cur : [...cur, ulid] } };
  }
  return { props: { [kind]: ulid } };
}

export function removeRefBody(show: ShowView, sc: SchemaView, kind: string, raw: string): RefPatch {
  if (kind === "blocked_by")
    return { blocked_by: (show.blocked_by ?? []).map((r) => r.ulid).filter((u) => u !== raw) };
  if (sc.props[kind]?.multi)
    return { props: { [kind]: asList(show.props[kind]).filter((v) => v !== raw) } };
  return { props: { [kind]: null } };
}
```

`web/app/detail/history-labels.ts`:
```ts
const labels: Record<string, string> = {
  add: "作成",
  set: "変更",
  edit: "本文の編集",
  mv: "移動",
  rm: "削除",
  restore: "復元",
  purge: "完全削除",
  merge: "同期でマージ",
  repair: "同期で修復",
  comment: "コメント",
  undo: "取り消し",
};

// opLabel names the Hedgehug-Op of a commit; an op it does not know is shown as it is.
export function opLabel(op: string | null): string {
  if (op === null || op === "") return "変更";
  return labels[op] ?? op;
}
```

`web/app/detail/delete-actions.ts`:
```ts
import { type Api, ApiError, type TaskView, unwrap } from "../../core/api/client";
import type { TaskCache } from "../../core/cache";

export type Deps = { api: Api; cache: Pick<TaskCache, "applyWrite"> };
export type Target = { ulid: string; id: string; title: string };
type Ref = { ulid: string; store: string };

const at = (t: Ref) => ({ params: { path: { id: t.ulid }, query: { store: t.store } } });

// collectDescendants lists the live tasks under ulid, level by level, with parent:<ulid> queries.
export async function collectDescendants(
  api: Api,
  store: string,
  ulid: string,
  limit = 500,
): Promise<TaskView[]> {
  const out: TaskView[] = [];
  const queue = [ulid];
  while (queue.length > 0 && out.length < limit) {
    const id = queue.shift() as string;
    const r = await unwrap(
      api.GET("/api/tasks", { params: { query: { store, q: `parent:${id}` } } }),
    );
    for (const t of r.tasks ?? []) {
      out.push(t);
      queue.push(t.ulid);
    }
  }
  return out;
}

export async function deleteTask(d: Deps, t: Ref) {
  const r = await unwrap(d.api.DELETE("/api/tasks/{id}", at(t)));
  d.cache.applyWrite({ tasks: r.tasks ?? [], commit: r.commit });
  return r;
}

export async function restoreTask(d: Deps, t: Ref) {
  const r = await unwrap(d.api.POST("/api/tasks/{id}/restore", at(t)));
  d.cache.applyWrite({ tasks: r.tasks ?? [], commit: r.commit });
  return r;
}

// purgeTargetsOf reads the targets a 409 purge.unconfirmed lists; any other error gives null.
export function purgeTargetsOf(e: unknown): Target[] | null {
  if (!(e instanceof ApiError) || e.type !== "purge.unconfirmed") return null;
  const list = (e.args as { targets?: unknown }).targets;
  if (!Array.isArray(list)) return null;
  return list
    .filter(
      (x): x is Target =>
        typeof x === "object" && x !== null && "ulid" in x && "id" in x && "title" in x,
    )
    .map((x) => ({ ulid: String(x.ulid), id: String(x.id), title: String(x.title) }));
}

// probePurge asks to purge without a confirmation, which the server always refuses with the tasks it
// would remove. Anything else is an error of the caller to show.
export async function probePurge(d: Deps, t: Ref): Promise<Target[]> {
  try {
    await unwrap(
      d.api.DELETE("/api/tasks/{id}", {
        params: { path: { id: t.ulid }, query: { store: t.store, purge: true } },
      }),
    );
  } catch (e) {
    const targets = purgeTargetsOf(e);
    if (targets) return targets;
    throw e;
  }
  throw new Error("the server purged without a confirmation");
}

export async function purgeTask(d: Deps, t: Ref, confirm: string[]) {
  const r = await unwrap(
    d.api.DELETE("/api/tasks/{id}", {
      params: {
        path: { id: t.ulid },
        query: { store: t.store, purge: true, confirm: confirm.join(",") },
      },
    }),
  );
  d.cache.applyWrite({ ulids: r.ulids ?? [], commit: r.commit });
  return r;
}
```

`web/app/detail/Relations.svelte`:
```svelte
<script lang="ts">
  import { type ShowView, type TaskView, unwrap } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { onLinkClick, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import FieldInput from "./FieldInput.svelte";
  import { createLookup } from "./lookup";
  import {
    addRefBody,
    buildGroups,
    type RefPatch,
    type RelGroup,
    type RelItem,
    relationKinds,
    removeRefBody,
  } from "./relation-groups";
  import TaskPicker from "./TaskPicker.svelte";

  let { show, readonly = false }: { show: ShowView; readonly?: boolean } = $props();

  const { api, cache, schema } = core();
  const sc = useSubscribe(schema, () => schema.get(show.store));

  let groups = $state<RelGroup[]>([]);
  $effect(() => {
    const cur = show;
    const def = sc.current;
    if (!def) return;
    const lookup = createLookup({ api, cache, store: cur.store });
    let live = true;
    void buildGroups(cur, def, lookup).then((g) => {
      if (live) groups = g;
    });
    return () => {
      live = false;
    };
  });

  let adding = $state(false);
  let kind = $state("blocked_by");
  let error = $state("");
  const kinds = $derived(sc.current ? relationKinds(sc.current) : []);
  const taken = $derived(groups.find((g) => g.key === kind)?.items.map((i) => i.ulid) ?? []);
  const categoryOf = (status: string | null) =>
    sc.current?.statuses?.find((s) => s.name === status)?.category ?? "todo";

  async function patch(body: RefPatch): Promise<boolean> {
    error = "";
    try {
      const r = await unwrap(
        api.PATCH("/api/tasks/{id}", {
          params: { path: { id: show.ulid }, query: { store: show.store } },
          body,
        }),
      );
      cache.applyWrite({ task: r.task, commit: r.commit });
      return true;
    } catch (e) {
      error = errorMessage(e);
      return false;
    }
  }

  async function add(t: TaskView) {
    if (!sc.current) return;
    if (await patch(addRefBody(show, sc.current, kind, t.ulid))) adding = false;
  }

  function remove(g: RelGroup, item: RelItem) {
    if (sc.current) void patch(removeRefBody(show, sc.current, g.key, item.raw));
  }
</script>

<section aria-labelledby="rel-h">
  <div class="head">
    <h2 id="rel-h">関連</h2>
    <span class="hint">同じ Store（{show.store}）のタスクだけを参照できます</span>
    {#if !readonly}
      <Button size="sm" onclick={() => (adding = !adding)}>関連を追加</Button>
    {/if}
  </div>

  {#if adding && !readonly}
    <div class="add">
      <FieldInput
        editor="select"
        label="関連の種類"
        value={kind}
        options={kinds.map((k) => ({ value: k.key, label: k.label }))}
        nullable={false}
        onedit={(v) => (kind = String(v))}
        oncommit={() => {}}
      />
      <TaskPicker store={show.store} exclude={[show.ulid, ...taken]} onpick={add} />
    </div>
  {/if}
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#each groups as g (g.key)}
    <fieldset class="group" aria-label={g.label}>
      <h3>{g.label} <span class="note">{g.note}</span></h3>
      <ul>
        {#each g.items as it (it.raw)}
          <li>
            <StatusIcon status={it.status ?? "todo"} category={categoryOf(it.status)} />
            <a
              class="id"
              href={router.href({ name: "task", ulid: it.ulid })}
              onclick={(e) => onLinkClick(e, router.href({ name: "task", ulid: it.ulid }))}
              >{it.id}</a
            >
            <span class="title">{it.title}</span>
            {#if g.removable && !readonly}
              <button
                type="button"
                class="remove"
                aria-label="{it.id} を外す"
                onclick={() => remove(g, it)}
              >
                ×
              </button>
            {/if}
          </li>
        {/each}
      </ul>
    </fieldset>
  {:else}
    <p class="empty">関連はありません</p>
  {/each}
</section>

<style>
  .head {
    display: flex;
    gap: var(--space-8);
    align-items: baseline;
    margin-bottom: var(--space-8);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .hint,
  .note,
  .empty {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .head .hint {
    flex: 1;
  }
  .add {
    display: grid;
    grid-template-columns: 12em minmax(0, 1fr);
    gap: var(--space-8);
    margin-bottom: var(--space-8);
  }
  .group {
    margin: var(--space-12) 0 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  h3 {
    margin: 0 0 var(--space-4);
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--ink-2);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    min-height: var(--control-h-sm);
    font-size: var(--fs-13);
  }
  .id {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    color: var(--muted);
    text-decoration: none;
  }
  .title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .remove {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
  }
  .error {
    margin: var(--space-6) 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
```

`web/app/detail/Comments.svelte`:
```svelte
<script lang="ts">
  import { type ShowView, unwrap } from "../../core/api/client";
  import { formatDateTime } from "../../core/format";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { kbdLabel } from "../keys";
  import { toast } from "../toast.svelte";
  import Avatar from "../ui/Avatar.svelte";
  import Button from "../ui/Button.svelte";
  import BodyEditor from "./BodyEditor.svelte";

  type CommentView = NonNullable<ShowView["comments"]>[number];

  let { show, readonly = false }: { show: ShowView; readonly?: boolean } = $props();

  const { api, cache } = core();

  // What was written here, until the task is read again (show.updated changes) and has it.
  let overlay = $state<{
    added: CommentView[];
    changed: Record<string, CommentView>;
    removed: string[];
  }>({ added: [], changed: {}, removed: [] });
  $effect(() => {
    void show.updated;
    overlay = { added: [], changed: {}, removed: [] };
  });
  const list = $derived.by(() => {
    const base = show.comments ?? [];
    return [
      ...base
        .filter((c) => !overlay.removed.includes(c.ulid))
        .map((c) => overlay.changed[c.ulid] ?? c),
      ...overlay.added.filter((a) => !base.some((c) => c.ulid === a.ulid)),
    ];
  });

  let draft = $state("");
  let formError = $state("");
  let writer = $state<{ setText(t: string): void } | undefined>();
  let editingId = $state<string | null>(null);
  let editDraft = $state("");
  let errors = $state<Record<string, string>>({});
  const submitKey = kbdLabel("mod+enter");

  async function post() {
    const body = draft.trim();
    if (body === "") return;
    formError = "";
    try {
      const r = await unwrap(
        api.POST("/api/tasks/{id}/comments", {
          params: { path: { id: show.ulid }, query: { store: show.store } },
          body: { body },
        }),
      );
      cache.noteOwnCommit(r.commit);
      overlay.added = [...overlay.added, r.comment];
      draft = "";
      writer?.setText("");
    } catch (e) {
      formError = errorMessage(e);
    }
  }

  function startEdit(c: CommentView) {
    editingId = c.ulid;
    editDraft = c.body;
    errors = {};
  }

  async function saveEdit(c: CommentView) {
    errors = {};
    try {
      const r = await unwrap(
        api.PATCH("/api/tasks/{id}/comments/{cid}", {
          params: { path: { id: show.ulid, cid: c.ulid }, query: { store: show.store } },
          body: { body: editDraft },
        }),
      );
      cache.noteOwnCommit(r.commit);
      overlay.changed = { ...overlay.changed, [c.ulid]: r.comment };
      editingId = null;
    } catch (e) {
      errors = { ...errors, [c.ulid]: errorMessage(e) };
    }
  }

  async function remove(c: CommentView) {
    errors = {};
    try {
      const r = await unwrap(
        api.DELETE("/api/tasks/{id}/comments/{cid}", {
          params: { path: { id: show.ulid, cid: c.ulid }, query: { store: show.store } },
        }),
      );
      cache.noteOwnCommit(r.commit);
      overlay.removed = [...overlay.removed, c.ulid];
      toast({ message: "コメントを削除しました" });
    } catch (e) {
      errors = { ...errors, [c.ulid]: errorMessage(e) };
    }
  }
</script>

<section aria-labelledby="cm-h">
  <h2 id="cm-h">コメント <span class="count">{list.length}</span></h2>

  <ul>
    {#each list as c (c.ulid)}
      <li>
        <Avatar name={c.author} />
        <div class="main">
          <p class="meta">
            <strong>{c.author}</strong>
            ·
            <span class="mono">{formatDateTime(c.created, new Date())}</span>
          </p>
          {#if editingId === c.ulid}
            <BodyEditor
              value={editDraft}
              store={show.store}
              label="コメントを編集"
              minHeight="4em"
              onedit={(t) => (editDraft = t)}
              onsubmit={() => void saveEdit(c)}
            />
            <div class="row">
              <Button size="sm" variant="primary" onclick={() => void saveEdit(c)}>保存</Button>
              <Button size="sm" onclick={() => (editingId = null)}>キャンセル</Button>
            </div>
          {:else}
            <p class="body">{c.body}</p>
            {#if !readonly}
              <div class="row">
                <Button size="sm" variant="ghost" onclick={() => startEdit(c)}>編集</Button>
                <Button size="sm" variant="ghost" onclick={() => void remove(c)}>削除</Button>
              </div>
            {/if}
          {/if}
          {#if errors[c.ulid]}
            <p class="error" role="alert">{errors[c.ulid]}</p>
          {/if}
        </div>
      </li>
    {/each}
  </ul>

  {#if !readonly}
    <div class="form">
      <BodyEditor
        bind:this={writer}
        value={draft}
        store={show.store}
        label="コメントを書く"
        placeholder="コメントを書く（Markdown · @ で人、# でタスク · {submitKey} で送信）"
        minHeight="4em"
        onedit={(t) => (draft = t)}
        onsubmit={() => void post()}
      />
      <Button variant="primary" disabled={draft.trim() === ""} onclick={() => void post()}
        >コメント</Button
      >
    </div>
    {#if formError}
      <p class="error" role="alert">{formError}</p>
    {/if}
  {/if}
</section>

<style>
  h2 {
    margin: 0 0 var(--space-8);
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .count {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    color: var(--muted);
  }
  ul {
    display: grid;
    gap: var(--space-12);
    margin: 0 0 var(--space-12);
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    gap: var(--space-10);
  }
  .main {
    flex: 1;
    min-width: 0;
  }
  .meta {
    margin: 0;
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .meta strong {
    color: var(--ink);
  }
  .mono {
    font-family: var(--font-mono);
  }
  .body {
    margin: var(--space-4) 0;
    font-size: var(--fs-13);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .row {
    display: flex;
    gap: var(--space-4);
    margin-top: var(--space-4);
  }
  .form {
    display: grid;
    gap: var(--space-8);
    justify-items: end;
  }
  .form > :global(.editor) {
    justify-self: stretch;
  }
  .error {
    margin: var(--space-4) 0 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
```

`web/app/detail/History.svelte`:
```svelte
<script lang="ts">
  import { type Schemas, type ShowView, unwrap } from "../../core/api/client";
  import { cliRestoreAt } from "../../core/cli";
  import { formatDateTime } from "../../core/format";
  import { diffForRestore, type RestoreChange, showAt } from "../../core/history";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import { opLabel } from "./history-labels";

  type Log = Schemas["LogView"];

  let { show, readonly = false }: { show: ShowView; readonly?: boolean } = $props();

  const { api, cache, schema } = core();
  const sc = useSubscribe(schema, () => schema.get(show.store));

  let logs = $state<Log[]>([]);
  let loadError = $state("");
  $effect(() => {
    void show.updated;
    let live = true;
    unwrap(
      api.GET("/api/tasks/{id}/history", {
        params: { path: { id: show.ulid }, query: { store: show.store, limit: 50 } },
      }),
    ).then(
      (r) => {
        if (live) {
          logs = r ?? [];
          loadError = "";
        }
      },
      (e: unknown) => {
        if (live) loadError = errorMessage(e);
      },
    );
    return () => {
      live = false;
    };
  });

  // The commit to go back to, and what that changes: null while the task at that time is being read.
  let target = $state<Log | null>(null);
  let changes = $state<RestoreChange[] | null>(null);
  let busy = $state(false);
  let error = $state("");
  const when = (l: Log) => formatDateTime(l.date, new Date());

  async function ask(l: Log) {
    target = l;
    changes = null;
    error = "";
    try {
      const then = await showAt(api, show.ulid, l.hash, show.store);
      if (target !== l) return;
      changes = diffForRestore(show, then, {
        idOf: (u) => cache.task(u)?.id ?? u,
        propLabel: (n) => sc.current?.props[n]?.label || n,
      });
    } catch (e) {
      if (target === l) {
        error = errorMessage(e);
        changes = [];
      }
    }
  }

  async function restore() {
    if (!target) return;
    busy = true;
    error = "";
    try {
      const r = await unwrap(
        api.POST("/api/tasks/{id}/history/{commit}/restore", {
          params: { path: { id: show.ulid, commit: target.hash }, query: { store: show.store } },
        }),
      );
      cache.applyWrite({ task: r.task, commit: r.commit });
      toast({ message: r.note || `${show.id} を ${when(target)} の状態に戻しました` });
      target = null;
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<section aria-labelledby="hist-h">
  <div class="head">
    <h2 id="hist-h">履歴</h2>
    <span class="hint">git log から · 1 操作 1 コミット</span>
  </div>
  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {/if}
  <ol>
    {#each logs as l, i (l.hash)}
      <li>
        <span class="time">{when(l)}</span>
        <div class="main">
          <p>{l.author.name} · {opLabel(l.op)} <span class="subject">{l.subject}</span></p>
          <p class="trailer">Hedgehug-Op: {l.op ?? "-"} · {l.hash.slice(0, 7)}</p>
        </div>
        {#if i > 0 && !readonly}
          <Button size="sm" variant="ghost" onclick={() => void ask(l)}>この時点に戻す</Button>
        {/if}
      </li>
    {/each}
  </ol>
</section>

{#if target}
  <Dialog
    open
    title="{show.id} を {when(target)} の状態に戻す"
    width={620}
    cli={cliRestoreAt(show.id, target.hash)}
    onclose={() => (target = null)}
  >
    {#if changes === null}
      <p class="muted">この時点との違いを調べています…</p>
    {:else if changes.length === 0 && !error}
      <p>この時点と同じです。戻す内容はありません。</p>
    {:else if changes.length > 0}
      <p class="muted">変わる項目だけを表示しています。戻す操作も 1 コミットとして履歴に残ります</p>
      <table>
        <thead>
          <tr>
            <th>項目</th>
            <th>今</th>
            <th>戻したあと</th>
          </tr>
        </thead>
        <tbody>
          {#each changes as c (c.field)}
            <tr>
              <th scope="row">{c.label}</th>
              <td>{c.from}</td>
              <td>{c.to}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    {#snippet footer()}
      <Button onclick={() => (target = null)}>キャンセル</Button>
      {#if changes && changes.length > 0}
        <Button variant="confirm" disabled={busy} onclick={() => void restore()}
          >{changes.length}
          項目を戻す</Button
        >
      {/if}
    {/snippet}
  </Dialog>
{/if}

<style>
  .head {
    display: flex;
    gap: var(--space-8);
    align-items: baseline;
    margin-bottom: var(--space-8);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .hint,
  .subject,
  .trailer,
  .muted {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  ol {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: var(--space-10);
  }
  li {
    display: flex;
    gap: var(--space-12);
    align-items: flex-start;
  }
  .time {
    flex: none;
    width: 7.5em;
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .main {
    flex: 1;
    min-width: 0;
  }
  p {
    margin: 0;
    font-size: var(--fs-13);
  }
  .trailer {
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }
  table {
    width: 100%;
    margin-top: var(--space-8);
    border-collapse: collapse;
    font-size: var(--fs-13);
  }
  th,
  td {
    padding: var(--space-6) var(--space-8);
    border-bottom: var(--hairline) solid var(--line-soft);
    text-align: left;
    overflow-wrap: anywhere;
  }
  thead th {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .error {
    color: var(--danger);
    font-size: var(--fs-12);
  }
</style>
```

`web/app/detail/DeleteDialogs.svelte`:
```svelte
<script lang="ts">
  import type { ShowView, TaskView } from "../../core/api/client";
  import { cliPurge, cliRm } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { router } from "../router";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import {
    collectDescendants,
    deleteTask,
    probePurge,
    purgeTargetsOf,
    purgeTask,
    restoreTask,
    type Target,
  } from "./delete-actions";

  let { show, variant }: { show: ShowView; variant: "delete" | "deleted" } = $props();

  const { api, cache } = core();
  const deps = { api, cache };
  const ref = $derived({ ulid: show.ulid, store: show.store });

  let open = $state<"delete" | "purge" | null>(null);
  let descendants = $state<TaskView[] | null>(null);
  let targets = $state<Target[] | null>(null);
  let typed = $state("");
  let busy = $state(false);
  let error = $state("");

  async function openDelete() {
    open = "delete";
    error = "";
    descendants = null;
    try {
      descendants = await collectDescendants(api, show.store, show.ulid);
    } catch (e) {
      error = errorMessage(e);
      descendants = [];
    }
  }

  async function openPurge() {
    open = "purge";
    error = "";
    typed = "";
    targets = null;
    try {
      targets = await probePurge(deps, ref);
    } catch (e) {
      error = errorMessage(e);
      targets = [];
    }
  }

  async function confirmDelete() {
    busy = true;
    error = "";
    // Read before the await: the answer of the write changes the cache, and `show` with it.
    const { title } = show;
    const target = ref;
    try {
      await deleteTask(deps, target);
      open = null;
      toast({
        message: `「${title}」を削除しました`,
        undo: async () => {
          await restoreTask(deps, target);
        },
      });
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  async function restore() {
    const { id } = show;
    try {
      await restoreTask(deps, ref);
      toast({ message: `${id} を復元しました` });
    } catch (e) {
      toast({ message: errorMessage(e) });
    }
  }

  async function confirmPurge() {
    if (!targets) return;
    busy = true;
    error = "";
    // Read before the await: once the task is purged the cache forgets it, and `show` is gone.
    const { id } = show;
    try {
      await purgeTask(
        deps,
        ref,
        targets.map((t) => t.ulid),
      );
      open = null;
      toast({ message: `${id} を完全に削除しました` });
      router.go("/");
    } catch (e) {
      const again = purgeTargetsOf(e);
      if (again) {
        targets = again;
        typed = "";
        error = "対象が変わりました。もう一度確認してください";
      } else {
        error = errorMessage(e);
      }
    } finally {
      busy = false;
    }
  }

  const shown = $derived((descendants ?? []).slice(0, 5));
  const more = $derived(Math.max(0, (descendants ?? []).length - 5));
  const deleteLabel = $derived(
    descendants && descendants.length > 0 ? `${descendants.length + 1} 件を削除` : "削除",
  );
</script>

{#if variant === "delete"}
  <Button size="sm" onclick={() => void openDelete()}>削除</Button>
{:else}
  <Button size="sm" onclick={() => void restore()}>復元</Button>
  <Button size="sm" variant="danger" onclick={() => void openPurge()}>完全に削除</Button>
{/if}

{#if open === "delete"}
  <Dialog
    open
    title="{show.id} {show.title} を削除"
    width={560}
    cli={cliRm(show.id)}
    onclose={() => (open = null)}
  >
    {#if descendants === null}
      <p class="muted">配下を調べています…</p>
    {:else if descendants.length > 0}
      <p>
        配下の {descendants.length} 件も一緒に削除します。ファイルは残るので、あとから復元できます
      </p>
      <ul>
        {#each shown as t (t.ulid)}
          <li><span class="mono">{t.id}</span> {t.title}</li>
        {/each}
        {#if more > 0}
          <li class="muted">ほか {more} 件</li>
        {/if}
      </ul>
    {:else}
      <p>ファイルは残るので、あとから復元できます。</p>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    {#snippet footer()}
      <Button onclick={() => (open = null)}>キャンセル</Button>
      <Button
        variant="danger"
        disabled={busy || descendants === null}
        onclick={() => void confirmDelete()}
        >{deleteLabel}</Button
      >
    {/snippet}
  </Dialog>
{:else if open === "purge"}
  <Dialog
    open
    title="{show.id} を完全に削除"
    width={560}
    cli={cliPurge(show.id)}
    onclose={() => (open = null)}
  >
    <p>
      タスクファイルとコメントを消します。git の履歴には残りますが、Hedgehug
      からは復元できません。番号 {show.id}
      は再利用しません
    </p>
    {#if targets === null}
      <p class="muted">対象を調べています…</p>
    {:else}
      <ul>
        {#each targets as t (t.ulid)}
          <li>
            <span class="mono">{t.id}</span> {t.title} <span class="mono muted">{t.ulid}</span>
          </li>
        {/each}
      </ul>
      <label class="confirm">
        <span
          >履歴を書き換える操作です。確認のため Store 名
          <strong>{show.store}</strong>
          を入力してください</span
        >
        <input type="text" aria-label="Store 名" bind:value={typed} autocomplete="off">
      </label>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    {#snippet footer()}
      <Button onclick={() => (open = null)}>キャンセル</Button>
      <Button
        variant="danger"
        disabled={busy || targets === null || targets.length === 0 || typed !== show.store}
        onclick={() => void confirmPurge()}
        >完全に削除</Button
      >
    {/snippet}
  </Dialog>
{/if}

<style>
  p {
    margin: 0 0 var(--space-8);
    font-size: var(--fs-13);
    color: var(--ink-2);
  }
  ul {
    margin: 0 0 var(--space-12);
    padding: 0;
    list-style: none;
    font-size: var(--fs-13);
  }
  li {
    padding: var(--space-4) 0;
    overflow-wrap: anywhere;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .muted {
    color: var(--muted);
  }
  .confirm {
    display: grid;
    gap: var(--space-6);
    font-size: var(--fs-13);
  }
  input {
    height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font: inherit;
    font-family: var(--font-mono);
    color: var(--ink);
  }
  .error {
    color: var(--danger);
    font-size: var(--fs-12);
  }
</style>
```

`web/app/detail/TaskDetail.svelte` の変更（Edit で4か所。import の並びは `pnpm fix` が整える）:

1. import を足す。old: `  import Checklist from "./Checklist.svelte";` → new:
```svelte
  import Checklist from "./Checklist.svelte";
  import Comments from "./Comments.svelte";
  import DeleteDialogs from "./DeleteDialogs.svelte";
  import History from "./History.svelte";
  import Relations from "./Relations.svelte";
```
2. old: `        <!-- delete-action -->` → new:
```svelte
        {#if s.deleted_at == null && !s.broken}
          <DeleteDialogs show={s} variant="delete" />
        {/if}
```
3. old: `        <!-- deleted-actions -->` → new:
```svelte
        <DeleteDialogs show={s} variant="deleted" />
```
4. old: `          <!-- detail-sections -->` → new:
```svelte
          <Relations show={s} readonly={locked} />
          <Comments show={s} readonly={locked} />
          <History show={s} readonly={locked} />
```

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm fix && pnpm vitest run app/detail && pnpm check && pnpm lint`
Expected: PASS（Task 11 のテストも含めて全件、svelte-check 警告 0、biome 指摘 0）

- [ ] **Step 5: コミット**

```bash
git add web/app/detail
git commit -m "feat(web): タスク詳細の関連・コメント・履歴・削除と完全削除"
```

---

### Task 13: 作成ダイアログ（タスク・プロジェクト）

**Files:**
- Create: `web/app/create/repos.ts`（リポジトリの URL を `host/owner/repo` に揃える）、`context.ts`（作成の親とワークスペースを今いる画面から決める）
- Create: `web/app/create/WorkspaceChoice.svelte`、`CreateTask.svelte`、`CreateProject.svelte`、`CreateHost.svelte`
- Create: `web/app/create-state.svelte.ts`
- Modify: `web/app/App.svelte`（`<CreateHost />` を足す）
- Modify: `web/app/tree/TreeView.svelte`、`TreeView.test.ts`（空の状態の「タスクを作成」）、`web/app/sidebar/Sidebar.svelte`、`Sidebar.test.ts`（「プロジェクトを追加」）
- Test: `web/app/create/` の同名の `*.test.ts`

**Interfaces:**
- Consumes: Task 11 の `FieldInput`・`ChipsField`・`TaskPicker`・`BodyEditor`・`propRows`・`testkit.ts`、`core()`（`api`・`cache`・`stores`・`schema`）、`useSubscribe`、`router`、`selection`（`app/selection.svelte.ts`）、`toast`、`registerShortcut`・`kbdLabel`・`modKey`、`Dialog`・`Button`・`StoreBadge`・`KindIcon`、`canParent`・`parentRule`・`defaultStatus`、`cliAdd`、`errorMessage`、`unwrap`、`WorkspaceEntry`・`StoreDirectory.workspaces()`、`app/create-state.svelte.ts`（`createState.request`・`openCreate`・`closeCreate`。この Task が置く）
- Task 1 の成果: `POST /tasks` の本文に `body`・`repos` があり、`schema.d.ts` に入っていること
- Produces:
  - `repos.ts`: `repoIdOf(input: string): string | null`（`internal/context.RepoID` と同じ。ホストと `owner/repo` を小文字、`.git`・末尾の `/`・ポートを除く。ローカルのパスや owner/repo のない URL は null。すでに揃った `host/owner/repo` も受ける）
  - `context.ts`: `resolveCreateContext(i): { parent: TaskView | null; workspace: WorkspaceEntry | null }`、`workspaceKey(w): string`
  - `CreateHost.svelte`: props なし。App が1つ置く。C を登録し、`createState.request` に合わせて `CreateTask`・`CreateProject` を出す

**この Task で決めたこと**
- 本文は `BodyEditor`（CodeMirror）。`@`・`#` の補完が詳細と同じに効く。⌘↵ は `BodyEditor` が受けて作成する（ウィンドウ側の ⌘↵ は `defaultPrevented` なら何もしないので二重に作らない）
- リポジトリの正規化は Web 側（`repos.ts`）。サーバーは正規化しない（`internal/ops.AddInput.Repos` は repo ID をそのまま持ち、`internal/context.RepoID` を使うのは add の既定・store init・discover だけ）
- 担当者の既定は空（細部 41）。スキーマの props に既定値の欄はないので、props の既定も空
- C は `registerShortcut("c")`。入力欄の中と、ダイアログが開いている間は `registerShortcut` が効かせない
- ワークスペースの選択欄は radio の並び（Store のバッジを付けるため）。親があれば親のワークスペースに固定する
- 親の検索は選んでいるワークスペースの Store の中だけ（参照は Store の中だけ）。親は「×」で外せて、外すとワークスペースの選択欄が戻る

- [ ] **Step 1: 失敗するテストを書く**

`web/app/create/repos.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { repoIdOf } from "./repos";

describe("repoIdOf", () => {
  it("reads https, ssh, git and scp-like URLs the way internal/context.RepoID does", () => {
    expect(repoIdOf("https://github.com/Org-A/Web.git")).toBe("github.com/org-a/web");
    expect(repoIdOf("git@github.com:Org-A/web.git")).toBe("github.com/org-a/web");
    expect(repoIdOf("ssh://git@github.com:22/org-a/web")).toBe("github.com/org-a/web");
    expect(repoIdOf("git://GitHub.com/org-a/web/")).toBe("github.com/org-a/web");
    expect(repoIdOf("http://github.com/org-a/web")).toBe("github.com/org-a/web");
  });

  it("accepts an ID that is already in the form host/owner/repo", () => {
    expect(repoIdOf("github.com/org-a/web")).toBe("github.com/org-a/web");
    expect(repoIdOf("GitHub.com/Org-A/Web.git")).toBe("github.com/org-a/web");
  });

  it("refuses what is not a remote with an owner and a repo", () => {
    expect(repoIdOf("")).toBeNull();
    expect(repoIdOf("/home/me/repo")).toBeNull();
    expect(repoIdOf("file:///x/y")).toBeNull();
    expect(repoIdOf("https://github.com/org-a")).toBeNull();
    expect(repoIdOf("https://github.com/a/b/c")).toBeNull();
    expect(repoIdOf("web")).toBeNull();
  });

  it("ignores surrounding white space", () => {
    expect(repoIdOf("  https://github.com/org-a/web  ")).toBe("github.com/org-a/web");
  });
});
```

`web/app/create/context.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { taskView, workspaces } from "../detail/testkit";
import { resolveCreateContext, workspaceKey } from "./context";

const tasks = [
  taskView({ ulid: "UE", id: "REL-8", title: "リリース作業", kind: "epic" }),
  taskView({ ulid: "UP", id: "REL-1", title: "リリース準備", kind: "project" }),
  taskView({ ulid: "UT", id: "REL-6", title: "詳細で開いている", kind: "task" }),
  taskView({
    ulid: "UD",
    id: "REL-7",
    title: "削除済み",
    kind: "task",
    deleted_at: "2026-11-01T00:00:00Z",
  }),
  taskView({
    ulid: "US",
    id: "SH-3",
    title: "別のワークスペース",
    workspace: { ulid: "W2", name: "shared", id_prefix: "SH" },
  }),
];
const task = (u: string) => tasks.find((t) => t.ulid === u);
const base = { selection: null, route: { name: "home" } as never, task, workspaces };

describe("resolveCreateContext", () => {
  it("uses the selected row of the tree as the parent", () => {
    const c = resolveCreateContext({
      ...base,
      selection: "UE",
      route: { name: "project", ulid: "UP" },
    });
    expect(c.parent?.id).toBe("REL-8");
    expect(c.workspace?.name).toBe("release");
  });

  it("uses the project when nothing is selected", () => {
    const c = resolveCreateContext({ ...base, route: { name: "project", ulid: "UP" } });
    expect(c.parent?.id).toBe("REL-1");
  });

  it("uses the task on a task detail", () => {
    const c = resolveCreateContext({
      ...base,
      selection: "UE",
      route: { name: "task", ulid: "UT" },
    });
    expect(c.parent?.id).toBe("REL-6");
  });

  it("puts the task under the parent a request names, before anything else", () => {
    const c = resolveCreateContext({
      ...base,
      requestParent: "US",
      selection: "UE",
      route: { name: "project", ulid: "UP" },
    });
    expect(c.parent?.id).toBe("SH-3");
    expect(c.workspace?.name).toBe("shared");
  });

  it("skips a deleted or unknown task", () => {
    const c = resolveCreateContext({
      ...base,
      selection: "UD",
      route: { name: "workspace", store: "shared", ulid: "W2" },
    });
    expect(c.parent).toBeNull();
    expect(c.workspace?.name).toBe("shared");
  });

  it("is the root of the workspace on a workspace screen, and of the first workspace elsewhere", () => {
    expect(
      resolveCreateContext({ ...base, route: { name: "workspace", store: "shared", ulid: "W2" } })
        .workspace?.ulid,
    ).toBe("W2");
    expect(resolveCreateContext(base)).toEqual({ parent: null, workspace: workspaces[0] });
    expect(resolveCreateContext({ ...base, workspaces: [] })).toEqual({
      parent: null,
      workspace: null,
    });
  });

  it("keys a workspace by store and ULID", () => {
    expect(workspaceKey(workspaces[0] as (typeof workspaces)[number])).toBe("shared/W1");
  });
});
```

`web/app/create/WorkspaceChoice.test.ts`:
```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { workspaces } from "../detail/testkit";
import WorkspaceChoice from "./WorkspaceChoice.svelte";

describe("WorkspaceChoice", () => {
  it("lists every workspace with its prefix and store, and reports the chosen one", async () => {
    const onchange = vi.fn();
    render(WorkspaceChoice, { entries: workspaces, value: "shared/W1", name: "ws", onchange });
    expect(screen.getByRole("radiogroup", { name: "ワークスペース" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /release（REL）/ })).toBeChecked();
    await fireEvent.click(screen.getByRole("radio", { name: /shared（SH）/ }));
    expect(onchange).toHaveBeenCalledWith("shared/W2");
  });
});
```

`web/app/create/CreateTask.test.ts`:
```ts
import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { parentRule } from "../../core/schema";
import {
  fail,
  makeCore,
  mount,
  ok,
  schemaView,
  stubLayout,
  taskView,
  typeInto,
  viewOf,
  workspaces,
} from "../detail/testkit";
import { selection } from "../selection.svelte";
import { resetToasts, toasts } from "../toast.svelte";
import CreateTask from "./CreateTask.svelte";

const parent = taskView({ ulid: "UP", id: "REL-8", title: "リリース作業", kind: "task" });
const created = taskView({ ulid: "U9", id: "REL-9", title: "新しい作業" });

function setup(opts: { onclose?: () => void; one?: boolean } = {}) {
  const k = makeCore({
    tasks: [parent],
    workspaces: opts.one ? [workspaces[0] as (typeof workspaces)[number]] : undefined,
  });
  k.api.POST.mockImplementation(() => ok({ task: created, commit: "c1" }));
  const onclose = opts.onclose ?? vi.fn();
  return { k, onclose, ...mount(CreateTask, { request: { kind: "task" }, onclose }, k) };
}

async function typeTitle(text: string) {
  await fireEvent.input(screen.getByRole("textbox", { name: "タイトル" }), {
    target: { value: text },
  });
}

describe("CreateTask", () => {
  beforeAll(stubLayout);
  beforeEach(() => {
    selection.set(null);
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("takes the selected row as the parent, fixes the workspace to the parent's and creates", async () => {
    selection.set("UP");
    const { k, onclose } = setup();
    expect(screen.getByText("REL-8")).toBeInTheDocument();
    expect(screen.queryByRole("radiogroup", { name: "ワークスペース" })).toBeNull();
    expect(screen.getByText(/親と同じワークスペース/)).toBeInTheDocument();
    await typeTitle("新しい作業");
    expect(screen.getByText(/--parent REL-8/)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: /^作成/ }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.POST).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { store: "shared" } },
      body: { title: "新しい作業", parent: "UP", kind: "task", status: "todo" },
    });
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ task: created, commit: "c1" });
    expect(toasts.list[0]).toMatchObject({
      message: "REL-9 を作成しました · release",
      link: { label: "開く", href: "/tasks/U9" },
    });
  });

  it("decides the workspace when the stores finish loading after the dialog opened", async () => {
    const k = makeCore({ tasks: [parent] });
    k.stores.workspaces.mockReturnValue([]);
    mount(CreateTask, { request: { kind: "task" }, onclose: vi.fn() }, k);
    await typeTitle("早押し");
    expect(screen.getByRole("button", { name: /^作成/ })).toBeDisabled();
    k.stores.workspaces.mockReturnValue(workspaces);
    k.notify();
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
  });

  it("strikes out the kinds the parent cannot hold, with the reason as a tooltip", async () => {
    selection.set("UP");
    setup();
    const project = screen.getByRole("radio", { name: /プロジェクト/ });
    expect(project).toBeDisabled();
    expect(project.closest("label")).toHaveAttribute("title", parentRule(schemaView, "project"));
    await waitFor(() => expect(screen.getByRole("radio", { name: /タスク/ })).toBeChecked());
    expect(screen.getByRole("radio", { name: /バグ/ })).toBeEnabled();
  });

  it("asks for the workspace when there is no parent and several workspaces", async () => {
    const { k } = setup();
    expect(screen.getByRole("radio", { name: /release（REL）/ })).toBeChecked();
    await fireEvent.click(screen.getByRole("radio", { name: /shared（SH）/ }));
    await typeTitle("x");
    expect(screen.getByText(/ID は作成時に振ります/)).toHaveTextContent("shared（SH）");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: /^作成/ }));
    await waitFor(() => expect(k.api.POST).toHaveBeenCalled());
    const [, init] = k.api.POST.mock.calls[0] as [string, { body: Record<string, unknown> }];
    expect(init.body).toMatchObject({ title: "x", workspace: "shared" });
    expect(init.body).not.toHaveProperty("parent");
  });

  it("does not ask when there is only one workspace", () => {
    setup({ one: true });
    expect(screen.queryByRole("radiogroup", { name: "ワークスペース" })).toBeNull();
    expect(screen.getByText("ワークスペースは release だけです")).toBeInTheDocument();
  });

  it("'続けて作成' empties the title and the body and keeps the dialog", async () => {
    const { k, onclose, container } = setup();
    await typeTitle("一つ目");
    typeInto(container, "メモ");
    await waitFor(() => expect(screen.getByRole("button", { name: "続けて作成" })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: "続けて作成" }));
    await waitFor(() => expect(k.api.POST).toHaveBeenCalledTimes(1));
    const [, init] = k.api.POST.mock.calls[0] as [string, { body: { body: string } }];
    expect(init.body.body).toBe("メモ");
    await waitFor(() => expect(screen.getByRole("textbox", { name: "タイトル" })).toHaveValue(""));
    expect(viewOf(container).state.doc.toString()).toBe("");
    expect(onclose).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("creates with Enter in the title, but not while an IME composes", async () => {
    const { k } = setup();
    await typeTitle("日本語");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    const title = screen.getByRole("textbox", { name: "タイトル" });
    await fireEvent.keyDown(title, { key: "Enter", isComposing: true });
    expect(k.api.POST).not.toHaveBeenCalled();
    await fireEvent.keyDown(title, { key: "Enter" });
    await waitFor(() => expect(k.api.POST).toHaveBeenCalledTimes(1));
  });

  it("creates with Ctrl+Enter anywhere in the dialog", async () => {
    const { k } = setup();
    await typeTitle("x");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.keyDown(window, { key: "Enter", ctrlKey: true });
    await waitFor(() => expect(k.api.POST).toHaveBeenCalledTimes(1));
  });

  it("creates with Ctrl+Enter in the body editor, once", async () => {
    const { k, container } = setup();
    await typeTitle("x");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.keyDown(viewOf(container).contentDOM, {
      key: "Enter",
      code: "Enter",
      keyCode: 13,
      ctrlKey: true,
    });
    await waitFor(() => expect(k.api.POST).toHaveBeenCalledTimes(1));
    expect(viewOf(container).state.doc.toString()).toBe("");
  });

  it("cannot create without a title", () => {
    setup();
    expect(screen.getByRole("button", { name: /^作成/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: "続けて作成" })).toBeDisabled();
  });

  it("sends the properties and the assignee that were set", async () => {
    const { k } = setup();
    await typeTitle("x");
    await fireEvent.click(screen.getByRole("button", { name: "priority" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "high" }));
    const assignee = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(assignee, { target: { value: "sato" } });
    await fireEvent.keyDown(assignee, { key: "Enter" });
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: /^作成/ }));
    await waitFor(() => expect(k.api.POST).toHaveBeenCalled());
    const [, init] = k.api.POST.mock.calls[0] as [string, { body: object }];
    expect(init.body).toMatchObject({ assignees: ["sato"], props: { priority: "high" } });
  });

  it("shows the server's error in the dialog and stays open", async () => {
    const { k, onclose } = setup();
    k.api.POST.mockImplementation(() => fail(422, "workspace.choose", { names: ["a", "b"] }));
    await typeTitle("x");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: /^作成/ }));
    expect(await within(screen.getByRole("dialog")).findByRole("alert")).toBeInTheDocument();
    expect(onclose).not.toHaveBeenCalled();
  });

  it("lets the parent be removed, which brings the workspace choice back", async () => {
    selection.set("UP");
    setup();
    await fireEvent.click(screen.getByRole("button", { name: "親を外す" }));
    expect(screen.getByRole("radiogroup", { name: "ワークスペース" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /release（REL）/ })).toBeChecked();
  });

  it("puts the task under the parent that the request names", () => {
    const k = makeCore({ tasks: [parent] });
    mount(CreateTask, { request: { kind: "task", parent: "UP" }, onclose: vi.fn() }, k);
    expect(screen.getByText("REL-8")).toBeInTheDocument();
  });
});
```

`web/app/create/CreateProject.test.ts`:
```ts
import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeCore, mount, ok, taskView, workspaces } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import CreateProject from "./CreateProject.svelte";

const created = taskView({ ulid: "UPJ", id: "REL-20", title: "Web フロント刷新", kind: "project" });

function setup(one = false) {
  const k = makeCore({
    workspaces: one ? [workspaces[0] as (typeof workspaces)[number]] : undefined,
  });
  k.api.POST.mockImplementation(() => ok({ task: created, commit: "c1" }));
  const onclose = vi.fn();
  return { k, onclose, ...mount(CreateProject, { request: { kind: "project" }, onclose }, k) };
}

async function name(text: string) {
  await fireEvent.input(screen.getByRole("textbox", { name: "名前" }), { target: { value: text } });
  await waitFor(() => expect(screen.getByRole("button", { name: "作成" })).toBeEnabled());
}

describe("CreateProject", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("creates a project with the repositories, assignee and dates", async () => {
    const { k, onclose } = setup();
    await name("Web フロント刷新");

    const repo = screen.getByRole("textbox", { name: "リポジトリ" });
    await fireEvent.input(repo, { target: { value: "git@github.com:Org-A/Web.git" } });
    expect(
      screen.getByText(/Enter で github.com\/org-a\/web として追加します/),
    ).toBeInTheDocument();
    await fireEvent.keyDown(repo, { key: "Enter" });

    const who = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(who, { target: { value: "sato" } });
    await fireEvent.keyDown(who, { key: "Enter" });
    await fireEvent.change(screen.getByLabelText("due"), { target: { value: "2026-12-01" } });

    expect(screen.getByText(/--kind project/)).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "作成" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.POST).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { store: "shared" } },
      body: {
        title: "Web フロント刷新",
        kind: "project",
        workspace: "release",
        assignees: ["sato"],
        repos: ["github.com/org-a/web"],
        props: { due: "2026-12-01" },
      },
    });
    expect(toasts.list[0]).toMatchObject({
      message: "REL-20 を作成しました · release",
      link: { label: "開く", href: "/projects/UPJ" },
    });
  });

  it("refuses text that is not a repository and says why", async () => {
    setup();
    const repo = screen.getByRole("textbox", { name: "リポジトリ" });
    await fireEvent.input(repo, { target: { value: "/home/me/repo" } });
    await fireEvent.keyDown(repo, { key: "Enter" });
    expect(screen.getByRole("alert")).toHaveTextContent("リポジトリの URL として読めません");
  });

  it("creates a project without repositories (an operations-only project)", async () => {
    const { k } = setup();
    await name("運用");
    await fireEvent.click(screen.getByRole("button", { name: "作成" }));
    await waitFor(() => expect(k.api.POST).toHaveBeenCalled());
    const [, init] = k.api.POST.mock.calls[0] as [string, { body: object }];
    expect(init.body).not.toHaveProperty("repos");
  });

  it("asks for the workspace when there are several", async () => {
    const { k } = setup();
    await fireEvent.click(screen.getByRole("radio", { name: /shared（SH）/ }));
    await name("x");
    await fireEvent.click(screen.getByRole("button", { name: "作成" }));
    await waitFor(() => expect(k.api.POST).toHaveBeenCalled());
    const [, init] = k.api.POST.mock.calls[0] as [string, { body: { workspace: string } }];
    expect(init.body.workspace).toBe("shared");
  });

  it("fixes the workspace when there is one", () => {
    setup(true);
    expect(screen.queryByRole("radiogroup", { name: "ワークスペース" })).toBeNull();
    expect(screen.getByText(/release（REL）/)).toBeInTheDocument();
  });

  it("cannot be created without a name", () => {
    setup();
    expect(screen.getByRole("button", { name: "作成" })).toBeDisabled();
  });
});
```

`web/app/create/CreateHost.test.ts`:
```ts
import { fireEvent, screen } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { closeCreate, openCreate } from "../create-state.svelte";
import { makeCore, mount } from "../detail/testkit";
import CreateHost from "./CreateHost.svelte";

describe("CreateHost", () => {
  beforeEach(() => history.replaceState(null, "", "/"));
  afterEach(() => closeCreate());

  it("opens the task dialog with C", async () => {
    mount(CreateHost, {}, makeCore());
    expect(screen.queryByRole("dialog")).toBeNull();
    await fireEvent.keyDown(window, { key: "c" });
    expect(await screen.findByRole("dialog")).toHaveTextContent("タスクを作成");
  });

  it("ignores C typed into an input, and C with a modifier", async () => {
    mount(CreateHost, {}, makeCore());
    const input = document.createElement("input");
    document.body.append(input);
    await fireEvent.keyDown(input, { key: "c" });
    await fireEvent.keyDown(window, { key: "c", ctrlKey: true });
    await fireEvent.keyDown(window, { key: "c", metaKey: true });
    expect(screen.queryByRole("dialog")).toBeNull();
    input.remove();
  });

  it("opens the project dialog for openCreate({ kind: 'project' })", async () => {
    mount(CreateHost, {}, makeCore());
    openCreate({ kind: "project" });
    expect(await screen.findByRole("dialog")).toHaveTextContent("プロジェクトを作成");
  });

  it("closes with the cancel button", async () => {
    mount(CreateHost, {}, makeCore());
    openCreate({ kind: "task" });
    await fireEvent.click(await screen.findByRole("button", { name: "キャンセル" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("does not open a second dialog with C while one is open", async () => {
    mount(CreateHost, {}, makeCore());
    openCreate({ kind: "project" });
    await screen.findByRole("dialog");
    await fireEvent.keyDown(window, { key: "c" });
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    expect(screen.getByRole("dialog")).toHaveTextContent("プロジェクトを作成");
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm vitest run app/create`
Expected: FAIL（`./repos`・`./context` と `create/*.svelte` が存在しない）

- [ ] **Step 3: 実装する**

`web/app/create/repos.ts`:
```ts
// scpLike is user@host:owner/repo, the short form of ssh.
const scpLike = /^[^@/]+@([^:/]+):(.+)$/;
// An ID that is already host/owner/repo (not a URL).
const bare = /^[^/:@\s]+\.[^/:@\s]+\/[^/\s]+\/[^/\s]+$/;

const trimSlashes = (s: string) => s.replace(/^\/+|\/+$/g, "");
const trimGit = (s: string) => (s.endsWith(".git") ? s.slice(0, -4) : s);

function join(host: string, path: string): string | null {
  const parts = path.split("/");
  if (host === "" || parts.length !== 2 || parts[0] === "" || parts[1] === "") return null;
  return `${host}/${path}`.toLowerCase();
}

// repoIdOf turns a remote URL into <host>/<owner>/<repo> in lower case, without .git, a trailing slash or a
// port, as internal/context.RepoID does (https, http, ssh, git and user@host:path). A local path, or a URL
// without an owner and a repo, gives null. An ID already in that form is accepted as it is.
export function repoIdOf(input: string): string | null {
  const u = input.trim();
  let host = "";
  let path = "";
  const scp = scpLike.exec(u);
  if (scp && !u.includes("://")) {
    host = scp[1] ?? "";
    path = scp[2] ?? "";
  } else {
    let url: URL;
    try {
      url = new URL(u);
    } catch {
      const p = trimSlashes(trimGit(trimSlashes(u)));
      return bare.test(p) ? p.toLowerCase() : null;
    }
    if (!["https:", "http:", "ssh:", "git:"].includes(url.protocol)) return null;
    host = url.hostname;
    path = url.pathname;
  }
  return join(host, trimSlashes(trimGit(trimSlashes(path))));
}
```

`web/app/create/context.ts`:
```ts
import type { TaskView } from "../../core/api/client";
import type { WorkspaceEntry } from "../../core/stores";
import type { Route } from "../router";

export type CreateContext = { parent: TaskView | null; workspace: WorkspaceEntry | null };

export const workspaceKey = (w: Pick<WorkspaceEntry, "store" | "ulid">) => `${w.store}/${w.ulid}`;

// resolveCreateContext decides where a new task goes from the screen the user is on (細部 29): the parent a request
// names, the task of a task detail, the selected row of the tree, the project of a project screen; on a workspace
// screen the root of that workspace; elsewhere the root of the first workspace. A deleted or unknown task is skipped.
export function resolveCreateContext(i: {
  requestParent?: string;
  selection: string | null;
  route: Route;
  task: (ulid: string) => TaskView | undefined;
  workspaces: WorkspaceEntry[];
}): CreateContext {
  const live = (u: string | null | undefined): TaskView | undefined => {
    if (!u) return undefined;
    const t = i.task(u);
    return t && t.deleted_at == null ? t : undefined;
  };
  const workspaceOf = (t: TaskView) =>
    i.workspaces.find((w) => w.store === t.store && w.ulid === t.workspace.ulid) ?? null;

  const r = i.route;
  const parent =
    live(i.requestParent) ??
    (r.name === "task" ? live(r.ulid) : undefined) ??
    live(i.selection) ??
    (r.name === "project" ? live(r.ulid) : undefined);
  if (parent) return { parent, workspace: workspaceOf(parent) };

  if (r.name === "workspace") {
    const w = i.workspaces.find((x) => x.store === r.store && x.ulid === r.ulid);
    return { parent: null, workspace: w ?? i.workspaces[0] ?? null };
  }
  return { parent: null, workspace: i.workspaces[0] ?? null };
}
```

`web/app/create/WorkspaceChoice.svelte`:
```svelte
<script lang="ts">
  import type { WorkspaceEntry } from "../../core/stores";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { workspaceKey } from "./context";

  let {
    entries,
    value,
    name,
    onchange,
  }: { entries: WorkspaceEntry[]; value: string; name: string; onchange: (key: string) => void } =
    $props();
</script>

<div class="choice" role="radiogroup" aria-label="ワークスペース">
  {#each entries as w (workspaceKey(w))}
    <label class:on={workspaceKey(w) === value}>
      <input
        type="radio"
        {name}
        value={workspaceKey(w)}
        checked={workspaceKey(w) === value}
        onchange={() => onchange(workspaceKey(w))}
      >
      <span>{w.name}（{w.prefix}）</span>
      <StoreBadge store={w.store} tone={w.tone} />
    </label>
  {/each}
</div>

<style>
  .choice {
    display: grid;
    gap: var(--space-4);
  }
  label {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    padding: var(--space-6) var(--space-10);
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
    font-size: var(--fs-13);
    cursor: pointer;
  }
  label.on {
    border-color: var(--accent);
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
</style>
```

`web/app/create/CreateTask.svelte`:
```svelte
<script lang="ts">
  import { untrack } from "svelte";
  import { type TaskView, unwrap } from "../../core/api/client";
  import { cliAdd } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { canParent, defaultStatus, parentRule } from "../../core/schema";
  import { core } from "../context";
  import type { CreateRequest } from "../create-state.svelte";
  import BodyEditor from "../detail/BodyEditor.svelte";
  import FieldInput from "../detail/FieldInput.svelte";
  import { propRows } from "../detail/prop-rows";
  import TaskPicker from "../detail/TaskPicker.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import { kbdLabel, modKey } from "../keys";
  import { router } from "../router";
  import { selection } from "../selection.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { resolveCreateContext, workspaceKey } from "./context";
  import WorkspaceChoice from "./WorkspaceChoice.svelte";

  let { request, onclose }: { request: CreateRequest; onclose: () => void } = $props();

  const { api, cache, stores, schema } = core();
  const entries = useSubscribe(stores, () => stores.workspaces());

  // Where it goes is decided when the dialog opens.
  const contextOf = (workspaces: ReturnType<typeof stores.workspaces>) =>
    resolveCreateContext({
      requestParent: request.parent,
      selection: selection.ulid,
      route: router.route,
      task: (u) => cache.task(u),
      workspaces,
    });
  const ctx = untrack(() => contextOf(stores.workspaces()));
  let parent = $state<TaskView | null>(ctx.parent);
  let wsKey = $state(ctx.workspace ? workspaceKey(ctx.workspace) : "");
  // C pressed while the stores are still loading opens the dialog with no workspace to choose
  // from: decide again when they have loaded.
  $effect(() => {
    const list = entries.current;
    if (wsKey !== "" || parent || list.length === 0) return;
    const again = untrack(() => contextOf(list));
    if (again.parent) parent = again.parent;
    if (again.workspace) wsKey = workspaceKey(again.workspace);
  });
  const ws = $derived.by(() => {
    const p = parent;
    if (p) return entries.current.find((w) => w.store === p.store && w.ulid === p.workspace.ulid);
    return entries.current.find((w) => workspaceKey(w) === wsKey);
  });
  const showChoice = $derived(!parent && entries.current.length >= 2);

  const sc = useSubscribe(schema, () => (ws ? schema.get(ws.store) : undefined));
  $effect(() => {
    if (ws) void schema.load(ws.store);
  });

  let title = $state("");
  let body = $state("");
  let kind = $state("");
  let status = $state("");
  let assignees = $state<string[]>([]);
  let propValues = $state<Record<string, unknown>>({});
  let busy = $state(false);
  let error = $state("");
  let titleEl = $state<HTMLInputElement>();
  let bodyEl = $state<{ setText(t: string): void } | undefined>();

  $effect(() => {
    titleEl?.focus();
  });

  const parentKind = $derived(parent?.kind ?? null);
  const kinds = $derived(sc.current?.kinds ?? []);
  const allowed = (name: string) => !sc.current || canParent(sc.current, name, parentKind);
  $effect(() => {
    const s = sc.current;
    if (!s) return;
    const pk = parentKind;
    const cur = untrack(() => kind);
    if (cur && s.kinds?.some((k) => k.name === cur) && canParent(s, cur, pk)) return;
    kind =
      (
        s.kinds?.find((k) => k.default && canParent(s, k.name, pk)) ??
        s.kinds?.find((k) => canParent(s, k.name, pk))
      )?.name ?? "";
  });
  $effect(() => {
    const s = sc.current;
    if (!s) return;
    if (!s.statuses?.some((x) => x.name === untrack(() => status))) {
      status = defaultStatus(s, "todo");
    }
  });

  const defaultKindName = $derived(kinds.find((k) => k.default)?.name ?? "");
  const defaultStatusName = $derived(sc.current ? defaultStatus(sc.current, "todo") : "");
  const rows = $derived(
    (sc.current ? propRows(sc.current, {}) : []).filter((r) => r.editor !== null),
  );
  const statusOptions = $derived(
    (sc.current?.statuses ?? []).map((s) => ({
      value: s.name,
      label: `${s.label || s.name} · ${s.category}`,
    })),
  );
  const hold = $derived.by(() => {
    if (!parent) return "";
    const usable = kinds.filter((k) => allowed(k.name)).map((k) => k.label || k.name);
    return usable.length < kinds.length
      ? `${parent.id} の下に置けるのは${usable.join("・")}です`
      : "";
  });
  const canSubmit = $derived(!busy && title.trim() !== "" && !!ws && !!sc.current && kind !== "");
  const submitKey = kbdLabel("mod+enter");

  function cleanProps(): Record<string, unknown> {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(propValues)) {
      if (v === null || v === undefined || v === "" || (Array.isArray(v) && v.length === 0)) {
        continue;
      }
      out[k] = v;
    }
    return out;
  }

  const cli = $derived.by(() => {
    const props: Record<string, string> = {};
    if (status && status !== defaultStatusName) props.status = status;
    if (assignees.length > 0) props.assignees = assignees.join(",");
    for (const [k, v] of Object.entries(cleanProps())) {
      props[k] = Array.isArray(v) ? v.join(",") : String(v);
    }
    return cliAdd({
      title: title.trim() || "タイトル",
      parent: parent?.id,
      store: parent ? undefined : ws?.store,
      workspace: showChoice ? ws?.name : undefined,
      kind: kind && kind !== defaultKindName ? kind : undefined,
      body: body.trim() === "" ? undefined : body,
      props: Object.keys(props).length > 0 ? props : undefined,
    });
  });

  async function submit(more: boolean) {
    if (!canSubmit || !ws) return;
    busy = true;
    error = "";
    try {
      const props = cleanProps();
      const r = await unwrap(
        api.POST("/api/tasks", {
          params: { query: { store: ws.store } },
          body: {
            title: title.trim(),
            ...(body.trim() === "" ? {} : { body }),
            ...(parent ? { parent: parent.ulid } : { workspace: ws.name }),
            kind,
            status,
            ...(assignees.length > 0 ? { assignees } : {}),
            ...(Object.keys(props).length > 0 ? { props } : {}),
          },
        }),
      );
      cache.applyWrite({ task: r.task, commit: r.commit });
      toast({
        message: `${r.task.id} を作成しました · ${ws.name}`,
        link: { label: "開く", href: router.href({ name: "task", ulid: r.task.ulid }) },
      });
      if (more) {
        title = "";
        body = "";
        bodyEl?.setText("");
        titleEl?.focus();
      } else {
        onclose();
      }
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onTitleKey(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229 && !modKey(e)) {
      e.preventDefault();
      void submit(false);
    }
  }

  function removeParent() {
    const w = ws;
    parent = null;
    if (w) wsKey = workspaceKey(w);
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === "Enter" && modKey(e) && !e.defaultPrevented) {
      e.preventDefault();
      void submit(false);
    }
  }}
/>

<Dialog open title="タスクを作成" width={760} {cli} {onclose}>
  <div class="form">
    <div class="field">
      <span class="lbl">親</span>
      {#if parent}
        <span class="parent">
          <span class="mono">{parent.id}</span>
          {parent.title}
          <button type="button" aria-label="親を外す" onclick={removeParent}>×</button>
        </span>
      {:else if ws}
        <TaskPicker
          store={ws.store}
          label="親を検索"
          placeholder="親を ID かタイトルで検索（空のままならルート）"
          onpick={(t) => (parent = t)}
        />
        <span class="hint">親: なし（ワークスペースのルート）</span>
      {/if}
    </div>

    {#if showChoice}
      <div class="field">
        <WorkspaceChoice
          entries={entries.current}
          value={wsKey}
          name="create-ws"
          onchange={(k) => (wsKey = k)}
        />
        <span class="hint">全 Store にワークスペースが {entries.current.length} つあります</span>
      </div>
    {:else if ws}
      <div class="field">
        <span class="lbl">ワークスペース</span>
        <span class="fixed"
          >{ws.name}（{ws.prefix}） <StoreBadge store={ws.store} tone={ws.tone} /></span
        >
        <span class="hint"
          >{parent ? "親と同じワークスペースです" : `ワークスペースは ${ws.name} だけです`}</span
        >
      </div>
    {/if}

    <label class="field">
      <span class="lbl">タイトル</span>
      <input
        type="text"
        aria-label="タイトル"
        bind:this={titleEl}
        bind:value={title}
        onkeydown={onTitleKey}
        autocomplete="off"
      >
    </label>

    <div class="field">
      <span class="lbl">種類</span>
      <div class="kinds" role="radiogroup" aria-label="種類">
        {#each kinds as k (k.name)}
          {@const usable = allowed(k.name)}
          <label
            class:on={kind === k.name}
            class:off={!usable}
            title={usable || !sc.current ? undefined : parentRule(sc.current, k.name)}
          >
            <input
              type="radio"
              name="create-kind"
              value={k.name}
              checked={kind === k.name}
              disabled={!usable}
              onchange={() => (kind = k.name)}
            >
            <KindIcon kind={k.name} role={k.role} />
            {k.label || k.name}
          </label>
        {/each}
      </div>
      {#if hold}
        <span class="hint">{hold}</span>
      {/if}
    </div>

    <div class="field">
      <span class="lbl">本文 <span class="hint">Markdown · @ で人、# でタスク</span></span>
      {#if ws}
        <BodyEditor
          bind:this={bodyEl}
          value={body}
          store={ws.store}
          label="本文"
          minHeight="6em"
          onedit={(t) => (body = t)}
          onsubmit={() => void submit(false)}
        />
      {/if}
    </div>

    <div class="props">
      <div class="prow">
        <span class="lbl">ステータス</span>
        <FieldInput
          editor="select"
          label="ステータス"
          value={status}
          options={statusOptions}
          nullable={false}
          onedit={(v) => (status = String(v))}
          oncommit={() => {}}
        />
      </div>
      <div class="prow">
        <span class="lbl">担当者</span>
        <FieldInput
          editor="chips"
          label="担当者"
          value={assignees}
          onedit={(v) => (assignees = v as string[])}
          oncommit={() => {}}
        />
      </div>
      {#each rows as r (r.name)}
        <div class="prow">
          <span class="lbl">{r.label}</span>
          {#if r.editor === "enum" && !r.multi}
            <FieldInput
              editor="select"
              label={r.label}
              value={propValues[r.name] ?? null}
              options={r.options.map((o) => ({ value: o, label: o }))}
              onedit={(v) => (propValues[r.name] = v)}
              oncommit={() => {}}
            />
          {:else if r.editor === "enum" ||
            r.editor === "multi_enum" ||
            (r.editor === "user" && r.multi)}
            <FieldInput
              editor="chips"
              label={r.label}
              value={propValues[r.name] ?? []}
              options={r.options.map((o) => ({ value: o, label: o }))}
              onedit={(v) => (propValues[r.name] = v)}
              oncommit={() => {}}
            />
          {:else if r.editor === "bool"}
            <FieldInput
              editor="select"
              label={r.label}
              value={propValues[r.name] ?? null}
              options={[
                { value: "true", label: "はい" },
                { value: "false", label: "いいえ" },
              ]}
              onedit={(v) => (propValues[r.name] = v === null ? null : v === "true")}
              oncommit={() => {}}
            />
          {:else if r.editor === "date"}
            <FieldInput
              editor="date"
              label={r.label}
              value={propValues[r.name] ?? null}
              onedit={(v) => (propValues[r.name] = v)}
              oncommit={() => {}}
            />
          {:else if r.editor === "number"}
            <FieldInput
              editor="number"
              label={r.label}
              value={propValues[r.name] ?? null}
              onedit={(v) => (propValues[r.name] = v)}
              oncommit={() => {}}
            />
          {:else}
            <FieldInput
              editor="text"
              label={r.label}
              value={propValues[r.name] ?? null}
              onedit={(v) => (propValues[r.name] = v)}
              oncommit={() => {}}
            />
          {/if}
        </div>
      {/each}
    </div>

    {#if ws}
      <p class="dest">入れ先 <strong>{ws.name}（{ws.prefix}）</strong> · ID は作成時に振ります</p>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={!canSubmit} onclick={() => void submit(true)}>続けて作成</Button>
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="primary" disabled={!canSubmit} onclick={() => void submit(false)}
      >作成 {submitKey}</Button
    >
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: var(--space-12);
  }
  .field {
    display: grid;
    gap: var(--space-4);
  }
  .lbl {
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--ink-2);
  }
  .hint {
    font-size: var(--fs-12);
    font-weight: 400;
    color: var(--muted);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .parent,
  .fixed {
    display: inline-flex;
    gap: var(--space-8);
    align-items: center;
    font-size: var(--fs-13);
  }
  .parent button {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
  }
  input[type="text"] {
    width: 100%;
    height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  input[type="text"]:focus-visible {
    outline: 2px solid var(--accent);
  }
  .kinds {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-6);
  }
  .kinds label {
    position: relative;
    display: inline-flex;
    gap: var(--space-4);
    align-items: center;
    padding: var(--space-4) var(--space-10);
    border-radius: var(--radius-6);
    background: var(--sunken);
    color: var(--ink-2);
    font-size: var(--fs-13);
    cursor: pointer;
  }
  .kinds label.on {
    background: var(--ink);
    color: var(--surface);
  }
  .kinds label.off {
    background: transparent;
    box-shadow: inset 0 0 0 1px var(--line);
    color: var(--muted-2);
    text-decoration: line-through;
    cursor: not-allowed;
  }
  .kinds input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }
  .props {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-6) var(--space-16);
  }
  .prow {
    display: grid;
    grid-template-columns: 7em minmax(0, 1fr);
    align-items: center;
    gap: var(--space-8);
  }
  .dest {
    margin: 0;
    font-size: var(--fs-13);
    color: var(--ink-2);
  }
  .error {
    margin: 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
```

`web/app/create/CreateProject.svelte`:
```svelte
<script lang="ts">
  import { untrack } from "svelte";
  import { unwrap } from "../../core/api/client";
  import { cliAdd } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { canParent } from "../../core/schema";
  import { core } from "../context";
  import type { CreateRequest } from "../create-state.svelte";
  import FieldInput from "../detail/FieldInput.svelte";
  import { modKey } from "../keys";
  import { router } from "../router";
  import { selection } from "../selection.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { resolveCreateContext, workspaceKey } from "./context";
  import { repoIdOf } from "./repos";
  import WorkspaceChoice from "./WorkspaceChoice.svelte";

  let { onclose }: { request: CreateRequest; onclose: () => void } = $props();

  const { api, cache, stores, schema } = core();
  const entries = useSubscribe(stores, () => stores.workspaces());

  const ctx = untrack(() =>
    resolveCreateContext({
      selection: selection.ulid,
      route: router.route,
      task: (u) => cache.task(u),
      workspaces: stores.workspaces(),
    }),
  );
  let wsKey = $state(ctx.workspace ? workspaceKey(ctx.workspace) : "");
  const ws = $derived(entries.current.find((w) => workspaceKey(w) === wsKey));
  const showChoice = $derived(entries.current.length >= 2);

  const sc = useSubscribe(schema, () => (ws ? schema.get(ws.store) : undefined));
  $effect(() => {
    if (ws) void schema.load(ws.store);
  });

  let title = $state("");
  let repos = $state<string[]>([]);
  let assignees = $state<string[]>([]);
  let start = $state<string | null>(null);
  let due = $state<string | null>(null);
  let busy = $state(false);
  let error = $state("");
  let titleEl = $state<HTMLInputElement>();
  $effect(() => {
    titleEl?.focus();
  });

  const projectKind = $derived.by(() => {
    const s = sc.current;
    if (!s) return "";
    if (s.kinds?.some((k) => k.name === "project")) return "project";
    return s.kinds?.find((k) => k.role === "container" && canParent(s, k.name, null))?.name ?? "";
  });
  const hasStart = $derived(!!sc.current && "start" in sc.current.props);
  const hasDue = $derived(!!sc.current && "due" in sc.current.props);
  const canSubmit = $derived(!busy && title.trim() !== "" && !!ws && projectKind !== "");

  const cli = $derived.by(() => {
    const props: Record<string, string> = {};
    if (assignees.length > 0) props.assignees = assignees.join(",");
    if (repos.length > 0) props.repos = repos.join(",");
    if (start) props.start = start;
    if (due) props.due = due;
    return cliAdd({
      title: title.trim() || "名前",
      kind: "project",
      store: ws?.store,
      workspace: showChoice ? ws?.name : undefined,
      props: Object.keys(props).length > 0 ? props : undefined,
    });
  });

  const repoHint = (text: string) => {
    const id = repoIdOf(text);
    return id ? `Enter で ${id} として追加します（SSH・HTTPS・末尾の .git を揃えます）` : null;
  };

  async function submit() {
    if (!canSubmit || !ws) return;
    busy = true;
    error = "";
    try {
      const props: Record<string, unknown> = {};
      if (start) props.start = start;
      if (due) props.due = due;
      const r = await unwrap(
        api.POST("/api/tasks", {
          params: { query: { store: ws.store } },
          body: {
            title: title.trim(),
            kind: projectKind,
            workspace: ws.name,
            ...(assignees.length > 0 ? { assignees } : {}),
            ...(repos.length > 0 ? { repos } : {}),
            ...(Object.keys(props).length > 0 ? { props } : {}),
          },
        }),
      );
      cache.applyWrite({ task: r.task, commit: r.commit });
      toast({
        message: `${r.task.id} を作成しました · ${ws.name}`,
        link: { label: "開く", href: router.href({ name: "project", ulid: r.task.ulid }) },
      });
      onclose();
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onTitleKey(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229 && !modKey(e)) {
      e.preventDefault();
      void submit();
    }
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === "Enter" && modKey(e)) {
      e.preventDefault();
      void submit();
    }
  }}
/>

<Dialog open title="プロジェクトを作成" width={620} {cli} {onclose}>
  <div class="form">
    <label class="field">
      <span class="lbl">名前</span>
      <input
        type="text"
        aria-label="名前"
        bind:this={titleEl}
        bind:value={title}
        onkeydown={onTitleKey}
        autocomplete="off"
      >
    </label>

    {#if showChoice}
      <div class="field">
        <WorkspaceChoice
          entries={entries.current}
          value={wsKey}
          name="create-project-ws"
          onchange={(k) => (wsKey = k)}
        />
      </div>
    {:else if ws}
      <div class="field">
        <span class="lbl">ワークスペース</span>
        <span class="fixed"
          >{ws.name}（{ws.prefix}） <StoreBadge store={ws.store} tone={ws.tone} /></span
        >
      </div>
    {/if}
    <p class="hint">
      ID はこのワークスペースの接頭辞で、作成時に振ります。子タスクも同じワークスペースに入ります
    </p>

    <div class="field">
      <span class="lbl"
        >リポジトリ <span class="hint">— なくても作れます（運用だけのプロジェクト）</span></span
      >
      <FieldInput
        editor="chips"
        label="リポジトリ"
        value={repos}
        placeholder="https://github.com/org/repo.git"
        transform={repoIdOf}
        invalidMessage="リポジトリの URL として読めません（例: https://github.com/org/repo.git）"
        hintFor={repoHint}
        onedit={(v) => (repos = v as string[])}
        oncommit={() => {}}
      />
      <span class="hint">このリポジトリの中で hh add すると、このプロジェクトに入ります</span>
    </div>

    <div class="field">
      <span class="lbl">担当者</span>
      <FieldInput
        editor="chips"
        label="担当者"
        value={assignees}
        onedit={(v) => (assignees = v as string[])}
        oncommit={() => {}}
      />
    </div>

    {#if hasStart || hasDue}
      <div class="dates">
        {#if hasStart}
          <div class="field">
            <span class="lbl">start</span>
            <FieldInput
              editor="date"
              label="start"
              value={start}
              onedit={(v) => (start = v as string | null)}
              oncommit={() => {}}
            />
          </div>
        {/if}
        {#if hasDue}
          <div class="field">
            <span class="lbl">due</span>
            <FieldInput
              editor="date"
              label="due"
              value={due}
              onedit={(v) => (due = v as string | null)}
              oncommit={() => {}}
            />
          </div>
        {/if}
      </div>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>

  {#snippet footer()}
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="primary" disabled={!canSubmit} onclick={() => void submit()}>作成</Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: var(--space-12);
  }
  .field {
    display: grid;
    gap: var(--space-4);
  }
  .lbl {
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--ink-2);
  }
  .hint {
    margin: 0;
    font-size: var(--fs-12);
    font-weight: 400;
    color: var(--muted);
  }
  .fixed {
    display: inline-flex;
    gap: var(--space-8);
    align-items: center;
    font-size: var(--fs-13);
  }
  input[type="text"] {
    width: 100%;
    height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  input[type="text"]:focus-visible {
    outline: 2px solid var(--accent);
  }
  .dates {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-12);
  }
  .error {
    margin: 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
```

`web/app/create/CreateHost.svelte`:
```svelte
<script lang="ts">
  import { closeCreate, createState, openCreate } from "../create-state.svelte";
  import { registerShortcut } from "../keys";
  import CreateProject from "./CreateProject.svelte";
  import CreateTask from "./CreateTask.svelte";

  const req = $derived(createState.request);

  // C opens the task dialog. registerShortcut leaves the key alone while typing and while a dialog is open.
  $effect(() => registerShortcut("c", () => openCreate({ kind: "task" })));
</script>

{#if req?.kind === "task"}
  {#key req}
    <CreateTask request={req} onclose={closeCreate} />
  {/key}
{:else if req?.kind === "project"}
  {#key req}
    <CreateProject request={req} onclose={closeCreate} />
  {/key}
{/if}
```

`web/app/App.svelte` の変更。import に `CreateHost` を足し、`<Toasts />` の直前に置く:
```svelte
<script lang="ts">
  // 既存の import に足す
  import CreateHost from "./create/CreateHost.svelte";
</script>

<CreateHost />
<Toasts />
```

- [ ] **Step 4: 作成の状態と、まだなかった入口を足す**

`CreateHost`（C）・`CreateTask`・`CreateProject` は `createState` を読む。この Task が置く。4d-3 のツリーとサイドバーは、作成ダイアログがまだないので、空の状態の「タスクを作成」もサイドバーの「プロジェクトを追加」も持たない。ここで足す。

`web/app/create-state.svelte.ts`（新規）:
```ts
/** What the create dialog is asked for. `parent` is the ULID of the task the new one goes under. */
export type CreateRequest = { kind: "task" | "project"; parent?: string };

let request = $state<CreateRequest | null>(null);

/** The create dialog that is asked for; CreateTask and CreateProject read it. */
export const createState = {
  get request(): CreateRequest | null {
    return request;
  },
};

export function openCreate(init: CreateRequest): void {
  request = init;
}

export function closeCreate(): void {
  request = null;
}
```

`web/app/tree/TreeView.svelte` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```svelte
  import { core } from "../context";
  import Icon
```

変更後:
```svelte
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon
```

変更前:
```svelte
        <Empty variant="empty" {...emptyProps} />
```

変更後:
```svelte
        <Empty
          variant="empty"
          {...emptyProps}
          onCreate={() => openCreate({ kind: "task", parent: viewRoot ?? undefined })}
        />
```

`web/app/tree/TreeView.test.ts` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```ts
import { selection } from "../selection.svelte";
```

変更後:
```ts
import { closeCreate, createState } from "../create-state.svelte";
import { selection } from "../selection.svelte";
```

変更前:
```ts
  selection.set(null);
});
```

変更後:
```ts
  selection.set(null);
  closeCreate();
});
```

変更前:
```ts
  it("says that an empty project has no task yet, with no button for a screen that is not there", () => {
    setup({ roots: empty });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.getByText("リリース準備 v2.4 にはまだタスクがありません")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /タスクを作成/ })).toBeNull();
  });
```

変更後:
```ts
  it("offers to create the first task of an empty project", async () => {
    setup({ roots: empty });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.getByText("リリース準備 v2.4 にはまだタスクがありません")).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: /タスクを作成/ }));
    expect(createState.request).toEqual({ kind: "task", parent: "P" });
  });
```

`web/app/sidebar/Sidebar.svelte` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```svelte
  import { core } from "../context";
  import Icon
```

変更後:
```svelte
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon
```

変更前:
```svelte
      <span class="ws-title">ワークスペース</span>
    </div>
```

変更後:
```svelte
      <span class="ws-title">ワークスペース</span>
      <button
        type="button"
        class="add"
        aria-label="プロジェクトを追加"
        onclick={() => openCreate({ kind: "project" })}
      >
        <Icon name="folder-plus" size={14} />
      </button>
    </div>
```

変更前:
```svelte
  .ws {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
```

変更後:
```svelte
  .add {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--label);
    cursor: pointer;
  }
  .add:hover {
    background: var(--hover);
  }
  .ws {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
```

`web/app/sidebar/Sidebar.test.ts` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```ts
import { renderWithCore } from "../test-utils";
import Sidebar
```

変更後:
```ts
import { closeCreate, createState } from "../create-state.svelte";
import { renderWithCore } from "../test-utils";
import Sidebar
```

変更前:
```ts
  history.replaceState(null, "", "/");
});
```

変更後:
```ts
  history.replaceState(null, "", "/");
  closeCreate();
});
```

変更前:
```ts
    expect(location.pathname).toBe("/projects/01PRJ1");
  });
```

変更後:
```ts
    expect(location.pathname).toBe("/projects/01PRJ1");
  });

  it("プロジェクトを追加で作成ダイアログの要求を出す", async () => {
    const { getByRole } = mount();
    await fireEvent.click(getByRole("button", { name: "プロジェクトを追加" }));
    expect(createState.request).toEqual({ kind: "project" });
  });
```

- [ ] **Step 5: 通ることを確かめる**

Run: `cd web && pnpm fix && pnpm vitest run app && pnpm check && pnpm lint && pnpm build`
Expected: PASS（vitest 全件、svelte-check 警告 0、biome 指摘 0、vite build 成功）。`vite build` は CodeMirror で 500 kB を超えるチャンクの警告を出す（失敗ではない。`build.chunkSizeWarningLimit` を上げるかどうかは Task 2 の持ち物）。

- [ ] **Step 6: コミット**

```bash
git add web/app/create web/app/create-state.svelte.ts web/app/tree web/app/sidebar web/app/App.svelte
git commit -m "feat(web): タスクとプロジェクトの作成ダイアログ"
```

---

### Task 14: コマンドパレット

画面は `CommandPalette.dc.html`、動きは plan 前半の細部 30・40。⌘K で開く（⌘K の登録とサイドバーの検索の欄は、この Task が `Sidebar` に足す。`palette.open` を読んで描くのは `CommandPalette`）。入力から3つのモードを自動で選ぶ。コードは Task 2〜9 の実物の上で `svelte-check`・`vitest`・`biome ci`・`vite build` を通したものを載せている。

この Task で決めること:

1. **モードの判定**（`detectMode`）: 先頭（前の空白を除く）が `>` ならコマンド。`:` を含む語があるか、`due<2026-11-10` のような比較（`<`・`>`・`<=`・`>=`）の語があればクエリ。それ以外（空を含む）は ID・検索。ID・旧 ID・ULID・タイトルは区別せず、同じ入力に対して全部を探す
2. **ID・検索**: 入力が ID の形（`REL-6`、大文字小文字を問わない）か ULID の形なら、`GET /tasks/{入力}`（全 Store から引ける。細部 40）を1回呼ぶ（404 は無視）。サーバーは ID・旧 ID・ULID を受け付けるので、旧 ID で一致したときは返ってきた `id` が入力と違い、`former_ids` に入力がある → 「旧 ID SH-17 で一致」（canvas のとおり、入力を大文字にして出す）。続けて core の `searchTasks(api, 入力, { limit: 20 })`（語ごとに引用符で囲んだ自由語。ID かタイトルに全部の語が入るもの。全 Store）の結果を足す。重複は ULID で除き、最大 20 件。返ってきたタスクは `TaskCache.applyWrite({ tasks })` でキャッシュに入れる（詳細を開く前に下の「…に対して」のコマンドが使えるように）
3. **クエリ**: 入力をそのまま `GET /tasks?q=<入力>&tree=false` に渡し、「クエリに一致 · n 件」のグループにする（最大 20 件を出す。n は全件の数）。`query.invalid` などは `errorMessage` の文言と、サーバーの `detail` を出す。canvas の「このクエリで」（テーブルで開く・ビューとして保存・まとめて更新）は 4e・6 の機能なので出さない
4. **コマンド**（`>` か、モードがコマンドのとき。入力が空のときも）: 「コマンド」（今すぐ同期する ⇧⌘S、タスクを作成 C、プロジェクトを作成）、「最近開いたもの」（タスク詳細を開いた順に最大8件。このブラウザの `localStorage` の `hedgehug.recent` に ULID で持つ。読み書きできなくても動く）、「開く」（ワークスペースと各プロジェクトを開く）、選んでいるタスクがあれば「<ID> に対して」（ステータスを変更…、子タスクを作る C、詳細を開く ⌘↵）。入力の `>` の後ろの語で、ラベルと補足（最近開いたものは ID とタイトル）の部分一致（大文字小文字を問わない）で絞る。設定・doctor・Store の探索などの 4e 以降の画面へのコマンドと、`hh start`（ブランチを作って始める）は出さない
5. **ID・検索の結果の下の「<ID> に対して」**: 結果の最初のタスクに対するコマンド（canvas の「REL-6 に対して」）を結果の下に足す。矢印で下りて選べる
6. **「ステータスを変更…」**: パレットを閉じずに中身をステータスの一覧（そのタスクの Store の `statuses`）に替え、チップを「ステータス」にする。選ぶと `PATCH`（`status`）、Esc でパレットの最初に戻る
7. **選んでいるタスク**: ツリーで選んでいる行（`selection.ulid`）、なければタスク詳細を開いているときそのタスク
8. **⌘↵（新しいタブ）**: `window.open(<URL>, "_blank")`。`noopener` は付けない（ブラウザが `sessionStorage` を複製して開くので、トークンが新しいタブに引き継がれる。`noopener` を付けるとトークンがなくなり「hh ui から開いてください」の画面になる）。URL を持たないコマンド（同期など）は ↵ と同じ
9. **ほかのタスクとの境目**: `app/palette-state.svelte.ts`（`palette.open`・`openPalette()`・`closePalette()`）はこの Task が置く。タスク・プロジェクトの作成は `app/create-state.svelte.ts` の `openCreate({ kind: "task", parent? })`・`openCreate({ kind: "project" })`（Task 13 が置く）を呼ぶ

**Files:**
- Create: `web/app/palette/search.ts`
- Create: `web/app/palette/recent.ts`
- Create: `web/app/palette/CommandPalette.svelte`
- Create: `web/app/palette-state.svelte.ts`
- Modify: `web/app/App.svelte`（`<CommandPalette />` を1つ置く）
- Modify: `web/app/sidebar/Sidebar.svelte`（検索の欄と ⌘K）
- Test: `web/app/palette/search.test.ts`、`web/app/palette/recent.test.ts`、`web/app/palette/CommandPalette.test.ts`、`web/app/sidebar/Sidebar.test.ts`

**Interfaces:**
- Consumes: `Api`・`unwrap`・`TaskView`（`core/api/client`）、`searchTasks`（`core/search`。この Task では `findTasks` の名前で import する）、`errorMessage`、`TaskCache.applyWrite`・`task`、`StoreDirectory.workspaces()`・`tone`、`WorkspaceEntry`、`SchemaCache.get`、`core()`、`useSubscribe`、`router`（`go`・`href`・`route`）、`isMac`・`kbdLabel`・`modKey`（`app/keys`）、`toast`、`selection`、`palette`・`closePalette`（`app/palette-state.svelte`。この Task が置く）、`openCreate`（`app/create-state.svelte`。Task 13 が置く）、Task 9 の `patchTask`（`tree/actions`）と `tree/fixtures`（テストだけ）、`ui/StoreBadge`・`icons/Icon`（この Task が使う名前は `search` `sync` `plus` `status` `folder` `workspace`）・`icons/KindIcon`
- Produces:
  - `detectMode`・`searchTasks`・`buildCommands`・`taskCommands` と型 `PaletteItem`・`PaletteGroup`・`CommandEnv`・`SearchDeps`（`palette/search.ts`）
  - `readRecent`・`rememberTask`（`palette/recent.ts`）
  - 4e 以降のコマンドは `buildCommands` の「コマンド」グループに足す

- [ ] **Step 1: 失敗するテストを書く**

`web/app/palette/search.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import type { WorkspaceEntry } from "../../core/stores";
import { task } from "../tree/fixtures";
import {
  buildCommands,
  type CommandEnv,
  detectMode,
  type SearchDeps,
  searchTasks,
  taskCommands,
} from "./search";

describe("detectMode", () => {
  it.each([
    [">同期", "command"],
    ["  > sync", "command"],
    [">", "command"],
    ["status:doing assignee:@me", "query"],
    ["is:blocked", "query"],
    ["-status:canceled", "query"],
    ["due<2026-11-10", "query"],
    ["points>=3", "query"],
    ["sh-17", "id"],
    ["REL-6", "id"],
    ["01JC8ZK3Q4M5N6P7R8S9T0V1W2", "id"],
    ["API の破壊的変更", "id"],
    ["", "id"],
    ["   ", "id"],
  ])("%j is %s", (input, mode) => {
    expect(detectMode(input)).toBe(mode);
  });
});

const ok = (data: unknown) => ({ data, response: new Response(null, { status: 200 }) });
const notFound = {
  error: { type: "task.not_found", title: "Not Found", status: 404, detail: "not found" },
  response: new Response(null, { status: 404 }),
};

const rel6 = {
  ulid: "U6",
  id: "REL-6",
  title: "Web の API クライアントを v2 に更新",
  kind: "task",
  store: "shared",
  former_ids: ["SH-17"],
};

function deps(over: { tasks?: TaskView[]; listError?: unknown } = {}) {
  const GET = vi.fn(
    async (
      path: string,
      opts: { params: { path?: { id: string }; query?: { store?: string; q?: string } } },
    ) => {
      if (path === "/api/tasks/{id}") {
        const id = opts.params.path?.id.toLowerCase();
        const hit = id === "sh-17" || id === "rel-6" || id === "u6";
        return hit ? ok(rel6) : notFound;
      }
      if (over.listError) return over.listError;
      return ok({ tasks: over.tasks ?? [] });
    },
  );
  const prime = vi.fn();
  const d: SearchDeps = {
    api: { GET } as unknown as SearchDeps["api"],
    hrefOf: (u) => `/tasks/${u}`,
    prime,
  };
  return { d, GET, prime };
}

describe("searchTasks: ID and text", () => {
  it("finds a task by a former ID and says so", async () => {
    const { d } = deps();
    const r = await searchTasks(d, "sh-17", "id");
    expect(r.error).toBeNull();
    expect(r.groups).toHaveLength(1);
    expect(r.groups[0]?.name).toBe("タスク");
    expect(r.groups[0]?.items[0]).toMatchObject({
      type: "task",
      ulid: "U6",
      id: "REL-6",
      store: "shared",
      note: "旧 ID SH-17 で一致",
      href: "/tasks/U6",
    });
  });

  it("finds a task by its ID without a note, in any case", async () => {
    const { d } = deps();
    const r = await searchTasks(d, "rel-6", "id");
    expect(r.groups[0]?.items[0]).toMatchObject({ id: "REL-6" });
    expect(r.groups[0]?.items[0]).not.toHaveProperty("note");
  });

  it("looks the ID up once, in every store, and ignores an ID nobody has", async () => {
    const { d, GET } = deps();
    const r = await searchTasks(d, "REL-99", "id");
    expect(r.error).toBeNull();
    expect(r.groups).toEqual([]);
    expect(GET).toHaveBeenCalledWith("/api/tasks/{id}", { params: { path: { id: "REL-99" } } });
  });

  it("searches the words of the ID and title and adds what it finds to the cache", async () => {
    const found = [task("U9", "task", null, { id: "REL-9", title: "API v2 の移行ガイド" })];
    const { d, GET, prime } = deps({ tasks: found });
    const r = await searchTasks(d, "api v2", "id");
    expect(GET).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { q: '"api" "v2"', store: undefined } },
    });
    expect(prime).toHaveBeenCalledWith(found);
    expect(r.groups[0]?.items.map((i) => (i.type === "task" ? i.id : ""))).toEqual(["REL-9"]);
  });

  it("does not list a task twice and keeps the match by ID first", async () => {
    const found = [
      task("U6", "task", null, { id: "REL-6", title: "Web の API クライアントを v2 に更新" }),
      task("U9", "task", null, { id: "REL-9", title: "x" }),
    ];
    const { d } = deps({ tasks: found });
    const r = await searchTasks(d, "REL-6", "id");
    expect(r.groups[0]?.items.map((i) => (i.type === "task" ? i.ulid : ""))).toEqual(["U6", "U9"]);
  });

  it("returns nothing for an empty input and no group when nothing matches", async () => {
    const { d, GET } = deps();
    expect(await searchTasks(d, "  ", "id")).toEqual({ groups: [], error: null });
    expect(GET).not.toHaveBeenCalled();
    expect((await searchTasks(d, "zzz", "id")).groups).toEqual([]);
  });

  it("stops at 20 results", async () => {
    const many = Array.from({ length: 30 }, (_, i) =>
      task(`M${i}`, "task", null, { id: `REL-${i}` }),
    );
    const { d } = deps({ tasks: many });
    const r = await searchTasks(d, "x", "id");
    expect(r.groups[0]?.items).toHaveLength(20);
  });
});

describe("searchTasks: query", () => {
  it("passes the query as it is and names the group with the count", async () => {
    const found = [task("U1", "task", null), task("U2", "task", null)];
    const { d, GET } = deps({ tasks: found });
    const r = await searchTasks(d, "status:doing assignee:@me", "query");
    expect(GET).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { q: "status:doing assignee:@me", tree: false } },
    });
    expect(r.groups).toHaveLength(1);
    expect(r.groups[0]?.name).toBe("クエリに一致 · 2 件");
  });

  it("returns the error of a query that does not run", async () => {
    const { d } = deps({
      listError: {
        error: {
          type: "query.invalid",
          title: "Bad",
          status: 400,
          detail: "term 1 (foo:bar): unknown key",
        },
        response: new Response(null, { status: 400 }),
      },
    });
    const r = await searchTasks(d, "foo:bar", "query");
    expect(r.groups).toEqual([]);
    expect(r.error).toMatchObject({ type: "query.invalid" });
  });
});

const ws: WorkspaceEntry = {
  store: "shared",
  tone: "purple",
  ulid: "W1",
  name: "release",
  prefix: "REL",
  projects: [task("P1", "project", null, { id: "REL-15", title: "リリース準備 v2.4" })],
};

function env(over: Partial<CommandEnv> = {}): CommandEnv {
  return {
    mac: true,
    syncNow: vi.fn(),
    createTask: vi.fn(),
    createProject: vi.fn(),
    workspaces: [ws],
    go: vi.fn(),
    hrefs: {
      workspace: (s, u) => `/workspaces/${s}/${u}`,
      project: (u) => `/projects/${u}`,
      task: (u) => `/tasks/${u}`,
    },
    selected: null,
    recent: [],
    pickStatus: vi.fn(),
    ...over,
  };
}

const labels = (gs: ReturnType<typeof buildCommands>) =>
  gs.flatMap((g) => g.items.map((i) => (i.type === "command" ? i.label : i.title)));

describe("buildCommands", () => {
  it("lists the commands, then where to go", () => {
    const gs = buildCommands(env(), ">");
    expect(gs.map((g) => g.name)).toEqual(["コマンド", "開く"]);
    expect(labels(gs)).toEqual([
      "今すぐ同期する",
      "タスクを作成",
      "プロジェクトを作成",
      "release を開く",
      "リリース準備 v2.4 を開く",
    ]);
  });

  it("lists the tasks opened last, filtered by the words", () => {
    const e = env({
      recent: [task("R1", "task", null, { id: "REL-3", title: "移行ガイド" })],
    });
    const gs = buildCommands(e, ">");
    expect(gs.map((g) => g.name)).toEqual(["コマンド", "最近開いたもの", "開く"]);
    expect(gs[1]?.items[0]).toMatchObject({ type: "task", id: "REL-3", href: "/tasks/R1" });
    expect(buildCommands(e, ">ガイド").map((g) => g.name)).toEqual(["最近開いたもの"]);
  });

  it("filters by the words after >", () => {
    expect(labels(buildCommands(env(), ">同期"))).toEqual(["今すぐ同期する"]);
    expect(labels(buildCommands(env(), "> リリース"))).toEqual(["リリース準備 v2.4 を開く"]);
    expect(buildCommands(env(), ">zzz")).toEqual([]);
  });

  it("writes the shortcut for the platform", () => {
    const mac = buildCommands(env({ mac: true }), ">同期")[0]?.items[0];
    const other = buildCommands(env({ mac: false }), ">同期")[0]?.items[0];
    expect(mac).toMatchObject({ kbd: "⇧⌘S" });
    expect(other).toMatchObject({ kbd: "Shift+Ctrl+S" });
  });

  it("runs a command and goes to a workspace or a project with a URL for a new tab", () => {
    const e = env();
    const gs = buildCommands(e, ">");
    const sync = gs[0]?.items[0];
    if (sync?.type === "command") sync.run();
    expect(e.syncNow).toHaveBeenCalled();
    const proj = gs[1]?.items[1];
    expect(proj).toMatchObject({ href: "/projects/P1", store: "shared" });
    if (proj?.type === "command") proj.run();
    expect(e.go).toHaveBeenCalledWith("/projects/P1");
  });

  it("adds the commands for the selected task", () => {
    const t = task("T1", "task", null, { id: "REL-6" });
    const gs = buildCommands(env({ selected: t }), ">");
    expect(gs.map((g) => g.name)).toEqual(["コマンド", "開く", "REL-6 に対して"]);
  });
});

describe("taskCommands", () => {
  it("changes the status in the palette, creates a child, and opens the detail", () => {
    const e = env();
    const t = task("T1", "task", null, { id: "REL-6" });
    const g = taskCommands(e, t);
    expect(g.name).toBe("REL-6 に対して");
    const [status, child, detail] = g.items;
    expect(status).toMatchObject({ label: "ステータスを変更…", keepOpen: true });
    expect(child).toMatchObject({ label: "子タスクを作る", kbd: "C" });
    expect(detail).toMatchObject({ label: "詳細を開く", kbd: "⌘↵", href: "/tasks/T1" });
    if (status?.type === "command") status.run();
    expect(e.pickStatus).toHaveBeenCalledWith(t);
    if (child?.type === "command") child.run();
    expect(e.createTask).toHaveBeenCalledWith("T1");
    if (detail?.type === "command") detail.run();
    expect(e.go).toHaveBeenCalledWith("/tasks/T1");
  });
});
```

`web/app/palette/CommandPalette.test.ts`:

```ts
import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { closePalette, openPalette, palette } from "../palette-state.svelte";
import { selection } from "../selection.svelte";
import { schema, task } from "../tree/fixtures";

const h = vi.hoisted(() => ({
  core: undefined as unknown,
  go: vi.fn(),
  toast: vi.fn(),
}));

vi.mock("../context", () => ({ core: () => h.core }));
vi.mock("../router", () => ({
  router: {
    route: { name: "home" },
    go: h.go,
    href: (r: { name: string; ulid?: string; store?: string }) =>
      r.name === "task"
        ? `/tasks/${r.ulid}`
        : r.name === "project"
          ? `/projects/${r.ulid}`
          : `/workspaces/${r.store}/${r.ulid}`,
  },
}));
vi.mock("../toast.svelte", () => ({ toast: h.toast }));

import CommandPalette from "./CommandPalette.svelte";

const ok = (data: unknown) => ({ data, response: new Response(null, { status: 200 }) });
const rel6 = task("U6", "task", null, {
  id: "REL-6",
  title: "Web の API クライアントを v2 に更新",
});

function setup() {
  const none = () => () => {};
  const ws = {
    store: "shared",
    tone: "purple",
    ulid: "W1",
    name: "release",
    prefix: "REL",
    projects: [],
  };
  const api = {
    GET: vi.fn(
      async (
        path: string,
        opts: { params: { path?: { id: string }; query?: { store?: string; q?: string } } },
      ) => {
        if (path === "/api/tasks/{id}") {
          const hit = opts.params.path?.id.toLowerCase() === "sh-17";
          return hit
            ? ok({ ...rel6, former_ids: ["SH-17"] })
            : {
                error: { type: "task.not_found", title: "NF", status: 404 },
                response: new Response(null, { status: 404 }),
              };
        }
        if (opts.params.query?.q?.startsWith("foo:")) {
          return {
            error: {
              type: "query.invalid",
              title: "Bad",
              status: 400,
              detail: "term 1 (foo:bar): unknown key",
            },
            response: new Response(null, { status: 400 }),
          };
        }
        return ok({ tasks: [] });
      },
    ),
    POST: vi.fn(async () => ok([])),
    PATCH: vi.fn(async () => ok({ task: { ...rel6, status: "done" }, commit: "c1" })),
  };
  h.core = {
    api,
    cache: {
      task: (u: string) => (u === "U6" ? rel6 : undefined),
      applyWrite: vi.fn(),
      subscribe: none,
    },
    stores: {
      stores: () => [{ name: "shared" }, { name: "backend" }],
      workspaces: () => [ws],
      tone: () => "purple",
      subscribe: none,
    },
    schema: { get: () => schema, load: async () => schema, subscribe: none },
  };
  return { api };
}

const box = () => screen.getByRole("combobox");

beforeEach(() => {
  h.go.mockReset();
  h.toast.mockReset();
  localStorage.clear();
  closePalette();
  selection.set(null);
});

describe("CommandPalette", () => {
  it("renders nothing while closed and shows the dialog when opened", async () => {
    setup();
    render(CommandPalette);
    expect(screen.queryByRole("dialog")).toBeNull();
    openPalette();
    expect(await screen.findByRole("dialog", { name: "ID・検索・コマンド" })).toBeInTheDocument();
  });

  it("lists the tasks opened last when the input is empty", async () => {
    setup();
    localStorage.setItem("hedgehug.recent", JSON.stringify(["U6"]));
    render(CommandPalette);
    openPalette();
    expect(await screen.findByText("最近開いたもの")).toBeInTheDocument();
    expect(screen.getByText("Web の API クライアントを v2 に更新")).toBeInTheDocument();
  });

  it("closes with Escape", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.keyDown(await screen.findByRole("combobox"), { key: "Escape" });
    await waitFor(() => expect(palette.open).toBe(false));
  });

  it("runs a command: > narrows to the commands and Enter syncs", async () => {
    const { api } = setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">同期" } });
    expect(await screen.findByText("今すぐ同期する")).toBeInTheDocument();
    expect(screen.getByText("コマンド", { selector: "[data-mode]" })).toBeInTheDocument();
    await fireEvent.keyDown(box(), { key: "Enter" });
    await waitFor(() => expect(api.POST).toHaveBeenCalledWith("/api/sync", {}));
    expect(palette.open).toBe(false);
  });

  it("finds a task by a former ID, says so, and opens it with Enter", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "sh-17" } });
    expect(await screen.findByText("旧 ID SH-17 で一致")).toBeInTheDocument();
    expect(screen.getByText("REL-6")).toBeInTheDocument();
    expect(screen.getByText("REL-6 に対して")).toBeInTheDocument();
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(h.go).toHaveBeenCalledWith("/tasks/U6");
    expect(palette.open).toBe(false);
  });

  it("opens the result in a new tab with the modifier", async () => {
    setup();
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "sh-17" } });
    await screen.findByText("旧 ID SH-17 で一致");
    await fireEvent.keyDown(box(), { key: "Enter", ctrlKey: true, metaKey: true });
    expect(open).toHaveBeenCalledWith(expect.stringContaining("/tasks/U6"), "_blank");
    expect(h.go).not.toHaveBeenCalled();
    open.mockRestore();
  });

  it("goes down to the commands for the task and changes its status in the palette", async () => {
    const { api } = setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "sh-17" } });
    await screen.findByText("REL-6 に対して");
    await fireEvent.keyDown(box(), { key: "ArrowDown" });
    expect(screen.getByRole("option", { selected: true })).toHaveTextContent("ステータスを変更…");
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(palette.open).toBe(true);
    expect(screen.getByText("ステータス", { selector: "[data-mode]" })).toBeInTheDocument();
    await fireEvent.click(await screen.findByRole("option", { name: /done/ }));
    await waitFor(() => expect(api.PATCH).toHaveBeenCalled());
    expect(api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
      body: { status: "done" },
    });
    expect(palette.open).toBe(false);
  });

  it("wraps the selection around with the arrow keys", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await screen.findByRole("combobox");
    await fireEvent.keyDown(box(), { key: "ArrowUp" });
    const options = screen.getAllByRole("option");
    expect(options[options.length - 1]).toHaveAttribute("aria-selected", "true");
  });

  it("shows the message of a query that does not run", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "foo:bar" } });
    expect(await screen.findByRole("alert")).toHaveTextContent("term 1 (foo:bar): unknown key");
    expect(screen.getByText("クエリ", { selector: "[data-mode]" })).toBeInTheDocument();
  });

  it("offers the commands for the selected task", async () => {
    setup();
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    expect(await screen.findByText("REL-6 に対して")).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("option", { name: /子タスクを作る/ }));
    expect(createState.request).toEqual({ kind: "task", parent: "U6" });
  });
});
```

`web/app/palette/recent.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { readRecent, rememberTask } from "./recent";

function memory(initial?: string): Storage {
  const m = new Map<string, string>(initial ? [["hedgehug.recent", initial]] : []);
  return {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => void m.set(k, v),
  } as unknown as Storage;
}

describe("recent tasks", () => {
  it("puts the latest first, once, and keeps eight", () => {
    const s = memory();
    for (const u of ["a", "b", "c", "a", "d", "e", "f", "g", "h", "i"]) rememberTask(u, s);
    expect(readRecent(s)).toEqual(["i", "h", "g", "f", "e", "d", "a", "c"]);
  });

  it("is empty when nothing is stored, when the value is not a list, or when it cannot be read", () => {
    expect(readRecent(memory())).toEqual([]);
    expect(readRecent(memory("{}"))).toEqual([]);
    expect(readRecent(memory('[1,"a"]'))).toEqual(["a"]);
    expect(readRecent(memory("not json"))).toEqual([]);
    expect(
      readRecent({
        getItem() {
          throw new Error("blocked");
        },
      }),
    ).toEqual([]);
  });

  it("does not throw when the storage cannot be written", () => {
    const s = {
      getItem: () => null,
      setItem() {
        throw new Error("quota");
      },
    };
    expect(() => rememberTask("a", s)).not.toThrow();
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm vitest run app/palette`
Expected: FAIL（`./search`・`./CommandPalette.svelte` がない）

- [ ] **Step 3: 実装する**

`web/app/palette/search.ts`:

```ts
import { type Api, type TaskView, unwrap } from "../../core/api/client";
import { searchTasks as findTasks } from "../../core/search";
import type { WorkspaceEntry } from "../../core/stores";

export type PaletteMode = "id" | "query" | "command";

// A word of a query that has no colon: a comparison such as due<2026-11-10.
const COMPARISON = /^-?[A-Za-z_][\w-]*(<=|>=|<|>)\S/;
const ID = /^[A-Za-z][A-Za-z0-9]*-\d+$/;
const ULID = /^[0-9A-HJKMNP-TV-Za-hjkmnp-tv-z]{26}$/;
const MAX = 20;

// detectMode tells which of the three the input is. IDs, former IDs, ULIDs and
// titles are one mode: the same input is looked up as all of them.
export function detectMode(input: string): PaletteMode {
  const s = input.trimStart();
  if (s.startsWith(">")) return "command";
  const words = s.trim().split(/\s+/).filter(Boolean);
  if (words.some((w) => w.includes(":") || COMPARISON.test(w))) return "query";
  return "id";
}

export type TaskItem = {
  type: "task";
  key: string;
  ulid: string;
  id: string;
  title: string;
  kind: string;
  store: string;
  note?: string;
  href: string;
};

export type CommandItem = {
  type: "command";
  key: string;
  label: string;
  icon: "sync" | "plus" | "status" | "folder" | "workspace";
  kbd?: string;
  note?: string;
  store?: string;
  href?: string; // where it goes, for a new tab
  keepOpen?: boolean; // the palette stays open and shows something else
  run: () => void;
};

export type PaletteItem = TaskItem | CommandItem;
export type PaletteGroup = { name: string; items: PaletteItem[] };

type TaskLike = { ulid: string; id: string; title: string; kind: string; store: string };

function taskItem(t: TaskLike, hrefOf: (ulid: string) => string, note?: string): TaskItem {
  return {
    type: "task",
    key: `task:${t.ulid}`,
    ulid: t.ulid,
    id: t.id,
    title: t.title,
    kind: t.kind,
    store: t.store,
    href: hrefOf(t.ulid),
    ...(note ? { note } : {}),
  };
}

export type SearchDeps = {
  api: Api;
  hrefOf: (ulid: string) => string;
  // Puts what the search found in the cache.
  prime: (tasks: TaskView[]) => void;
};

export type SearchResult = { groups: PaletteGroup[]; error: unknown };

function isNotFound(e: unknown): boolean {
  return typeof e === "object" && e !== null && (e as { status?: number }).status === 404;
}

// matchNote says how an exact lookup matched when it was not by the current ID.
function matchNote(s: { id: string; ulid: string; former_ids?: string[] | null }, text: string) {
  const t = text.toLowerCase();
  if (s.id.toLowerCase() === t || s.ulid.toLowerCase() === t) return undefined;
  if ((s.former_ids ?? []).some((f) => f.toLowerCase() === t)) {
    return `旧 ID ${text.toUpperCase()} で一致`;
  }
  return undefined;
}

// searchTasks looks the input up in every store: as an ID, a former ID or a
// ULID (an exact lookup, first), and as words of an ID or a title; or, for the
// query mode, runs it as a query.
export async function searchTasks(
  d: SearchDeps,
  input: string,
  mode: "id" | "query",
): Promise<SearchResult> {
  const text = input.trim();
  if (text === "") return { groups: [], error: null };
  try {
    if (mode === "query") {
      const list = await unwrap(
        d.api.GET("/api/tasks", { params: { query: { q: text, tree: false } } }),
      );
      const tasks = list.tasks ?? [];
      d.prime(tasks);
      const items = tasks.slice(0, MAX).map((t) => taskItem(t, d.hrefOf));
      return {
        groups: items.length ? [{ name: `クエリに一致 · ${tasks.length} 件`, items }] : [],
        error: null,
      };
    }
    const found = new Map<string, TaskItem>();
    if (ID.test(text) || ULID.test(text)) {
      try {
        const s = await unwrap(d.api.GET("/api/tasks/{id}", { params: { path: { id: text } } }));
        found.set(s.ulid, taskItem(s, d.hrefOf, matchNote(s, text)));
      } catch (e) {
        if (!isNotFound(e)) throw e;
      }
    }
    const tasks = await findTasks(d.api, text, { limit: MAX });
    d.prime(tasks);
    for (const t of tasks) {
      if (!found.has(t.ulid)) found.set(t.ulid, taskItem(t, d.hrefOf));
    }
    const items = [...found.values()].slice(0, MAX);
    return { groups: items.length ? [{ name: "タスク", items }] : [], error: null };
  } catch (e) {
    return { groups: [], error: e };
  }
}

export type CommandEnv = {
  mac: boolean;
  syncNow: () => void;
  createTask: (parent?: string) => void;
  createProject: () => void;
  workspaces: WorkspaceEntry[];
  go: (path: string) => void;
  hrefs: {
    workspace: (store: string, ulid: string) => string;
    project: (ulid: string) => string;
    task: (ulid: string) => string;
  };
  selected: TaskView | null; // the task the commands for a task are about
  recent: TaskView[]; // the tasks opened last, the latest first
  pickStatus: (t: TaskView) => void;
};

// taskCommands are the commands about one task.
export function taskCommands(env: CommandEnv, t: TaskView): PaletteGroup {
  const href = env.hrefs.task(t.ulid);
  return {
    name: `${t.id} に対して`,
    items: [
      {
        type: "command",
        key: `task-status:${t.ulid}`,
        label: "ステータスを変更…",
        icon: "status",
        keepOpen: true,
        run: () => env.pickStatus(t),
      },
      {
        type: "command",
        key: `task-child:${t.ulid}`,
        label: "子タスクを作る",
        icon: "plus",
        kbd: "C",
        run: () => env.createTask(t.ulid),
      },
      {
        type: "command",
        key: `task-open:${t.ulid}`,
        label: "詳細を開く",
        icon: "folder",
        kbd: env.mac ? "⌘↵" : "Ctrl+↵",
        href,
        run: () => env.go(href),
      },
    ],
  };
}

// buildCommands lists the commands, then where to go, then the commands for
// the selected task. text is the input, with or without the leading >; a word
// in it narrows the list by its label and note.
export function buildCommands(env: CommandEnv, text: string): PaletteGroup[] {
  const word = text.replace(/^\s*>/, "").trim().toLowerCase();
  const keep = (i: CommandItem) =>
    word === "" || `${i.label} ${i.note ?? ""}`.toLowerCase().includes(word);

  const commands: CommandItem[] = [
    {
      type: "command",
      key: "cmd:sync",
      label: "今すぐ同期する",
      icon: "sync",
      kbd: env.mac ? "⇧⌘S" : "Shift+Ctrl+S",
      run: env.syncNow,
    },
    {
      type: "command",
      key: "cmd:create-task",
      label: "タスクを作成",
      icon: "plus",
      kbd: "C",
      run: () => env.createTask(),
    },
    {
      type: "command",
      key: "cmd:create-project",
      label: "プロジェクトを作成",
      icon: "plus",
      run: env.createProject,
    },
  ];

  const places: CommandItem[] = env.workspaces.flatMap((w) => {
    const wsHref = env.hrefs.workspace(w.store, w.ulid);
    const ws: CommandItem = {
      type: "command",
      key: `open:ws:${w.store}:${w.ulid}`,
      label: `${w.name} を開く`,
      icon: "workspace",
      store: w.store,
      href: wsHref,
      run: () => env.go(wsHref),
    };
    const projects = w.projects.map((p): CommandItem => {
      const href = env.hrefs.project(p.ulid);
      return {
        type: "command",
        key: `open:project:${p.ulid}`,
        label: `${p.title} を開く`,
        icon: "folder",
        note: w.name,
        store: w.store,
        href,
        run: () => env.go(href),
      };
    });
    return [ws, ...projects];
  });

  const recent = env.recent
    .filter((t) => word === "" || `${t.id} ${t.title}`.toLowerCase().includes(word))
    .map((t) => taskItem(t, env.hrefs.task));

  const groups: PaletteGroup[] = [
    { name: "コマンド", items: commands.filter(keep) },
    { name: "最近開いたもの", items: recent },
    { name: "開く", items: places.filter(keep) },
  ];
  if (env.selected) {
    const g = taskCommands(env, env.selected);
    groups.push({ ...g, items: g.items.filter((i) => i.type === "command" && keep(i)) });
  }
  return groups.filter((g) => g.items.length > 0);
}
```

`web/app/palette/recent.ts`:

```ts
const KEY = "hedgehug.recent";
const MAX = 8;

// readRecent is the ULIDs of the tasks opened last, the latest first. It is a
// convenience of this browser: when the storage cannot be read it is empty.
export function readRecent(storage: Pick<Storage, "getItem"> = localStorage): string[] {
  try {
    const v: unknown = JSON.parse(storage.getItem(KEY) ?? "[]");
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : [];
  } catch {
    return [];
  }
}

// rememberTask puts a task at the top of the list, once, and keeps the last eight.
export function rememberTask(
  ulid: string,
  storage: Pick<Storage, "getItem" | "setItem"> = localStorage,
): void {
  try {
    const next = [ulid, ...readRecent(storage).filter((u) => u !== ulid)].slice(0, MAX);
    storage.setItem(KEY, JSON.stringify(next));
  } catch {
    // The list is a convenience; losing it is fine.
  }
}
```

`web/app/palette/CommandPalette.svelte`:

```svelte
<script lang="ts">
  import { tick } from "svelte";
  import { ApiError, type TaskView, unwrap } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import { isMac, kbdLabel, modKey } from "../keys";
  import { closePalette, palette } from "../palette-state.svelte";
  import { router } from "../router";
  import { selection } from "../selection.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import { patchTask } from "../tree/actions";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { readRecent, rememberTask } from "./recent";
  import {
    buildCommands,
    type CommandEnv,
    detectMode,
    type PaletteGroup,
    type PaletteItem,
    searchTasks,
    taskCommands,
  } from "./search";

  const c = core();
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));

  let input = $state("");
  let active = $state(0);
  let picking = $state<TaskView | null>(null);
  let found = $state<{ groups: PaletteGroup[]; error: unknown }>({ groups: [], error: null });
  let inputEl = $state<HTMLInputElement | null>(null);
  let listEl = $state<HTMLDivElement | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  // The tasks opened last, for the empty palette. The sidebar owns the shortcut that opens it.
  let recentUlids = $state(readRecent());
  $effect(() => {
    if (router.route.name !== "task") return;
    rememberTask(router.route.ulid);
    recentUlids = readRecent();
  });

  // Start from an empty palette each time it opens.
  $effect(() => {
    if (!palette.open) return;
    input = "";
    active = 0;
    picking = null;
    found = { groups: [], error: null };
    void tick().then(() => inputEl?.focus());
  });

  const mode = $derived(detectMode(input));
  const modeLabel = $derived(
    picking ? "ステータス" : { id: "ID", query: "クエリ", command: "コマンド" }[mode],
  );

  const deps = {
    api: c.api,
    hrefOf: (ulid: string) => router.href({ name: "task", ulid }),
    prime: (tasks: TaskView[]) => c.cache.applyWrite({ tasks }),
  };

  $effect(() => {
    const text = input;
    const m = mode;
    clearTimeout(timer);
    seq++;
    if (!palette.open || picking || m === "command" || text.trim() === "") {
      found = { groups: [], error: null };
      return;
    }
    const mine = seq;
    timer = setTimeout(async () => {
      const r = await searchTasks(deps, text, m);
      if (mine === seq) found = r;
    }, 150);
    return () => clearTimeout(timer);
  });

  async function syncNow() {
    try {
      await unwrap(c.api.POST("/api/sync", {}));
      toast({ message: "同期しました" });
    } catch (e) {
      toast({ message: errorMessage(e) });
    }
  }

  async function setStatus(t: TaskView, status: string) {
    try {
      await patchTask(c, t, { status });
    } catch (e) {
      toast({ message: errorMessage(e) });
    }
  }

  function pickStatus(t: TaskView) {
    picking = t;
    input = "";
    active = 0;
  }

  const env = $derived.by<CommandEnv>(() => {
    cacheRev.current;
    storesRev.current;
    const sel = selection.ulid ?? (router.route.name === "task" ? router.route.ulid : null);
    return {
      mac: isMac(),
      syncNow: () => void syncNow(),
      createTask: (parent) => openCreate({ kind: "task", parent }),
      createProject: () => openCreate({ kind: "project" }),
      workspaces: c.stores.workspaces(),
      go: (path) => router.go(path),
      hrefs: {
        workspace: (store, ulid) => router.href({ name: "workspace", store, ulid }),
        project: (ulid) => router.href({ name: "project", ulid }),
        task: (ulid) => router.href({ name: "task", ulid }),
      },
      selected: sel ? (c.cache.task(sel) ?? null) : null,
      recent: recentUlids.flatMap((u) => c.cache.task(u) ?? []),
      pickStatus,
    };
  });

  function statusGroups(t: TaskView): PaletteGroup[] {
    const word = input.trim().toLowerCase();
    const items = (c.schema.get(t.store)?.statuses ?? [])
      .filter(
        (s) =>
          word === "" || (s.label || s.name).toLowerCase().includes(word) || s.name.includes(word),
      )
      .map((s) => ({
        type: "command" as const,
        key: `status:${s.name}`,
        label: s.label && s.label !== s.name ? `${s.label}（${s.name}）` : s.name,
        icon: "status" as const,
        note: s.name === t.status ? "今のステータス" : undefined,
        run: () => void setStatus(t, s.name),
      }));
    return items.length ? [{ name: `${t.id} のステータス`, items }] : [];
  }

  const groups = $derived.by<PaletteGroup[]>(() => {
    cacheRev.current;
    if (picking) return statusGroups(picking);
    if (mode === "command" || input.trim() === "") return buildCommands(env, input);
    const out = [...found.groups];
    const first = found.groups.flatMap((g) => g.items).find((i) => i.type === "task");
    if (mode === "id" && first?.type === "task") {
      const t = c.cache.task(first.ulid);
      if (t) out.push(taskCommands(env, t));
    }
    return out;
  });

  const view = $derived.by(() => {
    let n = 0;
    return groups.map((g) => ({
      name: g.name,
      items: g.items.map((item) => ({ item, index: n++ })),
    }));
  });
  const flat = $derived(groups.flatMap((g) => g.items));
  const activeKey = $derived(flat[active]?.key);
  const empty = $derived(flat.length === 0 && input.trim() !== "" && found.error === null);
  const error = $derived(found.error);
  const errorDetail = $derived(
    error instanceof ApiError && error.type === "query.invalid" ? error.detail : null,
  );

  $effect(() => {
    if (active >= flat.length) active = Math.max(0, flat.length - 1);
  });

  function activate(it: PaletteItem, newTab: boolean) {
    if (newTab && it.href) {
      // No noopener: the new tab gets a copy of sessionStorage, which holds the token.
      window.open(new URL(it.href, window.location.origin).href, "_blank");
      closePalette();
      return;
    }
    if (it.type === "task") {
      closePalette();
      router.go(it.href);
      return;
    }
    if (!it.keepOpen) closePalette();
    it.run();
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.isComposing) return;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (flat.length === 0) return;
      active = (active + (e.key === "ArrowDown" ? 1 : -1) + flat.length) % flat.length;
      void tick().then(() =>
        listEl?.querySelector('[aria-selected="true"]')?.scrollIntoView?.({ block: "nearest" }),
      );
    } else if (e.key === "Enter") {
      e.preventDefault();
      const it = flat[active];
      if (it) activate(it, modKey(e));
    } else if (e.key === "Escape") {
      e.preventDefault();
      if (picking) {
        picking = null;
        input = "";
      } else {
        closePalette();
      }
    }
  }
</script>

{#if palette.open}
  <!-- biome-ignore lint/a11y/noStaticElementInteractions: a click on the backdrop closes the dialog; the keyboard closes it with Esc -->
  <div
    class="backdrop"
    role="presentation"
    onmousedown={(e) => {
      if (e.target === e.currentTarget) closePalette();
    }}
  >
    <div class="panel" role="dialog" aria-modal="true" aria-label="ID・検索・コマンド">
      <div class="head">
        <Icon name="search" size={16} />
        <input
          bind:this={inputEl}
          type="text"
          role="combobox"
          aria-expanded="true"
          aria-controls="palette-list"
          aria-activedescendant={activeKey ? `palette-${activeKey}` : undefined}
          aria-label="ID・検索・コマンド"
          placeholder="ID・検索・コマンド"
          spellcheck="false"
          autocomplete="off"
          value={input}
          oninput={(e) => {
            input = e.currentTarget.value;
            active = 0;
          }}
          {onkeydown}
        >
        <span class="mode" data-mode={mode}>{modeLabel}</span>
      </div>

      <div class="list" id="palette-list" role="listbox" bind:this={listEl}>
        {#each view as g (g.name)}
          <!-- biome-ignore lint/a11y/useSemanticElements: a group of options in a listbox is a div with role=group -->
          <div class="group" role="group" aria-label={g.name}>
            <div class="gname">{g.name}</div>
            {#each g.items as { item, index } (item.key)}
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <!-- biome-ignore lint/a11y/useKeyWithClickEvents: the keys are handled by the input, which keeps the focus -->
              <div
                class="item"
                class:active={index === active}
                id="palette-{item.key}"
                role="option"
                aria-selected={index === active}
                tabindex="-1"
                onclick={() => activate(item, false)}
                onmousemove={() => (active = index)}
              >
                {#if item.type === "task"}
                  <KindIcon kind={item.kind} />
                  <span class="key">{item.id}</span>
                  <span class="text">{item.title}</span>
                  {#if item.note}
                    <span class="note">{item.note}</span>
                  {/if}
                  <span class="store"
                    ><StoreBadge store={item.store} tone={c.stores.tone(item.store)} /></span
                  >
                {:else}
                  <Icon name={item.icon} size={14} />
                  <span class="text">{item.label}</span>
                  {#if item.note}
                    <span class="note">{item.note}</span>
                  {/if}
                  {#if item.store}
                    <span class="store"
                      ><StoreBadge store={item.store} tone={c.stores.tone(item.store)} /></span
                    >
                  {/if}
                  {#if item.kbd}
                    <kbd>{item.kbd}</kbd>
                  {/if}
                {/if}
              </div>
            {/each}
          </div>
        {/each}
        {#if error}
          <p class="msg error" role="alert">
            {errorMessage(error)}
            {#if errorDetail}
              <code>{errorDetail}</code>
            {/if}
          </p>
        {:else if empty}
          <p class="msg">一致するものがありません</p>
        {/if}
      </div>

      <footer class="foot">
        <span><kbd>↑↓</kbd> 選ぶ</span>
        <span><kbd>↵</kbd> 開く</span>
        <span><kbd>{kbdLabel("mod+enter")}</kbd> 新しいタブで</span>
        <span><kbd>&gt;</kbd> で始めるとコマンドだけ</span>
        <span class="hint">ID・旧 ID・ULID・タイトル・クエリのどれでも</span>
      </footer>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 12vh;
    background: var(--scrim);
  }
  .panel {
    display: flex;
    flex-direction: column;
    width: min(38.75rem, calc(100vw - 2rem));
    max-height: 70vh;
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-8);
    background: var(--surface);
    box-shadow: var(--shadow-popover);
    overflow: hidden;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    padding: 0 0.875rem;
    border-bottom: var(--hairline) solid var(--line);
    color: var(--muted);
  }
  .head input {
    flex: 1;
    min-width: 0;
    height: 3rem;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--ink);
    font: inherit;
    font-size: 0.95rem;
  }
  .mode {
    flex: 0 0 auto;
    padding: 0.125rem 0.5rem;
    border-radius: var(--radius-pill);
    background: var(--hover);
    color: var(--ink-2);
    font-size: 0.72rem;
    font-weight: 500;
  }
  .mode[data-mode="query"] {
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .mode[data-mode="command"] {
    background: var(--ink);
    color: var(--surface);
  }
  .list {
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto;
    padding: 0.375rem;
  }
  .gname {
    padding: 0.5rem 0.625rem 0.25rem;
    color: var(--muted);
    font-size: 0.72rem;
    font-weight: 600;
  }
  .item {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    min-height: 2.25rem;
    padding: 0 0.625rem;
    border-radius: var(--radius-4);
    color: var(--ink);
    cursor: pointer;
  }
  .item.active {
    background: var(--accent-weak);
    box-shadow: inset 0 0 0 var(--hairline) var(--accent-weak);
  }
  .key {
    flex: 0 0 auto;
    font-family: var(--font-mono);
    font-size: 0.72rem;
    color: var(--muted);
  }
  .text {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .note {
    flex: 0 0 auto;
    font-size: 0.72rem;
    color: var(--muted);
  }
  .store {
    flex: 0 0 auto;
  }
  kbd {
    flex: 0 0 auto;
    padding: 0 0.3rem;
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-4);
    background: var(--sunken);
    font-family: var(--font-mono);
    font-size: 0.7rem;
    color: var(--ink-2);
  }
  .msg {
    margin: 0.75rem 0.625rem;
    color: var(--muted);
    font-size: 0.82rem;
  }
  .msg.error {
    color: var(--danger);
  }
  .msg code {
    margin-left: 0.5rem;
    font-family: var(--font-mono);
    color: var(--ink-2);
  }
  .foot {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem 1rem;
    padding: 0.5rem 0.875rem;
    border-top: var(--hairline) solid var(--line);
    color: var(--muted);
    font-size: 0.72rem;
  }
  .hint {
    margin-left: auto;
  }
</style>
```


- [ ] **Step 4: パレットの状態と、サイドバーの検索の欄と ⌘K を足す**

4d-3 の Task 8 は、まだパレットがないので、サイドバーに検索の欄も ⌘K も置いていない。この Task が足す。

`web/app/palette-state.svelte.ts`（新規）:
```ts
let open = $state(false);

/** Whether the command palette is open. The sidebar opens it; CommandPalette reads it. */
export const palette = {
  get open(): boolean {
    return open;
  },
};

export function openPalette(): void {
  open = true;
}

export function closePalette(): void {
  open = false;
}
```

`web/app/sidebar/Sidebar.svelte` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```svelte
<script lang="ts">
  import { core }
```

変更後:
```svelte
<script lang="ts">
  import { onMount } from "svelte";
  import { core }
```

変更前:
```svelte
  import KindIcon from "../icons/KindIcon.svelte";
  import { hrefOf
```

変更後:
```svelte
  import KindIcon from "../icons/KindIcon.svelte";
  import { kbdLabel, registerShortcut } from "../keys";
  import { openPalette } from "../palette-state.svelte";
  import { hrefOf
```

変更前:
```svelte
  const route = $derived(router.route);

  function workspaceActive
```

変更後:
```svelte
  const route = $derived(router.route);

  onMount(() => registerShortcut("mod+k", () => openPalette()));

  function workspaceActive
```

変更前:
```svelte
    <span class="host">{location.host}</span>
  </div>

  <section class="ws-section"
```

変更後:
```svelte
    <span class="host">{location.host}</span>
  </div>

  <button type="button" class="search" onclick={openPalette}>
    <Icon name="search" size={15} />
    <span class="ph">ID・検索・コマンド</span>
    <kbd>{kbdLabel("mod+k")}</kbd>
  </button>

  <section class="ws-section"
```

変更前:
```svelte
  .ws-section {
    display: flex;
```

変更後:
```svelte
  .search {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    width: 100%;
    min-height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--label);
    text-align: left;
    cursor: pointer;
  }
  .ph {
    flex: 1;
  }
  kbd {
    padding: 1px 5px;
    border: 1px solid var(--line-input);
    border-radius: var(--radius-4);
    color: var(--label);
    font-size: var(--fs-11);
  }
  .ws-section {
    display: flex;
```

`web/app/sidebar/Sidebar.test.ts` を変える。次の箇所を、それぞれ置き換える（`pnpm fix` が整形を直す）:

変更前:
```ts
import { renderWithCore } from "../test-utils";
import Sidebar
```

変更後:
```ts
import { closePalette, palette } from "../palette-state.svelte";
import { renderWithCore } from "../test-utils";
import Sidebar
```

変更前:
```ts
  history.replaceState(null, "", "/");
  closeCreate();
});
```

変更後:
```ts
  history.replaceState(null, "", "/");
  closePalette();
  closeCreate();
});
```

変更前:
```ts
    expect(location.pathname).toBe("/projects/01PRJ1");
  });
```

変更後:
```ts
    expect(location.pathname).toBe("/projects/01PRJ1");
  });

  it("検索欄と ⌘K でパレットを開く", async () => {
    vi.spyOn(navigator, "platform", "get").mockReturnValue("Linux x86_64");
    const { getByRole } = mount();
    await fireEvent.click(getByRole("button", { name: /ID・検索・コマンド/ }));
    expect(palette.open).toBe(true);
    closePalette();
    await fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(palette.open).toBe(true);
  });
```

- [ ] **Step 5: App.svelte に組み込む**

`web/app/App.svelte` に `import CommandPalette from "./palette/CommandPalette.svelte";` を足し、`<Toasts />` の直前（Task 7 が目印にした所）に1つだけ置く:

```svelte
<CommandPalette />
<Toasts />
```

- [ ] **Step 6: 通ることを確かめる**

Run: `cd web && pnpm fix && pnpm vitest run app/palette app/tree app/sidebar && pnpm check && pnpm lint && pnpm build`
Expected: PASS（`app/` の型エラー・警告・指摘が 0 件）

- [ ] **Step 7: コミット**

```bash
git add web/app/palette web/app/palette-state.svelte.ts web/app/sidebar web/app/App.svelte
git commit -m "feat(web): ⌘K のコマンドパレット（ID・旧 ID・ULID・タイトル・クエリ・コマンド）"
```

---

### Task 16b: E2E（作成・編集・削除・パレットの spec）

Task 16a のハーネスの上に、タスクの詳細（Task 11・12）・作成ダイアログ（Task 13）・コマンドパレット（Task 14）を使う spec を足す。4d-3 の実物の上で `make e2e` を通した（16a の 17 テストと合わせて 33 テスト。実装では 47 件、最終レビューのあとは 50 件。末尾の「実装で plan から変えた点」の C-4）。

**この E2E が見つけて、Task 12・13 に直しを入れたもの**:
- Task 12 の `DeleteDialogs.svelte`: 完全削除が成功してキャッシュがタスクを忘れると、`show` が消えて `${show.id}` が `TypeError` になり、トーストも `/` への移動も出なかった。`await` の前に値を読む
- Task 13 の `CreateTask.svelte`: ストア一覧が読み込まれる前に C を押すとワークスペースが決まらず、作成できないままだった。一覧が読めたら決め直す

**Files:**
- Create: `web/e2e/create.spec.ts`、`web/e2e/edit.spec.ts`、`web/e2e/delete.spec.ts`、`web/e2e/palette.spec.ts`
- Modify: `web/e2e/screens.ts`（`createDialog`・`createReady`・`properties`・`saved`）、`web/e2e/live.spec.ts`（詳細が CLI の変更に追従するテスト）

**Interfaces:**
- Consumes: Task 16a の `harness.ts`・`screens.ts`・`live.spec.ts`、Task 11〜14 の画面（各 spec の冒頭の一覧）
- Produces: `screens.ts` の `createDialog`・`createReady`・`properties`・`saved`

- [ ] **Step 1: screens.ts に作成ダイアログと詳細の取り方を足す**

`createReady`: 作成ダイアログはスキーマを読むまで作成できないので、キーで送る前に作成ボタンが押せるまで待つ。

`web/e2e/screens.ts` を変える（`pnpm fix` が整形を直す）:

変更前:
```ts
so find a toast by its text

```

変更後:
```ts
so find a toast by its text
//   Detail      the title is the textbox "タイトル"; the body is the textbox "本文"; the
//               save state is the status "保存の状態" ("保存中…", "保存済み",
//               "保存済み · push 待ち"); the property panel is the complementary region
//               "プロパティ"; status, kind and the enum properties are buttons that open a
//               menu of menuitems, named by the label ("ステータス", "priority"); a date
//               property is an input labelled by its name ("due")

```

変更前:
```ts
/** The toasts that have the text. */
```

変更後:
```ts
export function createDialog(page: Page): Locator {
  return page.getByRole("dialog", { name: "タスクを作成" });
}

/** Waits until the create dialog can create (it has read the schema of the store). */
export async function createReady(dialog: Locator): Promise<void> {
  await expect(dialog.getByRole("button", { name: /^作成/ })).toBeEnabled();
}

/** The toasts that have the text. */
```

変更前:
```ts
/**
 * Drags the row
```

変更後:
```ts
export function properties(page: Page): Locator {
  return page.getByRole("complementary", { name: "プロパティ" });
}

/** The save state of the detail, once a change is on disk. */
export function saved(page: Page): Locator {
  return page.getByRole("status", { name: "保存の状態" }).filter({ hasText: /^保存済み/ });
}

/**
 * Drags the row
```

- [ ] **Step 2: 作成の spec**

`web/e2e/create.spec.ts`（本文の入力欄は CodeMirror。種類のラジオは見えない `input` なので `check({ force: true })`）:
```ts
// Screens this spec relies on (see screens.ts for the shared list):
//   - C opens the dialog named "タスクを作成" (CreateTask) when no input has focus
//   - the dialog has the textbox "タイトル", the textbox "本文" (CodeMirror), the radio
//     group "種類" (a radio is named by the kind's label: タスク, バグ, エピック …; a kind
//     the parent does not accept is disabled), the parent as "<ID> <title>" with the
//     button "親を外す" (or the hint "親: なし" when there is none), and the buttons
//     "作成 ⌘↵" / "作成 Ctrl+↵", "続けて作成", "キャンセル"
//   - after a create the toast says "<ID> を作成しました · <workspace>" and has the link
//     "開く", which opens /tasks/<ulid>
//   - a click on a tree row selects it; the dialog's parent is the selected row
import { expect, test } from "./harness";
import { createDialog, createReady, row, toast } from "./screens";

test("the dialog creates a task in one commit and the toast opens its detail", async ({
  page,
  hh,
}) => {
  await hh.open(page);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("親: なし")).toBeVisible();

  await dialog.getByRole("textbox", { name: "タイトル" }).fill("Write the spec");
  await dialog.getByRole("radio", { name: "バグ" }).check({ force: true }); // the input is hidden behind its label
  await dialog.getByRole("textbox", { name: "本文" }).click();
  await page.keyboard.type("first line\n\nsecond line");
  await dialog.getByRole("button", { name: /^作成/ }).click();

  const made = toast(page, "DM-1 を作成しました");
  await expect(made).toBeVisible();
  await expect(dialog).toBeHidden();
  await expect(row(page, "Write the spec")).toBeVisible();

  const t = hh.show("DM-1");
  expect(t.title).toBe("Write the spec");
  expect(t.kind).toBe("bug");
  expect(t.body?.trim()).toBe("first line\n\nsecond line");
  expect(t.history).toHaveLength(1); // the title, the kind and the body are one commit

  await made.getByRole("link", { name: "開く" }).click();
  await expect(page).toHaveURL(new RegExp(`/tasks/${t.ulid}$`));
});

test("Ctrl+Enter creates", async ({ page, hh }) => {
  await hh.open(page);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("From the keyboard");
  await createReady(dialog);
  await page.keyboard.press("Control+Enter");
  await expect(toast(page, "DM-1 を作成しました")).toBeVisible();
  await expect(row(page, "From the keyboard")).toBeVisible();
  expect(hh.show("DM-1").title).toBe("From the keyboard");
});

test("Create more keeps the dialog and empties the title and the body", async ({ page, hh }) => {
  await hh.open(page);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  const title = dialog.getByRole("textbox", { name: "タイトル" });
  await title.fill("First");
  await dialog.getByRole("button", { name: "続けて作成" }).click();
  await expect(toast(page, "DM-1 を作成しました")).toBeVisible();
  await expect(dialog).toBeVisible();
  await expect(title).toHaveValue("");

  await title.fill("Second");
  await dialog.getByRole("button", { name: /^作成/ }).click();
  await expect(toast(page, "DM-2 を作成しました")).toBeVisible();
  expect(hh.show("DM-1").title).toBe("First");
  expect(hh.show("DM-2").title).toBe("Second");
});

test("the selected row is the parent, and the kinds it cannot hold are disabled", async ({
  page,
  hh,
}) => {
  const project = hh.add("Release", "--kind", "project");
  const task = hh.add("A plain task", "--parent", project.id);
  await hh.open(page);

  await row(page, "Release").click();
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await expect(dialog.getByText(project.id, { exact: true })).toBeVisible();
  await expect(dialog.getByRole("button", { name: "親を外す" })).toBeVisible();
  await expect(dialog.getByRole("radio", { name: "エピック" })).toBeEnabled();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("Under the project");
  await dialog.getByRole("button", { name: /^作成/ }).click();
  await expect(toast(page, "DM-3 を作成しました")).toBeVisible();
  expect(hh.show("DM-3").parent).toBe(project.ulid);

  await row(page, "A plain task").click();
  await page.keyboard.press("c");
  await expect(dialog.getByText(task.id, { exact: true })).toBeVisible();
  await expect(dialog.getByRole("radio", { name: "エピック" })).toBeDisabled();
  await expect(dialog.getByRole("radio", { name: "タスク" })).toBeEnabled();
});

test("Escape closes the dialog without creating", async ({ page, hh }) => {
  await hh.open(page);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("Never made");
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  expect(hh.json<unknown[]>("ls")).toHaveLength(0);
});
```

Run: `cd web && pnpm e2e e2e/create.spec.ts`
Expected: PASS（5 tests）

- [ ] **Step 3: 編集と自動保存の spec**

`web/e2e/edit.spec.ts`（CodeMirror は Markdown の箇条書きの行で Enter を押すと次の項目の `- [ ] ` を自動で入れるので、2行目は項目の文字だけを打つ）:
```ts
// Screens this spec relies on (see screens.ts for the shared list):
//   - /tasks/<ulid> is the detail of a task
//   - the title is the textbox "タイトル"; the body is the textbox "本文" (CodeMirror)
//   - the property panel (region "プロパティ") has the button "ステータス" that opens a menu
//     of menuitems named "<label> · <name>" (未着手 · todo, 進行中 · in_progress …), the
//     button "priority" with the menuitems low, mid, high, urgent, and the date input "due"
//   - the save state is the status "保存の状態": "保存済み" (or "保存済み · push 待ち")
//     once a change is on disk
//   - a change is saved 2 seconds after the last key, or when the field loses focus
import { expect, test } from "./harness";
import { properties, saved } from "./screens";

test("the title is saved when the field loses focus", async ({ page, hh }) => {
  const t = hh.add("Before");
  await hh.open(page, `/tasks/${t.ulid}`);
  const title = page.getByRole("textbox", { name: "タイトル" });
  await expect(title).toHaveValue("Before");

  await title.fill("After");
  await page.keyboard.press("Tab");
  await expect(saved(page)).toBeVisible();
  expect(hh.show(t.id).title).toBe("After");
});

test("a change is saved after two seconds without typing", async ({ page, hh }) => {
  const t = hh.add("Before");
  await hh.open(page, `/tasks/${t.ulid}`);
  const title = page.getByRole("textbox", { name: "タイトル" });

  await title.fill("Typed and left alone");
  await expect(title).toBeFocused();
  expect(hh.show(t.id).title).toBe("Before"); // nothing is written while the user types
  await expect(saved(page)).toBeVisible({ timeout: 8000 });
  expect(hh.show(t.id).title).toBe("Typed and left alone");
});

test("the status, an enum property and a date are saved from the property panel", async ({
  page,
  hh,
}) => {
  const t = hh.add("Props");
  await hh.open(page, `/tasks/${t.ulid}`);
  const panel = properties(page);

  await panel.getByRole("button", { name: "ステータス" }).click();
  await page.getByRole("menuitem", { name: /^進行中/ }).click();
  await expect(panel.getByRole("button", { name: "ステータス" })).toContainText("進行中");

  await panel.getByRole("button", { name: "priority" }).click();
  await page.getByRole("menuitem", { name: "high" }).click();

  await panel.getByLabel("due").fill("2026-12-01");
  await panel.getByLabel("due").blur();

  await expect
    .poll(() => {
      const s = hh.show(t.id);
      return [s.status, s.props.priority, s.props.due];
    })
    .toEqual(["in_progress", "high", "2026-12-01"]);
  await expect(saved(page)).toBeVisible();
});

test("the body is saved from the Markdown editor, and a checkbox toggles from the list", async ({
  page,
  hh,
}) => {
  const t = hh.add("With a body");
  await hh.open(page, `/tasks/${t.ulid}`);
  const body = page.getByRole("textbox", { name: "本文" });

  await body.click();
  // Enter in a list item starts the next item, as in any Markdown editor.
  await page.keyboard.type("- [ ] first step\nsecond step");
  await page.getByRole("textbox", { name: "タイトル" }).click(); // the editor loses focus
  await expect(saved(page)).toBeVisible();
  await expect.poll(() => hh.show(t.id).body?.trim()).toBe("- [ ] first step\n- [ ] second step");

  await expect(page.getByRole("heading", { name: "チェックリスト 0/2" })).toBeVisible();
  await page.getByRole("checkbox", { name: "first step" }).check();
  await expect.poll(() => hh.show(t.id).body?.trim()).toBe("- [x] first step\n- [ ] second step");
});

test("a change from the CLI does not overwrite the field being edited", async ({ page, hh }) => {
  const t = hh.add("Original");
  await hh.open(page, `/tasks/${t.ulid}`);
  const title = page.getByRole("textbox", { name: "タイトル" });

  await title.click();
  await page.keyboard.press("End");
  await page.keyboard.type(" mine");
  await expect(title).toHaveValue("Original mine");

  // The page fetches the task again when the event arrives; wait for that.
  const refetched = page.waitForResponse(
    (r) => r.request().method() === "GET" && new URL(r.url()).pathname === `/api/tasks/${t.ulid}`,
  );
  hh.hh("set", t.id, "title=Theirs");
  await refetched;
  await expect(title).toHaveValue("Original mine");

  await page.keyboard.press("Tab"); // the last writer wins
  await expect(saved(page)).toBeVisible();
  await expect.poll(() => hh.show(t.id).title).toBe("Original mine");
});
```

Run: `cd web && pnpm e2e e2e/edit.spec.ts`
Expected: PASS（5 tests）

- [ ] **Step 4: 削除・復元・完全削除の spec**

`web/e2e/delete.spec.ts`:
```ts
// Screens this spec relies on (see screens.ts for the shared list):
//   - the detail header has the button "削除"; it opens the dialog named
//     "<ID> <title> を削除", which says "配下の <n> 件も一緒に削除します", lists the
//     tasks that go with it and has the button "<n + 1> 件を削除" (just "削除" when
//     nothing is under the task); after the delete the toast says "「<title>」を削除しました"
//     and has the button "取り消し"
//   - the detail of a deleted task shows the status "削除済み" with the buttons "復元" and
//     "完全に削除"; "復元" shows the toast "<ID> を復元しました"
//   - "完全に削除" opens the dialog named "<ID> を完全に削除"; its textbox "Store 名"
//     must hold the store's name before the button "完全に削除" of the dialog enables;
//     after the purge the toast says "<ID> を完全に削除しました" and the page goes to "/"
import { expect, test } from "./harness";
import { toast } from "./screens";

test("deleting a task takes its children too, and Restore brings them back", async ({
  page,
  hh,
}) => {
  const parent = hh.add("Release prep", "--kind", "project");
  const child = hh.add("Rollback notes", "--parent", parent.id);
  await hh.open(page, `/tasks/${parent.ulid}`);

  await page.getByRole("button", { name: "削除", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: /を削除$/ });
  await expect(dialog).toContainText("配下の 1 件も一緒に削除します");
  await expect(dialog).toContainText(`${child.id} Rollback notes`);
  await dialog.getByRole("button", { name: "2 件を削除" }).click();

  await expect(toast(page, "「Release prep」を削除しました")).toBeVisible();
  const band = page.getByRole("status", { name: "削除済み" });
  await expect(band).toBeVisible();
  expect(hh.show(parent.id).deleted_at).not.toBeNull();
  expect(hh.show(child.id).deleted_at).not.toBeNull();

  await page.getByRole("button", { name: "復元" }).click();
  await expect(toast(page, `${parent.id} を復元しました`)).toBeVisible();
  await expect(band).toBeHidden();
  expect(hh.show(parent.id).deleted_at).toBeNull();
  expect(hh.show(child.id).deleted_at).toBeNull();
});

test("Undo in the toast restores the task", async ({ page, hh }) => {
  const t = hh.add("Deleted by mistake");
  await hh.open(page, `/tasks/${t.ulid}`);

  await page.getByRole("button", { name: "削除", exact: true }).click();
  await page
    .getByRole("dialog", { name: /を削除$/ })
    .getByRole("button", { name: "削除", exact: true })
    .click();
  const deleted = toast(page, "「Deleted by mistake」を削除しました");
  await expect(deleted).toBeVisible();
  expect(hh.show(t.id).deleted_at).not.toBeNull();

  await deleted.getByRole("button", { name: "取り消し" }).click();
  await expect.poll(() => hh.show(t.id).deleted_at).toBeNull();
});

test("a deleted task can be purged once the store's name is typed", async ({ page, hh }) => {
  const t = hh.add("Gone for good");
  hh.hh("rm", t.id);
  await hh.open(page, `/tasks/${t.ulid}`);
  await expect(page.getByRole("status", { name: "削除済み" })).toBeVisible();

  await page.getByRole("button", { name: "完全に削除" }).click();
  const dialog = page.getByRole("dialog", { name: `${t.id} を完全に削除` });
  const confirm = dialog.getByRole("button", { name: "完全に削除" });
  await expect(confirm).toBeDisabled();
  await dialog.getByRole("textbox", { name: "Store 名" }).fill("demo");
  await expect(confirm).toBeEnabled();
  await confirm.click();

  await expect(dialog).toBeHidden();
  await expect(toast(page, `${t.id} を完全に削除しました`)).toBeVisible();
  await expect(page).toHaveURL(`${hh.origin}/`);
  await expect.poll(() => canShow(hh.hh.bind(hh), t.id)).toBe(false);
});

function canShow(hh: (...args: string[]) => string, id: string): boolean {
  try {
    hh("show", id);
    return true;
  } catch {
    return false;
  }
}
```

Run: `cd web && pnpm e2e e2e/delete.spec.ts`
Expected: PASS（3 tests）

- [ ] **Step 5: コマンドパレットの spec**

⌘↵ の新しいタブは `window.open` を `noopener` なしで呼ぶので、ブラウザが `sessionStorage` を複製して開き、トークンが引き継がれる。

`web/e2e/palette.spec.ts`:
```ts
// Screens this spec relies on (see screens.ts for the shared list):
//   - Ctrl+K opens the command palette: the dialog "ID・検索・コマンド" with the combobox,
//     a listbox of options; typing an ID or a word finds the task as an option "<ID> <title>"
//   - Enter opens the detail of the option in this tab; Ctrl+Enter opens it in a new tab,
//     which is a copy of this one (window.open without noopener), so the token goes with it
import { expect, test } from "./harness";

test("the palette finds a task by its ID and opens it", async ({ page, hh }) => {
  const t = hh.add("Find me");
  hh.add("Another one");
  await hh.open(page);

  await page.keyboard.press("Control+k");
  const palette = page.getByRole("dialog", { name: "ID・検索・コマンド" });
  await expect(palette).toBeVisible();
  await palette.getByRole("combobox").fill(t.id);
  await expect(palette.getByRole("option", { name: new RegExp(`${t.id}.*Find me`) })).toBeVisible();
  await page.keyboard.press("Enter");

  await expect(page).toHaveURL(new RegExp(`/tasks/${t.ulid}$`));
  await expect(page.getByRole("textbox", { name: "タイトル" })).toHaveValue("Find me");
});

test("Ctrl+Enter opens the task in a new tab that keeps the token", async ({ page, hh }) => {
  const t = hh.add("In a new tab");
  await hh.open(page);

  await page.keyboard.press("Control+k");
  const palette = page.getByRole("dialog", { name: "ID・検索・コマンド" });
  await palette.getByRole("combobox").fill(t.id);
  await expect(palette.getByRole("option", { name: new RegExp(t.id) })).toBeVisible();
  const [popup] = await Promise.all([
    page.waitForEvent("popup"),
    page.keyboard.press("Control+Enter"),
  ]);

  await expect(popup).toHaveURL(new RegExp(`/tasks/${t.ulid}$`));
  // The token came with the tab: the detail loads instead of the "open it from hh ui" screen.
  await expect(popup.getByRole("textbox", { name: "タイトル" })).toHaveValue("In a new tab");
});
```

Run: `cd web && pnpm e2e e2e/palette.spec.ts`
Expected: PASS（2 tests）

- [ ] **Step 6: 詳細が CLI の変更に追従する spec**

`web/e2e/live.spec.ts` を変える:

変更前:
```ts
//   - hh.open waits for
```

変更後:
```ts
//   - the detail at /tasks/<ulid> follows the store: the button "ステータス" shows the label of
//     the current status
//   - hh.open waits for
```

変更前:
```ts
import { row } from "./screens";
```

変更後:
```ts
import { properties, row } from "./screens";
```

ファイルの末尾に足す:
```ts
test("the detail follows a status changed by the CLI", async ({ page, hh }) => {
  const t = hh.add("Watched");
  await hh.open(page, `/tasks/${t.ulid}`);
  const status = properties(page).getByRole("button", { name: "ステータス" });
  await expect(status).toContainText("未着手");

  hh.hh("set", t.id, "status=in_progress");
  await expect(status).toContainText("進行中");
});
```
Run: `cd web && pnpm e2e e2e/live.spec.ts`
Expected: PASS（6 tests）

- [ ] **Step 7: 全体を通す**

Run: `cd web && pnpm fix && pnpm lint && pnpm check && cd .. && make e2e`
Expected: Biome・svelte-check にエラーなし。`make e2e` が 33 テスト（実装では 49 件。完全削除を外す前は 50 件）すべて PASS（`pnpm exec playwright test --repeat-each=3` でも）

- [ ] **Step 8: コミット**
```bash
git add web/e2e
git commit -m "test(web): 作成・編集・削除・コマンドパレットの E2E を足す"
```

---

### Task 17: 仕上げ

README に GUI の開発手順を足し、`CONTRIBUTING.md` の「必要なもの」と「コマンド」を GUI のぶんだけ直し、全体の検査を通す。spec の更新（4d・4e の表、Onboarding と CreateStore の 4d への移動）は plan の確定時に別途行うので、ここでは触らない。

**Files:**
- Modify: `README.md`、`README.ja.md`、`CONTRIBUTING.md`

**Interfaces:**
- Consumes: Task 2 の `make web`・`web-check`・`e2e`・`pnpm dev` と `HEDGEHUG_UI`、Task 16a・16b の E2E、Task 1〜16b の成果すべて
- Produces: なし（文書と最終検査）

- [ ] **Step 1: README.md**

「Contributing」の前に足す（`hh ui` の `HEDGEHUG_UI` は `vite.config.ts` が読む）。

````markdown
## Development

The GUI is in `web/`: a Svelte app built with Vite, embedded in the binary. You need
Node and pnpm; `mise install` installs the versions pinned in `mise.toml`.

```bash
mise install
make build          # builds web/dist, then bin/hedgehug with the GUI inside
bin/hedgehug ui     # serves the GUI on 127.0.0.1 and opens the browser
```

To work on the GUI with reload, start the server and the Vite dev server side by side.
The dev server proxies `/api` to the server and opens the browser with its token:

```bash
bin/hedgehug ui --no-open       # prints: Hedgehug UI: http://127.0.0.1:<port>/#token=<token>
cd web
HEDGEHUG_UI='http://127.0.0.1:<port>/#token=<token>' pnpm dev
```

```bash
make web-check      # Biome, svelte-check, the API types and Vitest (also part of make check)
make e2e            # Playwright against bin/hedgehug (installs Chromium on first run)
```

A binary built without `make web` (`go build`, `go install`) has no GUI: `hh ui` serves a
page that says to build it.
````

- [ ] **Step 2: README.ja.md**

「コントリビュート」の前に足す。

````markdown
## 開発

GUI は `web/` にあります。Vite でビルドする Svelte のアプリで、バイナリに埋め込みます。Node と pnpm が要ります。`mise install` で `mise.toml` に固定した版が入ります。

```bash
mise install
make build          # web/dist を作り、GUI を埋め込んだ bin/hedgehug を作る
bin/hedgehug ui     # 127.0.0.1 で GUI を配り、ブラウザを開く
```

GUI をリロードしながら直すときは、サーバーと Vite の開発サーバーを並べて動かします。開発サーバーは `/api` をサーバーへ中継し、トークン付きでブラウザを開きます。

```bash
bin/hedgehug ui --no-open       # 1行目: Hedgehug UI: http://127.0.0.1:<port>/#token=<token>
cd web
HEDGEHUG_UI='http://127.0.0.1:<port>/#token=<token>' pnpm dev
```

```bash
make web-check      # Biome、svelte-check、API の型、Vitest（make check にも含まれます）
make e2e            # bin/hedgehug に対する Playwright（初回は Chromium を入れます）
```

`make web` を通さずに作ったバイナリ（`go build`、`go install`）には GUI が入りません。`hh ui` は、GUI をビルドするよう伝えるページを返します。
````

- [ ] **Step 3: CONTRIBUTING.md**

「Getting started」の「You need」の箇条書きに2行足し、コマンドの一覧を直す。

```markdown
- Node and pnpm, for the GUI in `web/` (the versions are pinned in `mise.toml`; `mise install`
  installs them and gotestsum)
```

```markdown
make check          # everything CI checks: tidy, lint, fmt, test, and the GUI (make web-check)
make test           # tests with the race detector
make cover          # tests with the race detector and coverage
make fmt            # format with gofumpt + goimports
make release-check  # goreleaser config and cross-compiling
make web            # the GUI into web/dist (the binary embeds it)
make build          # bin/hedgehug, with the GUI
make e2e            # Playwright against bin/hedgehug
make clean-cache    # empty the Go build cache when it grows too large
```

「GUI の整形」の一文を、「Do not commit while `make check` fails.」の前の段落に足す。

```markdown
Run `pnpm fix` in `web/` before you commit GUI code: `make web-check` fails on anything Biome
would format differently.
```

- [ ] **Step 4: 全体の検査**

Run: `make check`
Expected: `tidy-check`・`lint`・`fmt-check`・`test`（`-race`）・`web-check` がすべて成功

Run: `make release-check`
Expected: `goreleaser check` が通り、2つの OS（darwin・linux）の `go build` が成功する（`web/dist` が `.gitkeep` だけでも）

Run: `make web`
Expected: `vite build` が警告を出さない（どのチャンクも 500 kB 未満。CodeMirror は `codemirror-base` と `codemirror` の2つに分かれる）

Run: `make e2e`
Expected: 33 テスト（実装では 49 件。完全削除を外す前は 50 件。末尾の C-4）すべて PASS

Run: `git status --short`
Expected: 何も出ない（`web/dist` の `.gitkeep` が消えていない、`pnpm-lock.yaml` が変わっていない、`schema.d.ts` が古くない）

- [ ] **Step 5: 手で確かめる**

```bash
make build
export HEDGEHUG_HOME="$(mktemp -d)"
bin/hedgehug store init demo --prefix DM
bin/hedgehug ui
```

ブラウザで次を確かめる。`C` でタスクを作成し、ツリーに出る。行を開いてタイトルを直し、2 秒後に「保存済み」になる。別の端末で `HEDGEHUG_HOME=… bin/hedgehug set DM-1 title=changed` を実行し、行の題名が変わり「CLI で更新 · 今」が出る。行をドラッグして並べ替える。タスクを削除して復元する。

- [ ] **Step 6: コミット**
```bash
git add README.md README.ja.md CONTRIBUTING.md
git commit -m "docs: GUI の開発手順を足す"
```

---

## 付録: E2E が前提にする文言と role（詳細と作成）

画面の操作に使う文言と role の一覧（`getByRole(role, { name })`）。テストファイルの `web/app/detail/*.test.ts`・`web/app/create/*.test.ts` が同じ文言で確かめているので、E2E はこれと揃える。

**作成ダイアログ（タスク）**: `dialog`「タスクを作成」、`textbox`「タイトル」（Enter で作成）、`radiogroup`「種類」の `radio`（名前は種類の表示名。親に置けない種類は `disabled`）、`radiogroup`「ワークスペース」の `radio`（`release（REL）` のように「名前（接頭辞）」と Store 名。ワークスペースが1つなら出ない）、`textbox`「本文」（CodeMirror。`@`・`#` の補完が出る）、`button`「ステータス」（押すと `menu`「ステータス」、項目は `menuitem`「未着手 · todo」の形）、`textbox`「担当者」（Enter で追加）、スキーマの props はラベルが名前（`button`「priority」、`textbox`「area」、`spinbutton`「points」、日付は `getByLabel("due")`）、`button`「続けて作成」、`button`「作成 ⌘↵」（Linux・Windows は「作成 Ctrl+↵」。`/^作成/` で取る）、`button`「キャンセル」、親は `button`「親を外す」・`combobox`「親を検索」。作成後のトーストは「REL-9 を作成しました · release」と、リンク「開く」。
**作成ダイアログ（プロジェクト）**: `dialog`「プロジェクトを作成」、`textbox`「名前」、`textbox`「リポジトリ」（Enter で追加。入力中に「Enter で github.com/org/repo として追加します（…）」）、`textbox`「担当者」、日付「start」「due」、`button`「作成」。
**C**: ウィンドウで C（修飾キーなし、入力欄とダイアログの外）がタスクの作成ダイアログを開く。

**タスク詳細**: `navigation`「パンくず」、`textbox`「タイトル」（その場編集）、`status`「保存の状態」（「保存中…」「保存済み」「保存済み · push 待ち」。失敗は文言と `button`「やり直す」）、`button`「ウォッチ」／「ウォッチ中」、`button`「ULID をコピー」、ブロックの帯は `status`「ブロック」（「<ID> <題>（<状態>）にブロックされています。…」）、削除済みは `status`「削除済み」（「削除済み（<日時>）。編集はできません。」）、壊れたファイルは `alert`（「hh doctor で確かめてください。」と CLI の行）。
**プロパティ**: `button`「種類」「ステータス」（`menu` → `menuitem`「完了 · done」）、`textbox`「担当者」「ラベル」「リポジトリ」（Enter で追加。チップは `button`「<名前> を外す」）、スキーマの props（上と同じ）、定義にないキーは「<名前>（未定義）」。
**本文**: `textbox`「本文」（CodeMirror の contenteditable）。チェックリストは `heading`「チェックリスト n/m」と、項目の文字を名前にした `checkbox`。
**関連**: `heading`「関連」、「同じ Store（<名前>）のタスクだけを参照できます」、`button`「関連を追加」→ `button`「関連の種類」（`menu`）と `combobox`「タスクを検索」→ `option`「<ID> <題>」、各グループは `group`（名前は「ブロックされている」など）、`button`「<ID> を外す」、エラーは `alert`。
**コメント**: `heading`「コメント n」、`textbox`「コメントを書く」（CodeMirror。⌘↵ で送る）、`button`「コメント」、各コメントに `button`「編集」「削除」、編集中は `textbox`「コメントを編集」と `button`「保存」「キャンセル」。削除のトースト「コメントを削除しました」。他人のコメントは `alert`（`comment.not_author` の文言）。
**履歴**: `heading`「履歴」、各行に `button`「この時点に戻す」（最新の行にはない）→ `dialog`「<ID> を <日時> の状態に戻す」（変わる項目の `table`、`button`「N 項目を戻す」。同じなら「この時点と同じです」）。
**削除**: `button`「削除」→ `dialog`「<ID> <題> を削除」（配下があれば「配下の N 件も一緒に削除します」、`button`「N 件を削除」か「削除」）。トーストは「「<題>」を削除しました」と取り消し。削除済みの詳細は `button`「復元」（トースト「<ID> を復元しました」）と `button`「完全に削除」→ `dialog`「<ID> を完全に削除」（対象の一覧、`textbox`「Store 名」に Store 名を入れると `button`「完全に削除」が押せる。成功したら `/` へ）。（2026-11-03: main の決定 4ea022c で完全削除を GUI と API から外した。末尾「実装で plan から変えた点」の利用者の決定 7）。削除済みの詳細は「復元」だけになった

### 実装で plan から変えた点（2026-11-03、レビューのあと）

この plan の各 Task のコードは plan を書いた時点のもの。実装とレビューの途中で、次を変えた。「利用者の決定」は利用者がはっきり決めたもの、「Ruling」は controller が決めたもの、C-1〜C-18 は実装前に plan と実物を照合して決めたもの。

#### 利用者の決定

1. **コメントの削除は確認ダイアログを出す（取り消しは付けない。Task 12、2026-11-02）。** `ui/Dialog`、名前は「コメントを削除」、本文は「このコメントを削除します。元に戻せません。」、ボタンは「キャンセル」と「削除」（danger）、足元は `hh comment rm <ULID>`。失敗（`comment.not_author` など）はダイアログの中に `alert` で出して開いたままにする。成功のトースト「コメントを削除しました」はそのまま。E2E は、コメント行の「削除」を押し、名前「コメントを削除」の `dialog` の中の「削除」を押す。
2. **1 つしか持てない関連の種類に値があるときは、「関連の種類」のメニューでその項目を無効にする（Task 12）。** `hint` は「すでに設定されています（外してから追加）」。黙って値を置き換えない。
3. **GUI が出す `hh add` の行には、いつも `--no-repo` を付ける（Task 13）。** `cliAdd` が先頭のフラグとして付ける（タスクとプロジェクトの作成ダイアログ。リポジトリの指定は `-p repos=…` のまま）。ツリーの「下に追加」の行には `hh` の行がない。
4. **ダイアログが開いている間は、⌘K でパレットを開かない（Task 14）。** Sidebar の ⌘K は `[aria-modal="true"]:not([data-palette])` があれば何もしない（パレットのパネルが `data-palette` を持つ）。
5. **最近開いたものは、キャッシュにない分を `GET /api/tasks/{ulid}` で取り、いつも 8 件出す（Task 14。概要 plan の細部 47 を補う）。** パレットを開いたとき、`palette/recent.ts` の `fetchMissing` が取りに行き、`cache.prime` で入れる。404（消えたタスク）は記録（`hedgehug.recent`）から外す。ほかの失敗は何もしない。
6. **取り直しの絞り込みから外れた行は、その行だけ取り直す（最終レビュー、2026-11-03。外れた行だけ取り直す案）。** クエリの取り直しの結果に含まれなくなった行（ステータスが変わって `status:todo` のツリーから外れた行など）は、キャッシュに古い姿が残る。その回のクエリ・詳細の取り直しがすべて終わったあと、`us`（resync なら全行）のうちキャッシュにある行で、読み取りの印（`rowSince`）がイベント・resync より古いものを `GET` で取り直す。結果に残った行は印が新しいので、余分な `GET` は出ない。
7. **完全削除は GUI と API から外した（main の決定 4ea022c、2026-11-02。マージ後の 2026-11-03）。** `DELETE /api/tasks/{id}` の `purge`・`confirm`、`purge.unconfirmed`、`cliPurge`、`purge.*` のメッセージ、`cache.applyWrite` の `ulids` が main から消えたので、4d-4 も合わせた。削除済みの詳細は「復元」だけを出す。`delete-actions.ts` の `probePurge`・`purgeTargetsOf`・`purgeTask`・`Target` と `DeleteDialogs.svelte` の purge のダイアログ、そのテストと `delete.spec.ts` の完全削除の E2E を消した。E2E は 49 件（50 から 1 件減）。完全削除は `hh purge --older-than` に一本化し、GUI は 4e の「設定 → 同期と保守」に置く。

#### Ruling（controller の裁定）

- **C-1: `CommandPalette.svelte` に `openCreate` の import を足した。**
- **C-2: TreeRow の `onclick={onselect}` は今のコードにない。** `onclickcapture` のブロックの次に `ondblclick={onopen}` を足した。
- **C-3: TreeView のフッターの `⌘↵` は `kbdLabel("mod+enter")`。**
- **C-4: E2E の件数。** plan 本文の「33」は、実装前の既存 31 + 16b の 16 = 47 と読む。最終レビューの修正のあとは 50 件（下の「最終レビューで直した点」の 5 番）。本文の該当 3 か所に注記した。
- **C-5: 作成ダイアログ（タスク・プロジェクト）の足元の `hh` の行は、4d-3 の利用者の決定 8 に合わせる。** 空欄は `<タイトル>`・`<名前>` の形で、入力がそろうまでコピーできない（`Placeholder`・`cliDisabled`）。`cliAdd` の `title` を `Arg`（string | Placeholder）に広げた。
- **C-6: IME 変換中（`e.isComposing` または keyCode 229）は、`ui/Dialog` の Esc、`ChipsField` の Enter と `,`、`TaskPicker` の Enter と矢印、パレットの入力欄のキーで何もしない。** 4d-3 の「変換中の Esc は変換の取り消しだけ」に揃えた。テストを 1 件ずつ足した（TaskPicker は Task 11 で作るので Task 11 で直した）。
- **C-7: パレットの「選んでいるタスク」は、route がタスク詳細ならそのタスクを先にし、なければ `selection.ulid`。** 前のツリーの選択に効く不具合を避ける。
- **C-8: パレットのエラー表示は `errorMessage(error)` だけ。** `detail` を重ねて出さない（4d-3 R5 と同じ）。
- **C-9: パレットのキー表示は `kbdLabel("shift+mod+s")`・`kbdLabel("mod+enter")`。** `CommandEnv` の `mac` は外した。Windows・Linux は `Ctrl+Shift+S`。テストは `navigator.platform` の差し替えで確かめる。
- **C-10: `@codemirror/autocomplete` は入っているので `pnpm add` は不要。**
- **C-11: `vite build` は警告を出さない（出たら不具合）。**
- **C-12: vitest の絞りは `pnpm vitest run app`。**
- **C-13: 「Ctrl+Enter in the body editor, once」のテストは、本文に「メモ」を打ち、POST の body が `"メモ"` で 1 回だけ呼ばれたことを確かめる。**
- **C-14: プロパティの欄の分岐は、共有の `web/app/detail/PropField.svelte` に出した。** `PropsPanel` と Task 13 の `CreateTask` が使い、`statusOptions` も `prop-rows.ts` に出した。plan の丸ごと重複を避ける。bool は値を `String(value)` にして選択に渡す（plan のままだと boolean の true が「はい」に当たらない）。
- **C-15: `history-labels` に、コミットへ書く op をすべて足した。** `internal/ops`・sync・store を grep して確かめた（init、status、dep、watch、unwatch、move-in、move-out、purge-moved、rewrite-history、revert-overwritten、migrate、workspace、view、setup、replay）。全 op に日本語があることを確かめるテストを足した。
- **C-16: palette.spec は `ControlOrMeta+k`・`ControlOrMeta+Enter`。**
- **C-17: 「開いた直後にタイトル（名前）にフォーカスがある」テストを足し、通らなければ通るように直す。** Task 13 のレビューで、最初のテストが偽の合格だった（Dialog の初期フォーカスが上書きしていた）ので、Dialog は `[data-autofocus]` を先に選ぶようにし、`CreateTask` のタイトルに付けた。
- **C-18: 完全削除の 1 行目は「タスクのファイルとコメントを消します。Hedgehug からは復元できません。」。** 履歴と番号の扱いは 2 行目に分けた。（完全削除のダイアログごと外れた。main の決定 4ea022c）
- **C-9・C-15・C-16・C-18 のような影響の小さい指摘も、plan どおりに戻さず直す。**（CLAUDE.md の残さない方針）
- **コマンドパレットの同期は `sidebar/sync-run.svelte.ts`（`runSync`・`syncRun.busy`）で SyncCard と共有する。** `syncMessage(reports)` で結果を出し、実行中の 2 回目は何もしない。plan のパレットは結果を見なかった。
- **Task 17 の Step 4（`make check`・release-check・web・e2e の全体の検査）は、最終レビューの修正のあとに 1 回だけ回す。** 利用者の「全テストは仕上げに 1 回」に従う。
- **README の `make e2e` の説明は「Chromium が無ければ入れる」。** Makefile の `e2e` は実行のたびに `playwright install chromium` を呼ぶ。
- **delete.spec の完全削除の確認は、`hh show` でなく `GET /api/tasks/<ulid>` が 404 になるまで poll する。** `hh show` は失敗時に CLI の stderr をテスト出力に出す。（完全削除の E2E ごと外れた。main の決定 4ea022c）
- **最終レビューの再レビュー 1〜5 の指摘の裁定（下の「最終レビューで直した点」の 11〜13 番の根拠）。**
  - 持たれていない行: resync でも持たれていない行に `fetchRow` する。ULID ごとの `seq` で古い応答を捨てる。`cache.task()` を読む箇所の退行を避けるため、行をキャッシュに入れない別の方法は取らない。
  - 古い読み取りの順序づけ: ULID ごとの `lastChange`（`tasks.changed` の epoch）を持ち、`prime`・`fetchRow` は、それより前に始まった読み取りを捨てる。
  - 読み取りの応答を捨てたとき、その行がキャッシュになければ必ず取り直す（`seq` で重複を除く）のを、`prime`・`fetchRow` のどの経路でも 1 か所（`refetchMissing`。5 回目に `refetchStale` にまとめた）で行う。窓を 1 つずつ塞ぐ直しが 2 回続いたため、構造で直した。
  - 投稿直後のコメントの削除: 投稿直後のコメントを読み直しの前に削除したら、`added`・`changed` から外したうえで `removed` にも足す。削除の前に出た読み直しが古い姿で届いても出さない（`reconcile` が、読み直しに現れなくなったあとで `removed` から外す）。

#### 最終レビューで直した点

実装を終えたあとの全体のレビューと、その再レビュー 5 回の指摘を、すべて直した。直さなかったのは「最近開いたものの 404 以外の失敗は、開くたびに取り直す」の 1 件だけ（失敗の種類ごとに記録を持つ複雑さに見合わないと判断した）。

1. **詳細のほかのタスクがキャッシュで追従する。** `core/cache.ts` に、`tasks.changed` で「どのクエリ・詳細も持たない行」を `GET /api/tasks/{id}` で取り直す `fetchRow`（持ち主の判定は `heldUlids`）を足した。`createLookup` は取った行を `cache.prime` で入れる。TaskDetail（パンくず・ブロックの帯）と Relations は `crumbsSig`（行の題・ステータスなどの署名）を購読して引き直す。
2. **読み取りは書き込みとして入れない。** `epochNow()` と `prime(tasks, since)` を足し（`markWrite` なし）、パレットの検索・最近開いたものの取得は、要求の前の epoch を取って `prime` する。`recent.ts` の二重キャストは消えた。
3. **パレットのステータス変更はスキーマの読み込みを待つ。** `pickStatus` が `schema.load(store)` を呼び、パレットが `schemaRev` を購読する。読み込み中は「読み込み中…」、失敗は `errorMessage`。
4. **チップ欄で ⌘↵ が効く。** `ChipsField` は ⌘↵ のとき、チップを足すだけで `preventDefault` せず、ダイアログへ通す。入力が拒否されたときだけ `preventDefault`。CreateTask・CreateProject の window の ⌘↵ は `defaultPrevented` と IME 変換中を見る。
5. **E2E は 50 件（完全削除を外したあとは 49 件）。** 作成・編集・削除・パレットの 16 件のあと、live.spec に「タイトルの追従」「ブロックの帯」、create.spec に「詳細で C → 子」を足し、`Control+Enter` を `ControlOrMeta+Enter` に揃えた。
6. **Menu は `focusout` で `relatedTarget` が Menu・anchor の外なら閉じる。** TreeView は破棄時に選択を消し、`resolveCreateContext` は task の route で selection を見ない。TaskDetail は親の種類を `PropsPanel` の `parentKind` に渡す。パレットの同期結果の文言も、SyncCard と同じ `syncMessage` を使う。
7. **Comments の送信・編集の保存、削除・完全削除・復元の確認は、実行中の二重実行を止める。** 実行中は確認ダイアログを閉じられず、キャンセルを無効にする。`DeleteDialogs` の `openDelete`・`openPurge` は開いた回の番号で古い応答を捨てる。（`openPurge` と purge の確認は外れた。main の決定 4ea022c）
8. **関連の各行は ID のリンクのまま ULID のホバー表示とコピーを出す。** `IdLabel` に `href`・`onopen` を足した。`href` のときは keydown を止めない。Sidebar の `.add` は `--control-h-sm`。
9. **Comments の重ね表示は、全部を捨てず項目ごとに照らし合わせる。** `added` は読み直しに現れたら外し、`changed` は `base` の本文が編集前のままなら残し、`removed` は `base` から消えたら外す。`CommentView` に `updated` がないので、`changed` は編集時の本文（`from`）と比べる。投稿したばかりのコメントの編集は `added` の中も差し替え、読み直しが投稿時の本文で来ても、そのコメントを含まなくても編集を残す。投稿直後のコメントを読み直しの前に削除したときは、`added`・`changed` から外したうえで `removed` にも足し、削除の前に出た読み直しでも出さない。
10. **History は `show` の参照が変わるたびに履歴を取り直し、Comments の重ね表示は `show.comments` が変わったときに解く。** コメントだけのコミットは `updated` を変えないので、`updated` を見ると履歴と重ね表示が更新されなかった。
11. **`cache.ts` の読み取りの順序づけ。** ULID ごとの `rowSeq`（全体のカウンタ。応答が届いたとき・`forget` のときに消す）で、追い越された応答（成功も失敗も）を捨てる。`lastChange`（ULID ごと）と `resyncAt` で、イベント・resync より前に始まった読み取りを捨て（`changedSince`）、捨てた行がキャッシュになければ `refetchStale` が取り直す。`lastChange`・`lastWrite` は行を忘れても消さない（消すと removed の直後の読み取りを通すおそれがある）。resync でも持たれていない行を取り直す。
12. **行ごとの `rowSince`。** その行を最後に入れた（同じと確かめた）読み取りの `since`。`applyWrite` は書き込みの epoch。`queueRefresh` はその回の `fetchQuery`・`fetchDetail` がすべて終わったあと、`us` のうちキャッシュにある行に `refetchStale` を呼ぶ（利用者の決定 6）。`fetchDetail` に `last` を足し、追い越された詳細の取り直しは新しい方を待つ。
13. **ID で開いた詳細の最初の応答の前に `tasks.changed` が来たら、詳細を取り直す（作業中に見つけた既存の窓）。** `runDetail` の成功側で、書き込みの stale と同じ形で `changedSince(show.ulid, since)` なら `queueRefresh([show.ulid])` を出す。
14. **`cache.test.ts` の「順序の表」。** 手順 E（イベント）・R（resync）・W（自分の書き込み）・A（走っている読み取りがすべて応答）と、D（removed）・P（最初の lookup の応答）を並べ、持ち主 5 通り（missing・cached・query・leaving・detail）で回す（391 の並び）。最後に `cache.task(X)` が最新、`rowSeqs` が空、読み取りが尽きること、GET の数が変更の数の 2 倍以下であることを見る。`settle()`（`setTimeout`）を `setImmediate` の flush に替えて 31.5 秒を 6.7 秒にした（`refetchStale` を外すと missing と leaving が落ちる）。
15. **共有の部品: `PropField`（C-14）・`sync-run`・`IdLabel` の `href`・`Dialog` の `data-autofocus`。** 重複させず 1 か所にした。

#### 受け入れた挙動

- 削除済みの行を lookup が読むと、404 の `GET` が 1 本出る（`prime` の前からの挙動）。
- クエリの取り直しが失敗した回は、その回の `us`（resync なら全行）のうちキャッシュにある行に `GET` が 1 本ずつ出る（失敗時に確認を飛ばすと、絞り込みから外れた古い行が残るため）。
- resync ごとの `GET` の数の上限は、Store のタスク数。
