import { layoutChoices } from "../search/layout";
import type { Toasts } from "../toasts/toasts";
import type { ViewContent } from "./content";
import type { Views } from "./views";

// 「ビューとして保存」(E3b-10)の確認の表示の文と、保存できたあとの流れ。

// saveSummary は、確認の表示「レイアウト: {名前} ・ 条件: {クエリ}」の部品。query は前後の空白を除き、空なら null
// (画面は「(なし)」と出す)。
export function saveSummary(c: ViewContent): { layout: string; query: string | null } {
  const query = c.query.trim();
  return {
    layout: layoutChoices.find((l) => l.id === c.layout)?.label ?? c.layout,
    query: query === "" ? null : query,
  };
}

export interface SaveAsViewDeps {
  views: Pick<Views, "create">;
  toasts: Toasts;
  open(id: string): Promise<unknown>;
}

// saveAsView は、ビューを作る。断られたらその文を返す(ダイアログの入力欄の下に出す)。作れたら、トーストを出して
// そのビューの画面へ移り、null を返す。断り以外の失敗(エラーのトーストは views が出した)も null で、何もしない。
export async function saveAsView(
  deps: SaveAsViewDeps,
  name: string,
  c: ViewContent,
): Promise<string | null> {
  const r = await deps.views.create(name, c);
  if (r === null || typeof r === "string") return r;
  deps.toasts.show(`ビュー「${r.name}」を保存しました`);
  await deps.open(r.id);
  return null;
}
