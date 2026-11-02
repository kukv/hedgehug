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
    prime: vi.fn(),
    epochNow: vi.fn(() => 0),
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
