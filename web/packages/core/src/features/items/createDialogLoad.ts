import type { Kind } from "../../model/item";
import type { WorkspaceSettings } from "../../model/workspace";
import type { CatalogPort } from "../../port/catalog";
import type { WorkspacesPort } from "../../port/workspaces";
import type { Values } from "./createValues";
import { type People, type PropertyRow, propertyRows } from "./properties";

export interface WorkspaceContext {
  settings: WorkspaceSettings;
  people: People;
}

// createContextLoader は、ワークスペースの設定とメンバー・あなたを読み、ワークスペースごとに覚える。
export function createContextLoader(workspaces: WorkspacesPort, catalog: CatalogPort) {
  const cache = new Map<string, Promise<WorkspaceContext>>();
  return (storeId: string, workspaceId: string): Promise<WorkspaceContext> => {
    const key = `${storeId}/${workspaceId}`;
    let p = cache.get(key);
    if (!p) {
      p = Promise.all([
        workspaces.get(storeId, workspaceId),
        catalog.listMembers(storeId),
        catalog.me(storeId).catch(() => ""),
      ]).then(([d, members, me]) => ({ settings: d.settings, people: { members, me } }));
      cache.set(key, p);
      // 失敗は覚えない(次に読むときにもう一度読む)。
      p.catch(() => cache.delete(key));
    }
    return p;
  };
}

// rowsFor は、いまの種類・値でのプロパティの行。
export function rowsFor(
  ctx: WorkspaceContext,
  form: { kind: Kind; values: Values; projectId: string },
): PropertyRow[] {
  return propertyRows(
    ctx.settings,
    form.kind,
    (p) => form.values[p.id] ?? "",
    ctx.people,
    form.projectId,
  );
}
