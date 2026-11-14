import { describe, expect, it } from "vitest";
import type { InboxTarget, InboxTargetKind } from "../../model/inbox";
import { row } from "./inbox.test-util";
import { goneTextOf, projectRouteOf } from "./target";

const withTarget = (kind: InboxTargetKind, display: string) => {
  const r = row(1, "assign");
  const target: InboxTarget = { ...r.target, kind, display, project: "P1" };
  return { ...r, target };
};

describe("行の対象のプロジェクト (06「並びと行の表示」)", () => {
  it("opens the overview of the project for project and moved_project", () => {
    for (const kind of ["project", "moved_project"] as const) {
      expect(projectRouteOf(withTarget(kind, "HH-P1"))).toEqual({
        name: "project",
        store: "team",
        project: "HH-P1",
        tab: "overview",
      });
    }
  });

  it("has no route for deleted_project and for the targets that are not projects", () => {
    expect(projectRouteOf(withTarget("deleted_project", ""))).toBeNull();
    expect(projectRouteOf(withTarget("item", "HH-1"))).toBeNull();
  });

  it("shows 削除されたプロジェクト for deleted_project", () => {
    expect(goneTextOf("deleted_project")).toBe("削除されたプロジェクト");
    expect(goneTextOf("deleted_item")).toBe("削除されたアイテム");
    expect(goneTextOf("deleted_workspace")).toBe("削除されたワークスペース");
    expect(goneTextOf("project")).toBeNull();
  });
});
