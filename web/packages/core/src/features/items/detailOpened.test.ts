import { describe, expect, it } from "vitest";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakePlanningPort } from "../planning/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createDetail } from "./detail";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces } from "./settings.test-util";

// 開けたあとに onOpened を 1 回呼ぶ(最近開いたアイテムの記録。A5c-13)。見つからないときは呼ばない。
async function setup() {
  const opened: [string, string][] = [];
  const catalogPort = fakeCatalogPort();
  const catalog = createCatalog(catalogPort);
  await catalog.load();
  const task = item("S1", "S1-P1", 2, "Task", null, "画面");
  const detail = createDetail({
    planning: fakePlanningPort(),
    catalog,
    items: fakeItemsPort([task]),
    toasts: createToasts({ set: () => 0, clear: () => {} }),
    workspaces: fakeWorkspaces(),
    catalogPort,
    onChanged: () => {},
    onOpened: (s, i) => void opened.push([s, i]),
  });
  return { detail, opened, task };
}

describe("detail onOpened", () => {
  it("is called once after the item opens", async () => {
    const { detail, opened, task } = await setup();
    await detail.open("S1", task.id);
    expect(opened).toEqual([["S1", task.id]]);
  });

  it("is not called when the item is not found", async () => {
    const { detail, opened } = await setup();
    await detail.open("S1", "S1-P1-missing");
    expect(opened).toEqual([]);
  });

  it("is not called again by a reload", async () => {
    const { detail, opened, task } = await setup();
    await detail.open("S1", task.id);
    await detail.reload();
    expect(opened).toHaveLength(1);
  });
});
