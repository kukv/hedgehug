import { describe, expect, it, vi } from "vitest";
import { NotFound } from "../../model/errors";
import type { Item } from "../../model/item";
import type { MoveCandidate } from "../../model/itemAction";
import { flush, gate, item } from "./fixtures.test-util";
import { moveFailedMessage } from "./move";
import { settings } from "./settings.test-util";
import { visibleNeighbors } from "./siblings";
import { createTreeWrites, type TreeWritesDeps } from "./treeWrites";

// 木: HH-1(Epic) ─ HH-2・HH-3・HH-4・HH-5(Task) / HH-6(Epic)。並べ替えのたびに読み直した並びを rows に入れる。
const e1 = item("S1", "P1", 1, "Epic", null);
const e6 = item("S1", "P1", 6, "Epic", null);
const kids: Item[] = [2, 3, 4, 5].map((n) => item("S1", "P1", n, "Task", e1.id));
const [a, b, c, d] = kids as [Item, Item, Item, Item];

const candidate = (it: Item): MoveCandidate => ({
  kind: "item",
  id: it.id,
  display: `HH-${it.number}`,
  title: it.title,
  projectId: it.projectId,
  projectTitle: "ログイン",
});

// setup の reorder は、itemMenuHost.reorder と同じ約束の偽物: 見えている兄弟を基準に 1 件だけ動かし、読み直して、
// 動かせたら true(基準の兄弟がない・閲覧のみは false)。
function setup(over: Partial<TreeWritesDeps> = {}, readOnly = false) {
  let order = [...kids];
  let rows: Item[] = [e1, ...order, e6];
  const log: string[] = [];
  const toasts = { show: vi.fn() };
  const deps: TreeWritesDeps = {
    screen: () => ({ storeId: "S1", readOnly, settings, rows }),
    reorder: async (it, seen, dir) => {
      const { prev, next } = visibleNeighbors(seen, it.id);
      const anchor = dir === "up" ? prev : next;
      if (!anchor) return false;
      log.push(`reorder ${it.id} ${anchor.id} ${dir === "down"}`);
      order = order.filter((i) => i.id !== it.id);
      order.splice(order.findIndex((i) => i.id === anchor.id) + (dir === "down" ? 1 : 0), 0, it);
      rows = [e1, ...order, e6];
      log.push("refresh");
      return true;
    },
    moveCandidates: async (_s, id) => {
      log.push(`candidates ${id}`);
      return [candidate(e6)];
    },
    moveTo: async (it, cand) => {
      log.push(`moveTo ${it.id} ${cand.id}`);
      return true;
    },
    setStatus: async (id, status) => {
      log.push(`status ${id} ${status}`);
    },
    afterChange: async () => {
      log.push("refresh");
    },
    settled: async () => {},
    expand: (id) => {
      log.push(`expand ${id}`);
    },
    toasts,
    ...over,
  };
  return { writes: createTreeWrites(deps), log, toasts, order: () => order.map((i) => i.id) };
}

describe("Alt+↑↓ (moveSibling)", () => {
  it("does not start the next step before the previous one is saved and reloaded", async () => {
    const hold = gate();
    const calls: string[] = [];
    const { writes } = setup({
      reorder: async (it) => {
        calls.push(it.id);
        if (calls.length === 1) await hold.wait;
        return true;
      },
    });
    writes.moveSibling(b.id, "down");
    writes.moveSibling(b.id, "down");
    await flush();
    expect(calls).toHaveLength(1);
    hold.release();
    await flush();
    expect(calls).toHaveLength(2);
  });

  it("recomputes the neighbour after each reload: two presses move two places", async () => {
    const { writes, log, order } = setup();
    writes.moveSibling(b.id, "down");
    writes.moveSibling(b.id, "down");
    await flush();
    expect(log).toEqual([
      `reorder ${b.id} ${c.id} true`,
      "refresh",
      `reorder ${b.id} ${d.id} true`,
      "refresh",
    ]);
    expect(order()).toEqual([a.id, c.id, d.id, b.id]);
  });

  it("puts the item before the previous sibling when moving up", async () => {
    const { writes, log } = setup();
    writes.moveSibling(c.id, "up");
    await flush();
    expect(log[0]).toBe(`reorder ${c.id} ${b.id} false`);
  });

  it("drops the waiting steps when a step is not done (the end of the list, a refusal)", async () => {
    const end = setup();
    end.writes.moveSibling(a.id, "up");
    end.writes.moveSibling(b.id, "down");
    await flush();
    expect(end.log).toEqual([]);

    const calls: string[] = [];
    const refused = setup({
      reorder: async (it) => {
        calls.push(it.id);
        return false;
      },
    });
    refused.writes.moveSibling(b.id, "down");
    refused.writes.moveSibling(b.id, "down");
    await flush();
    expect(calls).toHaveLength(1);
  });

  it("drops the waiting steps on clear", async () => {
    const hold = gate();
    const calls: string[] = [];
    const { writes } = setup({
      reorder: async (it) => {
        await hold.wait;
        calls.push(it.id);
        return true;
      },
    });
    writes.moveSibling(a.id, "down");
    writes.moveSibling(a.id, "down");
    await flush();
    writes.clear();
    hold.release();
    await flush();
    expect(calls).toEqual([a.id]);
  });

  it("does nothing for a row that is not visible, and in a read-only store", async () => {
    const missing = setup();
    missing.writes.moveSibling("nope", "down");
    const ro = setup({}, true);
    ro.writes.moveSibling(b.id, "down");
    await flush();
    expect([missing.log, ro.log]).toEqual([[], []]);
    expect(ro.toasts.show).not.toHaveBeenCalled();
  });
});

describe("Alt+Shift+↑↓ (changeParent)", () => {
  it("opens the new parent, then moves under the first candidate in that direction", async () => {
    const { writes, log } = setup();
    writes.changeParent(c.id, "down");
    await flush();
    expect(log).toEqual([`candidates ${c.id}`, `expand ${e6.id}`, `moveTo ${c.id} ${e6.id}`]);
  });

  it("does nothing for an Epic, a read-only store, or when no candidate is visible that way", async () => {
    const epic = setup();
    epic.writes.changeParent(e1.id, "down");
    const ro = setup({}, true);
    ro.writes.changeParent(c.id, "down");
    const none = setup();
    none.writes.changeParent(c.id, "up");
    await flush();
    expect(epic.log).toEqual([]);
    expect(ro.log).toEqual([]);
    expect(none.log).toEqual([`candidates ${c.id}`]);
    expect(none.toasts.show).not.toHaveBeenCalled();
  });

  it("drops the waiting steps when the move is not done", async () => {
    const moved: string[] = [];
    const { writes } = setup({
      moveTo: async (it) => {
        moved.push(it.id);
        return false;
      },
    });
    writes.changeParent(b.id, "down");
    writes.changeParent(c.id, "down");
    await flush();
    expect(moved).toEqual([b.id]);
  });

  it("reports a failed candidate read like the move menu, and reloads when the item is gone", async () => {
    const broken = setup({
      moveCandidates: async () => {
        throw new Error("network");
      },
    });
    broken.writes.changeParent(b.id, "down");
    await flush();
    expect(broken.toasts.show).toHaveBeenCalledExactlyOnceWith(moveFailedMessage, { error: true });

    const gone = setup({
      moveCandidates: async () => {
        throw new NotFound("x");
      },
    });
    gone.writes.changeParent(b.id, "down");
    await flush();
    expect(gone.log).toEqual(["refresh"]);
    expect(gone.toasts.show).not.toHaveBeenCalled();
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
