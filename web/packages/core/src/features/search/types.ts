// 検索の結果(04「検索」)。サーバーの POST /api/search の応答を core の型にしたもの。

export interface SearchStatus {
  name: string;
  color: string;
  category: string;
}

export interface SearchAssignee {
  login: string;
  // name は表示名。メンバーにいない人は null。
  name: string | null;
}

export interface SearchPriority {
  name: string;
  color: string;
}

// SearchDeleted は、削除済みの行だけ。daysLeft は完全に削除されるまでの日数で、保持日数を過ぎていれば負。
export interface SearchDeleted {
  daysLeft: number;
}

export interface SearchRow {
  storeId: string;
  storeName: string;
  storeColor: string;
  workspaceId: string;
  projectId: string;
  itemId: string;
  // display は、いまの ID(HH-12)。
  display: string;
  title: string;
  kind: string;
  status: SearchStatus | null;
  assignee: SearchAssignee | null;
  priority: SearchPriority | null;
  // end は終了日(YYYY-MM-DD)。なければ空。
  end: string;
  points: number | null;
  // oldIdHit は、旧 ID だけで一致した行(「旧 ID」と添える)。
  oldIdHit: boolean;
  readOnly: boolean;
  deleted: SearchDeleted | null;
}

export interface SearchResult {
  // errors はクエリのエラー(1 語 1 行)。hasKey はキーのある語が 1 つ以上あるか。
  errors: string[];
  hasKey: boolean;
  // group は `group:` の値(グループ分けは E2)。なければ空。deleted は `is:deleted` の検索か。
  group: string;
  deleted: boolean;
  rows: SearchRow[];
}
