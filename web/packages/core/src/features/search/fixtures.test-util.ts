// 検索の流れ(createSearch)のテストの材料。search.test.ts と searchTable.test.ts で使う。
import { vi } from "vitest";
import type { ScreenStatesPort, SearchPort } from "../../port/search";
import { createScreenStates } from "../screenstate/screenStates";
import { createToasts, type Timer } from "../toasts/toasts";
import { createSearch } from "./search";
import { defaultTable } from "./table";
import type { SearchResult } from "./types";

// core の lib は ES2023 だけで setTimeout の型がないため、ここで宣言する。
declare function setTimeout(fn: () => void, ms: number): unknown;
declare function clearTimeout(handle: unknown): void;

export const timer: Timer = {
  set: (fn, ms) => setTimeout(fn, ms),
  clear: (h) => clearTimeout(h),
};

export const noTable = { sort: null, group: "none" };
const body = (query: string) =>
  JSON.stringify({ query, layout: "table", table: defaultTable, tree: { collapsed: [] } });
export const kindBugBody = body("kind:bug");
export const newBody = body("new");

export const result = (...ids: string[]): SearchResult => ({
  errors: [],
  hasKey: true,
  hasSort: false,
  group: "",
  deleted: false,
  rows: ids.map((id) => ({
    storeId: "s1",
    storeName: "main",
    storeColor: "#000",
    workspaceId: "w1",
    projectId: "p1",
    itemId: id,
    display: id,
    title: id,
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
    systems: [],
    sprint: "",
    release: "",
    project: { display: "HH-P1", title: "p" },
    parent: null,
    created: "",
    updated: "",
    unused: [],
    removed: [],
    custom: {},
  })),
  columns: [],
  groupBy: "",
  groups: [],
});

export function setup(saved: string | null = null) {
  const prune = vi.fn();
  const clear = vi.fn();
  const search = vi.fn(async (q: string) => result(q));
  const port: SearchPort = { search };
  const sp: ScreenStatesPort = {
    get: vi.fn(async () => saved),
    put: vi.fn(async () => {}),
  };
  const toasts = createToasts(timer);
  const s = createSearch({
    port,
    screenStates: createScreenStates(sp),
    timer,
    toasts,
    selection: { prune, clear },
  });
  return { s, search, sp, toasts, prune, clear };
}

// opened は、open が画面の状態を読み終えたあとの setup(open の検索と書き込みの記録は消す)。読む前の setter は
// 書かずに積み、検索も open に任せる(E3a-23)ので、読んだあとの動きを確かめるテストはこれを使う。
export async function opened(saved: string | null = null) {
  const t = setup(saved);
  await t.s.open();
  t.search.mockClear();
  vi.mocked(t.sp.put).mockClear();
  return t;
}
