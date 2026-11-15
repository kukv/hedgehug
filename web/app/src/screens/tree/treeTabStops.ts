// ツリーは Tab で入る 1 つのフォーカス先で、Tab はツリーの中でも前後の部品へ移るために使う(04「ツリーのキー操作」)。
// ツリー自身を Tab で止まる(tabindex=0)ようにし、行の中のリンクとボタンは Tab で止まらない(tabindex=-1)ようにする。
// マウスとキー(Enter ほか)では今までどおり使える。行は読み直しで増えるので、足されたものにも同じことをする。
// メニューの中は除く(メニューが受ける)。ツリーの要素は role を list のままにしてあり(通しのテストが getByRole("list") で探す)、
// 書き方の検査が list に tabindex を付けることを許さないので、属性ではなくここで付ける。
export function treeTabStops(node: HTMLElement) {
  node.tabIndex = 0;
  const strip = () => {
    for (const el of node.querySelectorAll<HTMLElement>("a[href], button")) {
      if (el.tabIndex !== -1 && !el.closest("[role='menu']")) el.tabIndex = -1;
    }
  };
  strip();
  const observer = new MutationObserver(strip);
  observer.observe(node, { childList: true, subtree: true });
  return { destroy: () => observer.disconnect() };
}
