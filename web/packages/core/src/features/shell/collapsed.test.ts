import { describe, expect, it, vi } from "vitest";
import type { ScreenStates } from "../screenstate/screenStates";
import { createCollapsed } from "./collapsed";

const statesOf = (value: unknown, write = vi.fn()): ScreenStates => ({
  read: vi.fn(async (_screen: string, fallback: unknown) =>
    value === undefined ? fallback : value,
  ) as ScreenStates["read"],
  write,
});

describe("createCollapsed (G1-14)", () => {
  it("reads the saved ids of the screen sidebar", async () => {
    const c = createCollapsed(statesOf({ collapsed: ["w1"] }));
    await c.ready;
    expect(c.has("w1")).toBe(true);
    expect(c.has("w2")).toBe(false);
  });

  it("writes the whole list when toggled", async () => {
    const write = vi.fn();
    const c = createCollapsed(statesOf({ collapsed: ["w1"] }, write));
    await c.ready;
    c.toggle("w2");
    expect(write).toHaveBeenCalledWith("sidebar", { collapsed: ["w1", "w2"] });
    c.toggle("w1");
    expect(write).toHaveBeenLastCalledWith("sidebar", { collapsed: ["w2"] });
    expect(c.has("w1")).toBe(false);
  });

  it("treats a broken value or an unreadable state as empty", async () => {
    for (const value of [undefined, "x", { collapsed: "w1" }, { collapsed: [1, "w1"] }, null]) {
      const c = createCollapsed(statesOf(value));
      await c.ready;
      expect(c.has("w1")).toBe(false);
    }
  });

  it("keeps a toggle made before the read resolves", async () => {
    const write = vi.fn();
    let release: (v: unknown) => void = () => {};
    const states: ScreenStates = {
      read: vi.fn(() => new Promise((r) => (release = r))) as ScreenStates["read"],
      write,
    };
    const c = createCollapsed(states);
    c.toggle("w2");
    expect(c.has("w2")).toBe(true);
    expect(write).not.toHaveBeenCalled();
    release({ collapsed: ["w1"] });
    await c.ready;
    expect(c.has("w1")).toBe(true);
    expect(c.has("w2")).toBe(true);
    expect(write).toHaveBeenCalledWith("sidebar", { collapsed: ["w1", "w2"] });
  });
});
