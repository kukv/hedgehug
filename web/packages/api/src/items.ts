import type { Item, ItemsPort } from "@hedgehug/core";
import { call } from "./errors";
import {
  type Item as ApiItem,
  addItemValue,
  addStep,
  checkStep,
  createItem,
  findItemByNumber,
  getItem,
  listProjectItems,
  nextItemNumber,
  removeItemValue,
  renameItem,
  reorderItem,
  setItemProperty,
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
    blocked: it.blocked,
    createdAt: it.createdAt,
    createdBy: it.createdBy,
    status: it.status,
    priority: it.priority,
    assignee: it.assignee,
    start: it.start,
    end: it.end,
    points: it.points ?? null,
    estimate: it.estimate ?? null,
    systems: it.systems ?? [],
    sprint: it.sprint,
    release: it.release,
    choices: Object.fromEntries(
      Object.entries(it.choices ?? {}).map(([id, ids]) => [id, ids ?? []]),
    ),
    fields: (it.fields ?? {}) as Item["fields"],
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
            values: input.values,
            ...(input.description ? { description: input.description } : {}),
            ...(input.templateId ? { template: input.templateId } : {}),
            ...(input.sprint ? { sprint: input.sprint } : {}),
          }),
        ),
      ),
    rename: (storeId, itemId, title) =>
      call(async () => toItem(storeId, await renameItem(storeId, itemId, { title }))),
    addStep: (storeId, parentId, title) =>
      call(async () => toItem(storeId, await addStep(storeId, parentId, { title }))),
    setProperty: (storeId, itemId, propertyId, value) =>
      call(async () =>
        toItem(storeId, await setItemProperty(storeId, itemId, propertyId, { value })),
      ),
    addValue: (storeId, itemId, propertyId, valueId) =>
      call(async () => toItem(storeId, await addItemValue(storeId, itemId, propertyId, valueId))),
    removeValue: (storeId, itemId, propertyId, valueId) =>
      call(async () =>
        toItem(storeId, await removeItemValue(storeId, itemId, propertyId, valueId)),
      ),
    checkStep: (storeId, itemId, done) =>
      call(async () => toItem(storeId, await checkStep(storeId, itemId, { done }))),
    reorder: (storeId, itemId, anchorId, after) =>
      call(async () => {
        await reorderItem(storeId, itemId, { anchor: anchorId, after });
      }),
    nextNumber: (storeId, workspaceId) =>
      call(async () => (await nextItemNumber(storeId, workspaceId)).number),
    findByNumber: (storeId, workspaceId, number) =>
      call(async () => toItem(storeId, await findItemByNumber(storeId, workspaceId, number))),
  };
}
