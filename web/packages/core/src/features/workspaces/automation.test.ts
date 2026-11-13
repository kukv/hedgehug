import { describe, expect, it } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import type { WorkspaceSettings } from "../../model/workspace";
import {
  keepStatus,
  newRuleForm,
  recentRows,
  ruleForm,
  ruleFormRefusal,
  ruleRows,
  ruleSaveRefusal,
  ruleSummary,
  scopeStatuses,
  switchOnRefusal,
  valueOptions,
  withAction,
  withTrigger,
} from "./automation";
import { defaultDetail } from "./fixtures.test-util";

// ステータスは Todo・InProgress・Done・Pending・Cancel。完了の既定は Done。
const base = defaultDetail("S", "W").settings;
const names = ["Todo", "InProgress", "Done", "Pending", "Cancel"];
const settings: WorkspaceSettings = {
  ...base,
  statuses: names.map((name, i) => ({ id: `st${i + 1}`, name, color: "gray" })),
  categoryDefaults: { done: "st3" },
};
const id = (name: string) => settings.statuses.find((v) => v.name === name)?.id ?? "";
const members = [{ login: "tanaka", name: "田中" }];

describe("ruleSummary", () => {
  it("writes the trigger and the action (03 の例)", () => {
    const r = {
      name: "x",
      trigger: "status",
      triggerStatus: id("Done"),
      allMerged: false,
      action: "assignee",
      value: "@none",
    } as const;
    expect(ruleSummary(settings, members, r)).toBe(
      "ステータスが Done になったとき → 担当者を 未割り当て に",
    );
  });
  it("uses the merged-all wording only in the list", () => {
    const r = {
      name: "x",
      trigger: "pr_merged",
      triggerStatus: "",
      allMerged: true,
      action: "status",
      value: id("Done"),
    } as const;
    expect(ruleSummary(settings, members, r)).toBe(
      "リンクした PR がすべてマージされたとき → ステータスを Done に",
    );
  });
  it("shows the member's name", () => {
    const r = {
      name: "x",
      trigger: "steps_done",
      triggerStatus: "",
      allMerged: false,
      action: "assignee",
      value: "tanaka",
    } as const;
    expect(ruleSummary(settings, members, r)).toBe("Step がすべて完了したとき → 担当者を 田中 に");
  });
});

