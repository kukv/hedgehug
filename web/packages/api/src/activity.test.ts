import { afterEach, describe, expect, it, vi } from "vitest";
import { createActivityPort } from "./activity";
import { setToken } from "./fetcher";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async (_url: string, _init?: RequestInit) =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

const urlOf = (fetch: ReturnType<typeof respond>, n = 0) =>
  new URL(String(fetch.mock.calls[n]?.[0]), "http://localhost");

describe("createActivityPort", () => {
  it("lists with only the kind for the unfiltered view, and a null list as empty", async () => {
    const fetch = respond(200, { rows: null, more: false });
    expect(await createActivityPort().list({ workspace: "", kind: "all" })).toEqual({
      rows: [],
      more: false,
    });
    const url = urlOf(fetch);
    expect(url.pathname).toBe("/api/activity");
    expect(url.searchParams.get("kind")).toBe("all");
    expect(url.searchParams.has("store")).toBe(false);
    expect(url.searchParams.has("workspace")).toBe(false);
    expect(url.searchParams.has("before")).toBe(false);
  });

  it("splits the workspace value into store and workspace, and passes before as is", async () => {
    const fetch = respond(200, { rows: [], more: true });
    const cursor = "2026-11-14T00:00:00.001Z|abc1234:0001";
    expect(await createActivityPort().list({ workspace: "S1/W1", kind: "sync" }, cursor)).toEqual({
      rows: [],
      more: true,
    });
    const url = urlOf(fetch);
    expect(url.searchParams.get("store")).toBe("S1");
    expect(url.searchParams.get("workspace")).toBe("W1");
    expect(url.searchParams.get("kind")).toBe("sync");
    expect(url.searchParams.get("before")).toBe(cursor);
  });
});
