import {
  createCatalogPort,
  createItemsPort,
  createNotesPort,
  createPlanningPort,
  createProjectsPort,
  createSyncPort,
  createSystemPort,
  createWorkspacesPort,
  rememberToken,
  setToken,
} from "@hedgehug/api";
import {
  createCatalog,
  createComments,
  createDetail,
  createItemDialog,
  createNavigation,
  createPlanning,
  createProjectDialog,
  createProjectScreen,
  createReflection,
  createRelations,
  createSaveAsTemplate,
  createScreen,
  createShell,
  createSync,
  createSystemInfo,
  createTabSignals,
  createWatch,
  createWorklogs,
  createWorkspaceDialog,
  createWorkspaceMover,
  createWorkspaceSettings,
  get,
  readonly,
  routeOfProject,
  routeOfWorkspace,
  type SaveAsTemplate,
  type SyncEventKind,
  writable,
} from "@hedgehug/core";
import { drafts, leaveItem } from "./drafts";
import { hereOf } from "./here";
import { browserHistory } from "./history";
import { createLinks } from "./links";
import { createNoteHistory } from "./noteHistory";
import { githubReloads, reloads } from "./reloads";
import { held, online as isOnline, tabId, visible } from "./tab";
import { toasts } from "./wireBase";

// 組み立て: api の実装を core に渡す(設計書 6 節)。画面はここで作った状態を使う。

// トークンは、ほかの何よりも先に決める(アドレスからハッシュを消してから、履歴を読む)。A2 の計画の決定 5。
const token = rememberToken(location, sessionStorage, (url) => history.replaceState(null, "", url));
setToken(token);

export const shell = createShell();
export const system = createSystemInfo(createSystemPort());
// drafts は書きかけの登録簿(実体は drafts.ts。tab.ts との循環を避けるため wire.ts の外に置く)。
export { drafts };
export const navigation = createNavigation(browserHistory(), drafts);
navigation.start();
export const links = createLinks(navigation, token);
export { toasts };

export const itemsPort = createItemsPort();
const catalogPort = createCatalogPort();
// 詳細パネルとプロジェクトの概要の、説明・コメントなどの口と、メンションの表示に使うメンバーの一覧。
export const notes = createNotesPort();
export const { itemHistory, projectHistory, lists, descriptionSaves } = createNoteHistory(notes);
export const listMembers = (storeId: string) => catalogPort.listMembers(storeId);
export const catalog = createCatalog(catalogPort);
const workspacesPort = createWorkspacesPort();
// ツリーでステータスを変えたら、開いている詳細パネルも読み直す。detail は後で作るので、呼ぶときに引く。
export const projectScreen = createProjectScreen(
  catalog,
  catalogPort,
  itemsPort,
  workspacesPort,
  toasts,
  () => void detail.reload(),
);
export const projectDialog = createProjectDialog({
  catalog,
  catalogPort,
  projects: createProjectsPort(),
  // 作ったら、そのプロジェクトの概要を開く。トーストは出さない(02「作成」)。
  onCreated: (storeId, project) =>
    navigation.go(routeOfProject(get(catalog.stores), storeId, project.id, "overview")),
});
export const workspaceSettings = createWorkspaceSettings({
  port: workspacesPort,
  catalog,
  toasts,
  listMembers: (s) => catalogPort.listMembers(s),
});
export const workspaceDialog = createWorkspaceDialog({
  catalog,
  port: workspacesPort,
  // 作ったら、そのワークスペースの概要を開く。トーストは出さない(03「ワークスペースの作成」)。
  onCreated: (storeId, workspace) =>
    navigation.go(routeOfWorkspace(get(catalog.stores), storeId, workspace.id, "overview")),
});
// スプリントとリリースの口と、開いているワークスペースの概要(見出しの「実行中」)。
export const planningPort = createPlanningPort();
export const planning = createPlanning(planningPort);
export const workspaceMover = createWorkspaceMover({ catalog, port: workspacesPort, toasts });
const saveAsTemplateCore = createSaveAsTemplate({ port: workspacesPort, toasts });
// 保存できたら、開いている設定の画面を読み直す(開いていなければ何もしない)。
export const saveAsTemplate: SaveAsTemplate = {
  async save(view, input, onRefused) {
    const ok = await saveAsTemplateCore.save(view, input, onRefused);
    if (ok) void workspaceSettings.reload();
    return ok;
  },
};
export const detail = createDetail({
  catalog,
  items: itemsPort,
  workspaces: workspacesPort,
  catalogPort,
  planning: planningPort,
  toasts,
  onChanged: () => void projectScreen.reload(),
  // 別のアイテムを開く・パネルを閉じるとき、アイテムの書きかけがあれば「入力を破棄」を出す(02「説明」)。
  leave: leaveItem,
});
// 詳細パネルとプロジェクトの概要は、同時に開くので、コメントの状態を別々に持つ(入力が混ざらない)。
export const itemComments = createComments({ notes, catalogPort, toasts, drafts, scope: "item" });
export const projectComments = createComments({
  notes,
  catalogPort,
  toasts,
  drafts,
  scope: "project",
});
export const worklogs = createWorklogs({ notes, toasts, now: () => new Date() });
export const relations = createRelations({
  notes,
  toasts,
  // 関連を足す・外すと「ブロック中」が変わるので、ツリーと開いている詳細パネルを読み直す。
  onChanged: () => void Promise.all([detail.reload(), projectScreen.reload()]),
});
// ウォッチを変えたら、開いている詳細パネルを読み直す(ウォッチ中の人の一覧が変わる)。
export const watch = createWatch({ notes, toasts, onChanged: () => void detail.reload() });
export const screen = createScreen({
  navigation,
  catalog,
  items: itemsPort,
  openItem: (storeId, itemId) => void detail.open(storeId, itemId),
});

export const itemDialog = createItemDialog({
  catalog,
  items: itemsPort,
  workspaces: workspacesPort,
  catalogPort,
  planning: planningPort,
  toasts,
  here: () => hereOf(screen, catalog),
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

const syncPort = createSyncPort();

// 画面への反映(06「画面への反映」)。store は一覧・開いているプロジェクト・詳細パネル、sync は同期の状態。
// 失敗は出さない(A2 の決定 29)。
// 同期の状態は、どちらの種類でも読み直す(保存で未送信の数が変わる。決定 S10)。
async function reflect(kinds: SyncEventKind[]) {
  const jobs: Promise<unknown>[] = [sync.load()];
  if (kinds.includes("store")) {
    jobs.push(
      catalog
        .load()
        .then(() =>
          Promise.all([
            projectScreen.reload(),
            detail.reload(),
            planning.reload(),
            ...[itemComments, projectComments, worklogs, relations, ...lists].map((section) =>
              section.reload(),
            ),
          ]),
        ),
    );
  }
  if (kinds.includes("store") || kinds.includes("github")) {
    jobs.push(...[...reloads].map((reload) => reload()));
  }
  if (kinds.includes("github")) {
    jobs.push(...[...githubReloads].map((reload) => reload()));
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
