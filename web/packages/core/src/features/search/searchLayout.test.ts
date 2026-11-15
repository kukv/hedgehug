import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { get } from "../../reactive/store";
import { kindBugBody, noTable, setup } from "./fixtures.test-util";
import { defaultTable } from "./table";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

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

  it("setLayout to table keeps the selection", async () => {
    const t = setup();
    await t.s.open();
    t.s.setLayout("table");
    expect(t.clear).not.toHaveBeenCalled();
  });

  // 前にテーブルで選んだ行を、ツリーで開いたあとも残さない(パレットの「選んでいるアイテム」に出さない。E3a-10)。
  it("open with a saved tree clears the selection, and with a table keeps it", async () => {
    const tree = setup('{"query":"a","layout":"tree"}');
    await tree.s.open();
    expect(tree.clear).toHaveBeenCalledTimes(1);
    const table = setup('{"query":"a","layout":"table"}');
    await table.s.open();
    expect(table.clear).not.toHaveBeenCalled();
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

  it("showTable in table writes nothing", async () => {
    const t = setup('{"query":"a","layout":"table"}');
    await t.s.open();
    t.s.showTable();
    expect(t.sp.put).not.toHaveBeenCalled();
    expect(t.clear).not.toHaveBeenCalled();
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
    expect(written(t).layout).toBe("table");
    expect(t.sp.put).toHaveBeenCalledTimes(1);
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
