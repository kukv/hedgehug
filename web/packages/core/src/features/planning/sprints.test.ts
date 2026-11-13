import { describe, expect, it } from "vitest";
import type { PlanningOverview, Sprint, SprintCard } from "../../model/planning";
import {
  periodText,
  pill,
  progressRatio,
  progressText,
  sections,
  sprintMenu,
  startDisabledReason,
} from "./sprints";

const sprint = (over: Partial<Sprint> = {}): Sprint => ({
  id: "SP4",
  name: "Sprint 4",
  goal: "",
  start: "2026-11-02",
  end: "2026-11-15",
  state: "active",
  confirmed: null,
  ...over,
});

const card = (over: Partial<SprintCard> = {}, s: Partial<Sprint> = {}): SprintCard => ({
  sprint: sprint(s),
  total: { count: 0, points: 0 },
  done: { count: 0, points: 0 },
  undone: 0,
  canceled: 0,
  items: [],
  ...over,
});

describe("sections", () => {
  it("puts 実行中, 計画中, 完了 in this order and keeps the order of the cards", () => {
    const closed = card({}, { id: "SP1", state: "closed" });
    const p2 = card({}, { id: "SP6", state: "planned" });
    const active = card({}, { id: "SP4" });
    const p1 = card({}, { id: "SP5", state: "planned" });
    expect(sections([closed, p2, active, p1])).toEqual([
      { state: "active", label: "実行中", cards: [active] },
      { state: "planned", label: "計画中", cards: [p2, p1] },
      { state: "closed", label: "完了", cards: [closed] },
    ]);
  });

  it("leaves out the states without cards", () => {
    const p = card({}, { state: "planned" });
    expect(sections([p])).toEqual([{ state: "planned", label: "計画中", cards: [p] }]);
    expect(sections([])).toEqual([]);
  });
});

describe("pill", () => {
  it("counts the end day itself as 1 day left", () => {
    expect(pill(sprint(), "2026-11-15")).toBe("残り 1 日");
    expect(pill(sprint(), "2026-11-14")).toBe("残り 2 日");
    expect(pill(sprint({ start: "2026-12-28", end: "2027-01-03" }), "2026-12-30")).toBe(
      "残り 5 日",
    );
  });

  it("is null for a running sprint without the end day", () => {
    expect(pill(sprint({ end: "" }), "2026-11-14")).toBeNull();
  });

  it("says the end has passed after the end day", () => {
    expect(pill(sprint(), "2026-11-16")).toBe("期限を過ぎています");
  });

  it("is null unless the sprint is running", () => {
    expect(pill(sprint({ state: "planned" }), "2026-11-14")).toBeNull();
    expect(pill(sprint({ state: "closed" }), "2026-11-14")).toBeNull();
  });
});

describe("progressText", () => {
  it("shows the counts and the points without the canceled", () => {
    const c = card({ total: { count: 8, points: 21 }, done: { count: 3, points: 8 } });
    expect(progressText(c)).toBe("3 / 8 件 · 8 / 21 pt");
  });

  it("shows 0 / 0 for an empty sprint", () => {
    expect(progressText(card())).toBe("0 / 0 件 · 0 / 0 pt");
  });

  it("adds the confirmed points for a closed sprint", () => {
    const c = card(
      { total: { count: 2, points: 5 }, done: { count: 2, points: 5 } },
      {
        state: "closed",
        confirmed: { committed: { count: 3, points: 8 }, completed: { count: 2, points: 5 } },
      },
    );
    expect(progressText(c)).toBe("2 / 2 件 · 5 / 5 pt · 確定 5/8 pt");
  });

  it("writes the points the way the detail panel does", () => {
    const c = card({ total: { count: 2, points: 2.5 }, done: { count: 1, points: 0.5 } });
    expect(progressText(c)).toBe("1 / 2 件 · 0.5 / 2.5 pt");
  });

  it("does not show the noise of adding decimals", () => {
    const c = card({ total: { count: 2, points: 0.1 + 0.2 }, done: { count: 0, points: 0 } });
    expect(progressText(c)).toBe("0 / 2 件 · 0 / 0.3 pt");
    const closed = card(
      { total: { count: 2, points: 0.1 + 0.2 }, done: { count: 2, points: 0.1 + 0.2 } },
      {
        state: "closed",
        confirmed: {
          committed: { count: 3, points: 0.7 + 0.1 },
          completed: { count: 2, points: 0.1 + 0.2 },
        },
      },
    );
    expect(progressText(closed)).toBe("2 / 2 件 · 0.3 / 0.3 pt · 確定 0.3/0.8 pt");
  });
});

describe("progressRatio", () => {
  it("is the ratio of the done points, and 0 when there are no points", () => {
    expect(
      progressRatio(card({ total: { count: 4, points: 8 }, done: { count: 1, points: 2 } })),
    ).toBe(0.25);
    expect(
      progressRatio(card({ total: { count: 2, points: 0 }, done: { count: 1, points: 0 } })),
    ).toBe(0);
  });
});

describe("periodText", () => {
  it("shows the start and the end", () => {
    expect(periodText(sprint(), 2026)).toBe("11/2 〜 11/15");
    expect(periodText(sprint({ start: "2025-12-29", end: "2026-01-11" }), 2026)).toBe(
      "2025/12/29 〜 1/11",
    );
  });

  it("shows 期間未定 without the days", () => {
    expect(periodText(sprint({ state: "planned", start: "", end: "" }), 2026)).toBe("期間未定");
  });

  it("shows なし for the missing one of a planned sprint", () => {
    expect(periodText(sprint({ state: "planned", end: "" }), 2026)).toBe("11/2 〜 なし");
    expect(periodText(sprint({ state: "planned", start: "" }), 2026)).toBe("なし 〜 11/15");
  });
});

describe("startDisabledReason", () => {
  const overview = (active: Sprint | null): PlanningOverview => ({
    active,
    defaults: { name: "Sprint 5", start: "2026-11-16", end: "2026-11-29" },
    sprints: [],
    releases: [],
    today: "2026-11-14",
    retentionDays: 30,
  });

  it("asks to close the running sprint first", () => {
    expect(startDisabledReason(overview(sprint()))).toBe(
      "実行中のスプリントを先に完了してください",
    );
  });

  it("is null without a running sprint", () => {
    expect(startDisabledReason(overview(null))).toBeNull();
  });

  it("looks at the cards before the overview is read", () => {
    expect(startDisabledReason(null, [card({}, { state: "planned" })])).toBeNull();
    expect(startDisabledReason(null, [card()])).toBe("実行中のスプリントを先に完了してください");
  });

  it("follows the overview once it is read", () => {
    expect(startDisabledReason(overview(null), [card()])).toBeNull();
  });
});

describe("sprintMenu", () => {
  it("has 編集… and 削除…", () => {
    expect(sprintMenu(sprint({ state: "planned" }), false)).toEqual([
      { id: "edit", label: "編集…", disabled: false },
      { id: "delete", label: "削除…", disabled: false },
    ]);
  });

  it("disables 削除… of the running sprint", () => {
    expect(sprintMenu(sprint(), false)[1]).toEqual({
      id: "delete",
      label: "削除…",
      disabled: true,
      hint: "実行中は削除できません",
    });
  });

  it("disables both in a read-only store", () => {
    expect(sprintMenu(sprint(), true)).toEqual([
      { id: "edit", label: "編集…", disabled: true, hint: "閲覧のみの Store です" },
      { id: "delete", label: "削除…", disabled: true, hint: "閲覧のみの Store です" },
    ]);
  });
});
