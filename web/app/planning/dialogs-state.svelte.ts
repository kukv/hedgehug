import type { Release, Sprint } from "../../core/planning";
import type { WorkspaceEntry } from "../../core/stores";

/** What a bulk update is on: the rows picked, or what a query finds. */
export type BulkTarget = { ulids?: string[]; query?: string; store?: string };

/** The dialog of a sprint or a release that is asked for; PlanDialogs draws it. */
export type PlanDialog =
  // Without an entry (from the palette, off a workspace) the dialog asks for the workspace.
  | { kind: "sprint-add"; entry?: WorkspaceEntry }
  | { kind: "sprint-edit"; entry: WorkspaceEntry; sprint: Sprint }
  | { kind: "sprint-start"; entry: WorkspaceEntry; sprint: Sprint; sprints: Sprint[] }
  | { kind: "sprint-close"; entry: WorkspaceEntry; sprint: Sprint; sprints: Sprint[] }
  | { kind: "sprint-rm"; entry: WorkspaceEntry; sprint: Sprint }
  | { kind: "release-add"; entry?: WorkspaceEntry }
  | { kind: "release-edit"; entry: WorkspaceEntry; release: Release }
  | { kind: "release-rm"; entry: WorkspaceEntry; release: Release }
  | { kind: "bulk"; target: BulkTarget };

let current = $state.raw<PlanDialog | null>(null);

export const planDialog = {
  get current(): PlanDialog | null {
    return current;
  },
};

export function openPlanDialog(d: PlanDialog): void {
  current = d;
}

export function closePlanDialog(): void {
  current = null;
}
