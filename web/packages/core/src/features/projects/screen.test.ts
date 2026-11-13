import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakeItemsPort, item } from "../items/fixtures.test-util";
import { fakeWorkspaces } from "../items/settings.test-util";
import { createToasts } from "../toasts/toasts";
import { createProjectScreen, ownerName } from "./screen";

async function setup() {
  const port = fakeCatalogPort();
  const catalog = createCatalog(port);
  await catalog.load();
  const epic = { ...item("S1", "S1-P1", 1, "Epic", null), rank: "W" };
  const first = { ...item("S1", "S1-P1", 3, "Epic", null), rank: "V" };
  const task = item("S1", "S1-P1", 2, "Task", epic.id);
  const items = fakeItemsPort([epic, task, first]);
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  let changed = 0;
  const screen = createProjectScreen(
    catalog,
    port,
    items,
    fakeWorkspaces(),
    toasts,
    () => void changed++,
  );
  return { screen, items, toasts, changed: () => changed };
}

describe("createProjectScreen", () => {
  it("loads the project with its tree and epics in sibling order", async () => {
    const { screen } = await setup();
    await screen.load("S1", "S1-P1");
    const v = get(screen.view);
    expect(v?.project.displayId).toBe("HH-P1");
    expect(v?.workspace.prefix).toBe("HH");
    expect(v?.epics.map((e) => e.number)).toEqual([3, 1]);
    expect(v?.tree.map((n) => n.item.number)).toEqual([3, 1]);
    expect(v?.tree[1]?.children.map((n) => n.item.number)).toEqual([2]);
    expect(v?.members).toEqual([{ login: "taro", name: "太郎" }]);
  });

  it("loads the workspace settings", async () => {
    const { screen } = await setup();
    await screen.load("S1", "S1-P1");
    expect(get(screen.view)?.settings.initialStatus).toBe("st1");
  });

  it("changes a status and reloads, showing a refusal as a toast", async () => {
    const { screen, items, toasts, changed } = await setup();
    await screen.load("S1", "S1-P1");
    await screen.setStatus("S1-I2", "st2");
    expect(items.calls).toEqual(['setProperty S1-I2 status "st2"']);
    expect(get(screen.view)?.items.find((it) => it.id === "S1-I2")?.status).toBe("st2");
    expect(changed()).toBe(1);
    items.refuseNext("テストの断り");
    await screen.setStatus("S1-I2", "st3");
    expect(get(toasts.list)[0]?.error).toBe(true);
    expect(changed()).toBe(1);
  });

  it("does not reload or notify when the screen moved on during the save", async () => {
    const { screen, items, changed } = await setup();
    await screen.load("S1", "S1-P1");
    const save = items.setProperty.bind(items);
    items.setProperty = async (...a) => {
      await screen.load("S1", "nope");
      return save(...a);
    };
    await screen.setStatus("S1-I2", "st2");
    expect(get(screen.view)).toBeNull();
    expect(changed()).toBe(0);
  });

  // 後から始めた読み直しに結果を譲ったときは、その読み直しが終わるまで返さない。返すと、呼んだ側(キーの順番待ちなど)が
  // 古い行で次の操作を始める。
  it("does not return before a newer load that superseded it has finished", async () => {
    const { screen, items } = await setup();
    await screen.load("S1", "S1-P1");
    const list = items.list.bind(items);
    let release = () => {};
    const gate = new Promise<void>((r) => {
      release = r;
    });
    let calls = 0;
    items.list = async (...a) => {
      calls += 1;
      const n = calls;
      const out = await list(...a);
      if (n === 2) await gate;
      return out;
    };
    let olderDone = false;
    const older = screen.reload().then(() => {
      olderDone = true;
    });
    const newer = screen.reload();
    for (let i = 0; i < 20; i++) await Promise.resolve();
    expect(olderDone).toBe(false);
    release();
    await Promise.all([older, newer]);
    expect(olderDone).toBe(true);
  });

  it("reloads the shown project and is null for a project not in the catalog", async () => {
    const { screen, items } = await setup();
    await screen.load("S1", "S1-P1");
    await items.create("S1", {
      projectId: "S1-P1",
      parentId: null,
      kind: "Epic",
      title: "新しい",
      values: {},
    });
    await screen.reload();
    expect(get(screen.view)?.items).toHaveLength(4);
    await screen.load("S1", "nope");
    expect(get(screen.view)).toBeNull();
  });
});

describe("ownerName", () => {
  it("shows なし, the member's name, or 不明なユーザー", () => {
    const members = [{ login: "taro", name: "太郎" }];
    expect(ownerName("", members)).toBe("なし");
    expect(ownerName("Taro", members)).toBe("太郎");
    expect(ownerName("jiro", members)).toBe("不明なユーザー");
  });
});
