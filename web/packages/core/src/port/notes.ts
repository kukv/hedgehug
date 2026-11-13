// 説明・コメント・作業時間・関連・履歴の口(アイテムとプロジェクト)。実装は api パッケージにある。
// 失敗は、断りなら Refused、ないものなら NotFound、それ以外はそのまま投げる。
// 区画ごとのタスクが、自分のメソッドをここに足す。

import type { Item } from "../model/item";
import type { Comment, Worklog } from "../model/notes";
import type { RelationCandidate, RelationLabel, RelationRow } from "../model/relation";

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
  // listWorklogs は、アイテムの作業時間の記録を古い順にすべて返す。
  listWorklogs(storeId: string, itemId: string): Promise<Worklog[]>;
  // addWorklog は、作業時間を記録する。hours は入力のままの文字列で、サーバーも検査する(断りは Refused)。
  addWorklog(storeId: string, itemId: string, hours: string, memo: string): Promise<Worklog>;
  deleteWorklog(storeId: string, itemId: string, worklogId: string): Promise<void>;
  // 関連(02「関連」)。label は、このアイテムから見た表記。
  listRelations(storeId: string, itemId: string): Promise<RelationRow[]>;
  relationCandidates(
    storeId: string,
    itemId: string,
    label: RelationLabel,
    query: string,
  ): Promise<RelationCandidate[]>;
  addRelation(
    storeId: string,
    itemId: string,
    label: RelationLabel,
    otherId: string,
  ): Promise<void>;
  removeRelation(
    storeId: string,
    itemId: string,
    label: RelationLabel,
    otherId: string,
  ): Promise<void>;
  // setWatch は、あなたのウォッチを入り切りして、変えたあとのアイテムを返す。
  setWatch(storeId: string, itemId: string, on: boolean): Promise<Item>;
}
