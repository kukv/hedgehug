import { NotFound, Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createCatalogPort } from "./catalog";
import { setToken } from "./fetcher";
import { createItemsPort } from "./items";
import { createWorkspacesPort } from "./workspaces";

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
  blocked: false,
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
      values: {},
    });
    const [url, init] = call(fetch);
    expect(url).toBe("/api/stores/S1/items");
    expect(JSON.parse(String(init.body))).toEqual({
      project: "P1",
      parent: "",
      kind: "Epic",
      title: "認証",
      values: {},
    });
    expect(it).toMatchObject({ id: "I1", storeId: "S1", parentId: null, watchers: [] });
  });

  it("sends the template id when creating, and omits it when none", async () => {
    const fetch = respond(200, apiItem);
    const input = {
      projectId: "P1",
      parentId: "E1",
      kind: "Task" as const,
      title: "調査",
      values: {},
    };
    await createItemsPort().create("S1", { ...input, templateId: "T1" });
    expect(JSON.parse(String(call(fetch)[1].body))).toMatchObject({ template: "T1" });
    await createItemsPort().create("S1", input);
    expect(JSON.parse(String(call(fetch, 1)[1].body))).not.toHaveProperty("template");
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

  it("reads the next number and reorders", async () => {
    const fetch = respond(200, { number: 14 });
    await expect(createItemsPort().nextNumber("S1", "W1")).resolves.toBe(14);
    expect(call(fetch)[0]).toBe("/api/stores/S1/workspaces/W1/next-item-number");
    const moved = respond(204, undefined);
    await expect(createItemsPort().reorder("S1", "I1", "I0", true)).resolves.toBeUndefined();
    expect(call(moved)[0]).toBe("/api/stores/S1/items/I1/reorder");
    expect(call(moved)[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({ anchor: "I0", after: true }),
    });
  });
});

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

const apiValue = { id: "st1", name: "未着手", color: "gray", category: "todo" };

