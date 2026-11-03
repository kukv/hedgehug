import { describe, expect, it } from "vitest";
import type { TreeNode } from "../../core/cache";
import { task } from "./fixtures";
import { orderRootsByStore, storesOf } from "./stores-order";

const tasks = new Map([
  ["B1", task("B1", "project", null, { store: "beta" })],
  ["A1", task("A1", "project", null, { store: "alpha" })],
  ["B2", task("B2", "project", null, { store: "beta" })],
  ["A2", task("A2", "project", null, { store: "alpha" })],
  ["X1", task("X1", "project", null, { store: "gone" })],
]);
const taskOf = (u: string) => tasks.get(u);
const node = (ulid: string): TreeNode => ({ ulid, children: [] });
const ulids = (ns: TreeNode[]) => ns.map((n) => n.ulid);

describe("orderRootsByStore", () => {
  it("orders by the stores and keeps the order within a store", () => {
    const roots = ["B1", "A1", "B2", "A2"].map(node);
    expect(ulids(orderRootsByStore(roots, ["alpha", "beta"], taskOf))).toEqual([
      "A1",
      "A2",
      "B1",
      "B2",
    ]);
    expect(ulids(orderRootsByStore(roots, ["beta", "alpha"], taskOf))).toEqual([
      "B1",
      "B2",
      "A1",
      "A2",
    ]);
  });
  it("puts a root of an unlisted store, or not in the cache, last", () => {
    const roots = ["X1", "N9", "A1", "B1"].map(node);
    expect(ulids(orderRootsByStore(roots, ["alpha", "beta"], taskOf))).toEqual([
      "A1",
      "B1",
      "X1",
      "N9",
    ]);
  });
  it("returns the same array when nothing moves", () => {
    const roots = ["A1", "A2", "B1"].map(node);
    expect(orderRootsByStore(roots, ["alpha", "beta"], taskOf)).toBe(roots);
  });
});

describe("storesOf", () => {
  it("lists each store once, in the order of the roots", () => {
    expect(storesOf(["B1", "A1", "B2", "N9"].map(node), taskOf)).toEqual(["beta", "alpha"]);
  });
});
