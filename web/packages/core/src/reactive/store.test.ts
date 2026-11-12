import { describe, expect, it } from "vitest";
import { get, readonly, writable } from "./store";

describe("writable", () => {
  it("calls a subscriber at once and on each change", () => {
    const s = writable(1);
    const seen: number[] = [];
    const stop = s.subscribe((v) => seen.push(v));
    s.set(2);
    s.update((v) => v + 1);
    stop();
    s.set(9);
    expect(seen).toEqual([1, 2, 3]);
  });

  it("does not call subscribers when the value is the same", () => {
    const s = writable("a");
    const seen: string[] = [];
    s.subscribe((v) => seen.push(v));
    s.set("a");
    expect(seen).toEqual(["a"]);
  });

  it("reads the current value with get, and readonly hides set", () => {
    const s = writable({ n: 1 });
    const r = readonly(s);
    s.set({ n: 2 });
    expect(get(r)).toEqual({ n: 2 });
    expect("set" in r).toBe(false);
  });
});
