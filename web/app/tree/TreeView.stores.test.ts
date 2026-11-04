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
  over: {
    oneStore?: boolean;
    betaSchema?: "none" | "ok" | "nodone";
    roots?: unknown;
    wsLate?: boolean; // GET /api/stores has not answered yet
    hidden?: string[]; // the stores the GUI leaves out
  } = {},
) {
  let wsReady = !over.wsLate;
  const tasks = new Map([
    ["A1", task("A1", "epic", null, { store: "alpha", workspace: wsOf.alpha, title: "alpha one" })],
    ["A2", task("A2", "task", "A1", { store: "alpha", workspace: wsOf.alpha, title: "alpha two" })],
    ["B1", task("B1", "task", null, { store: "beta", workspace: wsOf.beta, title: "beta one" })],
    ["B2", task("B2", "task", null, { store: "beta", workspace: wsOf.beta, title: "beta two" })],
    [
      "C1",
      task("C1", "task", null, { store: "alpha", workspace: wsOf.alpha, title: "alpha gone" }),
    ],
    ["C2", task("C2", "task", null, { store: "beta", workspace: wsOf.beta, title: "beta gone" })],
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
      loaded: () => wsReady,
      stores: () => (over.oneStore ? [{ name: "alpha" }] : [{ name: "alpha" }, { name: "beta" }]),
      workspace: (s: "alpha" | "beta", u: string) =>
        wsReady && wsOf[s]?.ulid === u ? entry(s) : undefined,
      workspaces: () => (wsReady ? [entry("alpha"), entry("beta")] : []),
      readOnly: () => null,
      hidden: (s: string) => over.hidden?.includes(s) ?? false,
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
    // GET /api/stores answers
    loadWorkspaces() {
      wsReady = true;
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

  it("leaves out the tasks of a hidden store, and does not count its canceled ones", () => {
    const { watchTree } = setup({ hidden: ["beta"] });
    render(TreeView, { view: all, query: "kind:task -status:canceled" });
    expect(screen.getAllByRole("treeitem").map((el) => el.getAttribute("data-ulid"))).toEqual([
      "A1",
      "A2",
    ]);
    expect(screen.getByText("2 件 · 中止 1 件を除外")).toBeInTheDocument();
    // what is asked for does not change: the hiding is of the screen only
    expect(watchTree).toHaveBeenCalledWith({
      q: "kind:task -status:canceled",
      tree: true,
      store: undefined,
    });
  });

  it("keeps the badges of both stores in the count of stores while one is hidden", () => {
    setup({ hidden: ["beta"] });
    render(TreeView, { view: all, query: "" });
    expect(screen.getAllByTitle("Store: alpha")).toHaveLength(2);
  });

  it("does not leave out a hidden store in the page of its workspace", () => {
    setup({ hidden: ["beta"] });
    render(TreeView, { view: { kind: "workspace", store: "beta", ulid: "WB" }, query: "" });
    // the fake answers with every root whatever the scope: the rows of beta are still there
    const ulids = screen.getAllByRole("treeitem").map((el) => el.getAttribute("data-ulid"));
    expect(ulids).toContain("B1");
    expect(ulids).toContain("B2");
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

  it("drops a held key when another row is selected before the schema arrives", async () => {
    const { api, loadBeta } = setup({ betaSchema: "none" });
    api.PATCH.mockResolvedValue(
      ok({ task: task("A2", "task", "A1", { store: "alpha", status: "done" }), commit: "c1" }),
    );
    render(TreeView, { view: all, query: "" });
    const b1 = treeitem("beta one");
    await fireEvent.click(b1);
    await fireEvent.keyDown(b1, { key: " " }); // waits for the schema of beta
    await fireEvent.click(treeitem("alpha two"));
    loadBeta();
    await new Promise((r) => setTimeout(r, 30));
    // Neither the row it was pressed on (no longer selected) nor the selected one is completed.
    expect(api.PATCH).not.toHaveBeenCalled();
  });

  it("makes the task of the foot input once the workspace has loaded", async () => {
    const { api, loadWorkspaces } = setup({ wsLate: true });
    api.POST.mockResolvedValue(
      ok({
        task: task("N1", "task", null, { store: "alpha", workspace: wsOf.alpha }),
        commit: "c1",
      }),
    );
    render(TreeView, { view: { kind: "workspace", store: "alpha", ulid: "WA" }, query: "" });
    await fireEvent.click(screen.getByRole("button", { name: /タスクを追加/ }));
    loadWorkspaces();
    const input = await screen.findByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）");
    await fireEvent.input(input, { target: { value: "新規" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    await waitFor(() =>
      expect(api.POST).toHaveBeenCalledWith("/api/tasks", {
        params: { query: { store: "alpha" } },
        body: expect.objectContaining({ title: "新規", workspace: "AAA" }),
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
