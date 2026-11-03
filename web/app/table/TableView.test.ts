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

  it("waits for the schema before it draws a property", async () => {
    setup();
    mount({ settings: { ...SETTINGS, columns: ["id", "points"] } });
    await waitFor(() =>
      expect(screen.getByRole("spinbutton", { name: "points: REL-4" })).toHaveValue(3),
    );
  });
});
