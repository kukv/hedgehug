import { describe, expect, test } from "vitest";
import { projectProgress } from "../items/progress";
import { settings } from "../items/settings.test-util";
import type { TreeNodeOf } from "../items/tree";
import { buildSearchTree, rowKey, type SearchTreeRow, treeScope } from "./searchTree";
import { dataOf, it, match, store } from "./searchTree.test-util";

// flat は、木を上から順に「鍵(薄い行は ~ を付ける)」の並びにする。
function flat(nodes: TreeNodeOf<SearchTreeRow>[]): string[] {
  return nodes.flatMap((n) => [
    n.item.kind === "item" && n.item.dim ? `~${n.item.id}` : n.item.id,
    ...flat(n.children),
  ]);
}

const work = store("work", false);
const team = store("team", true);

describe("buildSearchTree", () => {
  test("unfiltered shows every project in sidebar order", () => {
    const data = dataOf({
      "work/work-P1": [it("work", "work-P1", "a", "Epic", null)],
      "work/work-P2": [],
      "team/team-P1": [it("team", "team-P1", "b", "Epic", null)],
      "team/team-P2": [it("team", "team-P2", "c", "Epic", null)],
    });
    // 空の絞り込みでは、サーバーは削除されていないすべての行を返す。
    const rows = [
      match("work", "work-P1", "a"),
      match("team", "team-P1", "b"),
      match("team", "team-P2", "c"),
    ];
    const v = buildSearchTree([work, team], rows, false, data);
    expect(v.matched).toBe(3);
    expect(v.nodes.map((n) => n.item.id)).toEqual([
      "work/work-P1",
      "work/work-P2",
      "team/team-P1",
      "team/team-P2",
    ]);
    expect(v.sections.map((s) => `${s.store.id}:${s.workspace.id}`)).toEqual([
      "work:work-W1",
      "team:team-W1",
    ]);
    expect(v.sections[0]?.projects[1]?.node.children).toEqual([]);
    expect(flat(v.nodes).some((k) => k.startsWith("~"))).toBe(false);
    expect(v.hasProjects).toBe(true);
  });

  test("filtered keeps matches and their ancestors", () => {
    const items = [
      it("work", "work-P1", "E", "Epic", null, 1),
      it("work", "work-P1", "S", "Story", "E", 2),
      it("work", "work-P1", "T", "Task", "S", 3),
      it("work", "work-P1", "U", "Task", "E", 4),
    ];
    const v = buildSearchTree(
      [work],
      [match("work", "work-P1", "T")],
      true,
      dataOf({ "work/work-P1": items }),
    );
    expect(flat(v.nodes)).toEqual(["work/work-P1", "~work/E", "~work/S", "work/T"]);
    const [p] = v.nodes;
    expect(p?.item.parentId).toBeNull();
    expect(p?.children[0]?.item.parentId).toBe("work/work-P1");
    expect(p?.children[0]?.children[0]?.item.parentId).toBe("work/E");
    expect(v.matched).toBe(1);
  });

  test("filtered hides projects without matches", () => {
    const data = dataOf({
      "work/work-P1": [it("work", "work-P1", "a", "Epic", null)],
      "work/work-P2": [it("work", "work-P2", "b", "Epic", null)],
    });
    const v = buildSearchTree([work], [match("work", "work-P2", "b")], true, data);
    expect(v.nodes.map((n) => n.item.id)).toEqual(["work/work-P2"]);
    expect(v.sections[0]?.projects.map((p) => p.project.id)).toEqual(["work-P2"]);
  });

  test("heading only for workspaces with projects", () => {
    const data = dataOf({ "team/team-P1": [it("team", "team-P1", "b", "Epic", null)] });
    const v = buildSearchTree([work, team], [match("team", "team-P1", "b")], true, data);
    expect(v.sections.map((s) => s.workspace.id)).toEqual(["team-W1"]);
    const empty = buildSearchTree([store("none", false, 0)], [], false, data);
    expect(empty.sections).toEqual([]);
    expect(empty.hasProjects).toBe(false);
  });

  test("ignores sort order", () => {
    const items = [
      it("work", "work-P1", "E", "Epic", null, 1),
      it("work", "work-P1", "A", "Story", "E", 2),
      it("work", "work-P1", "B", "Story", "E", 3),
    ];
    const rows = [match("work", "work-P1", "B"), match("work", "work-P1", "A")];
    const v = buildSearchTree([work], rows, true, dataOf({ "work/work-P1": items }));
    expect(flat(v.nodes)).toEqual(["work/work-P1", "~work/E", "work/A", "work/B"]);
  });

  test("keys are per store", () => {
    const data = dataOf({
      "work/work-P1": [it("work", "work-P1", "x", "Epic", null)],
      "team/team-P1": [it("team", "team-P1", "x", "Epic", null)],
    });
    const both = buildSearchTree([work, team], [], false, data);
    expect(flat(both.nodes)).toEqual(["work/work-P1", "work/x", "team/team-P1", "team/x"]);
    const one = buildSearchTree([work, team], [match("team", "team-P1", "x")], true, data);
    expect(flat(one.nodes)).toEqual(["team/team-P1", "team/x"]);
    expect(rowKey("work", "x")).toBe("work/x");
  });

  test("matched counts only drawn matches", () => {
    const items = [
      it("work", "work-P1", "E", "Epic", null, 1),
      it("work", "work-P1", "S", "Story", "E", 2),
      it("work", "work-P1", "O", "Task", "gone", 3),
    ];
    const rows = [
      match("work", "work-P1", "S"),
      match("work", "work-P1", "O"),
      match("work", "work-P1", "Z"),
    ];
    const v = buildSearchTree([work], rows, true, dataOf({ "work/work-P1": items }));
    expect(flat(v.nodes)).toEqual(["work/work-P1", "~work/E", "work/S"]);
    expect(v.matched).toBe(1);
  });

  test("deleted rows are not matches", () => {
    const items = [it("work", "work-P1", "E", "Epic", null)];
    const v = buildSearchTree(
      [work],
      [match("work", "work-P1", "E", true)],
      true,
      dataOf({ "work/work-P1": items }),
    );
    expect(v.nodes).toEqual([]);
    expect(v.matched).toBe(0);
  });

  test("progress counts unmatched children", () => {
    const items = [
      it("work", "work-P1", "E", "Epic", null, 1),
      it("work", "work-P1", "S", "Story", "E", 2),
      it("work", "work-P1", "T", "Task", "S", 3),
      it("work", "work-P1", "U", "Task", "S", 4),
    ];
    const data = dataOf({ "work/work-P1": items });
    const v = buildSearchTree([work], [match("work", "work-P1", "T")], true, data);
    const p = v.sections[0]?.projects[0];
    expect(p?.data.items).toEqual(items);
    expect(projectProgress(p?.data.items ?? [], settings)).toEqual(
      projectProgress(items, settings),
    );
  });

  test("projects without data are not drawn", () => {
    const data = dataOf({ "work/work-P2": [] });
    const v = buildSearchTree([work], [], false, data);
    expect(v.nodes.map((n) => n.item.id)).toEqual(["work/work-P2"]);
    expect(v.hasProjects).toBe(true);
  });
});

describe("treeScope", () => {
  test("filtered gives only projects with matches", () => {
    const rows = [match("team", "team-P2", "x"), match("work", "work-P1", "y")];
    expect(treeScope([work, team], rows, true)).toEqual([
      { storeId: "work", projectId: "work-P1" },
      { storeId: "team", projectId: "team-P2" },
    ]);
  });

  test("unfiltered gives every project", () => {
    expect(treeScope([work, team], [], false)).toEqual([
      { storeId: "work", projectId: "work-P1" },
      { storeId: "work", projectId: "work-P2" },
      { storeId: "team", projectId: "team-P1" },
      { storeId: "team", projectId: "team-P2" },
    ]);
  });
});
