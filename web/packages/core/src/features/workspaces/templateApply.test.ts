import { describe, expect, it } from "vitest";
import type { WorkspaceTemplate } from "../../model/workspace";
import { applyTemplate, nextDescription, templateStructure } from "./templateApply";

const tpl = (id: string, extra: Partial<WorkspaceTemplate>): WorkspaceTemplate => ({
  id,
  name: id,
  kind: "Task",
  title: "",
  description: "",
  values: {},
  children: [],
  steps: [],
  ...extra,
});
const taskT = tpl("task", {
  title: "調査: ",
  description: "## 手順",
  values: { priority: "high", assignee: "taro" },
});
const bugT = tpl("bug", { kind: "Bug", description: "## 概要" });
const all = () => true;
const empty = { title: "", description: "", values: {}, template: null };

describe("nextDescription", () => {
  const next = taskT; // 説明は "## 手順"
  it("auto: fills an empty description, keeps a typed one, replaces the applied template's own", () => {
    expect(nextDescription("", "", next, "auto")).toBe("## 手順");
    expect(nextDescription("メモ", "", next, "auto")).toBe("メモ");
    expect(nextDescription("## 概要", "## 概要", next, "auto")).toBe("## 手順");
    expect(nextDescription("## 概要!", "## 概要", next, "auto")).toBe("## 概要!");
  });

  it("select: always the template's description", () => {
    expect(nextDescription("メモ", "", next, "select")).toBe("## 手順");
  });

  it("なし: clears a template's description and keeps a typed one", () => {
    expect(nextDescription("## 手順", "## 手順", null, "select")).toBe("");
    expect(nextDescription("メモ", "## 手順", null, "select")).toBe("メモ");
    expect(nextDescription("", "", null, "auto")).toBe("");
  });
});

describe("apply template", () => {
  it("fills an empty title and keeps a typed one", () => {
    expect(applyTemplate(empty, taskT, "auto", all).title).toBe("調査: ");
    expect(applyTemplate({ ...empty, title: "ログイン" }, taskT, "select", all).title).toBe(
      "ログイン",
    );
  });

  it("auto: puts the description only when empty or from the current template", () => {
    const applied = applyTemplate(empty, taskT, "auto", all);
    expect(applied.description).toBe("## 手順");
    // 種類を Bug に切り替えた: テンプレート由来なので置き換わる。
    expect(applyTemplate(applied, bugT, "auto", all).description).toBe("## 概要");
    // 1 文字でも足していれば残る。
    expect(
      applyTemplate({ ...applied, description: "## 手順!" }, bugT, "auto", all).description,
    ).toBe("## 手順!");
  });

  it("select: replaces the description; なし clears only a template's description", () => {
    const typed = { ...empty, description: "メモ" };
    expect(applyTemplate(typed, taskT, "select", all).description).toBe("## 手順");
    const applied = applyTemplate(empty, taskT, "select", all);
    expect(applyTemplate(applied, null, "select", all).description).toBe("");
    expect(
      applyTemplate({ ...applied, description: "メモ" }, null, "select", all).description,
    ).toBe("メモ");
  });

  it("overrides only the values the template has and the dialog shows", () => {
    const state = { ...empty, values: { priority: "low", systems: ["web"] } };
    const got = applyTemplate(state, taskT, "select", (id) => id !== "assignee");
    expect(got.values).toEqual({ priority: "high", systems: ["web"] });
    expect(got.template?.id).toBe("task");
  });
});

describe("template structure", () => {
  const members = [
    { login: "taro", name: "田中" },
    { login: "hanako", name: "佐藤" },
  ];

  it("lists children and their steps for an Epic", () => {
    const epic = tpl("epic", {
      kind: "Epic",
      children: [
        {
          id: "c1",
          kind: "Story",
          title: "QA",
          description: "",
          values: { assignee: "hanako" },
          steps: [
            { id: "s1", title: "確認", assignee: "@parent" },
            { id: "s2", title: "記録", assignee: "@none" },
          ],
        },
        { id: "c2", kind: "Task", title: "本番", description: "", values: {}, steps: [] },
      ],
    });
    expect(templateStructure(epic, "taro", members)).toEqual({
      heading: "一緒に作るもの: 子 2 件・Step 2 件",
      rows: [
        { depth: 0, kind: "Story", title: "QA", assignee: "佐藤" },
        { depth: 1, kind: "Step", title: "確認", assignee: "佐藤" },
        { depth: 1, kind: "Step", title: "記録", assignee: "未割り当て" },
        { depth: 0, kind: "Task", title: "本番", assignee: "未割り当て" },
      ],
    });
  });

  it("uses the dialog's assignee for a Task's 親と同じ", () => {
    const t = tpl("t", { steps: [{ id: "s1", title: "再現", assignee: "@parent" }] });
    expect(templateStructure(t, "taro", members)?.rows).toEqual([
      { depth: 0, kind: "Step", title: "再現", assignee: "田中" },
    ]);
    expect(templateStructure(tpl("n", {}), "taro", members)).toBeNull();
  });
});
