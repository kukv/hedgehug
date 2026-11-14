import { Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createInboxPort } from "./inbox";

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

const counts = {
  all: 1,
  mention: 0,
  assign: 0,
  watch: 0,
  overwrite: 0,
  automation: 1,
  repair: 0,
};

// urlOf は、n 回目の呼び出しのアドレスを読む。
const urlOf = (fetch: ReturnType<typeof respond>, n = 0) =>
  new URL(String(fetch.mock.calls[n]?.[0]), "http://localhost");

describe("createInboxPort", () => {
  it("lists a tab, and a null list as empty", async () => {
    const fetch = respond(200, { rows: null, more: false, unread: counts });
    expect(await createInboxPort().list("automation")).toEqual({
      rows: [],
      more: false,
      unread: counts,
    });
    const url = urlOf(fetch);
    expect(url.pathname).toBe("/api/inbox");
    expect(url.searchParams.get("tab")).toBe("automation");
    expect(url.searchParams.has("before_at")).toBe(false);
    expect(url.searchParams.has("before_id")).toBe(false);
  });

  it("passes both before_at and before_id to read the next rows", async () => {
    const fetch = respond(200, { rows: [], more: false, unread: counts });
    const at = "2026-11-14T00:00:00.000000001+09:00";
    await createInboxPort().list("all", { at, id: 7 });
    const url = urlOf(fetch);
    expect(url.searchParams.get("before_at")).toBe(at);
    expect(url.searchParams.get("before_id")).toBe("7");
  });

  it("reads the unread counts", async () => {
    const fetch = respond(200, { counts });
    expect(await createInboxPort().unread()).toEqual(counts);
    expect(urlOf(fetch).pathname).toBe("/api/inbox/unread");
  });

  it("sends read, read-all, apply and keep", async () => {
    const fetch = respond(204, undefined);
    const port = createInboxPort();
    await port.setRead(3, false);
    await port.readAll("mention");
    await port.apply(4);
    await port.keep(5);
    expect(fetch.mock.calls.map((c) => [String(c[0]), c[1]?.method, c[1]?.body])).toEqual([
      ["/api/inbox/3/read", "PUT", JSON.stringify({ read: false })],
      ["/api/inbox/read-all", "POST", JSON.stringify({ tab: "mention" })],
      ["/api/inbox/4/apply", "POST", undefined],
      ["/api/inbox/5/keep", "POST", undefined],
    ]);
  });

  it("turns a 422 into Refused with the message of the server", async () => {
    const detail = "HH-1 は先に変更されたため、適用しませんでした";
    respond(422, { title: "Unprocessable Entity", status: 422, detail });
    const err = await createInboxPort()
      .apply(1)
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(Refused);
    expect((err as Error).message).toBe(detail);
  });
});
