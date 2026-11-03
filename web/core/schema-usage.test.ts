import { describe, expect, it } from "vitest";
import type { TaskView } from "./api/client";
import type { SchemaView } from "./schema";
import { schemaUsage } from "./schema-usage";

const schema = {
  statuses: [{ name: "todo" }, { name: "done" }, { name: "triage" }],
  kinds: [{ name: "task" }, { name: "bug" }],
  props: {
    priority: { type: "enum", values: ["low", "high"] },
    area: { type: "multi_enum", values: ["be", "fe"] },
    points: { type: "number" },
    due: { type: "date" },
  },
  workflow: { transitions: {}, required: {} },
  scoped: [],
} as unknown as SchemaView;

const t = (status: string, kind: string, props: Record<string, unknown> = {}) =>
  ({ status, kind, props }) as unknown as TaskView;

describe("schemaUsage", () => {
  it("counts the tasks of each status and kind, 0 for one nobody uses", () => {
    const u = schemaUsage(schema, [t("todo", "task"), t("todo", "bug"), t("done", "task")]);
    expect(u.statuses).toEqual({ todo: 2, done: 1, triage: 0 });
    expect(u.kinds).toEqual({ task: 2, bug: 1 });
  });

  it("counts the tasks that hold a value of a property; an empty value is none", () => {
    const u = schemaUsage(schema, [
      t("todo", "task", { points: 3, priority: "high" }),
      t("todo", "task", { points: 0, priority: "", area: [] }),
      t("todo", "task", { points: null, due: "2026-11-01" }),
      t("todo", "task", {}),
    ]);
    expect(u.props).toEqual({ priority: 1, area: 0, points: 2, due: 1 });
  });

  it("counts the tasks of each value of an enum, and of a multi_enum once for each value it holds", () => {
    const u = schemaUsage(schema, [
      t("todo", "task", { priority: "low", area: ["be", "fe"] }),
      t("todo", "task", { priority: "low", area: ["be"] }),
      t("todo", "task", { priority: "other", area: ["gone"] }),
    ]);
    expect(u.values).toEqual({
      priority: { low: 2, high: 0 },
      area: { be: 2, fe: 1 },
    });
  });

  it("is all zero for no tasks", () => {
    const u = schemaUsage(schema, []);
    expect(u.statuses.todo).toBe(0);
    expect(u.values.priority).toEqual({ low: 0, high: 0 });
  });

  it("counts only the names the schema defines, whatever a task holds (toString is a name, not a method)", () => {
    const u = schemaUsage(schema, [
      t("toString", "constructor", { priority: "valueOf", area: ["toString"], hasOwnProperty: 1 }),
    ]);
    expect(u.statuses).toEqual({ todo: 0, done: 0, triage: 0 });
    expect(u.kinds).toEqual({ task: 0, bug: 0 });
    expect(u.values.priority).toEqual({ low: 0, high: 0 });
    expect(u.values.area).toEqual({ be: 0, fe: 0 });
    const named = {
      ...schema,
      statuses: [{ name: "toString" }],
    } as unknown as SchemaView;
    expect(schemaUsage(named, [t("toString", "task")]).statuses).toEqual({ toString: 1 });
  });
});
