import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";
import { tick } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import type { TableSettings } from "../../core/views";
import { closeCreate, createState } from "../create-state.svelte";
import { emitter } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
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
  // The stores whose schema cannot be read (until it is put in the schemas).
  unreadable?: string[];
  // What becomes of the parent the list does not hold, once the table asks for it: it is read
  // (the default), it is read when the test says (hold), or it cannot be read (fail).
  parent?: "read" | "hold" | "fail";
};

function setup(over: Over = {}) {
  const ev = emitter();
  const tasks = new Map<string, TaskView>([
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
  // The parent is not in the list of the query: the cache has it only once the table asks for it.
  const parents = new Map<string, TaskView>([
    [
      "E1",
      task("E1", "epic", null, {
        id: "REL-1",
        title: "API v2 への移行",
        progress: { done: 1, total: 5 },
      }),
    ],
  ]);
  // The tree of the query brings the ancestors of the tasks it finds, once it has answered.
  let treeLoading = (over.parent ?? "read") !== "fail";
  // The tasks the answer of the tree has.
  const answered = new Set<string>();
  const readParent = (_u?: string) => {
    for (const [u, p] of parents) tasks.set(u, p);
    treeLoading = false;
    for (const u of ulids) answered.add(u);
    ev.emit();
  };
  let ulids = over.ulids ?? ["T1", "T2", "T3"];
  const setUlids = (u: string[]) => {
    ulids = u;
  };
  if (over.many) {
    ulids = Array.from({ length: over.many }, (_, i) => {
      tasks.set(`N${i}`, task(`N${i}`, "task", null, { id: `REL-${100 + i}`, title: `many ${i}` }));
      return `N${i}`;
    });
  }
  const watchTree = vi.fn((_q: { q: string; tree: boolean; store?: string }) => {
    const mode = over.parent ?? "read";
    if (mode === "read") queueMicrotask(() => readParent());
    return {
      roots: () => [...answered].map((ulid) => ({ ulid, children: [] })),
      cycle: () => [],
      error: () => (mode === "fail" ? new Error("boom") : null),
      loading: () => treeLoading,
      dispose: vi.fn(),
    };
  });
  const watchList = vi.fn((q: { q: string }) => ({
    ulids: () => (/(^| )status:canceled/.test(q.q) ? (over.canceled ?? []) : ulids),
    error: () => over.error ?? null,
    loading: () => over.loading ?? false,
    dispose() {},
  }));
  const pending = new Set<string>();
  // The words of the server for a task whose edit it turned down.
  const refused = new Map<string, string>();
  const save = {
    edit: vi.fn(),
    flush: vi.fn(async () => {}),
    editing: (u: string) => pending.has(u),
    takeRefused: (u: string) => {
      const why = refused.get(u) ?? null;
      refused.delete(u);
      return why;
    },
    state: () => over.saveState ?? { kind: "idle" },
    subscribe: ev.subscribe,
  };
  const schemas: Record<string, SchemaView> = { shared, me };
  for (const s of over.unreadable ?? []) delete schemas[s];
  h.core = {
    cache: {
      task: (u: string) => tasks.get(u),
      watchList,
      watchTree,
      subscribe: ev.subscribe,
    },
    stores: {
      stores: () => (over.stores ?? ["shared"]).map((name) => ({ name })),
      readOnly: () => null,
      tone: (s: string) => (s === "me" ? "orange" : "purple"),
      subscribe: ev.subscribe,
    },
    schema: {
      get: (s: string) => schemas[s],
      load: async (s: string) => {
        const v = schemas[s];
        if (!v) throw new Error("unreadable");
        return v;
      },
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
    crumbTitle: "release",
    emptyProps: { scope: "workspace", name: "release" },
    notFoundProps: { scope: "workspace", name: "W1" },
    schemas: (stores: string[]) =>
      stores.map((s) => schemas[s]).filter((s): s is SchemaView => s !== undefined),
    ...over.scope,
  };
  // The server answers a write: the task as it is now.
  const write = (ulid: string, patch: Partial<TaskView>) => {
    tasks.set(ulid, { ...(tasks.get(ulid) as TaskView), ...patch });
    ev.emit();
  };
  return {
    save,
    watchList,
    answered,
    watchTree,
    ev,
    schemas,
    tasks,
    pending,
    refused,
    write,
    readParent,
    setUlids,
  };
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
  resetToasts();
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

  it("writes the name and the Store of a saved view into the command that saves the settings", () => {
    setup();
    mount({
      settings: { columns: ["id", "due"], groupBy: "status" },
      saved: { name: "今週", store: "shared" },
    });
    const line = screen.getByLabelText(
      "コマンドをコピー: hh view save 今週 --layout table --columns id,due --group-by status --store shared",
    );
    expect(line).toBeEnabled();
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

    it("only shows the columns that are not edited", async () => {
      setup();
      mount({ settings: { ...SETTINGS, columns: ["id", "parent", "created"] } });
      // The parent of REL-4 and REL-2 is REL-1; REL-9 has none.
      await waitFor(() =>
        expect(screen.getAllByRole("link").map((a) => a.textContent)).toEqual([
          "REL-4",
          "REL-1",
          "REL-2",
          "REL-1",
          "REL-9",
        ]),
      );
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

  describe("an edit the server turns down", () => {
    it("says why in a toast, once, and takes the edit back instead of offering to send it again", async () => {
      const { refused, ev } = setup();
      mount();
      await fireEvent.click(screen.getByRole("button", { name: "ステータス: REL-4" }));
      await fireEvent.click(screen.getByRole("menuitem", { name: /done/ }));
      expect(toasts.list).toEqual([]);
      refused.set("T1", "done にするには points が必要です");
      ev.emit();
      await tick();
      expect(toasts.list.map((t) => t.message)).toEqual([
        "REL-4: done にするには points が必要です",
      ]);
      ev.emit();
      await tick();
      expect(toasts.list).toHaveLength(1);
    });
  });

  describe("parents the query does not list", () => {
    type Handle = { dispose: ReturnType<typeof vi.fn> };

    it("reads the ancestors with one tree of the query of the list, and lets go of it with the screen", async () => {
      const { watchTree } = setup();
      const { unmount } = mount({ settings: { ...SETTINGS, columns: ["id", "parent"] } });
      await waitFor(() => expect(screen.getAllByRole("link", { name: "REL-1" })).toHaveLength(2));
      expect(watchTree.mock.calls).toEqual([
        [{ q: "workspace:release -status:canceled", tree: true, store: "shared" }],
      ]);
      const handle = watchTree.mock.results[0]?.value as Handle;
      expect(handle.dispose).not.toHaveBeenCalled();
      unmount();
      expect(handle.dispose).toHaveBeenCalledTimes(1);
    });

    it("asks for the tree without the sort: and group: terms, which it does not take", async () => {
      const { watchTree } = setup();
      mount({
        query: 'sort:due group:status title:"sort:x" -status:canceled',
        settings: { ...SETTINGS, columns: ["id", "parent"] },
      });
      await waitFor(() => expect(screen.getAllByRole("link", { name: "REL-1" })).toHaveLength(2));
      expect(watchTree.mock.calls[0]?.[0].q).toBe(
        'workspace:release title:"sort:x" -status:canceled',
      );
    });

    it("keeps a parent of a task new in the list on its way until the tree has answered for the task", async () => {
      const { tasks, setUlids, answered, ev } = setup();
      mount({ settings: { ...SETTINGS, columns: ["id", "parent"] } });
      await waitFor(() => expect(screen.getAllByRole("link", { name: "REL-1" })).toHaveLength(2));
      tasks.set("T4", task("T4", "task", "E2", { id: "REL-20", title: "new", status: "todo" }));
      setUlids(["T1", "T2", "T3", "T4"]);
      ev.emit();
      await tick();
      expect(screen.getAllByLabelText("親を読み込み中")).toHaveLength(1);
      expect(screen.queryByLabelText("親を読み込めません")).toBeNull();
      // The tree answers, and the parent is not to be had.
      answered.add("T4");
      ev.emit();
      await tick();
      expect(screen.queryByLabelText("親を読み込み中")).toBeNull();
      expect(screen.getAllByLabelText("親を読み込めません")).toHaveLength(1);
    });

    it("reads the tree only while the parent column, the kind column or the group of parents is shown", async () => {
      const { watchTree } = setup();
      const { rerender } = mount();
      expect(watchTree).not.toHaveBeenCalled();
      await rerender({ settings: { ...SETTINGS, columns: ["id", "kind"] } });
      expect(watchTree).toHaveBeenCalledTimes(1);
      const first = watchTree.mock.results[0]?.value as Handle;
      await rerender({ settings: SETTINGS });
      expect(first.dispose).toHaveBeenCalledTimes(1);
      await rerender({ settings: { ...SETTINGS, groupBy: "parent" } });
      expect(watchTree).toHaveBeenCalledTimes(2);
      const second = watchTree.mock.results[1]?.value as Handle;
      expect(second.dispose).not.toHaveBeenCalled();
      await rerender({ settings: SETTINGS });
      expect(second.dispose).toHaveBeenCalledTimes(1);
    });

    it("shows a parent on its way apart from no parent, and sorts by it once it is read", async () => {
      const { readParent } = setup({ parent: "hold", ulids: ["T3", "T1", "T2"] });
      mount({ settings: { ...SETTINGS, columns: ["id", "parent"] } });
      // Waiting for the parent of REL-4 and REL-2; REL-9 has none.
      expect(screen.getAllByLabelText("親を読み込み中")).toHaveLength(2);
      expect(screen.getAllByText("—")).toHaveLength(1);
      await fireEvent.click(
        within(screen.getByRole("columnheader", { name: /^親/ })).getByRole("button"),
      );
      readParent("E1");
      await waitFor(() => expect(screen.queryByLabelText("親を読み込み中")).toBeNull());
      expect(order().filter((id) => id !== "REL-1")).toEqual(["REL-4", "REL-2", "REL-9"]);
    });

    it("says a parent that cannot be read is unknown, not that there is none", () => {
      setup({ parent: "fail" });
      mount({ settings: { ...SETTINGS, columns: ["id", "parent"] } });
      expect(screen.getAllByLabelText("親を読み込めません")).toHaveLength(2);
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
      expect(
        within(menu).getByRole("menuitemcheckbox", { name: "ID", checked: true }),
      ).toBeInTheDocument();
      expect(
        within(menu).getByRole("menuitemcheckbox", { name: "親", checked: false }),
      ).toBeInTheDocument();
      // The properties of the schema come after the builtin columns, by name.
      expect(
        within(menu)
          .getAllByRole("menuitemcheckbox")
          .map((i) => i.textContent?.trim())
          .slice(-2),
      ).toEqual(["期限", "points"]);
      await fireEvent.click(within(menu).getByRole("menuitemcheckbox", { name: /^親/ }));
      expect(onsettings).toHaveBeenCalledWith({
        ...SETTINGS,
        columns: [...SETTINGS.columns, "parent"],
      });
      expect(screen.getByRole("menu", { name: "列" })).toBeInTheDocument();
      await fireEvent.click(within(menu).getByRole("menuitemcheckbox", { name: /^期限/ }));
      expect(onsettings).toHaveBeenLastCalledWith({
        ...SETTINGS,
        columns: ["id", "title", "status", "assignee"],
      });
    });

    it("does not take out the last column", async () => {
      setup();
      mount({ settings: { ...SETTINGS, columns: ["id"] } });
      await fireEvent.click(screen.getByRole("button", { name: "列 1" }));
      expect(screen.getByRole("menuitemcheckbox", { name: /^ID/ })).toBeDisabled();
    });

    it("keeps the names the table does not show when a column is added", async () => {
      setup();
      const { onsettings } = mount({ settings: { ...SETTINGS, columns: ["id", "ghost"] } });
      await fireEvent.click(screen.getByRole("button", { name: "列 1" }));
      await fireEvent.click(screen.getByRole("menuitemcheckbox", { name: /^タイトル/ }));
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
    const heads = () =>
      [...document.querySelectorAll(".group")].map((e) =>
        e.textContent?.replace(/\s+/g, " ").trim(),
      );

    it("draws a heading before the rows of each parent, with the progress of the parent", async () => {
      setup();
      mount({ settings: { ...SETTINGS, groupBy: "parent" } });
      await waitFor(() =>
        expect(heads()).toEqual(["API v2 への移行 REL-1 1/5 完了 2 件", "親なし 1 件"]),
      );
      expect(order()).toEqual(["REL-4", "REL-2", "REL-9"]);
    });

    it("does not call a parent that is not read yet unknown or missing", async () => {
      const { readParent } = setup({ parent: "hold" });
      mount({ settings: { ...SETTINGS, groupBy: "parent" } });
      expect(heads()).toEqual(["読み込み中… 2 件", "親なし 1 件"]);
      readParent("E1");
      await waitFor(() =>
        expect(heads()).toEqual(["API v2 への移行 REL-1 1/5 完了 2 件", "親なし 1 件"]),
      );
    });

    it("says a parent that cannot be read is unknown", () => {
      setup({ parent: "fail" });
      mount({ settings: { ...SETTINGS, groupBy: "parent" } });
      expect(heads()).toEqual(["不明な親 2 件", "親なし 1 件"]);
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

    it("has no clear action when the filter is already empty", () => {
      setup({ ulids: [], scope: { apiBase: "", cliBase: "", store: undefined } });
      mount({ view: { kind: "all" }, query: "" });
      expect(screen.getByText("一致するタスクはありません")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "フィルタを解除" })).toBeNull();
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

  describe("a row being edited", () => {
    const BY_TITLE: TableSettings = { columns: ["id", "title", "status"], groupBy: "none" };
    const sortByTitle = () =>
      fireEvent.click(
        within(screen.getByRole("columnheader", { name: /^タイトル/ })).getByRole("button"),
      );

    it("keeps its place and its focus while the focus is in it, and moves when the focus leaves", async () => {
      const { write } = setup();
      mount({ settings: BY_TITLE });
      await sortByTitle();
      const before = order();
      expect(before).toHaveLength(3);
      const last = before[2] as string;
      const input = screen.getByRole("textbox", { name: `タイトル: ${last}` });
      input.focus();
      await fireEvent.input(input, { target: { value: "AAA" } });
      write(last === "REL-9" ? "T3" : last === "REL-4" ? "T1" : "T2", { title: "AAA" });
      await tick();
      expect(order()).toEqual(before);
      expect(input).toHaveFocus();
      input.blur();
      await tick();
      expect(order()[0]).toBe(last);
    });

    it("keeps its place while its save is not answered, and does not remount the cell", async () => {
      const { write, pending, ev } = setup();
      mount({ settings: BY_TITLE });
      await sortByTitle();
      const before = order();
      const last = before[2] as string;
      const ulid = last === "REL-9" ? "T3" : last === "REL-4" ? "T1" : "T2";
      const input = screen.getByRole("textbox", { name: `タイトル: ${last}` });
      input.focus();
      await fireEvent.input(input, { target: { value: "AAA" } });
      pending.add(ulid);
      input.blur();
      write(ulid, { title: "AAA" });
      await tick();
      expect(order()).toEqual(before);
      expect(screen.getByRole("textbox", { name: `タイトル: ${last}` })).toBe(input);
      expect(input).toHaveValue("AAA");
      pending.delete(ulid);
      ev.emit();
      await tick();
      expect(order()[0]).toBe(last);
    });

    it("shows the value of the server again once a refused edit is taken back and the focus leaves", async () => {
      const { pending, refused, ev } = setup();
      mount({ settings: BY_TITLE });
      const input = screen.getByRole("textbox", { name: "タイトル: REL-4" });
      input.focus();
      await fireEvent.input(input, { target: { value: "B" } });
      pending.add("T1");
      ev.emit();
      await tick();
      pending.delete("T1");
      refused.set("T1", "no");
      ev.emit();
      await tick();
      expect(input).toHaveValue("B");
      input.blur();
      await tick();
      expect(input).toHaveValue("移行ガイドを書く");
    });

    it("keeps a row that no longer matches the query while the user is in it, and drops it after", async () => {
      const { setUlids, ev } = setup();
      mount({ settings: BY_TITLE });
      const input = screen.getByRole("textbox", { name: "タイトル: REL-2" });
      input.focus();
      setUlids(["T1", "T3"]);
      ev.emit();
      await tick();
      expect(order()).toEqual(["REL-4", "REL-2", "REL-9"]);
      expect(screen.getByRole("textbox", { name: "タイトル: REL-2" })).toBe(input);
      expect(input).toHaveFocus();
      input.blur();
      await tick();
      expect(order()).toEqual(["REL-4", "REL-9"]);
    });

    it("stays in its group when its status changes, with the focus on the cell", async () => {
      const { write } = setup();
      mount({ settings: { ...BY_TITLE, groupBy: "status" } });
      const heads = () =>
        [...document.querySelectorAll(".group")].map((e) =>
          e.textContent?.replace(/\s+/g, " ").trim(),
        );
      expect(heads()).toEqual(["backlog 1 件", "todo 1 件", "doing 1 件"]);
      const pick = screen.getByRole("button", { name: "ステータス: REL-4" });
      await fireEvent.click(pick);
      await fireEvent.click(screen.getByRole("menuitem", { name: /doing/ }));
      write("T1", { status: "doing" });
      await tick();
      expect(heads()).toEqual(["backlog 1 件", "todo 1 件", "doing 1 件"]);
      expect(screen.getByRole("button", { name: "ステータス: REL-4" })).toBe(pick);
      expect(pick).toHaveFocus();
      pick.blur();
      await tick();
      expect(heads()).toEqual(["backlog 1 件", "doing 2 件"]);
    });
  });

  it("numbers the rows for a screen reader, the header being the first", () => {
    setup();
    mount();
    expect(screen.getByRole("table")).toHaveAttribute("aria-rowcount", "4");
    const rows = [...document.querySelectorAll("tbody tr")];
    expect(rows.map((r) => r.getAttribute("aria-rowindex"))).toEqual(["2", "3", "4"]);
  });

  it("numbers the rows of the window by their place in the whole list", () => {
    setup({ many: 250 });
    h.vstart = 30;
    mount();
    expect(screen.getByRole("table")).toHaveAttribute("aria-rowcount", "251");
    const rows = [...document.querySelectorAll("tbody tr")];
    expect(rows[0]).toHaveAttribute("aria-rowindex", "32");
  });

  it("tells the store whose schema cannot be read, and stops once it has been read", async () => {
    const { ev, schemas } = setup({ unreadable: ["shared"] });
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "shared の schema.yaml を読み込めていません",
    );
    schemas.shared = shared;
    ev.emit();
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  });

  it("forgets the sort of a column that is taken out, and does not bring it back with the column", async () => {
    setup();
    const { rerender } = mount();
    await fireEvent.click(
      within(screen.getByRole("columnheader", { name: /^ID/ })).getByRole("button"),
    );
    expect(screen.getByRole("columnheader", { name: /^ID/ })).toHaveAttribute(
      "aria-sort",
      "ascending",
    );
    await rerender({ settings: { ...SETTINGS, columns: ["title", "status"] } });
    await rerender({ settings: SETTINGS });
    expect(screen.getByRole("columnheader", { name: /^ID/ })).toHaveAttribute("aria-sort", "none");
    expect(order()).toEqual(["REL-4", "REL-2", "REL-9"]);
  });

  it("waits for the schema before it draws a property", async () => {
    setup();
    mount({ settings: { ...SETTINGS, columns: ["id", "points"] } });
    await waitFor(() =>
      expect(screen.getByRole("spinbutton", { name: "points: REL-4" })).toHaveValue(3),
    );
  });
});
