import { render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { schema, task } from "./fixtures";

type Opts = {
  ulid: string;
  onDragStart: () => void;
  onDragEnd: () => void;
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
    rowDnd: (o: Opts) => () => {
      h.rows.set(o.ulid, o);
      return () => {};
    },
    treeAutoScroll: () => () => () => {},
  };
});

import TreeView from "./TreeView.svelte";

const ok = (data: unknown) => ({ data, response: new Response(null, { status: 200 }) });

function setup() {
  const tasks = new Map([
    ["P", task("P", "project", null, { context: true, id: "REL-15", title: "リリース準備 v2.4" })],
    ["E8", task("E8", "epic", "P", { id: "REL-8", title: "リリース作業" })],
    ["T7", task("T7", "task", "P", { id: "REL-7", title: "スキーマを再生成" })],
    ["T9", task("T9", "task", "P", { id: "REL-9", title: "負荷試験" })],
  ]);
  const roots = [
    {
      ulid: "P",
      children: [
        { ulid: "E8", children: [] },
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
      subscribe: none,
    },
    stores: { workspace: () => ws, workspaces: () => [ws], tone: () => "purple", subscribe: none },
    schema: { get: () => schema, load: async () => schema, subscribe: none },
  };
  return { api };
}

const row = (u: string) => h.rows.get(u) as Opts;

beforeEach(() => {
  h.toast.mockReset();
  h.rows.clear();
});

describe("TreeView drag and drop", () => {
  it("registers every row for dragging", () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect([...h.rows.keys()].sort()).toEqual(["E8", "T7", "T9"]);
  });

  it("marks the row a drop would be refused on, and explains why", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    row("E8").onDragStart();
    row("T7").onOver("child", "E8");
    expect(await screen.findByText("ここには置けません")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("エピックはタスクの下に置けません");
    expect(screen.getByRole("status")).toHaveTextContent("（shared の schema.yaml の kinds）");
    row("T7").onOver(null, null);
    await waitFor(() => expect(screen.queryByText("ここには置けません")).toBeNull());
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
  });
});
