import { describe, expect, it } from "vitest";
import {
  bulkUpdatedLabel,
  closeLabel,
  closeToValue,
  daysLeft,
  groupByKind,
  hasPlans,
  inRankOrder,
  nextSprint,
  placed,
  pointsOf,
  ptLabel,
  queryValue,
  releaseDue,
  type Sprint,
  sprintDates,
  sprintsInOrder,
  sumPoints,
  usableTemplates,
  worklogLabel,
} from "./planning";
import { tv } from "./testing";

const sp = (ulid: string, state: string, start?: string): Sprint => ({
  ulid,
  name: ulid,
  state,
  start,
  done: 0,
  tasks: 0,
  points: 0,
  workspace: { ulid: "W", name: "main", id_prefix: "T" },
});
const now = new Date(2026, 10, 4, 15);

describe("hasPlans", () => {
  it("is true with a sprint or a release", () => {
    expect(hasPlans({ sprints: 0, releases: 0 })).toBe(false);
    expect(hasPlans({ sprints: 1, releases: 0 })).toBe(true);
    expect(hasPlans({ sprints: 0, releases: 2 })).toBe(true);
  });
});

describe("daysLeft", () => {
  it("counts the last day and stops at 0", () => {
    expect(daysLeft("2026-11-13", now)).toBe(10);
    expect(daysLeft("2026-11-04", now)).toBe(1);
    expect(daysLeft("2026-11-01", now)).toBe(0);
  });
});

describe("the words of dates", () => {
  it("writes the days of a sprint", () => {
    expect(sprintDates({ start: "2026-11-02", end: "2026-11-13" })).toBe("11/02 – 11/13");
    expect(sprintDates({ start: "2026-11-02" })).toBe("11/02 – —");
    expect(sprintDates({})).toBe("日付なし");
  });
  it("writes when a release is due", () => {
    expect(releaseDue("2026-11-12", now)).toBe("予定 2026-11-12（あと 8 日）");
    expect(releaseDue("2026-11-04", now)).toBe("予定 2026-11-04（今日）");
    expect(releaseDue("2026-11-01", now)).toBe("予定 2026-11-01（3 日前）");
    expect(releaseDue(undefined, now)).toBe("予定日なし");
  });
});

describe("the order of sprints", () => {
  it("takes the earliest planned sprint as next, ULID breaking a tie", () => {
    const s = [
      sp("C", "planned", "2026-12-01"),
      sp("B", "planned", "2026-11-16"),
      sp("A", "planned", "2026-11-16"),
      sp("D", "active", "2026-11-01"),
    ];
    expect(nextSprint(s)?.ulid).toBe("A");
    expect(nextSprint([sp("D", "active")])).toBeUndefined();
  });
  it("lists active, planned, then closed from the newest", () => {
    const got = sprintsInOrder([
      sp("old", "closed", "2026-01-01"),
      sp("new", "closed", "2026-10-01"),
      sp("p", "planned"),
      sp("a", "active"),
    ]);
    expect(got.map((s) => s.ulid)).toEqual(["a", "p", "new", "old"]);
  });
});

describe("queryValue", () => {
  it("quotes a value with a space", () => {
    expect(queryValue("v2")).toBe("v2");
    expect(queryValue("Sprint 44")).toBe('"Sprint 44"');
  });
});

describe("the tasks of a plan", () => {
  it("sorts by rank, then by ULID", () => {
    const got = inRankOrder([
      tv("B", "b", { rank: "a" }),
      tv("A", "a", { rank: "a" }),
      tv("C", "c", { rank: "0" }),
    ]);
    expect(got.map((t) => t.ulid)).toEqual(["C", "A", "B"]);
  });
  it("reads and sums points", () => {
    const a = tv("A", "a", { points: 3 });
    const b = tv("B", "b", { points: null });
    expect(pointsOf(a)).toBe(3);
    expect(pointsOf(b)).toBeNull();
    expect(pointsOf(tv("D", "d", { points: 0 }))).toBe(0);
    expect(pointsOf(tv("E", "e"))).toBeNull();
    // a number in props is a custom property, not the points of the task
    expect(pointsOf(tv("F", "f", { props: { points: 9 } }))).toBeNull();
    expect(sumPoints([a, b, tv("C", "c", { points: 1.5 })])).toBe(4.5);
    expect(ptLabel(4.5)).toBe("4.5 pt");
  });
  it("places a task among the others", () => {
    expect(placed(["a", "b", "c"], "c", 0)).toEqual(["c", "a", "b"]);
    expect(placed(["a", "b", "c"], "a", 1)).toEqual(["b", "a", "c"]);
    expect(placed(["a", "b", "c"], "a", 9)).toEqual(["b", "c", "a"]);
  });
  it("groups by kind in the fixed order, others last", () => {
    const got = groupByKind([
      tv("A", "a", { kind: "bug" }),
      tv("B", "b", { kind: "step" }),
      tv("C", "c"),
      tv("D", "d", { kind: "gone" }),
      tv("E", "e", { kind: "story" }),
    ]);
    expect(got.map((g) => g.kind)).toEqual(["story", "task", "bug", "step", "gone"]);
  });
});

describe("closing", () => {
  const next = sp("N", "planned");
  it("gives the value of to", () => {
    expect(closeToValue({ kind: "next" })).toBe("next");
    expect(closeToValue({ kind: "backlog" })).toBe("backlog");
    expect(closeToValue({ kind: "sprint", name: "S9" })).toBe("S9");
  });
  it("says where the tasks go", () => {
    expect(closeLabel(0, { kind: "next" }, next)).toBe("閉じる");
    expect(closeLabel(4, { kind: "next" }, next)).toBe("閉じて 4 件を N へ");
    expect(closeLabel(4, { kind: "backlog" }, next)).toBe("閉じて 4 件をバックログへ");
    expect(closeLabel(1, { kind: "sprint", name: "S9" }, next)).toBe("閉じて 1 件を S9 へ");
  });
});

describe("worklogLabel", () => {
  it("compares with the estimate when there is one", () => {
    expect(worklogLabel("2h", "6h")).toBe("2h / 見積もり 6h");
    expect(worklogLabel("2h", undefined)).toBe("2h");
  });
});

describe("usableTemplates", () => {
  it("keeps what the limit lists", () => {
    const all = [{ name: "a" }, { name: "b" }];
    expect(usableTemplates(all, ["b"])).toEqual([{ name: "b" }]);
    expect(usableTemplates(all, [])).toEqual(all);
    expect(usableTemplates(all, null)).toEqual(all);
  });
});

describe("bulkUpdatedLabel", () => {
  it("says the value it set, or that it cleared the field", () => {
    expect(bulkUpdatedLabel(3, "ステータス", "done")).toBe("3 件のステータスを done にしました");
    expect(bulkUpdatedLabel(3, "ラベル", "")).toBe("3 件のラベルを外しました");
  });
});
