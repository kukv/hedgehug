// アクティビティ(06「アクティビティ」)の型。行の文・実行者の名前・対象の形はサーバーが作る。

// ActivityKind は、種類の絞り込み。
export type ActivityKind = "all" | "user" | "sync" | "automation";

// ActivityFilter の workspace は、空(すべて)か「{storeId}/{workspaceId}」。
export interface ActivityFilter {
  workspace: string;
  kind: ActivityKind;
}

export type ActivityActor = "user" | "automation" | "github" | "sync" | "none";

// ActivityTarget の link は、current ならいまの ID で開き、moved なら別の Store へ移した旧 ID を転送記録から探して開く。空は文字だけ。
export interface ActivityTarget {
  display: string;
  link: "" | "current" | "moved";
  // title は、プロジェクトのときだけ。
  title: string;
}

export interface ActivityStore {
  id: string;
  name: string;
  // color は、画面のトークン --t-<色> の名前。
  color: string;
}

export interface ActivityRow {
  // key は、行の鍵。cursor は、続きを読むときの before にそのまま渡す値。
  key: string;
  cursor: string;
  at: string;
  actor: ActivityActor;
  who: string;
  avatar: string;
  text: string;
  store: ActivityStore;
  workspace: string;
  item?: ActivityTarget;
  project?: ActivityTarget;
  sha: string;
}

export interface ActivityPage {
  rows: ActivityRow[];
  // more は、続きがあるか。
  more: boolean;
}
