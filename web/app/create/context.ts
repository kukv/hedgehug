import type { TaskView } from "../../core/api/client";
import type { WorkspaceEntry } from "../../core/stores";
import type { Route } from "../router";

export type CreateContext = { parent: TaskView | null; workspace: WorkspaceEntry | null };

export const workspaceKey = (w: Pick<WorkspaceEntry, "store" | "ulid">) => `${w.store}/${w.ulid}`;

// resolveCreateContext decides where a new task goes from the screen the user is on (細部 29): the parent a request
// names, the task of a task detail, the selected row of the tree, the project of a project screen; on a workspace
// screen the root of that workspace; elsewhere the root of the first workspace. A deleted or unknown task is skipped.
export function resolveCreateContext(i: {
  requestParent?: string;
  selection: string | null;
  route: Route;
  task: (ulid: string) => TaskView | undefined;
  workspaces: WorkspaceEntry[];
}): CreateContext {
  const live = (u: string | null | undefined): TaskView | undefined => {
    if (!u) return undefined;
    const t = i.task(u);
    return t && t.deleted_at == null ? t : undefined;
  };
  const workspaceOf = (t: TaskView) =>
    i.workspaces.find((w) => w.store === t.store && w.ulid === t.workspace.ulid) ?? null;

  const r = i.route;
  const parent =
    live(i.requestParent) ??
    (r.name === "task" ? live(r.ulid) : undefined) ??
    live(i.selection) ??
    (r.name === "project" ? live(r.ulid) : undefined);
  if (parent) return { parent, workspace: workspaceOf(parent) };

  if (r.name === "workspace") {
    const w = i.workspaces.find((x) => x.store === r.store && x.ulid === r.ulid);
    return { parent: null, workspace: w ?? i.workspaces[0] ?? null };
  }
  return { parent: null, workspace: i.workspaces[0] ?? null };
}
