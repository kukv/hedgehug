import {
  createCatalogPort,
  createItemsPort,
  createProjectsPort,
  createSystemPort,
  createWorkspacesPort,
  rememberToken,
  setToken,
} from "@hedgehug/api";
import {
  createCatalog,
  createDetail,
  createItemDialog,
  createNavigation,
  createProjectDialog,
  createProjectScreen,
  createScreen,
  createShell,
  createSystemInfo,
  createToasts,
  createWorkspaceDialog,
  createWorkspaceMover,
  createWorkspaceSettings,
  get,
  type Here,
  routeOfProject,
  routeOfWorkspace,
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
const workspacesPort = createWorkspacesPort();
export const workspaceSettings = createWorkspaceSettings({
  port: workspacesPort,
  catalog,
  toasts,
});
export const workspaceDialog = createWorkspaceDialog({
  catalog,
  port: workspacesPort,
  // 作ったら、そのワークスペースの概要を開く。トーストは出さない(03「ワークスペースの作成」)。
  onCreated: (storeId, workspace) =>
    navigation.go(routeOfWorkspace(get(catalog.stores), storeId, workspace.id, "overview")),
});
export const workspaceMover = createWorkspaceMover({ catalog, port: workspacesPort, toasts });
export const detail = createDetail({
  catalog,
  items: itemsPort,
  toasts,
  onChanged: () => void projectScreen.reload(),
});
export const screen = createScreen({
  navigation,
  catalog,
  items: itemsPort,
  openItem: (storeId, itemId) => void detail.open(storeId, itemId),
});

// here は、作成ダイアログの初期値に使う、いま開いている画面のワークスペースとプロジェクト(02)。
function here(): Here {
  const c = get(screen.current);
  if (c?.name === "workspace") return { workspaceId: c.workspaceId };
  if (c?.name === "project") {
    return {
      workspaceId: catalog.project(c.storeId, c.projectId)?.workspaceId,
      projectId: c.projectId,
    };
  }
  return {};
}

export const itemDialog = createItemDialog({
  catalog,
  items: itemsPort,
  toasts,
  here,
  openProjectDialog: (workspaceId) => void projectDialog.open(workspaceId),
  // 「続けて作成」がオフなら、作ったアイテムの詳細パネルを開く(02「作成したあと」)。
  onCreated: (item, keptOpen) => {
    void projectScreen.reload();
    if (!keptOpen) {
      void detail.open(item.storeId, item.id);
      return;
    }
    // 続けて作成: 開いている詳細パネルの子の一覧も読み直す(決定 19)。
    const v = get(detail.view);
    if (v) void detail.open(v.store.id, v.item.id);
  },
});

// 画面を移ったら、サイドバーのドロワーと詳細パネルを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => {
  shell.closeDrawer();
  detail.close();
  itemDialog.close();
  projectDialog.close();
  workspaceDialog.close();
});

// 一覧を読んでから、アドレスの解決を始める(一覧がないと Store 名と ID を解決できない)。
void catalog.load().then(() => screen.start());
