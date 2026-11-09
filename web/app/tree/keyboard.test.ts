import { describe, expect, it } from "vitest";
import type { TaskView } from "../../core/api/client";
import { schema, task } from "./fixtures";
import { flatten } from "./flatten";
import { type KeyContext, type KeyInput, keyAction, needsSchema, writeTargetOf } from "./keyboard";

// P (view root)
//  ├ E1 (epic)
//  │   ├ T1
//  │   └ T2
//  ├ T3
//  └ S1 (story, collapsed: child T4)
const tasks = new Map([
  ["P", task("P", "project", null)],
  ["E1", task("E1", "epic", "P")],
  ["T1", task("T1", "task", "E1")],
  ["T2", task("T2", "task", "E1", { status: "done" })],
  ["T3", task("T3", "task", "P")],
  ["S1", task("S1", "story", "P")],
  ["T4", task("T4", "step", "S1")],
]);
const tree = [
  {
    ulid: "P",
    children: [
      {
        ulid: "E1",
        children: [
          { ulid: "T1", children: [] },
          { ulid: "T2", children: [] },
        ],
      },
      { ulid: "T3", children: [] },
      { ulid: "S1", children: [{ ulid: "T4", children: [] }] },
    ],
  },
];
const taskOf = (u: string) => tasks.get(u);

function ctx(selected: string | null, over: Partial<KeyContext> = {}): KeyContext {
  return {
    rows: flatten(tree, "P", taskOf, new Set(["S1"])),
    selected,
    schema,
    viewRoot: "P",
    taskOf,
    ...over,
  };
}
const key = (k: string, m: Partial<KeyInput> = {}): KeyInput => ({
  key: k,
  shift: false,
  alt: false,
  mod: false,
  ...m,
});

describe("which keys need the schema", () => {
  it("is true for the keys that act through the statuses", () => {
    for (const k of [" ", "s", "S", "k", "K"]) {
      expect(needsSchema(key(k))).toBe(true);
    }
  });
  it("is false for moving, folding, adding, reordering and indenting, and for a key with a modifier", () => {
    for (const k of ["ArrowDown", "ArrowLeft", "Enter", "a", "Tab"]) {
      expect(needsSchema(key(k))).toBe(false);
    }
    expect(needsSchema(key("ArrowUp", { alt: true }))).toBe(false);
    expect(needsSchema(key("Tab", { alt: true }))).toBe(false);
    expect(needsSchema(key("s", { mod: true }))).toBe(false);
  });
});

describe("moving the selection", () => {
  it("selects the next and previous visible row", () => {
    expect(keyAction(key("ArrowDown"), ctx("E1"))).toEqual({ type: "select", ulid: "T1" });
    expect(keyAction(key("ArrowUp"), ctx("T1"))).toEqual({ type: "select", ulid: "E1" });
  });
  it("stays on the first and last row", () => {
    expect(keyAction(key("ArrowUp"), ctx("E1"))).toEqual({ type: "select", ulid: "E1" });
    expect(keyAction(key("ArrowDown"), ctx("S1"))).toEqual({ type: "select", ulid: "S1" });
  });
  it("starts at the first row going down and at the last going up when nothing is selected", () => {
    expect(keyAction(key("ArrowDown"), ctx(null))).toEqual({ type: "select", ulid: "E1" });
    expect(keyAction(key("ArrowUp"), ctx(null))).toEqual({ type: "select", ulid: "S1" });
  });
  it("does nothing on an empty tree", () => {
    expect(keyAction(key("ArrowDown"), ctx(null, { rows: [] }))).toEqual({ type: "none" });
  });
});

describe("expanding and collapsing", () => {
  it("collapses an expanded row with ← and goes to the parent from a leaf", () => {
    expect(keyAction(key("ArrowLeft"), ctx("E1"))).toEqual({
      type: "toggle",
      ulid: "E1",
      open: false,
    });
    expect(keyAction(key("ArrowLeft"), ctx("T1"))).toEqual({ type: "select", ulid: "E1" });
  });
  it("does not go above the view root", () => {
    expect(keyAction(key("ArrowLeft"), ctx("T3"))).toEqual({ type: "none" });
  });
  it("expands a collapsed row with → and goes to the first child of an expanded one", () => {
    expect(keyAction(key("ArrowRight"), ctx("S1"))).toEqual({
      type: "toggle",
      ulid: "S1",
      open: true,
    });
    expect(keyAction(key("ArrowRight"), ctx("E1"))).toEqual({ type: "select", ulid: "T1" });
    expect(keyAction(key("ArrowRight"), ctx("T3"))).toEqual({ type: "none" });
  });
});