describe("workspaces port", () => {
  it("reads the detail into the model, with null lists as empty", async () => {
    const fetch = respond(200, {
      id: "W1",
      name: "Web",
      prefix: "HH",
      oldPrefixes: null,
      settings: {
        categoryDefaults: { todo: "st1" },
        initialPriority: "pr1",
        initialStatus: "st1",
        priorities: [{ id: "pr1", name: "中", color: "yellow" }],
        statuses: [apiValue],
        systems: null,
        properties: [
          {
            id: "p1",
            key: "area",
            name: "領域",
            type: "single",
            builtin: false,
            hidden: false,
            kinds: null,
            choices: null,
          },
        ],
      },
    });
    const d = await createWorkspacesPort().get("S1", "W1");
    expect(call(fetch)[0]).toBe("/api/stores/S1/workspaces/W1");
    expect(d).toMatchObject({ id: "W1", storeId: "S1", oldPrefixes: [] });
    expect(d.settings.systems).toEqual([]);
    expect(d.settings.statuses).toEqual([apiValue]);
    expect(d.settings.properties[0]).toMatchObject({ kinds: [], choices: [], type: "single" });
  });

  it("reads the usage with null maps as empty", async () => {
    const fetch = respond(200, { items: 3, firstNumbers: [1, 2, 3], values: { st1: 2 } });
    await expect(createWorkspacesPort().usage("S1", "W1")).resolves.toEqual({
      items: 3,
      firstNumbers: [1, 2, 3],
      values: { st1: 2 },
      properties: {},
    });
    expect(call(fetch)[0]).toBe("/api/stores/S1/workspaces/W1/usage");
  });

  it("creates a workspace and returns the model", async () => {
    const fetch = respond(200, { id: "W2", name: "API", prefix: "AP" });
    const w = await createWorkspacesPort().create("S1", "API", "AP");
    const [url, init] = call(fetch);
    expect(url).toBe("/api/stores/S1/workspaces");
    expect(JSON.parse(String(init.body))).toEqual({ name: "API", prefix: "AP" });
    expect(w).toEqual({ id: "W2", storeId: "S1", name: "API", prefix: "AP" });
  });

  it("sends the body of a rename, a prefix change and a move", async () => {
    const port = createWorkspacesPort();
    const rename = respond(204, undefined);
    await port.rename("S1", "W1", "フロント");
    expect(JSON.parse(String(call(rename)[1].body))).toEqual({ name: "フロント" });
    const prefix = respond(204, undefined);
    await port.changePrefix("S1", "W1", "WEB");
    expect(JSON.parse(String(call(prefix)[1].body))).toEqual({ prefix: "WEB" });
    const move = respond(204, undefined);
    await port.move("S1", "W1", "down");
    expect(call(move)[0]).toBe("/api/stores/S1/workspaces/W1/move");
    expect(JSON.parse(String(call(move)[1].body))).toEqual({ direction: "down" });
  });

  it("adds a property and sends the other property operations to their addresses", async () => {
    const port = createWorkspacesPort();
    const add = respond(200, {
      id: "p1",
      key: "area",
      name: "領域",
      type: "single",
      builtin: false,
      hidden: false,
      kinds: ["Epic"],
      choices: [],
    });
    const p = await port.addProperty("S1", "W1", {
      name: "領域",
      key: "area",
      type: "single",
      kinds: ["Epic"],
    });
    expect(JSON.parse(String(call(add)[1].body))).toEqual({
      name: "領域",
      key: "area",
      type: "single",
      kinds: ["Epic"],
    });
    expect(p).toMatchObject({ id: "p1", kinds: ["Epic"] });
    const kinds = respond(204, undefined);
    await port.setPropertyKinds("S1", "W1", "p1", ["Task"]);
    expect(JSON.parse(String(call(kinds)[1].body))).toEqual({ kinds: ["Task"] });
    const hidden = respond(204, undefined);
    await port.setPropertyHidden("S1", "W1", "p1", true);
    expect(JSON.parse(String(call(hidden)[1].body))).toEqual({ hidden: true });
    const del = respond(204, undefined);
    await port.deleteProperty("S1", "W1", "p1");
    expect(call(del)[1].method).toBe("DELETE");
    expect(call(del)[0]).toContain("/workspaces/W1/");
  });

  it("deletes a value with the replacement as a query, or without one", async () => {
    const port = createWorkspacesPort();
    const withR = respond(204, undefined);
    await port.deleteValue("S1", "W1", "statuses", "st2", "st1");
    expect(call(withR)[0]).toContain("replacement=st1");
    const without = respond(204, undefined);
    await port.deleteValue("S1", "W1", "statuses", "st2", "");
    expect(call(without)[0]).not.toContain("replacement");
  });

  it("sends the value operations and the defaults", async () => {
    const port = createWorkspacesPort();
    const add = respond(200, apiValue);
    await expect(port.addValue("S1", "W1", "statuses", "未着手")).resolves.toEqual(apiValue);
    expect(JSON.parse(String(call(add)[1].body))).toEqual({ name: "未着手" });
    const color = respond(204, undefined);
    await port.setValueColor("S1", "W1", "statuses", "st1", "blue");
    expect(JSON.parse(String(call(color)[1].body))).toEqual({ color: "blue" });
    const category = respond(204, undefined);
    await port.setStatusCategory("S1", "W1", "st1", "doing");
    expect(JSON.parse(String(call(category)[1].body))).toEqual({ category: "doing" });
    const initial = respond(204, undefined);
    await port.setInitialStatus("S1", "W1", "st2");
    expect(JSON.parse(String(call(initial)[1].body))).toEqual({ value: "st2" });
    const def = respond(204, undefined);
    await port.setCategoryDefault("S1", "W1", "done", "st4");
    expect(call(def)[0]).toBe("/api/stores/S1/workspaces/W1/category-defaults/done");
    expect(JSON.parse(String(call(def)[1].body))).toEqual({ value: "st4" });
  });

  it("turns a 422 into Refused", async () => {
    respond(422, { title: "Unprocessable Entity", status: 422, detail: "接頭辞が重複します" });
    await expect(createWorkspacesPort().changePrefix("S1", "W1", "HH")).rejects.toBeInstanceOf(
      Refused,
    );
  });
});
