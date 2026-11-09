import type { Api, ShowView } from "./api/client";
import { RELATIONS, type RelationField } from "./relations";

// showAt is the task as it was at the commit (GET /api/tasks/{id}/history/
// {commit}). The commit has no task: 404 restore_at.not_found.
export function showAt(api: Api, id: string, commit: string, store?: string): Promise<ShowView> {
  return api.showTaskAt(id, commit, { store });
}

export type RestoreChange = {
  field: string; // title, kind, status, parent, assignees, labels, a relation (blocked_by, causes, ...), body, props.<name>
  label: string;
  from: string; // now
  to: string; // after the restore
};

const NONE = "（なし）";

const shown = (v: unknown): string => {
  if (v === null || v === undefined || v === "") return NONE;
  if (Array.isArray(v)) return v.length === 0 ? NONE : v.map(shown).join("、");
  if (typeof v === "object") return JSON.stringify(v);
  return String(v);
};

const sorted = (xs: readonly string[] | null | undefined): string[] => [...(xs ?? [])].sort();
const sameSet = (
  a: readonly string[] | null | undefined,
  b: readonly string[] | null | undefined,
) => JSON.stringify(sorted(a)) === JSON.stringify(sorted(b));

// diffForRestore lists what a restore to `then` changes in `now`, the fields
// that differ and no others: the title, kind, status, parent, assignees,
// labels, blockers, props, and the body (shown only as changed). Nothing in
// the list means the task is the same as at that time. idOf shows the ULID of
// a parent as its ID; propLabel is the label of a property.
export function diffForRestore(
  now: ShowView,
  then: ShowView,
  opts: { idOf?: (ulid: string) => string; propLabel?: (name: string) => string } = {},
): RestoreChange[] {
  const idOf = opts.idOf ?? ((u: string) => u);
  const propLabel = opts.propLabel ?? ((n: string) => n);
  const out: RestoreChange[] = [];
  const scalar = (field: string, label: string, a: string, b: string) => {
    if (a !== b) out.push({ field, label, from: shown(a), to: shown(b) });
  };
  scalar("title", "タイトル", now.title, then.title);
  scalar("kind", "種類", now.kind, then.kind);
  scalar("status", "ステータス", now.status, then.status);
  if ((now.parent ?? null) !== (then.parent ?? null)) {
    out.push({
      field: "parent",
      label: "親",
      from: now.parent ? idOf(now.parent) : "ルート",
      to: then.parent ? idOf(then.parent) : "ルート",
    });
  }
  const list = (field: string, label: string, a: string[] | null, b: string[] | null) => {
    if (!sameSet(a, b)) out.push({ field, label, from: shown(a), to: shown(b) });
  };
  list("assignees", "担当者", now.assignees, then.assignees);
  list("labels", "ラベル", now.labels, then.labels);
  for (const r of RELATIONS) {
    list(
      r.name,
      r.label,
      (now[r.name as RelationField] ?? []).map((x) => x.id),
      (then[r.name as RelationField] ?? []).map((x) => x.id),
    );
  }
  const keys = [...new Set([...Object.keys(now.props ?? {}), ...Object.keys(then.props ?? {})])];
  for (const k of keys.sort()) {
    const a = now.props?.[k] ?? null;
    const b = then.props?.[k] ?? null;
    if (JSON.stringify(a) !== JSON.stringify(b)) {
      out.push({ field: `props.${k}`, label: propLabel(k), from: shown(a), to: shown(b) });
    }
  }
  if ((now.body ?? "") !== (then.body ?? "")) {
    out.push({ field: "body", label: "本文", from: "現在の本文", to: "この時点の本文" });
  }
  return out;
}
