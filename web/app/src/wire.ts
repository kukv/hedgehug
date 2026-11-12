import {
  createCatalogPort,
  createItemsPort,
  createProjectsPort,
  createSyncPort,
  createSystemPort,
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
  createReflection,
  createScreen,
  createShell,
  createSync,
  createSystemInfo,
  createTabSignals,
  createToasts,
  get,
  type Here,
  readonly,
  routeOfProject,
  type SyncEventKind,
  writable,
} from "@hedgehug/core";
import { browserHistory } from "./history";
import { createLinks } from "./links";
import { held, online as isOnline, tabId, visible } from "./tab";

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
});

const syncPort = createSyncPort();

// 画面への反映(06「画面への反映」)。store は一覧・開いているプロジェクト・詳細パネル、sync は同期の状態。
// 失敗は出さない(A2 の決定 29)。
// 同期の状態は、どちらの種類でも読み直す(保存で未送信の数が変わる。決定 S10)。
async function reflect(kinds: SyncEventKind[]) {
  const jobs: Promise<unknown>[] = [sync.load()];
  if (kinds.includes("store")) {
    jobs.push(catalog.load().then(() => Promise.all([projectScreen.reload(), detail.reload()])));
  }
  await Promise.allSettled(jobs);
}

export const reflection = createReflection({
  port: syncPort,
  timer: {
    set: (fn, ms) => setTimeout(fn, ms),
    clear: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
  },
  held,
  visible,
  apply: reflect,
});

export const sync = createSync({
  port: syncPort,
  toasts,
  online: isOnline,
  onSynced: () => void reflection.now(),
});

const signals = createTabSignals({
  port: syncPort,
  tab: tabId,
  held,
  visible,
  online: isOnline,
  now: () => Date.now(),
});

// syncNow は、相対時間と 24 時間の判定に使う時刻。30 秒ごとに進める(決定 S7)。
const now = writable(new Date());
export const syncNow = readonly(now);
const onlineState = writable(isOnline());
export const online = readonly(onlineState);

// started は、startSync のあとか。つながり方の状態を取りこぼさないよう、リスナーはモジュールを
// 読んだときに付ける。知らせと反映は、startSync のあとだけ行う(つないだときの読み直しが最初の解決より先に走らないように)。
let started = false;
const network = () => {
  onlineState.set(isOnline());
  if (started) {
    signals.check();
    reflection.wake();
  }
};
window.addEventListener("online", network);
window.addEventListener("offline", network);

// startSync は、出来事の受け取り・知らせ・時刻の更新を始める。最初の一覧を読んだあとに 1 回呼ぶ。
function startSync() {
  started = true;
  reflection.start();
  void sync.load();
  const recheck = () => {
    signals.check();
    reflection.wake();
  };
  setInterval(() => signals.check(), 1000);
  setInterval(() => now.set(new Date()), 30_000);
  document.addEventListener("visibilitychange", recheck);
  document.addEventListener("focusout", () => setTimeout(recheck, 0));
  document.addEventListener("focusin", () => signals.check());
}

// 一覧を読んでから、アドレスの解決と同期(出来事・知らせ)を始める(一覧がないと Store 名と ID を解決できない)。
void catalog.load().then(() => {
  screen.start();
  startSync();
});
