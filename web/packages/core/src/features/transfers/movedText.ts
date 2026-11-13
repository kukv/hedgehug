import type { Gone, MovedFrom } from "../../model/transfer";
import { formatDateTime } from "../dates/date";

// 旧 ID の文(02「見つからないとき」「移した日時の表示」、A5a-15・A5a-16)。サーバーは文を作らず、日時は
// 画面が formatDateTime で書く(日時の書き方を Go と TS に 2 つ持たないため)。

const when = (at: string, now: Date) => formatDateTime(new Date(at), now);
const unseen = "この Store はいま表示できません";

// movedLine は、旧 ID から開いたときに詳細パネルの上部に出す 1 行。
export function movedLine(m: MovedFrom, now: Date): string {
  const head = `${m.old} は ${when(m.at, now)} に ${m.display} へ移動`;
  return m.display === m.current
    ? `${head}しました`
    : `${head}し、その後 ${m.current} へ移動しました`;
}

// goneText は、旧 ID が見つかったが開けないときのトーストの文。
export function goneText(g: Gone, now: Date): string {
  const head = `${g.old} は ${when(g.at, now)} に`;
  switch (g.reason) {
    case "deleted":
      return `${head} ${g.newDisplay} へ移動しましたが、削除されています。設定の「削除済み」から復元できます`;
    case "purged":
      return `${head} ${g.newDisplay} へ移動しましたが、完全に削除されました`;
    case "hidden":
      return `${head} ${g.storeName} へ移動しました。${unseen}`;
    case "elsewhere":
      return `${head} ${g.newDisplay} へ移動し、その後 ${g.storeName} へ移動しました。${unseen}`;
  }
}
