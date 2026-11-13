// 履歴の 1 行(02「履歴」)。文はサーバーが作る。who は、操作した人の表示。at は、ミリ秒までの RFC 3339。
export interface HistoryRow {
  who: string;
  text: string;
  at: string;
}
