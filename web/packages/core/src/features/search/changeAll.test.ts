import { describe, expect, it, vi } from "vitest";
import { found } from "../palette/palette.test-util";
import { createChangeAll } from "./changeAll";
import type { SearchResult } from "./types";

function setup(result: SearchResult | null) {
  const calls: string[] = [];
  const set = vi.fn((rows: unknown[]) => calls.push(`set:${rows.length}`));
  const changeAll = createChangeAll({
    openSearch: async (q) => void calls.push(`open:${q}`),
    settled: async () => void calls.push("settled"),
    result: () => result,
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
    await b.changeAll("status:");
    expect(b.calls).toEqual(["open:status:", "settled", "set:0"]);
    const c = setup(null);
    await c.changeAll("kind:bug");
    expect(c.calls).toEqual(["open:kind:bug", "settled", "set:0"]);
  });
});
