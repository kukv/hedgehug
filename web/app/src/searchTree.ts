import {
  childKindOf,
  createSearchTreeData,
  type Item,
  itemDisplayId,
  type ProjectNode,
  type Route,
  type SearchTreeRow,
  type SearchTreeView,
  type StoreNode,
  type TreeNode,
  type TreeNodeOf,
  type WorkspaceNode,
} from "@hedgehug/core";
import { catalog, itemDialog, itemsPort, listMembers, workspacesPort } from "./wire";

// 検索の画面のツリー(E3a)の組み立て。search.ts がこれを読むので、ここから search.ts を読まない。

// searchTreeData は、プロジェクトのアイテム・ワークスペースの設定・Store のメンバーを取っておく(E3a-3)。
// Store が変わった知らせ(search.ts)と、ツリーを開いたとき・ツリーから書いたあと(SearchTree.svelte)に捨てて読み直す。
export const searchTreeData = createSearchTreeData({
  catalog,
  items: itemsPort,
  workspaces: workspacesPort,
  members: listMembers,
});

// projectRoute は、プロジェクトの行の行き先(プロジェクトの概要。E3a-12)。
export const projectRoute = (store: StoreNode, project: ProjectNode): Route => ({
  name: "project",
  store: store.name,
  project: project.displayId,
  tab: "overview",
});

// itemRoute は、アイテムの行の行き先(新しいタブで開くアイテムの入口)。
export const itemRoute = (store: StoreNode, workspace: WorkspaceNode, it: Item): Route => ({
  name: "item",
  store: store.name,
  item: itemDisplayId(workspace.prefix, it.number),
});

export const routeOf = (row: SearchTreeRow): Route =>
  row.kind === "project"
    ? projectRoute(row.store, row.project)
    : itemRoute(row.store, row.workspace, row.item);

// addEpic は、プロジェクトの行の「+」と C(その下に Epic を作る作成ダイアログ)。
export function addEpic(project: ProjectNode) {
  void itemDialog.open({
    workspaceId: project.workspaceId,
    projectId: project.id,
    kind: "Epic",
    parentId: null,
  });
}

// addChild は、アイテムの行の「+」(その行を親にした作成ダイアログ)。
export function addChild(parent: Item) {
  const kind = childKindOf(parent.kind);
  if (!kind) return;
  void itemDialog.open({
    workspaceId: parent.workspaceId,
    projectId: parent.projectId,
    kind,
    parentId: parent.id,
  });
}

// createAt は、C。アイテムの行は同じ親の兄弟、プロジェクトの行はその下の Epic を作る作成ダイアログを開く。
export function createAt(row: SearchTreeRow) {
  if (row.kind === "project") return addEpic(row.project);
  const it = row.item;
  void itemDialog.open({
    workspaceId: it.workspaceId,
    projectId: it.projectId,
    kind: it.kind,
    parentId: it.parentId,
  });
}

// itemNodes は、検索のツリーのアイテムの行を、行の部品(TreeRow)が描くアイテムの木にする。
export const itemNodes = (nodes: readonly TreeNodeOf<SearchTreeRow>[]): TreeNode[] =>
  nodes.flatMap((n) =>
    n.item.kind === "item" ? [{ item: n.item.item, children: itemNodes(n.children) }] : [],
  );

// dimKeys は、薄く出す行(一致しない祖先)の鍵。
export function dimKeys(nodes: readonly TreeNodeOf<SearchTreeRow>[]): Set<string> {
  const keys = new Set<string>();
  const walk = (ns: readonly TreeNodeOf<SearchTreeRow>[]) => {
    for (const n of ns) {
      if (n.item.kind === "item" && n.item.dim) keys.add(n.item.id);
      walk(n.children);
    }
  };
  walk(nodes);
  return keys;
}

// dataIn は、描いた木のプロジェクトのデータ。書き込みの画面とメニューは、Store が変わった知らせで取っておいたものを
// 捨てているあいだも、描いている木と同じデータを使う。
export function dataIn(view: SearchTreeView | null, storeId: string, projectId: string) {
  for (const s of view?.sections ?? []) {
    if (s.store.id !== storeId) continue;
    const p = s.projects.find((x) => x.project.id === projectId);
    if (p) return p.data;
  }
  return undefined;
}
