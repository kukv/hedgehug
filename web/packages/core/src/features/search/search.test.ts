import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { get } from "../../reactive/store";
import { kindBugBody, newBody, noTable, opened, result, setup } from "./fixtures.test-util";
import type { SearchResult } from "./types";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("createSearch (E1-19)", () => {
  it("searches once, 150ms after the last of three setQuery calls", async () => {
    const t = await opened();
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
    const t = await opened();
    t.s.setQuery("kind:bug");
    await vi.advanceTimersByTimeAsync(0);
    expect(t.sp.put).toHaveBeenCalledWith("search", kindBugBody);
  });

  it("drops a slow old answer that comes after a newer one", async () => {
    const t = await opened();
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
    const t = await opened();
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
    expect(t.sp.put).not.toHaveBeenCalled();
    await t.s.open();
    expect(t.sp.put).toHaveBeenCalledTimes(1);
    expect(t.sp.put).toHaveBeenLastCalledWith("search", kindBugBody);
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("kind:bug", noTable);
  });

  // 読み込み済みのタブでも、prepareQuery は印だけで、移れて open したときに 1 回書いて 1 回検索する(E3a-23)。
  it("prepareQuery after the first open writes and searches once at the next open in table", async () => {
    const t = await opened('{"query":"a","layout":"tree"}');
    t.s.prepareQuery("kind:bug");
    await vi.advanceTimersByTimeAsync(500);
    expect(t.sp.put).not.toHaveBeenCalled();
    expect(get(t.s.state)).toMatchObject({ query: "a", layout: "tree" });
    await t.s.open();
    expect(t.sp.put).toHaveBeenCalledTimes(1);
    expect(t.sp.put).toHaveBeenLastCalledWith("search", kindBugBody);
    expect(get(t.s.state).layout).toBe("table");
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("kind:bug", noTable);
  });

  it("reload searches again with the current query and keeps the rows on a failure", async () => {
    const t = await opened();
    await t.s.replaceQuery("q");
    t.search.mockRejectedValueOnce(new Error("down"));
    await t.s.reload();
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("q");
    expect(get(t.s.state).loading).toBe(false);
    await t.s.reload();
    expect(t.search).toHaveBeenCalledTimes(3);
  });

  it("drops an in-flight answer of replaceQuery when setQuery comes after it", async () => {
    const t = await opened();
    let resolveOld: (r: SearchResult) => void = () => {};
    t.search.mockImplementationOnce(() => new Promise<SearchResult>((r) => (resolveOld = r)));
    const replacing = t.s.replaceQuery("old");
    t.s.setQuery("new");
    expect(get(t.s.state).loading).toBe(true);
    resolveOld(result("OLD"));
    await replacing;
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("");
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
    // 検索は open の 1 回だけ(待っていた検索は取り消す)。
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("typed", noTable);
    await vi.advanceTimersByTimeAsync(150);
    expect(get(t.s.state).query).toBe("typed");
    expect(t.search).toHaveBeenCalledTimes(1);
  });

  it("keeps a query set by replaceQuery while open is still reading, and searches once with the saved table", async () => {
    const t = setup('{"query":"saved","layout":"tree","table":{"group":"kind"}}');
    const opening = t.s.open();
    const replacing = t.s.replaceQuery("palette");
    await Promise.all([opening, replacing]);
    expect(get(t.s.state)).toMatchObject({ query: "palette", layout: "table" });
    expect(get(t.s.state).table.group).toBe("kind");
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("palette", { sort: null, group: "kind" });
    expect(t.sp.put).toHaveBeenCalledTimes(1);
  });

  it("searches once at open even when the first read is slower than the wait", async () => {
    const t = setup();
    let release: (v: string | null) => void = () => {};
    t.sp.get = vi.fn(() => new Promise<string | null>((r) => (release = r)));
    const opening = t.s.open();
    t.s.setQuery("typed");
    await vi.advanceTimersByTimeAsync(500);
    expect(t.search).not.toHaveBeenCalled();
    release('{"query":"saved"}');
    await opening;
    await vi.advanceTimersByTimeAsync(500);
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("typed", noTable);
  });

  it("settled after replaceQuery before the first read waits for open's answer", async () => {
    const t = setup();
    let release: (v: string | null) => void = () => {};
    t.sp.get = vi.fn(() => new Promise<string | null>((r) => (release = r)));
    const opening = t.s.open();
    await t.s.replaceQuery("kind:bug");
    let done = false;
    void t.s.settled().then(() => (done = true));
    await vi.advanceTimersByTimeAsync(500);
    expect(done).toBe(false);
    release(null);
    await opening;
    await vi.advanceTimersByTimeAsync(0);
    expect(done).toBe(true);
    expect(get(t.s.state).resultQuery).toBe("kind:bug");
  });

  it("a later open with a query typed while reading searches once", async () => {
    const t = await opened('{"query":"saved"}');
    const opening = t.s.open();
    t.s.setQuery("typed");
    await opening;
    await vi.advanceTimersByTimeAsync(500);
    expect(get(t.s.state).query).toBe("typed");
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("typed", noTable);
    expect(t.sp.put).toHaveBeenCalledTimes(1);
  });

  it("opens with an empty query when the saved body is JSON null", async () => {
    const t = setup("null");
    await t.s.open();
    expect(t.search).toHaveBeenCalledWith("", noTable);
  });

  it("shows the error as a toast and keeps the previous rows when a search fails", async () => {
    const t = await opened();
    await t.s.replaceQuery("q");
    t.search.mockRejectedValueOnce(new Error("接続できません"));
    await t.s.replaceQuery("q2");
    expect(get(t.toasts.list).map((x) => [x.text, x.error])).toEqual([["接続できません", true]]);
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("q");
    expect(get(t.s.state).loading).toBe(false);
  });

  it("does not toast a failed reload (the reflection of a change shows no failure)", async () => {
    const t = await opened();
    await t.s.replaceQuery("q");
    t.search.mockRejectedValueOnce(new Error("down"));
    await t.s.reload();
    expect(get(t.toasts.list)).toEqual([]);
  });
});

