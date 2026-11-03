import { createSubscriber } from "svelte/reactivity";
import type { Layout } from "../core/views";

export type { Layout };
// The tabs the screens have so far; 4e-4 adds the settings of a project.
export type WorkspaceTab = "overview" | "tree" | "kanban" | "table" | "settings";
export type ProjectTab = "tree" | "kanban" | "table";
// store is "" for /settings, which names no Store: the screen goes to the first one.
export type SettingsPage = { page: "workspaces"; store: string };

export type Route =
  | { name: "home" }
  | { name: "workspace"; store: string; ulid: string; tab: WorkspaceTab }
  | { name: "project"; ulid: string; tab: ProjectTab }
  | { name: "task"; ulid: string }
  | { name: "view"; store: string; view: string }
  | { name: "query"; layout: Layout; q: string }
  | { name: "my" }
  | { name: "inbox" }
  | ({ name: "settings" } & SettingsPage);

const WORKSPACE_TABS: readonly string[] = ["tree", "kanban", "table", "settings"];
const PROJECT_TABS: readonly string[] = ["tree", "kanban", "table"];
const LAYOUTS: readonly string[] = ["tree", "kanban", "table"];

function decode(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}

/** The route of a path, or null when the path is none of ours. Only /query/<layout> reads the search (?q=). */
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
  if (head === "inbox" && seg.length === 1) return { name: "inbox" };
  if (head === "settings" && seg.length === 1) {
    return { name: "settings", page: "workspaces", store: "" };
  }
  if (head === "settings" && seg.length === 3 && a === "workspaces" && b) {
    return { name: "settings", page: "workspaces", store: b };
  }
  return null;
}

/** Unknown paths are the home route. search is read for /query/<layout> only. */
export function parseRoute(pathname: string, search = ""): Route {
  return matchRoute(pathname, search) ?? { name: "home" };
}

/** The path of a route; the query screen's also holds ?q= (left out when q is empty). */
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
      return "/inbox";
    case "settings":
      return r.store === "" ? "/settings" : `/settings/workspaces/${encodeURIComponent(r.store)}`;
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
   * except for the ?q= of /query/<layout>.
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
