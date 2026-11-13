import { describe, expect, it, vi } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import type { PlanningOverview, Sprint, SprintCard } from "../../model/planning";
import {
  closeBody,
  closeDefault,
  closeDestinations,
  deleteBody,
  newSprintForm,
  runSprintAction,
  sprintClosed,
  sprintCreated,
  sprintDeleted,
  sprintForm,
  sprintSaveFailed,
  sprintStarted,
  startBody,
  startDefaults,
} from "./sprintDialogs";

const sprint = (over: Partial<Sprint> = {}): Sprint => ({
  id: "SP4",
  name: "Sprint 4",
  goal: "",
  start: "2026-11-02",
  end: "2026-11-15",
  state: "planned",
  confirmed: null,
  ...over,
});

const card = (over: Partial<SprintCard> = {}): SprintCard => ({
  sprint: sprint(),
  total: { count: 0, points: 0 },
  done: { count: 0, points: 0 },
  undone: 0,
  canceled: 0,
  items: [],
  ...over,
});

const overview = (sprints: Sprint[]): PlanningOverview => ({
  active: null,
  defaults: { name: "Sprint 5", start: "2026-11-16", end: "2026-11-29" },
  sprints,
  releases: [],
  today: "2026-11-14",
  retentionDays: 30,
});

describe("sprint forms", () => {
  it("starts the create form from the overview's defaults with an empty goal", () => {
    expect(newSprintForm(overview([]))).toEqual({
      name: "Sprint 5",
      goal: "",
      start: "2026-11-16",
      end: "2026-11-29",
    });
  });

  it("starts the edit form from the sprint", () => {
    expect(sprintForm(sprint({ goal: "決済", start: "", end: "" }))).toEqual({
      name: "Sprint 4",
      goal: "決済",
      start: "",
      end: "",
    });
  });
});

describe("startBody", () => {
  it("counts items and points without canceled ones", () => {
    expect(startBody(card({ total: { count: 3, points: 8 }, canceled: 2 }))).toBe(
      "3 件・8 pt で始めます。",
    );
  });

  it("allows an empty sprint", () => {
    expect(startBody(card())).toBe("0 件・0 pt で始めます。");
  });

  it("does not show the float error of point sums", () => {
    expect(startBody(card({ total: { count: 2, points: 0.1 + 0.2 } }))).toBe(
      "2 件・0.3 pt で始めます。",
    );
  });
});

describe("startDefaults", () => {
  it("starts today and keeps an end date of today or later", () => {
    expect(startDefaults(sprint({ end: "2026-11-20" }), "2026-11-14")).toEqual({
      start: "2026-11-14",
      end: "2026-11-20",
    });
    expect(startDefaults(sprint({ end: "2026-11-14" }), "2026-11-14")).toEqual({
      start: "2026-11-14",
      end: "2026-11-14",
    });
  });

  it("ends 13 days after today when the end date is past or empty", () => {
    expect(startDefaults(sprint({ end: "2026-11-13" }), "2026-11-14").end).toBe("2026-11-27");
    expect(startDefaults(sprint({ end: "" }), "2026-12-25").end).toBe("2027-01-07");
  });
});

describe("closeBody", () => {
  it("shows done and undone counts", () => {
    expect(closeBody(card({ done: { count: 4, points: 10 }, undone: 2 }))).toBe(
      "完了 4 件・未完了 2 件です。",
    );
    expect(closeBody(card())).toBe("完了 0 件・未完了 0 件です。");
  });

  it("adds the note on canceled items when there are some", () => {
    expect(closeBody(card({ done: { count: 4, points: 10 }, undone: 2, canceled: 1 }))).toBe(
      "完了 4 件・未完了 2 件です。中止 1 件はスプリントに残り、集計には含めません。",
    );
  });
});

