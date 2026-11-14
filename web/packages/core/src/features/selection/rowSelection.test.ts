import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import {
  allReadOnly,
  createRowSelection,
  isSelected,
  type SelectedRow,
  selectedText,
} from "./rowSelection";

const row = (storeId: string, itemId: string, display = itemId, readOnly = false): SelectedRow => ({
  storeId,
  itemId,
  display,
  readOnly,
});

describe("rowSelection", () => {
  it("toggles rows and clears", () => {
    const sel = createRowSelection();
    const a = row("s1", "a");
    const b = row("s1", "b");
    sel.toggle(a, true);
    sel.toggle(b, true);
    expect(get(sel.rows)).toHaveLength(2);
    sel.toggle(a, false);
    expect(get(sel.rows).map((r) => r.itemId)).toEqual(["b"]);
    sel.clear();
    expect(get(sel.rows)).toEqual([]);
    sel.toggle(a, true);
    sel.toggle(a, true);
    expect(get(sel.rows)).toHaveLength(1);
  });

  it("selection keeps rows of different stores apart", () => {
    const sel = createRowSelection();
    sel.toggle(row("s1", "x"), true);
    sel.toggle(row("s2", "x"), true);
    expect(get(sel.rows)).toHaveLength(2);
    expect(isSelected(get(sel.rows), "s1", "x")).toBe(true);
    sel.toggle(row("s1", "x"), false);
    expect(isSelected(get(sel.rows), "s1", "x")).toBe(false);
    expect(isSelected(get(sel.rows), "s2", "x")).toBe(true);
  });

  it("prune keeps visible rows in screen order", () => {
    const sel = createRowSelection();
    const [a, b, c] = [row("s1", "a"), row("s1", "b"), row("s1", "c")];
    sel.toggle(b, true);
    sel.toggle(a, true);
    sel.prune([a, c, b]);
    expect(get(sel.rows).map((r) => r.itemId)).toEqual(["a", "b"]);
    sel.prune([c]);
    expect(get(sel.rows)).toEqual([]);
  });

  it("prune refreshes display and readOnly", () => {
    const sel = createRowSelection();
    sel.toggle(row("s1", "a", "HH-1"), true);
    sel.prune([row("s1", "a", "HH-9", true)]);
    expect(get(sel.rows)).toEqual([row("s1", "a", "HH-9", true)]);
  });

  it("texts", () => {
    expect(selectedText(3)).toBe("3 件を選択中");
    expect(allReadOnly([])).toBe(false);
    expect(allReadOnly([row("s", "a", "a", true), row("s", "b", "b", true)])).toBe(true);
    expect(allReadOnly([row("s", "a", "a", true), row("s", "b")])).toBe(false);
  });
});
