import { describe, expect, it } from "vitest";
import { createSelection, type SelectedItem } from "./selection";

const sel = (source: string): SelectedItem => ({
  storeId: "S1",
  itemId: source,
  source,
  setStatus: async () => {},
});

describe("createSelection", () => {
  it("is null with nothing registered", () => {
    expect(createSelection().current()).toBeNull();
  });

  it("prefers the lowest rank (detail over tree)", () => {
    const s = createSelection();
    s.provide(1, () => sel("tree"));
    s.provide(0, () => sel("detail"));
    expect(s.current()?.source).toBe("detail");
  });

  it("falls back to the next rank when the first returns null", () => {
    const s = createSelection();
    s.provide(0, () => null);
    s.provide(1, () => sel("tree"));
    expect(s.current()?.source).toBe("tree");
  });

  it("stops providing after the returned function is called", () => {
    const s = createSelection();
    const off = s.provide(0, () => sel("detail"));
    s.provide(1, () => sel("tree"));
    off();
    expect(s.current()?.source).toBe("tree");
  });
});
