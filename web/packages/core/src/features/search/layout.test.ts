import { describe, expect, it } from "vitest";
import { layoutChoices, readSearchState } from "./layout";
import { defaultTable } from "./table";

describe("readSearchState (E3a-2)", () => {
  it("fills the layout and the tree with the first values", () => {
    expect(readSearchState({ query: "a" })).toEqual({
      query: "a",
      layout: "table",
      table: defaultTable,
      tree: { collapsed: [] },
    });
  });

  it("uses table for an unknown layout", () => {
    expect(readSearchState({ layout: "kanban" }).layout).toBe("table");
  });

  it("uses an empty list when collapsed has a value of the wrong shape", () => {
    expect(readSearchState({ tree: { collapsed: ["s/1", 3] } }).tree).toEqual({ collapsed: [] });
  });

  it("keeps a saved tree layout and its collapsed keys", () => {
    const s = readSearchState({ layout: "tree", tree: { collapsed: ["s/1"] } });
    expect(s.layout).toBe("tree");
    expect(s.tree).toEqual({ collapsed: ["s/1"] });
  });

  it("reads null as the first values", () => {
    expect(readSearchState(null)).toMatchObject({ query: "", layout: "table" });
  });

  it("lists the layouts in the order tree, table (E3a-1)", () => {
    expect(layoutChoices).toEqual([
      { id: "tree", label: "ツリー" },
      { id: "table", label: "テーブル" },
    ]);
  });
});
