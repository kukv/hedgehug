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
