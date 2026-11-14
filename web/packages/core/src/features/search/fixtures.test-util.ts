// 検索の流れ(createSearch)のテストの材料。search.test.ts と searchTable.test.ts で使う。
import type { Timer } from "../toasts/toasts";
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
const body = (query: string) => JSON.stringify({ query, table: defaultTable });
export const kindBugBody = body("kind:bug");
export const newBody = body("new");

export const result = (...ids: string[]): SearchResult => ({
  errors: [],
  hasKey: true,
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
