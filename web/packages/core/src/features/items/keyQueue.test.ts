import { describe, expect, it } from "vitest";
import { flush, gate } from "./fixtures.test-util";
import { createKeyQueue, keepsQueue } from "./keyQueue";
import type { KeyInput } from "./treeKeys";

describe("createKeyQueue", () => {
  it("runs the pushed steps one at a time, in order", async () => {
    const q = createKeyQueue();
    const log: string[] = [];
    const first = gate();
    q.push(async () => {
      log.push("start 1");
      await first.wait;
      log.push("end 1");
      return true;
    });
    q.push(async () => {
      log.push("start 2");
      return true;
    });
    await flush();
    expect(log).toEqual(["start 1"]);
    expect(q.pending()).toBe(1);
    first.release();
    await flush();
    expect(log).toEqual(["start 1", "end 1", "start 2"]);
    expect(q.pending()).toBe(0);
  });

  it("drops the waiting steps when a step is refused or throws", async () => {
    for (const fail of [async () => false, async () => Promise.reject(new Error("x"))]) {
      const q = createKeyQueue();
      const ran: number[] = [];
      q.push(fail);
      q.push(async () => {
        ran.push(2);
        return true;
      });
      q.push(async () => {
        ran.push(3);
        return true;
      });
      await flush();
      expect(ran).toEqual([]);
      expect(q.pending()).toBe(0);
    }
  });

  it("clear drops the waiting steps but lets the running one finish", async () => {
    const q = createKeyQueue();
    const ran: number[] = [];
    const first = gate();
    q.push(async () => {
      await first.wait;
      ran.push(1);
      return true;
    });
    q.push(async () => {
      ran.push(2);
      return true;
    });
    await flush();
    q.clear();
    first.release();
    await flush();
    expect(ran).toEqual([1]);
  });

  it("starts again after it has drained", async () => {
    const q = createKeyQueue();
    const ran: number[] = [];
    q.push(async () => {
      ran.push(1);
      return true;
    });
    await flush();
    q.push(async () => {
      ran.push(2);
      return true;
    });
    await flush();
    expect(ran).toEqual([1, 2]);
  });
});

describe("keepsQueue", () => {
  const key = (k: string): KeyInput => ({
    key: k,
    ctrlKey: false,
    metaKey: false,
    altKey: false,
    shiftKey: false,
  });

  it("keeps the queue for the queued actions and for the modifier keys alone", () => {
    expect(keepsQueue(key("ArrowUp"), "move-up")).toBe(true);
    expect(keepsQueue(key("ArrowDown"), "reparent-down")).toBe(true);
    for (const k of ["Alt", "Shift", "Control", "Meta"])
      expect(keepsQueue(key(k), null)).toBe(true);
  });

  it("drops it for any other key, handled by the tree or not", () => {
    expect(keepsQueue(key("ArrowDown"), "down")).toBe(false);
    expect(keepsQueue(key(" "), "toggle-done")).toBe(false);
    expect(keepsQueue(key("x"), null)).toBe(false);
  });
});
