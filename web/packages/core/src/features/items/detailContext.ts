import type { PlanningOverview } from "../../model/planning";
import type { WorkspaceSettings } from "../../model/workspace";
import type { CatalogPort } from "../../port/catalog";
import type { PlanningPort } from "../../port/planning";
import type { WorkspacesPort } from "../../port/workspaces";
import type { People } from "./properties";

// 詳細パネルが読む、ワークスペースごとの設定とメンバーとスプリント・リリースの概要。一度読んだら使い回し、
// 画面への反映(reload)と詳細パネルを閉じたときに読み直す(A3 前半の決定 29)。detail.ts が 300 行の上限を超えるので分けた。

export type Context = { settings: WorkspaceSettings; people: People; plan: PlanningOverview };

export function createContexts(deps: {
  workspaces: WorkspacesPort;
  catalogPort: CatalogPort;
  planning: PlanningPort;
}) {
  const contexts = new Map<string, Promise<Context>>();
  return {
    // of は、ワークスペースの Context。refresh なら読み直す。
    of(storeId: string, workspaceId: string, refresh: boolean): Promise<Context> {
      const key = `${storeId}/${workspaceId}`;
      const cached = contexts.get(key);
      if (cached && !refresh) return cached;
      const read = (async () => {
        const [ws, members, me, plan] = await Promise.all([
          deps.workspaces.get(storeId, workspaceId),
          deps.catalogPort.listMembers(storeId).catch(() => []),
          // me が決まらないときは空(A2 の決定 3)。
          deps.catalogPort.me(storeId).catch(() => ""),
          deps.planning.overview(storeId, workspaceId),
        ]);
        return { settings: ws.settings, people: { members, me }, plan };
      })();
      contexts.set(key, read);
      // 読めなかったら、次のときに読み直せるよう残さない。
      read.catch(() => contexts.get(key) === read && contexts.delete(key));
      return read;
    },
  };
}
