import { describe, expect, it } from "vitest";
import type { MoveCandidate } from "../../model/itemAction";
import { item } from "./fixtures.test-util";
import { settings } from "./settings.test-util";
import { doneToggleTarget, reparentTarget } from "./treeMove";

// 04 の例の木: HH-2(Epic) ─ HH-3(Task)・HH-4(Task) ─ HH-5(Step) / HH-6(Epic) ─ HH-7(Task) / HH-8(Epic)
const e2 = item("S1", "P1", 2, "Epic", null);
const t3 = item("S1", "P1", 3, "Task", e2.id);
const t4 = item("S1", "P1", 4, "Task", e2.id);
const s5 = item("S1", "P1", 5, "Step", t4.id);
const e6 = item("S1", "P1", 6, "Epic", null);
const t7 = item("S1", "P1", 7, "Task", e6.id);
const e8 = item("S1", "P1", 8, "Epic", null);
const rows = [e2, t3, t4, s5, e6, t7, e8];

const cand = (it: { id: string; number: number }): MoveCandidate => ({
  kind: "item",
  id: it.id,
  display: `HH-${it.number}`,
  title: `アイテム ${it.number}`,
  projectId: "P1",
  projectTitle: "ログイン",
});

describe("reparentTarget", () => {
  // Task の移し先は Epic。いまの親(HH-2)と、Epic でないものは候補に入っていない。
  const forTask = [cand(e6), cand(e8)];

  it("takes the first candidate among the visible rows below, as in 04's example", () => {
    expect(reparentTarget(rows, t3.id, "down", forTask)?.display).toBe("HH-6");
    expect(reparentTarget(rows, t7.id, "down", [cand(e2), cand(e8)])?.display).toBe("HH-8");
  });

  it("takes the first candidate among the visible rows above when going up", () => {
    expect(reparentTarget(rows, t7.id, "up", [cand(e2)])?.display).toBe("HH-2");
    expect(reparentTarget(rows, e8.id, "up", [cand(e2), cand(e6)])?.display).toBe("HH-6");
  });

  it("does nothing when no candidate is in the visible rows or none is in that direction", () => {
    expect(reparentTarget(rows, t3.id, "up", forTask)).toBeNull();
    expect(reparentTarget(rows, t7.id, "down", [cand(e6)])).toBeNull();
    const without = rows.filter((r) => r.id !== e6.id);
    expect(reparentTarget(without, t3.id, "down", [cand(e6)])).toBeNull();
    expect(reparentTarget(rows, t3.id, "down", [])).toBeNull();
  });

  it("skips the rows of the item's own subtree and the project candidates", () => {
    expect(reparentTarget(rows, t4.id, "down", [cand(s5), cand(e6)])?.display).toBe("HH-6");
    const project: MoveCandidate = { ...cand(e6), kind: "project" };
    expect(reparentTarget(rows, t3.id, "down", [project])).toBeNull();
  });

  it("returns null for a row that is not visible", () => {
    expect(reparentTarget(rows, "nope", "down", forTask)).toBeNull();
  });
});

describe("doneToggleTarget", () => {
  it("goes to the default done status unless the category is done, then to the default todo status", () => {
    expect(doneToggleTarget(settings, "st1")).toBe("st3");
    expect(doneToggleTarget(settings, "st2")).toBe("st3");
    expect(doneToggleTarget(settings, "st4")).toBe("st3");
    expect(doneToggleTarget(settings, "st3")).toBe("st1");
  });

  it("treats a status that is not in the list as not done, and has no target without a default", () => {
    expect(doneToggleTarget(settings, "gone")).toBe("st3");
    expect(doneToggleTarget({ ...settings, categoryDefaults: {} }, "st1")).toBeNull();
  });
});
