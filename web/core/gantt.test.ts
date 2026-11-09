import { describe, expect, it } from "vitest";
import type { TaskView } from "./api/client";
import type { TreeNode } from "./cache";
import {
  chartRange,
  dateOf,
  dayOf,
  dragged,
  dropPatch,
  type GanttRow,
  ganttLinks,
  ganttRows,
  ganttWarnings,
  type RowsInput,
  spanOf,
  ticks,
} from "./gantt";
import { tv } from "./testing";
import { DEFAULT_GANTT } from "./views";

const d = (date: string): number => dayOf(date) as number;

const tasks = (...ts: TaskView[]) => new Map(ts.map((t) => [t.ulid, t]));

function rowsOf(
  roots: TreeNode[],
  all: Map<string, TaskView>,
  over: Partial<RowsInput> = {},
): GanttRow[] {
  return ganttRows({
    roots,
    taskOf: (u) => all.get(u),
    settings: DEFAULT_GANTT,
    collapsed: new Set(),
    categoryOf: () => "todo",
    sprintSpan: () => null,
    ...over,
  });
}

describe("days", () => {
  it("round-trips, and reads nothing else as a date", () => {
    expect(dateOf(d("2026-11-03"))).toBe("2026-11-03");
    expect(d("2026-11-04") - d("2026-11-03")).toBe(1);
    expect(dayOf("11/03")).toBeNull();
    expect(dayOf(undefined)).toBeNull();
  });

  it.each([
    ["both", "2026-11-03", "2026-11-05", { s: "2026-11-03", e: "2026-11-05" }],
    ["only the start", "2026-11-03", undefined, { s: "2026-11-03", e: "2026-11-03" }],
    ["only the end", undefined, "2026-11-05", { s: "2026-11-05", e: "2026-11-05" }],
    ["an end before the start", "2026-11-05", "2026-11-03", { s: "2026-11-05", e: "2026-11-05" }],
  ])("%s", (_, start, end, want) => {
    expect(spanOf(start, end)).toEqual({ s: d(want.s), e: d(want.e) });
  });

  it("has no span without a date", () => {
    expect(spanOf(undefined, "")).toBeNull();
  });
});

describe("ganttRows", () => {
  const epic = tv("E", "epic", { kind: "epic", due: "2026-11-05" });
  const child = tv("C", "child", { start: "2026-11-02", due: "2026-11-04" });
  const grand = tv("G", "grand", { start: "2026-11-03", due: "2026-11-09" });
  const tree: TreeNode[] = [
    {
      ulid: "E",
      children: [{ ulid: "C", children: [{ ulid: "G", children: [] }] }],
    },
  ];

  it("adds up a parent from everything under it, and says when it runs out of its own dates", () => {
    const [e, c, g] = rowsOf(tree, tasks(epic, child, grand));
    expect(e).toMatchObject({
      kind: "rollup",
      bar: { s: d("2026-11-03"), e: d("2026-11-09") },
      over: true,
      depth: 0,
    });
    expect(c).toMatchObject({ kind: "rollup", over: true, depth: 1 });
    expect(g).toMatchObject({ kind: "work", over: false, depth: 2 });
  });

  it("keeps adding up what a collapsed row hides", () => {
    const rows = rowsOf(tree, tasks(epic, child, grand), { collapsed: new Set(["E"]) });
    expect(rows.map((r) => r.ulid)).toEqual(["E"]);
    expect(rows[0]).toMatchObject({ expanded: false, hasChildren: true });
    expect(rows[0]?.bar?.e).toBe(d("2026-11-09"));
  });

  it("a roadmap shows the containers and counts the work under them", () => {
    const rows = rowsOf(tree, tasks(epic, child, grand), {
      settings: { ...DEFAULT_GANTT, roadmap: true },
    });
    expect(rows.map((r) => r.ulid)).toEqual(["E"]);
    expect(rows[0]).toMatchObject({ hasChildren: false, bar: { e: d("2026-11-09") } });
  });

  it("draws a task without dates over its sprint, and none without a sprint", () => {
    const rows = rowsOf(
      [
        { ulid: "P", children: [] },
        { ulid: "S", children: [] },
      ],
      tasks(tv("P", "plain"), tv("S", "sprinted")),
      { sprintSpan: (t) => (t.ulid === "S" ? { s: 1, e: 5 } : null) },
    );
    expect(rows[0]?.bar).toBeNull();
    expect(rows[1]).toMatchObject({ kind: "sprint", bar: { s: 1, e: 5 } });
  });

  it("reads the properties the settings name", () => {
    const t = tv("T", "t", { props: { begin: "2026-11-01", finish: "2026-11-02" } });
    const [r] = rowsOf([{ ulid: "T", children: [] }], tasks(t), {
      settings: { start: "begin", end: "finish", roadmap: false },
    });
    expect(r?.bar).toEqual({ s: d("2026-11-01"), e: d("2026-11-02") });
  });

  it("reads a built-in date from the field of the task and a custom one from props, whichever is the start", () => {
    const t = tv("T", "t", {
      start: "2026-11-01",
      due: "2026-11-09",
      props: { finish: "2026-11-03" },
    });
    const [r] = rowsOf([{ ulid: "T", children: [] }], tasks(t), {
      settings: { start: "start", end: "finish", roadmap: false },
    });
    expect(r?.bar).toEqual({ s: d("2026-11-01"), e: d("2026-11-03") });
  });
});

