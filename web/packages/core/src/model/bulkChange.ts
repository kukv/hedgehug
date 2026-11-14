import type { Member } from "./member";
import type { Category } from "./workspace";

// 一覧で選んだ行の一括変更(F2b2)の入出力。

// BulkField は、一括変更できる項目(ダイアログの項目のセレクトの順)。
export type BulkField =
  | "status"
  | "assignee"
  | "priority"
  | "sprint"
  | "release"
  | "systemsAdd"
  | "systemsRemove"
  | "points"
  | "estimate"
  | "start"
  | "end";

// BulkChangeInput は、変える項目と新しい値。値は名前で送る(担当者はログイン名)。空は値なし。
// 関連システムは values、ステータスを分類で変えるときは category(value は空)を使う。
export interface BulkChangeInput {
  field: BulkField;
  value: string;
  values: string[];
  category: Category | "";
}

// BulkChoices は、選んだアイテムから作った選択肢(ダイアログのセレクトの中身)。kinds は選んだアイテムの種類。
export interface BulkChoices {
  kinds: string[];
  statuses: string[];
  priorities: string[];
  sprints: string[];
  releases: string[];
  systems: string[];
  members: Member[];
}

// BulkChangeResult は、予告と実行の答え。kept は変えずに残したもの(選んだ順)、failed は保存できなかった Store、
// offBacklog は、変えたあとバックログに表示されなくなった件数、valueText は確認の本文に出す値。
export interface BulkChangeResult {
  changed: number;
  kept: { store: string; item: string; reason: string }[];
  failed: { store: string; name: string; count: number }[];
  offBacklog: { done: number; canceled: number };
  valueText: string;
}
