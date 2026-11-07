import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { closeCreate, createState } from "../create-state.svelte";
import { closePalette, openPalette, palette } from "../palette-state.svelte";
import { selection } from "../selection.svelte";
import { schema, task } from "../tree/fixtures";

const h = vi.hoisted(() => ({
  core: undefined as unknown,
  route: { name: "home" } as { name: string; ulid?: string; store?: string; page?: string },
  go: vi.fn(),
  toast: vi.fn(),
  runUndo: vi.fn(async () => {}),
  addWorkspace: vi.fn(),
  moveWorkspace: vi.fn(),
  moveStore: vi.fn(),
  saveView: vi.fn(),
  qualify: false, // the Stores share a prefix: IDs are shown as <store>:<ID>
}));

vi.mock("../context", () => ({ core: () => h.core }));
vi.mock("../router", () => ({
  router: {
    get route() {
      return h.route;
    },
    go: h.go,
    href: (r: {
      name: string;
      ulid?: string;
      store?: string;
      tab?: string;
      view?: string;
      layout?: string;
      q?: string;
      page?: string;
    }) =>
      r.name === "task"
        ? `/tasks/${r.ulid}`
        : r.name === "project"
          ? `/projects/${r.ulid}${r.tab && r.tab !== "tree" ? `/${r.tab}` : ""}`
          : r.name === "settings"
            ? r.page === "general" || r.page === "accounts" || r.page === "stores"
              ? `/settings/${r.page}`
              : `/settings/workspaces/${r.store}`
            : r.name === "view"
              ? `/views/${r.store}/${r.view}`
              : r.name === "query"
                ? `/query/${r.layout}?q=${r.q}`
                : r.name === "my"
                  ? "/my"
                  : r.name === "inbox"
                    ? "/inbox"
                    : r.name === "activity"
                      ? "/activity"
                      : `/workspaces/${r.store}/${r.ulid}${r.tab === "overview" ? "" : `/${r.tab}`}`,
  },
}));
vi.mock("../views/dialogs-state.svelte", () => ({ openViewSave: h.saveView }));
vi.mock("../toast.svelte", () => ({ toast: h.toast }));
vi.mock("../undo", async (orig) => ({
  ...(await orig<typeof import("../undo")>()),
  runUndo: h.runUndo,
}));
vi.mock("../workspace/dialogs-state.svelte", () => ({
  openWorkspaceAdd: h.addWorkspace,
  openWorkspaceMove: h.moveWorkspace,
  openStoreMove: h.moveStore,
}));

import { toApiError } from "../../core/api/client";
import CommandPalette from "./CommandPalette.svelte";

const ok = (data: unknown) => Promise.resolve(data);
const rel6 = task("U6", "task", null, {
  id: "REL-6",
  title: "Web の API クライアントを v2 に更新",
});

function setup(
  found: unknown[] = [],
  readOnly: string | null = null,
  hidden: string[] = [], // the Stores the GUI leaves out
) {
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
    getTask: vi.fn((id: string, _query?: unknown) => {
      const hit = ["sh-17", "shared:rel-6"].includes(id.toLowerCase());
      return hit
        ? ok({ ...rel6, former_ids: ["SH-17"] })
        : Promise.reject(toApiError(404, { type: "task.not_found", title: "NF", status: 404 }));
    }),
    listTasks: vi.fn((params?: { store?: string; q?: string }) =>
      params?.q?.startsWith("foo:")
        ? Promise.reject(
            toApiError(400, {
              type: "query.invalid",
              title: "Bad",
              status: 400,
              detail: "term 1 (foo:bar): unknown key",
            }),
          )
        : ok({ tasks: found }),
    ),
    sync: vi.fn((_query?: unknown) => ok([])),
    updateTask: vi.fn((_id?: string, _body?: unknown, _query?: unknown) =>
      ok({ task: { ...rel6, status: "done" }, commit: "c1" }),
    ),
  };
  h.core = {
    api,
    cache: {
      task: (u: string) =>
        u === "U6"
          ? rel6
          : u === "U9"
            ? { ...rel6, ulid: "U9", store: "backend" }
            : u === "P1"
              ? task("P1", "project", null, { id: "REL-15", title: "リリース準備 v2.4" })
              : undefined,
      applyWrite: vi.fn(),
      epochNow: () => 0,
      prime: vi.fn(),
      subscribe: none,
    },
    stores: {
      stores: () => [{ name: "shared" }, { name: "backend" }],
      workspaces: () => [ws, { ...ws, ulid: "W2", name: "ops", prefix: "OPS" }],
      visibleWorkspaces: () =>
        [ws, { ...ws, ulid: "W2", name: "ops", prefix: "OPS" }].filter(
          (w) => !hidden.includes(w.store),
        ),
      hidden: (s: string) => hidden.includes(s),
      readOnly: () => readOnly,
      tone: () => "purple",
      displayId: (s: string, id: string) => (h.qualify ? `${s}:${id}` : id),
      subscribe: none,
    },
    schema: { get: () => schema, load: async () => schema, subscribe: none },
    views: {
      list: () => [{ store: "backend", tone: "orange", view: { name: "ブロック中" } }],
      subscribe: none,
    },
  };
  return { api };
}

