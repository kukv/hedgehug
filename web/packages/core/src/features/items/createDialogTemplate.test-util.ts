import type { WorkspaceSettings } from "../../model/workspace";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createItemDialog } from "./createDialog";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces, settings, settings2 } from "./settings.test-util";

// 作成ダイアログのテンプレート(02「開き方と初期値」「ダイアログの中での連動」「作成したあと」)。
export const steps = [{ id: "s1", title: "準備", assignee: "@parent" }];
const base = { description: "", children: [], steps: [] as typeof steps };
export const ws1: WorkspaceSettings = {
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
export const ws2: WorkspaceSettings = {
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

export async function setup(s1: WorkspaceSettings = ws1) {
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
