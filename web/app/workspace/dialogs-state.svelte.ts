import type { TaskView } from "../../core/api/client";
import type { WorkspaceEntry } from "../../core/stores";
import { removeBlocker } from "../../core/workspace";
import { router } from "../router";

/** What a move needs of a task: a TaskView has all of it. */
export type MoveTask = Pick<TaskView, "ulid" | "id" | "title" | "store" | "workspace">;

/** The workspace dialog that is asked for; WorkspaceDialogs draws it. */
export type WorkspaceDialog =
  | { kind: "add"; store?: string }
  | { kind: "delete"; entry: WorkspaceEntry }
  | { kind: "prefix"; entry: WorkspaceEntry }
  | { kind: "move"; task: MoveTask };

// raw: the entries and tasks are the ones the directory and the cache hold, not copies of them.
let current = $state.raw<WorkspaceDialog | null>(null);

export const workspaceDialog = {
  get current(): WorkspaceDialog | null {
    return current;
  },
};

/** store is the Store the dialog starts with; without it, the default Store. */
export function openWorkspaceAdd(store?: string): void {
  current = { kind: "add", store };
}

/**
 * The confirmation opens only for a workspace with no task at all (Dialogs). With tasks left it says
 * why on the settings tab of the workspace, so that is where it goes.
 */
export function openWorkspaceDelete(entry: WorkspaceEntry): void {
  if (removeBlocker(entry) !== null) {
    router.go(
      router.href({ name: "workspace", store: entry.store, ulid: entry.ulid, tab: "settings" }),
    );
    return;
  }
  current = { kind: "delete", entry };
}

export function openPrefixChange(entry: WorkspaceEntry): void {
  current = { kind: "prefix", entry };
}

export function openWorkspaceMove(task: MoveTask): void {
  current = { kind: "move", task };
}

export function closeWorkspaceDialog(): void {
  current = null;
}
