// 関連(02「関連」)。表記は画面のとおり英語のまま。

export type RelationLabel =
  | "blocks"
  | "blocked by"
  | "causes"
  | "caused by"
  | "duplicates"
  | "duplicated by"
  | "relates to"
  | "clones"
  | "cloned by";

// relationLabels は、一覧の見出しの順であり、追加のセレクトの順(02「関連」)。
export const relationLabels: readonly RelationLabel[] = [
  "blocks",
  "blocked by",
  "causes",
  "caused by",
  "duplicates",
  "duplicated by",
  "relates to",
  "clones",
  "cloned by",
];

// RelationRow は、一覧の 1 行。workspaceName は、相手が別のワークスペースのときだけ(同じなら空)。
// statusName・statusColor は、相手のステータス(値がなければ空)。
export interface RelationRow {
  label: RelationLabel;
  id: string;
  display: string;
  title: string;
  workspaceName: string;
  statusName: string;
  statusColor: string;
}

// RelationCandidate は、相手の候補。disabledReason が空でなければ選べない(その理由)。
export interface RelationCandidate {
  id: string;
  display: string;
  title: string;
  workspaceName: string;
  disabledReason: string;
}
