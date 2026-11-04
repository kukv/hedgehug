import { describe, expect, it } from "vitest";
import { taskView, workspaces } from "../detail/testkit";
import { resolveCreateContext, workspaceKey } from "./context";

const tasks = [
  taskView({ ulid: "UE", id: "REL-8", title: "リリース作業", kind: "epic" }),
  taskView({ ulid: "UP", id: "REL-1", title: "リリース準備", kind: "project" }),
  taskView({ ulid: "UT", id: "REL-6", title: "詳細で開いている", kind: "task" }),
  taskView({
    ulid: "UD",
    id: "REL-7",
    title: "削除済み",
    kind: "task",
    deleted_at: "2026-11-01T00:00:00Z",
  }),
  taskView({
    ulid: "US",
    id: "SH-3",
    title: "別のワークスペース",
    workspace: { ulid: "W2", name: "shared", id_prefix: "SH" },
  }),
];
const task = (u: string) => tasks.find((t) => t.ulid === u);
const base = { selection: null, route: { name: "home" } as never, task, workspaces };

describe("resolveCreateContext", () => {
  it("uses the selected row of the tree as the parent", () => {
    const c = resolveCreateContext({
      ...base,
      selection: "UE",
      route: { name: "project", ulid: "UP", tab: "tree" },
    });
    expect(c.parent?.id).toBe("REL-8");
    expect(c.workspace?.name).toBe("release");
  });

  it("uses the project when nothing is selected", () => {
    const c = resolveCreateContext({
      ...base,
      route: { name: "project", ulid: "UP", tab: "tree" },
    });
    expect(c.parent?.id).toBe("REL-1");
  });

  it("uses the task on a task detail", () => {
    const c = resolveCreateContext({
      ...base,
      selection: "UE",
      route: { name: "task", ulid: "UT" },
    });
    expect(c.parent?.id).toBe("REL-6");
  });

  it("does not fall back to a row selected earlier when the task of the detail is deleted or not read yet", () => {
    for (const ulid of ["UD", "UNKNOWN"]) {
      const c = resolveCreateContext({ ...base, selection: "UE", route: { name: "task", ulid } });
      expect(c.parent).toBeNull();
    }
  });

  it("puts the task under the parent a request names, before anything else", () => {
    const c = resolveCreateContext({
      ...base,
      requestParent: "US",
      selection: "UE",
      route: { name: "project", ulid: "UP", tab: "tree" },
    });
    expect(c.parent?.id).toBe("SH-3");
    expect(c.workspace?.name).toBe("shared");
  });

  it("skips a deleted or unknown task", () => {
    const c = resolveCreateContext({
      ...base,
      selection: "UD",
      route: { name: "workspace", store: "shared", ulid: "W2", tab: "tree" },
    });
    expect(c.parent).toBeNull();
    expect(c.workspace?.name).toBe("shared");
  });

  it("is the root of the workspace on a workspace screen, and of the first workspace elsewhere", () => {
    expect(
      resolveCreateContext({
        ...base,
        route: { name: "workspace", store: "shared", ulid: "W2", tab: "tree" },
      }).workspace?.ulid,
    ).toBe("W2");
    expect(resolveCreateContext(base)).toEqual({ parent: null, workspace: workspaces[0] });
    expect(resolveCreateContext({ ...base, workspaces: [] })).toEqual({
      parent: null,
      workspace: null,
    });
  });

  it("starts from the first writable workspace when the place it would start from is read-only", () => {
    const readOnly = (store: string) => (store === "shared" ? "x" : null);
    const other = { ...(workspaces[0] as (typeof workspaces)[number]), store: "demo", ulid: "W9" };
    const list = [...workspaces, other];
    expect(resolveCreateContext({ ...base, workspaces: list, readOnly }).workspace).toBe(other);
    expect(
      resolveCreateContext({
        ...base,
        workspaces: list,
        readOnly,
        route: { name: "workspace", store: "shared", ulid: "W2", tab: "tree" },
      }).workspace,
    ).toBe(other);
    // Nothing can be written: it stays where it would have started, and the dialog says so.
    expect(resolveCreateContext({ ...base, readOnly: () => "x" }).workspace).toBe(workspaces[0]);
    // The workspace of a parent is the parent's, written or not.
    expect(
      resolveCreateContext({ ...base, workspaces: list, readOnly, requestParent: "US" }).workspace
        ?.name,
    ).toBe("shared");
  });

  it("keys a workspace by store and ULID", () => {
    expect(workspaceKey(workspaces[0] as (typeof workspaces)[number])).toBe("shared/W1");
  });
});
