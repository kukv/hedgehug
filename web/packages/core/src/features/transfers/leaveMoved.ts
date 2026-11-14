import type { ProjectTransferResult } from "../../model/transfer";
import { get } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import type { Navigation } from "../router/navigation";

export interface LeaveMovedDeps {
  // route は、画面のアドレス。変わった回数だけを見る。
  navigation: Pick<Navigation, "route" | "replace">;
  catalog: Pick<Catalog, "stores" | "load">;
}

// LeaveMoved は、「移動」を押したときに呼び、移せたら返った関数に結果を渡す(createAfterMoved と同じ形)。
export type LeaveMoved = () => (result: ProjectTransferResult) => Promise<void>;

// createLeaveMoved は、自分でプロジェクトを移せたあとの移り先(A5b-19)。移したプロジェクトの新しいアドレスの
// 「設定」タブへ移り(履歴には積まない)、そのあとでサイドバーの一覧を読み直す。先に読み直すと、出していた
// プロジェクトが消えたと見られてワークスペースの概要へ移ってしまう。読み直す前の一覧には移したプロジェクトが
// ないので、アドレスは移動の結果の新しい ID から作る(読み直すまでは locateProject のいまの ID で解決する)。
// 「移動」を押してから送り終わるまでに画面を移っていたら、移らずに読み直すだけにする(A5a-18 と同じ)。
// 読み直しの失敗は、移動はできているので投げない(次の同期の読み直しで直る)。
export function createLeaveMoved(deps: LeaveMovedDeps): LeaveMoved {
  let routes = 0;
  deps.navigation.route.subscribe(() => {
    routes += 1;
  });
  return () => {
    const at = routes;
    return async (result) => {
      if (at === routes) {
        const store = get(deps.catalog.stores).find((s) => s.id === result.toStore);
        deps.navigation.replace(
          store
            ? { name: "project", store: store.name, project: result.display, tab: "settings" }
            : { name: "home" },
        );
      }
      await deps.catalog.load().catch(() => {});
    };
  };
}
