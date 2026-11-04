import { createSubscriber } from "svelte/reactivity";

/**
 * Reads a core object from runes. The read re-runs whenever the object notifies,
 * and the subscription lives only while something reads it.
 */
export function useSubscribe<T>(
  source: { subscribe(fn: () => void): () => void },
  read: () => T,
): { readonly current: T } {
  const subscribe = createSubscriber((update) => source.subscribe(update));
  return {
    get current(): T {
      subscribe();
      return read();
    },
  };
}

/** Whether there are two Stores or more: the screens show the Store of a row only then. */
export function useManyStores(stores: {
  subscribe(fn: () => void): () => void;
  stores(): readonly unknown[];
}): { readonly current: boolean } {
  return useSubscribe(stores, () => stores.stores().length >= 2);
}

/** The ID of a task with its Store name where the prefix is in two Stores (follows the directory). */
export function useDisplayId(stores: {
  subscribe(fn: () => void): () => void;
  displayId(store: string, id: string): string;
}): { readonly current: (store: string, id: string) => string } {
  return useSubscribe(stores, () => (store: string, id: string) => stores.displayId(store, id));
}
