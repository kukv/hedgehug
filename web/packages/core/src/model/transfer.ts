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
interface GoneBase {
  old: string;
  at: string; // 旧 ID と、旧 ID が一致した記録の日時(ISO 8601)
  newDisplay: string; // その記録の新 ID
}
// Gone は、旧 ID が見つかったが開けないときの中身(文は画面が作る)。storeName は、hidden なら記録の移動先の
// Store 名、elsewhere ならより新しい記録の移動先の Store 名で、どちらも文に書くので必ずある。
export type Gone =
  | (GoneBase & { reason: "deleted" | "purged"; storeName?: string })
  | (GoneBase & { reason: "hidden" | "elsewhere"; storeName: string });
// Located は、画面のアドレスの Store 名と ID の解決の結果。
export type Located =
  | { kind: "found"; storeId: string; projectId: string; itemId: string; movedFrom?: MovedFrom }
  | ({ kind: "gone" } & Gone);

// ここから、プロジェクトの別のワークスペース・別の Store への移動(02 **移動**、トラック A5b)。

// ProjectTransferTarget は、プロジェクトを移す先の候補のワークスペース 1 つ(並びはサーバーが決める。A5b-16)。
export interface ProjectTransferTarget {
  storeId: string;
  storeName: string;
  workspaceId: string;
  workspaceName: string;
}
// NotCarriedKind は、「移らないもの」の種類(A5b-8)。
export type NotCarriedKind =
  | "hidden"
  | "hiddenFixed"
  | "workflow"
  | "template"
  | "templateDefault"
  | "disabled";
// NotCarried は、移動先に引き継げないプロジェクト別のルール 1 つ(文は画面が作る。A5b-8)。
export interface NotCarried {
  kind: NotCarriedKind;
  name: string;
  missing: string[]; // 移動先にない名前(workflow は From・To の順、templateDefault はテンプレート名)
  count: number; // disabled の件数
}
// ProjectTransferPreview は、確認ダイアログの予告(A5b-23)。
export interface ProjectTransferPreview {
  items: number; // 削除されていないアイテムの数 N
  deleted: number; // 削除済みのアイテムの数 M
  prefix: string; // 移動先の接頭辞
  source: string; // {移動元} の文(A5b-10)
  destination: string; // {移動先} の文(A5b-10)
  crossStore: boolean;
  remoteNotice: boolean;
  destinationHasRemote: boolean; // オフラインの判断に使う
  hasRules: boolean; // プロジェクト別のルールか専用ルールがあるか
  notCarried: NotCarried[];
  replacements: StatusReplacement[]; // 移動元のワークスペースのステータスの並び。ルールだけが使うものは count が 0
}
// ProjectTransferInput は、プロジェクトの移動の送信の本文(A5b-22)。
export interface ProjectTransferInput {
  toStore: string;
  toWorkspace: string;
  statusChoices: Record<string, string>; // 元のステータスの内部 ID → 移動先のステータスの内部 ID
}
// ProjectTransferResult は、移したプロジェクト(内部 ID は変わらない)と、{移動先} の文(トーストに使う)。
export interface ProjectTransferResult {
  toStore: string;
  projectId: string;
  display: string;
  destination: string;
}
// LocatedProject は、画面のアドレスの Store 名とプロジェクトの ID の解決の結果。
export type LocatedProject =
  | { kind: "found"; storeId: string; projectId: string; movedFrom?: MovedFrom }
  | ({ kind: "gone" } & Gone);

// ここから、ワークスペースの別の Store への移動(03「別の Store へ移動」、トラック B5c)。

// WorkspaceTransferTarget は、ワークスペースを移す先の Store 1 つ(並びは Store の並び。B5c-15)。
export interface WorkspaceTransferTarget {
  storeId: string;
  storeName: string;
}
// WorkspaceTransferTargets は、移す先の候補。hiddenOnly は、候補が 0 件で、非表示の Store なら移せるとき true。
export interface WorkspaceTransferTargets {
  targets: WorkspaceTransferTarget[];
  hiddenOnly: boolean;
}
// WorkspaceTransferPerson は、移動先のメンバーに足す人。
export interface WorkspaceTransferPerson {
  login: string;
  name: string; // 表示名
}
// WorkspaceTransferPreview は、確認ダイアログの予告(B5c-20)。
export interface WorkspaceTransferPreview {
  source: string; // 移動元の Store 名
  destination: string; // 移動先の Store 名
  projects: number;
  items: number; // 削除されていないアイテムの数 I
  deleted: number; // 削除済みの数 M
  people: WorkspaceTransferPerson[]; // 足す人(小文字のログイン名の順)
  remoteNotice: boolean;
  destinationHasRemote: boolean; // オフラインの判断に使う
}
// WorkspaceTransferResult は、移したワークスペース(内部 ID は変わらない)と、移動先の Store 名・接頭辞。
export interface WorkspaceTransferResult {
  toStore: string;
  toStoreName: string;
  workspaceId: string;
  prefix: string;
}
// LocatedWorkspace は、画面のアドレスの Store 名と接頭辞の解決の結果。
export type LocatedWorkspace =
  | { kind: "found"; storeId: string; workspaceId: string; movedFrom?: MovedFrom }
  | ({ kind: "gone" } & Gone);
