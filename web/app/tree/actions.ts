import type { Schemas, TaskView } from "../../core/api/client";
import type { AppCore } from "../context";
import type { MoveArgs } from "./placement";

type Deps = Pick<AppCore, "api" | "cache">;

// moveTask says the commit it made; null when the task was already there.
export async function moveTask(
  d: Deps,
  task: TaskView,
  to: MoveArgs,
): Promise<{ task: TaskView; commit: string | null }> {
  const r = await d.api.moveTask(task.ulid, to, { store: task.store });
  d.cache.applyWrite({ task: r.task, commit: r.commit });
  return { task: r.task, commit: r.commit ?? null };
}

export async function patchTask(
  d: Deps,
  task: TaskView,
  body: Schemas.PatchInputBody,
): Promise<TaskView> {
  const r = await d.api.updateTask(task.ulid, body, { store: task.store });
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

// createBelow adds a task right after the row above it (or at the end when
// there is none) in one commit.
export async function createBelow(d: Deps, i: CreateBelow): Promise<TaskView> {
  const r = await d.api.createTask(
    {
      title: i.title,
      kind: i.kind,
      ...(i.parent ? { parent: i.parent } : { workspace: i.workspace }),
      ...(i.after ? { after: i.after } : {}),
    },
    { store: i.store },
  );
  d.cache.applyWrite({ task: r.task, commit: r.commit });
  return r.task;
}
