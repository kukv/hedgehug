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
    { ...base, id: "T1", kind: "Task", name: "調査", title: "調査: ", values: { priority: "pr1" } },
    { ...base, id: "T2", kind: "Task", name: "作業", title: "", values: {}, steps },
    { ...base, id: "B1", kind: "Bug", name: "不具合", title: "不具合: ", values: {} },
  ],
  templateDefaults: { Task: "T1", Bug: "B1" },
};
const ws2: WorkspaceSettings = {
  ...settings2,
  templates: [
    { ...base, id: "X1", kind: "Task", name: "別", title: "別: ", values: { assignee: "jiro" } },
  ],
  templateDefaults: { Task: "X1" },
};

async function setup() {
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
    workspaces: fakeWorkspaces({ "S1-W1": ws1, "S2-W1": ws2 }),
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

describe("create dialog templates", () => {
  it("applies the kind's default when it opens", async () => {
    const { dialog } = await setup();
    const s = get(dialog.state);
    expect(s?.form).toMatchObject({ kind: "Task", templateId: "T1", title: "調査: " });
    expect(s?.form.values.priority).toBe("pr1");
    expect(s?.templates.map((o) => o.label)).toEqual(["なし", "調査", "作業"]);
  });

  it("switching the kind drops the template and applies the new default; the title stays", async () => {
    const { dialog } = await setup();
    await dialog.update({ kind: "Bug" });
    expect(get(dialog.state)?.form).toMatchObject({ templateId: "B1", title: "調査: " });
    await dialog.update({ kind: "Story" });
    expect(get(dialog.state)?.form.templateId).toBe("");
    expect(get(dialog.state)?.templates).toEqual([]);
  });

  it("selecting a template overwrites its values and shows what is made with it", async () => {
    const { dialog } = await setup();
    const values = get(dialog.state)?.form.values ?? {};
    await dialog.update({ values: { ...values, assignee: "taro" } });
    await dialog.update({ templateId: "T2" });
    const s = get(dialog.state);
    expect(s?.form.templateId).toBe("T2");
    expect(s?.structure).toEqual({
      heading: "一緒に作るもの: Step 1 件",
      rows: [{ depth: 0, kind: "Step", title: "準備", assignee: "太郎" }],
    });
  });

  it("changing the workspace applies the new workspace's default after resetting values", async () => {
    const { dialog } = await setup();
    await dialog.update({ workspaceId: "S2-W1" });
    const s = get(dialog.state);
    expect(s?.form.templateId).toBe("X1");
    expect(s?.form.values.assignee).toBe("jiro");
    expect(s?.form.values.priority).toBe("np1");
  });

  it("sends the template, toasts the others and keeps it when continuing", async () => {
    const { dialog, items, shown } = await setup();
    await dialog.update({ templateId: "T2", title: "作業する", keepOpen: true });
    await dialog.submit();
    expect(items.createdTemplates.at(-1)).toBe("T2");
    expect(shown().at(-1)).toMatch(/^HH-\d+ ほか 1 件を作成しました$/);
    // 続けて作成: タイトルだけ空にし、テンプレートと「一緒に作るもの」は残す。書き出しは入れ直さない。
    const s = get(dialog.state);
    expect(s?.form).toMatchObject({ templateId: "T2", title: "" });
    expect(s?.structure?.rows).toHaveLength(1);
  });

  it("sends no template when none is chosen", async () => {
    const { dialog, items, shown } = await setup();
    await dialog.update({ templateId: "", title: "なしで作る" });
    await dialog.submit();
    expect(items.createdTemplates.at(-1)).toBeUndefined();
    expect(shown().at(-1)).toMatch(/^HH-\d+ を作成しました$/);
  });

  it("keeps the template when the project changes (B5 narrows it later)", async () => {
    const { dialog } = await setup();
    await dialog.update({ templateId: "T2" });
    await dialog.update({ projectId: "S1-P2" });
    expect(get(dialog.state)?.form.templateId).toBe("T2");
  });

  it("does not put the template's title back after continuing or a project change", async () => {
    const { dialog } = await setup();
    await dialog.update({ title: "調べる", keepOpen: true });
    await dialog.submit();
    expect(get(dialog.state)?.form).toMatchObject({ templateId: "T1", title: "" });
    await dialog.update({ projectId: "S1-P2" });
    expect(get(dialog.state)?.form).toMatchObject({ templateId: "T1", title: "" });
  });
});
