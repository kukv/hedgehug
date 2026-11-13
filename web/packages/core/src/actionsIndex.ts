// アイテムとプロジェクトの操作(トラック A4b)の core の export。index.ts が全部を再公開する
// (index.ts は 300 行の上限なので、A4b の分はここに足す)。

export { type CopyIdDeps, createCopyId } from "./features/items/copyId";
export {
  createItemDelete,
  type DeleteDialogState,
  type ItemDelete,
  type ItemDeleteDeps,
  itemDeleteDialog,
  itemDeletedMessage,
  itemDeleteFailedMessage,
} from "./features/items/delete";
export {
  createItemDuplicate,
  duplicateFailedMessage,
  type ItemDuplicate,
  type ItemDuplicateDeps,
} from "./features/items/duplicate";
export {
  createItemKind,
  type ItemKind,
  type ItemKindDeps,
  type KindState,
  kindFailedMessage,
  kindOptions,
} from "./features/items/kind";
export {
  type ItemMenuContext,
  type ItemMenuId,
  itemMenu,
  itemMenuContext,
  type MenuEntry,
  menuReasons,
} from "./features/items/menu";
export {
  createItemMenuHost,
  type ItemMenuHost,
  type ItemMenuHostContext,
  type ItemMenuHostDeps,
  type ItemMenuState,
  menuAt,
  reorderFailedMessage,
} from "./features/items/menuHost";
export {
  createItemMove,
  type ItemMove,
  type ItemMoveDeps,
  type MoveMenuState,
  movedToProjectMessage,
  movedUnderMessage,
  moveFailedMessage,
} from "./features/items/move";
export { filterMoveCandidates } from "./features/items/moveCandidates";
export {
  createItemRefresh,
  type ItemRefresh,
  type ItemRefreshDeps,
} from "./features/items/refresh";
export { visibleNeighbors, visibleRows } from "./features/items/siblings";
export {
  createProjectDelete,
  type DeleteTarget,
  type ProjectDelete,
  type ProjectDeleteDeps,
  type ProjectDeleteDialogState,
  projectDeleteDialog,
  projectDeletedMessage,
  projectDeleteNote,
} from "./features/projects/delete";
export { projectTabOf } from "./features/router/route";
export type { TemplateSubject } from "./features/workspaces/saveAsTemplate";
export type {
  DeletePreview,
  MoveCandidate,
  MoveTarget,
  ProjectDeletePreview,
} from "./model/itemAction";
export type { ItemActionsPort } from "./port/itemActions";
