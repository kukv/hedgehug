import { createSubscriber } from "svelte/reactivity";

// The tabs the screens have so far; 4e-3 adds kanban and table, 4e-4 the settings of a project.
export type WorkspaceTab = "overview" | "tree" | "settings";
export type ProjectTab = "tree";
// store is "" for /settings, which names no Store: the screen goes to the first one.
export type SettingsPage = { page: "workspaces"; store: string };

export type Route =
  | { name: "home" }
  | { name: "workspace"; store: string; ulid: string; tab: WorkspaceTab }
  | { name: "project"; ulid: string; tab: ProjectTab }
  | { name: "task"; ulid: string }
  | ({ name: "settings" } & SettingsPage);

function decode(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}

/** The route of a path, or null when the path is none of ours. */
function matchRoute(pathname: string): Route | null {
  const seg = pathname.split("/").filter(Boolean).map(decode);
  const [head, a, b, c] = seg;
  if (head === undefined) return { name: "home" };
  if (head === "workspaces" && a && b) {
    if (seg.length === 3) return { name: "workspace", store: a, ulid: b, tab: "overview" };
    if (seg.length === 4 && (c === "tree" || c === "settings")) {
      return { name: "workspace", store: a, ulid: b, tab: c };
    }
    return null;
  }
  if (head === "projects" && a && (seg.length === 2 || (seg.length === 3 && b === "tree"))) {
    return { name: "project", ulid: a, tab: "tree" };
  }
  if (head === "tasks" && seg.length === 2 && a) return { name: "task", ulid: a };
  if (head === "settings" && seg.length === 1) {
    return { name: "settings", page: "workspaces", store: "" };
  }
  if (head === "settings" && seg.length === 3 && a === "workspaces" && b) {
    return { name: "settings", page: "workspaces", store: b };
  }
  return null;
}

/** Unknown paths are the home route. */
export function parseRoute(pathname: string): Route {
  return matchRoute(pathname) ?? { name: "home" };
}

export function hrefOf(r: Route): string {
  switch (r.name) {
    case "home":
      return "/";
    case "workspace": {
      const base = `/workspaces/${encodeURIComponent(r.store)}/${encodeURIComponent(r.ulid)}`;
      return r.tab === "overview" ? base : `${base}/${r.tab}`;
    }
    case "project":
      return `/projects/${encodeURIComponent(r.ulid)}`;
    case "task":
      return `/tasks/${encodeURIComponent(r.ulid)}`;
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

let last: { path: string; route: Route } | null = null;

export const router = {
  get route(): Route {
    subscribe();
    const path = location.pathname;
    if (last?.path !== path) last = { path, route: parseRoute(path) };
    return last.route;
  },

  /** Moves to a path of ours; anything else goes to "/". Query and hash are dropped. */
  go(path: string, opts: { replace?: boolean } = {}): void {
    const url = new URL(path, location.origin);
    const route = matchRoute(url.pathname);
    const target = route ? hrefOf(route) : "/";
    if (target !== location.pathname || location.search !== "" || location.hash !== "") {
      if (opts.replace) history.replaceState(null, "", target);
      else history.pushState(null, "", target);
    }
    for (const fn of listeners) fn();
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