describe("closeDestinations", () => {
  const list = [
    sprint({ id: "A", name: "Sprint 4", state: "active" }),
    sprint({ id: "U", name: "未定", start: "", end: "" }),
    sprint({ id: "L", name: "Sprint 7", start: "2026-12-01", end: "2026-12-14" }),
    sprint({ id: "E", name: "Sprint 5", start: "2026-11-16", end: "2026-11-29" }),
    sprint({ id: "C", name: "Sprint 3", state: "closed" }),
    sprint({ id: "E2", name: "Sprint 6", start: "2026-11-16", end: "" }),
  ];

  it("lists planned sprints by start date, undecided last, then the backlog", () => {
    expect(closeDestinations(overview(list), "A")).toEqual([
      { id: "E", label: "Sprint 5" },
      { id: "E2", label: "Sprint 6" },
      { id: "L", label: "Sprint 7" },
      { id: "U", label: "未定" },
      { id: null, label: "バックログ" },
    ]);
  });

  it("defaults to the earliest planned sprint", () => {
    expect(closeDefault(overview(list), "A")).toBe("E");
  });

  it("offers only the backlog when no sprint is planned", () => {
    const o = overview([sprint({ id: "A", state: "active" })]);
    expect(closeDestinations(o, "A")).toEqual([{ id: null, label: "バックログ" }]);
    expect(closeDefault(o, "A")).toBeNull();
  });
});

describe("deleteBody", () => {
  it("says the items leave the sprint when it has some", () => {
    expect(deleteBody("Sprint 4", 3, 30)).toBe(
      "「Sprint 4」を削除します。入っている 3 件はスプリントから外れます。未完了のものはバックログに戻ります。" +
        "設定の「削除済み」から戻せます。30 日を過ぎると完全に削除されます。",
    );
  });

  it("omits the items sentence for an empty sprint", () => {
    expect(deleteBody("Sprint 4", 0, 14)).toBe(
      "「Sprint 4」を削除します。設定の「削除済み」から戻せます。14 日を過ぎると完全に削除されます。",
    );
  });
});

describe("toasts", () => {
  it("uses the texts of F1-18", () => {
    expect(sprintCreated("Sprint 5")).toBe("Sprint 5 を作成しました");
    expect(sprintStarted("Sprint 5")).toBe("Sprint 5 を開始しました");
    expect(sprintClosed("Sprint 5")).toBe("Sprint 5 を完了しました");
    expect(sprintDeleted).toBe("スプリントを削除しました");
    expect(sprintSaveFailed).toBe("保存できませんでした");
  });
});

describe("runSprintAction", () => {
  const deps = () => ({ toasts: { show: vi.fn() }, reload: vi.fn(async () => {}) });

  it("shows the toast, reloads and returns true on success", async () => {
    const d = deps();
    const refused = vi.fn();
    expect(await runSprintAction(d, async () => "Sprint 5 を開始しました", refused)).toBe(true);
    expect(d.toasts.show).toHaveBeenCalledWith("Sprint 5 を開始しました");
    expect(d.reload).toHaveBeenCalledOnce();
    expect(refused).not.toHaveBeenCalled();
  });

  it("shows no toast when the action has none (edit)", async () => {
    const d = deps();
    expect(await runSprintAction(d, async () => null, vi.fn())).toBe(true);
    expect(d.toasts.show).not.toHaveBeenCalled();
    expect(d.reload).toHaveBeenCalledOnce();
  });

  it("passes a refusal to the dialog without a toast", async () => {
    const d = deps();
    const refused = vi.fn();
    const op = async () => {
      throw new Refused("同じ名前のスプリントがあります");
    };
    expect(await runSprintAction(d, op, refused)).toBe(false);
    expect(refused).toHaveBeenCalledWith("同じ名前のスプリントがあります");
    expect(d.toasts.show).not.toHaveBeenCalled();
  });

  it("shows the error toast for other failures and reloads", async () => {
    const d = deps();
    const refused = vi.fn();
    const op = async () => {
      throw new NotFound("gone");
    };
    expect(await runSprintAction(d, op, refused)).toBe(false);
    expect(d.toasts.show).toHaveBeenCalledWith("保存できませんでした", { error: true });
    expect(refused).not.toHaveBeenCalled();
    expect(d.reload).toHaveBeenCalledOnce();
  });
});
