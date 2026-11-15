import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createMyTasksPort } from "./myTasks";

afterEach(() => {
  vi.unstubAllGlobals();
});

function respond(body: unknown) {
  const fetch = vi.fn(
    async (_url: string, _init?: RequestInit) =>
      new Response(JSON.stringify(body), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

describe("createMyTasksPort", () => {
  it("posts the filter and turns null errors and rows into empty lists", async () => {
    const fetch = respond({ errors: null, rows: null });
    const r = await createMyTasksPort().list("priority:high");
    expect(r).toEqual({ errors: [], rows: [] });
    expect(fetch.mock.calls[0]?.[0]).toBe("/api/my-tasks");
    expect(JSON.parse(String(fetch.mock.calls[0]?.[1]?.body))).toEqual({
      filter: "priority:high",
    });
  });
});
