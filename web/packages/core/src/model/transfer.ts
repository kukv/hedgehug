// 別のワークスペース・別の Store への移動(02「別のワークスペースと別の Store への移動」、トラック A5a)。

// TransferEpic は、移動先のプロジェクトの、親に選べる Epic(display は MB-1)。
export interface TransferEpic {
  id: string;
  display: string;
  title: string;
}
// TransferProject は、移動先のワークスペースのプロジェクトと、その Epic(並び順)。
export interface TransferProject {
  id: string;
  title: string;
  epics: TransferEpic[];
}
// TransferTarget は、移す先の候補のワークスペース 1 つ(並びはサーバーが決める。A5a-13)。
export interface TransferTarget {
  storeId: string;
  storeName: string;
  workspaceId: string;
  workspaceName: string;
  prefix: string;
  projects: TransferProject[];
}
// TransferDestination は、予告を読む移動先。
export interface TransferDestination {
  storeId: string;
  workspaceId: string;
  projectId: string;
}
// StatusChoice は、「ステータスの置き換え」のセレクトの選択肢 1 つ。
export interface StatusChoice {
  id: string;
  name: string;
}
// ステータスの置き換えの 1 行。status は使えない元のステータスの内部 ID、count は移すもののうちその件数。
export interface StatusReplacement {
  status: string;
  name: string;
  count: number;
  choices: StatusChoice[];
  default: string;
}
// TransferPreview は、ダイアログの予告(A5a-14)。
export interface TransferPreview {
  below: number; // 下の削除されていないものの件数 N
  deleted: number; // 下の削除済みの件数 M
  nextDisplay: string; // 移動先の次の番号の ID(HH-13)
  crossStore: boolean; // 別の Store か
  remoteNotice: boolean; // 「移動先に remote がないため…」を出すか
  destinationHasRemote: boolean; // 移動先の Store に remote があるか(オフラインの判断に使う)
  replacements: StatusReplacement[]; // 移動元のワークスペースのステータスの並び
}
// TransferInput は、移動の送信の本文(A5a-19)。
export interface TransferInput {
  toStore: string;
  toWorkspace: string;
  toProject: string;
  toParent?: string; // Epic を移すときは付けない
  statusChoices: Record<string, string>; // 元のステータスの内部 ID → 移動先のステータスの内部 ID
}
// TransferResult は、移したアイテム(内部 ID は別の Store へ移しても変わらない)と、空になった項目の名前。
export interface TransferResult {
  toStore: string;
  itemId: string;
  display: string;
  emptied: string[];
}
// MovedFrom は、旧 ID から開いたときの印(A5a-15・A5a-16)。
export interface MovedFrom {
  old: string;
  at: string; // 旧 ID と、旧 ID が一致した記録の日時(ISO 8601)
  display: string; // その記録の新 ID
  current: string; // 開いたアイテムのいまの ID(display と同じなら、そのあとは移していない)
}
// GoneReason は、旧 ID で開けない理由(A5a-15)。
export type GoneReason = "deleted" | "purged" | "hidden" | "elsewhere";
// Gone は、旧 ID が見つかったが開けないときの中身(文は画面が作る)。
export interface Gone {
  reason: GoneReason;
  old: string;
  at: string; // 旧 ID と、旧 ID が一致した記録の日時(ISO 8601)
  newDisplay: string; // その記録の新 ID
  storeName?: string; // hidden: 記録の移動先の Store 名、elsewhere: より新しい記録の移動先の Store 名
}
// Located は、画面のアドレスの Store 名と ID の解決の結果。
export type Located =
  | { kind: "found"; storeId: string; projectId: string; itemId: string; movedFrom?: MovedFrom }
  | ({ kind: "gone" } & Gone);
