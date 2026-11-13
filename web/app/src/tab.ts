// このタブの状態を、core に関数として渡す(core は DOM を知らない)。
import { drafts } from "./drafts";

// holdReasons は、自動の同期と画面への反映を待つ条件(06「自動で同期する」。C2 画面の決定 S9)。
// ドラッグは、それを作るトラックがここに足す。
export const holdReasons: (() => boolean)[] = [
  () => document.querySelector('[aria-modal="true"]') !== null,
  () => document.querySelector('[role="menu"]') !== null,
  () =>
    document.activeElement?.closest(
      "input:not([type='checkbox']):not([type='radio']), textarea, select, [contenteditable='true']",
    ) != null,
  // 説明・コメントの書きかけ(保存していない入力が空でないもの)。フォーカスが外れても待つ(A4a-21)。
  () => drafts.anyHeld(),
];

export const held = () => holdReasons.some((r) => r());
export const visible = () => document.visibilityState === "visible";
export const online = () => navigator.onLine;
export const isMac = () => /Mac/.test(navigator.platform || navigator.userAgent);
export const tabId = crypto.randomUUID();
