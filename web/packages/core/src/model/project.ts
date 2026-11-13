export type ProjectStatus = "Todo" | "InProgress" | "Done" | "Pending" | "Cancel";

export interface Project {
  id: string;
  storeId: string;
  workspaceId: string;
  number: number;
  rank: string;
  title: string;
  // owner は、ログイン名。空は「なし」。
  owner: string;
  status: ProjectStatus;
  // start・end は YYYY-MM-DD。空は「なし」。
  start: string;
  end: string;
  // repos は、結びつけた GitHub のリポジトリ(owner/name)。大文字小文字を区別しない辞書順。
  repos: string[];
}

export interface NewProject {
  title: string;
  owner: string;
  status: ProjectStatus;
  start: string;
  end: string;
}
