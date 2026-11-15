// 検索のツリー(04「ツリー」の検索の画面。E3a-3〜6)。Store → ワークスペース → プロジェクトの行 → アイテムの木を組み立てる。
// 絞り込み中は、一致したアイテムと、その祖先だけを残す(一致しない祖先は薄く出す)。並びは catalog と兄弟の並びで、
// 応答の行の並び(sort:)は使わない。行の鍵は `{storeId}/{内部 ID}`(内部 ID は Store の中でしか一意と言えないため)。

import type { Item } from "../../model/item";
import type { Member } from "../../model/member";
import type { WorkspaceSettings } from "../../model/workspace";
import type { ProjectNode, StoreNode, WorkspaceNode } from "../catalog/catalog";
import { buildTree, type TreeNode, type TreeNodeOf, type TreeRowLike } from "../items/tree";
import type { TreeScreen } from "../items/treeWrites";
import type { SearchRow } from "./types";

export type SearchTreeRow =
  | {
      id: string;
      parentId: string | null;
      kind: "project";
      store: StoreNode;
      workspace: WorkspaceNode;
      project: ProjectNode;
    }
  | {
      id: string;
      parentId: string | null;
      kind: "item";
      store: StoreNode;
      workspace: WorkspaceNode;
      item: Item;
      // dim は、絞り込みに一致しない祖先(文字を薄くする)。
      dim: boolean;
    };

// ProjectData は、プロジェクトの行と木に要るもの。items は絞り込まないすべて(進捗は一致しない子も数える)。
export interface ProjectData {
  items: Item[];
  settings: WorkspaceSettings;
  members: Member[];
}

export interface SearchTreeSection {
  store: StoreNode;
  workspace: WorkspaceNode;
  // node は、プロジェクトの行と、その下。
  projects: { project: ProjectNode; data: ProjectData; node: TreeNodeOf<SearchTreeRow> }[];
}

export interface SearchTreeView {
  sections: SearchTreeSection[];
  // nodes は、フォーカスに渡す木(プロジェクトの行が根)。
  nodes: TreeNodeOf<SearchTreeRow>[];
  // matched は、木に描いた行のうち、一致したアイテムの行の数(E3a-6)。
  matched: number;
  // hasProjects は、表示中の Store にプロジェクトが 1 つでもあるか(E3a-19 の空の文の出し分け)。
  hasProjects: boolean;
}

type DataOf = (storeId: string, projectId: string) => ProjectData | undefined;

export const rowKey = (storeId: string, id: string) => `${storeId}/${id}`;

// matchKeys は、一致した行の鍵(削除済みの行は入れない)。
const matchKeys = (rows: SearchRow[]) =>
  new Set(rows.filter((r) => r.deleted === null).map((r) => rowKey(r.storeId, r.itemId)));

// treeScope は、木に入れるプロジェクト(E3a-3)。絞り込み中は一致した行があるものだけ。並びは catalog の並び(E3a-4)。
export function treeScope(
  stores: StoreNode[],
  rows: SearchRow[],
  filtered: boolean,
): { storeId: string; projectId: string }[] {
  const hit = new Set(
    rows.filter((r) => r.deleted === null).map((r) => rowKey(r.storeId, r.projectId)),
  );
  return stores.flatMap((s) =>
    s.workspaces.flatMap((w) =>
      w.projects
        .filter((p) => !filtered || hit.has(rowKey(s.id, p.id)))
        .map((p) => ({ storeId: s.id, projectId: p.id })),
    ),
  );
}

// grow は、アイテムの木を検索のツリーの行にする。絞り込み中は、一致したものと、一致を子孫に持つものだけを残す。
function grow(
  nodes: TreeNode[],
  parentKey: string,
  place: { store: StoreNode; workspace: WorkspaceNode },
  matches: ReadonlySet<string>,
  filtered: boolean,
): TreeNodeOf<SearchTreeRow>[] {
  return nodes.flatMap((n) => {
    const key = rowKey(place.store.id, n.item.id);
    const children = grow(n.children, key, place, matches, filtered);
    const hit = matches.has(key);
    if (filtered && !hit && children.length === 0) return [];
    const row: SearchTreeRow = {
      id: key,
      parentId: parentKey,
      kind: "item",
      ...place,
      item: n.item,
      dim: filtered && !hit,
    };
    return [{ item: row, children }];
  });
}

const countMatched = (nodes: TreeNodeOf<SearchTreeRow>[], matches: ReadonlySet<string>): number =>
  nodes.reduce(
    (sum, n) =>
      sum +
      (n.item.kind === "item" && matches.has(n.item.id) ? 1 : 0) +
      countMatched(n.children, matches),
    0,
  );

// buildSearchTree は、検索の画面のツリーを組み立てる。データのないプロジェクトは出さない。絞り込み中に、一致した行が
// 木に入らなかった(親が見つからない)プロジェクトも出さない。
export function buildSearchTree(
  stores: StoreNode[],
  rows: SearchRow[],
  filtered: boolean,
  data: DataOf,
): SearchTreeView {
  const matches = matchKeys(rows);
  const scope = new Set(
    treeScope(stores, rows, filtered).map((p) => rowKey(p.storeId, p.projectId)),
  );
  const sections: SearchTreeSection[] = [];
  for (const store of stores) {
    for (const workspace of store.workspaces) {
      const projects: SearchTreeSection["projects"] = [];
      for (const project of workspace.projects) {
        const key = rowKey(store.id, project.id);
        const d = scope.has(key) ? data(store.id, project.id) : undefined;
        if (!d) continue;
        const children = grow(buildTree(d.items), key, { store, workspace }, matches, filtered);
        if (filtered && children.length === 0) continue;
        const row: SearchTreeRow = {
          id: key,
          parentId: null,
          kind: "project",
          store,
          workspace,
          project,
        };
        projects.push({ project, data: d, node: { item: row, children } });
      }
      if (projects.length > 0) sections.push({ store, workspace, projects });
    }
  }
  const nodes = sections.flatMap((s) => s.projects.map((p) => p.node));
  return {
    sections,
    nodes,
    matched: countMatched(nodes, matches),
    hasProjects: stores.some((s) => s.workspaces.some((w) => w.projects.length > 0)),
  };
}

// screenFor は、フォーカスのある行(鍵 key)の書き込みの画面(E3a-17、Review Focus 2・3)。rows は見えている行。
// 同じ Store・ワークスペースの見えている行だけを使い、閲覧のみと設定は、その行の Store とプロジェクトで決める。
export function screenFor(
  rows: readonly SearchTreeRow[],
  key: string,
  data: DataOf,
): TreeScreen | null {
  const at = rows.find((r) => r.id === key);
  if (!at) return null;
  const projectId = at.kind === "project" ? at.project.id : at.item.projectId;
  const d = data(at.store.id, projectId);
  if (!d) return null;
  const same = rows.filter((r) => r.store.id === at.store.id && r.workspace.id === at.workspace.id);
  const order: TreeRowLike[] = same.map((r) =>
    r.kind === "project"
      ? { id: r.project.id, parentId: null }
      : { id: r.item.id, parentId: r.item.parentId },
  );
  return {
    storeId: at.store.id,
    readOnly: at.store.readOnly,
    settings: d.settings,
    rows: same.flatMap((r) => (r.kind === "item" ? [r.item] : [])),
    order,
  };
}
