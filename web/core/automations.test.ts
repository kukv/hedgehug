import { describe, expect, it } from "vitest";
import {
  ISSUE_FIELDS,
  projectRuleRow,
  requiredNote,
  ruleTarget,
  ruleYaml,
  setLabel,
  statLabel,
  whenLabel,
  withField,
} from "./automations";
import type { Rule, RuleStat } from "./github";
import type { SchemaView } from "./schema";

const schema = {
  statuses: [
    { name: "todo", label: "未着手", category: "todo", default: true },
    { name: "in_progress", label: "進行中", category: "doing" },
    { name: "done", label: "完了", category: "done", default: true },
  ],
} as unknown as SchemaView;

const rule = (over: Partial<Rule> = {}): Rule => ({
  id: "r1",
  on: "pr.opened",
  enabled: true,
  disabled_in: [],
  ...over,
});
const ref = (ulid: string, title = "リリース") => ({ ulid, id: "REL-1", title });
const stat = (over: Partial<RuleStat> = {}): RuleStat => ({
  rule: "r1",
  applied: 0,
  rejected: 0,
  rejected_tasks: [],
  ...over,
});

describe("the words of a rule", () => {
  it("says what it sets", () => {
    expect(setLabel(rule({ status: "in_progress" }), schema)).toBe("ステータス → 進行中");
    expect(setLabel(rule({ category: "done" }), schema)).toBe("カテゴリ done の既定（完了）");
    expect(ruleTarget(rule({ category: "done" }), schema)).toBe("done");
  });

  it("words the condition", () => {
    expect(whenLabel("all_linked_prs_merged")).toBe("リンク中の PR がすべてマージ済み");
  });

  it("writes the rule as the file does", () => {
    expect(ruleYaml(rule())).toBe("id: r1 · on: pr.opened");
    expect(ruleYaml(rule({ on: "pr.merged", when: "all_linked_prs_merged" }))).toBe(
      "id: r1 · on: pr.merged · when: all_linked_prs_merged",
    );
    expect(ruleYaml(rule({ project: ref("P1") }))).toBe(
      "id: r1 · on: pr.opened · where: project:P1",
    );
  });

  it.each([
    [rule({ enabled: false }), stat({ applied: 3 }), "無効"],
    [rule(), undefined, "直近 7 日 0 回"],
    [rule(), stat({ applied: 5 }), "直近 7 日 5 回"],
    [rule(), stat({ applied: 5, rejected: 1 }), "直近 7 日 5 回 · 拒否 1"],
  ])("counts %#", (r, s, want) => {
    expect(statLabel(r, s)).toBe(want);
  });
});

describe("requiredNote", () => {
  const done = rule({ category: "done" });
  const task = { ulid: "T1", id: "REL-11", title: "x" };

  it("says nothing for a rule that nothing can stop", () => {
    expect(requiredNote(done, schema, [], undefined)).toBeNull();
  });

  it("says what the status needs", () => {
    expect(requiredNote(done, schema, ["points"], undefined)).toBe(
      "完了には points が必要です。points が空のタスクでは拒否され、通知に出ます。",
    );
  });

  it("adds the tasks it was refused on", () => {
    expect(
      requiredNote(done, schema, ["points"], stat({ rejected: 1, rejected_tasks: [task] })),
    ).toBe(
      "完了には points が必要です。points が空のタスクでは拒否され、通知に出ます。直近 7 日では REL-11 で、制約により 1 回拒否されました。",
    );
    expect(
      requiredNote(done, schema, [], stat({ rejected: 3, rejected_tasks: [task, task] })),
    ).toBe("直近 7 日では REL-11 ほか 1 件 で、制約により 3 回拒否されました。");
  });
});

describe("projectRuleRow", () => {
  const row = (r: Rule, whole?: Rule) => projectRuleRow(r, whole ?? r, "P1", "shared");

  it("a rule of the Store that runs here is stopped by the project's switch", () => {
    expect(row(rule())).toEqual({
      source: "shared",
      own: false,
      project: "P1",
      checked: true,
      fixed: false,
      state: "このプロジェクトで有効",
    });
  });

  it("a rule stopped here can be turned on again", () => {
    const r = rule({ enabled: false, disabled_in: [ref("P1")] });
    expect(row(r, rule())).toMatchObject({
      checked: false,
      fixed: false,
      state: "このプロジェクトで無効",
      project: "P1",
    });
  });

  it("a rule the Store turned off cannot be turned on here", () => {
    const off = rule({ enabled: false });
    expect(row(off)).toMatchObject({ checked: false, fixed: true, state: "shared で無効" });
  });

  it("a rule stopped in a project above cannot be turned on here", () => {
    const r = rule({ enabled: false, disabled_in: [ref("P0")] });
    expect(row(r, rule())).toMatchObject({ fixed: true, state: "親のプロジェクトで無効" });
  });

  it("a rule of the project uses its own switch", () => {
    const r = rule({ project: ref("P1") });
    expect(row(r)).toMatchObject({
      source: "このプロジェクトだけ",
      own: true,
      project: undefined,
      checked: true,
      state: "有効",
    });
  });

  it("names the project a rule above comes from", () => {
    expect(row(rule({ project: ref("P0", "親") })).source).toBe("親");
  });
});

describe("the fields of the issue sync", () => {
  it("has a row for every field", () => {
    expect(ISSUE_FIELDS.map((f) => f.label)).toEqual([
      "タイトル",
      "本文",
      "状態",
      "担当者",
      "マイルストーン",
    ]);
  });

  it("keeps the order of the table", () => {
    expect(withField(["status", "title"], "body", true)).toEqual(["title", "body", "status"]);
    expect(withField(["title", "body"], "title", false)).toEqual(["body"]);
  });
});
