import { describe, expect, it } from "vitest";
import { dayOf } from "./gantt";
import {
  amountLabel,
  type BurndownView,
  burndownFacts,
  burndownPlot,
  cfdPlot,
  daysLabel,
  estimateLines,
  hoursOf,
  velocityPlot,
} from "./reports";

const d = (date: string): number => dayOf(date) as number;

const burndown = (over: Partial<BurndownView> = {}): BurndownView => ({
  sprint: {
    ulid: "S1",
    name: "Sprint 1",
    state: "active",
    start: "2026-11-02",
    end: "2026-11-06",
  },
  by: "points",
  days: [
    { date: "2026-11-02", scope: 8, done: 0, remaining: 8, added: 0 },
    { date: "2026-11-03", scope: 11, done: 2, remaining: 9, added: 3 },
  ],
  ...over,
});

describe("burndownPlot", () => {
  it("runs over the whole sprint, with nothing on the days to come", () => {
    const p = burndownPlot(burndown());
    expect(p.xs).toEqual([0, 1, 2, 3, 4].map((i) => d("2026-11-02") + i));
    expect(p.remaining).toEqual([8, 9, null, null, null]);
    expect(p.scope).toEqual([8, 11, null, null, null]);
  });

  it("draws the ideal from the first scope to 0 on the last day", () => {
    expect(burndownPlot(burndown()).ideal).toEqual([8, 6, 4, 2, 0]);
  });

  it("starts where the sprint started to hold something, not at its date", () => {
    const p = burndownPlot(
      burndown({
        days: [
          { date: "2026-11-02", scope: 0, done: 0, remaining: 0, added: 0 },
          { date: "2026-11-03", scope: 8, done: 0, remaining: 8, added: 0 },
        ],
      }),
    );
    expect(p.remaining).toEqual([null, 8, null, null, null]);
    expect(p.ideal[0]).toBe(8);
  });

  it("takes in a day past the end, for a sprint that ran over", () => {
    const v = burndown({
      days: [{ date: "2026-11-08", scope: 1, done: 1, remaining: 0, added: 0 }],
    });
    expect(burndownPlot(v).xs.at(-1)).toBe(d("2026-11-08"));
  });

  it("has nothing to draw for a sprint with no days and no dates", () => {
    const v = burndown({ sprint: { ulid: "S", name: "S", state: "closed" }, days: [] });
    expect(burndownPlot(v).xs).toEqual([]);
  });

  it("has one day for a sprint of one day", () => {
    const v = burndown({
      sprint: { ulid: "S", name: "S", state: "active", start: "2026-11-02", end: "2026-11-02" },
      days: [{ date: "2026-11-02", scope: 4, done: 0, remaining: 4, added: 0 }],
    });
    expect(burndownPlot(v).ideal).toEqual([4]);
  });
});

describe("burndownFacts", () => {
  it("says where the sprint is now and what joined", () => {
    expect(burndownFacts(burndown())).toEqual({ remaining: 9, scope: 11, added: 3 });
  });

  it("is all zero with no days", () => {
    expect(burndownFacts(burndown({ days: null }))).toEqual({ remaining: 0, scope: 0, added: 0 });
  });
});

describe("amountLabel", () => {
  it("says what is counted", () => {
    expect(amountLabel(2.5, "points")).toBe("2.5 pt");
    expect(amountLabel(3, "count")).toBe("3 件");
  });
});

describe("velocityPlot", () => {
  const row = (name: string, points: number) => ({
    sprint: { ulid: name, name, state: "closed" },
    points,
    tasks: 2,
    unpointed: 0,
  });

  it("averages the last three", () => {
    const p = velocityPlot([row("a", 100), row("b", 10), row("c", 20), row("d", 30)]);
    expect(p.names).toEqual(["a", "b", "c", "d"]);
    expect(p.average).toBe(20);
  });

  it("has no average with no sprint", () => {
    expect(velocityPlot([]).average).toBeNull();
  });
});

describe("cfdPlot", () => {
  it("stacks the columns with the last at the bottom, the top first", () => {
    const p = cfdPlot({
      by: "category",
      columns: ["todo", "doing", "done", "unknown"],
      days: [
        { date: "2026-11-02", counts: { todo: 3, doing: 2, done: 1 } },
        { date: "2026-11-03", counts: { todo: 1, doing: 2, done: 3, unknown: 1 } },
      ],
    });
    expect(p.stacks.map((s) => s.name)).toEqual(["todo", "doing", "done", "unknown"]);
    expect(p.stacks[0]?.top).toEqual([6, 7]);
    expect(p.stacks[1]?.top).toEqual([3, 6]);
    expect(p.stacks[2]?.top).toEqual([1, 4]);
    expect(p.stacks[3]?.top).toEqual([0, 1]);
    expect(p.stacks[0]?.values).toEqual([3, 1]);
  });
});

describe("durations", () => {
  it.each([
    ["1h30m", 1.5],
    ["45m", 0.75],
    ["2h", 2],
    ["", null],
    [undefined, null],
    ["soon", null],
  ])("%s", (text, want) => {
    expect(hoursOf(text)).toBe(want);
  });

  it("reads a time of a day or more in days", () => {
    expect(daysLabel("36h")).toBe("1.5 日");
    expect(daysLabel("5h30m")).toBe("5h30m");
  });
});

describe("estimateLines", () => {
  it("places the marks against the widest and says what ran over", () => {
    const [a, b, c] = estimateLines({
      estimate: "3h",
      actual: "5h",
      tasks: [
        { ulid: "A", id: "T-1", title: "a", estimate: "2h", actual: "4h" },
        { ulid: "B", id: "T-2", title: "b", estimate: "1h", actual: "1h" },
        { ulid: "C", id: "T-3", title: "c", actual: "1h" },
      ],
    });
    expect(a).toMatchObject({ estimateAt: 50, actualAt: 100, over: true });
    expect(b).toMatchObject({ estimateAt: 25, over: false });
    expect(c).toMatchObject({ estimate: "—", estimateAt: null, over: false });
  });
});
