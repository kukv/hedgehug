import { describe, expect, it } from "vitest";
import type { WorkspaceSettings } from "../../model/workspace";
import { get } from "../../reactive/store";
import { setup, ws1 } from "./createDialogTemplate.test-util";

// プロジェクトを変えたときの入力の絞り込み(02「プロジェクトを変えたとき」、B5a-21)。
describe("create dialog project change", () => {
  // P2 は、InProgress を使わず、テンプレートは T2 だけ(Task の既定も T2)。
  const withRules = (over: Partial<WorkspaceSettings["projectRules"][string]> = {}) => ({
    ...ws1,
    categoryDefaults: { todo: "st1", done: "st3", canceled: "st4" },
    projectRules: {
      "S1-P2": {
        hiddenStatuses: ["st2"],
        workflow: {},
        templates: ["T2"],
        templateDefaults: { Task: "T2" as const },
        disabledRules: [],
        ...over,
      },
    },
  });

  it("keeps the template when the new project can use it", async () => {
    const { dialog } = await setup(withRules());
    await dialog.update({ templateId: "T2" });
    await dialog.update({ projectId: "S1-P2" });
    expect(get(dialog.state)?.form.templateId).toBe("T2");
  });

  it("drops a template the new project cannot use and applies that project's default", async () => {
    const { dialog } = await setup(withRules());
    expect(get(dialog.state)?.form).toMatchObject({
      templateId: "T1",
      description: "## 調査の手順",
    });
    await dialog.update({ projectId: "S1-P2" });
    const s = get(dialog.state);
    expect(s?.form).toMatchObject({ templateId: "T2", description: "## 作業の手順" });
    expect(s?.templates.map((o) => o.label)).toEqual(["なし", "作業"]);
  });

  it("empties the template's description when the new project has no default; a typed one stays", async () => {
    const none = withRules({ templateDefaults: { Task: "@none" } });
    const a = await setup(none);
    await a.dialog.update({ projectId: "S1-P2" });
    expect(get(a.dialog.state)?.form).toMatchObject({ templateId: "", description: "" });
    const b = await setup(none);
    await b.dialog.update({ description: "自分で書いた" });
    await b.dialog.update({ projectId: "S1-P2" });
    expect(get(b.dialog.state)?.form).toMatchObject({
      templateId: "",
      description: "自分で書いた",
    });
  });

  it("keeps a typed description when another template is applied for the new project", async () => {
    const { dialog } = await setup(withRules());
    await dialog.update({ description: "自分で書いた" });
    await dialog.update({ projectId: "S1-P2" });
    expect(get(dialog.state)?.form).toMatchObject({
      templateId: "T2",
      description: "自分で書いた",
    });
  });

  it("puts the status back to the initial one when the new project does not use it", async () => {
    const { dialog } = await setup(withRules());
    const values = get(dialog.state)?.form.values ?? {};
    await dialog.update({ values: { ...values, status: "st2" } });
    await dialog.update({ projectId: "S1-P2" });
    const s = get(dialog.state);
    expect(s?.form.values.status).toBe("st1");
    const status = s?.rows.find((r) => r.property.id === "status");
    expect(status?.options.map((o) => o.id)).toEqual(["st1", "st3", "st4"]);
  });

  // 開いたプロジェクト(P1)自身が既定を持つとき
  const own = (templateDefaults: Record<string, string>) => ({
    ...ws1,
    projectRules: {
      "S1-P1": {
        hiddenStatuses: [],
        workflow: {},
        templates: [],
        templateDefaults,
        disabledRules: [],
      },
    },
  });

  it("applies the project's default template when it opens", async () => {
    const { dialog } = await setup(own({ Task: "T2" }));
    expect(get(dialog.state)?.form).toMatchObject({
      templateId: "T2",
      description: "## 作業の手順",
    });
  });

  it("applies the project's default for the new kind when the kind changes", async () => {
    const { dialog } = await setup(own({ Bug: "@none" }));
    await dialog.update({ kind: "Bug" });
    expect(get(dialog.state)?.form).toMatchObject({ templateId: "", description: "" });
  });
});
