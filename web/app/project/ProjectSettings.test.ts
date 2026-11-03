import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import { FakeConn } from "../../core/testing";
import { fail, ok, schemaView } from "../detail/testkit";
import { emitter, renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import { task } from "../tree/fixtures";
import { entry } from "../workspace/testkit";
import ProjectSettings from "./ProjectSettings.svelte";

const project = task("P1", "project", null, {
  id: "REL-15",
  title: "リリース準備 v2.4",
  workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
}) as TaskView;

// The Store's own workflow: done needs points, and nothing leaves canceled.
const sch = {
  ...schemaView,
  workflow: {
    transitions: { canceled: ["todo"] },
    required: { done: ["points"] },
  },
  scoped: [],
};

type Scoped = {
  where: string;
  statuses?: string[];
  workflow?: { transitions: Record<string, string[]>; required: Record<string, string[]> };
};

const bad = (id: string, message: string) =>
  ({
    ...task(`U-${id}`, "task", "P1", { id, title: `title of ${id}` }),
    problems: [{ code: "required", field: "points", message }],
  }) as TaskView;

type Opts = { scoped?: Scoped; violations?: TaskView[]; stores?: string[]; notFound?: boolean };

function setup(opts: Opts = {}) {
  const state: { scoped?: Scoped; violations: TaskView[] } = {
    scoped: opts.scoped,
    violations: opts.violations ?? [],
  };
  const conn = new FakeConn();
  const view = () => ({
    commit: null,
    project: { ulid: "P1", id: "REL-15", title: "リリース準備 v2.4" },
    scoped: state.scoped,
  });
  // The schema cache: what the Store holds, and the moment it reads it again.
  const cache = emitter();
  // reading the schema again after a write of ours: the cache tells what the Store holds now
  const reload = vi.fn(async () => cache.emit());
  const api = {
    GET: vi.fn(() => ok({ tasks: state.violations })),
    PUT: vi.fn((_path: string, init: { body: Record<string, unknown> }) => {
      const b = init.body as {
        statuses?: string[];
        transitions?: Record<string, string[]>;
        required?: Record<string, string[]>;
      };
      const old = state.scoped;
      const transitions = b.transitions ?? old?.workflow?.transitions ?? {};
      const required = b.required ?? old?.workflow?.required ?? {};
      const statuses = b.statuses ?? old?.statuses;
      const flow =
        Object.keys(transitions).length + Object.keys(required).length > 0
          ? { transitions, required }
          : undefined;
      state.scoped =
        (statuses?.length ?? 0) > 0 || flow
          ? {
              where: "project:P1",
              statuses: statuses?.length ? statuses : undefined,
              workflow: flow,
            }
          : undefined;
      return ok(view());
    }),
    DELETE: vi.fn(() => {
      state.scoped = undefined;
      return ok(view());
    }),
  };
  const none = () => () => {};
  const names = opts.stores ?? ["shared"];
  const r = renderWithCore(
    ProjectSettings,
    { ulid: "P1" },
    {
      api,
      conn,
      cache: {
        task: (u: string) => (u === "P1" && !opts.notFound ? project : undefined),
        watchDetail: () => ({
          ulid: () => null,
          error: () => (opts.notFound ? { status: 404 } : null),
          dispose() {},
        }),
        subscribe: none,
      },
      stores: {
        stores: () => names.map((name) => ({ name })),
        workspaces: () => [entry()],
        workspace: () => entry(),
        loaded: () => true,
        tone: () => "purple",
        subscribe: none,
      },
      schema: {
        get: () => ({ ...sch, scoped: state.scoped ? [state.scoped] : [] }),
        load: async () => sch,
        reload,
        subscribe: cache.subscribe,
      },
    },
  );
  return { ...r, api, conn, state, reload, schemaChanged: cache.emit };
}

const save = () => screen.getByRole("button", { name: "保存" });
const putBody = (api: { PUT: ReturnType<typeof vi.fn> }) => api.PUT.mock.lastCall?.[1].body;

beforeEach(() => {
  resetToasts();
  history.replaceState(null, "", "/projects/P1/settings");
});
afterEach(cleanup);

describe("ProjectSettings", () => {
  it("reads the violations of the project, in its Store", async () => {
    const { api } = setup();
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    expect(api.GET).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { q: "is:invalid project:P1", store: "shared" } },
    });
  });

  it("lists the statuses of the Store in order, all shown while there is no override", async () => {
    setup();
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    const boxes = within(group).getAllByRole("checkbox");
    expect(boxes.map((b) => (b as HTMLInputElement).checked)).toEqual([
      true,
      true,
      true,
      true,
      true,
    ]);
    expect(boxes.map((b) => b.closest("label")?.textContent?.replace(/\s+/g, " ").trim())).toEqual([
      expect.stringMatching(/^未着手 todo/),
      expect.stringMatching(/^進行中 in_progress/),
      expect.stringMatching(/^待機中 waiting/),
      expect.stringMatching(/^完了 done/),
      expect.stringMatching(/^中止 canceled/),
    ]);
    expect(screen.getByText(/5 \/ 5（Store と同じ）/)).toBeTruthy();
    expect(screen.getByRole("button", { name: "すべて Store の値に戻す…" })).toBeDisabled();
  });

  it("does not let the default of a category be taken off, and says why", async () => {
    setup();
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    const todo = within(group).getByRole("checkbox", { name: /未着手/ });
    expect(todo).toBeDisabled();
    expect(within(group).getByRole("checkbox", { name: /待機中/ })).toBeEnabled();
    expect(screen.getByText(/カテゴリの既定は外せません/)).toBeTruthy();
  });

  it("starts with the override the server has", async () => {
    setup({
      scoped: {
        where: "project:P1",
        statuses: ["todo", "in_progress", "done", "canceled"],
      },
    });
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    await waitFor(() =>
      expect(
        (within(group).getByRole("checkbox", { name: /待機中/ }) as HTMLInputElement).checked,
      ).toBe(false),
    );
    expect(screen.getByText(/4 \/ 5/)).toBeTruthy();
    expect(screen.queryByText(/Store と同じ）/)).toBeNull();
    expect(screen.getByRole("button", { name: "すべて Store の値に戻す…" })).toBeEnabled();
    await fireEvent.click(screen.getByRole("button", { name: "Store と同じに戻す" }));
    expect(within(group).getByRole("checkbox", { name: /待機中/ })).toBeChecked();
    expect(screen.queryByRole("button", { name: "Store と同じに戻す" })).toBeNull();
  });

  it("keeps save off while nothing changed", async () => {
    setup();
    await screen.findByRole("group", { name: "表示するステータス" });
    expect(save()).toBeDisabled();
  });

  it("saves a status taken off with that part alone, then toasts and goes quiet", async () => {
    const { api, reload } = setup();
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    await fireEvent.click(within(group).getByRole("checkbox", { name: /待機中/ }));
    expect(save()).toBeEnabled();
    await fireEvent.click(save());
    await waitFor(() => expect(api.PUT).toHaveBeenCalledTimes(1));
    expect(api.PUT).toHaveBeenCalledWith("/api/schema/scoped/{project}", {
      params: { path: { project: "P1" }, query: { store: "shared" } },
      body: { statuses: ["todo", "in_progress", "done", "canceled"] },
    });
    await waitFor(() =>
      expect(toasts.list.map((t) => t.message)).toEqual(["プロジェクトの設定を保存しました"]),
    );
    expect(save()).toBeDisabled();
    // the schema is read again at once, not when the server's event comes
    expect(reload).toHaveBeenCalledWith("shared");
  });

  it("goes back to the Store's statuses with an empty list", async () => {
    const { api } = setup({
      scoped: { where: "project:P1", statuses: ["todo", "in_progress", "done", "canceled"] },
    });
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    const waiting = within(group).getByRole("checkbox", { name: /待機中/ });
    await waitFor(() => expect(waiting).not.toBeChecked());
    await fireEvent.click(waiting);
    await fireEvent.click(save());
    await waitFor(() => expect(putBody(api)).toEqual({ statuses: [] }));
  });

  it("adds a constraint on the transitions of a status, with all of the status", async () => {
    const { api } = setup();
    await screen.findByRole("group", { name: "表示するステータス" });
    const form = screen.getByRole("group", { name: "遷移の制約を足す" });
    await fireEvent.change(within(form).getByRole("combobox", { name: "遷移元のステータス" }), {
      target: { value: "todo" },
    });
    const add = within(form).getByRole("button", { name: "遷移の制約を足す" });
    expect(add).toBeDisabled();
    await fireEvent.click(within(form).getByRole("checkbox", { name: /進行中/ }));
    await fireEvent.click(within(form).getByRole("checkbox", { name: /中止/ }));
    await fireEvent.click(add);
    const rows = screen.getByRole("list", { name: "遷移の制約" });
    expect(within(rows).getByText("このプロジェクトで足した")).toBeTruthy();
    await fireEvent.click(save());
    await waitFor(() =>
      expect(putBody(api)).toEqual({ transitions: { todo: ["in_progress", "canceled"] } }),
    );
  });

  it("adds a field a status needs, and sends the whole of required", async () => {
    const { api } = setup({
      scoped: {
        where: "project:P1",
        workflow: { transitions: {}, required: { done: ["area"] } },
      },
    });
    await screen.findByRole("group", { name: "表示するステータス" });
    const form = screen.getByRole("group", { name: "必須の項目を足す" });
    await fireEvent.change(within(form).getByRole("combobox", { name: "ステータス" }), {
      target: { value: "in_progress" },
    });
    await fireEvent.click(within(form).getByRole("checkbox", { name: "points" }));
    await fireEvent.click(within(form).getByRole("button", { name: "必須の項目を足す" }));
    await fireEvent.click(save());
    await waitFor(() =>
      expect(putBody(api)).toEqual({
        required: { done: ["area"], in_progress: ["points"] },
      }),
    );
  });

  it("offers the properties and the built-in fields the server accepts, and no others", async () => {
    setup();
    const form = await screen.findByRole("group", { name: "必須の項目を足す" });
    const names = within(form)
      .getAllByRole("checkbox")
      .map((b) => b.closest("label")?.textContent?.trim());
    expect(names).toContain("points");
    expect(names).toContain("assignees");
    expect(names).toContain("blocked_by");
    expect(names).not.toContain("title");
    expect(names).not.toContain("status");
  });

  it("shows the Store's constraints as they are, with no way to take them off", async () => {
    setup({
      scoped: {
        where: "project:P1",
        workflow: { transitions: { todo: ["done"] }, required: { in_progress: ["points"] } },
      },
    });
    const transitions = await screen.findByRole("list", { name: "遷移の制約" });
    const rows = await within(transitions).findAllByRole("listitem");
    expect(rows).toHaveLength(2);
    const store = rows.find((r) => within(r).queryByText("Store"));
    expect(store?.textContent).toMatch(/中止/);
    expect(within(store as HTMLElement).queryByRole("button")).toBeNull();
    const mine = rows.find((r) => within(r).queryByText("このプロジェクトで足した"));
    expect(within(mine as HTMLElement).getByRole("button", { name: /外す/ })).toBeTruthy();

    const required = screen.getByRole("list", { name: "必須の項目" });
    const storeRequired = within(required)
      .getAllByRole("listitem")
      .find((r) => within(r).queryByText("Store"));
    expect(storeRequired?.textContent).toMatch(/points/);
    expect(within(storeRequired as HTMLElement).queryByRole("button")).toBeNull();
  });

  it("takes off what the project added, and sends the part that is left", async () => {
    const { api } = setup({
      scoped: {
        where: "project:P1",
        workflow: { transitions: { todo: ["done", "waiting"] }, required: {} },
      },
    });
    const transitions = await screen.findByRole("list", { name: "遷移の制約" });
    await fireEvent.click(await within(transitions).findByRole("button", { name: /待機中.*外す/ }));
    await fireEvent.click(save());
    await waitFor(() => expect(putBody(api)).toEqual({ transitions: { todo: ["done"] } }));
    await waitFor(() => expect(save()).toBeDisabled());
    await fireEvent.click(within(transitions).getByRole("button", { name: /完了.*外す/ }));
    await fireEvent.click(save());
    await waitFor(() => expect(putBody(api)).toEqual({ transitions: {} }));
  });

  it("shows the words of a refusal and keeps what was edited", async () => {
    const { api } = setup();
    api.PUT.mockImplementationOnce((() =>
      fail(422, "scoped.invalid", { problems: ["scoped[0].statuses: ..."] })) as never);
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    const waiting = within(group).getByRole("checkbox", { name: /待機中/ });
    await fireEvent.click(waiting);
    await fireEvent.click(save());
    await waitFor(() => expect(screen.getAllByRole("alert").length).toBeGreaterThan(0));
    expect(waiting).not.toBeChecked();
    expect(save()).toBeEnabled();
    expect(toasts.list).toHaveLength(0);
  });

  it("sends one request when save is pressed twice", async () => {
    const { api } = setup();
    let release: (v: unknown) => void = () => {};
    api.PUT.mockImplementationOnce(() => new Promise((r) => (release = r)) as never);
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    await fireEvent.click(within(group).getByRole("checkbox", { name: /待機中/ }));
    await fireEvent.click(save());
    expect(save()).toBeDisabled();
    await fireEvent.click(save());
    expect(api.PUT).toHaveBeenCalledTimes(1);
    release(await ok({ commit: null, project: { ulid: "P1" } }));
    await waitFor(() => expect(save()).toBeDisabled());
  });

  it("asks before it takes the whole override off, and says the templates go too", async () => {
    const { api, schemaChanged, reload } = setup({
      scoped: { where: "project:P1", statuses: ["todo", "in_progress", "done", "canceled"] },
    });
    const reset = await screen.findByRole("button", { name: "すべて Store の値に戻す…" });
    await waitFor(() => expect(reset).toBeEnabled());
    await fireEvent.click(reset);
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(/テンプレート/)).toBeTruthy();
    expect(
      within(dialog).getByRole("button", { name: "コマンドをコピー: hh schema scope rm REL-15" }),
    ).toBeTruthy();
    expect(api.DELETE).not.toHaveBeenCalled();
    await fireEvent.click(within(dialog).getByRole("button", { name: "やめる" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(api.DELETE).not.toHaveBeenCalled();

    await fireEvent.click(reset);
    await fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "すべて戻す" }),
    );
    await waitFor(() => expect(api.DELETE).toHaveBeenCalledTimes(1));
    expect(api.DELETE).toHaveBeenCalledWith("/api/schema/scoped/{project}", {
      params: { path: { project: "P1" }, query: { store: "shared" } },
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(toasts.list.map((t) => t.message)).toEqual([
      "プロジェクトの設定を Store の値に戻しました",
    ]);
    expect(reload).toHaveBeenCalledWith("shared");
    schemaChanged();
    await waitFor(() => expect(reset).toBeDisabled());
  });

  it("lists the tasks that break the rules, with links, or says there are none", async () => {
    setup({ violations: [bad("REL-11", "points is required to enter done")] });
    const list = await screen.findByRole("list", { name: "違反しているタスク" });
    const row = within(list).getByRole("listitem");
    expect(within(row).getByRole("link", { name: "REL-11" })).toHaveAttribute(
      "href",
      "/tasks/U-REL-11",
    );
    expect(row.textContent).toMatch(/points is required to enter done/);
    expect(screen.getByText("いまの違反 1 件")).toBeTruthy();
    cleanup();
    setup();
    expect(await screen.findByText("違反しているタスクはありません")).toBeTruthy();
  });

  it("writes the command for the edit under it", async () => {
    setup();
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    expect(
      screen.getByRole("button", { name: "コマンドをコピー: hh schema scope set REL-15 <変更>" }),
    ).toBeTruthy();
    await fireEvent.click(within(group).getByRole("checkbox", { name: /待機中/ }));
    expect(
      screen.getByRole("button", {
        name: "コマンドをコピー: hh schema scope set REL-15 --statuses todo,in_progress,done,canceled",
      }),
    ).toBeTruthy();
  });

  it("adds --store to the commands only with two Stores or more", async () => {
    setup({ stores: ["shared", "backend"] });
    await screen.findByRole("group", { name: "表示するステータス" });
    expect(
      screen.getByRole("button", {
        name: "コマンドをコピー: hh schema scope set REL-15 <変更> --store shared",
      }),
    ).toBeTruthy();
  });

  it("makes no templates, automations or last-changed date", async () => {
    setup();
    await screen.findByRole("group", { name: "表示するステータス" });
    expect(screen.queryByText(/テンプレート/)).toBeNull();
    expect(screen.queryByText(/自動化/)).toBeNull();
    expect(screen.queryByText(/最終更新/)).toBeNull();
  });

  describe("when the schema changes", () => {
    const hidden = { where: "project:P1", statuses: ["todo", "in_progress", "done", "canceled"] };

    it("shows the new override, and reads the violations again, when nothing is being edited", async () => {
      const { api, conn, state, schemaChanged } = setup();
      const group = await screen.findByRole("group", { name: "表示するステータス" });
      await screen.findByText("違反しているタスクはありません");
      state.scoped = hidden;
      schemaChanged();
      conn.emit("schema.changed", { store: "shared" });
      await waitFor(() =>
        expect(within(group).getByRole("checkbox", { name: /待機中/ })).not.toBeChecked(),
      );
      expect(screen.queryByText(/ほかで変更されました/)).toBeNull();
      await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(2));
    });

    it("leaves the violations alone at a change of another Store", async () => {
      const { api, conn } = setup({ stores: ["shared", "backend"] });
      await screen.findByText("違反しているタスクはありません");
      conn.emit("schema.changed", { store: "backend" });
      conn.emit("tasks.changed", { store: "backend", ulids: [], removed: [] });
      await new Promise((r) => setTimeout(r, 10));
      expect(api.GET).toHaveBeenCalledTimes(1);
    });

    it("keeps an edit that is under way, and offers to read again", async () => {
      const { state, schemaChanged } = setup();
      const group = await screen.findByRole("group", { name: "表示するステータス" });
      await fireEvent.click(within(group).getByRole("checkbox", { name: /待機中/ }));
      state.scoped = {
        where: "project:P1",
        workflow: { transitions: {}, required: { done: ["area"] } },
      };
      schemaChanged();
      await screen.findByText(/ほかで変更されました/);
      expect(within(group).getByRole("checkbox", { name: /待機中/ })).not.toBeChecked();
      await fireEvent.click(screen.getByRole("button", { name: "取り直す" }));
      await waitFor(() => expect(screen.queryByText(/ほかで変更されました/)).toBeNull());
      expect(within(group).getByRole("checkbox", { name: /待機中/ })).toBeChecked();
      expect(save()).toBeDisabled();
    });

    it("takes in the other person's override once the edit is undone by hand", async () => {
      const { api, state, schemaChanged } = setup();
      const group = await screen.findByRole("group", { name: "表示するステータス" });
      await fireEvent.click(within(group).getByRole("checkbox", { name: /待機中/ }));
      state.scoped = {
        where: "project:P1",
        workflow: { transitions: {}, required: { done: ["area"] } },
      };
      schemaChanged();
      await screen.findByText(/ほかで変更されました/);
      await fireEvent.click(within(group).getByRole("checkbox", { name: /待機中/ }));
      await waitFor(() => expect(screen.queryByText(/ほかで変更されました/)).toBeNull());
      expect(
        within(screen.getByRole("list", { name: "必須の項目" })).getByText("area"),
      ).toBeTruthy();
      // the next edit is made over the other person's override, and keeps it
      await fireEvent.click(within(group).getByRole("checkbox", { name: /待機中/ }));
      await fireEvent.click(save());
      await waitFor(() => expect(api.PUT).toHaveBeenCalledTimes(1));
      expect(putBody(api)).toEqual({ statuses: ["todo", "in_progress", "done", "canceled"] });
      expect(state.scoped?.workflow?.required).toEqual({ done: ["area"] });
    });

    it("takes in what the Store holds after its own save, without saying it changed", async () => {
      const { state, schemaChanged } = setup();
      const group = await screen.findByRole("group", { name: "表示するステータス" });
      await fireEvent.click(within(group).getByRole("checkbox", { name: /待機中/ }));
      await fireEvent.click(save());
      await waitFor(() => expect(toasts.list).toHaveLength(1));
      expect(state.scoped?.statuses).toEqual(hidden.statuses);
      schemaChanged();
      await new Promise((r) => setTimeout(r, 10));
      expect(screen.queryByText(/ほかで変更されました/)).toBeNull();
      expect(within(group).getByRole("checkbox", { name: /待機中/ })).not.toBeChecked();
      expect(save()).toBeDisabled();
    });

    it("reads the violations again at tasks.changed and at resync", async () => {
      const { api, conn } = setup();
      await screen.findByText("違反しているタスクはありません");
      conn.emit("tasks.changed", { store: "shared", ulids: [], removed: [] });
      await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(2));
      conn.emit("resync");
      await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(3));
    });
  });

  it("stops reading when the screen is gone", async () => {
    const { api, conn, unmount } = setup();
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    unmount();
    conn.emit("schema.changed", { store: "shared" });
    conn.emit("tasks.changed", { store: "shared", ulids: [], removed: [] });
    conn.emit("resync");
    await new Promise((r) => setTimeout(r, 10));
    expect(api.GET).toHaveBeenCalledTimes(1);
  });

  it("shows the answer of the latest read when an older one arrives last", async () => {
    const { api, conn } = setup();
    await screen.findByText("違反しているタスクはありません");
    const pending: ((v: unknown) => void)[] = [];
    api.GET.mockImplementation((() => new Promise((r) => pending.push(r))) as never);
    conn.emit("resync");
    conn.emit("resync");
    await waitFor(() => expect(pending).toHaveLength(2));
    pending[1]?.(await ok({ tasks: [bad("REL-12", "the new one")] }));
    await screen.findByText("the new one");
    pending[0]?.(await ok({ tasks: [bad("REL-11", "the old one")] }));
    await new Promise((r) => setTimeout(r, 10));
    expect(screen.getByText("the new one")).toBeTruthy();
    expect(screen.queryByText("the old one")).toBeNull();
  });

  it("keeps the list of violations and shows the error when a new read fails", async () => {
    const { api, conn } = setup({ violations: [bad("REL-11", "the one that stays")] });
    await screen.findByText("the one that stays");
    api.GET.mockImplementation((() => fail(500, "internal")) as never);
    conn.emit("resync");
    await waitFor(() => expect(screen.getAllByRole("alert").length).toBeGreaterThan(0));
    expect(screen.getByText("the one that stays")).toBeTruthy();
  });

  it("says the project is not found", async () => {
    setup({ notFound: true });
    expect(await screen.findByText(/見つかりません/)).toBeTruthy();
    expect(screen.queryByRole("group", { name: "表示するステータス" })).toBeNull();
  });
});
