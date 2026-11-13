import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakePlanningPort } from "../planning/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createDetail } from "./detail";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces } from "./settings.test-util";

// 詳細パネルが持つ兄弟(メニューの「上へ移動」「下へ移動」の見えている兄弟になる)。
async function setup() {
  const catalogPort = fakeCatalogPort();
  const catalog = createCatalog(catalogPort);
  await catalog.load();
  const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
  const otherEpic = { ...item("S1", "S1-P1", 5, "Epic", null, "決済"), rank: "W" };
  const task = item("S1", "S1-P1", 2, "Task", epic.id, "画面");
  const s1 = { ...item("S1", "S1-P1", 3, "Step", task.id, "設計"), rank: "W" };
  const s2 = { ...item("S1", "S1-P1", 4, "Step", task.id, "実装"), rank: "V" };
  const detail = createDetail({
    planning: fakePlanningPort(),
    catalog,
    items: fakeItemsPort([epic, otherEpic, task, s1, s2]),
    toasts: createToasts({ set: () => 0, clear: () => {} }),
    workspaces: fakeWorkspaces(),
    catalogPort,
    onChanged: () => {},
  });
  return { detail, epic, task, s1 };
}

describe("createDetail siblings", () => {
  it("keeps the siblings of the open item, itself included, in sibling order", async () => {
    const { detail, task, s1, epic } = await setup();
    await detail.open("S1", s1.id);
    expect(get(detail.view)?.siblings.map((c) => c.title)).toEqual(["実装", "設計"]);
    await detail.open("S1", task.id);
    expect(get(detail.view)?.siblings.map((c) => c.id)).toEqual([task.id]);
    await detail.open("S1", epic.id);
    expect(get(detail.view)?.siblings.map((c) => c.title)).toEqual(["認証", "決済"]);
  });
});
