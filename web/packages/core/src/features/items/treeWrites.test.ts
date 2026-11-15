import { describe, expect, it } from "vitest";
import { NotFound } from "../../model/errors";
import type { MoveCandidate } from "../../model/itemAction";
import { flush, gate } from "./fixtures.test-util";
import { moveFailedMessage } from "./move";
import { settings } from "./settings.test-util";
import { a, b, c, candidate, d, e1, e6, setup } from "./treeWrites.test-util";

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

  it("does nothing for an Epic without a project row, a read-only store, or when no candidate is visible that way", async () => {
    const epic = setup({
      moveCandidates: async (_s, id) => {
        epic.log.push(`candidates ${id}`);
        return [{ ...candidate(e6), kind: "project", id: "P2" }];
      },
    });
    epic.writes.changeParent(e1.id, "down");
    const ro = setup({}, true);
    ro.writes.changeParent(c.id, "down");
    const none = setup();
    none.writes.changeParent(c.id, "up");
    await flush();
    // プロジェクトの画面(order なし)の Epic は、候補も読まない(E3a-17)。
    expect(epic.log).toEqual([]);
    expect(ro.log).toEqual([]);
    expect(none.log).toEqual([`candidates ${c.id}`]);
    expect(none.toasts.show).not.toHaveBeenCalled();
  });

  it("changeParent moves an Epic under a visible project", async () => {
    const p2 = { id: "P2", parentId: null };
    const projectCandidate: MoveCandidate = {
      kind: "project",
      id: "P2",
      display: "HH-P2",
      title: "決済",
      projectId: "P2",
      projectTitle: "",
    };
    const { writes, log } = setup({
      screen: () => ({
        storeId: "S1",
        readOnly: false,
        settings,
        rows: [e1, e6],
        order: [p2, e1, e6],
      }),
      moveCandidates: async () => [projectCandidate],
    });
    writes.changeParent(e1.id, "up");
    await flush();
    expect(log).toEqual([`expand P2`, `moveTo ${e1.id} P2`]);
  });

  it("uses order when given", async () => {
    const { writes, log } = setup({
      screen: () => ({
        storeId: "S1",
        readOnly: false,
        settings,
        rows: [e1, a, e6],
        order: [e1, e6, a],
      }),
    });
    // rows だと a の上に e6 はないが、order では a の上に e6 がある。
    writes.changeParent(a.id, "up");
    await flush();
    expect(log).toEqual([`candidates ${a.id}`, `expand ${e6.id}`, `moveTo ${a.id} ${e6.id}`]);
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
