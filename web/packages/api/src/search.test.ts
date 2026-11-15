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
  start: "",
  estimate: null,
  systems: null,
  sprint: "",
  release: "",
  project: { display: "HH-P1", title: "p" },
  parent: null,
  created: "2026-11-12T09:00:00Z",
  updated: "2026-11-12T09:00:00Z",
  unused: null,
  removed: null,
  custom: {},
};

describe("createSearchPort", () => {
  it("posts the query and maps the rows", async () => {
    const fetch = respond(200, {
      errors: null,
      hasKey: true,
      hasSort: true,
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

  it("sends sort and group only with a table, and maps columns and groups", async () => {
    const fetch = respond(200, {
      errors: [],
      hasKey: false,
      hasSort: false,
      group: "",
      deleted: false,
      rows: [row],
      columns: [{ key: "id", name: "ID", type: "", sortable: true }],
      groupBy: "kind",
      groups: [{ label: "task", rows: [0] }],
    });
    const r = await createSearchPort().search("", {
      sort: { key: "title", desc: true },
      group: "kind",
    });
    expect(fetch.mock.calls[0]?.[1]?.body).toBe(
      JSON.stringify({ query: "", sort: { key: "title", desc: true }, group: "kind" }),
    );
    expect(r.columns).toEqual([{ key: "id", name: "ID", type: "", sortable: true }]);
    expect(r.groupBy).toBe("kind");
    expect(r.groups).toEqual([{ label: "task", rows: [0] }]);
    expect(r.rows[0]).toMatchObject({ systems: [], unused: [], removed: [], parent: null });
  });

  it("maps a null list of rows to an empty one", async () => {
    respond(200, { errors: ["x"], hasKey: false, hasSort: false, group: "", deleted: false, rows: null });
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
