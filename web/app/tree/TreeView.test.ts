import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../core/api/client";
import { selection } from "../selection.svelte";
import { emitter } from "../test-utils";
import { schema, task } from "./fixtures";

const h = vi.hoisted(() => ({
  core: undefined as unknown,
  go: vi.fn(),
  toast: vi.fn(),
  // The window of the virtualizer: where it starts, and how long it takes to follow scrollToIndex.
  vstart: 0,
  vdelay: 0,
}));

vi.mock("../context", () => ({ core: () => h.core }));
vi.mock("../router", () => ({
  router: {
    route: { name: "project", ulid: "P" },
    go: h.go,
    href: (r: { name: string; ulid: string }) => (r.name === "home" ? "/" : `/tasks/${r.ulid}`),
  },
  onLinkClick: (e: MouseEvent, path: string) => {
    e.preventDefault();
    h.go(path);
  },
}));
vi.mock("../toast.svelte", () => ({ toast: h.toast }));
vi.mock("@tanstack/svelte-virtual", () => ({
  createVirtualizer: () => ({
    subscribe(fn: (v: unknown) => void) {
      const inst = {
        getVirtualItems: () =>
          Array.from({ length: 10 }, (_, i) => {
            const index = h.vstart + i;
            return { index, start: index * 39, size: 39, key: index };
          }),
        getTotalSize: () => 39 * 250,
        setOptions() {},
        scrollToIndex(i: number) {
          if (h.vdelay === 0) return;
          setTimeout(() => {
            h.vstart = Math.max(0, i - 5);
            fn(inst);
          }, h.vdelay);
        },
      };
      fn(inst);
      return () => {};
    },
    setOptions() {},
  }),
}));

import TreeView from "./TreeView.svelte";

const ok = (data: unknown) => ({ data, response: new Response(null, { status: 200 }) });

function setup(
  over: {
    roots?: unknown;
    error?: unknown;
    many?: number;
    noSchema?: boolean;
    schemaFails?: boolean;
    noDone?: boolean;
    blocked?: string[];
    noProject?: boolean;
    noWorkspace?: boolean;
    detailError?: unknown;
  } = {},
) {
  const tasks = new Map<string, ReturnType<typeof task>>([
    ["P", task("P", "project", null, { context: true, id: "REL-15", title: "リリース準備 v2.4" })],
    ["E1", task("E1", "epic", "P", { id: "REL-1", title: "API v2 への移行" })],
    ["T1", task("T1", "task", "E1", { id: "REL-4", title: "移行ガイドを書く" })],
    [
      "T3",
      task("T3", "task", "P", {
        id: "REL-6",
        title: "Web のクライアントを更新",
        ...(over.blocked ? { blocked_by: [{ ulid: "B9", id: "REL-9", title: "待たせる" }] } : {}),
      }),
    ],
  ]);
  if (over.noProject) tasks.delete("P");
  const usedSchema = over.noDone
    ? ({
        ...schema,
        statuses: schema.statuses?.filter((s) => s.category !== "done"),
      } as typeof schema)
    : schema;
  let roots = over.roots ?? [
    {
      ulid: "P",
      children: [
        { ulid: "E1", children: [{ ulid: "T1", children: [] }] },
        { ulid: "T3", children: [] },
      ],
    },
  ];
  if (over.many) {
    const kids = Array.from({ length: over.many }, (_, i) => {
      tasks.set(`M${i}`, task(`M${i}`, "task", "P", { id: `REL-${100 + i}`, title: `many ${i}` }));
      return { ulid: `M${i}`, children: [] };
    });
    roots = [{ ulid: "P", children: kids }];
  }
  const ev = emitter();
  let schemaNow: typeof schema | undefined = over.noSchema ? undefined : usedSchema;
  const watchTree = vi.fn(() => ({
    roots: () => roots,
    error: () => over.error ?? null,
    loading: () => false,
    dispose() {},
  }));
  const seen: (() => void)[] = [];
  const api = { POST: vi.fn(), PATCH: vi.fn() };
  const core = {
    api,
    cache: {
      task: (u: string) => tasks.get(u),
      live: () => undefined,
      watchTree,
      watchList: vi.fn((q: { q: string }) => ({
        ulids: () => (q.q.includes("is:blocked") ? (over.blocked ?? []) : []),
        error: () => null,
        loading: () => false,
        dispose() {},
      })),
      watchDetail: vi.fn(() => ({
        ulid: () => "P",
        error: () => over.detailError ?? null,
        dispose() {},
      })),
      applyWrite: vi.fn(),
      // The tree is read again once reorder() is called.
      refreshTrees: vi.fn(() => new Promise<void>((resolve) => seen.push(resolve))),
      subscribe: (fn: () => void) => ev.subscribe(fn),
    },
    stores: {
      loaded: () => true,
      workspace: () =>
        over.noWorkspace
          ? undefined
          : {
              store: "shared",
              tone: "purple",
              ulid: "W1",
              name: "release",
              prefix: "REL",
              projects: [],
            },
      workspaces: () => [
        {
          store: "shared",
          tone: "purple",
          ulid: "W1",
          name: "release",
          prefix: "REL",
          projects: [],
        },
      ],
      tone: () => "purple",
      subscribe: (fn: () => void) => ev.subscribe(fn),
    },
    schema: {
      get: () => schemaNow,
      load: async () => {
        if (over.schemaFails) throw new Error("schema.yaml が読めません");
        return usedSchema;
      },
      subscribe: (fn: () => void) => ev.subscribe(fn),
    },
  };
  h.core = core;
  return {
    core,
    api,
    watchTree,
    // The tree is read again
    emit: () => ev.emit(),
    // The tree is read again, and the order of the children of P is now this
    reorder(order: string[]) {
      roots = [{ ulid: "P", children: order.map((u) => ({ ulid: u, children: [] })) }];
      ev.emit();
      for (const done of seen.splice(0)) done();
    },
    // The schema of the store arrives
    loadSchema() {
      schemaNow = usedSchema;
      ev.emit();
    },
  };
}

