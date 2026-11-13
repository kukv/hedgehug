// コメント 1 件(02「コメント」)。by は投稿者のログイン名、byName は表示名(メンバーにいなければ「不明なユーザー」)、
// at は投稿の時刻(RFC 3339)。edited は、投稿のあとに編集されているとき真。
export interface Comment {
  id: string;
  by: string;
  byName: string;
  at: string;
  body: string;
  edited: boolean;
}

// 作業時間の記録 1 件(02「作業時間」)。hours は「1.25」の表記(小数第 2 位まで・末尾の 0 を省く)、
// date は記録した日(YYYY-MM-DD)、byName は記録した人の表示名(メンバーにいなければ「不明なユーザー」)。
export interface Worklog {
  id: string;
  by: string;
  byName: string;
  date: string;
  hours: string;
  memo: string;
}
