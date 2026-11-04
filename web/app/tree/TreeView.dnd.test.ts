import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { tick } from "svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { schema, task } from "./fixtures";

type Opts = {
  ulid: string;
  locked?: boolean;
  openChildren: boolean;
  onDragStart: () => void;
  onOver: (place: string | null, source: string | null) => void;
  onDrop: (source: string, place: string) => void;
};
const h = vi.hoisted(() => ({
  core: undefined as unknown,
  toast: vi.fn(),
  rows: new Map<string, unknown>(),
  attached: 0,
  dragEnd: undefined as undefined | (() => void),
}));

vi.mock("../context", () => ({ core: () => h.core }));
vi.mock("../router", () => ({
  router: { route: { name: "project", ulid: "P" }, go: vi.fn(), href: () => "/" },
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
      h.attached++;
      h.rows.set(get().ulid, get());
      return () => {};
    },
    treeDragEnd: (onEnd: () => void) => () => {
      h.dragEnd = onEnd;
      return () => {};
    },
    treeAutoScroll: () => () => () => {},
  };
});

import TreeView from "./TreeView.svelte";

const ok = (data: unknown) => ({ data, response: new Response(null, { status: 200 }) });

function setup(readOnly: string | null = null) {
  const tasks = new Map([
    ["P", task("P", "project", null, { context: true, id: "REL-15", title: "リリース準備 v2.4" })],
    ["E8", task("E8", "epic", "P", { id: "REL-8", title: "リリース作業" })],
    ["T7", task("T7", "task", "P", { id: "REL-7", title: "スキーマを再生成" })],
    ["T9", task("T9", "task", "P", { id: "REL-9", title: "負荷試験" })],
    ["T1", task("T1", "task", "E8", { id: "REL-1", title: "子" })],
  ]);
  const roots = [
    {
      ulid: "P",
      children: [
        { ulid: "E8", children: [{ ulid: "T1", children: [] }] },
        { ulid: "T7", children: [] },
        { ulid: "T9", children: [] },
      ],
    },
  ];
  const none = () => () => {};
  const ws = {
    store: "shared",
    tone: "purple",
    ulid: "W1",
    name: "release",
    prefix: "REL",
    projects: [],
  };
  const api = { POST: vi.fn(), PATCH: vi.fn() };
  h.core = {
    api,
    cache: {
      task: (u: string) => tasks.get(u),
      live: () => undefined,
      watchTree: () => ({
        roots: () => roots,
        error: () => null,
        loading: () => false,
        dispose() {},
      }),
      watchList: () => ({ ulids: () => [], error: () => null, loading: () => false, dispose() {} }),
      watchDetail: () => ({ ulid: () => "P", error: () => null, dispose() {} }),
      applyWrite: vi.fn(),
      refreshTrees: vi.fn(async () => {}),
      subscribe: none,
    },
    stores: {
      workspace: () => ws,
      workspaces: () => [ws],
      readOnly: () => readOnly,
      tone: () => "purple",
      subscribe: none,
    },
    schema: { get: () => schema, load: async () => schema, subscribe: none },
  };
  return {
    api,
    refresh: (h.core as { cache: { refreshTrees: ReturnType<typeof vi.fn> } }).cache.refreshTrees,
  };
}

const row = (u: string) => h.rows.get(u) as Opts;

beforeEach(() => {
  h.toast.mockReset();
  h.rows.clear();
  h.attached = 0;
  h.dragEnd = undefined;
});