beforeEach(() => {
  h.vstart = 0;
  h.vdelay = 0;
  h.go.mockReset();
  h.toast.mockReset();
  selection.set(null);
});

describe("TreeView", () => {
  it("shows the rows of the project with the count of matched tasks", () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.getByText("API v2 への移行")).toBeInTheDocument();
    expect(screen.getByText("移行ガイドを書く")).toBeInTheDocument();
    expect(screen.getByText("3 件 · 中止 0 件を除外")).toBeInTheDocument();
  });

  it("selects a row on click", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(screen.getByText("Web のクライアントを更新"));
    expect(selection.ulid).toBe("T3");
  });

  it("keeps the title typed before the schema is read, and adds the task once it is", async () => {
    const { api, loadSchema } = setup({ noSchema: true });
    api.POST.mockImplementation(async (path: string) =>
      path === "/api/tasks"
        ? ok({ task: task("N1", "task", "P", { id: "REL-9", title: "早い" }) })
        : ok({}),
    );
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(screen.getByRole("button", { name: /タスクを追加/ }));
    const input = screen.getByRole("textbox", { name: "新しいタスクのタイトル" });
    expect(input).toHaveAttribute("readonly");
    expect(input).toHaveAttribute("placeholder", "読み込み中…");
    await fireEvent.input(input, { target: { value: "早い" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(api.POST).not.toHaveBeenCalled();
    expect(input).toHaveValue("早い");

    loadSchema();
    await waitFor(() => expect(input).not.toHaveAttribute("readonly"));
    await fireEvent.keyDown(input, { key: "Enter" });
    await waitFor(() => expect(api.POST).toHaveBeenCalledWith("/api/tasks", expect.anything()));
  });

  it("runs a key pressed before the schema is read once it is, in the order pressed", async () => {
    const { api, loadSchema } = setup({ noSchema: true });
    api.PATCH.mockResolvedValue(
      ok({ task: task("T3", "task", "P", { status: "done" }), commit: "c3" }),
    );
    api.POST.mockResolvedValue(ok({ task: task("T3", "task", "E1"), commit: "c4" }));
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen
      .getByText("Web のクライアントを更新")
      .closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: " " });
    await fireEvent.keyDown(row, { key: "Tab" });
    expect(api.PATCH).not.toHaveBeenCalled();
    expect(api.POST).not.toHaveBeenCalled();

    loadSchema();
    await waitFor(() => expect(api.POST).toHaveBeenCalled());
    expect(api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "T3" }, query: { store: "shared" } },
      body: { status: "done" },
    });
    expect(api.POST).toHaveBeenCalledWith("/api/tasks/{id}/move", {
      params: { path: { id: "T3" }, query: { store: "shared" } },
      body: { parent: "E1" },
    });
  });

  it("does not wait for the schema for the keys that do not need it", async () => {
    setup({ noSchema: true });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen.getByText("API v2 への移行").closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: "ArrowDown" });
    expect(selection.ulid).toBe("T1");
  });

  it("says so when the schema cannot be read, and drops the keys that waited", async () => {
    const { api } = setup({ noSchema: true, schemaFails: true });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen
      .getByText("Web のクライアントを更新")
      .closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: " " });
    await waitFor(() => expect(h.toast).toHaveBeenCalledTimes(1));
    await fireEvent.keyDown(row, { key: " " });
    expect(h.toast).toHaveBeenCalledTimes(2);
    expect(api.PATCH).not.toHaveBeenCalled();
  });

  it("folds a row with its chevron", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(screen.getByRole("button", { name: "折りたたむ: API v2 への移行" }));
    expect(screen.queryByText("移行ガイドを書く")).toBeNull();
  });

  it("adds a task below the selected row with Enter and puts it in place", async () => {
    const { api } = setup();
    api.POST.mockImplementation(async (path: string) =>
      path === "/api/tasks"
        ? ok({ task: task("N1", "task", "P", { id: "REL-20", title: "新しい" }), commit: "c1" })
        : ok({ task: task("N1", "task", "P"), commit: "c2" }),
    );
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen.getByText("API v2 への移行").closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: "Enter" });
    const input = await screen.findByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）");
    await fireEvent.input(input, { target: { value: "新しい" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    await waitFor(() => expect(api.POST).toHaveBeenCalled());
    expect(api.POST).toHaveBeenCalledTimes(1);
    expect(api.POST).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { store: "shared" } },
      body: { title: "新しい", kind: "task", parent: "P", after: "E1" },
    });
  });

  it("completes the selected task with Space", async () => {
    const { api } = setup();
    api.PATCH.mockResolvedValue(
      ok({ task: task("T3", "task", "P", { status: "done" }), commit: "c3" }),
    );
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen
      .getByText("Web のクライアントを更新")
      .closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: " " });
    await waitFor(() => expect(api.PATCH).toHaveBeenCalled());
    expect(api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "T3" }, query: { store: "shared" } },
      body: { status: "done" },
    });
  });

  it("does not run the keys while typing in the filter", async () => {
    const { api } = setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen
      .getByText("Web のクライアントを更新")
      .closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    const filter = screen.getByLabelText("フィルタ（hh ls と同じクエリ）");
    await fireEvent.keyDown(filter, { key: " " });
    expect(api.PATCH).not.toHaveBeenCalled();
  });

  it("asks for the base query plus the typed query, and shows the hh ls line", async () => {
    const { watchTree } = setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(watchTree).toHaveBeenLastCalledWith({
      q: "project:P -status:canceled",
      tree: true,
      store: "shared",
    });
    const filter = screen.getByLabelText("フィルタ（hh ls と同じクエリ）");
    await fireEvent.input(filter, { target: { value: "assignee:@me" } });
    await fireEvent.keyDown(filter, { key: "Enter" });
    await waitFor(() =>
      expect(watchTree).toHaveBeenLastCalledWith({
        q: "project:P assignee:@me",
        tree: true,
        store: "shared",
      }),
    );
    expect(screen.getByText(/hh ls .*project:REL-15 assignee:@me/)).toBeInTheDocument();
  });

  it("shows the message of query.invalid under the filter", () => {
    const error = Object.assign(Object.create(ApiError.prototype), {
      status: 400,
      type: "query.invalid",
      detail: "term 1 (foo:bar): unknown key",
      args: {},
      body: {},
    });
    setup({ error });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.getByRole("alert")).toHaveTextContent("term 1 (foo:bar): unknown key");
  });

  it("does not run the row keys from the chevron button", async () => {
    const { api } = setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(screen.getByText("Web のクライアントを更新"));
    const chev = screen.getByRole("button", { name: "折りたたむ: API v2 への移行" });
    await fireEvent.keyDown(chev, { key: " " });
    await fireEvent.keyDown(chev, { key: "Enter" });
    expect(api.PATCH).not.toHaveBeenCalled();
    expect(screen.queryByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）")).toBeNull();
  });

  it("gives the focus back to the row when the add row is closed", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen.getByText("API v2 への移行").closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: "Enter" });
    const input = await screen.findByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）");
    await fireEvent.keyDown(input, { key: "Escape" });
    await waitFor(() => expect(row).toHaveFocus());
    await fireEvent.keyDown(row, { key: "Enter" });
    const again = await screen.findByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）");
    await fireEvent.keyDown(again, { key: "Enter" }); // an empty title closes it
    await waitFor(() => expect(row).toHaveFocus());
  });

  it("adds right below the selected row even when it is the last one shown, in one request", async () => {
    const { api } = setup();
    api.POST.mockResolvedValue(ok({ task: task("N1", "task", "P"), commit: "c1" }));
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen
      .getByText("Web のクライアントを更新")
      .closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: "Enter" });
    const input = await screen.findByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）");
    await fireEvent.input(input, { target: { value: "直下" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    await waitFor(() => expect(api.POST).toHaveBeenCalled());
    expect(api.POST).toHaveBeenCalledTimes(1);
    expect(api.POST).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { store: "shared" } },
      body: { title: "直下", kind: "task", parent: "P", after: "T3" },
    });
    expect(api.POST).not.toHaveBeenCalledWith("/api/tasks/{id}/move", expect.anything());
  });

  it("does not open the input again when Esc closed it while the task was being made", async () => {
    const { api } = setup();
    let answer: (v: unknown) => void = () => {};
    api.POST.mockReturnValue(new Promise((resolve) => (answer = resolve)));
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen.getByText("API v2 への移行").closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: "Enter" });
    const input = await screen.findByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）");
    await fireEvent.input(input, { target: { value: "遅い" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    await waitFor(() => expect(api.POST).toHaveBeenCalled());
    await fireEvent.keyDown(input, { key: "Escape" });
    await waitFor(() => expect(row).toHaveFocus());

    answer(ok({ task: task("N1", "task", "E1"), commit: "c1" }));
    await new Promise((r) => setTimeout(r, 50));
    expect(screen.queryByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）")).toBeNull();
    expect(row).toHaveFocus();
  });

  describe("keys that move a task, pressed one after another", () => {
    async function start(over: { many: number }) {
      const t = setup(over);
      render(TreeView, { view: { kind: "project", ulid: "P" } });
      const row = screen.getByText("many 3").closest('[role="treeitem"]') as HTMLElement;
      await fireEvent.click(row);
      return { ...t, row };
    }
    const moved = (to: object) => ({
      params: { path: { id: "M3" }, query: { store: "shared" } },
      body: to,
    });

    it("sends each press in turn, deciding on the rows the move before it left", async () => {
      const { api, row, reorder } = await start({ many: 4 });
      api.POST.mockResolvedValue(ok({ task: task("M3", "task", "P"), commit: "c" }));
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await waitFor(() => expect(api.POST).toHaveBeenCalledTimes(1));
      expect(api.POST).toHaveBeenLastCalledWith("/api/tasks/{id}/move", moved({ before: "M2" }));
      // The next one waits until the tree shows the first move.
      await new Promise((r) => setTimeout(r, 60));
      expect(api.POST).toHaveBeenCalledTimes(1);
      reorder(["M0", "M1", "M3", "M2"]);
      await waitFor(() => expect(api.POST).toHaveBeenCalledTimes(2));
      expect(api.POST).toHaveBeenLastCalledWith("/api/tasks/{id}/move", moved({ before: "M1" }));
      reorder(["M0", "M3", "M1", "M2"]);
      await waitFor(() => expect(api.POST).toHaveBeenCalledTimes(3));
      expect(api.POST).toHaveBeenLastCalledWith("/api/tasks/{id}/move", moved({ before: "M0" }));
    });

    it("sends the next press at once after a move that changed nothing", async () => {
      const { api, row, core } = await start({ many: 4 });
      api.POST.mockResolvedValue(ok({ task: task("M3", "task", "P"), commit: null }));
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await waitFor(() => expect(api.POST).toHaveBeenCalledTimes(2));
      expect(core.cache.refreshTrees).not.toHaveBeenCalled();
    });

    it("counts a tree that cannot be read again as a failure, once, and drops the rest", async () => {
      const { api, row, core } = await start({ many: 4 });
      api.POST.mockResolvedValue(ok({ task: task("M3", "task", "P"), commit: "c" }));
      vi.mocked(core.cache.refreshTrees).mockRejectedValueOnce(new Error("読めません"));
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await waitFor(() => expect(h.toast).toHaveBeenCalledTimes(1));
      await new Promise((r) => setTimeout(r, 60));
      expect(api.POST).toHaveBeenCalledTimes(1);
      expect(h.toast).toHaveBeenCalledTimes(1);
    });

    it("sends nothing more, and says nothing, once the view is gone", async () => {
      const { api, row, reorder } = await start({ many: 4 });
      api.POST.mockResolvedValue(ok({ task: task("M3", "task", "P"), commit: "c" }));
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await waitFor(() => expect(api.POST).toHaveBeenCalledTimes(1));
      cleanup();
      reorder(["M0", "M1", "M3", "M2"]);
      await new Promise((r) => setTimeout(r, 60));
      expect(api.POST).toHaveBeenCalledTimes(1);
      expect(h.toast).not.toHaveBeenCalled();
    });

    it("drops the moves still waiting when one fails, and says so once", async () => {
      const { api, row, reorder } = await start({ many: 4 });
      api.POST.mockImplementation(
        () => new Promise((_, reject) => setTimeout(() => reject(new Error("書き込めません")), 30)),
      );
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await waitFor(() => expect(h.toast).toHaveBeenCalledTimes(1));
      reorder(["M0", "M1", "M3", "M2"]);
      await new Promise((r) => setTimeout(r, 60));
      expect(api.POST).toHaveBeenCalledTimes(1);
      expect(h.toast).toHaveBeenCalledTimes(1);
    });

    it("drops the moves still waiting when the selection leaves the row", async () => {
      const { api, row, reorder } = await start({ many: 4 });
      api.POST.mockResolvedValue(ok({ task: task("M3", "task", "P"), commit: "c" }));
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await fireEvent.keyDown(row, { key: "ArrowUp", altKey: true });
      await waitFor(() => expect(api.POST).toHaveBeenCalledTimes(1));
      selection.set("M0");
      reorder(["M0", "M1", "M3", "M2"]);
      await new Promise((r) => setTimeout(r, 60));
      expect(api.POST).toHaveBeenCalledTimes(1);
    });
  });

  it("waits for a row the window draws late, and focuses it", async () => {
    setup({ many: 250 });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const first = screen.getAllByRole("treeitem")[0] as HTMLElement;
    await fireEvent.click(first);
    selection.set("M100");
    h.vdelay = 30;
    await fireEvent.keyDown(first, { key: "ArrowDown" });
    await waitFor(() => {
      expect(document.activeElement?.getAttribute("data-ulid")).toBe("M101");
    });
  });

  it("keeps the title and the focus when the task cannot be made", async () => {
    const { api } = setup();
    api.POST.mockRejectedValue(new Error("書き込めません"));
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen.getByText("API v2 への移行").closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: "Enter" });
    const input = await screen.findByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）");
    await fireEvent.input(input, { target: { value: "残る" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    await waitFor(() => expect(h.toast).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(input).toHaveValue("残る"));
    expect(input).toHaveFocus();
  });

  it("lets a late wait for a row give way to a newer one", async () => {
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
    await waitFor(() => expect(document.activeElement?.getAttribute("data-ulid")).toBe("M199"));
    // The first request draws its window late (and moves the mock window back): it must not
    // then take the focus for M101.
    await new Promise((r) => setTimeout(r, 200));
    expect(document.activeElement?.getAttribute("data-ulid")).not.toBe("M101");
  });

  it("says so, with a way home, when the project is not there", async () => {
    const detailError = Object.assign(Object.create(ApiError.prototype), {
      status: 404,
      type: "task.not_found",
      detail: "no such task",
      args: { id: "P" },
      body: {},
    });
    setup({ noProject: true, detailError });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.getByText("プロジェクトが見つかりません")).toBeInTheDocument();
    expect(screen.queryByText("読み込み中…")).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "ホームへ戻る" }));
    expect(h.go).toHaveBeenCalledWith("/");
  });

  it("says why when the project cannot be shown for another reason", () => {
    const detailError = Object.assign(Object.create(ApiError.prototype), {
      status: 500,
      type: "internal",
      detail: "boom",
      args: {},
      body: {},
    });
    setup({ noProject: true, detailError });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.queryByText("プロジェクトが見つかりません")).toBeNull();
    expect(screen.queryByText("読み込み中…")).toBeNull();
  });

  it("says so, with a way home, when the workspace is not there", async () => {
    setup({ noWorkspace: true });
    render(TreeView, { view: { kind: "workspace", store: "shared", ulid: "W9" } });
    expect(screen.getByText("ワークスペース「W9」が見つかりません")).toBeInTheDocument();
    expect(screen.queryByText("読み込み中…")).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "ホームへ戻る" }));
    expect(h.go).toHaveBeenCalledWith("/");
  });

  it("forgets the selection of another screen when it comes up", () => {
    selection.set("T1");
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(selection.ulid).toBeNull();
  });

  it("keeps one row for the Tab key when the selection is not a row of the screen", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    selection.set("NOT-HERE");
    await waitFor(() =>
      expect(screen.getAllByRole("treeitem")[0]).toHaveAttribute("tabindex", "0"),
    );
    expect(
      screen.getAllByRole("treeitem").filter((r) => r.getAttribute("tabindex") === "0"),
    ).toHaveLength(1);
    selection.set("T3");
    await waitFor(() =>
      expect(
        screen.getByText("Web のクライアントを更新").closest('[role="treeitem"]'),
      ).toHaveAttribute("tabindex", "0"),
    );
  });

  it("keeps the input where it was until the new task is in the tree", async () => {
    const { api } = setup();
    api.POST.mockResolvedValue(
      ok({ task: task("N1", "task", "P", { id: "REL-20", title: "一件目" }), commit: "c1" }),
    );
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen.getByText("API v2 への移行").closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: "Enter" });
    const input = await screen.findByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）");
    await fireEvent.input(input, { target: { value: "一件目" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    await waitFor(() => expect(api.POST).toHaveBeenCalledTimes(1));
    // the tree has not read the new task yet: the input stays below the epic and its child
    await waitFor(() => expect(input).toHaveValue(""));
    const next = screen.getByPlaceholderText("タイトルを入力して Enter（Esc で閉じる）");
    const order = (el: Element) =>
      screen.getByText("Web のクライアントを更新").compareDocumentPosition(el);
    expect(order(next) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy();
    expect(next).toBe(input);
    expect(input).toHaveFocus();
  });

  it("opens a link of the breadcrumb through the router", async () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(screen.getByRole("link", { name: /release/ }));
    expect(h.go).toHaveBeenCalledWith("/tasks/W1");
  });

  it("writes the keys of the footer as the platform does", () => {
    setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.getByText("Shift+Tab")).toBeInTheDocument();
    expect(screen.getByText("Alt+↑")).toBeInTheDocument();
    expect(screen.getByText("Alt+↓")).toBeInTheDocument();
  });

  it("does not take the focus from an element the user moved it to", async () => {
    const { emit } = setup();
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen
      .getByText("Web のクライアントを更新")
      .closest('[role="treeitem"]') as HTMLElement;
    row.focus();
    expect(row).toHaveFocus();
    const filter = screen.getByLabelText("フィルタ（hh ls と同じクエリ）");
    filter.focus();
    emit();
    await new Promise((r) => setTimeout(r, 30));
    expect(filter).toHaveFocus();
  });

  it("opens the new parent only once the task has moved", async () => {
    const { api } = setup();
    api.POST.mockRejectedValueOnce(new Error("書き込めません"));
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    await fireEvent.click(screen.getByRole("button", { name: "折りたたむ: API v2 への移行" }));
    expect(screen.queryByText("移行ガイドを書く")).toBeNull();
    const row = screen
      .getByText("Web のクライアントを更新")
      .closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: "Tab" }); // under the epic; it fails
    await waitFor(() => expect(h.toast).toHaveBeenCalledTimes(1));
    expect(screen.queryByText("移行ガイドを書く")).toBeNull();
    api.POST.mockResolvedValueOnce(ok({ task: task("T3", "task", "E1"), commit: "c9" }));
    await fireEvent.keyDown(row, { key: "Tab" });
    expect(await screen.findByText("移行ガイドを書く")).toBeInTheDocument();
  });

  it("sets the status chosen in the S menu", async () => {
    const { api } = setup();
    api.PATCH.mockResolvedValue(
      ok({ task: task("T3", "task", "P", { status: "doing" }), commit: "c3" }),
    );
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen
      .getByText("Web のクライアントを更新")
      .closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: "s" });
    await fireEvent.click(await screen.findByRole("menuitem", { name: /doing/ }));
    await waitFor(() => expect(api.PATCH).toHaveBeenCalled());
    expect(api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "T3" }, query: { store: "shared" } },
      body: { status: "doing" },
    });
  });

  it("sets the kind chosen in the K menu", async () => {
    const { api } = setup();
    api.PATCH.mockResolvedValue(
      ok({ task: task("T3", "bug", "P", { status: "todo" }), commit: "c3" }),
    );
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen
      .getByText("Web のクライアントを更新")
      .closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: "k" });
    await fireEvent.click(await screen.findByRole("menuitem", { name: /バグ/ }));
    await waitFor(() => expect(api.PATCH).toHaveBeenCalled());
    expect(api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "T3" }, query: { store: "shared" } },
      body: { kind: "bug" },
    });
  });

  it("says why Space does nothing when the schema has no done status", async () => {
    const { api } = setup({ noDone: true });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const row = screen
      .getByText("Web のクライアントを更新")
      .closest('[role="treeitem"]') as HTMLElement;
    await fireEvent.click(row);
    await fireEvent.keyDown(row, { key: " " });
    await waitFor(() => expect(h.toast).toHaveBeenCalledTimes(1));
    expect(h.toast.mock.calls[0]?.[0].message).toContain("完了にできません");
    expect(api.PATCH).not.toHaveBeenCalled();
  });

  it("shows the blocker's ID on a task the server says is blocked", () => {
    setup({ blocked: ["T3"] });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.getByText(/REL-9 待ち/)).toBeInTheDocument();
  });

  it("draws only the window of rows when the list is long", () => {
    setup({ many: 250 });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.getAllByRole("treeitem")).toHaveLength(10);
  });
});

