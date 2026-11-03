import { describe, expect, it, vi } from "vitest";
import type { TaskCache } from "./cache";
import type { StoreDirectory, StoreTone } from "./stores";
import { FakeConn, fakeApi, problem, settle } from "./testing";
import {
  DEFAULT_KANBAN,
  DEFAULT_TABLE,
  draftOf,
  type SavedView,
  sameDraft,
  saveBodyOf,
  ViewDirectory,
  type ViewDraft,
} from "./views";

const view = (name: string, over: Partial<SavedView> = {}): SavedView => ({
  name,
  layout: "table",
  query: "status:todo",
  table: { columns: ["id", "title", "due"], group_by: "parent" },
  kanban: { columns: "category", swimlane: "assignee" },
  ...over,
});

describe("draftOf", () => {
  it("reads every setting of the view", () => {
    expect(draftOf(view("a"))).toEqual({
      layout: "table",
      query: "status:todo",
      kanban: { columns: "category", swimlane: "assignee" },
      table: { columns: ["id", "title", "due"], groupBy: "parent" },
    });
  });

  it("takes the default for a value it does not know, and for columns that are none", () => {
    const d = draftOf(
      view("a", {
        layout: "gantt",
        table: { columns: null, group_by: "sprint" },
        kanban: { columns: "wip", swimlane: "team" },
      }),
    );
    expect(d.layout).toBe("tree");
    expect(d.kanban).toEqual(DEFAULT_KANBAN);
    expect(d.table).toEqual(DEFAULT_TABLE);
    expect(d.table.columns).not.toBe(DEFAULT_TABLE.columns);
  });
});

describe("saveBodyOf", () => {
  it("holds every setting, in the names of the API", () => {
    const d = draftOf(view("a"));
    expect(saveBodyOf(d)).toEqual({
      layout: "table",
      query: "status:todo",
      columns: ["id", "title", "due"],
      group_by: "parent",
      kanban_columns: "category",
      swimlane: "assignee",
    });
    expect(saveBodyOf(d).columns).not.toBe(d.table.columns);
  });
});

describe("sameDraft", () => {
  const base = (): ViewDraft => draftOf(view("a"));

  it("leaves the layout out", () => {
    expect(sameDraft(base(), { ...base(), layout: "kanban" })).toBe(true);
  });

  it("tells a change of the query, the kanban or the table", () => {
    expect(sameDraft(base(), { ...base(), query: "status:todo " })).toBe(true);
    expect(sameDraft(base(), { ...base(), query: "status:done" })).toBe(false);
    expect(
      sameDraft(base(), { ...base(), kanban: { columns: "status", swimlane: "assignee" } }),
    ).toBe(false);
    expect(
      sameDraft(base(), { ...base(), kanban: { columns: "category", swimlane: "none" } }),
    ).toBe(false);
    expect(
      sameDraft(base(), { ...base(), table: { columns: ["id", "title", "due"], groupBy: "none" } }),
    ).toBe(false);
    expect(
      sameDraft(base(), {
        ...base(),
        table: { columns: ["id", "due", "title"], groupBy: "parent" },
      }),
    ).toBe(false);
    expect(
      sameDraft(base(), { ...base(), table: { columns: ["id", "title"], groupBy: "parent" } }),
    ).toBe(false);
  });
});

// A StoreDirectory that the test drives: the stores, their colours and whether they are known.
class FakeStores {
  names: string[];
  known = true;
  tones = new Map<string, StoreTone>();
  private readonly subs = new Set<() => void>();
  constructor(names: string[]) {
    this.names = names;
  }
  stores() {
    return this.names.map((name) => ({ name }));
  }
  tone(s: string): StoreTone {
    return this.tones.get(s) ?? "teal";
  }
  loaded() {
    return this.known;
  }
  subscribe(fn: () => void) {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }
  change() {
    for (const fn of [...this.subs]) fn();
  }
}

