import { type Api, unwrap } from "../../core/api/client";
import { errorMessage } from "../../core/messages";
import { toast } from "../toast.svelte";
import { syncMessage } from "./sync-summary";

let busy = $state(false);

/** Whether a sync that the sidebar or the palette started is running. */
export const syncRun = {
  get busy(): boolean {
    return busy;
  },
};

/** Runs a sync now (of one store, or of every one) and says how it went. A second call while one runs does nothing. */
export async function runSync(api: Api, store?: string): Promise<void> {
  if (busy) return;
  busy = true;
  try {
    const reports = await unwrap(
      store === undefined
        ? api.POST("/api/sync")
        : api.POST("/api/sync", { params: { query: { store } } }),
    );
    toast({ message: syncMessage(reports) });
  } catch (e) {
    toast({ message: errorMessage(e) });
  } finally {
    busy = false;
  }
}
