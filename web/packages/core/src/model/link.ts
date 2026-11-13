// PR・Issue のリンク(07「PR と Issue のリンク」)。checks はこのマシンが読んだ値で、Store に保存しない。

export type LinkKind = "pr" | "issue";
export type LinkState = "open" | "draft" | "merged" | "closed";
export type LinkReview = "none" | "pending" | "approved" | "changes_requested";
export type LinkChecks = "" | "success" | "failure" | "pending";

export interface LinkRow {
  kind: LinkKind;
  repo: string;
  number: number;
  title: string;
  state: LinkState;
  review: LinkReview;
  checks: LinkChecks;
  // error は、取り直しの失敗の文。なければ空。
  error: string;
}

export interface ProjectLinkRow extends LinkRow {
  // item は、アイテムの内部 ID。display は、画面の ID(HH-2)。
  item: string;
  display: string;
}

export interface LinkInput {
  kind: LinkKind;
  repo: string;
  // number は、入力のまま。正の整数でなければサーバーが断る。
  number: string;
  addToProject: boolean;
}

export type LinkTarget = Pick<LinkRow, "kind" | "repo" | "number">;

// RepoFailure は、プロジェクトのリポジトリの前回の探索の失敗。repo は書き方のまま、text は理由。
export interface RepoFailure {
  repo: string;
  text: string;
}
