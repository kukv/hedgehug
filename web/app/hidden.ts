import type { StoreDirectory } from "../core/stores";

/**
 * Whether what is of a Store is shown: false only for a Store that is hidden (an unknown Store, or
 * none, is shown). The one question every list asks (細部 93); call it where the directory is read
 * reactively (after reading the subscription in the same $derived).
 */
export function storeShown(
  stores: Pick<StoreDirectory, "hidden">,
  store: string | undefined,
): boolean {
  return store === undefined || !stores.hidden(store);
}

/** The names of the Stores that are hidden, in the order of the directory. */
export function hiddenStores(stores: Pick<StoreDirectory, "stores" | "hidden">): string[] {
  return stores
    .stores()
    .filter((s) => stores.hidden(s.name))
    .map((s) => s.name);
}
