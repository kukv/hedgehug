import { flushSync } from "svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { online } from "./online";

afterEach(() => vi.restoreAllMocks());

describe("online", () => {
  it("navigator.onLine を読み、online / offline イベントで購読者へ届け、解除できる", () => {
    const onLine = vi.spyOn(navigator, "onLine", "get").mockReturnValue(true);
    const seen: boolean[] = [];
    const stop = $effect.root(() => {
      $effect(() => {
        seen.push(online.current);
      });
    });
    flushSync();
    expect(seen).toEqual([true]);

    onLine.mockReturnValue(false);
    window.dispatchEvent(new Event("offline"));
    flushSync();
    expect(seen).toEqual([true, false]);

    onLine.mockReturnValue(true);
    window.dispatchEvent(new Event("online"));
    flushSync();
    expect(seen).toEqual([true, false, true]);

    stop();
    onLine.mockReturnValue(false);
    window.dispatchEvent(new Event("offline"));
    flushSync();
    expect(seen).toHaveLength(3);
  });
});
