import { describe, expect, it } from "vitest";
import type { Item } from "../../model/item";
import { flush } from "./fixtures.test-util";
import { settings } from "./settings.test-util";
import type { TreeWritesDeps } from "./treeWrites";
import { a, c, candidate, e1, e6, kids, setup } from "./treeWrites.test-util";

// 検索のツリーでは、Store をまたいで同じ内部 ID のアイテムが並ぶ。積んだときの Store と違う画面では、待っている分を実行しない。
describe("the store at the time of the key", () => {
  // switching は、画面の Store を S1 から始め、over の書き込みの偽物が toS2 を呼ぶと S2(同じ ID の行)へ移す。
  function switching(over: (toS2: () => void) => Partial<TreeWritesDeps>) {
    let storeId = "S1";
    return setup({
      screen: () => ({ storeId, readOnly: false, settings, rows: [e1, ...kids, e6] }),
      ...over(() => {
        storeId = "S2";
      }),
    });
  }

  it("moveSibling does not run a waiting step on the same ID in another store", async () => {
    const calls: string[] = [];
    const { writes } = switching((toS2) => ({
      reorder: async (it) => {
        calls.push(it.id);
        // 次のキーを積んだあとで、読み直した画面が別の Store になる。
        await Promise.resolve();
        toS2();
        return true;
      },
    }));
    writes.moveSibling(a.id, "down");
    writes.moveSibling(a.id, "down");
    await flush();
    expect(calls).toEqual([a.id]);
  });

  it("changeParent does not run a waiting step, nor move after the screen changed store", async () => {
    const moved: string[] = [];
    const { writes, log } = switching((toS2) => ({
      moveTo: async (it) => {
        moved.push(it.id);
        toS2();
        return true;
      },
    }));
    writes.changeParent(c.id, "down");
    writes.changeParent(c.id, "down");
    await flush();
    expect(moved).toEqual([c.id]);
    expect(log.filter((l) => l.startsWith("candidates"))).toEqual([`candidates ${c.id}`]);

    const late = switching((toS2) => ({
      moveCandidates: async () => {
        toS2();
        return [candidate(e6)];
      },
    }));
    late.writes.changeParent(c.id, "down");
    await flush();
    expect(late.log).toEqual([]);
  });
});

describe("Space (toggleDone)", () => {
  it("goes to the default done status, and back to the default todo status", async () => {
    const { writes, log } = setup();
    await writes.toggleDone(a.id);
    expect(log).toEqual([`status ${a.id} st3`]);
    const doneRow: Item = { ...a, status: "st3" };
    const again = setup({
      screen: () => ({ storeId: "S1", readOnly: false, settings, rows: [doneRow] }),
    });
    await again.writes.toggleDone(a.id);
    expect(again.log).toEqual([`status ${a.id} st1`]);
  });

  it("does nothing in a read-only store, for a row that is not visible, or without a default", async () => {
    const ro = setup({}, true);
    await ro.writes.toggleDone(a.id);
    const missing = setup();
    await missing.writes.toggleDone("nope");
    const bare = setup({
      screen: () => ({
        storeId: "S1",
        readOnly: false,
        settings: { ...settings, categoryDefaults: {} },
        rows: [a],
      }),
    });
    await bare.writes.toggleDone(a.id);
    expect([ro.log, missing.log, bare.log]).toEqual([[], [], []]);
  });
});
