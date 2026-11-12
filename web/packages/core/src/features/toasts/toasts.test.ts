import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createToasts, type Timer } from "./toasts";

// fakeTimer は、時間を手で進める時計。
function fakeTimer() {
  let now = 0;
  let seq = 0;
  const pending = new Map<number, { at: number; fn: () => void }>();
  const timer: Timer = {
    set(fn, ms) {
      seq += 1;
      pending.set(seq, { at: now + ms, fn });
      return seq;
    },
    clear(handle) {
      pending.delete(handle as number);
    },
  };
  const advance = (ms: number) => {
    now += ms;
    for (const [h, p] of [...pending]) {
      if (p.at <= now) {
        pending.delete(h);
        p.fn();
      }
    }
  };
  return { timer, advance, pending };
}

const texts = (t: ReturnType<typeof createToasts>) => get(t.list).map((x) => x.text);

describe("toasts", () => {
  it("hides a toast after 4.5 seconds and an error after 7 seconds", () => {
    const { timer, advance } = fakeTimer();
    const toasts = createToasts(timer);
    toasts.show("HH-13 を作成しました");
    toasts.show("タイトルは空にできません", { error: true });
    expect(get(toasts.list)).toEqual([
      { id: "t1", text: "HH-13 を作成しました", error: false },
      { id: "t2", text: "タイトルは空にできません", error: true },
    ]);
    advance(4499);
    expect(texts(toasts)).toHaveLength(2);
    advance(1);
    expect(texts(toasts)).toEqual(["タイトルは空にできません"]);
    advance(2500);
    expect(texts(toasts)).toEqual([]);
  });

  it("keeps 4 at most and drops the oldest", () => {
    const { timer, pending } = fakeTimer();
    const toasts = createToasts(timer);
    for (const n of [1, 2, 3, 4, 5]) toasts.show(`#${n}`);
    expect(texts(toasts)).toEqual(["#2", "#3", "#4", "#5"]);
    expect(pending.size).toBe(4);
  });

  it("closes on dismiss and stops its timer", () => {
    const { timer, pending } = fakeTimer();
    const toasts = createToasts(timer);
    toasts.show("コピーしました: HH-1");
    toasts.dismiss("t1");
    expect(texts(toasts)).toEqual([]);
    expect(pending.size).toBe(0);
    toasts.dismiss("t1");
    expect(texts(toasts)).toEqual([]);
  });
});
