import { describe, expect, it } from "vitest";
import { task } from "./fixtures";
import { checkParent, pickKind } from "./placement";

describe("checkParent", () => {
  it("accepts a placement the kinds allow", () => {
    expect(checkParent(task("T1", "task", null), task("E1", "epic", null))).toBeNull();
    expect(checkParent(task("P1", "project", null), null)).toBeNull();
  });

  it("rejects an epic under a task, in the words of the canvas", () => {
    const epic = task("E8", "epic", null, { id: "REL-8", title: "リリース作業" });
    const r = checkParent(epic, task("T7", "task", null));
    expect(r?.title).toBe("Epic は Task の下に置けません");
    expect(r?.reason).toBe(
      "「REL-8 リリース作業」は Epic です。Epic を置けるのは Project の下だけです",
    );
  });

  it("writes the IDs as it is told to", () => {
    const epic = task("E8", "epic", null, { id: "REL-8", title: "リリース作業", store: "a" });
    const r = checkParent(epic, task("T7", "task", null), (t) => `${t.store}:${t.id}`);
    expect(r?.reason).toContain("「a:REL-8 リリース作業」");
  });

  it("rejects a project under an epic, a story at the top and a step under a project", () => {
    expect(checkParent(task("P1", "project", null), task("E1", "epic", null))?.title).toBe(
      "Project は Epic の下に置けません",
    );
    expect(checkParent(task("S1", "story", null), null)?.title).toBe("Story は最上位に置けません");
    expect(checkParent(task("S2", "step", null), task("P1", "project", null))?.title).toBe(
      "Step は Project の下に置けません",
    );
  });

  it("rejects a parent in another store", () => {
    const r = checkParent(task("T1", "task", null), task("T2", "task", null, { store: "backend" }));
    expect(r?.title).toBe("別の Store のタスクの下には置けません");
    expect(r?.reason).toContain("shared");
    expect(r?.reason).toContain("backend");
  });
});

describe("pickKind", () => {
  it("picks the default kind of the parent, and none under a step", () => {
    expect(pickKind(null)).toBe("project");
    expect(pickKind("project")).toBe("task");
    expect(pickKind("epic")).toBe("task");
    expect(pickKind("story")).toBe("step");
    expect(pickKind("bug")).toBe("step");
    expect(pickKind("step")).toBeNull();
  });
});
