import { type Api, type TaskView, unwrap } from "../../core/api/client";
import type { TaskCache } from "../../core/cache";

export type Crumb = {
  ulid: string;
  id: string;
  title: string;
  kind: string;
  status: string;
  parent: string | null;
  store: string;
};
export type Lookup = (idOrUlid: string) => Promise<Crumb | undefined>;

function crumbOf(
  t: Pick<TaskView, "ulid" | "id" | "title" | "kind" | "status" | "parent" | "store">,
): Crumb {
  return {
    ulid: t.ulid,
    id: t.id,
    title: t.title,
    kind: t.kind,
    status: t.status,
    parent: t.parent,
    store: t.store,
  };
}

// createLookup finds a task by ID or ULID: the cache first, then GET /tasks/{id}. Requests for the same key that
// are in flight at once share one request; nothing is kept afterwards, so a title that changed is read again.
export function createLookup(opts: {
  api: Api;
  cache: Pick<TaskCache, "task">;
  store: string;
}): Lookup {
  const inflight = new Map<string, Promise<Crumb | undefined>>();
  return (key) => {
    const hit = opts.cache.task(key);
    if (hit) return Promise.resolve(crumbOf(hit));
    let p = inflight.get(key);
    if (!p) {
      p = unwrap(
        opts.api.GET("/api/tasks/{id}", {
          params: { path: { id: key }, query: { store: opts.store } },
        }),
      )
        .then(crumbOf, () => undefined)
        .finally(() => inflight.delete(key));
      inflight.set(key, p);
    }
    return p;
  };
}

// ancestorChain walks up from start (a parent ULID) and returns the ancestors, the root first. It stops at a
// task it cannot read and after limit steps (a parent cycle cannot loop forever).
export async function ancestorChain(
  start: string | null,
  lookup: Lookup,
  limit = 16,
): Promise<Crumb[]> {
  const chain: Crumb[] = [];
  let cur = start;
  while (cur && chain.length < limit) {
    const c = await lookup(cur);
    if (!c) break;
    chain.unshift(c);
    cur = c.parent;
  }
  return chain;
}

// collapseCrumbs shortens a long chain for the breadcrumb: the first, an ellipsis (null) and the last two.
export function collapseCrumbs<T>(chain: T[]): (T | null)[] {
  if (chain.length <= 3) return chain;
  return [chain[0] as T, null, ...chain.slice(-2)];
}
