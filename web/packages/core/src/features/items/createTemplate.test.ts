import { describe, expect, it } from "vitest";
import type { WorkspaceSettings, WorkspaceTemplate } from "../../model/workspace";
import {
  applyDefault,
  applyToForm,
  othersOf,
  structureOf,
  type TemplateForm,
  templateOptions,
} from "./createTemplate";
import { settings } from "./settings.test-util";

const tpl = (t: Partial<WorkspaceTemplate> & { id: string; kind: string }): WorkspaceTemplate => ({
  name: t.id,
  title: "",
  description: "",
  values: {},
  children: [],
  steps: [],
  ...t,
});

// T1: Task(書き出し・優先度・F2・ポイント)。T2: Task(担当者・Step 2 件)。E1: Epic(子 2 件・Step 1 件)。
const t1 = tpl({
  id: "T1",
  kind: "Task",
  name: "調査",
  title: "調査: ",
  values: { priority: "pr1", F2: "メモ", points: 3 },
});
const t2 = tpl({
  id: "T2",
  kind: "Task",
  name: "作業",
  values: { assignee: "hanako" },
  steps: [
    { id: "s1", title: "準備", assignee: "@parent" },
    { id: "s2", title: "記録", assignee: "@none" },
  ],
});
const e1 = tpl({
  id: "E1",
  kind: "Epic",
  name: "リリース",
  children: [
    {
      id: "c1",
      kind: "Story",
      title: "QA",
      description: "",
      values: { assignee: "taro" },
      steps: [{ id: "s3", title: "確認", assignee: "@parent" }],
    },
    { id: "c2", kind: "Task", title: "本番反映", description: "", values: {}, steps: [] },
  ],
});
const withTemplates: WorkspaceSettings = {
  ...settings,
  templates: [t1, t2, e1],
  templateDefaults: { Task: "T1" },
};
const form = (f: Partial<TemplateForm> = {}): TemplateForm => ({
  kind: "Task",
  title: "",
  values: { status: "st1", priority: "pr2" },
  templateId: "",
  description: "",
  templateDescription: "",
  ...f,
});

describe("templateOptions", () => {
  it("lists the kind's templates after なし, and nothing when there are none", () => {
    expect(templateOptions(withTemplates, "Task")).toEqual([
      { value: "", label: "なし" },
      { value: "T1", label: "調査" },
      { value: "T2", label: "作業" },
    ]);
    expect(templateOptions(withTemplates, "Bug")).toEqual([]);
  });
});

describe("applyToForm", () => {
  it("fills an empty title and overwrites only the template's shown values", () => {
    const out = applyToForm(
      form({ values: { status: "st2", priority: "pr2", F2: "前" } }),
      withTemplates,
      "T1",
    );
    // ポイントは Story だけのプロパティなので Task には当てはめない(03「その種類で使わないプロパティは適用しない」)。
    expect(out).toEqual(
      form({
        title: "調査: ",
        templateId: "T1",
        values: { status: "st2", priority: "pr1", F2: "メモ" },
      }),
    );
  });
  it("keeps a typed title", () => {
    expect(applyToForm(form({ title: "ログイン" }), withTemplates, "T1").title).toBe("ログイン");
  });
  it("turns template numbers into input text (A3a 決定 12)", () => {
    const out = applyToForm(
      form({ kind: "Story" }),
      { ...withTemplates, templates: [{ ...t1, kind: "Story" }] },
      "T1",
    );
    expect(out.values.points).toBe("3");
  });
  it("switching to なし keeps the input", () => {
    const applied = applyToForm(form(), withTemplates, "T1");
    expect(applyToForm(applied, withTemplates, "")).toEqual({ ...applied, templateId: "" });
  });
});

describe("applyDefault", () => {
  it("drops the current template and applies the kind's default", () => {
    expect(applyDefault(form({ templateId: "T2" }), withTemplates).templateId).toBe("T1");
    expect(applyDefault(form({ kind: "Bug", templateId: "T2" }), withTemplates).templateId).toBe(
      "",
    );
  });
});

