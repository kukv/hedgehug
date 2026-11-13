import { describe, expect, it } from "vitest";
import type { WorkspaceSettings } from "../../model/workspace";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createItemDialog } from "./createDialog";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces, settings, settings2 } from "./settings.test-util";

// 作成ダイアログのテンプレート(02「開き方と初期値」「ダイアログの中での連動」「作成したあと」)。
const steps = [{ id: "s1", title: "準備", assignee: "@parent" }];
const base = { description: "", children: [], steps: [] as typeof steps };
const ws1: WorkspaceSettings = {
  ...settings,
  templates: [
    {
      ...base,
      id: "T1",
      kind: "Task",
      name: "調査",
      title: "調査: ",
      description: "## 調査の手順",
      values: { priority: "pr1" },
    },
    {
      ...base,
      id: "T2",
      kind: "Task",
      name: "作業",
      title: "",
      description: "## 作業の手順",
      values: {},
      steps,
    },
    {
      ...base,
      id: "B1",
      kind: "Bug",
      name: "不具合",
      title: "不具合: ",
      description: "## 不具合の概要",
      values: {},
    },
  ],
  templateDefaults: { Task: "T1", Bug: "B1" },
};
const ws2: WorkspaceSettings = {
  ...settings2,
  templates: [
    {
      ...base,
      id: "X1",
      kind: "Task",
      name: "別",
      title: "別: ",
      description: "## 別の手順",
      values: { assignee: "jiro" },
    },
  ],
  templateDefaults: { Task: "X1" },
};

async function setup(s1: WorkspaceSettings = ws1) {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const items = fakeItemsPort([item("S1", "S1-P1", 1, "Epic", null, "認証")]);
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const dialog = createItemDialog({
    catalog,
    items,
    toasts,
    here: () => ({ workspaceId: "S1-W1", projectId: "S1-P1" }),
    openProjectDialog: () => {},
    workspaces: fakeWorkspaces({ "S1-W1": s1, "S2-W1": ws2 }),
    catalogPort: {
      ...fakeCatalogPort(),
      listMembers: async () => [
        { login: "taro", name: "太郎" },
        { login: "jiro", name: "次郎" },
      ],
    },
    onCreated: () => {},
  });
  await dialog.open();
  const shown = () => get(toasts.list).map((t) => t.text);
  return { dialog, items, shown };
}

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
});
