import type { TaskView } from "../../core/api/client";
import type { WorkspaceEntry } from "../../core/stores";
import { readOnlySentence } from "../read-only";
import type { MenuItem } from "../ui/Menu.svelte";

/** The value of the menu item that moves a task to another workspace. */
export const MOVE_VALUE = "move-workspace";

/**
 * The menu item "別のワークスペースへ移動…" of the task detail and of a row of the tree. A task goes to
 * another workspace of its own Store only, so the item is off when the Store has no other one, or
 * when the Store is read-only (readOnly is the reason, in Japanese).
 */
export function moveWorkspaceItem(
  task: Pick<TaskView, "store" | "workspace">,
  entries: WorkspaceEntry[],
  readOnly: string | null = null,
): MenuItem {
  const others = entries.filter((w) => w.store === task.store && w.ulid !== task.workspace.ulid);
  return {
    value: MOVE_VALUE,
    label: "別のワークスペースへ移動…",
    disabled: readOnly !== null || others.length === 0,
    hint:
      readOnly !== null
        ? readOnlySentence(readOnly)
        : others.length === 0
          ? "この Store にはほかのワークスペースがありません"
          : undefined,
  };
}
