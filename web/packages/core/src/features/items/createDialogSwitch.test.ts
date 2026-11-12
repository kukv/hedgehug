import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createItemDialog } from "./createDialog";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces, settings2 } from "./settings.test-util";

// ワークスペースの切り替えと値の連動(02「ダイアログの中での連動」、決定 25)。S2-W1 は別の Store の設定 settings2。
async function setupSwitch(members2: { login: string; name: string }[]) {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const dialog = createItemDialog({
    catalog,
    items: fakeItemsPort([item("S1", "S1-P1", 1, "Epic", null, "認証")]),
    toasts: createToasts({ set: () => 0, clear: () => {} }),
    here: () => ({ workspaceId: "S1-W1", projectId: "S1-P1" }),
    openProjectDialog: () => {},
    workspaces: fakeWorkspaces({ "S2-W1": settings2 }),
    catalogPort: {
      ...fakeCatalogPort(),
      listMembers: async (s) => (s === "S2" ? members2 : [{ login: "taro", name: "太郎" }]),
    },
    onCreated: () => {},
  });
  await dialog.open();
  return dialog;
}

const typed = {
  status: "st2",
  priority: "pr1",
  systems: ["sy1"],
  F1: "C1",
  assignee: "taro",
  start: "2026-11-11",
  end: "2026-11-20",
  points: "3",
  estimate: "2",
};

const kept = {
  status: "ns1",
  priority: "np1",
  assignee: "taro",
  start: "2026-11-11",
  end: "2026-11-20",
  points: "3",
  estimate: "2",
};

describe("create dialog workspace switch", () => {
  it("resets status, priority, systems and custom values and keeps the rest", async () => {
    const dialog = await setupSwitch([{ login: "TARO", name: "太郎" }]);
    await dialog.update({ values: typed });
    await dialog.update({ workspaceId: "S2-W1" });
    expect(get(dialog.state)?.form.values).toEqual(kept);
    expect(get(dialog.state)?.settings).toBe(settings2);
  });

  it("drops the assignee who is not a member of the other store", async () => {
    const dialog = await setupSwitch([{ login: "jiro", name: "次郎" }]);
    await dialog.update({ values: typed });
    await dialog.update({ workspaceId: "S2-W1" });
    expect(get(dialog.state)?.form.values.assignee).toBeUndefined();
    expect(get(dialog.state)?.form.values.start).toBe("2026-11-11");
  });

  it("applies the switch to values typed while the workspace switch is loading", async () => {
    const dialog = await setupSwitch([{ login: "taro", name: "太郎" }]);
    const switching = dialog.update({ workspaceId: "S2-W1" });
    await dialog.update({ values: typed });
    await switching;
    expect(get(dialog.state)?.form.values).toEqual(kept);
  });
});
