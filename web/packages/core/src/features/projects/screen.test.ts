import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakeItemsPort, item } from "../items/fixtures.test-util";
import { createProjectScreen, ownerName } from "./screen";

async function setup() {
  const port = fakeCatalogPort();
  const catalog = createCatalog(port);
  await catalog.load();
  const epic = { ...item("S1", "S1-P1", 1, "Epic", null), rank: "W" };
  const first = { ...item("S1", "S1-P1", 3, "Epic", null), rank: "V" };
  const task = item("S1", "S1-P1", 2, "Task", epic.id);
  const items = fakeItemsPort([epic, task, first]);
  return { screen: createProjectScreen(catalog, port, items), items };
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
