import { describe, expect, it } from "vitest";
import type { Item } from "../../model/item";
import { rowMenuContext } from "./rowMenu";

function item(id: string, parentId: string | null, rank: string): Item {
  return {
    id,
    storeId: "S1",
    workspaceId: "W1",
    projectId: "P1",
    parentId,
    number: 1,
    kind: parentId ? "Story" : "Epic",
    rank,
    title: id,
    watchers: [],
    blocked: false,
    status: "st1",
    priority: "",
    assignee: "",
    start: "",
    end: "",
    points: null,
    estimate: null,
    systems: [],
    sprint: "",
    release: "",
    choices: {},
    fields: {},
    createdAt: "2026-11-13T09:00:00.000Z",
    createdBy: "taro",
  };
}

describe("rowMenuContext", () => {
  // 兄弟 3 件(順位 b・a・c)と、B の子 1 件。
  const items = [
    item("B", null, "b"),
    item("A", null, "a"),
    item("C", null, "c"),
    item("K", "B", "d"),
  ];

  it("orders the project's items by rank and passes the children as a row menu", () => {
    const got = rowMenuContext(items, "B");
    expect(got?.item.id).toBe("B");
    expect(got?.ctx.rows.map((r) => r.id)).toEqual(["A", "B", "C", "K"]);
    expect(got?.ctx.children.map((r) => r.id)).toEqual(["K"]);
    expect(got?.ctx.row).toBe(true);
  });

  it("keeps a child whose rank sorts between the siblings out of the children of others", () => {
    // 子 K の順位 "ab" は、兄弟 A と B のあいだに並ぶ。上下の移動は同じ親の兄弟だけを見るので、混ざってよい。
    const mixed = [
      item("B", null, "b"),
      item("A", null, "a"),
      item("C", null, "c"),
      item("K", "B", "ab"),
    ];
    const got = rowMenuContext(mixed, "B");
    expect(got?.ctx.rows.map((r) => r.id)).toEqual(["A", "K", "B", "C"]);
    expect(got?.ctx.children.map((r) => r.id)).toEqual(["K"]);
    expect(rowMenuContext(mixed, "A")?.ctx.children).toEqual([]);
  });

  it("returns null for an item not in the list", () => {
    expect(rowMenuContext(items, "X")).toBeNull();
  });
});
