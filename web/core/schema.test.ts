import { describe, expect, it, vi } from "vitest";
import type { Schemas } from "./api/client";
import {
  canParent,
  defaultStatusIn,
  parentRule,
  propEditorOf,
  SchemaCache,
  type SchemaView,
} from "./schema";
import { FakeConn, fakeApi, settle } from "./testing";

// The schema of a new store (internal/app/domain/schema Default), as GET /api/schema
// tells it.
const kinds = [
  { name: "project", label: "プロジェクト", role: "container", parents: ["root"] },
  { name: "epic", label: "エピック", role: "container", parents: ["root", "project"] },
  { name: "story", label: "ストーリー", role: "work", parents: ["root", "project", "epic"] },
  {
    name: "task",
    label: "タスク",
    role: "work",
    parents: ["root", "project", "epic", "story", "task"],
    default: true,
  },
  { name: "bug", label: "バグ", role: "work", parents: ["root", "project", "epic", "story"] },
  { name: "note", label: "メモ", role: "work", parents: null },
  { name: "nowhere", role: "work", parents: [] },
] as Schemas["KindDef"][];

const schema: SchemaView = {
  statuses: [
    { name: "backlog", label: "バックログ", category: "todo" },
    { name: "todo", label: "未着手", category: "todo", default: true },
    { name: "in_progress", label: "進行中", category: "doing", default: true },
    { name: "done", label: "完了", category: "done", default: true },
    { name: "canceled", label: "中止", category: "canceled", default: true },
  ],
  kinds,
  props: {
    priority: { type: "enum", values: ["low", "mid", "high", "urgent"] },
    multi: { type: "multi_enum", values: ["a", "b"] },
    start: { type: "date" },
    points: { type: "number" },
    owner: { type: "user" },
    relates: { type: "task_ref", multi: true, label: "関連", symmetric: true },
    estimate: { type: "duration" },
    done_flag: { type: "bool" },
    site: { type: "url" },
    memo: { type: "text" },
    at: { type: "datetime" },
  },
  workflow: { transitions: {}, required: {} },
  scoped: [],
};

describe("canParent", () => {
  it("allows the parents the kind lists, and the top as root", () => {
    expect(canParent(schema, "epic", null)).toBe(true);
    expect(canParent(schema, "epic", "project")).toBe(true);
    expect(canParent(schema, "epic", "task")).toBe(false);
    expect(canParent(schema, "task", "task")).toBe(true);
    expect(canParent(schema, "project", "project")).toBe(false);
    expect(canParent(schema, "project", null)).toBe(true);
  });

  it("allows anywhere when the kind lists no parents, and nowhere for an empty list", () => {
    expect(canParent(schema, "note", "bug")).toBe(true);
    expect(canParent(schema, "note", null)).toBe(true);
    expect(canParent(schema, "nowhere", null)).toBe(false);
  });

  it("refuses a kind the schema does not have", () => {
    expect(canParent(schema, "unknown", null)).toBe(false);
  });
});

describe("parentRule", () => {
  it("names the kinds that can be the parent", () => {
    expect(parentRule(schema, "epic")).toBe("エピックを置ける親はルートとプロジェクトだけです");
    expect(parentRule(schema, "story")).toBe(
      "ストーリーを置ける親はルート、プロジェクトとエピックだけです",
    );
  });

  it("has its own words for the top only, anywhere and nowhere", () => {
    expect(parentRule(schema, "project")).toBe("プロジェクトはルートにだけ置けます");
    expect(parentRule(schema, "note")).toBe("メモはどこにでも置けます");
    expect(parentRule(schema, "nowhere")).toBe("nowhereは親に置けません");
  });
});

describe("defaultStatusIn: todo and done", () => {
  it("is the default of the category", () => {
    expect(defaultStatusIn(schema, "todo")).toBe("todo");
    expect(defaultStatusIn(schema, "done")).toBe("done");
  });

  it("falls back to the first of the category, and to empty", () => {
    const s: SchemaView = {
      ...schema,
      statuses: [
        { name: "a", category: "todo" },
        { name: "b", category: "todo" },
      ],
    };
    expect(defaultStatusIn(s, "todo")).toBe("a");
    expect(defaultStatusIn(s, "done")).toBe("");
  });
});

describe("defaultStatusIn", () => {
  const s: SchemaView = {
    ...schema,
    statuses: [
      { name: "todo", category: "todo", default: true },
      { name: "wip", category: "doing" },
      { name: "review", category: "doing", default: true },
      { name: "done", category: "done", default: true },
    ],
  };

  it("is the default of any category, else its first status", () => {
    expect(defaultStatusIn(s, "doing")).toBe("review");
    expect(defaultStatusIn(s, "todo")).toBe("todo");
  });

  it("is empty for a category the schema has no status in", () => {
    expect(defaultStatusIn(s, "canceled")).toBe("");
  });
});

