// pressWhileOpen は、メニューのボタンを押した時点でメニューが開いていたかを、押している間だけ覚える。
// メニューは外(ボタンも含む)を押すと閉じるので、続くクリックで開き直さないために使う。
// ボタンの外で指を離したときも、離した直後に忘れる。
export function pressWhileOpen(wasOpen: boolean, remember: (open: boolean) => void): void {
  remember(wasOpen);
  const forget = () => setTimeout(() => remember(false), 0);
  window.addEventListener("pointerup", forget, { once: true, capture: true });
  window.addEventListener("pointercancel", forget, { once: true, capture: true });
}
