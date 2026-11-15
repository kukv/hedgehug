// 検索のツリーのデータ(E3a-3)。プロジェクトのアイテム・ワークスペースの設定・Store のメンバーを読み、鍵ごとに取っておく。
// 設定はワークスペースごと、メンバーは Store ごとに 1 回だけ読む。Store が変わった知らせ(reloads)で reset して読み直す。

import type { Member } from "../../model/member";
import type { WorkspaceSettings } from "../../model/workspace";
import type { ItemsPort } from "../../port/items";
import type { WorkspacesPort } from "../../port/workspaces";
import type { Catalog } from "../catalog/catalog";
import type { ProjectData } from "./searchTree";

export interface SearchTreeData {
  get(storeId: string, projectId: string): ProjectData | undefined;
  // load は、まだないものだけ読む。読めなかったプロジェクトは入れない(次の load で読み直す)。
  load(scope: { storeId: string; projectId: string }[]): Promise<void>;
  // reset は、取っておいたものを捨てる。飛んでいた読み込みの結果も捨てる。
  reset(): void;
}

export interface SearchTreeDataDeps {
  catalog: Catalog;
  items: Pick<ItemsPort, "list">;
  workspaces: Pick<WorkspacesPort, "get">;
  members(storeId: string): Promise<Member[]>;
}

export function createSearchTreeData(deps: SearchTreeDataDeps): SearchTreeData {
  let loaded = new Map<string, ProjectData>();
  let pending = new Map<string, Promise<void>>();
  let settings = new Map<string, Promise<WorkspaceSettings>>();
  let members = new Map<string, Promise<Member[]>>();

  // once は、鍵ごとに 1 回だけ読む。失敗したら取っておかない(次に読み直す)。
  function once<T>(cache: Map<string, Promise<T>>, key: string, read: () => Promise<T>) {
    const hit = cache.get(key);
    if (hit) return hit;
    const p = read();
    cache.set(key, p);
    p.catch(() => {
      if (cache.get(key) === p) cache.delete(key);
    });
    return p;
  }

  async function loadProject(storeId: string, projectId: string, key: string) {
    const workspaceId = deps.catalog.project(storeId, projectId)?.workspaceId;
    if (workspaceId === undefined) return;
    const mine = loaded;
    const [items, s, m] = await Promise.all([
      deps.items.list(storeId, projectId),
      once(
        settings,
        `${storeId}/${workspaceId}`,
        async () => (await deps.workspaces.get(storeId, workspaceId)).settings,
      ),
      once(members, storeId, () => deps.members(storeId)),
    ]);
    // reset のあとに届いた結果は捨てる。
    if (mine === loaded) loaded.set(key, { items, settings: s, members: m });
  }

  return {
    get: (storeId, projectId) => loaded.get(`${storeId}/${projectId}`),
    async load(scope) {
      const runs = scope.map(({ storeId, projectId }) => {
        const key = `${storeId}/${projectId}`;
        if (loaded.has(key)) return Promise.resolve();
        // 読んでいる途中のものは、その読み込みを待つ(同じプロジェクトを 2 回読まない)。
        const flying = pending.get(key);
        if (flying) return flying;
        const map = pending;
        const run = loadProject(storeId, projectId, key).finally(() => {
          if (map.get(key) === run) map.delete(key);
        });
        map.set(key, run);
        return run;
      });
      await Promise.allSettled(runs);
    },
    reset() {
      loaded = new Map();
      pending = new Map();
      settings = new Map();
      members = new Map();
    },
  };
}
