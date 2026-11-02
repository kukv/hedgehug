import { describe, expect, it } from "vitest";
import type { TreeNode } from "../../core/cache";
import { task } from "./fixtures";
import { countLabel, countMatched, flatten, viewRoots } from "./flatten";

const tasks = new Map([
  ["P", task("P", "project", null)],
  ["E1", task("E1", "epic", "P")],
  ["T1", task("T1", "task", "E1")],
  ["T2", task("T2", "task", "E1")],
  ["T3", task("T3", "task", "P")],
]);
const taskOf = (u: string) => tasks.get(u);
const tree: TreeNode[] = [
  {
    ulid: "P",
    context: true,
    children: [
      {
        ulid: "E1",
        children: [
          { ulid: "T1", children: [] },
          { ulid: "T2", context: true, children: [] },
        ],
      },
      { ulid: "T3", children: [] },
    ],
  },
];

describe("viewRoots", () => {
  it("returns the roots as they are for a workspace", () => {
    expect(viewRoots(tree, null)).toBe(tree);
  });
  it("returns the children of the project, which is itself left out", () => {
    expect(viewRoots(tree, "P").map((n) => n.ulid)).toEqual(["E1", "T3"]);
  });
  it("returns nothing when the project is not in the tree", () => {
    expect(viewRoots(tree, "ZZ")).toEqual([]);
  });
});

describe("flatten", () => {
  it("lists the expanded rows with depth and parent", () => {
    const rows = flatten(tree, "P", taskOf, new Set());
    expect(rows.map((r) => [r.ulid, r.depth, r.parent, r.expanded])).toEqual([
      ["E1", 0, "P", true],
      ["T1", 1, "E1", false],
      ["T2", 1, "E1", false],
      ["T3", 0, "P", false],
    ]);
  });
  it("leaves out the children of a collapsed row", () => {
    const rows = flatten(tree, "P", taskOf, new Set(["E1"]));
    expect(rows.map((r) => r.ulid)).toEqual(["E1", "T3"]);
    expect(rows[0]).toMatchObject({ hasChildren: true, expanded: false, childCount: 2 });
  });
  it("uses a null parent at the top of a workspace", () => {
    const rows = flatten([{ ulid: "T3", children: [] }], null, taskOf, new Set());
    expect(rows[0]?.parent).toBeNull();
  });
});

describe("countMatched / countLabel", () => {
  it("counts the matched tasks, including collapsed ones, and not the context rows", () => {
    expect(countMatched(tree, "P", taskOf)).toBe(3); // E1, T1 and T3; T2 is context
  });
  it("writes the count and the canceled tasks left out", () => {
    expect(countLabel(16, null)).toBe("16 件");
    expect(countLabel(16, 1)).toBe("16 件 · 中止 1 件を除外");
  });
});
