import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { get } from "../../reactive/store";
import { found } from "../palette/palette.test-util";
import { createChangeAll } from "./changeAll";
import { result, setup as setupSearch } from "./fixtures.test-util";
import type { SearchResult } from "./types";

function setup(res: SearchResult | null) {
  const calls: string[] = [];
  const set = vi.fn((rows: unknown[]) => calls.push(`set:${rows.length}`));
  const changeAll = createChangeAll({
    openSearch: async (q) => {
      calls.push(`open:${q}`);
      return true;
    },
    settled: async () => void calls.push("settled"),
    state: () => ({ result: res, resultQuery: res ? "kind:bug" : null }),
    selection: { set },
    openBulkChange: async () => void calls.push("bulk"),
  });
  return { calls, set, changeAll };
}

describe("createChangeAll", () => {
  it("opens the search, waits, selects all selectable rows, then opens the bulk change", async () => {
    const rows = found(2).rows.map((r, i) => ({ ...r, itemId: `I${i}`, deleted: null }));
    const { calls, set, changeAll } = setup({ ...found(2), rows });
    await changeAll("kind:bug");
    expect(calls).toEqual(["open:kind:bug", "settled", "set:2", "bulk"]);
    expect(set.mock.calls[0]?.[0]).toHaveLength(2);
  });

  it("only clears the selection when there are no rows or an error", async () => {
    const a = setup(found(0));
    await a.changeAll("kind:bug");
    expect(a.calls).toEqual(["open:kind:bug", "settled", "set:0"]);
    const b = setup({ ...found(0), errors: ["x"] });
    await b.changeAll("kind:bug");
    expect(b.calls).toEqual(["open:kind:bug", "settled", "set:0"]);
  });
});

// 検索の流れ(createSearch)と組み合わせて、結果が求めたクエリへの答えでないときは何もしないことを確かめる。
describe("createChangeAll with createSearch", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  function withSearch(moved = true) {
    const t = setupSearch();
    const set = vi.fn();
    const bulk = vi.fn(async () => {});
    const changeAll = createChangeAll({
      openSearch: async (q) => {
        t.s.prepareQuery(q);
        if (!moved) t.s.cancelPrepare();
        return moved;
      },
      settled: () => t.s.settled(),
      state: () => get(t.s.state),
      selection: { set },
      openBulkChange: bulk,
    });
    return { ...t, set, bulk, changeAll };
  }

  it("does nothing when the search fails and the previous rows are left", async () => {
    const t = withSearch();
    await t.s.replaceQuery("old");
    t.search.mockRejectedValueOnce(new Error("down"));
    const running = t.changeAll("kind:bug");
    await t.s.open();
    await running;
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("old");
    expect(t.set).not.toHaveBeenCalled();
    expect(t.bulk).not.toHaveBeenCalled();
  });

  it("does nothing when the same query once succeeded and its new search fails", async () => {
    const t = withSearch();
    await t.s.replaceQuery("kind:bug");
    t.search.mockRejectedValueOnce(new Error("down"));
    const running = t.changeAll("kind:bug");
    await t.s.open();
    await running;
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("kind:bug");
    expect(t.set).not.toHaveBeenCalled();
    expect(t.bulk).not.toHaveBeenCalled();
  });

  it("does nothing when the move is declined, and a later open does not open the dialog", async () => {
    const t = withSearch(false);
    await t.changeAll("kind:bug");
    await t.s.open();
    await vi.advanceTimersByTimeAsync(0);
    expect(t.set).not.toHaveBeenCalled();
    expect(t.bulk).not.toHaveBeenCalled();
  });

  it("does nothing when the query is replaced while waiting", async () => {
    const t = withSearch();
    let resolve: (r: SearchResult) => void = () => {};
    t.search.mockImplementationOnce(() => new Promise<SearchResult>((r) => (resolve = r)));
    const running = t.changeAll("kind:bug");
    const opening = t.s.open();
    await vi.advanceTimersByTimeAsync(0);
    t.s.setQuery("other");
    resolve(result("kind:bug"));
    await opening;
    await vi.advanceTimersByTimeAsync(150);
    await running;
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("other");
    expect(t.set).not.toHaveBeenCalled();
    expect(t.bulk).not.toHaveBeenCalled();
  });

  it("sets resultQuery to the query the result answers", async () => {
    const t = withSearch();
    const running = t.changeAll("kind:bug");
    await t.s.open();
    await running;
    expect(get(t.s.state).resultQuery).toBe("kind:bug");
    expect(t.set).toHaveBeenCalledTimes(1);
    expect(t.bulk).toHaveBeenCalledTimes(1);
  });
});
