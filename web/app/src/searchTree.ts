import {
  childKindOf,
  createSearchTreeData,
  type Item,
  itemDisplayId,
  menuAt,
  type ProjectNode,
  type Route,
  type SearchTreeData,
  type SearchTreeRow,
  type SearchTreeView,
  type StoreNode,
  screenFor,
  type TreeNode,
  type TreeNodeOf,
  type WorkspaceNode,
} from "@hedgehug/core";
import { openProjectMenu } from "./projectRows";
import { catalog, itemDialog, itemsPort, listMembers, workspacesPort } from "./wire";
import { itemMenuHost } from "./wireActions";

// 検索の画面のツリー(E3a)の組み立て。search.ts がこれを読むので、ここから search.ts を読まない。

const data = createSearchTreeData({
  catalog,
  items: itemsPort,
  workspaces: workspacesPort,
  members: listMembers,
});
// resets は、捨てた回数。
let resets = 0;

// searchTreeData は、プロジェクトのアイテム・ワークスペースの設定・Store のメンバーを取っておく(E3a-3)。
// Store が変わった知らせ(search.ts)と、ツリーを開いたとき・ツリーから書いたあと(SearchTree.svelte)に捨てて読み直す。
// load は、読んでいるあいだに捨てられたら読み直す(捨てる前に始めた読み込みは結果を入れないので、そのまま組むと
// 木が空になる)。解けたときは、scope のうち読めたものがすべて入っている。
export const searchTreeData: SearchTreeData = {
  get: data.get,
  async load(scope) {
    let at: number;
    do {
      at = resets;
      await data.load(scope);
    } while (at !== resets);
  },
  reset() {
    resets += 1;
    data.reset();
  },
};

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

// treeScreen は、フォーカスのある行の書き込みの画面(キー操作の screen)。focus は見えている行といまの行。
export function treeScreen(
  focus: { rows: readonly SearchTreeRow[]; currentId: string | null },
  view: SearchTreeView | null,
) {
  const dataOf = (storeId: string, projectId: string) => dataIn(view, storeId, projectId);
  return focus.currentId ? screenFor(focus.rows, focus.currentId, dataOf) : null;
}

// openTreeMenu は、行のメニュー(行の「⋯」とメニューキー)。visible は見えている行、view は描いている木。
// アイテムの行の見えている兄弟は、同じ Store・ワークスペースの見えている行から決める。
export function openTreeMenu(
  row: SearchTreeRow,
  rect: { left: number; bottom: number },
  visible: readonly SearchTreeRow[],
  view: SearchTreeView | null,
) {
  if (row.kind === "project") return openProjectMenu(row.store, row.workspace, row.project, rect);
  const { x, y } = menuAt(rect);
  const dataOf = (storeId: string, projectId: string) => dataIn(view, storeId, projectId);
  const rows = screenFor(visible, row.id, dataOf)?.rows ?? [];
  const items = dataOf(row.store.id, row.item.projectId)?.items ?? [];
  const children = items.filter((c) => c.parentId === row.item.id);
  itemMenuHost.open(row.item, { rows, children, row: true }, x, y);
}
