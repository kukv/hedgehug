// アイテムとプロジェクトの操作(トラック A4b)の core の export。index.ts が全部を再公開する
// (index.ts は 300 行の上限なので、A4b の分はここに足す)。core の外(app・api・ui・e2e)が使うものだけを出す。

export { createCopyId } from "./features/items/copyId";
export { createItemDelete } from "./features/items/delete";
export { createItemDuplicate } from "./features/items/duplicate";
export { keepsQueue } from "./features/items/keyQueue";
export { createItemKind, type ItemKind, kindOptions } from "./features/items/kind";
export { createItemMenuHost, type ItemMenuHost, menuAt } from "./features/items/menuHost";
export { createItemMove } from "./features/items/move";
export { createItemRefresh } from "./features/items/refresh";
export { createTreeFocus, type TreeFocus } from "./features/items/treeFocus";
export {
  isQueuedAction,
  leftToTarget,
  type TreeKeyAction,
  treeKeyAction,
} from "./features/items/treeKeys";
export { createTreeWrites } from "./features/items/treeWrites";
export { createProjectDelete, leaveDeletedProject } from "./features/projects/delete";
export { projectTabOf } from "./features/router/route";
export { createAfterMoved } from "./features/transfers/afterMoved";
export {
  createTransferDialog,
  type ReplacementRow,
  type TransferDialog,
  type TransferDialogDeps,
  type TransferDialogState,
} from "./features/transfers/dialog";
export {
  changeRows,
  changesHeading,
  noEpicMessage,
  noTargetsMessage,
  offlineMessage,
  replacementsHeading,
  transferFailedMessage,
  transferIntro,
  transferredMessage,
} from "./features/transfers/dialogText";
export { createLeaveMoved } from "./features/transfers/leaveMoved";
export { createMovedNotice, type MovedNotice } from "./features/transfers/movedNotice";
export { openMoved } from "./features/transfers/openMoved";
export {
  createProjectTransfer,
  type ProjectTransfer,
  type ProjectTransferDeps,
  type ProjectTransferDialogState,
  type ProjectTransferSection,
  type ProjectTransferSubject,
} from "./features/transfers/projectDialog";
export {
  noProjectTargetsMessage,
  notCarriedHeading,
  notCarriedLine,
  projectMovedMessage,
  projectTargetLabel,
  projectTransferIntro,
  projectTransferNote,
  projectTransferOk,
  projectTransferTitle,
  replacementLabel,
  replacementSelectLabel,
} from "./features/transfers/projectDialogText";
export {
  createWorkspaceDelete,
  type DeleteWorkspaceTarget,
  leaveDeletedWorkspace,
  type WorkspaceDelete,
  type WorkspaceDeleteState,
} from "./features/workspaces/delete";
export type { TemplateSubject } from "./features/workspaces/saveAsTemplate";
export { deleteMenuItem } from "./features/workspaces/sidebarMenu";
export type { MoveCandidate, MovePreview, MovePreviewStatus } from "./model/itemAction";
export type {
  Gone,
  GoneReason,
  Located,
  LocatedProject,
  MovedFrom,
  NotCarried,
  NotCarriedKind,
  ProjectTransferInput,
  ProjectTransferPreview,
  ProjectTransferResult,
  ProjectTransferTarget,
  StatusChoice,
  StatusReplacement,
  TransferDestination,
  TransferEpic,
  TransferInput,
  TransferPreview,
  TransferProject,
  TransferResult,
  TransferTarget,
} from "./model/transfer";
export type { ItemActionsPort } from "./port/itemActions";
export type { TransfersPort } from "./port/transfers";
