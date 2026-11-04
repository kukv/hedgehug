import { render } from "@testing-library/svelte";
import { type Component, flushSync } from "svelte";
import { type AppCore, CORE_KEY } from "./context";

/** A minimal subscribe / emit pair for fakes of the core objects. */
export function emitter() {
  const fns = new Set<() => void>();
  return {
    subscribe(fn: () => void): () => void {
      fns.add(fn);
      return () => {
        fns.delete(fn);
      };
    },
    emit(): void {
      for (const fn of [...fns]) fn();
    },
    size(): number {
      return fns.size;
    },
  };
}

/** A core made of fakes; `over` replaces any of the eight objects. */
export function fakeCore(over: Partial<Record<keyof AppCore, unknown>> = {}): AppCore {
  const e = emitter();
  const base = {
    api: {},
    conn: {
      state: "open",
      lastOpenAt: null,
      sync: new Map(),
      unread: 0,
      subscribe: e.subscribe,
      on: () => () => {},
      start() {},
      stop() {},
    },
    cache: {
      subscribe: e.subscribe,
      // a query that finds nothing
      watchList: () => ({ ulids: () => [], error: () => null, loading: () => false, dispose() {} }),
    },
    save: { subscribe: e.subscribe },
    stores: {
      stores: () => [],
      tone: () => "teal",
      workspaces: () => [],
      workspace: () => undefined,
      loaded: () => true,
      reload: async () => {},
      subscribe: e.subscribe,
    },
    schema: { subscribe: e.subscribe },
    views: {
      list: () => [],
      get: () => undefined,
      loaded: () => true,
      subscribe: e.subscribe,
    },
    accounts: {
      accounts: () => [],
      loaded: () => true,
      error: () => null,
      load: async () => {},
      reload: async () => {},
      subscribe: e.subscribe,
    },
  };
  return { ...base, ...over } as unknown as AppCore;
}

/** Renders a component with a fake core in its context. */
export function renderWithCore<P extends Record<string, unknown>>(
  C: Component<P>,
  props: P,
  over: Partial<Record<keyof AppCore, unknown>> = {},
) {
  const c = fakeCore(over);
  const r = render(C as Component<Record<string, unknown>>, {
    props,
    context: new Map([[CORE_KEY, c]]),
  });
  flushSync(); // run the effects and onMount callbacks before the test looks
  return { core: c, ...r };
}
