import { type Component, flushSync } from "svelte";
import { vi } from "vitest";
import { ApiError, type TaskView } from "../../core/api/client";
import type { SavedView, ViewEntry, ViewSaveBody } from "../../core/views";
import { makeCore } from "../detail/testkit";
import { renderWithCore } from "../test-utils";

/** A saved view as GET /api/views answers it: the defaults for what the file does not say. */
export function savedView(over: Partial<SavedView> = {}): SavedView {
  return {
    name: "weekly",
    layout: "table",
    query: "workspace:release -status:canceled",
    table: { columns: ["id", "title", "status", "assignee", "due"], group_by: "none" },
    kanban: { columns: "status", swimlane: "none" },
    ...over,
  };
}

export function viewEntry(
  over: Partial<SavedView> = {},
  store = "shared",
  tone: ViewEntry["tone"] = "purple",
): ViewEntry {
  return { store, tone, view: savedView(over) };
}

/** What GET /api/stores answers for one store. */
export function storeItem(name: string, isDefault = false) {
  return {
    name,
    default: isDefault,
    path: `/data/${name}`,
    remotes: [],
    account: "github.com/koki",
  };
}

function applied(v: SavedView, body: ViewSaveBody): SavedView {
  return {
    ...v,
    layout: body.layout ?? v.layout,
    query: body.query ?? v.query,
    table: {
      columns: body.columns ?? v.table.columns,
      group_by: body.group_by ?? v.table.group_by,
    },
    kanban: {
      columns: body.kanban_columns ?? v.kanban.columns,
      swimlane: body.swimlane ?? v.kanban.swimlane,
    },
  };
}

/** A ViewDirectory in memory: a write changes the list and tells the subscribers, as the real one does. */
export function makeViews(initial: ViewEntry[] = []) {
  let items = [...initial];
  const subs = new Set<() => void>();
  const notify = () => {
    for (const fn of [...subs]) fn();
  };
  return {
    list: vi.fn(() => items),
    get: vi.fn((store: string, name: string) =>
      items.find((e) => e.store === store && e.view.name === name),
    ),
    loaded: vi.fn(() => true),
    failed: vi.fn((_store: string) => false),
    subscribe: (fn: () => void) => {
      subs.add(fn);
      return () => {
        subs.delete(fn);
      };
    },
    save: vi.fn(async (store: string, name: string, body: ViewSaveBody) => {
      const at = items.find((e) => e.store === store && e.view.name === name);
      const view = applied(at?.view ?? savedView({ name, layout: "tree" }), body);
      const entry: ViewEntry = { store, tone: at?.tone ?? "purple", view };
      items = at ? items.map((e) => (e === at ? entry : e)) : [...items, entry];
      notify();
      return view;
    }),
    update: vi.fn(async (store: string, name: string, body: ViewSaveBody) => {
      const at = items.find((e) => e.store === store && e.view.name === name);
      if (!at) {
        throw new ApiError({ status: 404, type: "view.not_found", args: { name } });
      }
      const entry: ViewEntry = { ...at, view: applied(at.view, body) };
      items = items.map((e) => (e === at ? entry : e));
      notify();
      return entry.view;
    }),
    rename: vi.fn(async (store: string, name: string, to: string) => {
      const at = items.find((e) => e.store === store && e.view.name === name);
      if (!at) throw new Error("no such view");
      const entry: ViewEntry = { ...at, view: { ...at.view, name: to } };
      items = items.map((e) => (e === at ? entry : e));
      notify();
      return entry.view;
    }),
    remove: vi.fn(async (store: string, name: string) => {
      items = items.filter((e) => !(e.store === store && e.view.name === name));
      notify();
    }),
    /** Replaces the list as an event from outside would. */
    set(next: ViewEntry[]) {
      items = next;
      notify();
    },
  };
}

export type FakeViews = ReturnType<typeof makeViews>;

/** Renders a screen with the fake core of the detail screens plus the views. */
export function mountWithViews<P extends Record<string, unknown>>(
  C: Component<P>,
  props: P,
  opts: {
    views?: FakeViews;
    stores?: string[];
    readOnly?: string[];
    tasks?: TaskView[];
  } = {},
) {
  const k = makeCore({ tasks: opts.tasks });
  const readOnly = opts.readOnly ?? [];
  const items = (opts.stores ?? ["shared"]).map((n, i) => storeItem(n, i === 0));
  const stores = {
    ...k.stores,
    stores: vi.fn(() => items),
    tone: vi.fn((s: string) => (readOnly.includes(s) ? "readonly" : "purple")),
    hidden: vi.fn(() => false),
    visibleWorkspaces: vi.fn(() => []),
    sharedPrefixes: vi.fn(() => new Set<string>()),
    displayId: vi.fn((_store: string, id: string) => id),
  };
  const views = opts.views ?? makeViews();
  const { api, cache, save, schema, conn } = k;
  const r = renderWithCore(C, props, { api, cache, save, stores, schema, conn, views });
  flushSync();
  return { k, views, stores, ...r };
}
