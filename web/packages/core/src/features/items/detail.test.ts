import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { copiedMessage, copyFailedMessage, createDetail, normalizeTitle } from "./detail";
import { fakeItemsPort, flush, gate, item } from "./fixtures.test-util";

async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
  const task = item("S1", "S1-P1", 2, "Task", epic.id, "画面");
  const s1 = { ...item("S1", "S1-P1", 3, "Step", task.id, "設計"), rank: "W" };
  const s2 = { ...item("S1", "S1-P1", 4, "Step", task.id, "実装"), rank: "V" };
  const items = fakeItemsPort([epic, task, s1, s2]);
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  let changed = 0;
  const detail = createDetail({ catalog, items, toasts, onChanged: () => void changed++ });
  return { detail, items, toasts, task, s1, s2, changed: () => changed };
}

describe("createDetail", () => {
  it("opens an item with its ancestors and children in sibling order", async () => {
    const { detail, task } = await setup();
    await detail.open("S1", task.id);
    const v = get(detail.view);
    expect(v?.displayId).toBe("HH-2");
    expect(v?.ancestors.map((a) => a.title)).toEqual(["認証"]);
    expect(v?.children.map((c) => c.title)).toEqual(["実装", "設計"]);
    expect(v?.project.displayId).toBe("HH-P1");
    expect(v?.readOnly).toBe(false);
  });

  it("refuses an empty title without sending, and skips an unchanged one", async () => {
    const { detail, items, toasts, task } = await setup();
    await detail.open("S1", task.id);
    expect(await detail.rename("  \n ")).toBe(false);
    expect(get(toasts.list)[0]).toMatchObject({ text: "タイトルは空にできません", error: true });
    expect(await detail.rename(" 画面 ")).toBe(true);
    expect(items.calls).toEqual([]);
  });

  it("saves a normalized title and tells the screen", async () => {
    const { detail, items, task, changed } = await setup();
    await detail.open("S1", task.id);
    expect(await detail.rename("ログイン\n 画面")).toBe(true);
    expect(items.calls).toEqual([`rename ${task.id} ログイン 画面`]);
    expect(get(detail.view)?.item.title).toBe("ログイン 画面");
    expect(changed()).toBe(1);
  });

  it("adds a Step, ignoring an empty one", async () => {
    const { detail, items, task } = await setup();
    await detail.open("S1", task.id);
    expect(await detail.addStep("   ")).toBe(false);
    expect(await detail.addStep("確認")).toBe(true);
    expect(items.calls).toEqual([`addStep ${task.id} 確認`]);
    expect(get(detail.view)?.children.map((c) => c.title)).toContain("確認");
  });

  it("moves a Step up, doing nothing for the first", async () => {
    const { detail, items, task, s1, s2 } = await setup();
    await detail.open("S1", task.id);
    await detail.moveUp(s2.id);
    await detail.moveUp(s1.id);
    expect(items.calls).toEqual([`moveUp ${s1.id}`]);
  });

  it("closes when the item is gone, and toggles full screen", async () => {
    const { detail, task } = await setup();
    await detail.open("S1", task.id);
    detail.toggleFull();
    expect(get(detail.full)).toBe(true);
    await detail.open("S1", "nope");
    expect(get(detail.view)).toBeNull();
    detail.close();
    expect(get(detail.full)).toBe(false);
  });

  it("keeps the item opened while a save was in flight", async () => {
    const { detail, items, task, s1 } = await setup();
    const renameGate = gate();
    const getGate = gate();
    const rename = items.rename.bind(items);
    const getItem = items.get.bind(items);
    items.rename = async (...a) => {
      await renameGate.wait;
      return rename(...a);
    };
    items.get = async (st, id) => {
      if (id === s1.id) await getGate.wait;
      return getItem(st, id);
    };
    await detail.open("S1", task.id);
    const saving = detail.rename("新しい画面");
    await flush();
    const opening = detail.open("S1", s1.id);
    await flush();
    renameGate.release();
    await saving;
    getGate.release();
    await opening;
    expect(get(detail.view)?.item.id).toBe(s1.id);
  });

  it("reload reads the open item again (C2 画面の決定 S10)", async () => {
    const { detail, items, task } = await setup();
    await detail.open("S1", task.id);
    const i = items.all.findIndex((x) => x.id === task.id);
    items.all[i] = { ...task, title: "別の画面で変えたタイトル" };
    expect(get(detail.view)?.item.title).toBe("画面");
    await detail.reload();
    expect(get(detail.view)?.item.title).toBe("別の画面で変えたタイトル");
  });

  it("reload closes the panel when the item is gone (06「反映で変わる画面」)", async () => {
    const { detail, items, task } = await setup();
    await detail.open("S1", task.id);
    items.all.splice(
      items.all.findIndex((x) => x.id === task.id),
      1,
    );
    await detail.reload();
    expect(get(detail.view)).toBeNull();
  });

  it("reload does not bring back the old item while another is opening", async () => {
    const { detail, task, s1 } = await setup();
    await detail.open("S1", task.id);
    const opening = detail.open("S1", s1.id);
    await detail.reload();
    await opening;
    expect(get(detail.view)?.item.id).toBe(s1.id);
  });

  it("reload does not reopen a panel that closed because the item was gone", async () => {
    const { detail, items, task } = await setup();
    await detail.open("S1", task.id);
    const i = items.all.findIndex((x) => x.id === task.id);
    items.all.splice(i, 1);
    await detail.reload();
    expect(get(detail.view)).toBeNull();
    items.all.splice(i, 0, task);
    await detail.reload();
    expect(get(detail.view)).toBeNull();
  });

  it("reload does nothing when the panel is closed", async () => {
    const { detail } = await setup();
    await detail.reload();
    expect(get(detail.view)).toBeNull();
  });
});

describe("titles and copy messages", () => {
  it("normalizes spaces and line breaks", () => {
    expect(normalizeTitle("  a \n\t b  ")).toBe("a b");
  });

  it("says what was copied", () => {
    expect(copiedMessage("HH-2")).toBe("コピーしました: HH-2");
    expect(copyFailedMessage("HH-2")).toBe(
      "コピーできませんでした。HH-2 を選択してコピーしてください",
    );
  });
});
