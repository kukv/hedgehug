import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakePlanningPort, overviewOf, sprintOf } from "../planning/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createItemDialog } from "./createDialog";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces, settings2 } from "./settings.test-util";

// ワークスペースの切り替えと値の連動(02「ダイアログの中での連動」、決定 25)。S2-W1 は別の Store の設定 settings2。
async function setupSwitch(members2: { login: string; name: string }[]) {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const items = fakeItemsPort([item("S1", "S1-P1", 1, "Epic", null, "認証")]);
  const dialog = createItemDialog({
    catalog,
    items,
    toasts: createToasts({ set: () => 0, clear: () => {} }),
    here: () => ({ workspaceId: "S1-W1", projectId: "S1-P1" }),
    openProjectDialog: () => {},
    workspaces: fakeWorkspaces({ "S2-W1": settings2 }),
    catalogPort: {
      ...fakeCatalogPort(),
      listMembers: async (s) => (s === "S2" ? members2 : [{ login: "taro", name: "太郎" }]),
    },
    planning: fakePlanningPort({
      "S1-W1": overviewOf([
        sprintOf({ id: "SP1", name: "Sprint 1", state: "active" }),
        sprintOf({ id: "SP0", name: "Sprint 0", state: "closed" }),
      ]),
      "S2-W1": overviewOf([sprintOf({ id: "SP9", name: "Sprint 9" })]),
    }),
    onCreated: () => {},
  });
  await dialog.open();
  return Object.assign(dialog, { items });
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

// スプリントの欄(02「ダイアログの項目」「ダイアログの中での連動」、F1-21)。
describe("create dialog sprint", () => {
  it("starts at the backlog with the sprints not closed of the workspace", async () => {
    const dialog = await setupSwitch([]);
    expect(get(dialog.state)?.form.sprint).toBe("");
    expect(get(dialog.state)?.sprints).toEqual([
      { value: "", label: "バックログ" },
      { value: "SP1", label: "Sprint 1(実行中)" },
    ]);
  });

  it("goes back to the backlog on Epic and stays there on Story", async () => {
    const dialog = await setupSwitch([]);
    await dialog.update({ sprint: "SP1" });
    await dialog.update({ kind: "Bug" });
    expect(get(dialog.state)?.form.sprint).toBe("SP1");
    await dialog.update({ kind: "Epic" });
    expect(get(dialog.state)?.form.sprint).toBe("");
    await dialog.update({ kind: "Story" });
    expect(get(dialog.state)?.form.sprint).toBe("");
  });

  it("goes back to the backlog when the workspace changes, even if picked while loading", async () => {
    const dialog = await setupSwitch([{ login: "taro", name: "太郎" }]);
    const switching = dialog.update({ workspaceId: "S2-W1" });
    await dialog.update({ sprint: "SP1" });
    await switching;
    expect(get(dialog.state)?.form.sprint).toBe("");
    expect(get(dialog.state)?.sprints.map((o) => o.value)).toEqual(["", "SP9"]);
  });

  it("keeps the sprint picked while the project reloads", async () => {
    const dialog = await setupSwitch([]);
    const reloading = dialog.update({ projectId: "S1-P2" });
    await dialog.update({ sprint: "SP1" });
    await reloading;
    expect(get(dialog.state)?.form.sprint).toBe("SP1");
  });

  it("creates the item in the picked sprint", async () => {
    const dialog = await setupSwitch([]);
    await dialog.update({ title: "画面", sprint: "SP1" });
    await dialog.submit();
    expect(dialog.items.all.find((it) => it.title === "画面")?.sprint).toBe("SP1");
  });
});
