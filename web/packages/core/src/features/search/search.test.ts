import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { get } from "../../reactive/store";
import { kindBugBody, newBody, noTable, result, setup } from "./fixtures.test-util";
import { defaultTable } from "./table";
import type { SearchResult } from "./types";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("createSearch (E1-19)", () => {
  it("searches once, 150ms after the last of three setQuery calls", async () => {
    const t = setup();
    t.s.setQuery("a");
    t.s.setQuery("ab");
    t.s.setQuery("abc");
    expect(get(t.s.state).query).toBe("abc");
    await vi.advanceTimersByTimeAsync(149);
    expect(t.search).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("abc", noTable);
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("abc");
    expect(get(t.s.state).loading).toBe(false);
  });

  it("writes the query to the screen state at each setQuery", async () => {
    const t = setup();
    t.s.setQuery("kind:bug");
    await vi.advanceTimersByTimeAsync(0);
    expect(t.sp.put).toHaveBeenCalledWith("search", kindBugBody);
  });

  it("drops a slow old answer that comes after a newer one", async () => {
    const t = setup();
    const resolvers: Record<string, (r: SearchResult) => void> = {};
    t.search.mockImplementation(
      (q: string) => new Promise<SearchResult>((r) => (resolvers[q] = r)),
    );
    t.s.setQuery("a");
    await vi.advanceTimersByTimeAsync(150);
    t.s.setQuery("b");
    await vi.advanceTimersByTimeAsync(150);
    resolvers.b?.(result("B"));
    await vi.advanceTimersByTimeAsync(0);
    resolvers.a?.(result("A"));
    await vi.advanceTimersByTimeAsync(0);
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("B");
    expect(get(t.s.state).loading).toBe(false);
  });

  it("open searches with the saved query", async () => {
    const t = setup('{"query":"status:done"}');
    await t.s.open();
    expect(t.search).toHaveBeenCalledWith("status:done", noTable);
    expect(get(t.s.state)).toMatchObject({ query: "status:done", loading: false });
    expect(get(t.s.state).result).not.toBeNull();
  });

  it("open with nothing saved searches with an empty query", async () => {
    const t = setup();
    await t.s.open();
    expect(t.search).toHaveBeenCalledWith("", noTable);
  });

  it("replaceQuery searches at once and cancels a waiting search", async () => {
    const t = setup();
    t.s.setQuery("old");
    await t.s.replaceQuery("new");
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("new", noTable);
    expect(get(t.s.state).query).toBe("new");
    await vi.advanceTimersByTimeAsync(500);
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.sp.put).toHaveBeenLastCalledWith("search", newBody);
  });

  it("prepareQuery does not search, and the next open writes and searches it once", async () => {
    const t = setup('{"query":"saved"}');
    t.s.setQuery("typing");
    t.s.prepareQuery("kind:bug");
    await vi.advanceTimersByTimeAsync(500);
    expect(t.search).not.toHaveBeenCalled();
    expect(get(t.s.state).query).toBe("kind:bug");
    await t.s.open();
    expect(t.sp.put).toHaveBeenLastCalledWith("search", kindBugBody);
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("kind:bug", noTable);
  });

  it("prepareQuery after the first open writes at once in table", async () => {
    const t = setup('{"query":"a","layout":"tree"}');
    await t.s.open();
    t.s.prepareQuery("kind:bug");
    expect(t.sp.put).toHaveBeenLastCalledWith("search", kindBugBody);
    expect(get(t.s.state).layout).toBe("table");
  });

  it("reload searches again with the current query and keeps the rows on a failure", async () => {
    const t = setup();
    await t.s.replaceQuery("q");
    t.search.mockRejectedValueOnce(new Error("down"));
    await t.s.reload();
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("q");
    expect(get(t.s.state).loading).toBe(false);
    await t.s.reload();
    expect(t.search).toHaveBeenCalledTimes(3);
  });

  it("drops an in-flight answer of replaceQuery when setQuery comes after it", async () => {
    const t = setup();
    let resolveOld: (r: SearchResult) => void = () => {};
    t.search.mockImplementationOnce(() => new Promise<SearchResult>((r) => (resolveOld = r)));
    const replacing = t.s.replaceQuery("old");
    t.s.setQuery("new");
    expect(get(t.s.state).loading).toBe(true);
    resolveOld(result("OLD"));
    await replacing;
    expect(get(t.s.state).result).toBeNull();
    expect(get(t.s.state).loading).toBe(true);
    await vi.advanceTimersByTimeAsync(150);
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("new");
    expect(get(t.s.state).loading).toBe(false);
  });

  it("keeps a query typed while open is still reading the saved one", async () => {
    const t = setup('{"query":"saved"}');
    const opening = t.s.open();
    t.s.setQuery("typed");
    await opening;
    await vi.advanceTimersByTimeAsync(150);
    expect(get(t.s.state).query).toBe("typed");
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("typed", noTable);
  });

  it("keeps a query set by replaceQuery while open is still reading", async () => {
    const t = setup('{"query":"saved"}');
    const opening = t.s.open();
    const replacing = t.s.replaceQuery("palette");
    await Promise.all([opening, replacing]);
    expect(get(t.s.state).query).toBe("palette");
    expect(t.search).not.toHaveBeenCalledWith("saved", noTable);
  });

  it("opens with an empty query when the saved body is JSON null", async () => {
    const t = setup("null");
    await t.s.open();
    expect(t.search).toHaveBeenCalledWith("", noTable);
  });

  it("shows the error as a toast and keeps the previous rows when a search fails", async () => {
    const t = setup();
    await t.s.replaceQuery("q");
    t.search.mockRejectedValueOnce(new Error("接続できません"));
    await t.s.replaceQuery("q2");
    expect(get(t.toasts.list).map((x) => [x.text, x.error])).toEqual([["接続できません", true]]);
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("q");
    expect(get(t.s.state).loading).toBe(false);
  });

  it("does not toast a failed reload (the reflection of a change shows no failure)", async () => {
    const t = setup();
    await t.s.replaceQuery("q");
    t.search.mockRejectedValueOnce(new Error("down"));
    await t.s.reload();
    expect(get(t.toasts.list)).toEqual([]);
  });
});