describe("links and warnings", () => {
  const ref = (ulid: string) => ({ ulid, id: ulid, title: ulid });
  const a = tv("A", "a", { id: "T-1", start: "2026-11-02", due: "2026-11-04" });
  const on = (date: string) =>
    tv("B", "b", {
      id: "T-2",
      blocked_by: [ref("A"), ref("GONE")],
      start: date,
      due: "2026-11-09",
    });
  const top = (...u: string[]): TreeNode[] => u.map((ulid) => ({ ulid, children: [] }));

  it("says a blocker is late when it ends after the other starts", () => {
    const rows = rowsOf(top("A", "B"), tasks(a, on("2026-11-03")));
    const links = ganttLinks(rows);
    expect(links).toHaveLength(1);
    expect(links[0]?.late).toBe(true);
    expect(ganttWarnings(rows, links)).toEqual([
      {
        key: "A-B",
        id: "T-1 → T-2",
        text: "ブロックしている T-1 の終了（11/04）が、T-2 の開始（11/03）より後です。",
      },
    ]);
  });

  it("is on time when the blocker ends on the day the other starts", () => {
    const rows = rowsOf(top("A", "B"), tasks(a, on("2026-11-04")));
    expect(ganttLinks(rows)[0]?.late).toBe(false);
    expect(ganttWarnings(rows, ganttLinks(rows))).toEqual([]);
  });

  it("has a sentence for a parent that a child runs out of", () => {
    const p = tv("P", "p", { id: "T-9", due: "2026-11-03" });
    const rows = rowsOf([{ ulid: "P", children: [{ ulid: "A", children: [] }] }], tasks(p, a));
    expect(ganttWarnings(rows, [])[0]?.text).toBe(
      "子の期間（11/02 – 11/04）が、設定した期間（11/03 – 11/03）からはみ出しています。",
    );
  });
});

describe("the axis", () => {
  const range = { s: d("2026-10-30"), e: d("2026-11-03") };

  it("takes in the bars, the marks and today, with room around them", () => {
    const rows = rowsOf(
      [{ ulid: "A", children: [] }],
      tasks(tv("A", "a", { start: "2026-11-05", due: "2026-11-06" })),
    );
    expect(chartRange(rows, [d("2026-11-20")], d("2026-11-01"), "day")).toEqual({
      s: d("2026-11-01") - 3,
      e: d("2026-11-20") + 3,
    });
  });

  it("marks days with the month on the first and the weekends", () => {
    const t = ticks(range, "day");
    expect(t.map((x) => x.label)).toEqual(["30", "31", "11/1", "2", "3"]);
    expect(t.map((x) => x.weekend)).toEqual([false, true, true, false, false]);
  });

  it("marks Mondays and firsts of the month", () => {
    expect(ticks(range, "week").map((x) => x.label)).toEqual(["11/2"]);
    expect(ticks(range, "month").map((x) => x.label)).toEqual(["2026/11"]);
  });
});

describe("a drop", () => {
  const bar = { s: 10, e: 12 };

  it.each([
    ["move", 2, { s: 12, e: 14 }],
    ["start", 1, { s: 11, e: 12 }],
    ["start", 5, { s: 12, e: 12 }],
    ["end", -1, { s: 10, e: 11 }],
    ["end", -5, { s: 10, e: 10 }],
  ] as const)("%s by %i", (mode, delta, want) => {
    expect(dragged(bar, mode, delta)).toEqual(want);
  });

  it("writes only what changes, at the top of the body for the built-in dates", () => {
    const task = tv("T", "t", { start: "2026-11-02", due: "2026-11-04" });
    expect(dropPatch({ task }, { s: d("2026-11-02"), e: d("2026-11-05") }, DEFAULT_GANTT)).toEqual({
      due: "2026-11-05",
    });
  });

  it("gives a task with no dates both", () => {
    const task = tv("T", "t");
    expect(dropPatch({ task }, { s: d("2026-11-02"), e: d("2026-11-05") }, DEFAULT_GANTT)).toEqual({
      start: "2026-11-02",
      due: "2026-11-05",
    });
  });

  it("writes the custom date properties of the view in props", () => {
    const settings = { start: "kickoff", end: "due", roadmap: false };
    const task = tv("T", "t", { due: "2026-11-04", props: { kickoff: "2026-11-02" } });
    expect(dropPatch({ task }, { s: d("2026-11-01"), e: d("2026-11-05") }, settings)).toEqual({
      props: { kickoff: "2026-11-01" },
      due: "2026-11-05",
    });
  });
});
