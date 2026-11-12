import type { Item, ItemsPort } from "@hedgehug/core";
import { call } from "./errors";
import {
  type Item as ApiItem,
  addStep,
  createItem,
  findItemByNumber,
  getItem,
  listProjectItems,
  moveItemUp,
  nextItemNumber,
  renameItem,
} from "./generated";

export function toItem(storeId: string, it: ApiItem): Item {
  return {
    id: it.id,
    storeId,
    workspaceId: it.workspace,
    projectId: it.project,
    parentId: it.parent,
    number: it.number,
    kind: it.kind,
    rank: it.rank,
    title: it.title,
    watchers: it.watchers ?? [],
    createdAt: it.createdAt,
    createdBy: it.createdBy,
  };
}

export function createItemsPort(): ItemsPort {
  return {
    list: (storeId, projectId) =>
      call(async () =>
        ((await listProjectItems(storeId, projectId)) ?? []).map((it) => toItem(storeId, it)),
      ),
    get: (storeId, itemId) => call(async () => toItem(storeId, await getItem(storeId, itemId))),
    create: (storeId, input) =>
      call(async () =>
        toItem(
          storeId,
          await createItem(storeId, {
            project: input.projectId,
            parent: input.parentId ?? "",
            kind: input.kind,
            title: input.title,
          }),
        ),
      ),
    rename: (storeId, itemId, title) =>
      call(async () => toItem(storeId, await renameItem(storeId, itemId, { title }))),
    addStep: (storeId, parentId, title) =>
      call(async () => toItem(storeId, await addStep(storeId, parentId, { title }))),
    moveUp: (storeId, itemId) =>
      call(async () => {
        await moveItemUp(storeId, itemId);
      }),
    nextNumber: (storeId, workspaceId) =>
      call(async () => (await nextItemNumber(storeId, workspaceId)).number),
    findByNumber: (storeId, workspaceId, number) =>
      call(async () => toItem(storeId, await findItemByNumber(storeId, workspaceId, number))),
  };
}
