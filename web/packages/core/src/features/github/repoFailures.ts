import { NotFound } from "../../model/errors";
import type { RepoFailure } from "../../model/link";
import type { LinksPort } from "../../port/links";
import { type Readable, readonly, writable } from "../../reactive/store";

// プロジェクトの概要のリポジトリの行の下に出す「PR を探せませんでした: {理由}」(02「概要」)。
// 文言は、このマシンが前回の探索で記録した理由(Store には保存しない)。

export const scanFailedText = (reason: string) => `PR を探せませんでした: ${reason}`;

export interface RepoFailuresState {
  loaded: boolean;
  failures: RepoFailure[];
}

export interface RepoFailures {
  state: Readable<RepoFailuresState>;
  load(storeId: string, projectId: string): Promise<void>;
  reload(): Promise<void>;
  reset(): void;
  // failureOf は、リポジトリの書き方(大文字と小文字を区別しない)から理由を引く。なければ空文字。
  failureOf(repo: string): string;
}

const blank: RepoFailuresState = { loaded: false, failures: [] };

export function createRepoFailures(deps: { links: LinksPort }): RepoFailures {
  const state = writable<RepoFailuresState>(blank);
  let current = blank;
  let target: { storeId: string; projectId: string } | null = null;
  let loads = 0;

  const put = (next: RepoFailuresState) => {
    current = next;
    state.set(next);
  };

  async function read() {
    if (!target) return;
    const t = target;
    loads += 1;
    const mine = loads;
    try {
      const failures = await deps.links.listRepoFailures(t.storeId, t.projectId);
      if (mine === loads) put({ loaded: true, failures });
    } catch (e) {
      if (e instanceof NotFound && mine === loads) put({ loaded: true, failures: [] });
    }
  }

  return {
    state: readonly(state),
    async load(storeId, projectId) {
      if (target?.storeId !== storeId || target.projectId !== projectId) {
        target = { storeId, projectId };
        put(blank);
      }
      await read();
    },
    reload: read,
    reset() {
      target = null;
      loads += 1;
      put(blank);
    },
    failureOf(repo) {
      const key = repo.toLowerCase();
      return current.failures.find((f) => f.repo.toLowerCase() === key)?.text ?? "";
    },
  };
}
