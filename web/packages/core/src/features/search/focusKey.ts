// focusesFilter は、/ キーで絞り込み欄に移るかどうか(01 キーボード)。true なら、呼び出し側が欄へフォーカスを移し、
// / を入力しない。入力中とダイアログを開いているとき、欄のない画面では使えない。
export function focusesFilter(e: {
  key: string;
  ctrl: boolean;
  meta: boolean;
  alt: boolean;
  typing: boolean;
  dialogOpen: boolean;
  hasField: boolean;
}): boolean {
  if (e.key !== "/" || e.ctrl || e.meta || e.alt) return false;
  return !e.typing && !e.dialogOpen && e.hasField;
}
