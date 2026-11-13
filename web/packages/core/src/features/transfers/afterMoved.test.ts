import { describe, expect, it } from "vitest";
import { writable } from "../../reactive/store";
import type { DetailView } from "../items/detail";
import { item } from "../items/fixtures.test-util";
import { createAfterMoved } from "./afterMoved";

// 移したあとの詳細パネルの切り替えと読み直し(A5a-18)。「移動」を押したときに afterMoved(from, panelItem) を呼び、
// 成功したら返った関数に結果を渡す。panelItem は押したときにパネルが出していたアイテムの内部 ID。別の Store へ
// 移しても内部 ID は変わらない。
function setup() {
  const calls: string[] = [];
  const view = writable<DetailView | null>(null);
  const route = writable("/s/S1/p/HH-P1");
  const detail = {
    view,
    close: () => void calls.push("close"),
    open: async (storeId: string, itemId: string) => void calls.push(`open ${storeId}/${itemId}`),
  };
  const refresh = { afterChange: async () => void calls.push("change") };
  return { calls, view, route, afterMoved: createAfterMoved({ detail, refresh, route }) };
}

const from = item("S1", "S1-P1", 3, "Task", null, "決済");
const step = item("S1", "S1-P1", 4, "Step", from.id, "設計");
const other = item("S1", "S1-P1", 5, "Task", null, "検索");
const result = { toStore: "S3", itemId: from.id, display: "TM-4", emptied: [] };
const showing = (it: typeof from) => ({ item: it }) as DetailView;

describe("createAfterMoved", () => {
  it("closes the panel without asking, opens the moved item in the new Store, then reloads", async () => {
    const { calls, view, afterMoved } = setup();
    view.set(showing(from));
    await afterMoved(from, from.id)(result);
    expect(calls).toEqual(["close", "open S3/S1-I3", "change"]);
  });

  it("opens the same item again inside one Store (the internal ID stays)", async () => {
    const { calls, view, afterMoved } = setup();
    view.set(showing(from));
    await afterMoved(from, from.id)({ ...result, toStore: "S1" });
    expect(calls).toEqual(["close", "open S1/S1-I3", "change"]);
  });

  it("opens the item again even when a reload closed the panel while sending", async () => {
    const { calls, view, afterMoved } = setup();
    view.set(showing(step));
    const done = afterMoved(from, step.id);
    view.set(null);
    await done(result);
    expect(calls).toEqual(["close", "open S3/S1-I4", "change"]);
  });

  it("only reloads when the panel showed neither the moved item nor something below it", async () => {
    const { calls, afterMoved } = setup();
    await afterMoved(from, undefined)(result);
    expect(calls).toEqual(["change"]);
  });

  it("does not switch the panel when another item was opened while sending", async () => {
    const { calls, view, afterMoved } = setup();
    view.set(showing(from));
    const done = afterMoved(from, from.id);
    view.set(showing(other));
    await done(result);
    expect(calls).toEqual(["change"]);
  });

  it("does not open the item again when the screen changed while sending", async () => {
    const { calls, view, route, afterMoved } = setup();
    view.set(showing(from));
    const done = afterMoved(from, from.id);
    route.set("/s/S1/w/HH");
    view.set(null);
    await done(result);
    expect(calls).toEqual(["change"]);
  });
});
