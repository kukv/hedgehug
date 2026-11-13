import { Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createNotesPort } from "./notes";

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

const call = (fetch: ReturnType<typeof respond>) =>
  fetch.mock.calls[0] as unknown as [string, RequestInit];

const item = { kind: "item", storeId: "S1", id: "I1" } as const;
const project = { kind: "project", storeId: "S1", id: "P1" } as const;

describe("notes port: description", () => {
  it("reads an item's description", async () => {
    const fetch = respond(200, { text: "## 手順" });
    expect(await createNotesPort().description(item)).toBe("## 手順");
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/description");
  });

  it("reads a project's description", async () => {
    const fetch = respond(200, { text: "" });
    expect(await createNotesPort().description(project)).toBe("");
    expect(call(fetch)[0]).toBe("/api/stores/S1/projects/P1/description");
  });

  it("saves with the base and the text, and returns the merged text and the conflict", async () => {
    const fetch = respond(200, { text: "合わせた", conflict: true });
    const result = await createNotesPort().describe(item, "前", "後");
    const [url, init] = call(fetch);
    expect(url).toBe("/api/stores/S1/items/I1/description");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(String(init.body))).toEqual({ base: "前", text: "後" });
    expect(result).toEqual({ text: "合わせた", conflict: true });
  });

  it("saves a project's description at its own address", async () => {
    const fetch = respond(200, { text: "後", conflict: false });
    await createNotesPort().describe(project, "前", "後");
    expect(call(fetch)[0]).toBe("/api/stores/S1/projects/P1/description");
  });

  it("turns a 422 into a refusal with the server's message", async () => {
    respond(422, { detail: "閲覧のみの Store です" });
    await expect(createNotesPort().describe(item, "", "x")).rejects.toThrow(Refused);
  });
});
