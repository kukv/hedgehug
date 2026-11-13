import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createDetail } from "./detail";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces } from "./settings.test-util";

// 開いているアイテムを、ほかの書き手(同期・ほかのタブ・ほかのプログラム)が別のワークスペースへ移したとき(A5a-18)。
// 読み直しで、内部 ID が同じでもワークスペースが変わっていれば、確認を出さずに、全画面も解除して閉じる。
async function setup() {
  const catalogPort = fakeCatalogPort();
  const catalog = createCatalog(catalogPort);
  await catalog.load();
  const task = item("S1", "S1-P1", 2, "Task", null, "画面");
  const items = fakeItemsPort([task]);
  const detail = createDetail({
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
  return { detail, items, task };
}

describe("createDetail when the open item moved to another workspace", () => {
  it("closes without asking when another writer moved the open item to another workspace (same internal ID)", async () => {
    const { detail, items, task } = await setup();
    await detail.open("S1", task.id);
    detail.toggleFull();
    items.all[items.all.indexOf(task)] = { ...task, workspaceId: "S1-W2" };
    await detail.reload();
    expect(get(detail.view)).toBeNull();
    expect(get(detail.full)).toBe(false);
  });

  it("keeps the panel when the reloaded item stays in the same workspace", async () => {
    const { detail, items, task } = await setup();
    await detail.open("S1", task.id);
    items.all[items.all.indexOf(task)] = { ...task, title: "新しい名前" };
    await detail.reload();
    expect(get(detail.view)?.item.title).toBe("新しい名前");
  });
});

// target は、パネルが開こうとしているアイテム。旧 ID から開いたときの上部の 1 行(A5a-16)を、開けなかったときだけ
// 捨てるのに使う。開く途中で読み直し(画面への反映)が追い越しても、同じアイテムを開こうとしたままである。
describe("createDetail target", () => {
  it("keeps the item while a reload overtakes the open", async () => {
    const { detail, task } = await setup();
    const opening = detail.open("S1", task.id);
    await detail.reload();
    await opening;
    expect(detail.target()).toEqual({ storeId: "S1", itemId: task.id });
    expect(get(detail.view)?.item.id).toBe(task.id);
  });

  it("is null after the open finds nothing", async () => {
    const { detail } = await setup();
    await detail.open("S1", "S1-I99");
    expect(detail.target()).toBeNull();
  });
});
