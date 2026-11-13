import { afterEach, describe, expect, it, vi } from "vitest";
import { createCatalogPort } from "./catalog";
import { setToken } from "./fetcher";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

function call(fetch: ReturnType<typeof respond>, n = 0) {
  return fetch.mock.calls[n] as unknown as [string, RequestInit];
}

describe("catalog port", () => {
  it("adds the store ID to workspaces and reads null lists as empty", async () => {
    respond(200, [{ id: "W1", name: "Web", prefix: "WEB", oldPrefixes: ["HH"] }]);
    await expect(createCatalogPort().listWorkspaces("S1")).resolves.toEqual([
      { id: "W1", storeId: "S1", name: "Web", prefix: "WEB", oldPrefixes: ["HH"] },
    ]);
    respond(200, [{ id: "W1", name: "Web", prefix: "HH", oldPrefixes: null }]);
    await expect(createCatalogPort().listWorkspaces("S1")).resolves.toEqual([
      { id: "W1", storeId: "S1", name: "Web", prefix: "HH", oldPrefixes: [] },
    ]);
    respond(200, null);
    await expect(createCatalogPort().listStores()).resolves.toEqual([]);
  });

  it("reads the login of me", async () => {
    const fetch = respond(200, { login: "taro" });
    await expect(createCatalogPort().me("S1")).resolves.toBe("taro");
    expect(call(fetch)[0]).toBe("/api/stores/S1/me");
  });
});