const box = () => screen.getByRole("combobox");

beforeEach(() => {
  h.go.mockReset();
  h.toast.mockReset();
  h.runUndo.mockClear();
  h.addWorkspace.mockClear();
  h.moveWorkspace.mockClear();
  h.moveStore.mockClear();
  h.saveView.mockClear();
  h.qualify = false;
  localStorage.clear();
  closePalette();
  closeCreate();
  h.route = { name: "home" };
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

  it("writes the ID of a task with its Store where the prefix is shared", async () => {
    h.qualify = true;
    setup();
    localStorage.setItem("hedgehug.recent", JSON.stringify(["U6"]));
    render(CommandPalette);
    openPalette();
    expect(await screen.findByText("shared:REL-6")).toBeInTheDocument();
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
    await waitFor(() => expect(api.sync).toHaveBeenCalledWith());
    expect(palette.open).toBe(false);
  });

  it("adds a workspace from the command", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), {
      target: { value: ">ワークスペースを追加" },
    });
    await screen.findByRole("option", { name: /ワークスペースを追加/ });
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(h.addWorkspace).toHaveBeenCalledTimes(1);
    expect(palette.open).toBe(false);
  });

  it("takes back the last operation without asking, in the store of the workspace on screen", async () => {
    const { api } = setup();
    h.route = { name: "workspace", store: "backend", ulid: "W9" };
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">取り消" } });
    await screen.findByRole("option", { name: /直前の操作を取り消す/ });
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(h.runUndo).toHaveBeenCalledTimes(1);
    expect(h.runUndo).toHaveBeenCalledWith(expect.objectContaining({ api }), "backend");
    expect(palette.open).toBe(false);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("takes back in the store of the task on screen and of the settings, and asks for no store at home", async () => {
    setup();
    h.route = { name: "task", ulid: "U9" };
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">取り消" } });
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(h.runUndo).toHaveBeenLastCalledWith(expect.anything(), "backend");

    closePalette();
    h.route = { name: "settings", page: "workspaces", store: "backend" };
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">取り消" } });
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(h.runUndo).toHaveBeenLastCalledWith(expect.anything(), "backend");

    closePalette();
    h.route = { name: "home" };
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">取り消" } });
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(h.runUndo).toHaveBeenLastCalledWith(expect.anything(), undefined);
  });

  it("opens the sync and maintenance settings", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), {
      target: { value: ">同期と保守" },
    });
    await fireEvent.click(await screen.findByRole("option", { name: /設定の同期と保守を開く/ }));
    expect(h.go).toHaveBeenCalledWith("/settings/general");
    expect(palette.open).toBe(false);
  });

  it.each([
    ["アカウント", /設定のアカウントと認証を開く/, "/settings/accounts"],
    ["マウント中", /設定のマウント中の Store を開く/, "/settings/stores"],
  ] as const)("opens the settings of %s", async (word, name, path) => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: `>${word}` } });
    await fireEvent.click(await screen.findByRole("option", { name }));
    expect(h.go).toHaveBeenCalledWith(path);
    expect(palette.open).toBe(false);
  });

  it("runs the diagnosis in the sync and maintenance settings", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">doctor" } });
    await fireEvent.click(await screen.findByRole("option", { name: /doctor を実行/ }));
    expect(h.go).toHaveBeenCalledWith("/settings/general");
    expect(palette.open).toBe(false);
  });

  describe("hidden Stores", () => {
    it("leaves their tasks out of the tasks opened last", async () => {
      setup([], null, ["backend"]);
      localStorage.setItem("hedgehug.recent", JSON.stringify(["U6", "U9"]));
      render(CommandPalette);
      openPalette();
      await screen.findByText("最近開いたもの");
      // U9 is of backend and has the same title as U6
      expect(screen.getAllByText("Web の API クライアントを v2 に更新")).toHaveLength(1);
    });

    it("asks for what a search finds without them", async () => {
      const { api } = setup([], null, ["backend"]);
      render(CommandPalette);
      openPalette();
      await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "backend" } });
      await waitFor(() =>
        expect(api.listTasks).toHaveBeenCalledWith({
          q: '-store:backend "backend"',
          store: undefined,
        }),
      );
    });

    it("still opens a task of a hidden Store by its ID, like a URL does", async () => {
      setup([], null, ["shared"]);
      render(CommandPalette);
      openPalette();
      await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "SH-17" } });
      expect(await screen.findByText("Web の API クライアントを v2 に更新")).toBeInTheDocument();
    });

    it("opens a task of a hidden Store by <store>:<ID>, asking the server with it as typed", async () => {
      const { api } = setup([], null, ["shared"]);
      render(CommandPalette);
      openPalette();
      await fireEvent.input(await screen.findByRole("combobox"), {
        target: { value: "shared:REL-6" },
      });
      expect(await screen.findByText("Web の API クライアントを v2 に更新")).toBeInTheDocument();
      expect(api.getTask).toHaveBeenCalledWith("shared:REL-6");
      expect(screen.getByRole("combobox")).toBeInTheDocument();
      // the input is an ID, not a query
      expect(screen.getByText("ID", { selector: "*" })).toBeInTheDocument();
    });

    it("finds the same task when its Store is not hidden", async () => {
      setup([{ ...rel6, ulid: "U9", id: "BE-1", title: "backend のタスク", store: "backend" }]);
      render(CommandPalette);
      openPalette();
      await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "backend" } });
      expect(await screen.findByText("backend のタスク")).toBeInTheDocument();
    });

    it("leaves out their saved views, their workspaces and their settings", async () => {
      setup([], null, ["backend", "shared"]);
      render(CommandPalette);
      openPalette();
      await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
      await screen.findByText("今すぐ同期する");
      expect(screen.queryByRole("option", { name: /ブロック中 を開く/ })).toBeNull();
      expect(screen.queryByRole("option", { name: /release/ })).toBeNull();
      expect(screen.queryByRole("option", { name: /設定のスキーマを開く/ })).toBeNull();
    });

    it("still lists them when they are not hidden", async () => {
      setup();
      render(CommandPalette);
      openPalette();
      await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
      await screen.findByText("今すぐ同期する");
      expect(screen.getByRole("option", { name: /ブロック中 を開く/ })).toBeInTheDocument();
    });
  });

  it("opens the settings of the workspace", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">設定" } });
    await fireEvent.click(
      await screen.findByRole("option", { name: /ワークスペースの設定を開く.*release/ }),
    );
    expect(h.go).toHaveBeenCalledWith("/workspaces/shared/W1/settings");
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
    await waitFor(() => expect(api.updateTask).toHaveBeenCalled());
    expect(api.updateTask).toHaveBeenCalledWith("U6", { status: "done" }, { store: "shared" });
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

  it("opens the tasks of the person and a saved view from the commands", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">開く" } });
    await fireEvent.click(await screen.findByRole("option", { name: /マイタスクを開く/ }));
    expect(h.go).toHaveBeenCalledWith("/my");
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">通知" } });
    await fireEvent.click(await screen.findByRole("option", { name: /通知を開く/ }));
    expect(h.go).toHaveBeenLastCalledWith("/inbox");
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">アクティ" } });
    await fireEvent.click(await screen.findByRole("option", { name: /アクティビティを開く/ }));
    expect(h.go).toHaveBeenLastCalledWith("/activity");
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">ブロック" } });
    await fireEvent.click(await screen.findByRole("option", { name: /ブロック中 を開く/ }));
    expect(h.go).toHaveBeenLastCalledWith("/views/backend/ブロック中");
  });

  it.each([
    ["the page of a project", { name: "project", ulid: "P1" }, true],
    ["the detail of a project", { name: "task", ulid: "P1" }, true],
    ["the detail of a task that is none", { name: "task", ulid: "U6" }, false],
    ["the home", { name: "home" }, false],
  ])("opens the settings of the project from %s: %s", async (_, route, offered) => {
    setup();
    h.route = route;
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), {
      target: { value: ">プロジェクトの設定" },
    });
    if (!offered) {
      await waitFor(() =>
        expect(screen.queryByRole("option", { name: /プロジェクトの設定/ })).toBeNull(),
      );
      return;
    }
    await fireEvent.click(await screen.findByRole("option", { name: /プロジェクトの設定を開く/ }));
    expect(h.go).toHaveBeenLastCalledWith("/projects/P1/settings");
  });

  it("offers to open a query that found tasks as a table, and to save it as a view", async () => {
    setup([rel6]);
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), {
      target: { value: "status:doing assignee:@me" },
    });
    expect(await screen.findByRole("group", { name: "このクエリで" })).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("option", { name: /ビューとして保存/ }));
    expect(h.saveView).toHaveBeenCalledWith({
      draft: {
        layout: "table",
        query: "status:doing assignee:@me",
        kanban: { columns: "status", swimlane: "none" },
        table: { columns: ["id", "title", "status", "assignee", "due"], groupBy: "none" },
        gantt: { start: "start", end: "due", roadmap: false },
      },
    });
    expect(palette.open).toBe(false);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), {
      target: { value: "status:doing assignee:@me" },
    });
    await fireEvent.click(await screen.findByRole("option", { name: /テーブルで開く/ }));
    expect(h.go).toHaveBeenCalledWith("/query/table?q=status:doing assignee:@me");
  });

  it("offers nothing for a query typed after the one that was searched, until it is searched", async () => {
    setup([rel6]);
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), {
      target: { value: "status:doing" },
    });
    expect(await screen.findByRole("group", { name: "このクエリで" })).toBeInTheDocument();
    // the new text has not been searched yet: it may be a wrong query
    await fireEvent.input(box(), { target: { value: "status:doing kind:bug" } });
    expect(screen.queryByRole("group", { name: "このクエリで" })).toBeNull();
    expect(await screen.findByRole("group", { name: "このクエリで" })).toBeInTheDocument();
  });

  it("offers nothing for a query that found nothing or does not run", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), {
      target: { value: "status:doing" },
    });
    expect(await screen.findByText("一致するものがありません")).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "このクエリで" })).toBeNull();
    await fireEvent.input(box(), { target: { value: "foo:bar" } });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "このクエリで" })).toBeNull();
  });

  it("offers the commands for the selected task", async () => {
    setup();
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    expect(await screen.findByText("REL-6 に対して")).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("option", { name: /子タスクを作成/ }));
    expect(createState.request).toEqual({ kind: "task", parent: "U6" });
  });

  it("offers nothing that writes when every Store is read-only: only the detail of the task", async () => {
    setup([], "the remote cannot be written");
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    expect(await screen.findByText("REL-6 に対して")).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /詳細を開く/ })).toBeInTheDocument();
    for (const name of [
      /ステータスを変更/,
      /子タスクを作成/,
      /別のワークスペースへ移動/,
      /別の Store へ移動/,
      /タスクを作成/,
      /プロジェクトを作成/,
      /ワークスペースを追加/,
    ]) {
      expect(screen.queryByRole("option", { name })).toBeNull();
    }
    expect(screen.getByRole("option", { name: /今すぐ同期する/ })).toBeInTheDocument();
  });

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

  it("opens the move-to-another-Store dialog for the selected task from the command", async () => {
    setup();
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">Store" } });
    await fireEvent.click(await screen.findByRole("option", { name: /別の Store へ移動/ }));
    expect(h.moveStore).toHaveBeenCalledWith(expect.objectContaining({ ulid: "U6" }));
    expect(palette.open).toBe(false);
  });

  it("does nothing on Enter or the arrows while the IME is composing", async () => {
    const { api } = setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">同期" } });
    await screen.findByText("今すぐ同期する");
    await fireEvent.keyDown(box(), { key: "Enter", isComposing: true });
    await fireEvent.keyDown(box(), { key: "Enter", keyCode: 229 });
    await fireEvent.keyDown(box(), { key: "Escape", isComposing: true });
    expect(api.sync).not.toHaveBeenCalled();
    expect(palette.open).toBe(true);
  });

  it("takes the task of the detail route before the selected row", async () => {
    setup();
    selection.set("OTHER");
    h.route = { name: "task", ulid: "U6" };
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    expect(await screen.findByText("REL-6 に対して")).toBeInTheDocument();
  });

  it("writes the key of the new tab for the platform", async () => {
    setup();
    const platform = vi.spyOn(navigator, "platform", "get").mockReturnValue("MacIntel");
    render(CommandPalette);
    openPalette();
    expect(await screen.findByText("⌘↵", { selector: "kbd" })).toBeInTheDocument();
    platform.mockReturnValue("Linux x86_64");
    closePalette();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    openPalette();
    expect(await screen.findByText("Ctrl+↵", { selector: "kbd" })).toBeInTheDocument();
    platform.mockRestore();
  });

  it("says which stores failed to sync, and does not start a second sync while one runs", async () => {
    const { api } = setup();
    let release: (v: unknown) => void = () => {};
    api.sync.mockImplementation(
      () =>
        new Promise((r) => {
          release = r;
        }) as never,
    );
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">同期" } });
    await fireEvent.keyDown(await screen.findByRole("combobox"), { key: "Enter" });
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">同期" } });
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(api.sync).toHaveBeenCalledTimes(1);
    release(
      ok([
        { store: "shared", ok: true },
        { store: "backend", ok: false, error: "boom" },
      ]),
    );
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
    expect(h.toast.mock.calls[0]?.[0].message).toContain("backend");
  });

  it("goes back to the first item when Esc leaves the status list", async () => {
    setup();
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    await fireEvent.click(await screen.findByRole("option", { name: /ステータスを変更/ }));
    await screen.findByText("ステータス", { selector: "[data-mode]" });
    await fireEvent.keyDown(box(), { key: "ArrowDown" });
    await fireEvent.keyDown(box(), { key: "Escape" });
    expect(await screen.findByText("ID", { selector: "[data-mode]" })).toBeInTheDocument();
    expect(screen.getAllByRole("option")[0]).toHaveAttribute("aria-selected", "true");
  });

  it("matches a status name in any case", async () => {
    setup();
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    await fireEvent.click(await screen.findByRole("option", { name: /ステータスを変更/ }));
    await fireEvent.input(box(), { target: { value: "DOI" } });
    expect(await screen.findByRole("option", { name: /doing/ })).toBeInTheDocument();
  });

  it("lists the statuses of a task whose store schema is not read yet, once it is", async () => {
    setup();
    const core = h.core as { schema: { get: unknown; load: unknown; subscribe: unknown } };
    const subs = new Set<() => void>();
    let loaded = false;
    core.schema.get = () => (loaded ? schema : undefined);
    core.schema.subscribe = (fn: () => void) => {
      subs.add(fn);
      return () => subs.delete(fn);
    };
    let arrive!: () => void;
    const gate = new Promise<void>((r) => {
      arrive = r;
    });
    const load = vi.fn(async () => {
      await gate;
      loaded = true;
      for (const fn of subs) fn();
      return schema;
    });
    core.schema.load = load;
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    await fireEvent.click(await screen.findByRole("option", { name: /ステータスを変更/ }));
    expect(load).toHaveBeenCalledWith("shared");
    expect(await screen.findByText("読み込み中…")).toBeInTheDocument();
    arrive();
    expect(await screen.findByRole("option", { name: /doing/ })).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText("読み込み中…")).toBeNull());
  });

  it("says so when the schema of the store cannot be read", async () => {
    setup();
    const core = h.core as { schema: { get: unknown; load: unknown } };
    core.schema.get = () => undefined;
    core.schema.load = async () => {
      throw new Error("down");
    };
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    await fireEvent.click(await screen.findByRole("option", { name: /ステータスを変更/ }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("keeps Tab inside the palette", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    const input = await screen.findByRole("combobox");
    const e = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true });
    input.dispatchEvent(e);
    expect(e.defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(input);
  });

  it("fetches the recent tasks the cache lacks and drops one that is gone", async () => {
    const { api } = setup();
    const u7 = task("U7", "task", null, { id: "REL-7", title: "あとから届く" });
    api.getTask.mockImplementation((id: string) =>
      id === "U7"
        ? ok(u7)
        : Promise.reject(toApiError(404, { type: "task.not_found", title: "NF", status: 404 })),
    );
    const core = h.core as { cache: { task: (u: string) => unknown; prime: unknown } };
    const known = core.cache.task;
    const store = new Map<string, unknown>();
    core.cache.task = (u: string) => store.get(u) ?? known(u);
    core.cache.prime = vi.fn((tasks: { ulid: string }[]) => {
      for (const t of tasks) store.set(t.ulid, t);
    });
    localStorage.setItem("hedgehug.recent", JSON.stringify(["U6", "U7", "U8"]));
    render(CommandPalette);
    openPalette();
    expect(await screen.findByText("あとから届く")).toBeInTheDocument();
    expect(api.getTask).toHaveBeenCalledWith("U7");
    expect(api.getTask).toHaveBeenCalledWith("U8");
    await waitFor(() =>
      expect(JSON.parse(localStorage.getItem("hedgehug.recent") ?? "[]")).toEqual(["U6", "U7"]),
    );
  });
});
