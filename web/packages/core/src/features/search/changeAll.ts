import type { RowSelection } from "../selection/rowSelection";
import { selectable } from "./table";
import type { SearchResult } from "./types";

export interface ChangeAllDeps {
  // openSearch は、パレットの「検索で開く」と同じもの(検索の画面を、絞り込みを入れ替えて開く)。
  openSearch(query: string): Promise<void>;
  settled(): Promise<void>;
  result(): SearchResult | null;
  selection: Pick<RowSelection, "set">;
  openBulkChange(): Promise<void>;
}

// createChangeAll は、パレットの「{N} 件をまとめて変更…」(E2a-19)。検索の画面を開き、結果が出たら選べる行をすべて選び、
// 一括変更のダイアログを開く。結果が 0 件・エラーなら、選択を空にするだけでダイアログは開かない。
export function createChangeAll(deps: ChangeAllDeps): (query: string) => Promise<void> {
  return async (query) => {
    await deps.openSearch(query);
    await deps.settled();
    const result = deps.result();
    const rows = result && result.errors.length === 0 ? selectable(result.rows) : [];
    deps.selection.set(rows);
    if (rows.length === 0) return;
    await deps.openBulkChange();
  };
}
