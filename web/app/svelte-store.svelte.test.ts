import { flushSync } from "svelte";
import { describe, expect, it, vi } from "vitest";
import { useManyStores, useSubscribe } from "./svelte-store";
import { emitter } from "./test-utils";

describe("useSubscribe", () => {
  it("core の通知で読み直し、購読は不要になれば外す", async () => {
    const e = emitter();
    let n = 0;
    const store = useSubscribe({ subscribe: e.subscribe }, () => n);
    const seen: number[] = [];
    const stop = $effect.root(() => {
      $effect(() => {
        seen.push(store.current);
      });
    });
    flushSync();
    n = 1;
    e.emit();
    flushSync();
    expect(seen).toEqual([0, 1]);
    expect(e.size()).toBe(1);
    stop();
    await vi.waitFor(() => expect(e.size()).toBe(0));
  });
});

describe("useManyStores", () => {
  it("is true from two Stores, and follows the Stores", () => {
    const e = emitter();
    let names = ["a"];
    const many = useManyStores({
      subscribe: e.subscribe,
      stores: () => names.map((name) => ({ name })),
    });
    const seen: boolean[] = [];
    const stop = $effect.root(() => {
      $effect(() => {
        seen.push(many.current);
      });
    });
    flushSync();
    names = ["a", "b"];
    e.emit();
    flushSync();
    names = ["a"];
    e.emit();
    flushSync();
    expect(seen).toEqual([false, true, false]);
    stop();
  });
});
