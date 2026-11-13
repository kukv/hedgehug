// アイテムとプロジェクトの操作(トラック A4b)の core の export。index.ts が全部を再公開する
// (index.ts は 300 行の上限なので、A4b の分はここに足す)。

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
  type ItemMenuContext,
  type ItemMenuId,
  itemMenu,
  itemMenuContext,
  type MenuEntry,
  menuReasons,
} from "./features/items/menu";
export { filterMoveCandidates } from "./features/items/moveCandidates";
export {
  createItemRefresh,
  type ItemRefresh,
  type ItemRefreshDeps,
} from "./features/items/refresh";
export { visibleNeighbors, visibleRows } from "./features/items/siblings";
export type {
  DeletePreview,
  MoveCandidate,
  MoveTarget,
  ProjectDeletePreview,
} from "./model/itemAction";
export type { ItemActionsPort } from "./port/itemActions";
