import type { ItemsPort } from "../../port/items";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import type { Navigation } from "../router/navigation";
import { type Resolved, resolveRoute, routeOfProject } from "../router/resolve";

// いま出す画面。アドレスを一覧の状態で内部 ID にする(A2 の計画の決定 7)。
// アイテムの入口は、そのアイテムのプロジェクトの「ツリー」に替え(履歴に積まない)、詳細パネルを開く(01)。

export interface Screen {
  // current は、出す画面。アドレスを解決しているあいだは、前の画面のまま。最初の解決までは null。
  current: Readable<Resolved | null>;
  // start は、アドレスと一覧の変化を受け始め、止める関数を返す。一覧を読んだあとで呼ぶ。
  start(): () => void;
}

export interface ScreenDeps {
  navigation: Navigation;
  catalog: Catalog;
  items: ItemsPort;
  // openItem は、アイテムの入口で開いたアイテムの詳細パネルを開く。
  openItem(storeId: string, itemId: string): void;
}

function sameScreen(a: Resolved | null, b: Resolved): boolean {
  return a !== null && JSON.stringify(a) === JSON.stringify(b);
}

export function createScreen(deps: ScreenDeps): Screen {
  const current = writable<Resolved | null>(null);
  // 解決が重なったときは、最後に始めたものの結果だけを使う。
  let latest = 0;

  async function resolve() {
    latest += 1;
    const mine = latest;
    const stores = get(deps.catalog.stores);
    const r = await resolveRoute(get(deps.navigation.route), stores, deps.items);
    if (mine !== latest) return;
    if (r.name === "item") {
      deps.navigation.replace(routeOfProject(stores, r.storeId, r.projectId, "tree"));
      deps.openItem(r.storeId, r.itemId);
      return;
    }
    // 前と同じ画面なら出し直さない(出し直すと、画面が作り直されて入力中の文字やスクロールが消える)。
    if (!sameScreen(get(current), r)) current.set(r);
  }

  return {
    current: readonly(current),
    start() {
      // どちらの subscribe も、いまの値ですぐに 1 回呼ぶ。2 回目の解決が 1 回目に勝つので、結果は同じ。
      const stopRoute = deps.navigation.route.subscribe(() => void resolve());
      const stopCatalog = deps.catalog.stores.subscribe(() => void resolve());
      return () => {
        stopRoute();
        stopCatalog();
      };
    },
  };
}
