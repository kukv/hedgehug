import { describe, expect, it } from "vitest";
import { readSearchState, type SavedSearch } from "../search/layout";
import { type ViewContent, viewContent, viewDirty, viewScreenKey } from "./content";

const saved: ViewContent = {
  query: "kind:bug",
  layout: "table",
  table: { columns: ["id", "title"], group: "none", sort: { key: "end", desc: true } },
};

// screenOf は、保存した内容と同じ画面の状態(開閉は空)。
function screenOf(c: ViewContent): SavedSearch {
  return readSearchState({ ...c, tree: { collapsed: [] } });
}

describe("viewDirty (E3b-4)", () => {
  it("is false for the same content", () => {
    expect(viewDirty(saved, screenOf(saved))).toBe(false);
  });

  it("is true when only the query differs", () => {
    expect(viewDirty(saved, { ...screenOf(saved), query: "kind:task" })).toBe(true);
  });

  it("is true when the layout differs", () => {
    expect(viewDirty(saved, { ...screenOf(saved), layout: "tree" })).toBe(true);
  });

  it("is true when the column order differs", () => {
    const s = screenOf(saved);
    expect(viewDirty(saved, { ...s, table: { ...s.table, columns: ["title", "id"] } })).toBe(true);
  });

  it("is false when only tree.collapsed differs", () => {
    expect(viewDirty(saved, { ...screenOf(saved), tree: { collapsed: ["S1/I1"] } })).toBe(false);
  });

  it("is false after changing a setting and changing it back", () => {
    const s = screenOf(saved);
    const changed = { ...s, table: { ...s.table, sort: null } };
    const back = { ...changed, table: { ...changed.table, sort: { desc: true, key: "end" } } };
    expect(viewDirty(saved, back)).toBe(false);
  });
});

describe("viewContent", () => {
  it("takes only query, layout and table in the shape of readSearchState", () => {
    const screen: SavedSearch = { ...screenOf(saved), tree: { collapsed: ["x"] } };
    expect(viewContent(screen)).toEqual(saved);
  });
});

describe("viewScreenKey (E3b-3)", () => {
  it("is view: and the id", () => {
    expect(viewScreenKey("01jabcdefghjkmnpqrstvwxyz0")).toBe("view:01jabcdefghjkmnpqrstvwxyz0");
  });
});
