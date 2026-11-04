import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Schemas } from "../core/api/client";
import { fakeApi, problem } from "../core/testing";
import type { AppCore } from "./context";
import { resetToasts, toasts } from "./toast.svelte";
import { routeStore, runUndo, undoMessage } from "./undo";

type UndoWrite = Schemas["UndoWrite"];

function result(over: Partial<UndoWrite> = {}): UndoWrite {
  return {
    commit: "c2",
    kept: [],
    undone: {
      hash: "c1",
      date: "2026-11-03T10:00:00Z",
      author: { name: "e2e", email: "e2e@example.com" },
      store: "shared",
      op: "set",
      task: "T1",
      id: "REL-6",
      subject: "set REL-6",
      trailers: [],
    },
    ...over,
  };
}

beforeEach(() => resetToasts());

describe("undoMessage", () => {
  it("names the task and the operation", () => {
    expect(undoMessage(result())).toBe("REL-6 の変更を取り消しました");
    expect(undoMessage(result(), (store, id) => `${store}:${id}`)).toBe(
      "shared:REL-6 の変更を取り消しました",
    );
  });

  it("leaves out the task when the operation is not about one", () => {
    const r = result();
    expect(undoMessage({ ...r, undone: { ...r.undone, id: null, op: "workspace" } })).toBe(
      "ワークスペースの変更を取り消しました",
    );
  });

  it("says how many values it kept because others changed them later", () => {
    const kept = [
      { path: "t", field: "status", author: "tanaka" },
      { path: "t", field: "title", author: "sato" },
    ];
    expect(undoMessage(result({ kept }))).toBe(
      "REL-6 の変更を取り消しました · あとで別の人が変えた 2 件の値は残しました",
    );
  });

  it("says when nothing could be taken back", () => {
    const kept = [{ path: "t", field: "status", author: "tanaka" }];
    expect(undoMessage(result({ commit: null, kept }))).toBe(
      "REL-6 の変更は、変えた値がすべてあとで別の人に変えられていたため、何も戻しませんでした",
    );
  });
});

describe("routeStore", () => {
  const taskOf = (u: string) => (u === "T1" ? { store: "backend" } : undefined);

  it("takes the store of a workspace, and of the project or task shown", () => {
    expect(
      routeStore({ name: "workspace", store: "shared", ulid: "W1", tab: "overview" }, taskOf),
    ).toBe("shared");
    expect(routeStore({ name: "project", ulid: "T1", tab: "tree" }, taskOf)).toBe("backend");
    expect(routeStore({ name: "task", ulid: "T1" }, taskOf)).toBe("backend");
  });

  it("takes the store a saved view is saved in, none for the screens of every store", () => {
    expect(routeStore({ name: "view", store: "org-b/ops", view: "weekly" }, taskOf)).toBe(
      "org-b/ops",
    );
    expect(routeStore({ name: "query", layout: "table", q: "kind:bug" }, taskOf)).toBeUndefined();
    expect(routeStore({ name: "my" }, taskOf)).toBeUndefined();
  });

  it("takes the store of the first workspace at home, which shows its overview", () => {
    expect(routeStore({ name: "home" }, taskOf, { store: "shared" })).toBe("shared");
    expect(routeStore({ name: "home" }, taskOf)).toBeUndefined();
  });

  it("takes the store of the settings pages of a Store (workspaces, schema), none for /settings that names no store", () => {
    expect(routeStore({ name: "settings", page: "workspaces", store: "org-b/ops" }, taskOf)).toBe(
      "org-b/ops",
    );
    expect(
      routeStore({ name: "settings", page: "schema", store: "org-b/ops", tab: "statuses" }, taskOf),
    ).toBe("org-b/ops");
    expect(routeStore({ name: "settings", page: "general" }, taskOf)).toBeUndefined();
    expect(routeStore({ name: "settings", page: "accounts" }, taskOf)).toBeUndefined();
    expect(routeStore({ name: "settings", page: "stores" }, taskOf)).toBeUndefined();
  });

  it("has none for a task the cache does not hold", () => {
    expect(routeStore({ name: "task", ulid: "NOPE" }, taskOf)).toBeUndefined();
  });
});

describe("runUndo", () => {
  function setup(route: Parameters<typeof fakeApi>[0]) {
    const { api, calls } = fakeApi(route);
    const noteOwnCommit = vi.fn();
    const refresh = vi.fn();
    const stores = { displayId: (_store: string, id: string) => id };
    const c = { api, cache: { noteOwnCommit }, views: { refresh }, stores } as unknown as Pick<
      AppCore,
      "api" | "cache" | "views" | "stores"
    >;
    return { c, calls, noteOwnCommit, refresh };
  }

  it("posts to /api/undo with the store, remembers the commit and says what it did", async () => {
    const { c, calls, noteOwnCommit } = setup(() => result());
    await runUndo(c, "shared");
    expect(calls).toHaveLength(1);
    expect(calls[0]).toMatchObject({ method: "POST", path: "/api/undo" });
    expect(calls[0]?.query.get("store")).toBe("shared");
    expect(noteOwnCommit).toHaveBeenCalledWith("c2");
    expect(toasts.list.map((t) => t.message)).toEqual(["REL-6 の変更を取り消しました"]);
  });

  it("reads the views of the store again after it takes back a change of a view", async () => {
    // The server tells views.changed from its poll: a delete taken back before the poll saw it
    // leaves the tree as it was, so nothing is told, and the list would keep the view out.
    const { c, refresh } = setup(() =>
      result({ undone: { ...result().undone, op: "view", task: null, id: null, store: "work" } }),
    );
    await runUndo(c, "work");
    expect(refresh).toHaveBeenCalledWith("work");
  });

  it("does not read the views again after it takes back another operation", async () => {
    const { c, refresh } = setup(() => result());
    await runUndo(c, "shared");
    expect(refresh).not.toHaveBeenCalled();
  });

  it("sends no store when there is none", async () => {
    const { c, calls } = setup(() => result());
    await runUndo(c);
    expect(calls[0]?.query.has("store")).toBe(false);
  });

  it("shows nothing to undo as the message of the server", async () => {
    const { c, noteOwnCommit } = setup(() => problem(404, "undo.nothing"));
    await runUndo(c);
    expect(toasts.list.map((t) => t.message)).toEqual(["取り消せる操作がありません"]);
    expect(noteOwnCommit).not.toHaveBeenCalled();
  });

  it("shows why it is refused", async () => {
    const { c } = setup(() => problem(409, "undo.blocked", { op: "purge" }));
    await runUndo(c);
    expect(toasts.list.map((t) => t.message)).toEqual(["完全に消したものは取り消せません"]);
  });
});
