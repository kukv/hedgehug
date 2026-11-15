// 検索と画面の状態(トラック E1)の core の export。index.ts が全部を再公開する。

export { createScreenStates, type ScreenStates } from "./features/screenstate/screenStates";
export {
  applyCandidate,
  type CandidateScope,
  type CandidateSources,
  candidateSource,
  type FilterCandidate,
  filterCandidates,
} from "./features/search/candidates";
export { type CaretTerm, caretTerm } from "./features/search/caret";
export { type CellView, cellView, updatedHint } from "./features/search/cells";
export { type ChangeAllDeps, createChangeAll } from "./features/search/changeAll";
export { searchHelp } from "./features/search/help";
export {
  layoutChoices,
  readSearchState,
  type SearchLayout,
  type TreeSettings,
} from "./features/search/layout";
export { rowMenuContext } from "./features/search/rowMenu";
export { type RowStatusDeps, setRowStatus } from "./features/search/rowStatus";
export {
  createSearch,
  type Search,
  type SearchDeps,
  type SearchState,
  searchScreen,
  searchWaitMs,
} from "./features/search/search";
export {
  buildSearchTree,
  type ProjectData,
  rowKey,
  type SearchTreeRow,
  type SearchTreeSection,
  type SearchTreeView,
  screenFor,
  treeScope,
} from "./features/search/searchTree";
export {
  createSearchTreeData,
  type SearchTreeData,
  type SearchTreeDataDeps,
} from "./features/search/searchTreeData";
export {
  columnChoices,
  defaultTable,
  groupChoices,
  headerCheck,
  nextSort,
  readTable,
  selectable,
  shownColumns,
  type TableSettings,
  tableRequest,
  toggleColumn,
} from "./features/search/table";
export type {
  SearchAssignee,
  SearchColumn,
  SearchDeleted,
  SearchGroup,
  SearchPriority,
  SearchResult,
  SearchRow,
  SearchStatus,
  SearchTableRequest,
} from "./features/search/types";
export {
  assigneeText,
  deletedNote,
  type EmptyText,
  emptyText,
  readOnlyBands,
} from "./features/search/view";
export type { ScreenStatesPort, SearchPort } from "./port/search";
