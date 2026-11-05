import { core } from "../context";
import { plansWrittenCount } from "../planning/plans.svelte";

/**
 * One report: read when what load reads changes, and again after any commit of the Store. load
 * returns null while there is nothing to ask. Only the answer to the last request is kept.
 */
export function useReport<T>(
  store: () => string,
  load: () => Promise<T> | null,
): {
  readonly value: T | null;
  readonly loading: boolean;
  readonly error: unknown;
} {
  const c = core();
  let value = $state.raw<T | null>(null);
  let loading = $state(true);
  let error = $state.raw<unknown>(null);
  let rev = $state(0);

  $effect(() => {
    const name = store();
    const offs = [
      c.conn.on("tasks.changed", (e) => {
        if (e.store === name) rev++;
      }),
      c.conn.on("resync", () => rev++),
    ];
    return () => {
      for (const off of offs) off();
    };
  });

  $effect(() => {
    void rev;
    void plansWrittenCount();
    const p = load();
    if (!p) {
      value = null;
      error = null;
      loading = false;
      return;
    }
    let live = true;
    loading = true;
    p.then((v) => {
      if (!live) return;
      value = v;
      error = null;
      loading = false;
    }).catch((e: unknown) => {
      if (!live) return;
      value = null;
      error = e;
      loading = false;
    });
    return () => {
      live = false;
    };
  });

  return {
    get value() {
      return value;
    },
    get loading() {
      return loading;
    },
    get error() {
      return error;
    },
  };
}
