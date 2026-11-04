import { readOnlyReason, type StoreDirectory } from "../core/stores";
import { useSubscribe } from "./svelte-store";
import { toast } from "./toast.svelte";

/** Why a Store cannot be written right now, in Japanese, or null (read it after subscribing to the directory). */
export function readOnlyNow(
  stores: Pick<StoreDirectory, "readOnly">,
  store: string,
): string | null {
  const raw = stores.readOnly(store);
  return raw === null ? null : readOnlyReason(raw);
}

/**
 * Why a Store cannot be written, in Japanese, or null when it can. It follows the directory, so it
 * changes when a sync finds the remote writable (or not).
 */
export function useReadOnly(
  stores: Pick<StoreDirectory, "readOnly" | "subscribe">,
  store: () => string | undefined,
): { readonly current: string | null } {
  return useSubscribe(stores, () => {
    const s = store();
    return s === undefined ? null : readOnlyNow(stores, s);
  });
}

/** The sentence a read-only Store is explained with (the band and the toast). */
export function readOnlySentence(reason: string): string {
  return `この Store は閲覧のみです（${reason}）`;
}

/** What a keyboard operation says when a read-only Store refuses it (the buttons are disabled). */
export function readOnlyToast(reason: string): void {
  toast({ message: readOnlySentence(reason) });
}
