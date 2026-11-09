import { ApiError, type Schemas } from "../../core/api/client";
import { errorMessage, violationLines } from "../../core/messages";
import { relationLabel } from "../../core/relations";

/** The kind of a reference the move cuts, as the screen says it (細部 47・67). */
export function referenceKind(field: string): string {
  switch (field) {
    case "sprint":
      return "スプリント";
    case "releases":
      return "リリース";
    default:
      return relationLabel(field);
  }
}

export type UnlinkRow = { key: string; kind: string; target: string; id: string };

/** The references the move cuts, in the order of the plan: the kind, what it points to, the task it is on. */
export function unlinkedRows(unlinked: Schemas.MoveUnlinkView[]): UnlinkRow[] {
  return unlinked.map((u) => ({
    key: `${u.task.ulid} ${u.field} ${u.target.ulid}`,
    kind: referenceKind(u.field),
    target: u.target.label,
    id: u.task.id,
  }));
}

/** The label of the button that moves: it counts the references it cuts. */
export function moveButton(cut: number): string {
  return cut > 0 ? `${cut} 件の参照を外して移動` : "移動";
}

/**
 * Where the task goes in the destination. from is the parent it leaves ("<ID> <title>"; null when it
 * has none), parent the one it goes under, workspace the name of the workspace whose root it goes to.
 */
export function parentSentence(i: {
  from: string | null;
  parent: string | null;
  workspace: string;
}): string {
  const to = i.parent !== null ? `${i.parent} の下に置きます` : `${i.workspace} のルートに置きます`;
  return i.from !== null ? `${i.from} から外し、${to}` : to;
}

/** The two sentences of the notice when accounts lose sight of the task (細部 47). */
export function hiddenSentences(hidden: string[], from: string): string[] {
  return [
    `見えなくなる人: ${hidden.join("、")}`,
    `移動前の内容は ${from} の git 履歴に残り、${from} を読める人は過去の内容を見られます`,
  ];
}

/**
 * An error that carries the violations of the destination (move.invalid, schema.violation): the
 * sentence errorMessage gives without the English detail and the violations, and each violation as a
 * line. null for any other error (細部 49).
 */
export function violationView(e: unknown): { headline: string; lines: string[] } | null {
  if (!(e instanceof ApiError)) return null;
  if (e.type !== "move.invalid" && e.type !== "schema.violation") return null;
  const lines = violationLines(e.args).filter((l) => l !== "");
  if (lines.length === 0) return null;
  const { violations: _, ...args } = e.args;
  const headline = errorMessage(new ApiError({ status: e.status, type: e.type, args }));
  return { headline, lines };
}
