import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createScreenStatesPort, createSearchPort } from "./search";

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

const row = {
  storeId: "s1",
  storeName: "main",
  storeColor: "#111",
  workspaceId: "w1",
  projectId: "p1",
  itemId: "i1",
  display: "HH-1",
  title: "t",
  kind: "task",
  status: null,
  assignee: null,
  priority: null,
  end: "",
  points: null,
  oldIdHit: false,
  readOnly: false,
  deleted: null,
};

describe("createSearchPort", () => {
  it("posts the query and maps the rows", async () => {
    const fetch = respond(200, {
      errors: null,
      hasKey: true,
      group: "kind",
      deleted: false,
      rows: [{ ...row, assignee: { login: "a", name: null }, points: 3 }],
    });
    const r = await createSearchPort().search("kind:task");
    expect(String(fetch.mock.calls[0]?.[0])).toBe("/api/search");
    expect(fetch.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(fetch.mock.calls[0]?.[1]?.body).toBe(JSON.stringify({ query: "kind:task" }));
    expect(r.errors).toEqual([]);
    expect(r.group).toBe("kind");
    expect(r.rows[0]).toMatchObject({
      itemId: "i1",
      assignee: { login: "a", name: null },
      points: 3,
    });
  });

  it("maps a null list of rows to an empty one", async () => {
    respond(200, { errors: ["x"], hasKey: false, group: "", deleted: false, rows: null });
    expect(await createSearchPort().search("")).toMatchObject({ errors: ["x"], rows: [] });
  });
});

describe("createScreenStatesPort", () => {
  it("gets the saved body, null when there is none", async () => {
    const fetch = respond(200, { body: null });
    expect(await createScreenStatesPort().get("search")).toBeNull();
    expect(String(fetch.mock.calls[0]?.[0])).toBe("/api/screen-states/search");
  });

  it("puts a body", async () => {
    const fetch = respond(204, undefined);
    await createScreenStatesPort().put("search", '{"query":"a"}');
    expect(String(fetch.mock.calls[0]?.[0])).toBe("/api/screen-states/search");
    expect(fetch.mock.calls[0]?.[1]?.method).toBe("PUT");
    expect(fetch.mock.calls[0]?.[1]?.body).toBe(JSON.stringify({ body: '{"query":"a"}' }));
  });
});
