// コマンドパレット(02「コマンドパレット」、A5c)の core の export。index.ts が全部を再公開する。

export {
  buildRows,
  type Candidate,
  type GroupId,
  type Row,
} from "./features/palette/candidates";
export {
  byName,
  type Matched,
  matchItems,
  matchProjects,
  matchWorkspaces,
} from "./features/palette/match";
