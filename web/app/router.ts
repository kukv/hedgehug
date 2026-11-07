import { createSubscriber } from "svelte/reactivity";
import { INBOX_TABS } from "../core/notifications";
import type { Layout } from "../core/views";

export type { Layout };
export type WorkspaceTab =
  | "overview"
  | "tree"
  | "kanban"
  | "table"
  | "backlog"
  | "sprints"
  | "releases"
  | "gantt"
  | "reports"
  | "settings";
export type ProjectTab = "overview" | "tree" | "kanban" | "table" | "gantt" | "settings";
// "general", "accounts" and "stores" are of this machine and names no Store; the pages of a Store carry it.
// The first tab of each is the one without ?tab=.
export const SCHEMA_TABS = [
  "statuses",
  "kinds",
  "props",
  "relations",
  "workflow",
  "templates",
] as const;
export type SchemaTab = (typeof SCHEMA_TABS)[number];
export type SettingsPage =
  | { page: "general" }
  | { page: "accounts" }
  | { page: "stores" }
  | { page: "workspaces"; store: string }
  | { page: "automations"; store: string }
  | { page: "schema"; store: string; tab: SchemaTab };

// "" is every commit; exclude is the ones of people, only the ones of syncs, automation the ones of
// the automations (the fetches, the rules and the issues).
export type ActivitySync = "" | "exclude" | "only" | "automation";

export type Route =
  | { name: "home" }
  | { name: "workspace"; store: string; ulid: string; tab: WorkspaceTab }
  | { name: "project"; ulid: string; tab: ProjectTab }
  | { name: "task"; ulid: string }
  | { name: "view"; store: string; view: string }
  | { name: "query"; layout: Layout; q: string }
  | { name: "my" }
  | { name: "inbox"; tab: (typeof INBOX_TABS)[number]["tab"] }
  | { name: "activity"; workspaces: string[]; sync: ActivitySync }
  | ({ name: "settings" } & SettingsPage);

const WORKSPACE_TABS: readonly string[] = [
  "tree",
  "kanban",
  "table",
  "backlog",
  "sprints",
  "releases",
  "gantt",
  "reports",
  "settings",
];
const PROJECT_TABS: readonly string[] = [
  "overview",
  "tree",
  "kanban",
  "table",
  "gantt",
  "settings",
];
const LAYOUTS: readonly string[] = ["tree", "kanban", "table", "gantt"];

function decode(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}

/** The route of a path, or null when the path is none of ours. Only /query/<layout> (?q=), /activity (?workspace=, ?sync=), /inbox (?tab=) and /settings/schema/<store> (?tab=) read the search. */
function matchRoute(pathname: string, search: string): Route | null {
  const seg = pathname.split("/").filter(Boolean).map(decode);
  const [head, a, b, c] = seg;
  if (head === undefined) return { name: "home" };
  if (head === "workspaces" && a && b) {
    if (seg.length === 3) return { name: "workspace", store: a, ulid: b, tab: "overview" };
    if (seg.length === 4 && c && WORKSPACE_TABS.includes(c)) {
      return { name: "workspace", store: a, ulid: b, tab: c as WorkspaceTab };
    }
    return null;
  }
  if (head === "projects" && a) {
    if (seg.length === 2) return { name: "project", ulid: a, tab: "tree" };
    if (seg.length === 3 && b && PROJECT_TABS.includes(b)) {
      return { name: "project", ulid: a, tab: b as ProjectTab };
    }
    return null;
  }
  if (head === "tasks" && seg.length === 2 && a) return { name: "task", ulid: a };
  if (head === "views" && seg.length === 3 && a && b) return { name: "view", store: a, view: b };
  if (head === "query" && seg.length === 2 && a && LAYOUTS.includes(a)) {
    return { name: "query", layout: a as Layout, q: new URLSearchParams(search).get("q") ?? "" };
  }
  if (head === "my" && seg.length === 1) return { name: "my" };
  if (head === "inbox" && seg.length === 1) {
    const tab = new URLSearchParams(search).get("tab") ?? "";
    return { name: "inbox", tab: INBOX_TABS.find((t) => t.tab === tab)?.tab ?? "all" };
  }
  if (head === "activity" && seg.length === 1) {
    const p = new URLSearchParams(search);
    const sync = p.get("sync");
    return {
      name: "activity",
      workspaces: (p.get("workspace") ?? "").split(",").filter(Boolean),
      sync: sync === "exclude" || sync === "only" || sync === "automation" ? sync : "",
    };
  }
  if (head === "settings" && (seg.length === 1 || (seg.length === 2 && a === "general"))) {
    return { name: "settings", page: "general" };
  }
  if (head === "settings" && seg.length === 2 && (a === "accounts" || a === "stores")) {
    return { name: "settings", page: a };
  }
  if (
    head === "settings" &&
    seg.length === 3 &&
    (a === "workspaces" || a === "automations" || a === "schema") &&
    b
  ) {
    if (a !== "schema") return { name: "settings", page: a, store: b };
    const tab = new URLSearchParams(search).get("tab") ?? "";
    return {
      name: "settings",
      page: a,
      store: b,
      tab: SCHEMA_TABS.find((t) => t === tab) ?? "statuses",
    };
  }
  return null;
}

