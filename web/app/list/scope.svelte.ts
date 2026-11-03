import type { ApiError, TaskView } from "../../core/api/client";
import { quoteArg } from "../../core/cli";
import type { SchemaView } from "../../core/schema";
import type { WorkspaceEntry } from "../../core/stores";
import { core } from "../context";
import { useSubscribe } from "../svelte-store";
import { baseQuery } from "../tree/filter";
import type { ListScope } from "./scope";

// useListScope resolves a ListScope against the core: the workspace or project it names, the store
// it is in, and the fixed part of the query that lists it. Call it while a component is created.
// The tree, the kanban and the table of one workspace or project read the same things from it.
export function useListScope(get: () => ListScope) {
  const c = core();
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));

  type DetailHandle = ReturnType<typeof c.cache.watchDetail>;
  let detail = $state<DetailHandle | null>(null);
  $effect(() => {
    const s = get();
    if (s.kind !== "project") return;
    const h = c.cache.watchDetail(s.ulid);
    detail = h;
    return () => {
      h.dispose();
      detail = null;
    };
  });

  const project = $derived.by<TaskView | undefined>(() => {
    cacheRev.current;
    const s = get();
    return s.kind === "project" ? c.cache.task(s.ulid) : undefined;
  });
  const store = $derived.by<string | undefined>(() => {
    const s = get();
    if (s.kind === "workspace") return s.store;
    return s.kind === "project" ? project?.store : undefined;
  });
  const workspace = $derived.by<WorkspaceEntry | undefined>(() => {
    storesRev.current;
    const s = get();
    if (s.kind === "workspace") return c.stores.workspace(s.store, s.ulid);
    if (s.kind !== "project" || !project) return undefined;
    return c.stores
      .workspaces()
      .find((w) => w.store === project.store && w.ulid === project.workspace.ulid);
  });
  // The workspace the address names is not in the stores (removed, or never there).
  const workspaceMissing = $derived.by(() => {
    storesRev.current;
    return get().kind === "workspace" && !workspace && c.stores.loaded();
  });
  // Why the project cannot be shown (deleted, not found, not fetched).
  const error = $derived.by<ApiError | null>(() => {
    cacheRev.current;
    return project ? null : (detail?.error() ?? null);
  });
  const base = $derived.by(() => {
    const s = get();
    if (s.kind === "workspace")
      return workspace ? { kind: "workspace" as const, name: workspace.name } : null;
    if (s.kind === "project") {
      return project ? { kind: "project" as const, ulid: project.ulid, id: project.id } : null;
    }
    return null;
  });
  const ready = $derived(get().kind === "all" || base !== null);
  const manyStores = $derived.by(() => {
    storesRev.current;
    return c.stores.stores().length >= 2;
  });

  return {
    /** The workspace or project is known (always true for every store). */
    get ready(): boolean {
      return ready;
    },
    /** The workspace or project is not there: the address names none, or the server says 404. */
    get notFound(): boolean {
      return workspaceMissing || error?.status === 404;
    },
    /** Why the project could not be read, when it is not a 404; null otherwise. */
    get error(): ApiError | null {
      return error;
    },
    /** The fixed part of the query for the API (the project by ULID); "" for every store or while not ready. */
    get apiBase(): string {
      return base ? baseQuery(base, "api") : "";
    },
    /** The fixed part as the user would type it in hh (the project by ID); "" for every store or while not ready. */
    get cliBase(): string {
      return base ? baseQuery(base, "cli") : "";
    },
    /** The store of the workspace or project; undefined for every store. */
    get store(): string | undefined {
      return store;
    },
    get workspace(): WorkspaceEntry | undefined {
      return workspace;
    },
    get project(): TaskView | undefined {
      return project;
    },
    /**
     * The fixed part of the query of a view saved from this screen: the cliBase, with the store in
     * front when two or more stores are mounted (a view is listed on every store). "" for every
     * store or while not ready.
     */
    get saveBase(): string {
      if (!base || !store) return "";
      const own = baseQuery(base, "cli");
      return manyStores ? `store:${quoteArg(store)} ${own}` : own;
    },
    /**
     * The schemas of the stores that are loaded, in the order asked. One that is not is fetched, and
     * the caller reads again when the schema cache notifies.
     */
    schemas(stores: string[]): SchemaView[] {
      schemaRev.current;
      const out: SchemaView[] = [];
      for (const name of stores) {
        const s = c.schema.get(name);
        if (s) out.push(s);
        else void c.schema.load(name).catch(() => undefined);
      }
      return out;
    },
  };
}
