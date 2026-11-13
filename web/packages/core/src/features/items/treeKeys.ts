import type { Item } from "../../model/item";

// ツリーのキーの割り当て(04「ツリーのキー操作」、A4b-20)。キーを何の操作にするかだけを決め、実行は app が行う。
// core は DOM の型を持たないので、キーボードのイベントは必要な欄だけの形で受ける。

export interface KeyInput {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  // 文字を変換している最中のキーは、ツリーのキーに数えない。
  isComposing?: boolean;
}

// 読むだけのキー(閲覧のみの Store の行でも使える)。
export type ReadKeyAction = "up" | "down" | "right" | "left" | "open" | "open-tab" | "menu";
// 続けて押したとき 1 回ずつ順に実行するキー(Alt+↑↓、Alt+Shift+↑↓)。
export type QueuedKeyAction = "move-up" | "move-down" | "reparent-up" | "reparent-down";
// 書き込みのキー。閲覧のみの Store・無効のときは、実行する側が何もしない(トーストも出さない)。
export type WriteKeyAction =
  | QueuedKeyAction
  | "toggle-done"
  | "status-menu"
  | "change-kind"
  | "create-sibling";
export type TreeKeyAction = ReadKeyAction | WriteKeyAction;

const queued: ReadonlySet<TreeKeyAction> = new Set<TreeKeyAction>([
  "move-up",
  "move-down",
  "reparent-up",
  "reparent-down",
]);
export const isQueuedAction = (a: TreeKeyAction | null): a is QueuedKeyAction =>
  a !== null && queued.has(a);

const plainKeys: Record<string, TreeKeyAction> = {
  ArrowUp: "up",
  ArrowDown: "down",
  ArrowRight: "right",
  ArrowLeft: "left",
  Enter: "open",
  ContextMenu: "menu",
  " ": "toggle-done",
};
// 文字のキーは、大文字小文字を区別しない(Caps Lock・Shift でも同じ)。Ctrl・Cmd・Alt つきは別のショートカットなので使わない。
const letterKeys: Record<string, TreeKeyAction> = {
  s: "status-menu",
  k: "change-kind",
  c: "create-sibling",
};

// treeKeyAction は、フォーカスのある行 row で押されたキーの操作を返す。ツリーのキーでなければ null。
// Epic の種類は変えられないので、Epic の K は何もしない。
export function treeKeyAction(
  event: KeyInput,
  row: Pick<Item, "kind"> | null,
): TreeKeyAction | null {
  if (!row || event.isComposing) return null;
  const { key, ctrlKey, metaKey, altKey, shiftKey } = event;
  const noModifier = !ctrlKey && !metaKey && !altKey;
  if (noModifier) {
    const action = (shiftKey ? undefined : plainKeys[key]) ?? letter(key, row);
    if (action) return action;
  }
  if (altKey && !ctrlKey && !metaKey && (key === "ArrowUp" || key === "ArrowDown")) {
    return `${shiftKey ? "reparent" : "move"}-${key === "ArrowUp" ? "up" : "down"}`;
  }
  if (key === "Enter" && (ctrlKey || metaKey) && !altKey && !shiftKey) return "open-tab";
  if (key === "F10" && shiftKey && noModifier) return "menu";
  return null;
}

// KeyTarget は、キーを受けた部品の種類。tree はツリー自身、button は行の中のボタン、other はそれ以外(タイトルのリンクなど)。
// core は DOM の型を持たないので、app が決めて渡す。
export type KeyTarget = "tree" | "button" | "other";

// leftToTarget は、行の中の部品にフォーカスがあるとき、そのキーを部品の働きに任せるか。Enter はリンクもボタンも
// 開く・押すので任せる。Space はボタンだけが押す(リンクの Space はページをスクロールするだけなので、ツリーが受ける)。
export function leftToTarget(action: TreeKeyAction, target: KeyTarget): boolean {
  if (target === "tree") return false;
  if (action === "open") return true;
  return action === "toggle-done" && target === "button";
}

function letter(key: string, row: Pick<Item, "kind">): TreeKeyAction | null {
  const action = letterKeys[key.toLowerCase()];
  return action === "change-kind" && row.kind === "Epic" ? null : (action ?? null);
}
