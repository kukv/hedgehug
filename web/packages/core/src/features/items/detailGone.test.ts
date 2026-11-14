import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakePlanningPort } from "../planning/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createDetail } from "./detail";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces } from "./settings.test-util";

// 開いているアイテムが、同期・ほかのタブ・ほかのプログラムで削除されたとき(A4b-22)。読み直しで消えていたら、
// 確認を出さずに、全画面も解除して閉じる(02「詳細パネル」)。
async function setup() {
  const catalogPort = fakeCatalogPort();
  const gone = new Set<string>();
  const listProjects = catalogPort.listProjects.bind(catalogPort);
  catalogPort.listProjects = async (s, w) =>
    (await listProjects(s, w)).filter((p) => !gone.has(p.id));
  const catalog = createCatalog(catalogPort);
  await catalog.load();
  const task = item("S1", "S1-P1", 2, "Task", null, "画面");
  const other = item("S1", "S1-P1", 3, "Task", null, "決済");
  const items = fakeItemsPort([task, other]);
  const detail = createDetail({
    planning: fakePlanningPort(),
    catalog,
    items,
    toasts: createToasts({ set: () => 0, clear: () => {} }),
    workspaces: fakeWorkspaces(),
    catalogPort,
    onChanged: () => {},
    // 確認を出す口。出したら失敗にする(閉じるときに確認してはいけない)。
    leave: async () => {
      throw new Error("asked for a confirmation");
    },
  });
  return { detail, catalog, items, gone, task, other };
}

describe("createDetail when the open item is gone", () => {
  it("closes on reload without asking, and leaves full screen", async () => {
    const { detail, items, task, other } = await setup();
    await detail.open("S1", task.id);
    detail.toggleFull();
    items.all.splice(items.all.indexOf(task), 1);
    await detail.reload();
    expect(get(detail.view)).toBeNull();
    expect(get(detail.full)).toBe(false);
    // 次に開くアイテムが、全画面のまま出ない。
    await detail.open("S1", other.id);
    expect(get(detail.full)).toBe(false);
  });

  it("closes the same way when the item's project is gone from the list", async () => {
    const { detail, catalog, gone, task } = await setup();
    await detail.open("S1", task.id);
    detail.toggleFull();
    gone.add("S1-P1");
    await catalog.load();
    await detail.reload();
    expect(get(detail.view)).toBeNull();
    expect(get(detail.full)).toBe(false);
  });
});
