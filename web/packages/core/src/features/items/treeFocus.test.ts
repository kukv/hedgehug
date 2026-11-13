import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { item } from "./fixtures.test-util";
import { buildTree } from "./tree";
import { createTreeFocus } from "./treeFocus";

// 木: HH-1(Epic) ─ HH-2(Task)・HH-3(Task) ─ HH-4(Step) / HH-5(Epic) ─ HH-6(Task)
const e1 = item("S1", "P1", 1, "Epic", null);
const t2 = item("S1", "P1", 2, "Task", e1.id);
const t3 = item("S1", "P1", 3, "Task", e1.id);
const s4 = item("S1", "P1", 4, "Step", t3.id);
const e5 = item("S1", "P1", 5, "Epic", null);
const t6 = item("S1", "P1", 6, "Task", e5.id);
const all = [e1, t2, t3, s4, e5, t6];
const ids = (f: ReturnType<typeof createTreeFocus>) => get(f.state).rows.map((r) => r.id);
const at = (f: ReturnType<typeof createTreeFocus>) => get(f.state).currentId;

function made(items = all) {
  const f = createTreeFocus();
  f.setTree(buildTree(items));
  return f;
}

describe("createTreeFocus", () => {
  it("has no current row until the tree is entered, then starts at the first row", () => {
    const f = made();
    expect(at(f)).toBeNull();
    f.enter();
    expect(at(f)).toBe(e1.id);
    expect(ids(f)).toEqual([e1.id, t2.id, t3.id, s4.id, e5.id, t6.id]);
  });

  it("keeps the current row on a re-entry only when asked to (back from a menu)", () => {
    const f = made();
    f.enter();
    f.move("down");
    f.enter(true);
    expect(at(f)).toBe(t2.id);
    f.enter();
    expect(at(f)).toBe(e1.id);
  });

  it("moves up and down, and stops at both ends", () => {
    const f = made();
    f.enter();
    f.move("up");
    expect(at(f)).toBe(e1.id);
    for (let i = 0; i < 9; i++) f.move("down");
    expect(at(f)).toBe(t6.id);
    f.move("up");
    expect(at(f)).toBe(e5.id);
  });

  it("→ opens a closed row, then goes to the first child; does nothing on a leaf", () => {
    const f = made();
    f.enter();
    f.toggle(e1.id);
    expect(ids(f)).toEqual([e1.id, e5.id, t6.id]);
    f.move("right");
    expect(ids(f)).toContain(t2.id);
    expect(at(f)).toBe(e1.id);
    f.move("right");
    expect(at(f)).toBe(t2.id);
    f.move("right");
    expect(at(f)).toBe(t2.id);
  });

  it("← closes an open row, then goes to the parent; does nothing at the top level", () => {
    const f = made();
    f.enter();
    f.focus(s4.id);
    f.move("left");
    expect(at(f)).toBe(t3.id);
    f.move("left");
    expect(ids(f)).not.toContain(s4.id);
    expect(at(f)).toBe(t3.id);
    f.move("left");
    expect(at(f)).toBe(e1.id);
    f.move("left");
    expect(ids(f)).toEqual([e1.id, e5.id, t6.id]);
    f.move("left");
    expect(at(f)).toBe(e1.id);
    expect(ids(f)).toEqual([e1.id, e5.id, t6.id]);
  });

  it("closing a row moves the focus onto it when the focus was below it", () => {
    const f = made();
    f.enter();
    f.focus(s4.id);
    f.toggle(e1.id);
    expect(at(f)).toBe(e1.id);
  });

  it("moves to the next visible row when the current row disappears, else the previous one", () => {
    const f = made();
    f.enter();
    f.focus(t3.id);
    f.setTree(buildTree(all.filter((i) => i.id !== t3.id && i.id !== s4.id)));
    expect(at(f)).toBe(e5.id);
    f.focus(t6.id);
    f.setTree(buildTree([e1, t2, e5]));
    expect(at(f)).toBe(e5.id);
    f.setTree(buildTree([e1, t2]));
    expect(at(f)).toBe(t2.id);
    f.setTree([]);
    expect(at(f)).toBeNull();
  });

  it("keeps the current row and the open state across a reload", () => {
    const f = made();
    f.enter();
    f.toggle(e1.id);
    f.focus(e5.id);
    f.setTree(buildTree([...all]));
    expect(at(f)).toBe(e5.id);
    expect(ids(f)).toEqual([e1.id, e5.id, t6.id]);
  });

  it("ignores a row that is not visible", () => {
    const f = made();
    f.enter();
    f.toggle(e1.id);
    f.focus(t2.id);
    expect(at(f)).toBe(e1.id);
  });

  it("expand opens a closed row and does nothing to an open one", () => {
    const f = made();
    f.enter();
    f.toggle(e5.id);
    f.expand(e5.id);
    expect(ids(f)).toContain(t6.id);
    f.expand(e5.id);
    expect(ids(f)).toContain(t6.id);
  });
});