describe("Enter", () => {
  it("opens a row to add below the selected one, under the same parent", () => {
    expect(keyAction(key("Enter"), ctx("T1"))).toEqual({
      type: "add-below",
      after: "T1",
      parent: "E1",
    });
    expect(keyAction(key("Enter"), ctx("E1"))).toEqual({
      type: "add-below",
      after: "E1",
      parent: "P",
    });
  });
  it("opens the detail with the modifier", () => {
    expect(keyAction(key("Enter", { mod: true }), ctx("T1"))).toEqual({
      type: "open-detail",
      ulid: "T1",
    });
  });
  it("does nothing with no selection or with Shift / Alt", () => {
    expect(keyAction(key("Enter"), ctx(null))).toEqual({ type: "none" });
    expect(keyAction(key("Enter", { shift: true }), ctx("T1"))).toEqual({ type: "none" });
  });
});

describe("Tab / Shift+Tab", () => {
  it("indents under the previous sibling, at the end", () => {
    expect(keyAction(key("Tab"), ctx("T3"))).toEqual({
      type: "move",
      ulid: "T3",
      to: { parent: "E1" },
    });
  });
  it("rejects a task under a task", () => {
    expect(keyAction(key("Tab"), ctx("T2"))).toMatchObject({
      type: "reject",
      rejection: { title: "Task は Task の下に置けません" },
    });
  });
  it("leaves Tab alone on the first sibling", () => {
    expect(keyAction(key("Tab"), ctx("E1"))).toEqual({ type: "none" });
    expect(keyAction(key("Tab"), ctx("T1"))).toEqual({ type: "none" });
  });
  it("rejects an indent the kinds do not allow", () => {
    // the previous sibling of S1 (a story) is T3 (a task)
    const r = keyAction(key("Tab"), ctx("S1"));
    expect(r).toMatchObject({
      type: "reject",
      rejection: { title: "Story は Task の下に置けません" },
    });
  });
  it("outdents after the parent", () => {
    expect(keyAction(key("Tab", { shift: true }), ctx("T1"))).toEqual({
      type: "move",
      ulid: "T1",
      to: { parent: "P", after: "E1" },
    });
  });
  it("rejects an outdent to the root of a workspace view: only a project goes there", () => {
    // seen from a workspace the epic is a root: it has no parent
    const topTask = (u: string) =>
      u === "E1" ? { ...(tasks.get("E1") as TaskView), parent: null } : tasks.get(u);
    const rows = flatten(
      tree.flatMap((n) => n.children),
      null,
      topTask,
      new Set(["S1"]),
    );
    const c = ctx("T1", { rows, viewRoot: null, taskOf: topTask });
    expect(keyAction(key("Tab", { shift: true }), c)).toMatchObject({
      type: "reject",
      rejection: { title: "Task は最上位に置けません" },
    });
  });
  it("does not outdent a top row of the view", () => {
    expect(keyAction(key("Tab", { shift: true }), ctx("T3"))).toEqual({ type: "none" });
  });
  it("rejects an outdent the kinds do not allow", () => {
    // a step under a story cannot go to the project
    const rows = flatten(tree, "P", taskOf, new Set());
    expect(keyAction(key("Tab", { shift: true }), ctx("T4", { rows }))).toMatchObject({
      type: "reject",
      rejection: { title: "Step は Project の下に置けません" },
    });
  });
});

describe("Alt+↑↓", () => {
  it("moves before the previous sibling and after the next one", () => {
    expect(keyAction(key("ArrowUp", { alt: true }), ctx("T3"))).toEqual({
      type: "move",
      ulid: "T3",
      to: { before: "E1" },
    });
    expect(keyAction(key("ArrowDown", { alt: true }), ctx("E1"))).toEqual({
      type: "move",
      ulid: "E1",
      to: { after: "T3" },
    });
  });
  it("stays among its own siblings", () => {
    expect(keyAction(key("ArrowUp", { alt: true }), ctx("T1"))).toEqual({ type: "none" });
    expect(keyAction(key("ArrowDown", { alt: true }), ctx("T2"))).toEqual({ type: "none" });
  });
});