describe("propEditorOf", () => {
  it("maps the types of the schema", () => {
    expect(propEditorOf(schema, "priority")).toBe("enum");
    expect(propEditorOf(schema, "multi")).toBe("multi_enum");
    expect(propEditorOf(schema, "start")).toBe("date");
    expect(propEditorOf(schema, "points")).toBe("number");
    expect(propEditorOf(schema, "owner")).toBe("user");
    expect(propEditorOf(schema, "relates")).toBe("taskref");
    expect(propEditorOf(schema, "estimate")).toBe("duration");
    expect(propEditorOf(schema, "done_flag")).toBe("bool");
    expect(propEditorOf(schema, "site")).toBe("url");
    expect(propEditorOf(schema, "memo")).toBe("text");
    expect(propEditorOf(schema, "at")).toBe("datetime");
  });

  it("knows the fields every task has", () => {
    expect(propEditorOf(schema, "status")).toBe("status");
    expect(propEditorOf(schema, "kind")).toBe("kind");
    expect(propEditorOf(schema, "assignees")).toBe("user");
    expect(propEditorOf(schema, "watchers")).toBe("user");
    expect(propEditorOf(schema, "blocked_by")).toBe("taskref");
    expect(propEditorOf(schema, "title")).toBe("text");
  });

  it("is null for what the schema does not define", () => {
    expect(propEditorOf(schema, "legacy")).toBeNull();
  });
});

describe("SchemaCache", () => {
  function setup() {
    let current = schema;
    const { api, calls } = fakeApi((r) => (r.path === "/api/schema" ? current : {}));
    const conn = new FakeConn();
    const cache = new SchemaCache({ api, conn: conn.asConnection() });
    return { cache, conn, calls, set: (s: SchemaView) => (current = s) };
  }

  it("loads a store once and keeps it", async () => {
    const { cache, calls } = setup();
    expect(cache.get("shared")).toBeUndefined();
    const [a, b] = await Promise.all([cache.load("shared"), cache.load("shared")]);
    expect(a).toEqual(schema);
    expect(b).toBe(a);
    expect(cache.get("shared")).toBe(a);
    await cache.load("shared");
    expect(calls).toHaveLength(1);
    expect(calls[0]?.query.get("store")).toBe("shared");
  });

  it("fetches again at schema.changed and tells the subscribers", async () => {
    const { cache, conn, set } = setup();
    const first = await cache.load("shared");
    const fn = vi.fn();
    cache.subscribe(fn);

    set({ ...schema, kinds: kinds.slice(0, 2) });
    conn.emit("schema.changed", { store: "shared" });
    await vi.waitFor(() => expect(cache.get("shared")?.kinds?.length).toBe(2));
    expect(cache.get("shared")).not.toBe(first);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("reload fetches a held store at once, and leaves one nobody asked for", async () => {
    const { cache, calls, set } = setup();
    await cache.reload("shared");
    expect(calls).toHaveLength(0);
    await cache.load("shared");
    set({ ...schema, kinds: kinds.slice(0, 2) });
    await cache.reload("shared");
    expect(cache.get("shared")?.kinds?.length).toBe(2);
    expect(calls).toHaveLength(2);
  });

  it("keeps the same object when the schema did not change", async () => {
    const { cache, conn, calls } = setup();
    const first = await cache.load("shared");
    const fn = vi.fn();
    cache.subscribe(fn);
    conn.emit("resync");
    await vi.waitFor(() => expect(calls).toHaveLength(2));
    await settle();
    expect(cache.get("shared")).toBe(first);
    expect(fn).not.toHaveBeenCalled();
  });

  it("does not let an older fetch that ends last overwrite the newer schema", async () => {
    let n = 0;
    let release: (s: SchemaView) => void = () => undefined;
    const newer = { ...schema, kinds: kinds.slice(0, 2) };
    const { api } = fakeApi(() => {
      n++;
      if (n === 1) return Promise.resolve(schema);
      if (n === 2) return new Promise<SchemaView>((r) => (release = r));
      return newer;
    });
    const conn = new FakeConn();
    const cache = new SchemaCache({ api, conn: conn.asConnection() });
    await cache.load("shared");
    conn.emit("resync"); // fetch 2, held
    await settle();
    conn.emit("schema.changed", { store: "shared" }); // fetch 3, quick
    await vi.waitFor(() => expect(cache.get("shared")?.kinds?.length).toBe(2));
    release(schema);
    await settle();
    expect(cache.get("shared")?.kinds?.length).toBe(2);
  });

  it("leaves alone a store nobody asked for at schema.changed", async () => {
    const { cache, conn, calls } = setup();
    conn.emit("schema.changed", { store: "other" });
    await settle();
    expect(cache.get("other")).toBeUndefined();
    expect(calls).toHaveLength(0);
  });
});
