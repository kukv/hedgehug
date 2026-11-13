import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { fakeItemActions } from "./actions.test-util";
import { createItemDuplicate, duplicateFailedMessage } from "./duplicate";
import { flush, item } from "./fixtures.test-util";

const task = item("S1", "S1-P1", 2, "Task", null, "画面");
const ro = item("S2", "S2-P1", 1, "Task", null, "閲覧のみ");

async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const actions = fakeItemActions();
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  // order は、詳細パネルを開く・読み直すの順(新しいアイテムを開いてから読み直す)。
  const order: string[] = [];
  const duplicate = createItemDuplicate({
    actions,
    catalog,
    toasts,
    detail: {
      open: async (store, id) => {
        order.push(`open ${store}/${id}`);
      },
    },
    refresh: {
      afterChange: async () => {
        order.push("change");
      },
    },
  });
  const messages = () => get(toasts.list).map((t) => `${t.error ? "!" : ""}${t.text}`);
  return { duplicate, actions, order, messages };
}

describe("createItemDuplicate", () => {
  it("duplicates, switches the panel to the new item, reloads, and shows no toast", async () => {
    const { duplicate, actions, order, messages } = await setup();
    await duplicate.run(task);
    expect(actions.calls).toEqual([`duplicate ${task.id}`]);
    expect(order).toEqual([`open S1/S1-copy-of-${task.id}`, "change"]);
    expect(messages()).toEqual([]);
  });

  it("does nothing in a read-only Store", async () => {
    const { duplicate, actions, order } = await setup();
    await duplicate.run(ro);
    expect(actions.calls).toEqual([]);
    expect(order).toEqual([]);
  });

  it("sends one duplicate for a double run", async () => {
    const { duplicate, actions } = await setup();
    const hold = actions.holdNext();
    const first = duplicate.run(task);
    await flush();
    await duplicate.run(task);
    hold.release();
    await first;
    expect(actions.calls).toEqual([`duplicate ${task.id}`]);
  });

  it("shows a refusal as an error toast and leaves the panel alone", async () => {
    const { duplicate, actions, order, messages } = await setup();
    actions.refuseNext("複製できません");
    await duplicate.run(task);
    expect(messages()).toEqual(["!複製できません"]);
    expect(order).toEqual([]);
  });

  it("shows another failure with the failure message", async () => {
    const { duplicate, actions, order, messages } = await setup();
    actions.failNext();
    await duplicate.run(task);
    expect(messages()).toEqual([`!${duplicateFailedMessage}`]);
    expect(order).toEqual([]);
    // 失敗のあとも、もう一度できる。
    await duplicate.run(task);
    expect(order).toEqual([`open S1/S1-copy-of-${task.id}`, "change"]);
  });

  it("reloads without a toast when the item is already gone", async () => {
    const { duplicate, actions, order, messages } = await setup();
    actions.missing.add(task.id);
    await duplicate.run(task);
    expect(messages()).toEqual([]);
    expect(order).toEqual(["change"]);
  });
});
