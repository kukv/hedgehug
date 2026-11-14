import { NotFound, Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createItemsPort } from "./items";
import { createPlanningPort } from "./planning";

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
  const [url, init] = fetch.mock.calls[n] as unknown as [string, RequestInit];
  return { url, method: init.method, body: init.body ? JSON.parse(String(init.body)) : undefined };
}

const base = "/api/stores/S1/workspaces/W1";

const sprint = {
  id: "SP4",
  name: "Sprint 4",
  goal: "",
  start: "2026-11-02",
  end: "2026-11-15",
  state: "active",
  confirmed: null,
};

const release = { id: "R1", name: "v1.0", date: "", description: "", state: "unreleased" };

const row = {
  id: "I1",
  display: "HH-1",
  kind: "Task",
  title: "調査",
  status: "st1",
  assignee: "",
  points: null,
};

describe("planning port: reads", () => {
  it("reads the overview into the model", async () => {
    const fetch = respond(200, {
      active: sprint,
      defaults: { name: "Sprint 5", start: "2026-11-16", end: "2026-11-29" },
      sprints: [sprint],
      releases: [release],
      retentionDays: 14,
    });
    const o = await createPlanningPort().overview("S1", "W1");
    expect(call(fetch).url).toBe(`${base}/planning`);
    expect(o).toEqual({
      active: sprint,
      defaults: { name: "Sprint 5", start: "2026-11-16", end: "2026-11-29" },
      sprints: [sprint],
      releases: [release],
      retentionDays: 14,
    });
  });

  it("reads the cards, with null lists as empty", async () => {
    const totals = { count: 1, points: 3 };
    respond(200, [{ sprint, total: totals, done: totals, undone: 0, canceled: 0, items: [row] }]);
    expect(await createPlanningPort().sprintCards("S1", "W1")).toEqual([
      { sprint, total: totals, done: totals, undone: 0, canceled: 0, items: [row] },
    ]);
    const fetch = respond(200, null);
    expect(await createPlanningPort().releaseCards("S1", "W1")).toEqual([]);
    expect(call(fetch).url).toBe(`${base}/releases`);
  });
});

describe("planning port: writes", () => {
  it("creates, updates, starts, closes and deletes a sprint", async () => {
    const input = { name: "Sprint 5", goal: "", start: "2026-11-16", end: "2026-11-29" };
    let fetch = respond(200, { ...sprint, ...input, id: "SP5", state: "planned" });
    const p = createPlanningPort();
    expect(await p.createSprint("S1", "W1", input)).toMatchObject({ id: "SP5" });
    expect(call(fetch)).toEqual({ url: `${base}/sprints`, method: "POST", body: input });

    fetch = respond(204, undefined);
    await p.updateSprint("S1", "W1", "SP5", input);
    await p.startSprint("S1", "W1", "SP5", "2026-11-16", "2026-11-29");
    await p.closeSprint("S1", "W1", "SP4", null);
    await p.closeSprint("S1", "W1", "SP4", "SP5");
    await p.deleteSprint("S1", "W1", "SP5");
    expect(fetch.mock.calls.map((_, i) => call(fetch, i))).toEqual([
      { url: `${base}/sprints/SP5`, method: "PUT", body: input },
      {
        url: `${base}/sprints/SP5/start`,
        method: "POST",
        body: { start: "2026-11-16", end: "2026-11-29" },
      },
      { url: `${base}/sprints/SP4/close`, method: "POST", body: { destination: null } },
      { url: `${base}/sprints/SP4/close`, method: "POST", body: { destination: "SP5" } },
      { url: `${base}/sprints/SP5`, method: "DELETE", body: undefined },
    ]);
  });

  it("creates, updates, changes the state of and deletes a release", async () => {
    const input = { name: "v1.0", date: "2026-12-01", description: "" };
    let fetch = respond(200, { ...release, ...input });
    const p = createPlanningPort();
    expect(await p.createRelease("S1", "W1", input)).toMatchObject({ id: "R1" });
    expect(call(fetch)).toEqual({ url: `${base}/releases`, method: "POST", body: input });

    fetch = respond(204, undefined);
    await p.updateRelease("S1", "W1", "R1", input);
    await p.setReleaseState("S1", "W1", "R1", "released");
    await p.deleteRelease("S1", "W1", "R1");
    expect(fetch.mock.calls.map((_, i) => call(fetch, i))).toEqual([
      { url: `${base}/releases/R1`, method: "PUT", body: input },
      { url: `${base}/releases/R1/state`, method: "PUT", body: { state: "released" } },
      { url: `${base}/releases/R1`, method: "DELETE", body: undefined },
    ]);
  });

  it("sends null to put an item back in the backlog or out of a release", async () => {
    const fetch = respond(204, undefined);
    const p = createPlanningPort();
    await p.setItemSprint("S1", "I1", null);
    await p.setItemSprint("S1", "I1", "SP4");
    await p.setItemRelease("S1", "I1", null);
    expect(fetch.mock.calls.map((_, i) => call(fetch, i))).toEqual([
      { url: "/api/stores/S1/items/I1/sprint", method: "PUT", body: { sprint: null } },
      { url: "/api/stores/S1/items/I1/sprint", method: "PUT", body: { sprint: "SP4" } },
      { url: "/api/stores/S1/items/I1/release", method: "PUT", body: { release: null } },
    ]);
  });

  it("turns a 422 into Refused and a 404 into NotFound", async () => {
    respond(422, { detail: "閲覧のみの Store です" });
    await expect(createPlanningPort().deleteSprint("S1", "W1", "SP4")).rejects.toBeInstanceOf(
      Refused,
    );
    respond(404, { detail: "not found" });
    await expect(createPlanningPort().overview("S1", "W1")).rejects.toBeInstanceOf(NotFound);
  });
});

// アイテムの作成で送るスプリントと、読むスプリント・リリース(F1-5)。
describe("items port: sprint and release", () => {
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
    sprint: "SP1",
    release: "",
  };
  const input = {
    projectId: "P1",
    parentId: null,
    kind: "Epic" as const,
    title: "認証",
    values: {},
  };

  it("reads the sprint and the release of an item", async () => {
    respond(200, apiItem);
    const it = await createItemsPort().create("S1", input);
    expect(it).toMatchObject({ sprint: "SP1", release: "" });
  });

  it("sends the sprint when creating, and omits it when none", async () => {
    const fetch = respond(200, apiItem);
    await createItemsPort().create("S1", { ...input, sprint: "SP1" });
    expect(call(fetch).body).toMatchObject({ sprint: "SP1" });
    await createItemsPort().create("S1", { ...input, sprint: "" });
    expect(call(fetch, 1).body).not.toHaveProperty("sprint");
  });
});
