import { unwrap } from "../../core/api/client";
import type { Release, Sprint } from "../../core/planning";
import { core } from "../context";

// A write of this tab reads the lists again at once: the event of the change comes later.
let written = $state(0);

export function plansWritten(): void {
  written++;
}

/**
 * The sprints and the releases of a workspace. Any commit of the Store reads them again: a commit
 * that changes neither a task nor the schema comes as tasks.changed, with no task in it.
 */
export function usePlans(where: () => { store: string; name: string }): {
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
    const { store } = where();
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
    const { store, name } = where();
    void rev;
    void written;
    let live = true;
    const query = { store, workspace: name };
    Promise.all([
      unwrap(c.api.GET("/api/sprints", { params: { query } })),
      unwrap(c.api.GET("/api/releases", { params: { query } })),
    ])
      .then(([s, r]) => {
        if (!live) return;
        sprints = s ?? [];
        releases = r ?? [];
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
