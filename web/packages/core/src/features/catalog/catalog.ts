import type { Project } from "../../model/project";
import type { StoreInfo } from "../../model/store";
import type { Workspace } from "../../model/workspace";
import type { CatalogPort } from "../../port/catalog";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { projectDisplayId } from "../router/route";

// 一覧の状態: 表示中の Store → ワークスペース → プロジェクト(サイドバー・作成ダイアログ・アドレスの解決が使う)。
// 起動時と、プロジェクトを作ったあとに読み直す(A2 の計画の決定 19)。

export interface ProjectNode extends Project {
  displayId: string;
}

export interface WorkspaceNode extends Workspace {
  projects: ProjectNode[];
}

export interface StoreNode extends StoreInfo {
  workspaces: WorkspaceNode[];
}

export interface Catalog {
  stores: Readable<StoreNode[]>;
  // load は、すべてを読み直す。失敗はそのまま投げ、いまの一覧は変えない。
  load(): Promise<void>;
  store(storeId: string): StoreNode | undefined;
  workspace(storeId: string, workspaceId: string): WorkspaceNode | undefined;
  project(storeId: string, projectId: string): ProjectNode | undefined;
}

export function createCatalog(port: CatalogPort): Catalog {
  const stores = writable<StoreNode[]>([]);
  // 読み直しが重なったときは、最後に始めたものの結果だけを使う。
  let latest = 0;
  let newest: Promise<void> = Promise.resolve();

  async function loadWorkspace(storeId: string, w: Workspace): Promise<WorkspaceNode> {
    const projects = await port.listProjects(storeId, w.id);
    return {
      ...w,
      projects: projects.map((p) => ({ ...p, displayId: projectDisplayId(w.prefix, p.number) })),
    };
  }

  async function loadStore(info: StoreInfo): Promise<StoreNode> {
    const workspaces = await port.listWorkspaces(info.id);
    return {
      ...info,
      workspaces: await Promise.all(workspaces.map((w) => loadWorkspace(info.id, w))),
    };
  }

  const store = (storeId: string) => get(stores).find((s) => s.id === storeId);
  const workspace = (storeId: string, workspaceId: string) =>
    store(storeId)?.workspaces.find((w) => w.id === workspaceId);

  return {
    stores: readonly(stores),
    async load() {
      latest += 1;
      const mine = latest;
      const run = (async () => {
        const infos = await port.listStores();
        const loaded = await Promise.all(infos.map(loadStore));
        if (mine === latest) stores.set(loaded);
      })();
      newest = run;
      await run;
      // 後から始めた読み直しに結果を譲ったときは、その読み直しが終わるまで返さない(呼んだ側が、自分の保存のあとの
      // 一覧を読めるように)。後の読み直しの失敗は、その呼び出しが受け取る。
      let waiting: Promise<void>;
      do {
        waiting = newest;
        await waiting.catch(() => {});
      } while (waiting !== newest);
    },
    store,
    workspace,
    project(storeId, projectId) {
      for (const w of store(storeId)?.workspaces ?? []) {
        const p = w.projects.find((x) => x.id === projectId);
        if (p) return p;
      }
      return undefined;
    },
  };
}

// writableWorkspaces は、作成ダイアログに出すワークスペース(閲覧のみの Store のものを除く。01・02)。
// 非表示の Store は、一覧にもともと入らない。
export function writableWorkspaces(stores: StoreNode[]): WorkspaceNode[] {
  return stores.filter((s) => !s.readOnly).flatMap((s) => s.workspaces);
}
