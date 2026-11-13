import { Refused } from "../../model/errors";
import type { PlanningOverview, Release, Sprint } from "../../model/planning";
import type { PlanningPort } from "../../port/planning";

// スプリントとリリースのテストの部品。

export const sprintOf = (over: Partial<Sprint> = {}): Sprint => ({
  id: "SP1",
  name: "Sprint 1",
  goal: "",
  start: "",
  end: "",
  state: "planned",
  confirmed: null,
  ...over,
});

export const releaseOf = (over: Partial<Release> = {}): Release => ({
  id: "R1",
  name: "v1.0",
  date: "",
  description: "",
  state: "unreleased",
  ...over,
});

export const overviewOf = (sprints: Sprint[] = [], releases: Release[] = []): PlanningOverview => ({
  active: sprints.find((s) => s.state === "active") ?? null,
  defaults: { name: "Sprint 9", start: "2026-11-16", end: "2026-11-29" },
  sprints,
  releases,
  retentionDays: 30,
});

export interface FakePlanning extends PlanningPort {
  calls: string[];
  // overviews は、ワークスペースの ID → 概要(ないワークスペースは空の概要)。書き換えると次の読み込みに出る。
  overviews: Record<string, PlanningOverview>;
  // refuseNext は、次の setItemSprint か setItemRelease を、この文言の断りにする(1 回だけ)。
  refuseNext(message: string): void;
  // failNext は、次の setItemSprint か setItemRelease を、断りでない失敗 e にする(1 回だけ)。
  failNext(e: Error): void;
}

// fakePlanningPort は、概要とアイテムの所属だけを答える口。ほかの操作は呼ばれたら失敗する。
export function fakePlanningPort(overviews: Record<string, PlanningOverview> = {}): FakePlanning {
  const calls: string[] = [];
  let failure: Error | null = null;
  const take = () => {
    const e = failure;
    failure = null;
    if (e) throw e;
  };
  const unused = () => Promise.reject(new Error("not used"));
  return {
    calls,
    overviews,
    refuseNext(m) {
      failure = new Refused(m);
    },
    failNext(e) {
      failure = e;
    },
    async overview(_s, w) {
      calls.push(`overview ${w}`);
      return overviews[w] ?? overviewOf();
    },
    async setItemSprint(_s, item, id) {
      take();
      calls.push(`sprint ${item} ${id}`);
    },
    async setItemRelease(_s, item, id) {
      take();
      calls.push(`release ${item} ${id}`);
    },
    sprintCards: unused,
    releaseCards: unused,
    createSprint: unused,
    updateSprint: unused,
    startSprint: unused,
    closeSprint: unused,
    deleteSprint: unused,
    createRelease: unused,
    updateRelease: unused,
    setReleaseState: unused,
    deleteRelease: unused,
  };
}
