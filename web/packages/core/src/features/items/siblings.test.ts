import { describe, expect, it } from "vitest";
import { item } from "./fixtures.test-util";
import { visibleNeighbors, visibleRows } from "./siblings";
import { buildTree } from "./tree";

// 04 の例: HH-5・HH-3・HH-4・HH-7・HH-9 の順に並ぶ兄弟(HH-4 は絞り込みで見えない)。
const epic = item("S1", "S1-P1", 1, "Epic", null);
const other = item("S1", "S1-P1", 2, "Epic", null);
const t5 = item("S1", "S1-P1", 5, "Task", epic.id);
const s6 = item("S1", "S1-P1", 6, "Step", t5.id);
const t3 = item("S1", "S1-P1", 3, "Task", epic.id);
const t7 = item("S1", "S1-P1", 7, "Task", epic.id);
const elsewhere = item("S1", "S1-P2", 8, "Epic", null);

describe("visibleNeighbors", () => {
  it("returns the visible sibling before and after, skipping other rows in between", () => {
    const rows = [epic, t5, s6, t3, t7, other];
    expect(visibleNeighbors(rows, t3.id)).toEqual({ prev: t5, next: t7 });
    expect(visibleNeighbors(rows, t5.id)).toEqual({ next: t3 });
    expect(visibleNeighbors(rows, t7.id)).toEqual({ prev: t3 });
  });

  it("does not look at siblings that are not visible (a filtered-out row is not a neighbor)", () => {
    // HH-4 は見えていない。HH-5 の次は HH-3、HH-3 の次は HH-7 になる。
    expect(visibleNeighbors([t5, t3, t7], t3.id)).toEqual({ prev: t5, next: t7 });
  });

  it("treats the first and last visible sibling as the edges", () => {
    expect(visibleNeighbors([t5, s6], s6.id)).toEqual({});
    expect(visibleNeighbors([t5], t5.id)).toEqual({});
  });

  it("compares Epics inside one project only", () => {
    expect(visibleNeighbors([epic, other, elsewhere], other.id)).toEqual({ prev: epic });
    expect(visibleNeighbors([epic, other, elsewhere], elsewhere.id)).toEqual({});
  });

  it("returns nothing for an item that is not in the rows", () => {
    expect(visibleNeighbors([epic], t5.id)).toEqual({});
  });
});

describe("visibleRows", () => {
  // 同じ並び順の値なら、作成の順(HH-3 が HH-5 より先)に並ぶ。
  const tree = buildTree([epic, t5, s6, t3, other]);
  const ids = (rows: { id: string }[]) => rows.map((r) => r.id);

  it("lists the rows top to bottom, with the children of an open row", () => {
    expect(ids(visibleRows(tree, new Set()))).toEqual(ids([epic, t3, t5, s6, other]));
  });

  it("leaves out what is under a closed row", () => {
    expect(ids(visibleRows(tree, new Set([t5.id])))).toEqual(ids([epic, t3, t5, other]));
    expect(ids(visibleRows(tree, new Set([epic.id])))).toEqual(ids([epic, other]));
  });
});
