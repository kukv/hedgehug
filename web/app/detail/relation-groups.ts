import type { Schemas, ShowView } from "../../core/api/client";
import { RELATIONS, type RelationField } from "../../core/relations";
import type { Lookup } from "./lookup";

type Ref = Schemas.Ref;

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
export type RefPatch = Record<string, string[]>;

const refsOf = (show: ShowView, name: string): Ref[] => show[name as RelationField] ?? [];

// buildGroups lists the relations of a task: the five it holds, then their reverse sides (labelled
// with the display name of the other direction) and the backlinks. A task that cannot be read keeps
// the reference as it is stored; empty groups are left out.
export async function buildGroups(show: ShowView, lookup: Lookup): Promise<RelGroup[]> {
  const item = async (ref: Ref): Promise<RelItem> => {
    const c = await lookup(ref.ulid);
    return {
      raw: ref.ulid,
      ulid: c?.ulid ?? ref.ulid,
      id: c?.id ?? ref.id,
      title: c?.title ?? ref.title,
      status: c?.status ?? null,
    };
  };
  const groups: RelGroup[] = [];
  const push = async (
    key: string,
    label: string,
    note: string,
    refs: Ref[],
    removable: boolean,
  ) => {
    if (refs.length === 0) return;
    groups.push({ key, label, note, removable, items: await Promise.all(refs.map(item)) });
  };

  for (const r of RELATIONS) await push(r.name, r.label, r.name, refsOf(show, r.name), true);
  for (const r of RELATIONS) {
    await push(
      `reverse:${r.inverse}`,
      r.inverseLabel,
      r.inverse,
      show.reverse?.[r.inverse] ?? [],
      false,
    );
  }
  await push(
    "referenced_by",
    "バックリンク",
    `本文・コメントの #${show.id}`,
    show.referenced_by ?? [],
    false,
  );
  return groups;
}

// relationKinds is what "関連を追加" can add: the five relations, not their reverse sides.
export const relationKinds = (): { key: string; label: string }[] =>
  RELATIONS.map((r) => ({ key: r.name, label: r.label }));

export function addRefBody(show: ShowView, kind: string, ulid: string): RefPatch {
  const cur = refsOf(show, kind).map((r) => r.ulid);
  return { [kind]: cur.includes(ulid) ? cur : [...cur, ulid] };
}

export function removeRefBody(show: ShowView, kind: string, ulid: string): RefPatch {
  return {
    [kind]: refsOf(show, kind)
      .map((r) => r.ulid)
      .filter((u) => u !== ulid),
  };
}
