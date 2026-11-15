import { describe, expect, test } from "vitest";
import type { Item } from "../../model/item";
import { settings } from "../items/settings.test-util";
import { visibleRows } from "../items/siblings";
import { buildSearchTree, type ProjectData, screenFor } from "./searchTree";
import { it, match, store } from "./searchTree.test-util";

// 検索のツリーの書き込みの画面(screenFor)。フォーカスのある行の Store・ワークスペースで決める。

const work = store("work", false);
const team = store("team", true);
const teamSettings = { ...settings, initialStatus: "team-st" };

function dataOf(table: Record<string, Item[]>) {
  return (storeId: string, projectId: string): ProjectData | undefined => {
    const items = table[`${storeId}/${projectId}`];
    if (!items) return undefined;
    return { items, settings: storeId === "team" ? teamSettings : settings, members: [] };
  };
}

const data = dataOf({
  "work/work-P1": [it("work", "work-P1", "x", "Epic", null)],
  "work/work-P2": [it("work", "work-P2", "y", "Epic", null)],
  "team/team-P1": [it("team", "team-P1", "x", "Epic", null)],
  "team/team-P2": [],
});
const rows = visibleRows(buildSearchTree([work, team], [], false, data).nodes, new Set());

describe("screenFor", () => {
  test("screenFor uses the row's store", () => {
    const ro = screenFor(rows, "team/x", data);
    expect(ro?.storeId).toBe("team");
    expect(ro?.readOnly).toBe(true);
    expect(ro?.settings).toBe(teamSettings);
    const rw = screenFor(rows, "work/x", data);
    expect(rw?.storeId).toBe("work");
    expect(rw?.readOnly).toBe(false);
    expect(rw?.settings).toBe(settings);
    expect(screenFor(rows, "team/team-P2", data)?.readOnly).toBe(true);
  });

  test("unknown key gives null", () => {
    expect(screenFor(rows, "work/none", data)).toBeNull();
  });

  test("order includes project rows of the same workspace only", () => {
    const s = screenFor(rows, "work/y", data);
    expect(s?.order).toEqual([
      { id: "work-P1", parentId: null },
      { id: "x", parentId: null },
      { id: "work-P2", parentId: null },
      { id: "y", parentId: null },
    ]);
    expect(s?.rows.map((r) => `${r.storeId}/${r.id}`)).toEqual(["work/x", "work/y"]);
  });

  test("rowsFor gives only visible siblings", () => {
    const sib = (id: string, n: number) => it("work", "work-P1", id, "Story", "E", n);
    const items = [
      it("work", "work-P1", "E", "Epic", null, 1),
      sib("HH-3", 3),
      sib("HH-4", 4),
      sib("HH-5", 5),
      sib("HH-7", 7),
      sib("HH-9", 9),
    ];
    const d = dataOf({ "work/work-P1": items });
    const hits = ["HH-3", "HH-5", "HH-9"].map((id) => match("work", "work-P1", id));
    const view = buildSearchTree([work], hits, true, d);
    const s = screenFor(visibleRows(view.nodes, new Set()), "work/HH-5", d);
    expect(s?.rows.map((r) => r.id)).toEqual(["E", "HH-3", "HH-5", "HH-9"]);
  });
});
