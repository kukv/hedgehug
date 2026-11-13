import { describe, expect, it } from "vitest";
import { item } from "../items/fixtures.test-util";
import { createAfterMoved } from "./afterMoved";

// 移したあとの詳細パネルの切り替えと読み直し(A5a-18)。panelItem は「移動」を押したときにパネルが出していた
// アイテムの内部 ID。別の Store へ移しても内部 ID は変わらない。
function setup() {
  const calls: string[] = [];
  const detail = {
    close: () => void calls.push("close"),
    open: async (storeId: string, itemId: string) => void calls.push(`open ${storeId}/${itemId}`),
  };
  const refresh = { afterChange: async () => void calls.push("change") };
  return { calls, afterMoved: createAfterMoved({ detail, refresh }) };
}

const from = item("S1", "S1-P1", 3, "Task", null, "決済");
const step = item("S1", "S1-P1", 4, "Step", from.id, "設計");
const result = { toStore: "S3", itemId: from.id, display: "TM-4", emptied: [] };

describe("createAfterMoved", () => {
  it("closes the panel without asking, opens the moved item in the new Store, then reloads", async () => {
    const { calls, afterMoved } = setup();
    await afterMoved(from, result, from.id);
    expect(calls).toEqual(["close", "open S3/S1-I3", "change"]);
  });

  it("opens the same item again inside one Store (the internal ID stays)", async () => {
    const { calls, afterMoved } = setup();
    await afterMoved(from, { ...result, toStore: "S1" }, from.id);
    expect(calls).toEqual(["close", "open S1/S1-I3", "change"]);
  });

  it("opens the item below that the panel was showing, in the new Store", async () => {
    const { calls, afterMoved } = setup();
    await afterMoved(from, result, step.id);
    expect(calls).toEqual(["close", "open S3/S1-I4", "change"]);
  });

  it("only reloads when the panel showed neither the moved item nor something below it", async () => {
    const { calls, afterMoved } = setup();
    await afterMoved(from, result, undefined);
    expect(calls).toEqual(["change"]);
  });
});
