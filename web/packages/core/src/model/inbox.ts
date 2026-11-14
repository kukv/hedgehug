// 受信箱(06)の型。行の文・理由・ボタンの状態はサーバーが作る。

// InboxTab は、受信箱のタブ。
export type InboxTab =
  | "all"
  | "mention"
  | "assign"
  | "watch"
  | "overwrite"
  | "automation"
  | "repair";

// InboxRowKind は、通知の種類。automation_stopped は、自動化が制約で止まった通知。
export type InboxRowKind =
  | "automation"
  | "automation_stopped"
  | "mention"
  | "assign"
  | "watch"
  | "overwrite"
  | "repair";

// InboxTargetKind は、行の対象。moved は別の Store へ移ったアイテム(display は通知を作ったときの ID)。
export type InboxTargetKind = "item" | "moved" | "deleted_item" | "workspace" | "deleted_workspace";

export interface InboxTarget {
  kind: InboxTargetKind;
  // item は、アイテムの内部 ID。display は、画面に出す ID(HH-12)。itemKind は、item のときのアイテムの種類。
  item: string;
  display: string;
  itemKind: string;
  title: string;
  workspace: string;
  workspaceName: string;
  // section は、workspace のとき開くワークスペースの設定の区画。
  section: string;
}

export interface InboxStore {
  id: string;
  name: string;
  // color は、画面のトークン --t-<色> の名前。
  color: string;
  readOnly: boolean;
}

// InboxAction は、止まった通知の 2 つのボタンの状態(C4a-10)。result が空でなければ、ボタンの代わりに結果を出す。
export interface InboxAction {
  visible: boolean;
  applyEnabled: boolean;
  keepEnabled: boolean;
  note: string;
  result: "" | "applied" | "kept";
  status: string;
  statusName: string;
}

export interface InboxRow {
  id: number;
  // at は、通知を作った時刻(RFC 3339、ナノ秒まで)。続きの印にそのまま渡す。
  at: string;
  kind: InboxRowKind;
  read: boolean;
  // sender は、差出人のログイン名。空は「hedgehug」。
  sender: string;
  text: string;
  reason: string;
  store: InboxStore;
  target: InboxTarget;
  action?: InboxAction;
}

export type InboxCounts = Record<InboxTab, number>;

export interface InboxPage {
  rows: InboxRow[];
  // more は、続きがあるか。
  more: boolean;
  unread: InboxCounts;
}
