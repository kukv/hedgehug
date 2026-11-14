import { describe, expect, it } from "vitest";
import type { SelectedRow } from "../selection/rowSelection";
import {
  columnChoices,
  defaultTable,
  groupChoices,
  headerCheck,
  nextSort,
  readTable,
  selectable,
  shownColumns,
  tableRequest,
  toggleColumn,
} from "./table";
import type { SearchColumn, SearchRow } from "./types";

const col = (key: string): SearchColumn => ({ key, name: key, type: "", sortable: true });
const all = ["id", "kind", "title", "status", "priority", "start", "points"].map(col);
const row = (itemId: string, deleted = false): SearchRow =>
  ({
    storeId: "s",
    itemId,
    display: itemId,
    readOnly: false,
    deleted: deleted ? { daysLeft: 3 } : null,
  }) as SearchRow;
const sel = (itemId: string): SelectedRow => ({
  storeId: "s",
  itemId,
  display: itemId,
  readOnly: false,
});

describe("readTable", () => {
  it("returns the default for undefined", () => {
    expect(readTable(undefined)).toEqual(defaultTable);
  });
  it("replaces only malformed fields", () => {
    expect(readTable({ columns: "x", group: "kind", sort: { key: 1 } })).toEqual({
      columns: defaultTable.columns,
      group: "kind",
      sort: null,
    });
    expect(readTable({ group: "bogus" }).group).toBe("none");
  });
  it("keeps valid fields", () => {
    const v = { columns: ["id", "title"], group: "none", sort: { key: "id", desc: true } };
    expect(readTable(v)).toEqual(v);
  });
});

describe("groupChoices", () => {
  it("has 11 choices", () => {
    expect(groupChoices).toHaveLength(11);
    expect(groupChoices[0]).toEqual({ id: "none", label: "なし" });
  });
});

describe("toggleColumn", () => {
  it("adds to the end, removes, ignores title", () => {
    expect(toggleColumn(defaultTable, "start").columns.at(-1)).toBe("start");
    expect(toggleColumn(defaultTable, "priority").columns).not.toContain("priority");
    expect(toggleColumn(defaultTable, "title")).toBe(defaultTable);
  });
  it("clears sort only when hiding the sorted column", () => {
    const t = { ...defaultTable, sort: { key: "points", desc: true } };
    expect(toggleColumn(t, "points").sort).toBeNull();
    expect(toggleColumn(t, "priority").sort).toEqual({ key: "points", desc: true });
  });
});

describe("nextSort", () => {
  it("cycles asc, desc, none", () => {
    const a = nextSort(defaultTable, "title");
    expect(a.sort).toEqual({ key: "title", desc: false });
    const b = nextSort(a, "title");
    expect(b.sort).toEqual({ key: "title", desc: true });
    expect(nextSort(b, "title").sort).toBeNull();
    expect(nextSort(b, "points").sort).toEqual({ key: "points", desc: false });
  });
});

describe("tableRequest", () => {
  it("strips p: and lowercases", () => {
    const t = { ...defaultTable, sort: { key: "p:Customer", desc: true } };
    expect(tableRequest(t)).toEqual({ sort: { key: "customer", desc: true }, group: "none" });
    expect(tableRequest(defaultTable).sort).toBeNull();
  });
});

describe("shownColumns", () => {
  it("follows t.columns and skips missing columns", () => {
    const t = { ...defaultTable, columns: ["points", "p:gone", "id"] };
    expect(shownColumns(t, all).map((c) => c.key)).toEqual(["points", "id"]);
  });
});

describe("columnChoices", () => {
  it("follows the target order with checked and disabled", () => {
    const t = { ...defaultTable, columns: ["id", "title", "points"] };
    const c = columnChoices(t, all);
    expect(c.map((x) => x.id)).toEqual(all.map((x) => x.key));
    expect(c.find((x) => x.id === "title")).toMatchObject({ checked: true, disabled: true });
    expect(c.find((x) => x.id === "id")?.checked).toBe(true);
    expect(c.find((x) => x.id === "kind")?.checked).toBe(false);
  });
});

describe("selectable", () => {
  it("leaves out deleted rows", () => {
    const ids = selectable([row("a"), row("b", true), row("c")]).map((r) => r.itemId);
    expect(ids).toEqual(["a", "c"]);
  });
});

describe("headerCheck", () => {
  const rows = [row("a"), row("b"), row("x", true)];
  it("is true only when all selectable rows are selected", () => {
    expect(headerCheck(rows, [sel("a"), sel("b")])).toBe(true);
    expect(headerCheck(rows, [sel("a")])).toBe(false);
    expect(headerCheck([row("x", true)], [])).toBe(false);
  });
});
