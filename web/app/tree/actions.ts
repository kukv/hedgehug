import { type TaskView, unwrap } from "../../core/api/client";
import type { AppCore } from "../context";
import type { MoveArgs } from "./placement";

type Deps = Pick<AppCore, "api" | "cache">;

export async function moveTask(d: Deps, task: TaskView, to: MoveArgs): Promise<TaskView> {
  const r = await unwrap(
    d.api.POST("/api/tasks/{id}/move", {
      params: { path: { id: task.ulid }, query: { store: task.store } },
      body: to,
    }),
  );
  d.cache.applyWrite({ task: r.task, commit: r.commit });
  return r.task;
}

export async function patchTask(
  d: Deps,
  task: TaskView,
  body: { status?: string; kind?: string },
): Promise<TaskView> {
  const r = await unwrap(
    d.api.PATCH("/api/tasks/{id}", {
      params: { path: { id: task.ulid }, query: { store: task.store } },
      body,
    }),
  );
  d.cache.applyWrite({ task: r.task, commit: r.commit });
  return r.task;
}

export type CreateBelow = {
  title: string;
  kind: string;
  store: string;
  parent: string | null; // ULID; null for the top of a workspace
  workspace: string; // the prefix, used when there is no parent
  after: string | null; // the row it goes below; null for the end
};

// createBelow adds a task and, unless it goes to the end of the list, moves it
// right after the row above. That is two commits. The row above may be the last
// one shown while the filter hides later siblings, so the move is not skipped for it.
export async function createBelow(d: Deps, i: CreateBelow): Promise<TaskView> {
  const r = await unwrap(
    d.api.POST("/api/tasks", {
      params: { query: { store: i.store } },
      body: {
        title: i.title,
        kind: i.kind,
        ...(i.parent ? { parent: i.parent } : { workspace: i.workspace }),
      },
    }),
  );
  d.cache.applyWrite({ task: r.task, commit: r.commit });
  if (i.after === null) return r.task;
  return moveTask(d, r.task, { after: i.after });
}
