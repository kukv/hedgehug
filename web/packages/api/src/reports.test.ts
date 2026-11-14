import { Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createReportsPort } from "./reports";

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

function call(fetch: ReturnType<typeof respond>, n = 0) {
  const [url, init] = fetch.mock.calls[n] as unknown as [string, RequestInit];
  return { url, method: init.method, body: init.body ? JSON.parse(String(init.body)) : undefined };
}

const base = "/api/stores/S1/workspaces/W1/reports";

const sprint = {
  id: "SP4",
  name: "Sprint 4",
  goal: "",
  start: "2026-11-02",
  end: "2026-11-15",
  state: "active",
  confirmed: null,
};

// 空の一覧を null で返す、最小のレポート。
const emptyView = {
  selection: { unit: "points", sprint: "", layers: "category", estimate: "" },
  unit: "points",
  fallback: false,
  sprints: null,
  burn: null,
  burndown: { days: null, shown: 0, committed: null },
  velocity: null,
  average: null,
  period: { from: "2026-10-25", to: "2026-11-14" },
  layers: null,
  flow: null,
  cycle: { n: 0, median: null, p85: null, skipped: 0 },
  lead: { n: 0, median: null, p85: null, skipped: 0 },
  estimates: null,
  estimateTotal: { points: null, estimate: null, worked: 0, diff: null },
};

describe("reports port", () => {
  it("adds from and to to the query only when a period is given", async () => {
    const fetch = respond(200, emptyView);
    await createReportsPort().report("S1", "W1", { from: "2026-11-01", to: "2026-11-14" });
    await createReportsPort().report("S1", "W1", null);
    expect(call(fetch, 0).url).toBe(`${base}?from=2026-11-01&to=2026-11-14`);
    expect(call(fetch, 1).url).toBe(base);
  });

  it("turns null lists into empty arrays", async () => {
    respond(200, emptyView);
    const report = await createReportsPort().report("S1", "W1", null);
    expect(report.sprints).toEqual([]);
    expect(report.velocity).toEqual([]);
    expect(report.flow).toEqual([]);
    expect(report.layers).toEqual([]);
    expect(report.estimates).toEqual([]);
    expect(report.burndown.days).toEqual([]);
    expect(report.burn).toBeNull();
  });

  it("keeps nested sprints and empties null flow counts", async () => {
    respond(200, {
      ...emptyView,
      sprints: [sprint],
      burn: sprint,
      velocity: [{ sprint, planned: 5, done: 3 }],
      flow: [{ date: "2026-11-14", counts: null }],
    });
    const report = await createReportsPort().report("S1", "W1", null);
    expect(report.sprints).toEqual([sprint]);
    expect(report.burn).toEqual(sprint);
    expect(report.velocity).toEqual([{ sprint, planned: 5, done: 3 }]);
    expect(report.flow).toEqual([{ date: "2026-11-14", counts: [] }]);
  });

  it("reads the active numbers and empties a null burndown list", async () => {
    const fetch = respond(200, {
      sprint,
      unit: "count",
      burndown: { days: null, shown: 0, committed: null },
      added: 2,
      done: { count: 0, points: 0 },
      total: { count: 4, points: 8 },
    });
    const active = await createReportsPort().active("S1", "W1");
    expect(call(fetch).url).toBe(`${base}/active`);
    expect(active.sprint).toEqual(sprint);
    expect(active.burndown.days).toEqual([]);
    expect(active.total).toEqual({ count: 4, points: 8 });
  });

  it("sends only the given fields when selecting", async () => {
    const fetch = respond(204, undefined);
    await createReportsPort().select("S1", "W1", { unit: "count" });
    await createReportsPort().select("S1", "W1", { sprint: "SP4", estimate: "" });
    expect(call(fetch, 0)).toEqual({
      url: `${base}/selection`,
      method: "PUT",
      body: { unit: "count" },
    });
    expect(call(fetch, 1).body).toEqual({ sprint: "SP4", estimate: "" });
  });

  it("turns a 422 into Refused", async () => {
    respond(422, { detail: "期間が合いません" });
    await expect(createReportsPort().report("S1", "W1", null)).rejects.toBeInstanceOf(Refused);
  });
});