describe("TreeView empty states", () => {
  const empty = [{ ulid: "P", children: [] }];

  it("says that an empty project has no task yet, with no button for a screen that is not there", () => {
    setup({ roots: empty });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.getByText("リリース準備 v2.4 にはまだタスクがありません")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /タスクを作成/ })).toBeNull();
  });

  it("says that nothing matches a filter and clears it", async () => {
    setup({ roots: empty });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    const filter = screen.getByLabelText("フィルタ（hh ls と同じクエリ）") as HTMLInputElement;
    await fireEvent.input(filter, { target: { value: "assignee:@me" } });
    await fireEvent.keyDown(filter, { key: "Enter" });
    expect(await screen.findByText("一致するタスクはありません")).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "フィルタを解除" }));
    await waitFor(() => expect(filter.value).toBe(""));
  });

  it("shows no empty state when the query is wrong", () => {
    const error = Object.assign(Object.create(ApiError.prototype), {
      status: 400,
      type: "query.invalid",
      detail: "x",
      args: {},
      body: {},
    });
    setup({ roots: empty, error });
    render(TreeView, { view: { kind: "project", ulid: "P" } });
    expect(screen.queryByText("一致するタスクはありません")).toBeNull();
    expect(screen.queryByText(/にはまだタスクがありません/)).toBeNull();
  });
});
