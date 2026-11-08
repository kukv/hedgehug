import { describe, expect, it } from "vitest";
import type { SchemaView } from "./schema";
import { orphanedBy, propNames, referencesOf, statusColor } from "./schema-edit";

const schema = {
  statuses: [
    { name: "todo", category: "todo", default: true },
    { name: "on_hold", category: "todo" },
  ],
  kinds: [
    { name: "epic", role: "container", parents: ["root"] },
    { name: "sub", role: "work", parents: ["epic"] },
    { name: "task", role: "work", parents: ["root", "epic"], default: true },
  ],
  props: { b: { type: "number" }, a: { type: "text" } },
  prop_order: ["b", "a"],
  workflow: { transitions: { todo: ["on_hold"], on_hold: ["todo"] }, required: { on_hold: ["b"] } },
  scoped: [
    {
      where: "project:01J",
      statuses: ["todo", "on_hold"],
      templates: { default: { epic: "plan" } },
    },
  ],
} as unknown as SchemaView;

describe("schema-edit", () => {
  it("orders the props as the file does", () => {
    expect(propNames(schema)).toEqual(["b", "a"]);
  });
  it("falls back to the name order without prop_order", () => {
    expect(propNames({ props: { b: {}, a: {} } } as unknown as SchemaView)).toEqual(["a", "b"]);
  });
  it("draws a palette name with its token and keeps a plain color", () => {
    expect(statusColor("teal")).toBe("var(--store-teal-dot)");
    expect(statusColor("#3451D1")).toBe("#3451D1");
    expect(statusColor("url(x)")).toBeUndefined();
    expect(statusColor(undefined)).toBeUndefined();
  });
  it("lists the references a removal takes out", () => {
    expect(referencesOf(schema, "status", "on_hold")).toEqual([
      { text: "ワークフロー: todo から移せる先" },
      { text: "ワークフロー: on_hold からの遷移の制約" },
      { text: "ワークフロー: on_hold に入るときの必須の項目" },
      { text: "上書き: 表示するステータス", project: "01J" },
    ]);
    expect(referencesOf(schema, "kind", "epic")).toEqual([
      { text: "種類 sub の置ける親" },
      { text: "種類 task の置ける親" },
      { text: "上書き: 既定のテンプレート", project: "01J" },
    ]);
    expect(referencesOf(schema, "prop", "b")).toEqual([
      { text: "ワークフロー: on_hold に入るときの必須の項目" },
    ]);
  });
  it("finds the kinds a removal leaves with no parent", () => {
    expect(orphanedBy(schema, "epic")).toEqual(["sub"]);
  });
});