describe("structureOf and othersOf", () => {
  it("shows the children and steps with the assignee each would get", () => {
    expect(
      structureOf(withTemplates, form({ templateId: "T2", values: { assignee: "taro" } }), [
        { login: "taro", name: "太郎" },
      ]),
    ).toEqual({
      heading: "一緒に作るもの: Step 2 件",
      rows: [
        { depth: 0, kind: "Step", title: "準備", assignee: "太郎" },
        { depth: 0, kind: "Step", title: "記録", assignee: "未割り当て" },
      ],
    });
    expect(structureOf(withTemplates, form({ templateId: "T1" }), [])).toBeNull();
  });

  it("passes the settings through: a hidden assignee gives the child and its 親と同じ Step 未割り当て", () => {
    const hidden: WorkspaceSettings = {
      ...withTemplates,
      properties: withTemplates.properties.map((p) =>
        p.id === "assignee" ? { ...p, hidden: true } : p,
      ),
    };
    const members = [{ login: "taro", name: "太郎" }];
    expect(structureOf(hidden, form({ kind: "Epic", templateId: "E1" }), members)?.rows).toEqual([
      { depth: 0, kind: "Story", title: "QA", assignee: "未割り当て" },
      { depth: 1, kind: "Step", title: "確認", assignee: "未割り当て" },
      { depth: 0, kind: "Task", title: "本番反映", assignee: "未割り当て" },
    ]);
  });

  it.each([
    ["hidden", { hidden: true }],
    ["not used by the form's kind", { kinds: ["Story", "Bug"] }],
  ])("gives the root's 親と同じ Step 未割り当て when the assignee field is %s", (_, extra) => {
    const s: WorkspaceSettings = {
      ...withTemplates,
      properties: withTemplates.properties.map((p) =>
        p.id === "assignee" ? { ...p, ...extra } : p,
      ),
    };
    const out = structureOf(s, form({ templateId: "T2", values: { assignee: "taro" } }), [
      { login: "taro", name: "太郎" },
    ]);
    expect(out?.rows[0]).toEqual({ depth: 0, kind: "Step", title: "準備", assignee: "未割り当て" });
  });

  it("counts the other templates", () => {
    expect(othersOf(withTemplates, "E1")).toBe(3);
    expect(othersOf(withTemplates, "")).toBe(0);
  });
});

// 説明のあるテンプレート: D1・D2 は Task、DB は Bug。
const d1 = tpl({ id: "D1", kind: "Task", description: "## 手順" });
const d2 = tpl({ id: "D2", kind: "Task", description: "## 別の手順" });
const dBug = tpl({ id: "DB", kind: "Bug", description: "## 概要" });
const withDescriptions: WorkspaceSettings = {
  ...settings,
  templates: [d1, d2, dBug],
  templateDefaults: { Task: "D1", Bug: "DB" },
};

describe("the template's description", () => {
  it("select replaces whatever is typed, and remembers the template's description", () => {
    const out = applyToForm(form({ description: "メモ" }), withDescriptions, "D1");
    expect(out).toMatchObject({
      description: "## 手順",
      templateDescription: "## 手順",
      templateId: "D1",
    });
    expect(applyToForm(out, withDescriptions, "D2").description).toBe("## 別の手順");
  });

  it("the default fills an empty description, keeps a typed one, replaces the template's own", () => {
    const filled = applyDefault(form(), withDescriptions);
    expect(filled.description).toBe("## 手順");
    // 種類を Bug に切り替えた: テンプレート由来なので、新しい既定の説明に置き換わる。
    expect(applyDefault({ ...filled, kind: "Bug" }, withDescriptions).description).toBe("## 概要");
    // 1 文字足していれば、利用者の入力として残る。
    const typed = { ...filled, kind: "Bug" as const, description: "## 手順!" };
    expect(applyDefault(typed, withDescriptions)).toMatchObject({
      description: "## 手順!",
      templateDescription: "## 概要",
      templateId: "DB",
    });
  });

  it("compares with the template that was applied, even when the new settings do not have it", () => {
    // ワークスペースを変えた: 前のワークスペースのテンプレート D1 は、新しい設定にない。
    const applied = applyDefault(form(), withDescriptions);
    const other: WorkspaceSettings = {
      ...settings,
      templates: [tpl({ id: "X1", kind: "Task", description: "## 別" })],
      templateDefaults: { Task: "X1" },
    };
    expect(applyDefault(applied, other).description).toBe("## 別");
  });

  it("なし clears a template's description and keeps a typed one", () => {
    const applied = applyToForm(form(), withDescriptions, "D1");
    expect(applyToForm(applied, withDescriptions, "")).toMatchObject({
      description: "",
      templateDescription: "",
      templateId: "",
    });
    expect(applyToForm({ ...applied, description: "メモ" }, withDescriptions, "")).toMatchObject({
      description: "メモ",
      templateId: "",
    });
  });
});
