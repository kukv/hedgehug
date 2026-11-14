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

// InboxTargetKind は、行の対象。moved・moved_project は別の Store へ移ったアイテム・プロジェクト(display は通知を作ったときの ID)。store は、アイテム・プロジェクト・ワークスペースのどれも指さない(メンバーの表示名。リンクなし)。
export type InboxTargetKind =
  | "item"
  | "moved"
  | "deleted_item"
  | "workspace"
  | "deleted_workspace"
  | "project"
  | "moved_project"
  | "deleted_project"
  | "store";

export interface InboxTarget {
  kind: InboxTargetKind;
  // item は、アイテムの内部 ID。display は、画面に出す ID(HH-12)。itemKind は、item のときのアイテムの種類。
  item: string;
  // project は、プロジェクトの内部 ID(project・moved_project・deleted_project のとき)。
  project: string;
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

// InboxOverwrite は、上書きの通知の「あなたの値 → 残った値」と「自分の値に戻す」の状態(C4c2-11)。result が空でなければ、ボタンの代わりに結果を出す。
export interface InboxOverwrite {
  mine: string;
  kept: string;
  visible: boolean;
  enabled: boolean;
  note: string;
  result: "" | "applied";
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
  overwrite?: InboxOverwrite;
}

export type InboxCounts = Record<InboxTab, number>;

export interface InboxPage {
  rows: InboxRow[];
  // more は、続きがあるか。
  more: boolean;
  unread: InboxCounts;
}
