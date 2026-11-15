import type { SearchState } from "../search/layout";
import { type ViewContent, viewContent } from "../views/content";
import type { SavedView } from "../views/views";
import { type ActionDeps, moveTo } from "./actions";
import type { Candidate } from "./candidates";
import { byName } from "./match";

// パレットの「ビュー」の群と「ビューとして保存…」(E3b-16)。

// viewCandidates は、名前に入力を含むビューを一覧の順で返す(件数の上限と、入力が空のときに出さないのは buildRows)。
// 実行でそのビューを開き、Ctrl+Enter では新しいタブに開く。
export function viewCandidates(a: ActionDeps, list: SavedView[], input: string): Candidate[] {
  const all = list.map((v) =>
    moveTo(a, `view:${v.id}`, "view", v.name, { name: "view", id: v.id }),
  );
  return byName(all, input);
}

export interface SaveQueryAsViewDeps {
  // openSearch は、「検索で開く」と同じもの。移れたかを返す(「入力を破棄」の確認で断ったら false)。
  openSearch(query: string): Promise<boolean>;
  settled(): Promise<void>;
  state(): SearchState;
  openSave(content: ViewContent): void;
}

// createSaveQueryAsView は、「ビューとして保存…」。検索の画面をテーブルで開き、検索が落ち着いてから、そのときの
// 画面の内容(別のタブで保存した列を含む)でダイアログを開く。移れなかったら何もしない。
export function createSaveQueryAsView(deps: SaveQueryAsViewDeps): (query: string) => Promise<void> {
  return async (query) => {
    if (!(await deps.openSearch(query))) return;
    await deps.settled();
    deps.openSave(viewContent(deps.state()));
  };
}
