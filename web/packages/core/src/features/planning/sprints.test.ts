import { describe, expect, it } from "vitest";
import type { Member } from "../../model/member";
import type { PlanningOverview, Sprint, SprintCard } from "../../model/planning";
import type { WorkspaceDetail } from "../../model/workspace";
import { get } from "../../reactive/store";
import {
  createSprints,
  periodText,
  pill,
  progressRatio,
  progressText,
  type SprintsDeps,
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
});

describe("startDisabledReason", () => {
  const overview = (active: Sprint | null): PlanningOverview => ({
    active,
    defaults: { name: "Sprint 5", start: "2026-11-16", end: "2026-11-29" },
    sprints: [],
    releases: [],
    today: "2026-11-14",
  });

  it("asks to close the running sprint first", () => {
    expect(startDisabledReason(overview(sprint()))).toBe(
      "実行中のスプリントを先に完了してください",
    );
  });

  it("is null without a running sprint", () => {
    expect(startDisabledReason(overview(null))).toBeNull();
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

const members: Member[] = [{ login: "kukv", name: "Koki" }];
const detail = { settings: { statuses: [] } } as unknown as WorkspaceDetail;

// later は、あとで決める答え。
function later<T>() {
  let resolve: (v: T) => void = () => {};
  let reject: (e: Error) => void = () => {};
  const promise = new Promise<T>((ok, ng) => {
    resolve = ok;
    reject = ng;
  });
  return { promise, resolve, reject };
}

// depsOf は、カードを answers の順に返す口。設定とメンバーはいつも同じ値を返す。
function depsOf(answers: Promise<SprintCard[]>[]) {
  const calls: string[] = [];
  const deps: SprintsDeps = {
    port: {
      sprintCards: (s, w) => {
        calls.push(`${s}/${w}`);
        const next = answers.shift();
        if (!next) throw new Error("no answer");
        return next;
      },
    },
    workspaces: { get: () => Promise.resolve(detail) },
    listMembers: () => Promise.resolve(members),
  };
  return { deps, calls };
}

describe("createSprints", () => {
  it("reads the cards, the settings and the members of the workspace it opens", async () => {
    const { deps, calls } = depsOf([Promise.resolve([card()])]);
    const s = createSprints(deps);
    await s.open("S1", "W1");
    expect(calls).toEqual(["S1/W1"]);
    expect(get(s.state)).toEqual({
      storeId: "S1",
      workspaceId: "W1",
      cards: [card()],
      settings: detail.settings,
      members,
    });
  });

  it("does nothing on reload before opening", async () => {
    const { deps, calls } = depsOf([]);
    await createSprints(deps).reload();
    expect(calls).toEqual([]);
  });

  it("clears the old workspace while reading another", async () => {
    const second = later<SprintCard[]>();
    const { deps } = depsOf([Promise.resolve([card()]), second.promise]);
    const s = createSprints(deps);
    await s.open("S1", "W1");
    const opening = s.open("S1", "W2");
    expect(get(s.state)).toBeNull();
    second.resolve([]);
    await opening;
    expect(get(s.state)?.workspaceId).toBe("W2");
  });

  it("uses only the last of overlapping reads", async () => {
    const first = later<SprintCard[]>();
    const { deps } = depsOf([first.promise, Promise.resolve([])]);
    const s = createSprints(deps);
    const opening = s.open("S1", "W1");
    await s.reload();
    first.resolve([card()]);
    await opening;
    expect(get(s.state)?.cards).toEqual([]);
  });

  it("keeps the last cards when a reload fails, and does not throw", async () => {
    const failing = later<SprintCard[]>();
    const { deps } = depsOf([Promise.resolve([card()]), failing.promise]);
    const s = createSprints(deps);
    await s.open("S1", "W1");
    const reloading = s.reload();
    failing.reject(new Error("offline"));
    await expect(reloading).resolves.toBeUndefined();
    expect(get(s.state)?.cards).toEqual([card()]);
  });

  it("stops reading after close", async () => {
    const first = later<SprintCard[]>();
    const { deps, calls } = depsOf([first.promise]);
    const s = createSprints(deps);
    const opening = s.open("S1", "W1");
    s.close();
    first.resolve([card()]);
    await opening;
    await s.reload();
    expect(calls).toEqual(["S1/W1"]);
    expect(get(s.state)).toBeNull();
  });
});
