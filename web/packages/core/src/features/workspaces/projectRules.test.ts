import { describe, expect, it } from "vitest";
import { projectStatusOptions, statusOptions } from "../items/properties";
import { allowedTargets, rulesOf, unusedRefusal, usedStatusRows, usesStatus } from "./projectRules";
import { cancel, doing, done, P, review, sample, todo } from "./projectRules.fixtures.test-util";
import { projectWorkflowRows, transitionRefusal, workflowRows } from "./workflow";

describe("uses", () => {
  it("keeps the initial status and category defaults even when hidden", () => {
    const s = sample({ hiddenStatuses: [todo, review] });
    expect(usesStatus(s, P, todo)).toBe(true);
    expect(usesStatus(s, P, review)).toBe(false);
    const s2 = {
      ...s,
      initialStatus: doing,
      categoryDefaults: { ...s.categoryDefaults, todo: doing },
    };
    expect(usesStatus(s2, P, todo)).toBe(false);
  });

  it("uses everything for a project without rules", () => {
    expect(rulesOf(sample(), "nope").hiddenStatuses).toEqual([]);
    expect(usesStatus(sample(), "nope", review)).toBe(true);
  });

  it("says the refusal text for an unused status", () => {
    expect(unusedRefusal("レビュー中")).toBe("このプロジェクトでは「レビュー中」を使いません");
  });
});

describe("transition refusal with a project", () => {
  const tail = (a: string) => `ワークフローの制約: 未着手 から 完了 には変更できません(${a})`;

  it("needs both the workspace and the project to allow", () => {
    const s = sample({ workflow: { [todo]: [doing] } }, { workflow: { [todo]: [doing, done] } });
    expect(transitionRefusal(s, todo, done, P)).toBe(tail("進行中 のみ"));
    expect(transitionRefusal(s, todo, doing, P)).toBeNull();
    // project を渡さなければワークスペースだけ
    expect(transitionRefusal(s, todo, done)).toBeNull();
  });

  it("works with the project only", () => {
    expect(transitionRefusal(sample({ workflow: { [todo]: [doing] } }), todo, done, P)).toBe(
      tail("進行中 のみ"),
    );
  });

  it("says there is no target when the two have none in common", () => {
    const s = sample({ workflow: { [todo]: [doing] } }, { workflow: { [todo]: [done] } });
    expect(transitionRefusal(s, todo, done, P)).toBe(tail("変更できる先がありません"));
  });

  it("ignores a project row that checks only unused statuses", () => {
    const s = sample({ hiddenStatuses: [review], workflow: { [todo]: [review] } });
    expect(transitionRefusal(s, todo, done, P)).toBeNull();
    const s2 = sample({ hiddenStatuses: [review], workflow: { [todo]: [doing] } });
    expect(allowedTargets(s2, P, todo)).toEqual({ to: [doing], restricted: true });
  });

  it("drops unused statuses from the targets and the refusal", () => {
    const s = sample({ hiddenStatuses: [review] }, { workflow: { [todo]: [doing, review] } });
    expect(allowedTargets(s, P, todo)).toEqual({ to: [doing], restricted: true });
    expect(allowedTargets(sample({ hiddenStatuses: [review] }), P, todo)).toEqual({
      to: [doing, done, cancel],
      restricted: false,
    });
    expect(transitionRefusal(s, todo, done, P)).toBe(tail("進行中 のみ"));
  });

  it("passes a missing, empty or same from", () => {
    const s = sample({ workflow: { Gone: [doing] } }, { workflow: { Gone: [done] } });
    expect(transitionRefusal(s, "Gone", cancel, P)).toBeNull();
    expect(transitionRefusal(s, "", cancel, P)).toBeNull();
    expect(transitionRefusal(s, todo, todo, P)).toBeNull();
  });
});

