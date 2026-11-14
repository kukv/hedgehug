import type { ProjectTransferResult, WorkspaceTransferResult } from "../../model/transfer";
import { get } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import type { Navigation } from "../router/navigation";
import type { Route } from "../router/route";

export interface LeaveMovedDeps {
  // route は、画面のアドレス。変わった回数だけを見る。
  navigation: Pick<Navigation, "route" | "replace">;
  catalog: Pick<Catalog, "stores" | "load">;
}

// LeaveMoved は、「移動」を押したときに呼び、移せたら返った関数に結果を渡す(createAfterMoved と同じ形)。
export type LeaveMoved = () => (result: ProjectTransferResult) => Promise<void>;
export type LeaveWorkspaceMoved = () => (result: WorkspaceTransferResult) => Promise<void>;

// leaving は、「移動」を押したときからアドレスが変わっていなければ to のアドレスへ移り(履歴には積まない)、
// そのあとでサイドバーの一覧を読み直す形(プロジェクトとワークスペースで同じ)。
// 先に読み直すと、出していたものが消えたと見られてほかの画面へ移ってしまう。
// 読み直しの失敗は、移動はできているので投げない(次の同期の読み直しで直る)。
function leaving<R>(
  deps: LeaveMovedDeps,
  to: (result: R) => Route,
): () => (result: R) => Promise<void> {
  let routes = 0;
  deps.navigation.route.subscribe(() => {
    routes += 1;
  });
  return () => {
    const at = routes;
    return async (result) => {
      if (at === routes) deps.navigation.replace(to(result));
      await deps.catalog.load().catch(() => {});
    };
  };
}

// createLeaveMoved は、自分でプロジェクトを移せたあとの移り先(A5b-19)。移したプロジェクトの新しいアドレスの
// 「設定」タブへ移る。読み直す前の一覧には移したプロジェクトがないので、アドレスは移動の結果の新しい ID から作る
// (読み直すまでは locateProject のいまの ID で解決する)。「移動」を押してから送り終わるまでに画面を移っていたら、
// 移らずに読み直すだけにする(A5a-18 と同じ)。
export function createLeaveMoved(deps: LeaveMovedDeps): LeaveMoved {
  return leaving(deps, (result: ProjectTransferResult) => {
    const store = get(deps.catalog.stores).find((s) => s.id === result.toStore);
    return store
      ? { name: "project", store: store.name, project: result.display, tab: "settings" }
      : { name: "home" };
  });
}

// createLeaveWorkspaceMoved は、自分でワークスペースを移せたあとの移り先(B5c-17)。移動先の Store のワークスペースの
// 「設定」タブへ移る。アドレスは、読み直す前の一覧にない移動先の Store 名と接頭辞を、移動の結果から取って作る。
export function createLeaveWorkspaceMoved(deps: LeaveMovedDeps): LeaveWorkspaceMoved {
  return leaving(deps, (result: WorkspaceTransferResult) => ({
    name: "workspace",
    store: result.toStoreName,
    prefix: result.prefix,
    tab: "settings",
  }));
}
