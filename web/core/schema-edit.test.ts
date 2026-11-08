import { describe, expect, it } from "vitest";
import type { SchemaView } from "./schema";
import {
  colorToken,
  kindColorOf,
  kindIconOf,
  kindRoleOf,
  orphanedBy,
  propNames,
  referencesOf,
  statusColorOf,
} from "./schema-edit";

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
  it("names a prop once when prop_order repeats it", () => {
    const sc = { ...schema, prop_order: ["b", "a", "b", "gone"] } as unknown as SchemaView;
    expect(propNames(sc)).toEqual(["b", "a"]);
  });
  it("finds the color of a status and the icon and role of a kind", () => {
    const sc = {
      statuses: [{ name: "todo", category: "todo", color: "teal" }, { name: "done" }],
      kinds: [
        { name: "epic", role: "container", icon: "epic", color: "teal" },
        { name: "task", role: "work" },
      ],
    } as unknown as SchemaView;
    expect(statusColorOf(sc, "todo")).toBe("teal");
    expect(statusColorOf(sc, "done")).toBeUndefined();
    expect(statusColorOf(sc, "nope")).toBeUndefined();
    expect(statusColorOf(undefined, "todo")).toBeUndefined();
    expect(kindIconOf(sc, "epic")).toBe("epic");
    expect(kindIconOf(sc, "task")).toBeUndefined();
    expect(kindColorOf(sc, "epic")).toBe("teal");
    expect(kindColorOf(sc, "task")).toBeUndefined();
    expect(kindColorOf(undefined, "epic")).toBeUndefined();
    expect(kindRoleOf(sc, "epic")).toBe("container");
    expect(kindRoleOf(sc, "nope")).toBeUndefined();
    expect(kindRoleOf(undefined, "epic")).toBeUndefined();
  });
  it("falls back to the name order without prop_order", () => {
    expect(propNames({ props: { b: {}, a: {} } } as unknown as SchemaView)).toEqual(["a", "b"]);
  });
  it("draws a palette name with its token and keeps a plain color", () => {
    expect(colorToken("teal")).toBe("var(--store-teal-dot)");
    expect(colorToken("#3451D1")).toBe("#3451D1");
    expect(colorToken("url(x)")).toBeUndefined();
    expect(colorToken(undefined)).toBeUndefined();
  });
  it("lists the references a removal takes out", () => {
    expect(referencesOf(schema, "status", "on_hold")).toEqual([
      { text: "ワークフロー: todo から移せる先" },
      { text: "ワークフロー: on_hold からの遷移の制約" },
      { text: "ワークフロー: on_hold に入るときの必須の項目" },
      { text: "上書き: 表示するステータス", project: "01J" },
    ]);
    expect(referencesOf(schema, "kind", "epic")).toEqual([
      { text: "種類 sub のどの下に置けるか" },
      { text: "種類 task のどの下に置けるか" },
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
