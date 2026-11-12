import { describe, expect, it, vi } from "vitest";
import { newTabUrl, rememberToken, tokenFromHash, tokenKey } from "./token";

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

function memoryStorage() {
  const map = new Map<string, string>();
  return {
    map,
    getItem: (key: string) => map.get(key) ?? null,
    setItem: (key: string, value: string) => {
      map.set(key, value);
    },
  };
}

describe("rememberToken", () => {
  it("moves the token from the hash to the storage and removes the hash", () => {
    const storage = memoryStorage();
    const replace = vi.fn();
    const token = rememberToken(
      { hash: "#token=abc", pathname: "/s/work/i/HH-1", search: "" },
      storage,
      replace,
    );
    expect(token).toBe("abc");
    expect(storage.map.get(tokenKey)).toBe("abc");
    expect(replace).toHaveBeenCalledExactlyOnceWith("/s/work/i/HH-1");
  });

  it("reads the stored token after a reload, without touching the address", () => {
    const storage = memoryStorage();
    storage.setItem(tokenKey, "abc");
    const replace = vi.fn();
    expect(rememberToken({ hash: "", pathname: "/", search: "" }, storage, replace)).toBe("abc");
    expect(replace).not.toHaveBeenCalled();
  });

  it("returns empty when there is no token anywhere", () => {
    expect(rememberToken({ hash: "", pathname: "/", search: "" }, memoryStorage(), vi.fn())).toBe(
      "",
    );
  });
});

describe("newTabUrl", () => {
  it("puts the token in the hash, where tokenFromHash reads it", () => {
    const url = newTabUrl("/s/work/i/HH-1", "abc");
    expect(url).toBe("/s/work/i/HH-1#token=abc");
    expect(tokenFromHash(url.slice(url.indexOf("#")))).toBe("abc");
  });
});
