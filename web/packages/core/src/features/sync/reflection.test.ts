import { describe, expect, it } from "vitest";
import type { SyncEventKind } from "../../model/sync";
import type { SyncPort } from "../../port/sync";
import type { Timer } from "../toasts/toasts";
import { fakeSyncPort } from "./fixtures.test-util";
import { createReflection } from "./reflection";

// manualTimer は、時間を手で進める時計(toasts.test.ts の fakeTimer と同じ形)。
function manualTimer() {
  let now = 0;
  let seq = 0;
  const pending = new Map<number, { at: number; fn: () => void }>();
  const timer: Timer = {
    set(fn, ms) {
      seq += 1;
      pending.set(seq, { at: now + ms, fn });
      return seq;
    },
    clear: (h) => void pending.delete(h as number),
  };
  // advance は、先に待っている処理(失敗した watch のあとの timer.set ほか)を流してから時間を進める。
  const advance = async (ms: number) => {
    await flush();
    now += ms;
    for (const [h, p] of [...pending].sort((a, b) => a[1].at - b[1].at)) {
      if (p.at <= now) {
        pending.delete(h);
        p.fn();
      }
    }
    await flush();
  };
  return { timer, advance };
}

// flush は、マイクロタスクを回す(core は setTimeout の型を持たない)。
const flush = async () => {
  for (let i = 0; i < 50; i++) await Promise.resolve();
};

// stream は、watch の流れを外から動かす。open・send・end・fail で流れを進める。
function stream() {
  let on: Parameters<SyncPort["watch"]>[0] | undefined;
  let finish: (() => void) | undefined;
  let error: ((e: unknown) => void) | undefined;
  let connects = 0;
  let stopped = false;
  const watch: SyncPort["watch"] = (o) => {
    connects += 1;
    on = o;
    stopped = false;
    const done = new Promise<void>((resolve, reject) => {
      finish = resolve;
      error = reject;
    });
    return {
      done,
      stop: () => {
        stopped = true;
        finish?.();
      },
    };
  };
  return {
    watch,
    connects: () => connects,
    aborted: () => stopped,
    open: () => on?.open(),
    send: (k: SyncEventKind) => on?.event(k),
    end: () => finish?.(),
    fail: () => error?.(new Error("down")),
  };
}

function setup() {
  const s = stream();
  const { timer, advance } = manualTimer();
  const applied: SyncEventKind[][] = [];
  let held = false;
  let visible = true;
  const r = createReflection({
    port: fakeSyncPort({ watch: s.watch }),
    timer,
    held: () => held,
    visible: () => visible,
    apply: async (kinds) => {
      applied.push(kinds);
    },
  });
  return {
    s,
    r,
    advance,
    applied,
    hold: (v: boolean) => {
      held = v;
    },
    show: (v: boolean) => {
      visible = v;
    },
  };
}

describe("createReflection (06「画面への反映」)", () => {
  it("reloads everything when it connects (決定 S11)", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    expect(t.applied).toEqual([["store", "sync", "github", "inbox"]]);
  });

  it("applies an event at once when nothing holds", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    t.s.send("store");
    await t.advance(0);
    expect(t.applied.at(-1)).toEqual(["store"]);
  });

  it("collects events while held and applies them once (決定 S10)", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    t.hold(true);
    t.s.send("store");
    t.s.send("sync");
    t.s.send("store");
    await t.advance(1000);
    expect(t.applied).toHaveLength(1);
    t.hold(false);
    await t.advance(500);
    expect(t.applied).toEqual([
      ["store", "sync", "github", "inbox"],
      ["store", "sync"],
    ]);
  });

  it("applies a github event", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    t.s.send("github");
    await t.advance(0);
    expect(t.applied.at(-1)).toEqual(["github"]);
  });

  it("applies an inbox event (C4a-13)", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    t.s.send("inbox");
    await t.advance(0);
    expect(t.applied.at(-1)).toEqual(["inbox"]);
  });

  it("drops the stream while hidden and reloads everything when shown (決定 S22)", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    t.show(false);
    t.r.wake();
    await t.advance(0);
    expect(t.s.aborted()).toBe(true);
    await t.advance(20_000);
    expect(t.s.connects()).toBe(1);
    t.show(true);
    t.r.wake();
    await t.advance(0);
    expect(t.s.connects()).toBe(2);
    t.s.open();
    await t.advance(0);
    expect(t.applied).toEqual([
      ["store", "sync", "github", "inbox"],
      ["store", "sync", "github", "inbox"],
    ]);
  });

  it("does not connect while it starts hidden", async () => {
    const t = setup();
    t.show(false);
    t.r.start();
    await t.advance(5000);
    expect(t.s.connects()).toBe(0);
  });

  it("applies now even while held (手動の同期。決定 S12)", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    t.hold(true);
    await t.r.now();
    expect(t.applied.at(-1)).toEqual(["store", "sync", "github", "inbox"]);
  });

  it("reconnects after 1 second, doubling up to 10 seconds", async () => {
    const t = setup();
    t.r.start();
    t.s.fail();
    await t.advance(999);
    expect(t.s.connects()).toBe(1);
    await t.advance(1);
    expect(t.s.connects()).toBe(2);
    t.s.fail();
    await t.advance(2000);
    expect(t.s.connects()).toBe(3);
    t.s.end();
    await t.advance(4000);
    expect(t.s.connects()).toBe(4);
    t.s.open(); // つながったら間隔を戻す
    t.s.end();
    await t.advance(1000);
    expect(t.s.connects()).toBe(5);
  });

  it("stops watching and reconnecting when stopped", async () => {
    const t = setup();
    const stop = t.r.start();
    stop();
    await t.advance(20_000);
    expect(t.s.connects()).toBe(1);
  });
});
