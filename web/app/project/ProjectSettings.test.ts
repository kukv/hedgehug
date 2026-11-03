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

type Opts = {
  scoped?: Scoped;
  violations?: TaskView[];
  stores?: string[];
  notFound?: boolean;
  loadError?: Error;
};

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
        get: () =>
          opts.loadError ? undefined : { ...sch, scoped: state.scoped ? [state.scoped] : [] },
        load: async () => (opts.loadError ? Promise.reject(opts.loadError) : sch),
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
    await fireEvent.change(within(form).getByRole("combobox", { name: "必須にするステータス" }), {
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

    it("shows the new override when nothing is being edited", async () => {
      const { state, schemaChanged } = setup();
      const group = await screen.findByRole("group", { name: "表示するステータス" });
      state.scoped = hidden;
      schemaChanged();
      await waitFor(() =>
        expect(within(group).getByRole("checkbox", { name: /待機中/ })).not.toBeChecked(),
      );
      expect(screen.queryByText(/ほかで変更されました/)).toBeNull();
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
  });

  it("says why the screen cannot be drawn when the schema cannot be read", async () => {
    setup({ loadError: new Error("boom") });
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("スキーマを読めません");
    expect(screen.queryByText("読み込み中…")).toBeNull();
  });

  it("explains the checkboxes that cannot be taken off", async () => {
    setup();
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    expect(within(group).getByRole("checkbox", { name: /未着手/ })).toHaveAccessibleDescription(
      /カテゴリの既定は外せません/,
    );
    expect(
      within(group).getByRole("checkbox", { name: /待機中/ }),
    ).not.toHaveAccessibleDescription();
  });

  it("leaves the inputs alone while a request is on its way", async () => {
    const { api } = setup({
      scoped: {
        where: "project:P1",
        statuses: ["todo", "in_progress", "done", "canceled"],
        workflow: { transitions: { todo: ["done"] }, required: {} },
      },
    });
    let release: (v: unknown) => void = () => {};
    api.PUT.mockImplementationOnce(() => new Promise((r) => (release = r)) as never);
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    await waitFor(() =>
      expect(within(group).getByRole("checkbox", { name: /待機中/ })).not.toBeChecked(),
    );
    await fireEvent.click(within(group).getByRole("checkbox", { name: /進行中/ }));
    await fireEvent.click(save());
    expect(within(group).getByRole("checkbox", { name: /待機中/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Store と同じに戻す" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "完了を外す" })).toBeDisabled();
    expect(screen.getByRole("combobox", { name: "遷移元のステータス" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "すべて Store の値に戻す…" })).toBeDisabled();
    release(await ok({ commit: null, project: { ulid: "P1" } }));
    await waitFor(() =>
      expect(within(group).getByRole("checkbox", { name: /待機中/ })).toBeEnabled(),
    );
  });

  it("does not close the question with Esc while the request is on its way", async () => {
    const { api } = setup({
      scoped: { where: "project:P1", statuses: ["todo", "in_progress", "done", "canceled"] },
    });
    let release: (v: unknown) => void = () => {};
    api.DELETE.mockImplementationOnce(() => new Promise((r) => (release = r)) as never);
    const reset = await screen.findByRole("button", { name: "すべて Store の値に戻す…" });
    await waitFor(() => expect(reset).toBeEnabled());
    await fireEvent.click(reset);
    await fireEvent.click(screen.getByRole("button", { name: "すべて戻す" }));
    await waitFor(() => expect(api.DELETE).toHaveBeenCalledTimes(1));
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.getByRole("dialog")).toBeTruthy();
    release(await ok({ commit: null, project: { ulid: "P1" } }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("says nothing changed elsewhere when the next edit comes right after our own save", async () => {
    setup();
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    await fireEvent.click(within(group).getByRole("checkbox", { name: /待機中/ }));
    await fireEvent.click(save());
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    await fireEvent.click(within(group).getByRole("checkbox", { name: /進行中/ }));
    expect(save()).toBeEnabled();
    expect(screen.queryByText(/ほかで変更されました/)).toBeNull();
  });

  it("keeps the saved values, and says nothing changed elsewhere, when the schema cannot be read again", async () => {
    const { reload } = setup();
    // the read fails: the cache holds the old schema and tells nobody
    reload.mockImplementation(async () => {});
    const group = await screen.findByRole("group", { name: "表示するステータス" });
    await fireEvent.click(within(group).getByRole("checkbox", { name: /待機中/ }));
    await fireEvent.click(save());
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    await new Promise((r) => setTimeout(r, 10));
    expect(within(group).getByRole("checkbox", { name: /待機中/ })).not.toBeChecked();
    expect(screen.queryByText(/ほかで変更されました/)).toBeNull();
    expect(save()).toBeDisabled();
  });

  it("says the project is not found", async () => {
    setup({ notFound: true });
    expect(await screen.findByText(/見つかりません/)).toBeTruthy();
    expect(screen.queryByRole("group", { name: "表示するステータス" })).toBeNull();
  });
});
