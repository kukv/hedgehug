import { describe, expect, it } from "vitest";
import type { Item } from "../../model/item";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createItemDialog } from "./createDialog";
import { fakeItemsPort, item } from "./fixtures.test-util";

async function setup(here = {}) {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
  const task = item("S1", "S1-P1", 2, "Task", epic.id, "画面");
  const items = fakeItemsPort([epic, task]);
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const projectDialogs: string[] = [];
  const created: [Item, boolean][] = [];
  const dialog = createItemDialog({
    catalog,
    items,
    toasts,
    here: () => here,
    openProjectDialog: (ws) => void projectDialogs.push(ws),
    onCreated: (it, kept) => void created.push([it, kept]),
  });
  return { dialog, items, toasts, projectDialogs, created, epic, task };
}

describe("createItemDialog", () => {
  it("opens with a Task under the first Epic and the ID preview", async () => {
    const { dialog, epic } = await setup({ workspaceId: "S1-W1", projectId: "S1-P1" });
    await dialog.open();
    const s = get(dialog.state);
    expect(s?.form).toMatchObject({ projectId: "S1-P1", kind: "Task", parentId: epic.id });
    expect(s?.workspaces).toEqual([{ value: "S1-W1", label: "work / Web (HH)" }]);
    expect(s?.parents).toEqual([{ value: epic.id, label: "HH-1 認証" }]);
    expect(s?.preview).toBe("ID は HH-3 になります。");
  });

  it("re-picks the parent when the kind changes and keeps the title", async () => {
    const { dialog, task } = await setup({ workspaceId: "S1-W1", projectId: "S1-P1" });
    await dialog.open();
    await dialog.update({ title: "手順" });
    await dialog.update({ kind: "Step" });
    expect(get(dialog.state)?.form).toMatchObject({
      kind: "Step",
      parentId: task.id,
      title: "手順",
    });
    await dialog.update({ kind: "Epic" });
    expect(get(dialog.state)?.form.parentId).toBeNull();
    expect(get(dialog.state)?.parentMissing).toBe("");
  });

  it("shows the missing-parent words when there is no candidate", async () => {
    const { dialog } = await setup({ workspaceId: "S1-W1", projectId: "S1-P2" });
    await dialog.open();
    expect(get(dialog.state)?.parentMissing).toBe(
      "このプロジェクトには Epic がありません。先に作ってください",
    );
  });

  it("shows the server's refusal in the dialog", async () => {
    const { dialog } = await setup();
    await dialog.open({ kind: "Epic" });
    await dialog.submit();
    expect(get(dialog.state)?.error).toBe("タイトルを入力してください");
  });

  it("creates, toasts, and closes or keeps going", async () => {
    const { dialog, toasts, created } = await setup();
    await dialog.open({ kind: "Epic" });
    await dialog.update({ title: "決済", keepOpen: true });
    await dialog.submit();
    expect(get(toasts.list).map((t) => t.text)).toEqual(["HH-3 を作成しました"]);
    expect(get(dialog.state)?.form).toMatchObject({ title: "", keepOpen: true, kind: "Epic" });
    expect(get(dialog.state)?.preview).toBe("ID は HH-4 になります。");
    await dialog.update({ title: "通知", keepOpen: false });
    await dialog.submit();
    expect(get(dialog.state)).toBeNull();
    expect(created.map(([it, kept]) => [it.title, kept])).toEqual([
      ["決済", true],
      ["通知", false],
    ]);
  });

  it("does not open without a project and switches to the project dialog", async () => {
    const catalog = createCatalog({
      ...fakeCatalogPort(),
      listProjects: async () => [],
    });
    await catalog.load();
    const toasts = createToasts({ set: () => 0, clear: () => {} });
    const opened: string[] = [];
    const dialog = createItemDialog({
      catalog,
      items: fakeItemsPort(),
      toasts,
      here: () => ({}),
      openProjectDialog: (ws) => void opened.push(ws),
      onCreated: () => {},
    });
    await dialog.open();
    expect(get(dialog.state)).toBeNull();
    expect(get(toasts.list)[0]).toMatchObject({
      text: "Web にはプロジェクトがありません。先にプロジェクトを作ってください",
      error: true,
    });
    expect(opened).toEqual(["S1-W1"]);
  });
});
