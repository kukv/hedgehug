import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createPalettePort } from "./palette";

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

describe("palette port", () => {
  it("reads the index, turning null lists into empty ones", async () => {
    const fetch = respond(200, {
      items: [
        {
          storeId: "S1",
          workspaceId: "W1",
          projectId: "P1",
          itemId: "I1",
          number: 12,
          display: "HH-12",
          title: "パスワードを再設定する",
          projectTitle: "ログイン",
          aliases: [
            { id: "OLD-3", at: "2026-11-10T09:00:00.000Z" },
            { id: "HX-12", at: null },
          ],
        },
        {
          storeId: "S1",
          workspaceId: "W1",
          projectId: "P1",
          itemId: "I2",
          number: 13,
          display: "HH-13",
          title: "画面",
          projectTitle: "ログイン",
          aliases: null,
        },
      ],
      projects: [
        {
          storeId: "S1",
          workspaceId: "W1",
          projectId: "P1",
          display: "HH-P1",
          title: "ログイン",
          workspaceName: "Web",
          aliases: null,
        },
      ],
    });
    expect(await createPalettePort().index()).toEqual({
      items: [
        {
          storeId: "S1",
          workspaceId: "W1",
          projectId: "P1",
          itemId: "I1",
          number: 12,
          display: "HH-12",
          title: "パスワードを再設定する",
          projectTitle: "ログイン",
          aliases: [
            { id: "OLD-3", at: "2026-11-10T09:00:00.000Z" },
            { id: "HX-12", at: null },
          ],
        },
        {
          storeId: "S1",
          workspaceId: "W1",
          projectId: "P1",
          itemId: "I2",
          number: 13,
          display: "HH-13",
          title: "画面",
          projectTitle: "ログイン",
          aliases: [],
        },
      ],
      projects: [
        {
          storeId: "S1",
          workspaceId: "W1",
          projectId: "P1",
          display: "HH-P1",
          title: "ログイン",
          workspaceName: "Web",
          aliases: [],
        },
      ],
    });
    expect(call(fetch)[0]).toBe("/api/palette-index");
    respond(200, { items: null, projects: null });
    expect(await createPalettePort().index()).toEqual({ items: [], projects: [] });
  });

  it("reads the recent items, newest first", async () => {
    const recent = [
      { storeId: "S2", itemId: "I1", display: "TM-2", title: "画面", projectTitle: "チーム" },
    ];
    const fetch = respond(200, recent);
    expect(await createPalettePort().recent()).toEqual(recent);
    expect(call(fetch)[0]).toBe("/api/recent-items");
    respond(200, null);
    expect(await createPalettePort().recent()).toEqual([]);
  });

  it("posts the opened item", async () => {
    const fetch = respond(204, undefined);
    await createPalettePort().opened("S1", "I1");
    const [url, init] = call(fetch);
    expect(url).toBe("/api/recent-items");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ storeId: "S1", itemId: "I1" });
  });
});
