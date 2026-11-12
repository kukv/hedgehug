import { NotFound, Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createCatalogPort } from "./catalog";
import { setToken } from "./fetcher";
import { createItemsPort } from "./items";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      // 204 の応答は本文を持てない(Response は null の本文だけを受ける)。
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

const apiItem = {
  id: "I1",
  workspace: "W1",
  project: "P1",
  parent: null,
  number: 13,
  kind: "Epic",
  rank: "V",
  title: "認証",
  watchers: null,
  createdAt: "2026-11-13T09:00:00.000Z",
  createdBy: "taro",
};

describe("items port", () => {
  it("creates an item with an empty parent for none, and returns the model", async () => {
    const fetch = respond(200, apiItem);
    const it = await createItemsPort().create("S1", {
      projectId: "P1",
      parentId: null,
      kind: "Epic",
      title: "認証",
    });
    const [url, init] = call(fetch);
    expect(url).toBe("/api/stores/S1/items");
    expect(JSON.parse(String(init.body))).toEqual({
      project: "P1",
      parent: "",
      kind: "Epic",
      title: "認証",
    });
    expect(it).toMatchObject({ id: "I1", storeId: "S1", parentId: null, watchers: [] });
  });

  it("turns a 422 into Refused with the words of the spec", async () => {
    respond(422, {
      title: "Unprocessable Entity",
      status: 422,
      detail: "タイトルは空にできません",
    });
    const err = await createItemsPort()
      .rename("S1", "I1", " ")
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(Refused);
    expect((err as Refused).message).toBe("タイトルは空にできません");
  });

  it("turns a 404 into NotFound", async () => {
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createItemsPort().findByNumber("S1", "W1", 99)).rejects.toBeInstanceOf(NotFound);
  });

  it("reads the next number and moves up", async () => {
    const fetch = respond(200, { number: 14 });
    await expect(createItemsPort().nextNumber("S1", "W1")).resolves.toBe(14);
    expect(call(fetch)[0]).toBe("/api/stores/S1/workspaces/W1/next-item-number");
    const moved = respond(204, undefined);
    await expect(createItemsPort().moveUp("S1", "I1")).resolves.toBeUndefined();
    expect(call(moved)[0]).toBe("/api/stores/S1/items/I1/move-up");
  });
});

describe("catalog port", () => {
  it("adds the store ID to workspaces and reads null lists as empty", async () => {
    respond(200, [{ id: "W1", name: "Web", prefix: "HH" }]);
    await expect(createCatalogPort().listWorkspaces("S1")).resolves.toEqual([
      { id: "W1", storeId: "S1", name: "Web", prefix: "HH" },
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
