import { describe, expect, it } from "vitest";
import type { PlanningOverview, Sprint } from "../../model/planning";
import type { PlanningPort } from "../../port/planning";
import { get } from "../../reactive/store";
import { activeLabel, createPlanning } from "./overview";

const sprint = (over: Partial<Sprint> = {}): Sprint => ({
  id: "SP4",
  name: "Sprint 4",
  goal: "",
  start: "2026-11-02",
  end: "2026-11-15",
  state: "active",
  confirmed: null,
  ...over,
});

const overview = (active: Sprint | null): PlanningOverview => ({
  active,
  defaults: { name: "Sprint 5", start: "2026-11-16", end: "2026-11-29" },
  sprints: active ? [active] : [],
  releases: [],
  today: "2026-11-14",
});

describe("activeLabel", () => {
  it("is null without an active sprint", () => {
    expect(activeLabel(overview(null), 2026)).toBeNull();
  });

  it("shows the name and the days of this year", () => {
    expect(activeLabel(overview(sprint()), 2026)).toBe("実行中: Sprint 4(11/2〜11/15)");
  });

  it("writes the year for days of another year", () => {
    const s = sprint({ name: "Sprint 1", start: "2025-12-29", end: "2026-01-11" });
    expect(activeLabel(overview(s), 2026)).toBe("実行中: Sprint 1(2025/12/29〜1/11)");
  });
});

// portOf は、overview だけを答える口。answers に入れた順に返す(待たせるときは決まっていない Promise を入れる)。
function portOf(answers: Promise<PlanningOverview>[]) {
  const calls: string[] = [];
  const port = {
    overview: (s: string, w: string) => {
      calls.push(`${s}/${w}`);
      const next = answers.shift();
      if (!next) throw new Error("no answer");
      return next;
    },
  } as PlanningPort;
  return { port, calls };
}

// later は、あとで決める答え。
function later() {
  let resolve: (o: PlanningOverview) => void = () => {};
  let reject: (e: Error) => void = () => {};
  const promise = new Promise<PlanningOverview>((ok, ng) => {
    resolve = ok;
    reject = ng;
  });
  return { promise, resolve, reject };
}

describe("createPlanning", () => {
  it("reads the overview of the workspace it opens", async () => {
    const { port, calls } = portOf([Promise.resolve(overview(sprint()))]);
    const p = createPlanning(port);
    await p.open("S1", "W1");
    expect(calls).toEqual(["S1/W1"]);
    expect(get(p.state)).toEqual({
      storeId: "S1",
      workspaceId: "W1",
      overview: overview(sprint()),
    });
  });

  it("does nothing on reload before opening", async () => {
    const { port, calls } = portOf([]);
    await createPlanning(port).reload();
    expect(calls).toEqual([]);
  });

  it("reads again on reload", async () => {
    const { port, calls } = portOf([
      Promise.resolve(overview(null)),
      Promise.resolve(overview(sprint())),
    ]);
    const p = createPlanning(port);
    await p.open("S1", "W1");
    await p.reload();
    expect(calls).toEqual(["S1/W1", "S1/W1"]);
    expect(get(p.state)?.overview.active?.name).toBe("Sprint 4");
  });

  it("clears the old workspace while reading another", async () => {
    const second = later();
    const { port } = portOf([Promise.resolve(overview(sprint())), second.promise]);
    const p = createPlanning(port);
    await p.open("S1", "W1");
    const opening = p.open("S1", "W2");
    expect(get(p.state)).toBeNull();
    second.resolve(overview(null));
    await opening;
    expect(get(p.state)?.workspaceId).toBe("W2");
  });

  it("uses only the last of overlapping reads", async () => {
    const first = later();
    const { port } = portOf([first.promise, Promise.resolve(overview(null))]);
    const p = createPlanning(port);
    const opening = p.open("S1", "W1");
    await p.reload();
    first.resolve(overview(sprint()));
    await opening;
    expect(get(p.state)?.overview.active).toBeNull();
  });

  it("keeps the state while reading the same workspace again", async () => {
    const second = later();
    const { port } = portOf([Promise.resolve(overview(sprint())), second.promise]);
    const p = createPlanning(port);
    await p.open("S1", "W1");
    const opening = p.open("S1", "W1");
    expect(get(p.state)?.overview.active?.name).toBe("Sprint 4");
    second.resolve(overview(null));
    await opening;
  });

  it("does not throw when the overview cannot be read, and shows no label", async () => {
    const failing = later();
    const { port } = portOf([failing.promise]);
    const p = createPlanning(port);
    const opening = p.open("S1", "W1");
    failing.reject(new Error("offline"));
    await expect(opening).resolves.toBeUndefined();
    expect(get(p.state)).toBeNull();
  });

  it("keeps the last overview when a reload fails", async () => {
    const failing = later();
    const { port } = portOf([Promise.resolve(overview(sprint())), failing.promise]);
    const p = createPlanning(port);
    await p.open("S1", "W1");
    const reloading = p.reload();
    failing.reject(new Error("offline"));
    await expect(reloading).resolves.toBeUndefined();
    expect(get(p.state)?.overview.active?.name).toBe("Sprint 4");
  });

  it("stops reading after close", async () => {
    const first = later();
    const { port, calls } = portOf([first.promise]);
    const p = createPlanning(port);
    const opening = p.open("S1", "W1");
    p.close();
    first.resolve(overview(sprint()));
    await opening;
    await p.reload();
    expect(calls).toEqual(["S1/W1"]);
    expect(get(p.state)).toBeNull();
  });
});
