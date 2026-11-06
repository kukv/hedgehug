import type { Release, Sprint } from "../../core/planning";
import { core } from "../context";

// A write of this tab reads the lists again at once: the event of the change comes later.
let written = $state(0);

export function plansWritten(): void {
  written++;
}

/** Reads as a dependency: an effect that reads it runs again after a write of this tab. */
export const plansWrittenCount = (): number => written;

/**
 * The sprints and the releases of a workspace. Any commit of the Store reads them again: a commit
 * that changes neither a task nor the schema comes as tasks.changed, with no task in it. where is
 * null while the screen has no one workspace: the lists are empty then.
 */
export function usePlans(where: () => { store: string; name: string } | null): {
  readonly sprints: Sprint[];
  readonly releases: Release[];
  readonly loaded: boolean;
  readonly error: unknown;
} {
  const c = core();
  let sprints = $state.raw<Sprint[]>([]);
  let releases = $state.raw<Release[]>([]);
  let loaded = $state(false);
  let error = $state.raw<unknown>(null);
  let rev = $state(0);

  $effect(() => {
    const store = where()?.store;
    if (store === undefined) return;
    const offs = [
      c.conn.on("tasks.changed", (e) => {
        if (e.store === store) rev++;
      }),
      c.conn.on("resync", () => rev++),
    ];
    return () => {
      for (const off of offs) off();
    };
  });

  $effect(() => {
    const at = where();
    void rev;
    void written;
    if (!at) {
      sprints = [];
      releases = [];
      error = null;
      loaded = true;
      return;
    }
    const { store, name } = at;
    let live = true;
    const query = { store, workspace: name };
    Promise.all([c.api.listSprints(query), c.api.listReleases(query)])
      .then(([s, r]) => {
        if (!live) return;
        sprints = Array.isArray(s) ? s : [];
        releases = Array.isArray(r) ? r : [];
        error = null;
        loaded = true;
      })
      .catch((e: unknown) => {
        if (!live) return;
        error = e;
        loaded = true;
      });
    return () => {
      live = false;
    };
  });

  return {
    get sprints() {
      return sprints;
    },
    get releases() {
      return releases;
    },
    get loaded() {
      return loaded;
    },
    get error() {
      return error;
    },
  };
}
