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

/** Runs a sync now and says how it went. A second call while one runs does nothing. */
export async function runSync(api: Api): Promise<void> {
  if (busy) return;
  busy = true;
  try {
    const reports = await unwrap(api.POST("/api/sync"));
    toast({ message: syncMessage(reports) });
  } catch (e) {
    toast({ message: errorMessage(e) });
  } finally {
    busy = false;
  }
}
