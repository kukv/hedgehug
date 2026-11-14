// 検索の結果(04「検索」)。サーバーの POST /api/search の応答を core の型にしたもの。

// SearchStatus は、行のステータス。点の色は分類(category)で決める。
export interface SearchStatus {
  name: string;
  category: string;
}

export interface SearchAssignee {
  login: string;
  // name は表示名。メンバーにいない人は null。
  name: string | null;
}

export interface SearchPriority {
  name: string;
}

// SearchDeleted は、削除済みの行だけ。daysLeft は完全に削除されるまでの日数で、0 以下なら保持日数を過ぎている。
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
  // start は開始日(YYYY-MM-DD)。なければ空。estimate は見積もり時間。
  start: string;
  estimate: number | null;
  // systems は関連システムの名前(一覧にない値は「(削除された値)」)。
  systems: string[];
  sprint: string;
  release: string;
  project: { display: string; title: string };
  parent: { itemId: string; display: string; title: string } | null;
  // created・updated は RFC 3339。
  created: string;
  updated: string;
  // unused は「—」にする列の鍵、removed は「(削除された値)」にする列の鍵。
  unused: string[];
  removed: string[];
  // custom は追加プロパティの列の鍵 → 値。type は行のワークスペースでのプロパティの型。
  custom: Record<string, { text: string; type: string }>;
}

// SearchColumn は、テーブルの列。type は追加プロパティの列だけが持つ(固定の列は空)。
export interface SearchColumn {
  key: string;
  name: string;
  type: string;
  sortable: boolean;
}

// SearchGroup は、グループ 1 つ。rows は SearchResult.rows の位置。
export interface SearchGroup {
  label: string;
  rows: number[];
}

// SearchTableRequest は、テーブルの設定。sort は列見出しの並べ替え、group はテーブルの「グループ」(none か空なら設定なし)。
export interface SearchTableRequest {
  sort: { key: string; desc: boolean } | null;
  group: string;
}

export interface SearchResult {
  // errors はクエリのエラー(1 語 1 行)。hasKey はキーのある語が 1 つ以上あるか。
  errors: string[];
  hasKey: boolean;
  // group は `group:` の値。なければ空。deleted は `is:deleted` の検索か。
  group: string;
  deleted: boolean;
  rows: SearchRow[];
  // columns はテーブルの列。groupBy は使ったまとめ方(なければ空)、groups はそのグループ(分けないときは空)。
  columns: SearchColumn[];
  groupBy: string;
  groups: SearchGroup[];
}
