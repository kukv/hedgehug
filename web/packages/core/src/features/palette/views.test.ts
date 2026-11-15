import { describe, expect, test } from "vitest";
import { get, writable } from "../../reactive/store";
import { initialSearchState } from "../search/layout";
import type { SavedView } from "../views/views";
import { buildRows } from "./candidates";
import { labels, named, pick, setup } from "./palette.test-util";
import { createSaveQueryAsView, viewCandidates } from "./views";

// パレットの「ビュー」の群と「ビューとして保存…」(E3b-16)。

const content = { query: "kind:bug", layout: "table" as const, table: initialSearchState().table };
const view = (id: string, name: string): SavedView => ({ id, name, content });

describe("ビューの群", () => {
  test("名前に入力を含むビューを、一覧の順で route つきで返す", () => {
    const list = [view("v1", "BUG"), view("v2", "計画"), view("v3", "Bugs 2")];
    const out = viewCandidates({} as never, list, "bu");
    expect(out.map((c) => c.label)).toEqual(["BUG", "Bugs 2"]);
    expect(out[0]).toMatchObject({ group: "view", route: { name: "view", id: "v1" } });
  });

  test("入力が空なら出さず、6 件なら 5 件とほか 1 件", () => {
    const six = Array.from({ length: 6 }, (_, i) => view(`v${i}`, `バグ${i}`));
    const cands = (input: string) => viewCandidates({} as never, six, input);
    const empty = buildRows({ view: cands("") }, "", {});
    expect(empty.some((r) => r.kind === "head" && r.text === "ビュー")).toBe(false);
    const rows = buildRows({ view: cands("バグ") }, "バグ", {});
    expect(rows.filter((r) => r.kind === "candidate")).toHaveLength(5);
    expect(rows.at(-1)).toMatchObject({ kind: "more" });
  });

  test("パレットで選ぶとそのビューへ移り、Ctrl+Enter で新しいタブに開く", async () => {
    const { palette, calls } = await setup({ views: [view("v1", "バグ")] });
    await palette.open();
    palette.input("バ");
    expect(labels(palette)).toContain("バグ");
    await pick(palette, "バグ");
    expect(named(calls, "go")[0]?.args).toEqual([{ name: "view", id: "v1" }]);
    await palette.open();
    palette.input("バ");
    await pick(palette, "バグ", true);
    expect(named(calls, "openTab")[0]?.args).toEqual([{ name: "view", id: "v1" }]);
  });
});

describe("saveQueryAsView", () => {
  function fake(moved: boolean) {
    const log: string[] = [];
    const release: { fn: () => void } = { fn: () => {} };
    const state = writable({ ...initialSearchState(), query: "old" });
    const save = createSaveQueryAsView({
      openSearch: async (q) => {
        log.push(`open:${q}`);
        return moved;
      },
      settled: () => {
        log.push("settled");
        return new Promise<void>((r) => {
          release.fn = r;
        });
      },
      state: () => get(state),
      openSave: (c) => log.push(`save:${JSON.stringify(c)}`),
    });
    return { log, release, state, save };
  }

  test("saveQueryAsView waits for the search", async () => {
    const { log, release, state, save } = fake(true);
    const done = save("kind:bug");
    await Promise.resolve();
    await Promise.resolve();
    expect(log).toEqual(["open:kind:bug", "settled"]);
    const table = { ...initialSearchState().table, columns: ["id", "title", "points"] };
    state.set({ ...initialSearchState(), query: "kind:bug", table });
    release.fn();
    await done;
    expect(log).toEqual([
      "open:kind:bug",
      "settled",
      `save:${JSON.stringify({ query: "kind:bug", layout: "table", table })}`,
    ]);
  });

  test("saveQueryAsView does nothing when refused", async () => {
    const { log, save } = fake(false);
    await save("kind:bug");
    expect(log).toEqual(["open:kind:bug"]);
  });
});
