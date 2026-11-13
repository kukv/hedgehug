import { Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createItemsPort } from "./items";
import { relationCalls } from "./relationsPort";

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

describe("relations calls", () => {
  it("reads null lists as empty", async () => {
    const fetch = respond(200, null);
    expect(await relationCalls().listRelations("S1", "I1")).toEqual([]);
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/relations");
  });

  it("asks for candidates with the label and the query", async () => {
    const fetch = respond(200, [
      { id: "I3", display: "HH-3", title: "決済", workspaceName: "", disabledReason: "" },
    ]);
    const got = await relationCalls().relationCandidates("S1", "I1", "blocked by", "決済");
    expect(got).toEqual([
      { id: "I3", display: "HH-3", title: "決済", workspaceName: "", disabledReason: "" },
    ]);
    const url = new URL(call(fetch)[0], "http://x");
    expect(url.pathname).toBe("/api/stores/S1/items/I1/relation-candidates");
    expect(url.searchParams.get("label")).toBe("blocked by");
    expect(url.searchParams.get("q")).toBe("決済");
  });

  it("adds and removes with the label and the other item", async () => {
    const fetch = respond(204, undefined);
    await relationCalls().addRelation("S1", "I1", "blocks", "I3");
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/relations");
    expect(call(fetch)[1].method).toBe("POST");
    expect(JSON.parse(String(call(fetch)[1].body))).toEqual({ label: "blocks", other: "I3" });
    await relationCalls().removeRelation("S1", "I1", "blocked by", "I3");
    expect(call(fetch, 1)[0]).toBe("/api/stores/S1/items/I1/relations/remove");
    expect(JSON.parse(String(call(fetch, 1)[1].body))).toEqual({
      label: "blocked by",
      other: "I3",
    });
  });

  it("turns a 422 into Refused with the words of the spec", async () => {
    respond(422, { title: "Unprocessable Entity", status: 422, detail: "循環になります" });
    const err = await relationCalls()
      .addRelation("S1", "I1", "blocks", "I3")
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(Refused);
    expect((err as Refused).message).toBe("循環になります");
  });
});

describe("items port: blocked", () => {
  it("reads blocked from the item", async () => {
    respond(200, {
      id: "I1",
      workspace: "W1",
      project: "P1",
      parent: null,
      number: 1,
      kind: "Task",
      rank: "V",
      title: "画面",
      watchers: [],
      createdAt: "2026-11-13T09:00:00.000Z",
      createdBy: "taro",
      blocked: true,
    });
    expect(await createItemsPort().get("S1", "I1")).toMatchObject({ id: "I1", blocked: true });
  });
});
