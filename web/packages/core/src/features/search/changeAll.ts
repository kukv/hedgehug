import type { RowSelection } from "../selection/rowSelection";
import type { SearchState } from "./search";
import { selectable } from "./table";

export interface ChangeAllDeps {
  // openSearch は、パレットの「検索で開く」と同じもの(検索の画面を、絞り込みを入れ替えて開く)。
  // 移れたかを返す(「入力を破棄」の確認で断ったら false)。
  openSearch(query: string): Promise<boolean>;
  settled(): Promise<void>;
  state(): Pick<SearchState, "result" | "resultQuery">;
  selection: Pick<RowSelection, "set">;
  openBulkChange(): Promise<void>;
}

// createChangeAll は、パレットの「{N} 件をまとめて変更…」(E2a-19)。検索の画面を開き、結果が出たら選べる行をすべて選び、
// 一括変更のダイアログを開く。結果が 0 件・エラーなら、選択を空にするだけでダイアログは開かない。
// 移れなかったときと、結果が query への答えでない(検索の失敗で前の結果が残った・待つあいだに絞り込みが変わった)ときは、
// 選択も変えずに何もしない。
export function createChangeAll(deps: ChangeAllDeps): (query: string) => Promise<void> {
  return async (query) => {
    if (!(await deps.openSearch(query))) return;
    await deps.settled();
    const { result, resultQuery } = deps.state();
    if (resultQuery !== query) return;
    const rows = result && result.errors.length === 0 ? selectable(result.rows) : [];
    deps.selection.set(rows);
    if (rows.length === 0) return;
    await deps.openBulkChange();
  };
}
