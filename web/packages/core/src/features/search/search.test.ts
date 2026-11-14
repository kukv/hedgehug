import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ScreenStatesPort, SearchPort } from "../../port/search";
import { get } from "../../reactive/store";
import { createScreenStates } from "../screenstate/screenStates";
import type { Timer } from "../toasts/toasts";
import { createSearch } from "./search";
import type { SearchResult } from "./types";

// core の lib は ES2023 だけで setTimeout の型がないため、ここで宣言する。
declare function setTimeout(fn: () => void, ms: number): unknown;
declare function clearTimeout(handle: unknown): void;

const timer: Timer = {
  set: (fn, ms) => setTimeout(fn, ms),
  clear: (h) => clearTimeout(h),
};

const result = (...ids: string[]): SearchResult => ({
  errors: [],
  hasKey: true,
  group: "",
  deleted: false,
  rows: ids.map((id) => ({
    storeId: "s1",
    storeName: "main",
    storeColor: "#000",
    workspaceId: "w1",
    projectId: "p1",
    itemId: id,
    display: id,
    title: id,
    kind: "task",
    status: null,
    assignee: null,
    priority: null,
    end: "",
    points: null,
    oldIdHit: false,
    readOnly: false,
    deleted: null,
  })),
});

function setup(saved: string | null = null) {
  const search = vi.fn(async (q: string) => result(q));
  const port: SearchPort = { search };
  const sp: ScreenStatesPort = {
    get: vi.fn(async () => saved),
    put: vi.fn(async () => {}),
  };
  const s = createSearch({ port, screenStates: createScreenStates(sp), timer });
  return { s, search, sp };
}

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
    expect(t.search).toHaveBeenCalledWith("abc");
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("abc");
    expect(get(t.s.state).loading).toBe(false);
  });

  it("writes the query to the screen state at each setQuery", async () => {
    const t = setup();
    t.s.setQuery("kind:bug");
    await vi.advanceTimersByTimeAsync(0);
    expect(t.sp.put).toHaveBeenCalledWith("search", '{"query":"kind:bug"}');
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
    expect(t.search).toHaveBeenCalledWith("status:done");
    expect(get(t.s.state)).toMatchObject({ query: "status:done", loading: false });
    expect(get(t.s.state).result).not.toBeNull();
  });

  it("open with nothing saved searches with an empty query", async () => {
    const t = setup();
    await t.s.open();
    expect(t.search).toHaveBeenCalledWith("");
  });

  it("replaceQuery searches at once and cancels a waiting search", async () => {
    const t = setup();
    t.s.setQuery("old");
    await t.s.replaceQuery("new");
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("new");
    expect(get(t.s.state).query).toBe("new");
    await vi.advanceTimersByTimeAsync(500);
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.sp.put).toHaveBeenLastCalledWith("search", '{"query":"new"}');
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
});
