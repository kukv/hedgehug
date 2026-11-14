import { describe, expect, it } from "vitest";
import type { InboxTarget, InboxTargetKind } from "../../model/inbox";
import { row } from "./inbox.test-util";
import { goneTextOf, projectRouteOf, workspaceRouteOf } from "./target";

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

describe("Store を指す行 (06「並びと行の表示」)", () => {
  it("is neither gone nor a project, so the row shows the Store name without a link", () => {
    const r = withTarget("store", "");
    expect(r.target.kind).toBe("store");
    expect(goneTextOf("store")).toBeNull();
    expect(projectRouteOf(r)).toBeNull();
  });
});

describe("ワークスペースを指す行の開く区画 (06「並びと行の表示」)", () => {
  it("opens the Sprints and Releases tabs without a section", () => {
    expect(workspaceRouteOf("sprints")).toEqual({ tab: "sprints" });
    expect(workspaceRouteOf("releases")).toEqual({ tab: "releases" });
  });

  it("opens the settings sub-tab, automation when the section is empty", () => {
    expect(workspaceRouteOf("statuses")).toEqual({ tab: "settings", section: "statuses" });
    expect(workspaceRouteOf("")).toEqual({ tab: "settings", section: "automation" });
  });
});
