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
  cards: new Map<
    string,
    Get<{ ulid: string; lane: string; onDragStart: () => void; locked?: boolean }>
  >(),
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
  cardDnd:
    (get: Get<{ ulid: string; lane: string; onDragStart: () => void; locked?: boolean }>) => () => {
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
  // The Stores that cannot be written, with the raw reason of the directory.
  readOnly?: Record<string, string>;
  // The Stores the GUI leaves out.
  hidden?: string[];
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
      readOnly: (s: string) => o.readOnly?.[s] ?? null,
      hidden: (s: string) => o.hidden?.includes(s) ?? false,
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
    crumbTitle: "release",
    emptyProps: { scope: "workspace", name: "release" },
    notFoundProps: { scope: "workspace", name: "W1" },
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

  describe("hidden Stores", () => {
    const two = () =>
      setup({
        stores: ["shared", "work"],
        schemas: { shared: schema, work: schema },
        canceled: ["X", "Y"],
        tasks: [
          task("A", "task", null, { title: "共有", store: "shared", status: "todo" }),
          task("B", "task", null, { title: "仕事", store: "work", status: "todo" }),
          task("X", "task", null, { title: "共有の中止", store: "shared", status: "canceled" }),
          task("Y", "task", null, { title: "仕事の中止", store: "work", status: "canceled" }),
        ],
        hidden: ["work"],
      });

    it("leaves out the cards of a hidden Store on the board of every Store, and its canceled ones from the count", async () => {
      two();
      h.ls = { ...h.ls, store: undefined, apiBase: "", cliBase: "", workspace: undefined };
      mountView({ view: { kind: "all" }, query: "-status:canceled assignee:@me" });
      await waitFor(() => expect(screen.getByText("共有")).toBeInTheDocument());
      expect(screen.queryByText("仕事")).toBeNull();
      expect(screen.getByText(/中止 1 件を除外/)).toBeInTheDocument();
    });

    it("keeps the cards of a hidden Store on its own board", async () => {
      two();
      h.ls = { ...h.ls, store: "work" };
      mountView({ view: { kind: "workspace", store: "work", ulid: "W" } });
      await waitFor(() => expect(screen.getByText("仕事")).toBeInTheDocument());
    });
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

  it("clears the warning of a schema that could not be read once it has been read", async () => {
    setup();
    let have = false;
    const listeners = new Set<() => void>();
    (h.core as { schema: unknown }).schema = {
      get: () => (have ? schema : undefined),
      load: async () => {
        throw new Error("down");
      },
      subscribe: (fn: () => void) => {
        listeners.add(fn);
        return () => listeners.delete(fn);
      },
    };
    mountView();
    expect(await screen.findByRole("alert")).toHaveTextContent("schema.yaml を読み込めていません");
    have = true;
    for (const f of listeners) f();
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
    expect(await screen.findByText("調べる")).toBeInTheDocument();
  });

  it("draws no row of details under a card that has nothing to show", async () => {
    setup({ tasks: [task("A", "task", null, { title: "素のカード", status: "todo" })] });
    const { container } = mountView();
    await screen.findByText("素のカード");
    expect(container.querySelector(".meta")).toBeNull();
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

  it("writes the name and the Store of a saved view into the command that saves the settings", async () => {
    setup();
    mountView({ saved: { name: "今週", store: "shared" } });
    expect(
      await screen.findByLabelText(
        "コマンドをコピー: hh view save 今週 --layout kanban --kanban-columns status --swimlane none --store shared",
      ),
    ).toBeEnabled();
  });

  it("shows the command of the screen: hh ls, or the one it is given, and the one that saves the settings", async () => {
    setup();
    mountView({ settings: { columns: "category", swimlane: "assignee" } });
    expect(
      await screen.findByText(/\$ hh ls --store shared "workspace:release -status:canceled"/),
    ).toBeInTheDocument();
    // Not a saved view: the query goes in the command, and it is not to copy until it has a name.
    const line = screen.getByLabelText(
      /^コマンドをコピー: hh view save <名前> --layout kanban --query .*workspace:release.* --kanban-columns category --swimlane assignee$/,
    );
    expect(line).toBeDisabled();
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
    expect(createState.request).toEqual({ kind: "task", status: "review", category: "doing" });
  });

  it("opens it in the category of a column of categories", async () => {
    setup();
    mountView({ settings: { columns: "category", swimlane: "none" } });
    await fireEvent.click(await screen.findByRole("button", { name: "進行中 にタスクを追加" }));
    expect(createState.request).toEqual({ kind: "task", category: "doing" });
  });
});

describe("KanbanView: a read-only Store", () => {
  const SENTENCE = "この Store は閲覧のみです（remote に書き込めません）";
  const two = {
    tasks: [
      task("A", "task", null, { id: "REL-1", title: "調べる", status: "todo" }),
      task("R", "task", null, { id: "RO-1", title: "読むだけ", status: "todo", store: "ro" }),
    ],
    stores: ["shared", "ro"],
    schemas: { shared: schema, ro: schema },
    readOnly: { ro: "the remote cannot be written" },
  };

  it("does not make its cards draggable, and the others are", async () => {
    setup(two);
    mountView();
    await screen.findByText("読むだけ");
    expect(h.cards.get("R@")?.().locked).toBe(true);
    expect(h.cards.get("A@")?.().locked).toBe(false);
  });

  it("turns down a drop of its card, with the reason, and writes nothing", async () => {
    const { api } = setup(two);
    mountView();
    await screen.findByText("読むだけ");
    cellFor("doing")?.onDrop({ ulid: "R", lane: "" });
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
    expect(h.toast.mock.calls[0]?.[0].message).toContain("remote に書き込めません");
    expect(api.PATCH).not.toHaveBeenCalled();
  });

  it("keeps the plus while a Store can be written", async () => {
    setup(two);
    h.ls = { ...h.ls, store: undefined };
    mountView({ view: { kind: "all" } });
    expect(await screen.findByRole("button", { name: "進行中 にタスクを追加" })).toBeEnabled();
  });

  it("disables the plus, with the reason, when no Store can be written", async () => {
    setup({
      ...two,
      readOnly: { shared: "the remote cannot be written", ro: "the remote cannot be written" },
    });
    mountView();
    const plus = await screen.findByRole("button", { name: "進行中 にタスクを追加" });
    expect(plus).toBeDisabled();
    expect(plus).toHaveAttribute("title", SENTENCE);
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
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
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
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
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

  describe("moves that overlap", () => {
    const deferred = <T>() => {
      let resolve!: (v: T) => void;
      let reject!: (e: unknown) => void;
      const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
      });
      return { promise, resolve, reject };
    };

    it("decides a second drop of the card only after the first has been written", async () => {
      const { api, apply } = await ready();
      const first = deferred<ReturnType<typeof ok>>();
      api.PATCH.mockImplementationOnce(() => first.promise as never);
      cellFor("doing")?.onDrop(dragA);
      await waitFor(() => expect(api.PATCH).toHaveBeenCalledTimes(1));
      cellFor("review")?.onDrop(dragA);
      await new Promise((r) => setTimeout(r, 20));
      expect(api.PATCH).toHaveBeenCalledTimes(1);
      first.resolve(ok({ task: { ulid: "A" }, commit: "c1" }));
      await waitFor(() => expect(api.PATCH).toHaveBeenCalledTimes(2));
      expect(apply).toHaveBeenCalledTimes(2);
    });

    const needy = {
      ...schema,
      workflow: { transitions: {}, required: { done: ["watchers"] } },
    } as unknown as SchemaView;

    it("keeps taking drops after one failed outside the write, and says why", async () => {
      const { api } = await ready();
      const cache = (h.core as { cache: { task: (u: string) => TaskView | undefined } }).cache;
      const task = cache.task;
      let boom = true;
      cache.task = (u) => {
        if (boom) {
          boom = false;
          throw new Error("cache broke");
        }
        return task(u);
      };
      cellFor("doing")?.onDrop(dragA);
      await waitFor(() => expect(h.toast).toHaveBeenCalled());
      expect(h.toast.mock.calls[0]?.[0].message).toContain("cache broke");
      cellFor("doing")?.onDrop(dragA);
      await waitFor(() => expect(api.PATCH).toHaveBeenCalledTimes(1));
    });

    it("drops the dialog of a drop another drop has come after", async () => {
      const s = setup({ schemas: { shared: needy } });
      mountView();
      await screen.findByText("調べる");
      const read = deferred<ReturnType<typeof ok>>();
      s.api.GET.mockImplementationOnce(() => read.promise as never);
      cellFor("done")?.onDrop(dragA);
      await waitFor(() => expect(s.api.GET).toHaveBeenCalled());
      cellFor("review")?.onDrop({ ulid: "B", lane: "" });
      read.resolve(ok({ watchers: [] }));
      await waitFor(() => expect(s.api.PATCH).toHaveBeenCalledTimes(1));
      await new Promise((r) => setTimeout(r, 20));
      expect(screen.queryByRole("dialog")).toBeNull();
    });

    it("shows nothing and sends nothing for a drop that finishes after the board is gone", async () => {
      const s = setup({ schemas: { shared: needy } });
      const { unmount } = mountView();
      await screen.findByText("調べる");
      const read = deferred<ReturnType<typeof ok>>();
      s.api.GET.mockImplementationOnce(() => read.promise as never);
      cellFor("done")?.onDrop(dragA);
      await waitFor(() => expect(s.api.GET).toHaveBeenCalled());
      unmount();
      read.resolve(ok({ watchers: [] }));
      await new Promise((r) => setTimeout(r, 20));
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(s.api.PATCH).not.toHaveBeenCalled();
    });

    it("does not toast a failure that comes after the board is gone", async () => {
      const { api, unmount } = await ready();
      const write = deferred<ReturnType<typeof ok>>();
      api.PATCH.mockImplementationOnce(() => write.promise as never);
      cellFor("doing")?.onDrop(dragA);
      await waitFor(() => expect(api.PATCH).toHaveBeenCalled());
      unmount();
      write.reject(new Error("late"));
      await new Promise((r) => setTimeout(r, 20));
      expect(h.toast).not.toHaveBeenCalled();
    });
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
