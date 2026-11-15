import { describe, expect, it } from "vitest";
import { result } from "./fixtures.test-util";
import { deletedTreeText, layoutChoices, layoutNote, readSearchState } from "./layout";
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

describe("layoutNote (E3a-8)", () => {
  const base = result();

  it("notes sort: only", () => {
    expect(layoutNote("tree", { ...base, hasSort: true })).toBe(
      "ツリー・ガントでは sort: は使われず、並び順のままです",
    );
  });

  it("notes group: only", () => {
    expect(layoutNote("tree", { ...base, group: "kind" })).toBe(
      "ツリー・ガントでは group: は使われず、並び順のままです",
    );
  });

  it("notes both", () => {
    expect(layoutNote("tree", { ...base, hasSort: true, group: "kind" })).toBe(
      "ツリー・ガントでは sort: と group: は使われず、並び順のままです",
    );
  });

  it("has no note without sort: or group:, in the table, or without a result", () => {
    expect(layoutNote("tree", base)).toBeNull();
    expect(layoutNote("table", { ...base, hasSort: true, group: "kind" })).toBeNull();
    expect(layoutNote("tree", null)).toBeNull();
  });

  it("has the text for deleted items in the tree (E3a-7)", () => {
    expect(deletedTreeText).toBe("削除済みのアイテムはテーブルで表示します。");
  });
});
