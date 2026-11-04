import type { TaskView } from "../../core/api/client";
import type { WorkspaceEntry } from "../../core/stores";
import { readOnlySentence } from "../read-only";
import type { Route } from "../router";

export type CreateContext = { parent: TaskView | null; workspace: WorkspaceEntry | null };

export const workspaceKey = (w: Pick<WorkspaceEntry, "store" | "ulid">) => `${w.store}/${w.ulid}`;

export type CreateGuard = {
  /** Why nothing can be created now ("" when it can): no Store to write to, or the workspace chosen is read-only. */
  blocked: string;
  /** Whether the workspace is of a Store that cannot be written (shown, but not to be chosen). */
  isOff: (w: WorkspaceEntry) => boolean;
  /** The words after a workspace that is off: "<Store>（閲覧のみ）: <reason>", else "". */
  note: (w: WorkspaceEntry) => string;
};

// createGuard is what the create dialogs share (細部 35): reasonOf gives why a Store cannot be written
// (in Japanese), or null; ws is the workspace the task or project would go to.
export function createGuard(
  entries: WorkspaceEntry[],
  ws: WorkspaceEntry | undefined,
  reasonOf: (store: string) => string | null,
): CreateGuard {
  const noWritable = entries.length > 0 && entries.every((w) => reasonOf(w.store) !== null);
  const reason = ws ? reasonOf(ws.store) : null;
  return {
    blocked: noWritable
      ? "書き込める Store がありません"
      : reason !== null
        ? readOnlySentence(reason)
        : "",
    isOff: (w) => reasonOf(w.store) !== null,
    note: (w) => {
      const r = reasonOf(w.store);
      return r === null ? "" : `${w.store}（閲覧のみ）: ${r}`;
    },
  };
}

// resolveCreateContext decides where a new task goes from the screen the user is on (細部 29): the parent a request
// names, the task of a task detail (when it cannot be read, no parent: a row selected in a tree left earlier is not meant), the selected row of the tree, the project of a project screen; on a workspace
// screen the root of that workspace; elsewhere the root of the first workspace. A deleted or unknown task is skipped.
export function resolveCreateContext(i: {
  requestParent?: string;
  selection: string | null;
  route: Route;
  task: (ulid: string) => TaskView | undefined;
  workspaces: WorkspaceEntry[];
  /** Why a Store cannot be written, or null; a workspace of such a Store is passed over as the start. */
  readOnly?: (store: string) => string | null;
  /** Whether a Store is hidden; the default start is the first workspace of a Store that is not (the page a workspace names is its own start). */
  hidden?: (store: string) => boolean;
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
    (r.name === "task" ? live(r.ulid) : live(i.selection)) ??
    (r.name === "project" ? live(r.ulid) : undefined);
  if (parent) return { parent, workspace: workspaceOf(parent) };

  // A read-only Store is passed over for the first writable one (when nothing can be written the
  // start stays, and the dialog says so).
  const shown = i.hidden ? i.workspaces.filter((w) => !i.hidden?.(w.store)) : i.workspaces;
  const start =
    r.name === "workspace"
      ? (i.workspaces.find((x) => x.store === r.store && x.ulid === r.ulid) ?? shown[0])
      : shown[0];
  const writable = (w: WorkspaceEntry) => !i.readOnly?.(w.store);
  const workspace = start && !writable(start) ? (shown.find(writable) ?? start) : start;
  return { parent: null, workspace: workspace ?? null };
}
