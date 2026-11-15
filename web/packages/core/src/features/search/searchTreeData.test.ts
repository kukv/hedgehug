import { describe, expect, test, vi } from "vitest";
import type { Item } from "../../model/item";
import type { WorkspaceDetail } from "../../model/workspace";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { gate, item } from "../items/fixtures.test-util";
import { settings } from "../items/settings.test-util";
import { createSearchTreeData } from "./searchTreeData";

// 一覧は Store S1(書ける)・S2(閲覧のみ)。どちらもワークスペース W1 にプロジェクト P1・P2。
async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const items: Item[] = [
    item("S1", "S1-P1", 1, "Epic", null),
    item("S2", "S2-P2", 2, "Epic", null),
  ];
  const list = vi.fn(async (_s: string, projectId: string) =>
    items.filter((x) => x.projectId === projectId),
  );
  const get = vi.fn(
    async (storeId: string, workspaceId: string) =>
      ({ id: workspaceId, storeId, settings }) as unknown as WorkspaceDetail,
  );
  const members = vi.fn(async (storeId: string) => [{ login: storeId, name: storeId }]);
  const data = createSearchTreeData({ catalog, items: { list }, workspaces: { get }, members });
  return { data, list, get, members, items };
}

const all = [
  { storeId: "S1", projectId: "S1-P1" },
  { storeId: "S1", projectId: "S1-P2" },
  { storeId: "S2", projectId: "S2-P1" },
  { storeId: "S2", projectId: "S2-P2" },
];

const first = { storeId: "S1", projectId: "S1-P1" };

describe("createSearchTreeData", () => {
  test("loads each project once", async () => {
    const { data, list } = await setup();
    await Promise.all([data.load(all), data.load(all)]);
    await data.load(all);
    expect(list).toHaveBeenCalledTimes(4);
    expect(data.get("S1", "S1-P1")?.items.map((x) => x.id)).toEqual(["S1-I1"]);
    expect(data.get("S2", "S2-P2")?.settings).toBe(settings);
    expect(data.get("S2", "S2-P2")?.members).toEqual([{ login: "S2", name: "S2" }]);
    expect(data.get("S1", "none")).toBeUndefined();
  });

  test("settings once per workspace, members once per store", async () => {
    const { data, get, members } = await setup();
    await data.load(all);
    expect(get.mock.calls).toEqual([
      ["S1", "S1-W1"],
      ["S2", "S2-W1"],
    ]);
    expect(members.mock.calls).toEqual([["S1"], ["S2"]]);
  });

  test("reset drops in-flight results", async () => {
    const { data, list } = await setup();
    const g = gate();
    list.mockImplementationOnce(async () => {
      await g.wait;
      return [item("S1", "S1-P1", 9, "Epic", null)];
    });
    const loading = data.load([first]);
    data.reset();
    g.release();
    await loading;
    expect(data.get("S1", "S1-P1")).toBeUndefined();
    await data.load([first]);
    expect(data.get("S1", "S1-P1")?.items.map((x) => x.id)).toEqual(["S1-I1"]);
  });

  test.each(["list", "get", "members"] as const)(
    "failed project is retried on next load (%s)",
    async (which) => {
      const deps = await setup();
      deps[which].mockRejectedValueOnce(new Error(which));
      await deps.data.load([first]);
      expect(deps.data.get("S1", "S1-P1")).toBeUndefined();
      await deps.data.load([first]);
      expect(deps.data.get("S1", "S1-P1")?.items.map((x) => x.id)).toEqual(["S1-I1"]);
    },
  );
});
