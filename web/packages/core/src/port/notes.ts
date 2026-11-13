// 説明・コメント・作業時間・関連・履歴の口(アイテムとプロジェクト)。実装は api パッケージにある。
// 失敗は、断りなら Refused、ないものなら NotFound、それ以外はそのまま投げる。
// 区画ごとのタスクが、自分のメソッドをここに足す。

import type { Comment } from "../model/notes";

// NoteTarget は、対象のアイテムかプロジェクト(内部 ID)。
export interface NoteTarget {
  kind: "item" | "project";
  storeId: string;
  id: string;
}

export interface DescribeResult {
  // text は、保存したあとの説明(行マージの結果)。
  text: string;
  // conflict は、同じ行を別の人も変えていて、両方の版を残したとき。
  conflict: boolean;
}

export interface NotesPort {
  // description は、保存してある説明(なければ "")。
  description(target: NoteTarget): Promise<string>;
  // describe は、説明を保存する。base は編集を始めたときの説明(行マージの共通の祖先)。
  describe(target: NoteTarget, base: string, text: string): Promise<DescribeResult>;
  // listComments は、コメントを古い順にすべて返す。
  listComments(target: NoteTarget): Promise<Comment[]>;
  postComment(target: NoteTarget, body: string): Promise<Comment>;
  editComment(target: NoteTarget, commentId: string, body: string): Promise<Comment>;
  deleteComment(target: NoteTarget, commentId: string): Promise<void>;
}
