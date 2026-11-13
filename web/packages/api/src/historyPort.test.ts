import { NotFound } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { historyCalls } from "./historyPort";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async (_url: string) =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

const rows = [{ who: "太郎", text: "作成", at: "2026-11-13T09:00:00.000+09:00" }];

describe("history calls", () => {
  it("reads the history of an item", async () => {
    const fetch = respond(200, rows);
    expect(await historyCalls().history({ kind: "item", storeId: "S1", id: "I1" })).toEqual(rows);
    expect(fetch.mock.calls[0]?.[0]).toBe("/api/stores/S1/items/I1/history");
  });

  it("reads the history of a project", async () => {
    const fetch = respond(200, rows);
    await historyCalls().history({ kind: "project", storeId: "S1", id: "P1" });
    expect(fetch.mock.calls[0]?.[0]).toBe("/api/stores/S1/projects/P1/history");
  });

  it("reads a null list as empty, and a 404 as NotFound", async () => {
    respond(200, null);
    expect(await historyCalls().history({ kind: "item", storeId: "S1", id: "I1" })).toEqual([]);
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    const err = await historyCalls()
      .history({ kind: "item", storeId: "S1", id: "I1" })
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(NotFound);
  });
});
