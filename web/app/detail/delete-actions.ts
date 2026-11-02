import { type Api, ApiError, type TaskView, unwrap } from "../../core/api/client";
import type { TaskCache } from "../../core/cache";

export type Deps = { api: Api; cache: Pick<TaskCache, "applyWrite"> };
export type Target = { ulid: string; id: string; title: string };
type Ref = { ulid: string; store: string };

const at = (t: Ref) => ({ params: { path: { id: t.ulid }, query: { store: t.store } } });

// collectDescendants lists the live tasks under ulid, level by level, with parent:<ulid> queries.
export async function collectDescendants(
  api: Api,
  store: string,
  ulid: string,
  limit = 500,
): Promise<TaskView[]> {
  const out: TaskView[] = [];
  const queue = [ulid];
  while (queue.length > 0 && out.length < limit) {
    const id = queue.shift() as string;
    const r = await unwrap(
      api.GET("/api/tasks", { params: { query: { store, q: `parent:${id}` } } }),
    );
    for (const t of r.tasks ?? []) {
      out.push(t);
      queue.push(t.ulid);
    }
  }
  return out;
}

export async function deleteTask(d: Deps, t: Ref) {
  const r = await unwrap(d.api.DELETE("/api/tasks/{id}", at(t)));
  d.cache.applyWrite({ tasks: r.tasks ?? [], commit: r.commit });
  return r;
}

export async function restoreTask(d: Deps, t: Ref) {
  const r = await unwrap(d.api.POST("/api/tasks/{id}/restore", at(t)));
  d.cache.applyWrite({ tasks: r.tasks ?? [], commit: r.commit });
  return r;
}

// purgeTargetsOf reads the targets a 409 purge.unconfirmed lists; any other error gives null.
export function purgeTargetsOf(e: unknown): Target[] | null {
  if (!(e instanceof ApiError) || e.type !== "purge.unconfirmed") return null;
  const list = (e.args as { targets?: unknown }).targets;
  if (!Array.isArray(list)) return null;
  return list
    .filter(
      (x): x is Target =>
        typeof x === "object" && x !== null && "ulid" in x && "id" in x && "title" in x,
    )
    .map((x) => ({ ulid: String(x.ulid), id: String(x.id), title: String(x.title) }));
}

// probePurge asks to purge without a confirmation, which the server always refuses with the tasks it
// would remove. Anything else is an error of the caller to show.
export async function probePurge(d: Deps, t: Ref): Promise<Target[]> {
  try {
    await unwrap(
      d.api.DELETE("/api/tasks/{id}", {
        params: { path: { id: t.ulid }, query: { store: t.store, purge: true } },
      }),
    );
  } catch (e) {
    const targets = purgeTargetsOf(e);
    if (targets) return targets;
    throw e;
  }
  throw new Error("the server purged without a confirmation");
}

export async function purgeTask(d: Deps, t: Ref, confirm: string[]) {
  const r = await unwrap(
    d.api.DELETE("/api/tasks/{id}", {
      params: {
        path: { id: t.ulid },
        query: { store: t.store, purge: true, confirm: confirm.join(",") },
      },
    }),
  );
  d.cache.applyWrite({ ulids: r.ulids ?? [], commit: r.commit });
  return r;
}
