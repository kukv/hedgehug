import { describe, expect, it } from "vitest";
import { isWatching } from "./watch";

describe("isWatching", () => {
  it("matches the account as host/login, as the watchers are written", () => {
    expect(isWatching(["sato", "github.com/koki"], "github.com/koki")).toBe(true);
  });

  it("does not match a login alone, or another login that ends the same", () => {
    expect(isWatching(["github.com/koki"], "koki")).toBe(false);
    expect(isWatching(["github.com/nonkoki"], "github.com/koki")).toBe(false);
    expect(isWatching(["sato"], "github.com/koki")).toBe(false);
  });

  it("never matches when the store has no account", () => {
    expect(isWatching(["", "sato"], "")).toBe(false);
  });
});