/** Unknown paths are the home route. search is read for /query/<layout>, /activity, /inbox and /settings/schema/<store> only. */
export function parseRoute(pathname: string, search = ""): Route {
  return matchRoute(pathname, search) ?? { name: "home" };
}

/** The path of a route; the query screen's also holds ?q= (left out when q is empty), the activity's its filters, the inbox and the schema their tab (left out for the first). */
export function hrefOf(r: Route): string {
  switch (r.name) {
    case "home":
      return "/";
    case "workspace": {
      const base = `/workspaces/${encodeURIComponent(r.store)}/${encodeURIComponent(r.ulid)}`;
      return r.tab === "overview" ? base : `${base}/${r.tab}`;
    }
    case "project": {
      const base = `/projects/${encodeURIComponent(r.ulid)}`;
      return r.tab === "tree" ? base : `${base}/${r.tab}`;
    }
    case "task":
      return `/tasks/${encodeURIComponent(r.ulid)}`;
    case "view":
      return `/views/${encodeURIComponent(r.store)}/${encodeURIComponent(r.view)}`;
    case "query":
      // URLSearchParams writes the form the browser keeps (' as %27, a space as +), so a route
      // that is shown already compares equal to its own href.
      return r.q === ""
        ? `/query/${r.layout}`
        : `/query/${r.layout}?${new URLSearchParams({ q: r.q }).toString()}`;
    case "my":
      return "/my";
    case "inbox":
      return r.tab === "all" ? "/inbox" : `/inbox?tab=${r.tab}`;
    case "activity": {
      // The ULIDs of the workspaces are written with their commas, as the API takes them.
      const p = new URLSearchParams();
      if (r.workspaces.length > 0) p.set("workspace", r.workspaces.join(","));
      if (r.sync !== "") p.set("sync", r.sync);
      const q = p.toString().replaceAll("%2C", ",");
      return q === "" ? "/activity" : `/activity?${q}`;
    }
    case "settings":
      if (r.page === "general" || r.page === "accounts" || r.page === "stores") {
        return `/settings/${r.page}`;
      }
      if (r.page !== "schema") return `/settings/${r.page}/${encodeURIComponent(r.store)}`;
      return `/settings/schema/${encodeURIComponent(r.store)}${r.tab === "statuses" ? "" : `?tab=${r.tab}`}`;
  }
}

const listeners = new Set<() => void>();
const subscribe = createSubscriber((update) => {
  listeners.add(update);
  window.addEventListener("popstate", update);
  return () => {
    listeners.delete(update);
    window.removeEventListener("popstate", update);
  };
});

let last: { key: string; route: Route } | null = null;

export const router = {
  get route(): Route {
    subscribe();
    const key = location.pathname + location.search;
    if (last?.key !== key) last = { key, route: parseRoute(location.pathname, location.search) };
    return last.route;
  },

  /**
   * Moves to a path of ours; anything else goes to "/". The query and the hash are dropped,
   * except for the ?q= of /query/<layout>, the filters of /activity, and the tab of /inbox and of the schema.
   */
  go(path: string, opts: { replace?: boolean } = {}): void {
    const url = new URL(path, location.origin);
    const route = matchRoute(url.pathname, url.search);
    const target = route ? hrefOf(route) : "/";
    if (target !== location.pathname + location.search || location.hash !== "") {
      if (opts.replace) history.replaceState(null, "", target);
      else history.pushState(null, "", target);
    }
    for (const fn of listeners) fn();
  },

  /** go without adding to the history: the address changes and the back button skips it. */
  replace(path: string): void {
    router.go(path, { replace: true });
  },

  href(r: Route): string {
    return hrefOf(r);
  },
};

/** A link click that stays in the app; with ⌘/Ctrl/⇧ the page opens in a new tab that keeps the token. */
export function onLinkClick(e: MouseEvent, path: string): void {
  if (e.defaultPrevented || e.button !== 0 || e.altKey) return;
  e.preventDefault();
  if (e.metaKey || e.ctrlKey || e.shiftKey) {
    // The token is in sessionStorage, which is per tab. A tab the browser opens from a link
    // starts without it ("open it from hh ui" screen); window.open without noopener copies it.
    window.open(new URL(path, location.origin).href, "_blank");
    return;
  }
  router.go(path);
}
