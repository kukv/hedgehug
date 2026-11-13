import { describe, expect, it } from "vitest";
import type { Item } from "../../model/item";
import { get, writable } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { isItemDraft } from "../drafts/drafts";
import { createToasts } from "../toasts/toasts";
import { fakeItemActions } from "./actions.test-util";
import { createItemDelete, itemDeleteDialog, itemDeleteFailedMessage } from "./delete";
import type { DetailView } from "./detail";
import { flush, gate, item } from "./fixtures.test-util";

// viewOf は、delete が見る item と ancestors だけを持つ詳細パネルの表示。
const viewOf = (it: Item, ancestors: Item[]) => ({ item: it, ancestors }) as unknown as DetailView;

async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const actions = fakeItemActions();
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const refreshed: string[] = [];
  const confirms: unknown[] = [];
  let allow = true;
  const panel = writable<DetailView | null>(null);
  const del = createItemDelete({
    actions,
    catalog,
    toasts,
    detail: { view: panel },
    drafts: {
      confirmLeave: async (only) => {
        confirms.push(only);
        return allow;
      },
    },
    refresh: {
      afterDelete: async (it: Item) => {
        refreshed.push(it.id);
      },
    },
  });
  const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
  const task = item("S1", "S1-P1", 2, "Task", epic.id, "画面");
  const ro = item("S2", "S2-P1", 1, "Task", null, "閲覧のみ");
  const messages = () => get(toasts.list).map((t) => `${t.error ? "!" : ""}${t.text}`);
  return {
    del,
    actions,
    refreshed,
    messages,
    epic,
    task,
    ro,
    panel,
    confirms,
    deny: () => {
      allow = false;
    },
  };
}

describe("itemDeleteDialog", () => {
  const task = item("S1", "S1-P1", 3, "Task", null, "画面");

  it("writes the title, the body and the note of the spec", () => {
    const d = itemDeleteDialog(task, "HH-3", { count: 0, kinds: [], retentionDays: 30 });
    expect(d.title).toBe("Task を削除");
    expect(d.lines).toEqual([
      "HH-3 画面 を削除します。",
      "設定の「削除済み」から戻せます。30 日を過ぎると完全に削除されます。",
    ]);
    expect(d.ok).toBe("削除");
  });

  it("continues the body with the items below, naming each kind once in the spec order", () => {
    const d = itemDeleteDialog(task, "HH-3", {
      count: 5,
      kinds: ["Step", "Task", "Step"],
      retentionDays: 7,
    });
    expect(d.lines[0]).toBe(
      "HH-3 画面 を削除します。下にある 5 件(Task・Step)も一緒に削除されます。",
    );
    expect(d.lines[1]).toBe("設定の「削除済み」から戻せます。7 日を過ぎると完全に削除されます。");
    const epic = itemDeleteDialog({ ...task, kind: "Epic" }, "HH-1", {
      count: 4,
      kinds: ["Bug", "Epic", "Story"],
      retentionDays: 30,
    });
    expect(epic.title).toBe("Epic を削除");
    expect(epic.lines[0]).toContain("(Epic・Story・Bug)");
  });
});

