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
export { searchHelp } from "./features/search/help";
export {
  createSearch,
  type Search,
  type SearchDeps,
  type SearchState,
  searchScreen,
  searchWaitMs,
} from "./features/search/search";
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