describe("project status options", () => {
  it("lists only the used statuses and keeps the unused current one with a hint", () => {
    const s = sample({ hiddenStatuses: [review] });
    expect(statusOptions(s, todo, false, P).map((o) => o.id)).toEqual([todo, doing, done, cancel]);
    const kept = statusOptions(s, review, false, P);
    expect(kept.map((o) => o.id)).toEqual([todo, doing, review, done, cancel]);
    expect(kept[2]).toMatchObject({ checked: true, hint: "このプロジェクトでは使わない" });
    expect(kept[1]?.checked).toBe(false);
  });

  it("applies the project transitions when workflow is on", () => {
    const s = sample({ workflow: { [todo]: [doing] } });
    const opts = statusOptions(s, todo, true, P);
    expect(opts.find((o) => o.id === done)).toMatchObject({ disabled: true });
    expect(opts.find((o) => o.id === doing)?.disabled).toBeUndefined();
  });

  it("projectStatusOptions is statusOptions with the project", () => {
    const s = sample({ hiddenStatuses: [review] });
    expect(projectStatusOptions(s, P, todo)).toEqual(statusOptions(s, todo, false, P));
  });

  it("is unchanged without a project", () => {
    const s = sample({ hiddenStatuses: [review] });
    expect(statusOptions(s, todo).map((o) => o.id)).toEqual([todo, doing, review, done, cancel]);
  });
});

describe("project workflow rows", () => {
  it("shows only used statuses and disables cells the workspace forbids", () => {
    const s = sample(
      { hiddenStatuses: [review], workflow: { [todo]: [doing] } },
      { workflow: { [todo]: [doing, done] } },
    );
    const rows = projectWorkflowRows(s, P);
    expect(rows.map((r) => r.from)).toEqual([todo, doing, done, cancel]);
    const row = rows[0];
    expect(row?.note).toBe("制限あり");
    expect(row?.cells.map((c) => c.to)).toEqual([todo, doing, done, cancel]);
    expect(row?.cells.map((c) => c.checked)).toEqual([false, true, false, false]);
    expect(row?.cells[3]).toMatchObject({
      disabled: true,
      reason: "ワークスペースの制約で変更できません",
    });
    expect(row?.cells[2]?.disabled).toBeUndefined();
    expect(row?.blocked).toBeUndefined();
    expect(rows[1]?.note).toBe("ワークスペースと同じ");
  });

  it("says blocked when nothing is allowed by both", () => {
    const s = sample({ workflow: { [todo]: [doing] } }, { workflow: { [todo]: [done] } });
    expect(projectWorkflowRows(s, P)[0]?.blocked).toBe("変更できる先がありません");
  });

  it("treats a row of unused checks as 同じ", () => {
    const s = sample({ hiddenStatuses: [review], workflow: { [todo]: [review] } });
    expect(projectWorkflowRows(s, P)[0]?.note).toBe("ワークスペースと同じ");
  });

  it("leaves workflowRows without a project unchanged", () => {
    expect(workflowRows(sample())).toHaveLength(5);
  });
});

describe("used status rows", () => {
  it("counts, refuses by reason and counts in-use hidden statuses", () => {
    const s = sample({ hiddenStatuses: [review, done] }, { initialStatus: done });
    const items = [{ status: review }, { status: review }, { status: todo }, { status: cancel }];
    const { count, rows } = usedStatusRows(s, P, items);
    expect(count).toBe("4 / 5");
    expect(rows.map((r) => r.checked)).toEqual([true, true, false, true, true]);
    expect(rows[2]).toMatchObject({ disabled: false, usage: "2 件が使用中" });
    expect(rows[3]).toMatchObject({
      disabled: true,
      note: "初期ステータスで、完了の既定でもあるため外せません",
    });
    expect(rows[0]).toMatchObject({ disabled: true, note: "未着手の既定のため外せません" });
    expect(rows[0]?.usage).toBeUndefined();
  });

  it("counts the used ones against all", () => {
    const s = sample({ hiddenStatuses: [review] });
    expect(usedStatusRows(s, P, []).count).toBe("4 / 5");
  });

  it("says 初期ステータスのため外せません when it is not a default", () => {
    const s = sample({}, { initialStatus: review });
    expect(usedStatusRows(s, P, []).rows[2]?.note).toBe("初期ステータスのため外せません");
  });

  it("does not show usage for statuses that are used", () => {
    const rowsOf = usedStatusRows(sample(), P, [{ status: review }]).rows;
    expect(rowsOf[2]?.usage).toBeUndefined();
  });
});