describe("createSearch layout (E3a-2・9・10・22)", () => {
  const savedTree = (query: string, table: object) =>
    JSON.stringify({ query, layout: "tree", table, tree: { collapsed: ["s/1"] } });
  const written = (t: ReturnType<typeof setup>) =>
    JSON.parse(String(vi.mocked(t.sp.put).mock.lastCall?.[1]));

  it("open keeps the saved layout", async () => {
    const t = setup('{"query":"a","layout":"tree"}');
    await t.s.open();
    expect(get(t.s.state).layout).toBe("tree");
  });

  it("setLayout writes the state without searching", async () => {
    const t = setup();
    await t.s.open();
    t.s.setLayout("tree");
    expect(written(t)).toEqual({
      query: "",
      layout: "tree",
      table: defaultTable,
      tree: { collapsed: [] },
    });
    expect(t.search).toHaveBeenCalledTimes(1);
  });

  it("setLayout away from table clears the selection", async () => {
    const t = setup();
    await t.s.open();
    t.s.setLayout("tree");
    expect(t.clear).toHaveBeenCalledTimes(1);
  });

  it("setCollapsed writes the collapsed keys without searching", async () => {
    const t = setup();
    await t.s.open();
    t.s.setCollapsed(["s/1", "s/2"]);
    expect(written(t).tree).toEqual({ collapsed: ["s/1", "s/2"] });
    expect(get(t.s.state).tree).toEqual({ collapsed: ["s/1", "s/2"] });
    expect(t.search).toHaveBeenCalledTimes(1);
  });

  it("showTable switches to table", async () => {
    const t = setup('{"query":"a","layout":"tree"}');
    await t.s.open();
    t.s.showTable();
    expect(get(t.s.state).layout).toBe("table");
    expect(written(t).layout).toBe("table");
  });

  it("showTable before the first open keeps the saved state", async () => {
    const table = { columns: ["id", "title"], group: "none", sort: null };
    const t = setup(savedTree("kind:bug", table));
    t.s.showTable();
    expect(t.sp.put).not.toHaveBeenCalled();
    await t.s.open();
    const want = { query: "kind:bug", layout: "table", table, tree: { collapsed: ["s/1"] } };
    expect(get(t.s.state)).toMatchObject(want);
    expect(written(t)).toEqual(want);
    expect(t.sp.put).toHaveBeenCalledTimes(1);
  });

  it("showTable while open is reading switches to table after the read", async () => {
    const t = setup('{"query":"a","layout":"tree"}');
    const opening = t.s.open();
    t.s.showTable();
    await opening;
    expect(get(t.s.state).layout).toBe("table");
  });

  it("replaceQuery opens in table", async () => {
    const t = setup('{"query":"a","layout":"tree"}');
    await t.s.open();
    await t.s.replaceQuery("kind:bug");
    expect(get(t.s.state).layout).toBe("table");
    expect(t.sp.put).toHaveBeenLastCalledWith("search", kindBugBody);
  });

  it("prepareQuery before the first open keeps the saved table", async () => {
    const table = {
      columns: ["id", "title", "points"],
      group: "kind",
      sort: { key: "id", desc: true },
    };
    const t = setup(savedTree("a", table));
    t.s.prepareQuery("kind:bug");
    expect(t.sp.put).not.toHaveBeenCalled();
    await t.s.open();
    const want = { query: "kind:bug", layout: "table", table, tree: { collapsed: ["s/1"] } };
    expect(get(t.s.state)).toMatchObject(want);
    expect(written(t)).toEqual(want);
    expect(t.sp.put).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("kind:bug", { sort: table.sort, group: "kind" });
  });

  it("prepareQuery then cancelPrepare before open releases settled and keeps the query for the next open", async () => {
    const t = setup(savedTree("a", { columns: ["id", "title"], group: "none", sort: null }));
    t.s.prepareQuery("kind:bug");
    t.s.cancelPrepare();
    await t.s.settled();
    expect(t.sp.put).not.toHaveBeenCalled();
    await t.s.open();
    expect(written(t)).toMatchObject({ query: "kind:bug", layout: "table" });
  });

  it("prepareQuery while open is reading replaces the query after the read", async () => {
    const table = { columns: ["id", "title"], group: "none", sort: null };
    const t = setup(savedTree("a", table));
    const opening = t.s.open();
    t.s.prepareQuery("kind:bug");
    await opening;
    const want = { query: "kind:bug", layout: "table", table, tree: { collapsed: ["s/1"] } };
    expect(written(t)).toEqual(want);
    expect(t.sp.put).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("kind:bug", noTable);
    await t.s.settled();
  });
});
