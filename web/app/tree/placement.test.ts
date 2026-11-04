import { describe, expect, it } from "vitest";
import type { SchemaView } from "../../core/schema";
import { schema, task } from "./fixtures";
import { checkParent, kindLabel, pickKind } from "./placement";

describe("checkParent", () => {
  it("accepts a placement the kinds allow", () => {
    expect(checkParent(schema, task("T1", "task", null), task("E1", "epic", null))).toBeNull();
    expect(checkParent(schema, task("E1", "epic", null), null)).toBeNull();
  });

  it("rejects an epic under a task, in the words of the canvas", () => {
    const epic = task("E8", "epic", null, { id: "REL-8", title: "リリース作業" });
    const r = checkParent(schema, epic, task("T7", "task", null));
    expect(r?.title).toBe("エピックはタスクの下に置けません");
    expect(r?.reason).toBe(
      "「REL-8 リリース作業」はエピックです。エピックを置ける親はルートとプロジェクトだけです（shared の schema.yaml の kinds）。",
    );
  });

  it("writes the IDs as it is told to", () => {
    const epic = task("E8", "epic", null, { id: "REL-8", title: "リリース作業", store: "a" });
    const r = checkParent(schema, epic, task("T7", "task", null), (t) => `${t.store}:${t.id}`);
    expect(r?.reason).toContain("「a:REL-8 リリース作業」");
  });

  it("rejects a project under an epic and a story at the root where it cannot go", () => {
    expect(checkParent(schema, task("P1", "project", null), task("E1", "epic", null))?.title).toBe(
      "プロジェクトはエピックの下に置けません",
    );
    const narrow = {
      ...schema,
      kinds: schema.kinds?.map((k) => (k.name === "story" ? { ...k, parents: ["epic"] } : k)),
    } as SchemaView;
    expect(checkParent(narrow, task("S1", "story", null), null)?.title).toBe(
      "ストーリーはルートに置けません",
    );
  });

  it("rejects a parent in another store", () => {
    const r = checkParent(
      schema,
      task("T1", "task", null),
      task("T2", "task", null, { store: "backend" }),
    );
    expect(r?.title).toBe("別の Store のタスクの下には置けません");
    expect(r?.reason).toContain("shared");
    expect(r?.reason).toContain("backend");
  });
});

describe("kindLabel / pickKind", () => {
  it("falls back to the name when the schema has no label", () => {
    expect(kindLabel(schema, "epic")).toBe("エピック");
    expect(kindLabel(schema, "unknown")).toBe("unknown");
  });

  it("picks the default kind when it fits the parent, else the first work kind that fits", () => {
    expect(pickKind(schema, "epic")).toBe("task");
    expect(pickKind(schema, null)).toBe("task");
    const noTask = {
      ...schema,
      kinds: schema.kinds?.map((k) => (k.name === "task" ? { ...k, parents: ["project"] } : k)),
    } as SchemaView;
    expect(pickKind(noTask, "epic")).toBe("story");
    expect(pickKind(noTask, "task")).toBeNull();
  });
});
