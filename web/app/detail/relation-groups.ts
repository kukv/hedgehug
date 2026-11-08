import type { ShowView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import { propNames } from "../../core/schema-edit";
import type { Lookup } from "./lookup";

export type RelItem = {
  raw: string;
  ulid: string;
  id: string;
  title: string;
  status: string | null;
};
export type RelGroup = {
  key: string;
  label: string;
  note: string;
  items: RelItem[];
  removable: boolean;
};
export type RefPatch = { blocked_by?: string[]; props?: Record<string, unknown> };

function asList(v: unknown): string[] {
  if (Array.isArray(v)) return v.filter((x): x is string => typeof x === "string");
  return typeof v === "string" && v !== "" ? [v] : [];
}

// buildGroups lists the relations of a task: blocked_by, each task_ref property, their reverse sides
// (labelled with the property's inverse) and the backlinks. A task that cannot be read keeps the
// reference as it is stored; empty groups are left out.
export async function buildGroups(
  show: ShowView,
  sc: SchemaView,
  lookup: Lookup,
): Promise<RelGroup[]> {
  const item = async (
    raw: string,
    ref?: { ulid: string; id: string; title: string },
  ): Promise<RelItem> => {
    const c = await lookup(ref?.ulid ?? raw);
    return {
      raw,
      ulid: c?.ulid ?? ref?.ulid ?? raw,
      id: c?.id ?? ref?.id ?? raw,
      title: c?.title ?? ref?.title ?? "",
      status: c?.status ?? null,
    };
  };
  const groups: RelGroup[] = [];

  const blockers = show.blocked_by ?? [];
  if (blockers.length > 0) {
    groups.push({
      key: "blocked_by",
      label: "ブロックされている",
      note: "blocked_by",
      removable: true,
      items: await Promise.all(blockers.map((r) => item(r.ulid, r))),
    });
  }
  for (const name of propNames(sc)) {
    const def = sc.props[name];
    if (def?.type !== "task_ref") continue;
    const raws = asList(show.props[name]);
    if (raws.length === 0) continue;
    groups.push({
      key: name,
      label: def.label || name,
      note: name + (def.symmetric ? "（双方向）" : ""),
      removable: true,
      items: await Promise.all(raws.map((r) => item(r))),
    });
  }
  for (const name of Object.keys(show.reverse ?? {}).sort()) {
    const refs = show.reverse[name] ?? [];
    if (refs.length === 0) continue;
    const def = sc.props[name];
    groups.push({
      key: `reverse:${name}`,
      label: def?.inverse || `${def?.label || name}（逆）`,
      note: `${name} の逆方向`,
      removable: false,
      items: await Promise.all(refs.map((r) => item(r.ulid, r))),
    });
  }
  const back = show.referenced_by ?? [];
  if (back.length > 0) {
    groups.push({
      key: "referenced_by",
      label: "バックリンク",
      note: `本文・コメントの #${show.id}`,
      removable: false,
      items: await Promise.all(back.map((r) => item(r.ulid, r))),
    });
  }
  return groups;
}

// relationKinds is what "関連を追加" can add: blocked_by and the task_ref properties.
export function relationKinds(sc: SchemaView): { key: string; label: string }[] {
  const kinds = [{ key: "blocked_by", label: "ブロックされている" }];
  for (const name of propNames(sc)) {
    const def = sc.props[name];
    if (def?.type === "task_ref") kinds.push({ key: name, label: def.label || name });
  }
  return kinds;
}

export function addRefBody(show: ShowView, sc: SchemaView, kind: string, ulid: string): RefPatch {
  if (kind === "blocked_by") {
    const cur = (show.blocked_by ?? []).map((r) => r.ulid);
    return { blocked_by: cur.includes(ulid) ? cur : [...cur, ulid] };
  }
  if (sc.props[kind]?.multi) {
    const cur = asList(show.props[kind]);
    return { props: { [kind]: cur.includes(ulid) ? cur : [...cur, ulid] } };
  }
  return { props: { [kind]: ulid } };
}

export function removeRefBody(show: ShowView, sc: SchemaView, kind: string, raw: string): RefPatch {
  if (kind === "blocked_by")
    return { blocked_by: (show.blocked_by ?? []).map((r) => r.ulid).filter((u) => u !== raw) };
  if (sc.props[kind]?.multi)
    return { props: { [kind]: asList(show.props[kind]).filter((v) => v !== raw) } };
  return { props: { [kind]: null } };
}