describe("createSearch の画面の状態の鍵と復元 (E3b)", () => {
  const content = {
    query: "kind:bug",
    layout: "tree" as const,
    table: { columns: ["id", "title"], group: "none", sort: null },
  };

  it("uses the given screen key", async () => {
    const t = setup(null, { screen: "view:abc" });
    await t.s.open();
    expect(t.sp.get).toHaveBeenCalledWith("view:abc");
    t.s.setQuery("kind:bug");
    await vi.advanceTimersByTimeAsync(0);
    expect(t.sp.put).toHaveBeenCalledWith("view:abc", kindBugBody);
    expect(t.sp.put).not.toHaveBeenCalledWith("search", expect.anything());
  });

  it("fallback when nothing saved", async () => {
    const t = setup(null, { fallback: () => content });
    await t.s.open();
    expect(get(t.s.state)).toMatchObject({ ...content, tree: { collapsed: [] } });
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("kind:bug", noTable);
  });

  it("saved state wins over fallback", async () => {
    const fallback = vi.fn(() => content);
    const t = setup('{"query":"a"}', { fallback });
    await t.s.open();
    expect(fallback).not.toHaveBeenCalled();
    expect(get(t.s.state).query).toBe("a");
  });

  it("restore replaces content and searches", async () => {
    const t = await opened('{"query":"a","layout":"tree","tree":{"collapsed":["s/1"]}}');
    const table = { columns: ["id"], group: "kind", sort: { key: "id", desc: true } };
    t.s.restore({ query: "x", layout: "table", table });
    await vi.advanceTimersByTimeAsync(0);
    expect(get(t.s.state)).toMatchObject({ query: "x", layout: "table", table });
    expect(JSON.parse(vi.mocked(t.sp.put).mock.calls.at(-1)?.[1] ?? "")).toEqual({
      query: "x",
      layout: "table",
      table,
      tree: { collapsed: ["s/1"] },
    });
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("x", { sort: { key: "id", desc: true }, group: "kind" });
  });
});
