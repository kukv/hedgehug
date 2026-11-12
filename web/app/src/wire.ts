import {
  createCatalogPort,
  createItemsPort,
  createProjectsPort,
  createSystemPort,
  rememberToken,
  setToken,
} from "@hedgehug/api";
import {
  createCatalog,
  createNavigation,
  createProjectDialog,
  createProjectScreen,
  createScreen,
  createShell,
  createSystemInfo,
  createToasts,
  get,
  routeOfProject,
} from "@hedgehug/core";
import { browserHistory } from "./history";
import { createLinks } from "./links";

// 組み立て: api の実装を core に渡す(設計書 6 節)。画面はここで作った状態を使う。

// トークンは、ほかの何よりも先に決める(アドレスからハッシュを消してから、履歴を読む)。A2 の計画の決定 5。
const token = rememberToken(location, sessionStorage, (url) => history.replaceState(null, "", url));
setToken(token);

export const shell = createShell();
export const system = createSystemInfo(createSystemPort());
export const navigation = createNavigation(browserHistory());
navigation.start();
export const links = createLinks(navigation, token);
export const toasts = createToasts({
  set: (fn, ms) => setTimeout(fn, ms),
  clear: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
});

const itemsPort = createItemsPort();
const catalogPort = createCatalogPort();
export const catalog = createCatalog(catalogPort);
export const projectScreen = createProjectScreen(catalog, catalogPort, itemsPort);
export const projectDialog = createProjectDialog({
  catalog,
  catalogPort,
  projects: createProjectsPort(),
  // 作ったら、そのプロジェクトの概要を開く。トーストは出さない(02「作成」)。
  onCreated: (storeId, project) =>
    navigation.go(routeOfProject(get(catalog.stores), storeId, project.id, "overview")),
});
export const screen = createScreen({
  navigation,
  catalog,
  items: itemsPort,
  // 詳細パネルは Task 10 で足す。それまでアイテムの入口は、プロジェクトのツリーを開くだけ。
  openItem: () => {},
});

// 画面を移ったら、サイドバーのドロワーを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => shell.closeDrawer());

// 一覧を読んでから、アドレスの解決を始める(一覧がないと Store 名と ID を解決できない)。
void catalog.load().then(() => screen.start());
