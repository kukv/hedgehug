import { describe, expect, it } from "vitest";
import type { TaskView } from "./api/client";
import type { SchemaView } from "./schema";
import { schemaUsage } from "./schema-usage";

const schema = {
  statuses: [{ name: "todo" }, { name: "done" }, { name: "triage" }],
  priorities: ["low", "high"],
  labels: ["bug", "ux"],
  props: {
    tier: { type: "enum", values: ["a", "b"] },
    area: { type: "multi_enum", values: ["be", "fe"] },
    effort: { type: "number" },
    site: { type: "url" },
  },
  workflow: { transitions: {}, required: {} },
  scoped: [],
} as unknown as SchemaView;

const t = (
  status: string,
  kind: string,
  props: Record<string, unknown> = {},
  fields: Record<string, unknown> = {},
) => ({ status, kind, props, ...fields }) as unknown as TaskView;

describe("schemaUsage", () => {
  it("counts the tasks of each status, 0 for one nobody uses", () => {
    const u = schemaUsage(schema, [t("todo", "task"), t("todo", "bug"), t("done", "task")]);
    expect(u.statuses).toEqual({ todo: 2, done: 1, triage: 0 });
  });

  it("counts the tasks that hold a value of a property; an empty value is none", () => {
    const u = schemaUsage(schema, [
      t("todo", "task", { effort: 3, tier: "b" }),
      t("todo", "task", { effort: 0, tier: "", area: [] }),
      t("todo", "task", { effort: null, site: "https://example.com" }),
      t("todo", "task", {}),
    ]);
    expect(u.props).toEqual({ tier: 1, area: 0, effort: 2, site: 1 });
  });

  it("counts the tasks of each value of an enum, and of a multi_enum once for each value it holds", () => {
    const u = schemaUsage(schema, [
      t("todo", "task", { tier: "a", area: ["be", "fe"] }),
      t("todo", "task", { tier: "a", area: ["be"] }),
      t("todo", "task", { tier: "other", area: ["gone"] }),
    ]);
    expect(u.values).toEqual({
      tier: { a: 2, b: 0 },
      area: { be: 2, fe: 1 },
    });
  });

  it("counts the tasks of each priority and of each label, from the fields of the task", () => {
    const u = schemaUsage(schema, [
      t("todo", "task", {}, { priority: "low", labels: ["bug", "ux"] }),
      t("todo", "task", {}, { priority: "low", labels: ["bug", "bug"] }),
      t("todo", "task", {}, { priority: "gone", labels: ["gone"] }),
      t("todo", "task", {}, { priority: null, labels: null }),
    ]);
    expect(u.priorities).toEqual({ low: 2, high: 0 });
    expect(u.labels).toEqual({ bug: 2, ux: 1 });
  });

  it("has no counts for a schema that lists no priorities and no labels", () => {
    const bare = { ...schema, priorities: null, labels: null } as unknown as SchemaView;
    const u = schemaUsage(bare, [t("todo", "task", {}, { priority: "low", labels: ["bug"] })]);
    expect(u.priorities).toEqual({});
    expect(u.labels).toEqual({});
  });

  it("is all zero for no tasks", () => {
    const u = schemaUsage(schema, []);
    expect(u.statuses.todo).toBe(0);
    expect(u.values.tier).toEqual({ a: 0, b: 0 });
    expect(u.priorities).toEqual({ low: 0, high: 0 });
    expect(u.labels).toEqual({ bug: 0, ux: 0 });
  });

  it("counts only the names the schema defines, whatever a task holds (toString is a name, not a method)", () => {
    const u = schemaUsage(schema, [
      t(
        "toString",
        "constructor",
        { tier: "valueOf", area: ["toString"], hasOwnProperty: 1 },
        { priority: "toString", labels: ["valueOf"] },
      ),
    ]);
    expect(u.statuses).toEqual({ todo: 0, done: 0, triage: 0 });
    expect(u.values.tier).toEqual({ a: 0, b: 0 });
    expect(u.values.area).toEqual({ be: 0, fe: 0 });
    expect(u.priorities).toEqual({ low: 0, high: 0 });
    expect(u.labels).toEqual({ bug: 0, ux: 0 });
    const named = {
      ...schema,
      statuses: [{ name: "toString" }],
    } as unknown as SchemaView;
    expect(schemaUsage(named, [t("toString", "task")]).statuses).toEqual({ toString: 1 });
  });
});
