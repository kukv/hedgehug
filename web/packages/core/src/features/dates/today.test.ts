import { describe, expect, it, vi } from "vitest";
import { writable } from "../../reactive/store";
import { createToday, onDayChange } from "./today";

describe("today", () => {
  it("emits only when the day changes", () => {
    const now = writable(new Date(2026, 10, 15, 10));
    const seen: Date[] = [];
    createToday(now).subscribe((d) => seen.push(d));
    now.set(new Date(2026, 10, 15, 23, 59));
    expect(seen).toHaveLength(1);
    now.set(new Date(2026, 10, 16, 0, 0));
    expect(seen).toHaveLength(2);
  });

  it("onDayChange skips the first value", () => {
    const now = writable(new Date(2026, 10, 15, 10));
    const run = vi.fn();
    const stop = onDayChange(createToday(now), run);
    expect(run).not.toHaveBeenCalled();
    now.set(new Date(2026, 10, 16, 9));
    expect(run).toHaveBeenCalledTimes(1);
    stop();
    now.set(new Date(2026, 10, 17, 9));
    expect(run).toHaveBeenCalledTimes(1);
  });
});