function setup(world: Record<string, SavedView[] | "fail"> = {}) {
  const w = { ...world };
  const { api, calls } = fakeApi((r) => {
    const store = r.query.get("store") ?? "";
    if (r.path === "/api/views" && r.method === "GET") {
      const vs = w[store];
      if (vs === "fail") return problem(500, "internal");
      return vs ?? [];
    }
    const name = decodeURIComponent(r.path.slice("/api/views/".length));
    const body = r.body as Record<string, unknown>;
    // A write changes the world too, as the server does: the fetch after it reads the result.
    const cur = (Array.isArray(w[store]) ? w[store] : []) as SavedView[];
    if (r.method === "PUT") {
      const v = view(name, { ...body } as Partial<SavedView>);
      w[store] = [...cur.filter((x) => x.name !== name), v];
      return { view: v, commit: "c1" };
    }
    if (r.method === "PATCH" && body.name === undefined) {
      // An update: it never creates, and what is not sent stays.
      const at = cur.find((x) => x.name === name);
      if (!at) return problem(404, "view.not_found", { name });
      const v = { ...at, ...body } as SavedView;
      w[store] = cur.map((x) => (x === at ? v : x));
      return { view: v, commit: "c4" };
    }
    if (r.method === "PATCH") {
      const v = view(body.name as string);
      w[store] = [...cur.filter((x) => x.name !== name), v];
      return { view: v, commit: "c2" };
    }
    w[store] = cur.filter((x) => x.name !== name);
    return { view: view(name), commit: "c3" };
  });
  const conn = new FakeConn();
  const stores = new FakeStores(Object.keys(w));
  const noteOwnCommit = vi.fn();
  const dir = new ViewDirectory({
    api,
    conn: conn.asConnection(),
    stores: stores as unknown as StoreDirectory,
    cache: { noteOwnCommit } as unknown as TaskCache,
  });
  const views = (store: string) =>
    calls.filter((c) => c.path === "/api/views" && c.query.get("store") === store);
  return { dir, conn, stores, w, calls, noteOwnCommit, views };
}

const names = (dir: ViewDirectory) => dir.list().map((e) => `${e.store}/${e.view.name}`);

