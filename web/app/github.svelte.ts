import type { GitHubRepo, IssueFields, Rule, RuleStat } from "../core/github";
import { core } from "./context";
import { useSubscribe } from "./svelte-store";

// What a screen shows beside the tasks. These are reads a screen can do without: a failed one
// leaves what it has, and the next change reads again.
function useRead<T>(
  key: () => string | null,
  read: (key: string) => Promise<T>,
): { readonly current: T | undefined; reload: () => void } {
  let value = $state.raw<T>();
  let rev = $state(0);
  $effect(() => {
    const k = key();
    void rev;
    if (k === null) {
      value = undefined;
      return;
    }
    let live = true;
    void (async () => {
      try {
        const v = await read(k);
        if (live) value = v;
      } catch {
        // see above
      }
    })();
    return () => {
      live = false;
    };
  });
  return {
    get current() {
      return value;
    },
    reload: () => rev++,
  };
}

/** The reads of each repository of a Store; a change of its sync state reads them again. */
export function useGitHubRepos(store: () => string | undefined): {
  readonly current: GitHubRepo[];
} {
  const c = core();
  const sync = useSubscribe(c.conn, () => {
    const s = store();
    return s === undefined ? undefined : c.conn.sync.get(s);
  });
  const r = useRead(
    () => {
      void sync.current;
      return store() ?? null;
    },
    async (s) => (await c.api.getGithub({ store: s })) ?? [],
  );
  return {
    get current() {
      return r.current ?? [];
    },
  };
}

/** The reads of the repositories of every Store. */
export function useAllGitHubRepos(): { readonly current: GitHubRepo[] } {
  const c = core();
  const sync = useSubscribe(c.conn, () => JSON.stringify([...c.conn.sync.values()]));
  const r = useRead(
    () => {
      void sync.current;
      return "all";
    },
    async () => (await c.api.getGithub()) ?? [],
  );
  return {
    get current() {
      return r.current ?? [];
    },
  };
}

// Reads again when the file of the rules of the Store changes.
function followRules(store: () => string | undefined, reload: () => void): void {
  const c = core();
  $effect(() => {
    const offs = [
      c.conn.on("automations.changed", (e) => {
        if (e.store === store()) reload();
      }),
      c.conn.on("resync", reload),
    ];
    return () => {
      for (const off of offs) off();
    };
  });
}

/** The rules of a Store as they apply in a project, or all of the Store's without one. Undefined until read. */
export function useRules(
  store: () => string | undefined,
  project: () => string | undefined = () => undefined,
): { readonly current: Rule[] | undefined } {
  const c = core();
  const r = useRead(
    () => {
      const s = store();
      return s === undefined ? null : JSON.stringify([s, project() ?? ""]);
    },
    async (k) => {
      const [s, p] = JSON.parse(k) as [string, string];
      const query = p === "" ? { store: s } : { store: s, project: p };
      return (await c.api.listAutomations(query)) ?? [];
    },
  );
  followRules(store, r.reload);
  return r;
}

/** The fields of a Store that a link of an issue syncs by default. Undefined until read. */
export function useIssueFields(store: () => string | undefined): {
  readonly current: IssueFields | undefined;
} {
  const c = core();
  const r = useRead(
    () => store() ?? null,
    (s) => c.api.getAutomationFields({ store: s }),
  );
  followRules(store, r.reload);
  return r;
}

/** What the rules of a Store did in the last 7 days. It is counted from the history, so a sync reads it again. */
export function useRuleStats(store: () => string): { readonly current: RuleStat[] } {
  const c = core();
  const sync = useSubscribe(c.conn, () => c.conn.sync.get(store()));
  const r = useRead(
    () => {
      void sync.current;
      return store();
    },
    async (s) => (await c.api.getAutomationStats({ store: s })) ?? [],
  );
  followRules(store, r.reload);
  return {
    get current() {
      return r.current ?? [];
    },
  };
}
