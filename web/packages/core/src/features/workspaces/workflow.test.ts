import { describe, expect, it } from "vitest";
import { defaultDetail } from "./fixtures.test-util";
import { transitionRefusal, workflowRows } from "./workflow";

const settingsWith = (workflow: Record<string, string[]>) => ({
  ...defaultDetail("s1", "w1").settings,
  workflow,
});

describe("workflow rows", () => {
  it("shows どこへでも for a row without checks and — for itself", () => {
    const s = settingsWith({});
    const rows = workflowRows(s);
    expect(rows.map((r) => r.name)).toEqual(["未着手", "進行中", "レビュー中", "完了", "中止"]);
    expect(rows[0]?.note).toBe("どこへでも");
    expect(rows[0]?.cells.map((c) => c.self)).toEqual([true, false, false, false, false]);
  });

  it("checks the allowed targets and says 制限あり", () => {
    const base = settingsWith({});
    const [todo, doing] = base.statuses.map((v) => v.id);
    const rows = workflowRows(settingsWith({ [todo as string]: [doing as string] }));
    expect(rows[0]?.note).toBe("制限あり");
    expect(rows[0]?.cells.map((c) => c.checked)).toEqual([false, true, false, false, false]);
  });
});

describe("transition refusal", () => {
  it("uses the words of spec 03 and lists the allowed targets in status order", () => {
    const base = settingsWith({});
    const [todo, doing, done, pending] = base.statuses.map((v) => v.id) as string[];
    const s = settingsWith({ [todo as string]: [pending as string, doing as string] });
    expect(transitionRefusal(s, todo as string, done as string)).toBe(
      "ワークフローの制約: 未着手 から レビュー中 には変更できません(進行中・完了 のみ)",
    );
    expect(transitionRefusal(s, todo as string, doing as string)).toBeNull();
    expect(transitionRefusal(s, done as string, todo as string)).toBeNull();
    expect(transitionRefusal(s, todo as string, todo as string)).toBeNull();
  });
});
