import { readSearchState, type SavedSearch, type SearchLayout } from "../search/layout";
import type { TableSettings } from "../search/table";

// 保存したビューの内容(E3b-1)。ツリーの開閉は内容に入れない。

export interface ViewContent {
  query: string;
  layout: SearchLayout;
  table: TableSettings;
}

// viewContent は、内容を readSearchState で形をそろえて取り出す(項目の順と、形の合わない値の初めの値がそろう)。
export function viewContent(s: { query: unknown; layout: unknown; table: unknown }): ViewContent {
  const { query, layout, table } = readSearchState(s);
  return { query, layout, table };
}

// viewDirty は、画面の内容が保存した内容と違うか(「未保存の変更」。E3b-4)。列の並びの違いも変更とみなす。
export function viewDirty(saved: ViewContent, screen: SavedSearch): boolean {
  return JSON.stringify(viewContent(saved)) !== JSON.stringify(viewContent(screen));
}

// viewScreenKey は、ビューの画面の内容を置く画面の状態の鍵(E3b-3)。
export const viewScreenKey = (id: string) => `view:${id}`;
