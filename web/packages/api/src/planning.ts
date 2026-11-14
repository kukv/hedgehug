import type {
  BacklogFrame,
  BacklogRow,
  PlanningOverview,
  PlanningPort,
  PlanRow,
  Release,
  ReleaseCard,
  Sprint,
  SprintCard,
} from "@hedgehug/core";
import { call } from "./errors";
import {
  type BacklogFrame as ApiBacklogFrame,
  type BacklogRow as ApiBacklogRow,
  type PlanningOverview as ApiOverview,
  type Release as ApiRelease,
  type ReleaseCard as ApiReleaseCard,
  type PlanningRow as ApiRow,
  type Sprint as ApiSprint,
  type SprintCard as ApiSprintCard,
  closeSprint,
  createRelease,
  createSprint,
  deleteRelease,
  deleteSprint,
  getBacklog,
  getPlanning,
  listReleaseCards,
  listSprintCards,
  placeItem,
  setItemRelease,
  setItemSprint,
  setReleaseState,
  startSprint,
  updateRelease,
  updateSprint,
} from "./generated";

// スプリントとリリースの口の実装。API の型を core の型に直す(設計書 6 節)。
// 空の一覧は null で届いてもよいので、空にする。復元はトラック G が足す。

// API の列挙(状態・種類)は core の型と同じ文字列。
const toSprint = (s: ApiSprint): Sprint => s;
const toRelease = (r: ApiRelease): Release => r;
const toRow = (r: ApiRow): PlanRow => r;

const toBacklogRow = (r: ApiBacklogRow): BacklogRow => r;

const toBacklogFrame = (f: ApiBacklogFrame): BacklogFrame => ({
  sprint: f.sprint ? toSprint(f.sprint) : null,
  total: f.total,
  rows: (f.rows ?? []).map(toBacklogRow),
});

const toOverview = (o: ApiOverview): PlanningOverview => ({
  active: o.active ?? null,
  defaults: o.defaults,
  sprints: (o.sprints ?? []).map(toSprint),
  releases: (o.releases ?? []).map(toRelease),
  retentionDays: o.retentionDays,
});

const toSprintCard = (c: ApiSprintCard): SprintCard => ({
  ...c,
  items: (c.items ?? []).map(toRow),
});

const toReleaseCard = (c: ApiReleaseCard): ReleaseCard => ({
  ...c,
  items: (c.items ?? []).map(toRow),
});

export function createPlanningPort(): PlanningPort {
  return {
    overview: (s, w) => call(async () => toOverview(await getPlanning(s, w))),
    sprintCards: (s, w) =>
      call(async () => ((await listSprintCards(s, w)) ?? []).map(toSprintCard)),
    releaseCards: (s, w) =>
      call(async () => ((await listReleaseCards(s, w)) ?? []).map(toReleaseCard)),
    backlog: (s, w) =>
      call(async () => ((await getBacklog(s, w)).frames ?? []).map(toBacklogFrame)),
    createSprint: (s, w, input) => call(async () => toSprint(await createSprint(s, w, input))),
    updateSprint: (s, w, id, input) => call(() => updateSprint(s, w, id, input)),
    startSprint: (s, w, id, start, end) => call(() => startSprint(s, w, id, { start, end })),
    closeSprint: (s, w, id, destination) => call(() => closeSprint(s, w, id, { destination })),
    deleteSprint: (s, w, id) => call(() => deleteSprint(s, w, id)),
    createRelease: (s, w, input) => call(async () => toRelease(await createRelease(s, w, input))),
    updateRelease: (s, w, id, input) => call(() => updateRelease(s, w, id, input)),
    setReleaseState: (s, w, id, state) => call(() => setReleaseState(s, w, id, { state })),
    deleteRelease: (s, w, id) => call(() => deleteRelease(s, w, id)),
    setItemSprint: (s, item, sprint) => call(() => setItemSprint(s, item, { sprint })),
    setItemRelease: (s, item, release) => call(() => setItemRelease(s, item, { release })),
    placeItem: (s, item, sprint, anchor, after) =>
      call(() => placeItem(s, item, { sprint, anchor, after })),
  };
}