describe("ViewDirectory", () => {
  it("lists the views of every store: in the order of the stores, then by name", async () => {
    const { dir } = setup({
      shared: [view("weekly"), view("board")],
      personal: [view("mine")],
    });
    expect(dir.loaded()).toBe(false);
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    expect(names(dir)).toEqual(["shared/board", "shared/weekly", "personal/mine"]);
    expect(dir.get("shared", "weekly")?.view.query).toBe("status:todo");
    expect(dir.get("personal", "weekly")).toBeUndefined();
  });

  it("is not loaded until the stores are known, and then until every store has answered", async () => {
    const stores = new FakeStores([]);
    stores.known = false;
    const { api } = fakeApi(() => []);
    const dir = new ViewDirectory({
      api,
      conn: new FakeConn().asConnection(),
      stores: stores as unknown as StoreDirectory,
      cache: { noteOwnCommit() {} } as unknown as TaskCache,
    });
    await settle();
    expect(dir.loaded()).toBe(false);
    const fn = vi.fn();
    dir.subscribe(fn);
    stores.known = true;
    stores.names = ["shared"];
    stores.change();
    expect(dir.loaded()).toBe(false);
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    expect(fn).toHaveBeenCalled();
  });

  it("counts a store that cannot be read as done, with no views, and keeps what it had", async () => {
    const { dir, w, conn } = setup({ shared: [view("weekly")], broken: "fail" });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    expect(names(dir)).toEqual(["shared/weekly"]);
    w.shared = "fail";
    conn.emit("views.changed", { store: "shared" });
    await settle();
    expect(names(dir)).toEqual(["shared/weekly"]);
  });

  it("says which stores could not be read, until a later fetch reads them", async () => {
    const { dir, w, conn } = setup({ shared: [view("weekly")], broken: "fail" });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    expect(dir.failed("broken")).toBe(true);
    expect(dir.failed("shared")).toBe(false);
    const fn = vi.fn();
    dir.subscribe(fn);
    w.broken = [view("mine")];
    conn.emit("views.changed", { store: "broken" });
    await vi.waitFor(() => expect(dir.failed("broken")).toBe(false));
    expect(fn).toHaveBeenCalled();
    w.shared = "fail";
    conn.emit("views.changed", { store: "shared" });
    await vi.waitFor(() => expect(dir.failed("shared")).toBe(true));
    expect(fn.mock.calls.length).toBeGreaterThan(1);
  });

  it("fetches again only the store of views.changed", async () => {
    const { dir, conn, w, views } = setup({ shared: [view("weekly")], personal: [] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    const fn = vi.fn();
    dir.subscribe(fn);
    w.shared = [view("weekly"), view("monthly")];
    conn.emit("views.changed", { store: "shared" });
    await vi.waitFor(() => expect(names(dir)).toEqual(["shared/monthly", "shared/weekly"]));
    expect(views("shared")).toHaveLength(2);
    expect(views("personal")).toHaveLength(1);
    expect(fn).toHaveBeenCalledTimes(1);
    conn.emit("views.changed", { store: "nobody" });
    await settle();
    expect(views("nobody")).toHaveLength(0);
  });

  it("fetches every store again at resync, and keeps the entries that did not change", async () => {
    const { dir, conn, views } = setup({ shared: [view("weekly")], personal: [view("mine")] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    const before = dir.list();
    const fn = vi.fn();
    dir.subscribe(fn);
    conn.emit("resync");
    await vi.waitFor(() => expect(views("personal")).toHaveLength(2));
    await settle();
    expect(views("shared")).toHaveLength(2);
    expect(dir.list()).toEqual(before);
    expect(dir.list()[0]).toBe(before[0]);
    expect(fn).not.toHaveBeenCalled();
  });

  it("follows the stores: fetches one that came, drops one that went, takes a new colour", async () => {
    const { dir, stores, w } = setup({ shared: [view("weekly")] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    w.personal = [view("mine")];
    stores.names = ["shared", "personal"];
    stores.change();
    await vi.waitFor(() => expect(names(dir)).toEqual(["shared/weekly", "personal/mine"]));
    expect(dir.loaded()).toBe(true);
    stores.tones.set("personal", "readonly");
    stores.change();
    expect(dir.get("personal", "mine")?.tone).toBe("readonly");
    expect(dir.get("shared", "weekly")?.tone).toBe("teal");
    stores.names = ["personal"];
    stores.change();
    expect(names(dir)).toEqual(["personal/mine"]);
  });

  it("save puts the settings, notes the commit and shows the view at once", async () => {
    const { dir, calls, noteOwnCommit } = setup({ shared: [view("weekly")] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    const saved = await dir.save("shared", "board", { layout: "kanban", query: "kind:task" });
    expect(saved.name).toBe("board");
    const put = calls.find((c) => c.method === "PUT");
    expect(put?.path).toBe("/api/views/board");
    expect(put?.query.get("store")).toBe("shared");
    expect(put?.body).toEqual({ layout: "kanban", query: "kind:task" });
    expect(noteOwnCommit).toHaveBeenCalledWith("c1");
    expect(names(dir)).toEqual(["shared/board", "shared/weekly"]);
    expect(dir.get("shared", "board")?.view.query).toBe("kind:task");
    await dir.save("shared", "weekly", { query: "status:done" });
    expect(names(dir)).toEqual(["shared/board", "shared/weekly"]);
    expect(dir.get("shared", "weekly")?.view.query).toBe("status:done");
  });

  it("encodes the name in the path, and rename and remove change the list", async () => {
    const { dir, calls, noteOwnCommit } = setup({ shared: [view("今週 の作業"), view("b")] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    const renamed = await dir.rename("shared", "今週 の作業", "来週");
    expect(renamed.name).toBe("来週");
    const patch = calls.find((c) => c.method === "PATCH");
    expect(patch?.path).toBe(`/api/views/${encodeURIComponent("今週 の作業")}`);
    expect(patch?.body).toEqual({ name: "来週" });
    expect(patch?.query.get("store")).toBe("shared");
    expect(noteOwnCommit).toHaveBeenCalledWith("c2");
    expect(names(dir)).toEqual(["shared/b", "shared/来週"]);
    await dir.remove("shared", "b");
    const del = calls.find((c) => c.method === "DELETE");
    expect(del?.path).toBe("/api/views/b");
    expect(del?.query.get("store")).toBe("shared");
    expect(noteOwnCommit).toHaveBeenCalledWith("c3");
    expect(names(dir)).toEqual(["shared/来週"]);
  });

  it("update patches the settings, notes the commit and shows the view at once", async () => {
    const { dir, calls, noteOwnCommit } = setup({ shared: [view("weekly")] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    const updated = await dir.update("shared", "weekly", { layout: "kanban" });
    expect(updated.layout).toBe("kanban");
    const patch = calls.find((c) => c.method === "PATCH");
    expect(patch?.path).toBe("/api/views/weekly");
    expect(patch?.query.get("store")).toBe("shared");
    expect(patch?.body).toEqual({ layout: "kanban" });
    expect(noteOwnCommit).toHaveBeenCalledWith("c4");
    expect(dir.get("shared", "weekly")?.view.layout).toBe("kanban");
    expect(dir.get("shared", "weekly")?.view.query).toBe("status:todo");
  });

  it("update of a view that is gone throws not_found and does not bring the view back", async () => {
    const { dir, w } = setup({ shared: [view("weekly")] });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    w.shared = [];
    await expect(dir.update("shared", "weekly", { layout: "kanban" })).rejects.toMatchObject({
      status: 404,
      type: "view.not_found",
    });
    expect(w.shared).toEqual([]);
    expect(names(dir)).toEqual([]);
  });

  it("throws the error of a write and leaves the list as it was", async () => {
    const { api } = fakeApi((r) =>
      r.method === "GET" ? [view("weekly")] : problem(409, "view.name_taken", { name: "weekly" }),
    );
    const stores = new FakeStores(["shared"]);
    const noteOwnCommit = vi.fn();
    const dir = new ViewDirectory({
      api,
      conn: new FakeConn().asConnection(),
      stores: stores as unknown as StoreDirectory,
      cache: { noteOwnCommit } as unknown as TaskCache,
    });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    await expect(dir.save("shared", "Weekly", {})).rejects.toMatchObject({
      type: "view.name_taken",
      status: 409,
    });
    expect(names(dir)).toEqual(["shared/weekly"]);
    expect(noteOwnCommit).not.toHaveBeenCalled();
  });

  it("drops a fetch that a write overtook", async () => {
    let release: (vs: SavedView[]) => void = () => undefined;
    let n = 0;
    const { api } = fakeApi((r) => {
      if (r.method !== "GET") return { view: view("new"), commit: "c1" };
      n++;
      if (n === 1) return Promise.resolve([view("old")]);
      if (n === 2) return new Promise((res) => (release = res));
      return Promise.resolve([view("old"), view("new")]); // the fetch after the write
    });
    const conn = new FakeConn();
    const dir = new ViewDirectory({
      api,
      conn: conn.asConnection(),
      stores: new FakeStores(["shared"]) as unknown as StoreDirectory,
      cache: { noteOwnCommit() {} } as unknown as TaskCache,
    });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    conn.emit("views.changed", { store: "shared" }); // held
    await settle();
    await dir.save("shared", "new", {});
    expect(names(dir)).toEqual(["shared/new", "shared/old"]);
    release([view("old")]); // read before the write
    await settle();
    expect(names(dir)).toEqual(["shared/new", "shared/old"]);
  });

  it("reads the store again after a write, so a change of another client is not lost", async () => {
    const holds: Array<(vs: SavedView[]) => void> = [];
    let n = 0;
    const { api } = fakeApi((r) => {
      if (r.method !== "GET") return { view: view("mine"), commit: "c1" };
      n++;
      return n === 1
        ? Promise.resolve([view("a")])
        : n === 2
          ? new Promise<SavedView[]>((res) => holds.push(res))
          : Promise.resolve([view("a"), view("mine"), view("other")]);
    });
    const conn = new FakeConn();
    const dir = new ViewDirectory({
      api,
      conn: conn.asConnection(),
      stores: new FakeStores(["shared"]) as unknown as StoreDirectory,
      cache: { noteOwnCommit() {} } as unknown as TaskCache,
    });
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    conn.emit("views.changed", { store: "shared" }); // the fetch is held
    await settle();
    await dir.save("shared", "mine", {});
    expect(names(dir)).toContain("shared/mine");
    holds[0]?.([view("a"), view("other")]); // overtaken by the write: dropped
    await vi.waitFor(() => expect(names(dir)).toEqual(["shared/a", "shared/mine", "shared/other"]));
  });

  it("keeps the counter of a store that went and came back: an old fetch stays dropped", async () => {
    const holds: Array<(vs: SavedView[]) => void> = [];
    let n = 0;
    const { api } = fakeApi(() => {
      n++;
      if (n === 1) return new Promise<SavedView[]>((res) => holds.push(res));
      if (n === 2) return Promise.resolve([view("new")]);
      return [];
    });
    const stores = new FakeStores(["s"]);
    const dir = new ViewDirectory({
      api,
      conn: new FakeConn().asConnection(),
      stores: stores as unknown as StoreDirectory,
      cache: { noteOwnCommit() {} } as unknown as TaskCache,
    });
    await settle();
    stores.names = [];
    stores.change();
    stores.names = ["s"];
    stores.change();
    await vi.waitFor(() => expect(names(dir)).toEqual(["s/new"]));
    holds[0]?.([view("old")]);
    await settle();
    expect(names(dir)).toEqual(["s/new"]);
  });
});
