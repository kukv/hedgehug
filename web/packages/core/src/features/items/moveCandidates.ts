import type { MoveCandidate } from "../../model/itemAction";

// 「移動…」の候補の検索(02「同じワークスペースの中での移動」)。候補は API が全部返す(並びはプロジェクトの並び →
// ツリーの並び)ので、画面で絞る。ID の前方一致か、タイトルの部分一致で、どちらも大文字と小文字を区別しない。
// 検索語の前後の空白は取る。空なら全部。件数の上限は設けない。
export function filterMoveCandidates(all: MoveCandidate[], q: string): MoveCandidate[] {
  const word = q.trim().toLowerCase();
  if (word === "") return all;
  return all.filter(
    (c) => c.display.toLowerCase().startsWith(word) || c.title.toLowerCase().includes(word),
  );
}
