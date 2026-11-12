import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, setToken } from "./fetcher";
import { createSystemPort } from "./system";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("system port", () => {
  it("calls GET /api/system with the token and returns the model", async () => {
    const fetch = vi.fn(
      async () => new Response(JSON.stringify({ version: "v1" }), { status: 200 }),
    );
    vi.stubGlobal("fetch", fetch);
    setToken("tok");
    await expect(createSystemPort().getSystem()).resolves.toEqual({ version: "v1" });
    const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/system");
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer tok");
  });

  it("throws ApiError with the status when the server refuses", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("unauthorized", { status: 401 })),
    );
    await expect(createSystemPort().getSystem()).rejects.toMatchObject({ status: 401 });
    await expect(createSystemPort().getSystem()).rejects.toBeInstanceOf(ApiError);
  });
});
