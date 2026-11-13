import type { ItemActionsPort, Kind } from "@hedgehug/core";
import { call } from "./errors";
import {
  changeItemKind,
  deleteItem,
  deleteProject,
  duplicateItem,
  getItemDeletePreview,
  getProjectDeletePreview,
  listMoveCandidates,
  moveItem,
} from "./generated";

// アイテムとプロジェクトの操作(A4b)。変更の応答は使わないので、どれも値を返さない(duplicate だけ新しい ID)。
// 生成物の型名は import しない(Go の出力型の名前に左右されないため)。null の一覧は空にする。
export function createItemActionsPort(): ItemActionsPort {
  return {
    deletePreview: (storeId, itemId) =>
      call(async () => {
        const p = await getItemDeletePreview(storeId, itemId);
        return { count: p.count, kinds: (p.kinds ?? []) as Kind[], retentionDays: p.retentionDays };
      }),
    deleteItem: (storeId, itemId) =>
      call(async () => {
        await deleteItem(storeId, itemId);
      }),
    projectDeletePreview: (storeId, projectId) =>
      call(async () => {
        const p = await getProjectDeletePreview(storeId, projectId);
        return { items: p.items, retentionDays: p.retentionDays };
      }),
    deleteProject: (storeId, projectId) =>
      call(async () => {
        await deleteProject(storeId, projectId);
      }),
    moveCandidates: (storeId, itemId) =>
      call(async () =>
        ((await listMoveCandidates(storeId, itemId)) ?? []).map((c) => ({
          kind: c.kind,
          id: c.id,
          display: c.display,
          title: c.title,
          projectId: c.projectId,
          projectTitle: c.projectTitle,
        })),
      ),
    move: (storeId, itemId, to) =>
      call(async () => {
        await moveItem(storeId, itemId, to);
      }),
    changeKind: (storeId, itemId, kind) =>
      call(async () => {
        // 変えられる先は Story・Task・Bug だけ(API の enum)。Epic・Step はサーバーが断る(422 → Refused)。
        await changeItemKind(storeId, itemId, {
          kind: kind as Parameters<typeof changeItemKind>[2]["kind"],
        });
      }),
    duplicate: (storeId, itemId) => call(async () => (await duplicateItem(storeId, itemId)).id),
  };
}
