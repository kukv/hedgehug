import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createItemsPort } from "./items";

afterEach(() => {
  vi.unstubAllGlobals();
});

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

describe("items port: description", () => {
  it("sends the description of a new item only when there is one", async () => {
    const fetch = vi.fn(
      async () =>
        new Response(JSON.stringify(apiItem), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", fetch);
    setToken("tok");
    const input = {
      projectId: "P1",
      parentId: null,
      kind: "Epic" as const,
      title: "認証",
      values: {},
    };
    await createItemsPort().create("S1", { ...input, description: "## 概要" });
    await createItemsPort().create("S1", input);
    const bodies = fetch.mock.calls.map((c) =>
      JSON.parse(String((c as unknown as [string, RequestInit])[1].body)),
    );
    expect(bodies[0]).toMatchObject({ description: "## 概要" });
    expect(bodies[1]).not.toHaveProperty("description");
  });
});
