export type Kind = "Epic" | "Story" | "Task" | "Bug" | "Step";

export interface Item {
  id: string;
  storeId: string;
  workspaceId: string;
  projectId: string;
  // parentId は、親の内部 ID。Epic は null。
  parentId: string | null;
  number: number;
  kind: Kind;
  rank: string;
  title: string;
  watchers: string[];
  // blocked は、ブロック中か(ブロックしている相手に、完了・中止でないものがある。サーバーが決める)。
  blocked: boolean;
  createdAt: string;
  createdBy: string;
  // status・priority は値の ID。assignee はログイン名、start・end は YYYY-MM-DD(空は未設定)。
  status: string;
  priority: string;
  assignee: string;
  start: string;
  end: string;
  points: number | null;
  // estimate は、見積もり時間(時間の数)。
  estimate: number | null;
  systems: string[];
  // sprint・release は内部 ID。空はバックログ・「なし」。
  sprint: string;
  release: string;
  // choices は、選択肢の追加プロパティの値の ID(プロパティの ID ごと)。fields は、テキストなどの追加プロパティの値。
  choices: Record<string, string[]>;
  fields: Record<string, string | number | boolean | null>;
}

// PropertyInput は、プロパティの入力欄の値。数と日付は文字列、複数は ID の配列、チェックは真偽、空は ""。
export type PropertyInput = string | number | boolean | string[] | null;

export interface NewItem {
  projectId: string;
  parentId: string | null;
  kind: Kind;
  title: string;
  // values は、作成のときに書くプロパティの値(プロパティの ID ごと)。
  values: Record<string, PropertyInput>;
  // templateId は、一緒に作るテンプレート(空かなしは「なし」)。
  templateId?: string;
  // description は、作るアイテムの説明(空・なしは説明なし)。
  description?: string;
  // sprint は、入れるスプリントの内部 ID(空・なしはバックログ)。
  sprint?: string;
}
