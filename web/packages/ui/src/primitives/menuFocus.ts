// メニューのフォーカス(04「ツリーのキー操作」の「メニューを開いているあいだは、キーをメニューが受ける」)。
// 開いたら先頭の押せる項目にフォーカスを移し、↑↓・Home・End で項目を移り、閉じたら開く前のフォーカスに戻す。
// メニューの中の入力欄(検索欄)がすでにフォーカスを持っていれば奪わない。閉じたあとにフォーカスが別の部品
// (開いたダイアログなど)にあるときも戻さない。
const enabledItems = (node: HTMLElement) =>
  [...node.querySelectorAll<HTMLElement>("[role^='menuitem']")].filter(
    (el) => !(el as HTMLButtonElement).disabled,
  );

// target は、押したキーで移る先の項目の位置。メニューのキーでなければ null。
function target(key: string, at: number, count: number): number | null {
  // Home・End は、項目にフォーカスがあるときだけ(検索欄の中では、キャレットを動かすキー)。
  if (key === "Home") return at < 0 ? null : 0;
  if (key === "End") return at < 0 ? null : count - 1;
  const dir = key === "ArrowDown" ? 1 : key === "ArrowUp" ? -1 : 0;
  if (dir === 0) return null;
  // 項目にフォーカスがないとき(検索欄)は、↓なら先頭、↑なら末尾。先頭・末尾から先は回り込む。
  if (at < 0) return dir === 1 ? 0 : count - 1;
  return (at + dir + count) % count;
}

// origins は、開いているメニューごとの、閉じたときにフォーカスを戻す先。
const origins = new WeakMap<Element, Element | null>();

// originOf は、メニューを開いたときのフォーカス el から、閉じたときに戻す先を決める。メニューの項目から別のメニューを
// 開いたとき(アイテムのメニューの「移動…」など)は、その項目はすぐ消えるので、最初のメニューの戻す先を引き継ぐ。
function originOf(el: Element | null): Element | null {
  for (let at = el; at; at = at.parentElement) {
    const origin = origins.get(at);
    if (origin !== undefined) return origin;
  }
  return el;
}

// shownFrom は、メニューを出す前のフォーカス(Menu が読んで渡す)。head の欄は出たときにフォーカスを取り、そのあとで
// ここが動くので、そのときのフォーカスは開いた元ではない。渡されなければ、いまのフォーカスを開いた元とする。
export function menuFocus(node: HTMLElement, shownFrom?: Element | null) {
  const before = originOf(shownFrom === undefined ? document.activeElement : shownFrom);
  origins.set(node, before);
  queueMicrotask(() => {
    if (node.contains(document.activeElement)) return;
    enabledItems(node)[0]?.focus();
  });

  function onkeydown(event: KeyboardEvent) {
    if (event.altKey || event.ctrlKey || event.metaKey) return;
    const items = enabledItems(node);
    const to = target(
      event.key,
      items.indexOf(document.activeElement as HTMLElement),
      items.length,
    );
    if (to === null || items.length === 0) return;
    event.preventDefault();
    event.stopPropagation();
    items[to]?.focus();
  }
  node.addEventListener("keydown", onkeydown);

  return {
    destroy() {
      node.removeEventListener("keydown", onkeydown);
      const active = document.activeElement;
      const lost = active === null || active === document.body || node.contains(active);
      if (lost && before instanceof HTMLElement && before.isConnected) before.focus();
    },
  };
}
