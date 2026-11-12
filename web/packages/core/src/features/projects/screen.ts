import { Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { Member } from "../../model/member";
import type { WorkspaceSettings } from "../../model/workspace";
import type { CatalogPort } from "../../port/catalog";
import type { ItemsPort } from "../../port/items";
import type { WorkspacesPort } from "../../port/workspaces";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog, ProjectNode, StoreNode, WorkspaceNode } from "../catalog/catalog";
import { buildTree, compareSiblings, type TreeNode } from "../items/tree";
import type { Toasts } from "../toasts/toasts";

// プロジェクトの画面の状態(02「画面の見出しと構成」)。アイテムを作った・変えたあとは reload で読み直す(A2 の計画の決定 19)。

export interface ProjectView {
  store: StoreNode;
  workspace: WorkspaceNode;
  project: ProjectNode;
  items: Item[];
  tree: TreeNode[];
  // epics は、概要の「Epic」の区画の行(並び順)。
  epics: Item[];
  members: Member[];
  settings: WorkspaceSettings;
}

export interface ProjectScreen {
  view: Readable<ProjectView | null>;
  // load は、プロジェクトを読む。一覧にないプロジェクトなら null にする。
  load(storeId: string, projectId: string): Promise<void>;
  // reload は、いま出しているプロジェクトを読み直す。出していなければ何もしない。
  reload(): Promise<void>;
  // setStatus は、行のステータスを変えて読み直す。断られたらトーストに出す。
  setStatus(itemId: string, statusId: string): Promise<void>;
}

// ownerName は、オーナーの表示(01: 空は「なし」、メンバーにいない人は「不明なユーザー」)。
export function ownerName(login: string, members: Member[]): string {
  if (login === "") return "なし";
  return (
    members.find((m) => m.login.toLowerCase() === login.toLowerCase())?.name ?? "不明なユーザー"
  );
}

export function createProjectScreen(
  catalog: Catalog,
  catalogPort: CatalogPort,
  items: ItemsPort,
  workspaces: WorkspacesPort,
  toasts: Toasts,
  // onChanged は、ステータスを変えたあと(開いている詳細パネルを読み直す)。
  onChanged: () => void = () => {},
): ProjectScreen {
  const view = writable<ProjectView | null>(null);
  let shown: { storeId: string; projectId: string } | null = null;
  let latest = 0;

  async function load(storeId: string, projectId: string) {
    shown = { storeId, projectId };
    latest += 1;
    const mine = latest;
    const store = catalog.store(storeId);
    const project = catalog.project(storeId, projectId);
    const workspace = project && catalog.workspace(storeId, project.workspaceId);
    if (!store || !project || !workspace) {
      view.set(null);
      return;
    }
    const [list, members, ws] = await Promise.all([
      items.list(storeId, projectId),
      catalogPort.listMembers(storeId),
      workspaces.get(storeId, workspace.id),
    ]);
    if (mine !== latest) return;
    const epics = list.filter((it) => it.parentId === null).sort(compareSiblings);
    view.set({
      store,
      workspace,
      project,
      items: list,
      tree: buildTree(list),
      epics,
      members,
      settings: ws.settings,
    });
  }

  return {
    view: readonly(view),
    load,
    async reload() {
      if (shown) await load(shown.storeId, shown.projectId);
    },
    async setStatus(itemId, statusId) {
      if (!shown) return;
      try {
        await items.setProperty(shown.storeId, itemId, "status", statusId);
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        toasts.show(e.message, { error: true });
        return;
      }
      await load(shown.storeId, shown.projectId);
      onChanged();
    },
  };
}
