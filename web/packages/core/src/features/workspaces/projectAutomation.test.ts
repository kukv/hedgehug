import { describe, expect, it } from "vitest";
import type { AutomationRule, WorkspaceAutomation } from "../../model/workspace";
import { projectRuleRows } from "./automation";
import { P, sample } from "./projectRules.fixtures.test-util";

const rule = (id: string, over: Partial<AutomationRule> = {}): AutomationRule => ({
  id,
  name: id,
  trigger: "pr_merged",
  triggerStatus: "",
  allMerged: false,
  action: "status",
  value: "st4",
  enabled: true,
  ...over,
});
const auto = (rs: AutomationRule[]): WorkspaceAutomation => ({ rules: rs, recent: [] });
const rows = (rs: AutomationRule[], r = {}) => projectRuleRows(auto(rs), sample(r), [], P);

describe("projectRuleRows", () => {
  it("lists workspace rules and this project's rules, not other projects'", () => {
    const got = rows([rule("R1"), rule("R2", { scope: P }), rule("R3", { scope: "p2" })]);
    expect(got.map((r) => r.id)).toEqual(["R1", "R2"]);
  });
  it("writes the origin and the summary", () => {
    const got = rows([rule("R1"), rule("R2", { scope: P })]);
    expect(got.map((r) => r.origin)).toEqual(["ワークスペース", "このプロジェクトだけ"]);
    expect(got.map((r) => r.own)).toEqual([false, true]);
    expect(got[0]?.summary).toBe("リンクした PR がマージされたとき → ステータスを 完了 に");
  });
  it("turns a workspace rule off when the project excludes it", () => {
    const got = rows([rule("R1"), rule("R2")], { disabledRules: ["R2"] });
    expect(got.map((r) => r.enabled)).toEqual([true, false]);
    expect(got.map((r) => r.locked)).toEqual([false, false]);
  });
  it("notes and locks a rule disabled in the workspace", () => {
    const [r] = rows([rule("R1", { enabled: false })]);
    expect(r).toMatchObject({ enabled: false, locked: true, note: "ワークスペースで無効" });
  });
  it("uses the rule's own switch for a dedicated rule", () => {
    const got = rows([rule("R1", { scope: P, enabled: false }), rule("R2", { scope: P })]);
    expect(got.map((r) => [r.enabled, r.locked, r.note])).toEqual([
      [false, false, undefined],
      [true, false, undefined],
    ]);
  });
  it("is empty without rules", () => {
    expect(rows([])).toEqual([]);
  });
});
