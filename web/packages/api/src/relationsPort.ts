import type { NotesPort, RelationLabel } from "@hedgehug/core";
import { call } from "./errors";
import { addRelation, listRelationCandidates, listRelations, removeRelation } from "./generated";

export type RelationCalls = Pick<
  NotesPort,
  "listRelations" | "relationCandidates" | "addRelation" | "removeRelation"
>;

// relationCalls は、NotesPort の関連のメソッド(createNotesPort の戻り値に広げる)。
export function relationCalls(): RelationCalls {
  return {
    listRelations: (storeId, itemId) =>
      call(async () =>
        ((await listRelations(storeId, itemId)) ?? []).map((r) => ({
          ...r,
          label: r.label as RelationLabel,
        })),
      ),
    relationCandidates: (storeId, itemId, label, query) =>
      call(async () => (await listRelationCandidates(storeId, itemId, { label, q: query })) ?? []),
    addRelation: (storeId, itemId, label, otherId) =>
      call(async () => {
        await addRelation(storeId, itemId, { label, other: otherId });
      }),
    removeRelation: (storeId, itemId, label, otherId) =>
      call(async () => {
        await removeRelation(storeId, itemId, { label, other: otherId });
      }),
  };
}