describe("createItemDelete", () => {
  it("opens the dialog after reading the preview of that item", async () => {
    const { del, actions, task } = await setup();
    actions.preview = { count: 2, kinds: ["Step"], retentionDays: 14 };
    await del.open(task);
    const s = get(del.state);
    expect(actions.calls).toEqual([`deletePreview ${task.id}`]);
    expect(s?.displayId).toBe("HH-2");
    expect(s?.lines[0]).toBe("HH-2 画面 を削除します。下にある 2 件(Step)も一緒に削除されます。");
    expect(s?.lines[1]).toContain("14 日");
  });

  it("does nothing in a read-only Store", async () => {
    const { del, actions, ro } = await setup();
    await del.open(ro);
    expect(get(del.state)).toBeNull();
    expect(actions.calls).toEqual([]);
  });

  it("deletes on confirm: closes the dialog, shows the toast, and reloads", async () => {
    const { del, actions, refreshed, messages, task } = await setup();
    await del.open(task);
    await del.confirm();
    expect(actions.calls).toEqual([`deletePreview ${task.id}`, `deleteItem ${task.id}`]);
    expect(get(del.state)).toBeNull();
    expect(messages()).toEqual(["HH-2 を削除しました"]);
    expect(refreshed).toEqual([task.id]);
  });

  it("deletes nothing on cancel", async () => {
    const { del, actions, refreshed, messages, task } = await setup();
    await del.open(task);
    del.cancel();
    await del.confirm();
    expect(get(del.state)).toBeNull();
    expect(actions.calls).toEqual([`deletePreview ${task.id}`]);
    expect(messages()).toEqual([]);
    expect(refreshed).toEqual([]);
  });

  it("drops a preview that arrives after the dialog was cancelled or opened for another item", async () => {
    const { del, actions, task, epic } = await setup();
    const g = gate();
    const read = actions.deletePreview.bind(actions);
    actions.deletePreview = async (s, id) => {
      const p = await read(s, id);
      if (id === task.id) await g.wait;
      return p;
    };
    const first = del.open(task);
    await flush();
    del.cancel();
    g.release();
    await first;
    expect(get(del.state)).toBeNull();
    const slow = gate();
    actions.deletePreview = async (s, id) => {
      const p = await read(s, id);
      if (id === task.id) await slow.wait;
      return p;
    };
    const stale = del.open(task);
    await flush();
    await del.open(epic);
    slow.release();
    await stale;
    expect(get(del.state)?.item.id).toBe(epic.id);
  });

  it("sends one delete for a double confirm", async () => {
    const { del, actions, task } = await setup();
    await del.open(task);
    const hold = actions.holdNext();
    const first = del.confirm();
    await flush();
    await del.confirm();
    hold.release();
    await first;
    expect(actions.calls.filter((c) => c.startsWith("deleteItem"))).toEqual([
      `deleteItem ${task.id}`,
    ]);
  });

  it("shows a refusal as an error toast, closes the dialog, and does not reload", async () => {
    const { del, actions, refreshed, messages, task } = await setup();
    await del.open(task);
    actions.refuseNext("削除できません");
    await del.confirm();
    expect(messages()).toEqual(["!削除できません"]);
    expect(get(del.state)).toBeNull();
    expect(refreshed).toEqual([]);
  });

  it("shows another failure with the provisional words", async () => {
    const { del, actions, messages, task } = await setup();
    await del.open(task);
    actions.failNext();
    await del.confirm();
    expect(messages()).toEqual([`!${itemDeleteFailedMessage}`]);
  });

  it("reloads without a toast when the item is already gone (confirm or preview)", async () => {
    const { del, actions, refreshed, messages, task, epic } = await setup();
    await del.open(task);
    actions.missing.add(task.id);
    await del.confirm();
    expect(messages()).toEqual([]);
    expect(refreshed).toEqual([task.id]);
    actions.missing.add(epic.id);
    await del.open(epic);
    expect(get(del.state)).toBeNull();
    expect(refreshed).toEqual([task.id, epic.id]);
    expect(messages()).toEqual([]);
  });

  it("asks to discard the input first when the open panel is the item or its ancestor", async () => {
    const { del, actions, task, epic, panel, confirms, deny } = await setup();
    panel.set(viewOf(task, [epic]));
    await del.open(epic);
    expect(confirms).toEqual([isItemDraft]);
    expect(get(del.state)?.item.id).toBe(epic.id);
    del.cancel();
    deny();
    await del.open(task);
    expect(confirms).toEqual([isItemDraft, isItemDraft]);
    expect(get(del.state)).toBeNull();
    expect(actions.calls).toEqual([`deletePreview ${epic.id}`]);
  });

  it("does not ask when the open panel is unrelated to the item", async () => {
    const { del, task, epic, panel, confirms } = await setup();
    panel.set(viewOf(epic, []));
    await del.open(task);
    expect(confirms).toEqual([]);
    expect(get(del.state)?.item.id).toBe(task.id);
  });
});
