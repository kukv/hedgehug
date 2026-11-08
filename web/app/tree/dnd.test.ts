import { describe, expect, it } from "vitest";
import { dropDecision, placeOf } from "./dnd";
import { schema, task } from "./fixtures";

// P (project)
//  ├ E1 (epic) ─ T1, T2
//  ├ T7 (task) ─ T8
//  └ E8 (epic, REL-8 リリース作業)
const all = [
  task("P", "project", null),
  task("E1", "epic", "P"),
  task("T1", "task", "E1"),
  task("T2", "task", "E1"),
  task("T7", "task", "P"),
  task("T8", "task", "T7"),
  task("E8", "epic", "P", { id: "REL-8", title: "リリース作業" }),
];
const map = new Map(all.map((t) => [t.ulid, t]));
const taskOf = (u: string) => map.get(u);
const at = (u: string) => map.get(u) as NonNullable<ReturnType<typeof taskOf>>;

function decide(dragged: string, target: string, place: "before" | "after" | "child") {
  return dropDecision({
    dragged: at(dragged),
    target: at(target),
    place,
    schema,
    taskOf,
  });
}

describe("a Store that cannot be written", () => {
  it("names the Store that refuses, whichever side it is", () => {
    const d = dropDecision({
      dragged: at("T7"),
      target: at("T1"),
      place: "after",
      schema,
      taskOf,
      readOnly: { store: "dragged-store", why: "remote に書き込めません" },
    });
    expect(d).toMatchObject({ ok: false, silent: false });
    if (!d.ok) expect(d.reason).toBe("dragged-store は閲覧のみです（remote に書き込めません）。");
  });
});

describe("the IDs in a reason", () => {
  it("are written as the screen is told to", () => {
    const d = dropDecision({
      dragged: { ...at("T7"), store: "a" },
      target: { ...at("T1"), store: "b" },
      place: "after",
      schema,
      taskOf,
      idOf: (t) => `${t.store}:${t.id}`,
    });
    expect(d.ok ? "" : d.reason).toMatch(/「a:REL-7 .*」は a、「b:REL-1 .*」は b/);
  });
});

describe("placeOf", () => {
  it("maps the hitbox instruction to a place", () => {
    expect(placeOf({ type: "reorder-above" } as never)).toBe("before");
    expect(placeOf({ type: "reorder-below" } as never)).toBe("after");
    expect(placeOf({ type: "make-child" } as never)).toBe("child");
    expect(placeOf({ type: "instruction-blocked" } as never)).toBeNull();
    expect(placeOf(null)).toBeNull();
  });
});

describe("dropDecision: where the task goes", () => {
  it("makes a child at the end of the target", () => {
    expect(decide("T7", "E1", "child")).toEqual({ ok: true, to: { parent: "E1" } });
  });
  it("puts it before or after the target, under the target's parent", () => {
    expect(decide("E8", "E1", "before")).toEqual({ ok: true, to: { parent: "P", before: "E1" } });
    expect(decide("E8", "E1", "after")).toEqual({ ok: true, to: { parent: "P", after: "E1" } });
  });
  it("puts it at the top level next to a root", () => {
    const root = task("R1", "task", null);
    expect(
      dropDecision({
        dragged: at("T1"),
        target: root,
        place: "after",
        schema,
        taskOf,
      }),
    ).toEqual({ ok: true, to: { root: true, after: "R1" } });
  });
  it("moves among the siblings of another parent", () => {
    expect(decide("T8", "T1", "before")).toEqual({
      ok: true,
      to: { parent: "E1", before: "T1" },
    });
  });
});

describe("dropDecision: what it refuses", () => {
  it("refuses an epic under a task, in the words of the canvas", () => {
    expect(decide("E8", "T7", "child")).toEqual({
      ok: false,
      silent: false,
      title: "エピックはタスクの下に置けません",
      reason:
        "「REL-8 リリース作業」はエピックです。エピックのどの下に置けるかは最上位とプロジェクトだけです（shared の schema.yaml の kinds）。",
    });
  });
  it("refuses a place whose parent is a task when the kind needs another parent", () => {
    // before T8: the parent would be T7, a task
    expect(decide("E8", "T8", "before")).toMatchObject({
      ok: false,
      title: "エピックはタスクの下に置けません",
    });
  });
  it("refuses another store", () => {
    const other = task("X1", "task", null, { store: "backend" });
    expect(
      dropDecision({
        dragged: at("T7"),
        target: other,
        place: "child",
        schema,
        taskOf,
      }),
    ).toMatchObject({ ok: false, silent: false, title: "別の Store のタスクの下には置けません" });
    expect(
      dropDecision({
        dragged: at("T7"),
        target: other,
        place: "after",
        schema,
        taskOf,
      }),
    ).toMatchObject({ ok: false, title: "別の Store の並びには置けません" });
  });
  it("refuses another workspace", () => {
    const other = task("W2", "task", "P", {
      workspace: { ulid: "W9", name: "other", id_prefix: "OTH" },
    });
    expect(
      dropDecision({
        dragged: at("T7"),
        target: other,
        place: "child",
        schema,
        taskOf,
      }),
    ).toMatchObject({ ok: false, silent: false, title: "別のワークスペースには置けません" });
  });
  it("refuses its own descendants as a parent", () => {
    expect(decide("E1", "T1", "child")).toMatchObject({
      ok: false,
      silent: false,
      title: "自分の配下には置けません",
    });
    expect(decide("E1", "T1", "before")).toMatchObject({
      ok: false,
      title: "自分の配下には置けません",
    });
    expect(decide("T7", "T8", "child")).toMatchObject({
      ok: false,
      title: "自分の配下には置けません",
    });
  });
  it("refuses while the schema is not loaded", () => {
    expect(
      dropDecision({
        dragged: at("T7"),
        target: at("E1"),
        place: "child",
        schema: undefined,
        taskOf,
      }),
    ).toMatchObject({ ok: false, silent: false, title: "少し待ってからやり直してください" });
  });
  it("refuses when the new parent is not loaded", () => {
    const orphan = task("O1", "task", "MISSING");
    expect(
      dropDecision({
        dragged: at("T7"),
        target: orphan,
        place: "after",
        schema,
        taskOf,
      }),
    ).toMatchObject({ ok: false, title: "移動先の親を読み込めていません" });
  });
});

describe("dropDecision: what it ignores", () => {
  it("ignores a drop on itself", () => {
    expect(decide("T7", "T7", "child")).toMatchObject({ ok: false, silent: true });
    expect(decide("T7", "T7", "before")).toMatchObject({ ok: false, silent: true });
  });
  it("leaves a move to where the task already is to the server", () => {
    // The screen may list only some of the siblings, so it cannot tell.
    expect(decide("T7", "E1", "after")).toEqual({ ok: true, to: { parent: "P", after: "E1" } });
    expect(decide("T7", "E8", "before")).toEqual({ ok: true, to: { parent: "P", before: "E8" } });
    expect(decide("T8", "T7", "child")).toEqual({ ok: true, to: { parent: "T7" } });
  });
});
