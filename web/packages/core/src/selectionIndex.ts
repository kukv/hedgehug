// 一覧の行の選択(F2b1)の core の export。index.ts が全部を再公開する。

export {
  type BulkOption,
  bulkFields,
  changeButton,
  confirmLines,
  explanation,
  inputOf,
  summaryText,
  valueOptions as bulkValueOptions,
} from "./features/selection/bulkChangeForm";
export {
  bulkChangeToast,
  offBacklogText,
  reasonList,
} from "./features/selection/bulkChangeResult";
export {
  type BulkDelete,
  type BulkDeleteDeps,
  type BulkDeleteDialog,
  bulkDeleteDialog,
  bulkDeletedText,
  createBulkDelete,
} from "./features/selection/bulkDelete";
export {
  allReadOnly,
  createRowSelection,
  isSelected,
  type RowSelection,
  type SelectedRow,
  selectedText,
} from "./features/selection/rowSelection";
export type {
  BulkChangeInput,
  BulkChangeResult,
  BulkChoices,
  BulkField,
} from "./model/bulkChange";
