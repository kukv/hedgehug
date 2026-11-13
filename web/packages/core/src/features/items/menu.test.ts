import { describe, expect, it } from "vitest";
import { childKindOf } from "./createDefaults";
import { item } from "./fixtures.test-util";
import { type ItemMenuContext, itemMenu, itemMenuContext, menuReasons } from "./menu";

const ctx = (patch: Partial<ItemMenuContext> = {}): ItemMenuContext => ({
  kind: "Task",
  readOnly: false,
  row: false,
  hasPrev: true,
  hasNext: true,
  hasOtherProjects: true,
  ...patch,
});
const byId = (c: ItemMenuContext) => Object.fromEntries(itemMenu(c).map((e) => [e.id, e]));

describe("itemMenu", () => {
  it("lists the detail-panel items in the spec order with their labels, all enabled", () => {
    const entries = itemMenu(ctx());
    expect(entries.map((e) => [e.id, e.label])).toEqual([
      ["move-up", "上へ移動"],
      ["move-down", "下へ移動"],
      ["move", "移動…"],
      ["change-kind", "種類を変更…"],
      ["duplicate", "複製"],
      ["save-as-template", "テンプレートとして保存…"],
      ["copy-id", "ID をコピー"],
      ["delete", "削除…"],
    ]);
    expect(entries.every((e) => !e.disabled && !("hint" in e))).toBe(true);
  });

  it("starts the tree-row menu with 'open' and 'add a child' (the kind that goes under the row)", () => {
    const head = (kind: ItemMenuContext["kind"]) =>
      itemMenu(ctx({ kind, row: true }))
        .slice(0, 3)
        .map((e) => [e.id, e.label]);
    expect(head("Epic")).toEqual([
      ["open", "詳細を開く"],
      ["add-child", "Story を追加"],
      ["move-up", "上へ移動"],
    ]);
    for (const kind of ["Story", "Task", "Bug"] as const) {
      expect(head(kind)[1]).toEqual(["add-child", "Step を追加"]);
      expect(childKindOf(kind)).toBe("Step");
    }
    expect(itemMenu(ctx({ kind: "Task", row: true })).length).toBe(10);
  });

  it("has no 'add a child' for a Step row, and neither entry in the detail menu", () => {
    const step = itemMenu(ctx({ kind: "Step", row: true })).map((e) => e.id);
    expect(step).toEqual(["open", ...itemMenu(ctx()).map((e) => e.id)]);
    const detail = itemMenu(ctx({ kind: "Epic" })).map((e) => e.id);
    expect(detail).not.toContain("open");
    expect(detail).not.toContain("add-child");
  });

  it("disables 'add a child' in a read-only Store, but keeps 'open' enabled", () => {
    const entries = byId(ctx({ row: true, readOnly: true }));
    expect(entries["add-child"]).toEqual({
      id: "add-child",
      label: "Step を追加",
      disabled: true,
      hint: "閲覧のみの Store です",
    });
    expect(entries.open).toEqual({ id: "open", label: "詳細を開く", disabled: false });
  });

  it("disables the edges with a reason (the first visible sibling cannot go up)", () => {
    const top = byId(ctx({ hasPrev: false }));
    expect(top["move-up"]).toMatchObject({ disabled: true, hint: "先頭です" });
    expect(top["move-down"]?.disabled).toBe(false);
    const bottom = byId(ctx({ hasNext: false }));
    expect(bottom["move-down"]).toMatchObject({ disabled: true, hint: "末尾です" });
    expect(bottom["move-up"]?.disabled).toBe(false);
    expect(menuReasons.atTop).toBe("先頭です");
  });

  it("disables 'change kind' for an Epic, and 'move' for an Epic with no other project", () => {
    const epic = byId(ctx({ kind: "Epic", hasOtherProjects: false }));
    expect(epic["change-kind"]).toMatchObject({
      disabled: true,
      hint: "Epic の種類は変更できません",
    });
    expect(epic.move).toMatchObject({ disabled: true, hint: "移せるプロジェクトがありません" });
    // ほかのプロジェクトがあれば Epic も移せる。Epic 以外は、プロジェクトの数を見ない。
    expect(byId(ctx({ kind: "Epic" })).move?.disabled).toBe(false);
    expect(byId(ctx({ kind: "Story", hasOtherProjects: false })).move?.disabled).toBe(false);
    expect(byId(ctx({ kind: "Step" }))["change-kind"]?.disabled).toBe(false);
  });

  it("disables every write in a read-only Store except copying the ID, and the read-only reason wins", () => {
    const entries = byId(
      ctx({ readOnly: true, hasPrev: false, kind: "Epic", hasOtherProjects: false }),
    );
    for (const id of [
      "move-up",
      "move-down",
      "move",
      "change-kind",
      "duplicate",
      "save-as-template",
      "delete",
    ]) {
      expect(entries[id], id).toMatchObject({ disabled: true, hint: "閲覧のみの Store です" });
    }
    expect(entries["copy-id"]).toEqual({ id: "copy-id", label: "ID をコピー", disabled: false });
  });
});

describe("itemMenuContext", () => {
  const epic = item("S1", "S1-P1", 1, "Epic", null);
  const a = item("S1", "S1-P1", 2, "Task", epic.id);
  const b = item("S1", "S1-P1", 3, "Task", epic.id);

  it("takes the edges from the visible siblings and the project count from the workspace", () => {
    expect(itemMenuContext(a, [epic, a, b], { readOnly: false, projectCount: 2 })).toEqual({
      kind: "Task",
      readOnly: false,
      row: false,
      hasPrev: false,
      hasNext: true,
      hasOtherProjects: true,
    });
    expect(itemMenuContext(b, [a, b], { readOnly: true, projectCount: 1 })).toMatchObject({
      readOnly: true,
      hasPrev: true,
      hasNext: false,
      hasOtherProjects: false,
    });
  });

  it("marks a tree-row menu with row", () => {
    const opts = { readOnly: false, projectCount: 1, row: true };
    expect(itemMenuContext(a, [a], opts).row).toBe(true);
  });
});
