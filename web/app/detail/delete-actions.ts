import { type Api, type TaskView, unwrap } from "../../core/api/client";
import type { TaskCache } from "../../core/cache";

export type Deps = { api: Api; cache: Pick<TaskCache, "applyWrite"> };
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
