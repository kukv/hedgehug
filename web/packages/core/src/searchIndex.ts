// 検索と画面の状態(トラック E1)の core の export。index.ts が全部を再公開する。

export { createScreenStates, type ScreenStates } from "./features/screenstate/screenStates";
export { type CaretTerm, caretTerm } from "./features/search/caret";
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
  SearchDeleted,
  SearchPriority,
  SearchResult,
  SearchRow,
  SearchStatus,
} from "./features/search/types";
export type { ScreenStatesPort, SearchPort } from "./port/search";
