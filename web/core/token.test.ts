import { describe, expect, it, vi } from "vitest";
import { getToken, takeToken } from "./token";

function loc(hash: string): Location {
  return { hash, pathname: "/projects/X", search: "?a=1" } as Location;
}

describe("takeToken", () => {
  it("moves #token= into the storage and clears the hash", () => {
    sessionStorage.clear();
    const history = { replaceState: vi.fn() } as unknown as History;
    expect(takeToken(loc("#token=abc123"), sessionStorage, history)).toBe("abc123");
    expect(sessionStorage.getItem("hedgehug.token")).toBe("abc123");
    expect(history.replaceState).toHaveBeenCalledWith(null, "", "/projects/X?a=1");
  });

  it("returns the stored token when the URL has none", () => {
    sessionStorage.setItem("hedgehug.token", "kept");
    const history = { replaceState: vi.fn() } as unknown as History;
    expect(takeToken(loc(""), sessionStorage, history)).toBe("kept");
    expect(history.replaceState).not.toHaveBeenCalled();
  });

  it("returns null when there is no token anywhere", () => {
    sessionStorage.clear();
    const history = { replaceState: vi.fn() } as unknown as History;
    expect(takeToken(loc("#foo=1"), sessionStorage, history)).toBeNull();
    expect(getToken(sessionStorage)).toBeNull();
  });
});
