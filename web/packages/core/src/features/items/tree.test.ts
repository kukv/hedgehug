import { describe, expect, it } from "vitest";
import type { Item, Kind } from "../../model/item";
import { ancestorsOf, buildTree, compareSiblings, type TreeNode } from "./tree";

function item(
  id: string,
  kind: Kind,
  parentId: string | null,
  rank: string,
  createdAt = "2026-11-13T09:00:00.000Z",
): Item {
  return {
    id,
    storeId: "S1",
    workspaceId: "W1",
    projectId: "P1",
    parentId,
    number: 1,
    kind,
    rank,
    title: id,
    watchers: ["taro"],
    createdAt,
    createdBy: "taro",
  };
}

const ids = (nodes: TreeNode[]): unknown[] =>
  nodes.map((n) => (n.children.length ? [n.item.id, ids(n.children)] : n.item.id));

describe("compareSiblings", () => {
  it("orders by rank, then by the time of creation, then by ID", () => {
    const list = [
      item("C", "Task", "E", "W"),
      item("B", "Task", "E", "V", "2026-11-13T09:00:01.000Z"),
      item("A2", "Task", "E", "V"),
      item("A1", "Task", "E", "V"),
    ];
    expect([...list].sort(compareSiblings).map((x) => x.id)).toEqual(["A1", "A2", "B", "C"]);
  });

  it("compares the times as times, not as text", () => {
    const a = item("A", "Task", "E", "V", "2026-11-13T18:00:00.000+09:00");
    const b = item("B", "Task", "E", "V", "2026-11-13T10:00:00.000Z");
    expect(compareSiblings(a, b)).toBeLessThan(0);
  });
});

describe("buildTree", () => {
  it("nests epics, their children and steps in sibling order", () => {
    const tree = buildTree([
      item("E2", "Epic", null, "W"),
      item("E1", "Epic", null, "V"),
      item("T2", "Task", "E1", "W"),
      item("T1", "Task", "E1", "V"),
      item("S1", "Step", "T1", "V"),
      item("X", "Task", "gone", "V"),
    ]);
    expect(ids(tree)).toEqual([["E1", [["T1", ["S1"]], "T2"]], "E2"]);
  });
});

describe("ancestorsOf", () => {
  it("returns the ancestors from the root", () => {
    const items = [
      item("E", "Epic", null, "V"),
      item("T", "Task", "E", "V"),
      item("S", "Step", "T", "V"),
    ];
    expect(ancestorsOf(items, "S").map((x) => x.id)).toEqual(["E", "T"]);
    expect(ancestorsOf(items, "E")).toEqual([]);
    expect(ancestorsOf(items, "nope")).toEqual([]);
  });

  it("stops at a loop", () => {
    const items = [item("A", "Task", "B", "V"), item("B", "Epic", "A", "V")];
    expect(ancestorsOf(items, "A").map((x) => x.id)).toEqual(["B"]);
  });
});
