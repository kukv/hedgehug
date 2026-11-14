import type { MovedFrom } from "../../model/transfer";
import type { ItemsPort } from "../../port/items";
import type { TransfersPort } from "../../port/transfers";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import type { Navigation } from "../router/navigation";
import { type Resolved, resolveRoute, routeOfProject, routeOfWorkspace } from "../router/resolve";
import type { ProjectTab } from "../router/route";
import type { Toasts } from "../toasts/toasts";
import { goneText, movedLine } from "../transfers/movedText";

// いま出す画面。アドレスを一覧の状態で内部 ID にする(A2 の計画の決定 7)。
// アイテムの入口は、そのアイテムのプロジェクトの「ツリー」に替え(履歴に積まない)、詳細パネルを開く(01)。
// 旧 ID で開けないもの(削除済み・完全に削除・見えない Store)は、理由をトーストで出してホームに替える(02)。
// プロジェクトの旧 ID で開いたときは、アドレスを移したプロジェクトの同じタブに替え(履歴に積まない)、移動の印を
// projectMoved に置く(A5b-2・A5b-24)。

// ProjectMoved は、旧 ID から開いたプロジェクトの画面と、その移動の印。
export interface ProjectMoved {
  storeId: string;
  projectId: string;
  tab: ProjectTab;
  moved: MovedFrom;
  // text は、見出しの下に出す 1 行(movedLine。時刻は deps.now() で、開いたときに作る)。
  text: string;
}

export interface Screen {
  // current は、出す画面。アドレスを解決しているあいだは、前の画面のまま。最初の解決までは null。
  current: Readable<Resolved | null>;
  // projectMoved は、旧 ID から開いたプロジェクトの移動の印。そのあと出す画面が別のプロジェクトか別のタブになったら null。
  projectMoved: Readable<ProjectMoved | null>;
  // start は、アドレスと一覧の変化を受け始め、止める関数を返す。一覧を読んだあとで呼ぶ。
  start(): () => void;
}

export interface ScreenDeps {
  navigation: Navigation;
  catalog: Catalog;
  items: ItemsPort;
  transfers: Pick<TransfersPort, "locate" | "locateProject">;
  toasts: Toasts;
  now(): Date;
  // openItem は、アイテムの入口で開いたアイテムの詳細パネルを開く。旧 ID から開いたときは movedFrom を添える。
  openItem(storeId: string, itemId: string, movedFrom?: MovedFrom): void;
}

// ShownProject は、いま出しているプロジェクトの画面。store・project は、そのアドレスの Store 名と ID(アドレスの解決は
// 別の Store へも広がるので、解決した結果の storeId・projectId とは別に持つ)。workspaceId は、消えたときの移り先。
interface ShownProject {
  store: string;
  project: string;
  storeId: string;
  projectId: string;
  workspaceId: string;
}

function sameScreen(a: Resolved | null, b: Resolved): boolean {
  return a !== null && JSON.stringify(a) === JSON.stringify(b);
}

export function createScreen(deps: ScreenDeps): Screen {
  const current = writable<Resolved | null>(null);
  const projectMoved = writable<ProjectMoved | null>(null);
  // 解決が重なったときは、始めた順に結果を使い、後から始めたものをもう使っていれば、前のものは捨てる。アドレスが
  // 変わっていたら捨てる。最後に始めたものだけを使うと、一覧の読み直し(画面への反映)が解決より短い間隔で続いたとき、
  // いつまでも画面が出ない。
  let latest = 0;
  let applied = 0;
  // shown は、前の解決で出したプロジェクトの画面。同じアドレスがそのプロジェクトに解決しなくなったら、消えたと見る。
  let shown: ShownProject | null = null;

  async function resolve() {
    latest += 1;
    const mine = latest;
    const stores = get(deps.catalog.stores);
    const route = get(deps.navigation.route);
    // 出していたプロジェクトのアドレスを読み直すときは、サーバーの locateProject に聞かない。ほかの書き手が移した
    // ときも、移った先を開かずにワークスペースの概要へ移る(A5b-19。A5a-18 と同じ)。
    const reread =
      shown !== null &&
      route.name === "project" &&
      route.store === shown.store &&
      route.project === shown.project;
    const r = await resolveRoute(route, stores, deps.items, deps.transfers, !reread);
    if (mine < applied || get(deps.navigation.route) !== route) return;
    applied = mine;
    // 置き換えたホームの解決は gone を持たないので、同じトーストを 2 度出さない。
    if (r.name === "home" && r.gone) {
      deps.toasts.show(goneText(r.gone, deps.now()));
      return deps.navigation.replace({ name: "home" });
    }
    // 出していたプロジェクトが、同期・別のタブ・別のプログラムで削除されたときは、ワークスペースの概要へ移る
    // (A4b-22。履歴には積まない。ワークスペースもなければホーム)。アドレスを打ち込んだだけの解決できないものはホームのまま。
    // 別の Store の同じ ID に解決された場合も、消えたものとして移る。
    const was = shown;
    shown = null;
    if (
      was &&
      route.name === "project" &&
      route.store === was.store &&
      route.project === was.project &&
      !(r.name === "project" && r.storeId === was.storeId && r.projectId === was.projectId)
    )
      return deps.navigation.replace(routeOfWorkspace(stores, was.storeId, was.workspaceId));
    if (r.name === "project" && route.name === "project") {
      const workspaceId = deps.catalog.project(r.storeId, r.projectId)?.workspaceId ?? "";
      shown = {
        store: route.store,
        project: route.project,
        storeId: r.storeId,
        projectId: r.projectId,
        workspaceId,
      };
    }
    if (r.name === "project" && r.movedFrom) {
      const { movedFrom: moved, storeId, projectId, tab } = r;
      projectMoved.set({ storeId, projectId, tab, moved, text: movedLine(moved, deps.now()) });
      // 一覧がまだ古くても開けるよう、アドレスはサーバーが返したいまの ID で作る。
      const s = stores.find((x) => x.id === storeId);
      return deps.navigation.replace(
        s ? { name: "project", store: s.name, project: moved.current, tab } : { name: "home" },
      );
    }
    // サーバーがほかの Store で見つけたとき(Store の名前を変えたあとのブックマークなど)は、アドレスを見つけた Store の
    // 名前の形に替える(履歴に積まない)。替えないと、次の一覧の読み直しで、出していたプロジェクトが消えたと見られる。
    // 一覧にまだないとき(移した直後)は替えない。
    if (r.name === "project" && route.name === "project") {
      const to = routeOfProject(stores, r.storeId, r.projectId, r.tab);
      if (to.name === "project" && to.store !== route.store) return deps.navigation.replace(to);
    }
    if (r.name === "item") {
      deps.navigation.replace(routeOfProject(stores, r.storeId, r.projectId, "tree"));
      deps.openItem(r.storeId, r.itemId, r.movedFrom);
      return;
    }
    const m = get(projectMoved);
    if (
      m &&
      !(
        r.name === "project" &&
        r.storeId === m.storeId &&
        r.projectId === m.projectId &&
        r.tab === m.tab
      )
    )
      projectMoved.set(null);
    // 前と同じ画面なら出し直さない(出し直すと、画面が作り直されて入力中の文字やスクロールが消える)。
    if (!sameScreen(get(current), r)) current.set(r);
  }

  return {
    current: readonly(current),
    projectMoved: readonly(projectMoved),
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
