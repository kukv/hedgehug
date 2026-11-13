import type { Kind } from "../../model/item";
import type { PlanningOverview } from "../../model/planning";
import type { WorkspaceSettings } from "../../model/workspace";
import type { CatalogPort } from "../../port/catalog";
import type { PlanningPort } from "../../port/planning";
import type { WorkspacesPort } from "../../port/workspaces";
import type { Values } from "./createValues";
import { type People, type PropertyRow, propertyRows } from "./properties";

export interface WorkspaceContext {
  settings: WorkspaceSettings;
  people: People;
  // plan は、スプリントの選択肢に使う、ワークスペースのスプリントとリリースの概要。
  plan: PlanningOverview;
}

// createContextLoader は、ワークスペースの設定とメンバー・あなた・スプリントとリリースの概要を読み、ワークスペースごとに覚える。
export function createContextLoader(
  workspaces: WorkspacesPort,
  catalog: CatalogPort,
  planning: PlanningPort,
) {
  const cache = new Map<string, Promise<WorkspaceContext>>();
  return (storeId: string, workspaceId: string): Promise<WorkspaceContext> => {
    const key = `${storeId}/${workspaceId}`;
    let p = cache.get(key);
    if (!p) {
      p = Promise.all([
        workspaces.get(storeId, workspaceId),
        catalog.listMembers(storeId),
        catalog.me(storeId).catch(() => ""),
        planning.overview(storeId, workspaceId),
      ]).then(([d, members, me, plan]) => ({
        settings: d.settings,
        people: { members, me },
        plan,
      }));
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
  form: { kind: Kind; values: Values },
): PropertyRow[] {
  return propertyRows(ctx.settings, form.kind, (p) => form.values[p.id] ?? "", ctx.people);
}
