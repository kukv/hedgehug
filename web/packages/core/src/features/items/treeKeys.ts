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
export type TreeKeyAction = ReadKeyAction;

const plainKeys: Record<string, TreeKeyAction> = {
  ArrowUp: "up",
  ArrowDown: "down",
  ArrowRight: "right",
  ArrowLeft: "left",
  Enter: "open",
  ContextMenu: "menu",
};

// treeKeyAction は、フォーカスのある行 row で押されたキーの操作を返す。ツリーのキーでなければ null。
export function treeKeyAction(
  event: KeyInput,
  row: Pick<Item, "kind"> | null,
): TreeKeyAction | null {
  if (!row || event.isComposing) return null;
  const { key, ctrlKey, metaKey, altKey, shiftKey } = event;
  if (!ctrlKey && !metaKey && !altKey && !shiftKey) return plainKeys[key] ?? null;
  if (key === "Enter" && (ctrlKey || metaKey) && !altKey && !shiftKey) return "open-tab";
  if (key === "F10" && shiftKey && !ctrlKey && !metaKey && !altKey) return "menu";
  return null;
}