describe("the rule form", () => {
  it("starts with PR merged, change status, and the done default", () => {
    expect(newRuleForm(settings)).toEqual({
      name: "",
      trigger: "pr_merged",
      triggerStatus: "",
      allMerged: false,
      action: "status",
      value: id("Done"),
    });
  });
  it("takes the status choices of the action value from the given statuses", () => {
    const form = newRuleForm(settings);
    const all = valueOptions(form, settings, []);
    expect(all.length).toBe(settings.statuses.length);
    const narrowed = [{ value: id("Done"), label: "Done" }];
    expect(valueOptions(form, settings, [], narrowed)).toEqual(narrowed);
  });
  it("starts an edit with only the six input fields of the rule", () => {
    const rule = { ...newRuleForm(settings), id: "R1", name: "完了", enabled: false };
    expect(ruleForm(rule)).toEqual({ ...newRuleForm(settings), name: "完了" });
  });
  it("resets the value when the action changes", () => {
    const f = withAction(newRuleForm(settings), "assignee", settings);
    expect(f.value).toBe("@none");
    expect(withAction(f, "status", settings).value).toBe(settings.statuses[0]?.id);
  });
  it("gives the status trigger the first status and drops the condition", () => {
    const f = withTrigger({ ...newRuleForm(settings), allMerged: true }, "status", settings);
    expect(f.triggerStatus).toBe(settings.statuses[0]?.id);
    expect(f.allMerged).toBe(false);
  });
  it("takes the default statuses from the given statuses", () => {
    const used = [
      { value: id("InProgress"), label: "InProgress" },
      { value: id("Pending"), label: "Pending" },
    ];
    expect(newRuleForm(settings, used).value).toBe(id("InProgress"));
    expect(newRuleForm(settings, [...used, { value: id("Done"), label: "Done" }]).value).toBe(
      id("Done"),
    );
    const f = newRuleForm(settings, used);
    expect(withTrigger(f, "status", settings, used).triggerStatus).toBe(id("InProgress"));
    const assignee = withAction(f, "assignee", settings, used);
    expect(withAction(assignee, "status", settings, used).value).toBe(id("InProgress"));
  });
  it("lists the statuses a dedicated rule's project uses", () => {
    const hidden: WorkspaceSettings = {
      ...settings,
      initialStatus: id("Todo"),
      projectRules: {
        P1: {
          hiddenStatuses: [id("Pending")],
          workflow: {},
          templates: [],
          templateDefaults: {},
          disabledRules: [],
        },
      },
    };
    expect(scopeStatuses(hidden, "P1").map((o) => o.label)).toEqual([
      "Todo",
      "InProgress",
      "Done",
      "Cancel",
    ]);
    expect(scopeStatuses(hidden).map((o) => o.label)).toEqual(names);
  });
  it("keeps the stored status as an extra choice when the project does not use it", () => {
    const used = [
      { value: id("Todo"), label: "Todo" },
      { value: id("Done"), label: "Done" },
    ];
    expect(keepStatus(settings, used, id("Pending")).map((o) => o.label)).toEqual([
      "Todo",
      "Done",
      "Pending",
    ]);
    expect(keepStatus(settings, used, id("InProgress")).map((o) => o.label)).toEqual([
      "Todo",
      "InProgress",
      "Done",
    ]);
    expect(keepStatus(settings, used, id("Done"))).toEqual(used);
    expect(keepStatus(settings, used, "tanaka")).toEqual(used);
  });
  it("turns a failed save into the message shown in the dialog", () => {
    expect(ruleSaveRefusal(new Refused("同じ名前のルールがあります"))).toBe(
      "同じ名前のルールがあります",
    );
    expect(ruleSaveRefusal(new NotFound("R1"))).toBe("ルールを保存できませんでした");
    const other = new Error("boom");
    expect(() => ruleSaveRefusal(other)).toThrow(other);
  });
  it("refuses like the server", () => {
    const rules = [{ ...newRuleForm(settings), id: "R1", name: "完了で外す", enabled: true }];
    const f = newRuleForm(settings);
    expect(ruleFormRefusal({ ...f, name: " " }, rules, "")).toBe("名前を入力してください");
    expect(ruleFormRefusal({ ...f, name: " 完了で外す " }, rules, "")).toBe(
      "同じ名前のルールがあります",
    );
    expect(ruleFormRefusal({ ...f, name: "完了で外す" }, rules, "R1")).toBeNull();
    const same = {
      ...f,
      name: "y",
      trigger: "status" as const,
      triggerStatus: id("Done"),
      value: id("Done"),
    };
    expect(ruleFormRefusal(same, rules, "")).toBe("きっかけと同じステータスには変更できません");
  });
});

describe("the list", () => {
  it("refuses to switch on a rule that changes to its trigger status", () => {
    const r = {
      name: "y",
      trigger: "status",
      triggerStatus: "S",
      allMerged: false,
      action: "status",
      value: "S",
      id: "R",
      enabled: false,
    } as const;
    expect(switchOnRefusal(r)).toBe(
      "きっかけと同じステータスには変更できません。ルールを編集してください",
    );
  });
  it("lists rules and recent runs", () => {
    const a = {
      rules: [{ ...newRuleForm(settings), id: "R1", name: "PR マージで完了", enabled: true }],
      recent: [
        {
          at: new Date("2026-11-13T08:55:00Z"),
          item: "I",
          display: "HH-2",
          text: "「PR マージで完了」でステータスを Done に変更",
        },
      ],
    };
    expect(ruleRows(a, settings, members)[0]).toEqual({
      id: "R1",
      name: "PR マージで完了",
      summary: "リンクした PR がマージされたとき → ステータスを Done に",
      enabled: true,
    });
    expect(recentRows(a, new Date("2026-11-13T09:00:00Z"))).toEqual([
      { display: "HH-2", text: "「PR マージで完了」でステータスを Done に変更", time: "5 分前" },
    ]);
  });

  it("marks dedicated rules with their project's ID, and deleted projects", () => {
    const base = { ...newRuleForm(settings), enabled: true };
    const a = {
      rules: [
        { ...base, id: "R1", name: "全体" },
        { ...base, id: "R2", name: "生きている", scope: "P1", scopeLabel: "HH-P1" },
        { ...base, id: "R3", name: "消えた", scope: "P9", scopeLabel: "HH-P9", scopeDeleted: true },
      ],
      recent: [],
    };
    expect(ruleRows(a, settings, members).map((r) => r.scope)).toEqual([
      undefined,
      "HH-P1",
      "HH-P9(削除済み)",
    ]);
  });
});
