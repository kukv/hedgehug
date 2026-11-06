import type { Schemas } from "./api/client";
import { formatDate } from "./format";
import type { WorkspaceEntry } from "./stores";

// The same rules as ValidName and ValidIDPrefix in internal/app/domain/model.
const NAME = /^[a-z0-9][a-z0-9_-]*$/;
const PREFIX = /^[A-Z][A-Z0-9]{1,9}$/;

export const NAME_RULE = "名前は小文字の英数字と - _ で、英数字から始めてください";
export const PREFIX_RULE = "接頭辞は英大文字で始まる 2〜10 文字の英大文字と数字です";

type Known = Pick<WorkspaceEntry, "store" | "ulid" | "name" | "prefix">;

// nameProblem says why name cannot name a workspace of store, or null. self is the workspace that is
// being renamed: its own name does not count as taken. The server checks the same (workspace.bad_name,
// workspace.taken); this is for the words next to the field.
export function nameProblem(
  name: string,
  store: string,
  all: Known[],
  self?: string,
): string | null {
  if (name === "") return "名前を入れてください";
  if (!NAME.test(name)) return NAME_RULE;
  const lower = name.toLowerCase();
  const taken = all.some(
    (w) => w.store === store && w.ulid !== self && w.name.toLowerCase() === lower,
  );
  return taken ? `${name} は同じ Store のワークスペースが使っています` : null;
}

// prefixProblem is nameProblem for the id prefix. The prefixes of the other stores that are mounted
// count too (workspace.prefix_taken).
export function prefixProblem(
  prefix: string,
  store: string,
  all: Known[],
  self?: string,
): string | null {
  if (prefix === "") return "接頭辞を入れてください";
  if (!PREFIX.test(prefix)) return PREFIX_RULE;
  const same = all.find((w) => w.store === store && w.ulid !== self && w.prefix === prefix);
  if (same) return `${prefix} は同じ Store のワークスペース ${same.name} が使っています`;
  const other = all.find((w) => w.store !== store && w.prefix === prefix);
  if (other) {
    return `${prefix} はマウント中の ${other.store} のワークスペース ${other.name} が使っています。別の接頭辞にしてください`;
  }
  return null;
}

// removeBlocker is why a workspace cannot be removed, or null when it can: a workspace goes only when
// it has no task at all, deleted ones included, and no sprint or release (the server answers 409
// workspace.not_empty otherwise).
export function removeBlocker(
  w: Pick<WorkspaceEntry, "tasks" | "deleted" | "sprints" | "releases">,
): string | null {
  const n = w.tasks + w.deleted;
  const parts: string[] = [];
  if (n > 0) {
    parts.push(`タスク ${n} 件${w.deleted > 0 ? `（削除済み ${w.deleted} 件を含む）` : ""}`);
  }
  if (w.sprints > 0) parts.push(`スプリント ${w.sprints} 件`);
  if (w.releases > 0) parts.push(`リリース ${w.releases} 件`);
  if (parts.length === 0) return null;
  const plans = [w.sprints > 0 && "スプリント", w.releases > 0 && "リリース"].filter(Boolean);
  const how =
    plans.length === 0
      ? "別のワークスペースへ移すか完全に削除すると消せます"
      : `${n > 0 ? "タスクを別のワークスペースへ移すか完全に削除し、" : ""}${plans.join("と")}を削除すると消せます`;
  return `${parts.join("、")}が残っているため削除できません。${how}`;
}

// detachedText says what a move took off the tasks (the sprint and the releases that are not of the
// new workspace); "" when nothing.
export function detachedText(
  ds: Schemas.Detached[] | null | undefined,
  idOf: (id: string) => string = (id) => id,
): string {
  return (ds ?? [])
    .map((d) => {
      const parts: string[] = [];
      if (d.sprint) parts.push(`スプリント ${d.sprint}`);
      if (d.releases && d.releases.length > 0) parts.push(`リリース ${d.releases.join("、")}`);
      return parts.length > 0 ? `${idOf(d.id)} は${parts.join(" と")} から外れました` : "";
    })
    .filter((t) => t !== "")
    .join("。");
}

// currentEntry reads the workspace of entry as the directory has it now (the counts move while a
// dialog is open); entry itself when the directory no longer lists it.
export function currentEntry(all: WorkspaceEntry[], entry: WorkspaceEntry): WorkspaceEntry {
  return all.find((x) => x.store === entry.store && x.ulid === entry.ulid) ?? entry;
}

// prefixDay is the day the prefix was last set. A workspace without a prefix_set (the zero time,
// year 1, which doctor reports) has none: "—".
export function prefixDay(iso: string, now: Date): string {
  return iso === "" || iso.startsWith("0001-") ? "—" : formatDate(iso, now);
}
