// 保存したビュー(トラック E3b)の core の export。index.ts が全部を再公開する(index.ts は 300 行の上限に近い)。

export {
  type ViewContent,
  viewContent,
  viewDirty,
  viewScreenKey,
} from "./features/views/content";
export { createNameDialog, type NameDialog } from "./features/views/nameDialog";
export { saveAsView, saveSummary } from "./features/views/saveDialog";
export { createViews, type SavedView, type Views } from "./features/views/views";
