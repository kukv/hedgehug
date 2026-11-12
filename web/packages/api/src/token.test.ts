import { describe, expect, it } from "vitest";
import { tokenFromHash } from "./token";

describe("tokenFromHash", () => {
  it("reads the token from the hash that the server printed", () => {
    expect(tokenFromHash("#token=abc123")).toBe("abc123");
  });

  it("returns null when the URL has no token", () => {
    expect(tokenFromHash("")).toBeNull();
    expect(tokenFromHash("#foo=1")).toBeNull();
    expect(tokenFromHash("#token=")).toBeNull();
  });
});
