import { NotFound } from "../../model/errors";
import type { ProjectLinkRow } from "../../model/link";
import type { LinksPort } from "../../port/links";
import { type Readable, readonly, writable } from "../../reactive/store";

// プロジェクトの概要の「PR と Issue」(02「概要」)。GitHub には問い合わせず、Store に保存した値と、
// このマシンが読んだチェックの結果を出す。失敗は出さない(A2 の決定 29)。

export interface ProjectLinksState {
  loaded: boolean;
  rows: ProjectLinkRow[];
}

export interface ProjectLinks {
  state: Readable<ProjectLinksState>;
  load(storeId: string, projectId: string): Promise<void>;
  reload(): Promise<void>;
  reset(): void;
}

const blank: ProjectLinksState = { loaded: false, rows: [] };

export function createProjectLinks(deps: { links: LinksPort }): ProjectLinks {
  const state = writable<ProjectLinksState>(blank);
  let target: { storeId: string; projectId: string } | null = null;
  let loads = 0;

  async function read() {
    if (!target) return;
    const t = target;
    loads += 1;
    const mine = loads;
    try {
      const rows = await deps.links.listProject(t.storeId, t.projectId);
      if (mine === loads) state.set({ loaded: true, rows });
    } catch (e) {
      if (e instanceof NotFound && mine === loads) state.set({ loaded: true, rows: [] });
    }
  }

  return {
    state: readonly(state),
    async load(storeId, projectId) {
      if (target?.storeId !== storeId || target.projectId !== projectId) {
        target = { storeId, projectId };
        state.set(blank);
      }
      await read();
    },
    reload: read,
    reset() {
      target = null;
      loads += 1;
      state.set(blank);
    },
  };
}
