import type {
  BulkChangeInput,
  BulkChangeResult,
  ItemActionsPort,
  ItemRef,
  Kind,
} from "@hedgehug/core";
import { call } from "./errors";
import {
  bulkChange,
  bulkChangeChoices,
  bulkDelete,
  changeItemKind,
  deleteItem,
  deleteProject,
  duplicateItem,
  getItemDeletePreview,
  getProjectDeletePreview,
  listMoveCandidates,
  moveItem,
  previewBulkChange,
  previewBulkDelete,
  previewItemMove,
} from "./generated";

const bulkBody = (items: ItemRef[]) => ({
  items: items.map((i) => ({ store: i.storeId, item: i.itemId })),
});

const changeBody = (items: ItemRef[], input: BulkChangeInput) => ({
  ...bulkBody(items),
  field: input.field,
  value: input.value,
  values: input.values,
  category: input.category,
});

// toResult は、予告と実行の応答(null の一覧は空にする)。
const toResult = (r: Awaited<ReturnType<typeof bulkChange>>): BulkChangeResult => ({
  changed: r.changed,
  kept: (r.kept ?? []).map((k) => ({ store: k.store, item: k.item, reason: k.reason })),
  failed: (r.failed ?? []).map((f) => ({ store: f.store, name: f.name, count: f.count })),
  offBacklog: { done: r.offBacklog.done, canceled: r.offBacklog.canceled },
  valueText: r.valueText,
});

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
    movePreview: (storeId, itemId, to) =>
      call(async () => {
        const p = await previewItemMove(storeId, itemId, to);
        return {
          below: p.below,
          statuses: (p.statuses ?? []).map((s) => ({
            status: s.status,
            count: s.count,
            default: s.default,
          })),
        };
      }),
    move: (storeId, itemId, to, statuses) =>
      call(async () => {
        await moveItem(storeId, itemId, statuses ? { ...to, statuses } : to);
      }),
    changeKind: (storeId, itemId, kind) =>
      call(async () => {
        // 変えられる先は Story・Task・Bug だけ(API の enum)。Epic・Step はサーバーが断る(422 → Refused)。
        await changeItemKind(storeId, itemId, {
          kind: kind as Parameters<typeof changeItemKind>[2]["kind"],
        });
      }),
    bulkDeletePreview: (items) =>
      call(async () => {
        const p = await previewBulkDelete(bulkBody(items));
        return { stores: p.stores, retentionDays: p.retentionDays, readOnly: p.readOnly };
      }),
    bulkDelete: (items) =>
      call(async () => {
        const r = await bulkDelete(bulkBody(items));
        return { deleted: r.deleted, readOnly: r.readOnly };
      }),
    bulkChangeChoices: (items) =>
      call(async () => {
        const c = await bulkChangeChoices(bulkBody(items));
        return {
          kinds: c.kinds ?? [],
          statuses: c.statuses ?? [],
          priorities: c.priorities ?? [],
          sprints: c.sprints ?? [],
          releases: c.releases ?? [],
          systems: c.systems ?? [],
          members: (c.members ?? []).map((m) => ({ login: m.login, name: m.name })),
        };
      }),
    bulkChangePreview: (items, input) =>
      call(async () => toResult(await previewBulkChange(changeBody(items, input)))),
    bulkChange: (items, input) =>
      call(async () => toResult(await bulkChange(changeBody(items, input)))),
    duplicate: (storeId, itemId) => call(async () => (await duplicateItem(storeId, itemId)).id),
  };
}
