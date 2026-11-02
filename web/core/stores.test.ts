import { describe, expect, it, vi } from "vitest";
import { TaskCache } from "./cache";
import { StoreDirectory } from "./stores";
import { changed, FakeConn, fakeApi, problem, settle, tv } from "./testing";

const store = (name: string, def = false) => ({
  name,
  default: def,
  path: `/data/${name}`,
  remotes: [],
});
const ws = (ulid: string, name: string, prefix: string) => ({
  ulid,
  name,
  id_prefix: prefix,
  prefix_set: "2026-11-01T00:00:00Z",
  tasks: 0,
});
const project = (ulid: string, storeName: string, wsUlid: string, rank: string, title = ulid) =>
  tv(ulid, title, {
    kind: "project",
    store: storeName,
    rank,
    workspace: { ulid: wsUlid, name: "x", id_prefix: "X" },
  });

type World = {
  stores: ReturnType<typeof store>[];
  spaces: Record<string, ReturnType<typeof ws>[]>;
  projects: ReturnType<typeof project>[];
};

function setup(world: World) {
  const { api, calls } = fakeApi((r) => {
    if (r.path === "/api/stores") return world.stores;
    if (r.path === "/api/workspaces") return world.spaces[r.query.get("store") ?? ""] ?? [];
    if (r.path === "/api/tasks") return { tasks: world.projects };
    return {};
  });
  const conn = new FakeConn();
  const cache = new TaskCache({ api, conn: conn.asConnection() });
  const dir = new StoreDirectory({ api, conn: conn.asConnection(), cache });
  return { dir, conn, calls, cache, world };
}

const baseWorld = (): World => ({
  stores: [store("shared", true), store("personal")],
  spaces: {
    shared: [ws("W2", "web", "WEB"), ws("W1", "api", "API")],
    personal: [ws("W3", "home", "HOME")],
  },
  projects: [
    project("P2", "shared", "W1", "b"),
    project("P1", "shared", "W1", "a"),
    project("P3", "personal", "W3", "a"),
  ],
});

describe("StoreDirectory", () => {
  it("lists the workspaces by store order then name, with their projects by rank", async () => {
    const { dir, calls } = setup(baseWorld());
    expect(dir.loaded()).toBe(false);
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(3));
    expect(dir.loaded()).toBe(true);
    expect(dir.workspaces().map((w) => `${w.store}/${w.name}`)).toEqual([
      "shared/api",
      "shared/web",
      "personal/home",
    ]);
    const api = dir.workspace("shared", "W1");
    expect(api?.prefix).toBe("API");
    expect(api?.projects.map((p) => p.ulid)).toEqual(["P1", "P2"]);
    expect(dir.workspace("shared", "W2")?.projects).toEqual([]);
    expect(dir.workspace("personal", "W1")).toBeUndefined();
    const asked = calls
      .filter((c) => c.path === "/api/workspaces")
      .map((c) => c.query.get("store"));
    expect(asked.sort()).toEqual(["personal", "shared"]);
    expect(calls.some((c) => c.path === "/api/tasks" && c.query.get("q") === "kind:project")).toBe(
      true,
    );
  });

  it("gives the colours in the order of the stores, grey for read-only, and wraps", async () => {
    const w = baseWorld();
    w.stores = Array.from({ length: 9 }, (_, i) => store(`s${i}`));
    w.spaces = { s1: [ws("W1", "a", "A")] };
    const { dir, conn } = setup(w);
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    expect(dir.tone("s0")).toBe("teal");
    expect(dir.tone("s1")).toBe("orange");
    expect(dir.tone("s6")).toBe("brown");
    expect(dir.tone("s7")).toBe("teal");
    expect(dir.tone("s8")).toBe("orange");
    expect(dir.tone("nobody")).toBe("readonly");

    await vi.waitFor(() => expect(dir.workspace("s1", "W1")?.tone).toBe("orange"));
    const fn = vi.fn();
    dir.subscribe(fn);
    conn.changeSync({ store: "s1", remotes: [], read_only: "no write access" });
    expect(dir.tone("s1")).toBe("readonly");
    expect(fn).toHaveBeenCalled();
    expect(dir.workspace("s1", "W1")?.tone).toBe("readonly");
  });

  it("fetches the stores and workspaces again at tasks.changed and resync", async () => {
    const { dir, conn, world } = setup(baseWorld());
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(3));
    const first = dir.workspaces()[0];
    const fn = vi.fn();
    dir.subscribe(fn);

    world.spaces.shared?.push(ws("W9", "zzz", "ZZZ"));
    conn.emit("tasks.changed", changed("shared", [], "c1", "workspace.add"));
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(4));
    expect(dir.workspaces()[0]).toBe(first); // an entry that did not change is the same object
    expect(fn).toHaveBeenCalled();

    world.stores.push(store("third"));
    world.spaces.third = [ws("W7", "t", "TT")];
    conn.emit("resync");
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(5));
    expect(dir.stores().map((s) => s.name)).toEqual(["shared", "personal", "third"]);
  });

  it("follows the projects through the task cache", async () => {
    const { dir, conn, world } = setup(baseWorld());
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(3));
    const untouched = dir.workspace("personal", "W3");

    world.projects.push(project("P4", "shared", "W2", "a"));
    conn.emit("tasks.changed", changed("shared", ["P4"], "c2"));
    await vi.waitFor(() => expect(dir.workspace("shared", "W2")?.projects.length).toBe(1));
    expect(dir.workspace("personal", "W3")).toBe(untouched);
  });

  it("does not tell the subscribers when a reload brings nothing new", async () => {
    const { dir, conn } = setup(baseWorld());
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(3));
    await settle();
    const fn = vi.fn();
    dir.subscribe(fn);
    conn.emit("resync");
    await settle();
    await settle();
    expect(fn).not.toHaveBeenCalled();
  });

  it("holds why the stores could not be listed", async () => {
    const { api } = fakeApi(() => problem(500, "internal"));
    const conn = new FakeConn();
    const cache = new TaskCache({ api, conn: conn.asConnection() });
    const dir = new StoreDirectory({ api, conn: conn.asConnection(), cache });
    await vi.waitFor(() => expect(dir.error()).not.toBeNull());
    expect(dir.error()?.type).toBe("internal");
    expect(dir.loaded()).toBe(false);
    await expect(dir.reload()).rejects.toThrow();
  });
});