describe("Space / S / K", () => {
  it("completes a task, and reopens a done one", () => {
    expect(keyAction(key(" "), ctx("T1"))).toEqual({
      type: "set-status",
      ulid: "T1",
      status: "done",
    });
    expect(keyAction(key(" "), ctx("T2"))).toEqual({
      type: "set-status",
      ulid: "T2",
      status: "backlog",
    });
  });
  it("says why when the schema has no status of the category to go to", () => {
    const noDone = {
      ...schema,
      statuses: schema.statuses?.filter((s) => s.category !== "done"),
    } as typeof schema;
    expect(keyAction(key(" "), ctx("T1", { schema: noDone }))).toMatchObject({
      type: "reject",
      rejection: { title: "完了にできません" },
    });
    const noTodo = {
      ...schema,
      statuses: schema.statuses?.filter((s) => s.category !== "todo"),
    } as typeof schema;
    expect(keyAction(key(" "), ctx("T2", { schema: noTodo }))).toMatchObject({
      type: "reject",
      rejection: { title: "未着手に戻せません" },
    });
  });
  it("needs the schema", () => {
    expect(keyAction(key(" "), ctx("T1", { schema: undefined }))).toEqual({ type: "none" });
  });
  it("opens the status and kind menus", () => {
    expect(keyAction(key("s"), ctx("T1"))).toEqual({ type: "menu", which: "status", ulid: "T1" });
    expect(keyAction(key("k"), ctx("T1"))).toEqual({ type: "menu", which: "kind", ulid: "T1" });
    expect(keyAction(key("S"), ctx("T1"))).toEqual({ type: "menu", which: "status", ulid: "T1" });
  });
  it("ignores them with a modifier or no selection", () => {
    expect(keyAction(key("s", { mod: true }), ctx("T1"))).toEqual({ type: "none" });
    expect(keyAction(key("s"), ctx(null))).toEqual({ type: "none" });
  });
});

describe("other keys", () => {
  it("does nothing", () => {
    expect(keyAction(key("a"), ctx("T1"))).toEqual({ type: "none" });
    expect(keyAction(key("Escape"), ctx("T1"))).toEqual({ type: "none" });
  });
});

describe("a view over several stores", () => {
  // The top rows of two stores sit under the same null parent.
  const two = new Map([
    ["A1", task("A1", "project", null, { store: "alpha" })],
    ["A2", task("A2", "project", null, { store: "alpha" })],
    ["B1", task("B1", "project", null, { store: "beta" })],
    ["B2", task("B2", "project", null, { store: "beta" })],
  ]);
  const twoOf = (u: string) => two.get(u);
  const roots = ["A1", "A2", "B1", "B2"].map((ulid) => ({ ulid, children: [] }));
  const multi = (selected: string): KeyContext => ({
    rows: flatten(roots, null, twoOf, new Set()),
    selected,
    schema,
    viewRoot: null,
    taskOf: twoOf,
  });

  it("reorders within a store and stops at the edge of it", () => {
    expect(keyAction(key("ArrowUp", { alt: true }), multi("A2"))).toEqual({
      type: "move",
      ulid: "A2",
      to: { before: "A1" },
    });
    expect(keyAction(key("ArrowDown", { alt: true }), multi("A2"))).toEqual({ type: "none" });
    expect(keyAction(key("ArrowUp", { alt: true }), multi("B1"))).toEqual({ type: "none" });
  });
  it("does not indent the first top row of a store under the last one of another", () => {
    expect(keyAction(key("Tab"), multi("B1"))).toEqual({ type: "none" });
    expect(keyAction(key("Tab"), multi("B2"))).toMatchObject({ type: "reject" });
  });
});

describe("the task a write action acts on", () => {
  it("is the task of the actions that write, and the selection for a refusal", () => {
    expect(writeTargetOf({ type: "add-below", after: "A", parent: null }, "S")).toBe("A");
    expect(writeTargetOf({ type: "move", ulid: "M", to: { root: true } }, "S")).toBe("M");
    expect(writeTargetOf({ type: "set-status", ulid: "X", status: "done" }, "S")).toBe("X");
    expect(writeTargetOf({ type: "menu", which: "kind", ulid: "K" }, "S")).toBe("K");
    expect(writeTargetOf({ type: "reject", rejection: { title: "t", reason: "r" } }, "S")).toBe(
      "S",
    );
  });

  it("is null for the actions that only move the view", () => {
    expect(writeTargetOf({ type: "none" }, "S")).toBeNull();
    expect(writeTargetOf({ type: "select", ulid: "A" }, "S")).toBeNull();
    expect(writeTargetOf({ type: "toggle", ulid: "A", open: true }, "S")).toBeNull();
    expect(writeTargetOf({ type: "open-detail", ulid: "A" }, "S")).toBeNull();
  });
});
