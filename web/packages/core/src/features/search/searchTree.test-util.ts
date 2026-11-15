// 検索のツリー(searchTree.ts)のテストの材料。Store・ワークスペース・プロジェクトの並びと、一致した行を作る。
import type { Item, Kind } from "../../model/item";
import type { StoreNode } from "../catalog/catalog";
import { project } from "../catalog/fixtures.test-util";
import { item } from "../items/fixtures.test-util";
import { settings } from "../items/settings.test-util";
import { result } from "./fixtures.test-util";
import type { ProjectData } from "./searchTree";
import type { SearchRow } from "./types";

// store は、ワークスペース W1(プロジェクト n 個)を 1 つ持つ Store。
export function store(id: string, readOnly: boolean, projects = 2): StoreNode {
  const w = `${id}-W1`;
  return {
    id,
    name: id,
    color: "teal",
    readOnly,
    workspaces: [
      {
        id: w,
        storeId: id,
        name: "Web",
        prefix: "HH",
        oldPrefixes: [],
        projects: Array.from({ length: projects }, (_, i) => ({
          ...project(id, w, i + 1),
          displayId: `HH-P${i + 1}`,
        })),
      },
    ],
  };
}

// it は、内部 ID を指定したアイテム(同じ ID を 2 つの Store に置くため)。
export function it(
  storeId: string,
  projectId: string,
  id: string,
  kind: Kind,
  parentId: string | null,
  n = 1,
): Item {
  return { ...item(storeId, projectId, n, kind, parentId), id };
}

// match は、一致した行。ワークスペースは store() と同じ W1。
export function match(
  storeId: string,
  projectId: string,
  itemId: string,
  deleted = false,
): SearchRow {
  const row = result(itemId).rows[0] as SearchRow;
  return {
    ...row,
    storeId,
    workspaceId: `${storeId}-W1`,
    projectId,
    itemId,
    deleted: deleted ? { daysLeft: 3 } : null,
  };
}

// dataOf は、プロジェクト → アイテムの表から data の関数を作る。表にないプロジェクトは undefined。
export function dataOf(table: Record<string, Item[]>) {
  return (storeId: string, projectId: string): ProjectData | undefined => {
    const items = table[`${storeId}/${projectId}`];
    return items ? { items, settings, members: [] } : undefined;
  };
}
