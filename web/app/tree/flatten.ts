import type { TaskView } from "../../core/api/client";
import type { TreeNode } from "../../core/cache";

// A list longer than this is drawn with TanStack Virtual; a shorter one is
// drawn whole.
export const VIRTUALIZE_AT = 200;
// A row is 38px with a 1px gap.
export const ROW_HEIGHT = 39;

export type FlatRow = {
  ulid: string;
  task: TaskView;
  depth: number;
  // The parent in the tree: the view root for the top rows of a project, null
  // at the top of a workspace.
  parent: string | null;
  hasChildren: boolean;
  childCount: number;
  expanded: boolean;
};

// viewRoots is what the view lists at its top: the roots of the tree for a
// workspace, the children of the project (which is itself left out) for a
// project.
export function viewRoots(roots: TreeNode[], viewRoot: string | null): TreeNode[] {
  if (viewRoot === null) return roots;
  const find = (nodes: TreeNode[]): TreeNode | undefined => {
    for (const n of nodes) {
      if (n.ulid === viewRoot) return n;
      const f = find(n.children);
      if (f) return f;
    }
    return undefined;
  };
  return find(roots)?.children ?? [];
}

export function flatten(
  roots: TreeNode[],
  viewRoot: string | null,
  taskOf: (ulid: string) => TaskView | undefined,
  collapsed: ReadonlySet<string>,
): FlatRow[] {
  const out: FlatRow[] = [];
  const walk = (nodes: TreeNode[], depth: number, parent: string | null) => {
    for (const n of nodes) {
      const task = taskOf(n.ulid);
      if (!task) continue;
      const expanded = n.children.length > 0 && !collapsed.has(n.ulid);
      out.push({
        ulid: n.ulid,
        task,
        depth,
        parent,
        hasChildren: n.children.length > 0,
        childCount: n.children.length,
        expanded,
      });
      if (expanded) walk(n.children, depth + 1, n.ulid);
    }
  };
  walk(viewRoots(roots, viewRoot), 0, viewRoot);
  return out;
}

// countMatched counts the tasks the query found: every task of the view,
// collapsed ones too, except the ancestors the tree adds for context (the
// mark is on the node: the row is shared with the other queries).
export function countMatched(
  roots: TreeNode[],
  viewRoot: string | null,
  taskOf: (ulid: string) => TaskView | undefined,
): number {
  let n = 0;
  const walk = (nodes: TreeNode[]) => {
    for (const node of nodes) {
      if (!node.context && taskOf(node.ulid)) n++;
      walk(node.children);
    }
  };
  walk(viewRoots(roots, viewRoot));
  return n;
}

export function countLabel(matched: number, canceledLeftOut: number | null): string {
  return canceledLeftOut === null
    ? `${matched} 件`
    : `${matched} 件 · 中止 ${canceledLeftOut} 件を除外`;
}