describe("TreeView drag and drop", () => {
  it("registers every row for dragging", () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect([...h.rows.keys()].sort()).toEqual(["E8", "T1", "T7", "T9"]);
  });

  it("does not let a row of a read-only Store be dragged, and turns a drop on it down", async () => {
    const { api } = setup("the remote cannot be written");
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(row("T7").locked).toBe(true);
    row("T7").onDrop("T9", "after");
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
    expect(h.toast.mock.calls[0]?.[0].message).toContain("閲覧のみ");
    expect(h.toast.mock.calls[0]?.[0].message).toContain("remote に書き込めません");
    expect(api.POST).not.toHaveBeenCalled();
  });

  it("lets a row of a Store that can be written be dragged", () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(row("T7").locked).toBe(false);
  });

  it("tells the row whether it is open with children", () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(row("E8").openChildren).toBe(true);
    expect(row("T7").openChildren).toBe(false);
  });

  it("attaches each row once, however the screen is redrawn", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const n = h.attached;
    row("T7").onOver("after", "T9");
    row("T7").onOver(null, null);
    await tick();
    expect(h.attached).toBe(n);
  });

  it("clears the drag state when the drag ends anywhere", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    row("T9").onDragStart();
    row("T7").onOver("child", "T9");
    const el = screen.getByText("負荷試験").closest('[role="treeitem"]');
    await waitFor(() => expect(el?.className).toContain("dragging"));
    await waitFor(() => expect(el).not.toHaveAttribute("data-drop", "reject"));
    h.dragEnd?.();
    await waitFor(() => expect(el?.className).not.toContain("dragging"));
    await waitFor(() =>
      expect(screen.getByText("スキーマを再生成").closest('[role="treeitem"]')).not.toHaveAttribute(
        "data-drop",
      ),
    );
  });

  it("marks the row a drop would be refused on, and explains why", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    row("E8").onDragStart();
    row("T7").onOver("child", "E8");
    expect(await screen.findByText("ここには置けません")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("エピックはタスクの下に置けません");
    expect(screen.getByRole("status")).toHaveTextContent("（shared の schema.yaml の kinds）");
    const tree = screen.getByRole("tree");
    expect(tree).toHaveClass("refusing");
    row("T7").onOver(null, null);
    await waitFor(() => expect(screen.queryByText("ここには置けません")).toBeNull());
    // The room stays while the drag goes on, so the rows do not move.
    expect(tree).toHaveClass("refusing");
    h.dragEnd?.();
    await waitFor(() => expect(tree).not.toHaveClass("refusing"));
  });

  it("marks a place a drop would be accepted on, and says nothing", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    row("T9").onOver("after", "T7");
    const el = screen.getByText("負荷試験").closest('[role="treeitem"]');
    await waitFor(() => expect(el).toHaveAttribute("data-drop", "after"));
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("refuses a drop with a toast and does not change the data", async () => {
    const { api } = setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    row("T7").onDrop("E8", "child");
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
    expect(h.toast.mock.calls[0]?.[0].message).toContain("エピックはタスクの下に置けません");
    expect(api.POST).not.toHaveBeenCalled();
  });

  it("drops the drops still waiting when one is refused", async () => {
    const { api, refresh } = setup();
    let shown: () => void = () => {};
    refresh.mockReturnValueOnce(new Promise<void>((resolve) => (shown = resolve)));
    api.POST.mockResolvedValue(ok({ task: task("T9", "task", "P"), commit: "c1" }));
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    row("E8").onDrop("T9", "before"); // goes through, and waits for the tree
    row("T7").onDrop("E8", "child"); // refused: an epic under a task
    row("E8").onDrop("T7", "before"); // waits behind the refused one
    await waitFor(() => expect(api.POST).toHaveBeenCalledTimes(1));
    shown();
    await waitFor(() => expect(h.toast).toHaveBeenCalledTimes(1));
    await new Promise((r) => setTimeout(r, 40));
    expect(api.POST).toHaveBeenCalledTimes(1);
  });

  it("makes a child on an accepted drop", async () => {
    const { api } = setup();
    api.POST.mockResolvedValue(ok({ task: task("T9", "task", "E8"), commit: "c1" }));
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    row("E8").onDrop("T9", "child");
    await waitFor(() => expect(api.POST).toHaveBeenCalled());
    expect(api.POST).toHaveBeenCalledWith("/api/tasks/{id}/move", {
      params: { path: { id: "T9" }, query: { store: "shared" } },
      body: { parent: "E8" },
    });
  });

  it("sends a drop only after the tree shows the move before it", async () => {
    const { api, refresh } = setup();
    let shown: () => void = () => {};
    refresh.mockReturnValueOnce(new Promise<void>((resolve) => (shown = resolve)));
    api.POST.mockResolvedValue(ok({ task: task("T9", "task", "P"), commit: "c1" }));
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    row("E8").onDrop("T9", "before");
    row("E8").onDrop("T7", "before");
    await waitFor(() => expect(api.POST).toHaveBeenCalledTimes(1));
    await new Promise((r) => setTimeout(r, 40));
    expect(api.POST).toHaveBeenCalledTimes(1);
    shown();
    await waitFor(() => expect(api.POST).toHaveBeenCalledTimes(2));
  });

  it("reorders next to a sibling", async () => {
    const { api } = setup();
    api.POST.mockResolvedValue(ok({ task: task("T9", "task", "P"), commit: "c1" }));
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    row("E8").onDrop("T9", "before");
    await waitFor(() => expect(api.POST).toHaveBeenCalled());
    expect(api.POST).toHaveBeenCalledWith("/api/tasks/{id}/move", {
      params: { path: { id: "T9" }, query: { store: "shared" } },
      body: { parent: "P", before: "E8" },
    });
  });

  it("shows a toast when the server refuses the move", async () => {
    const { api } = setup();
    api.POST.mockResolvedValue({
      error: {
        type: "move.invalid",
        title: "Invalid",
        status: 422,
        detail: "x is not under the same parent",
      },
      response: new Response(null, { status: 422 }),
    });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    row("E8").onDrop("T9", "before");
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
    expect(h.toast.mock.calls[0]?.[0].message).toContain("ここには移せません");
    expect(h.toast.mock.calls[0]?.[0].message).toContain("x is not under the same parent");
  });

  it("says nothing and sends nothing for a drop on itself", async () => {
    const { api } = setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    row("T7").onDrop("T7", "child");
    await tick();
    expect(api.POST).not.toHaveBeenCalled();
    expect(h.toast).not.toHaveBeenCalled();
  });

  it("does not open the new parent when the move fails", async () => {
    const { api } = setup();
    api.POST.mockResolvedValue({
      error: { type: "move.invalid", title: "Invalid", status: 422, detail: "x" },
      response: new Response(null, { status: 422 }),
    });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    // fold E8, then drop T9 into it
    await fireEvent.click(screen.getByRole("button", { name: /折りたたむ: リリース作業/ }));
    expect(screen.queryByText("子")).toBeNull();
    row("E8").onDrop("T9", "child");
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
    expect(screen.queryByText("子")).toBeNull();
  });

  it("opens the new parent once the move has gone through", async () => {
    const { api } = setup();
    api.POST.mockResolvedValue(ok({ task: task("T9", "task", "E8"), commit: "c1" }));
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(screen.getByRole("button", { name: /折りたたむ: リリース作業/ }));
    expect(screen.queryByText("子")).toBeNull();
    row("E8").onDrop("T9", "child");
    expect(await screen.findByText("子")).toBeInTheDocument();
  });

  it("sends root: true next to a top-level task", async () => {
    const { api } = setup();
    api.POST.mockResolvedValue(ok({ task: task("E8", "epic", null), commit: "c1" }));
    render(TreeView, { view: { kind: "workspace", store: "shared", ulid: "W1" } });
    row("P").onDrop("E8", "after");
    await waitFor(() => expect(api.POST).toHaveBeenCalled());
    expect(api.POST).toHaveBeenCalledWith("/api/tasks/{id}/move", {
      params: { path: { id: "E8" }, query: { store: "shared" } },
      body: { root: true, after: "P" },
    });
  });
});
