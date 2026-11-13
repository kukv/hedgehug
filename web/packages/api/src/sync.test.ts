import { NotFound } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createSyncPort } from "./sync";

afterEach(() => {
  vi.unstubAllGlobals();
});

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

describe("createSyncPort", () => {
  it("reads the status with dates", async () => {
    respond(200, {
      auto: false,
      stores: [
        {
          store: "S1",
          name: "team",
          hidden: false,
          readOnly: true,
          error: "認証に失敗しました",
          unsent: 2,
          lastSuccess: "2026-11-13T10:00:00Z",
        },
        {
          store: "S2",
          name: "solo",
          hidden: true,
          readOnly: false,
          error: "",
          unsent: 0,
          lastSuccess: "",
        },
      ],
    });
    expect(await createSyncPort().status()).toEqual({
      auto: false,
      stores: [
        {
          storeId: "S1",
          name: "team",
          hidden: false,
          readOnly: true,
          error: "認証に失敗しました",
          unsent: 2,
          lastSuccess: new Date("2026-11-13T10:00:00Z"),
        },
        {
          storeId: "S2",
          name: "solo",
          hidden: true,
          readOnly: false,
          error: "",
          unsent: 0,
          lastSuccess: null,
        },
      ],
    });
  });

  it("syncs one store and turns 404 into NotFound", async () => {
    const fetch = respond(200, { result: "failed", message: "m", mirrorWarning: "" });
    expect(await createSyncPort().syncStore("S 1")).toEqual({
      result: "failed",
      message: "m",
      mirrorWarning: "",
    });
    expect(fetch.mock.calls[0]?.[0]).toBe("/api/stores/S%201/sync");
    respond(404, { detail: "Store がありません" });
    await expect(createSyncPort().syncStore("S9")).rejects.toBeInstanceOf(NotFound);
  });

  it("sends hold and network", async () => {
    const fetch = respond(204, undefined);
    await createSyncPort().hold("T", true);
    await createSyncPort().network("T", false);
    const bodies = fetch.mock.calls.map((c) => JSON.parse(String((c[1] as RequestInit).body)));
    expect(bodies).toEqual([
      { tab: "T", hold: true },
      { tab: "T", online: false },
    ]);
  });
});

describe("watch", () => {
  function streamOf(chunks: string[]) {
    const enc = new TextEncoder();
    return new ReadableStream<Uint8Array>({
      start(c) {
        for (const ch of chunks) c.enqueue(enc.encode(ch));
        c.close();
      },
    });
  }

  it("calls open, then each event, across chunk boundaries, with the token", async () => {
    const fetch = vi.fn(
      async (_url: string, _init?: RequestInit) =>
        new Response(
          streamOf([
            'event: store\ndata: {"store":"S1"}\n\nevent: sy',
            'nc\ndata: {"store":"S1"}\n\n',
          ]),
          { status: 200, headers: { "Content-Type": "text/event-stream" } },
        ),
    );
    vi.stubGlobal("fetch", fetch);
    setToken("tok");
    const seen: string[] = [];
    await createSyncPort().watch({ open: () => seen.push("open"), event: (k) => seen.push(k) })
      .done;
    expect(seen).toEqual(["open", "store", "sync"]);
    const init = fetch.mock.calls[0]?.[1] as RequestInit;
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer tok");
  });

  it("passes github events and ignores unknown ones", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(streamOf(["event: github\ndata: {}\n\nevent: other\ndata: {}\n\n"]), {
            status: 200,
            headers: { "Content-Type": "text/event-stream" },
          }),
      ),
    );
    setToken("tok");
    const seen: string[] = [];
    await createSyncPort().watch({ open: () => seen.push("open"), event: (k) => seen.push(k) })
      .done;
    expect(seen).toEqual(["open", "github"]);
  });

  it("throws when the server refuses", async () => {
    respond(401, undefined);
    await expect(
      createSyncPort().watch({ open: () => {}, event: () => {} }).done,
    ).rejects.toThrow();
  });

  it("resolves when stopped", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        (_url: string, init: RequestInit) =>
          new Promise((_resolve, reject) => {
            init.signal?.addEventListener("abort", () => reject(new Error("aborted")));
          }),
      ),
    );
    const w = createSyncPort().watch({ open: () => {}, event: () => {} });
    w.stop();
    await expect(w.done).resolves.toBeUndefined();
  });
});
