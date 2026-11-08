import { tick } from "svelte";
import { errorMessage } from "../../core/messages";
import { core } from "../context";
import { toast } from "../toast.svelte";
import { HANDLE, moved } from "./reorder";

// useOrder sends the new order of a list and reads the schema again: the screen is drawn from what
// the server now holds, never from a guess (a stale list is refused, and there is nothing to undo).
// The handles of the table are disabled while it is sent, and the focus goes back to the moved one.
// Call it while a component is being made.
export function useOrder(store: () => string, off: () => string | null) {
  const c = core();
  let busy = $state(false);
  return {
    get busy() {
      return busy;
    },
    // move sends names with the item at from put at to; at an edge (or while sending) nothing is sent.
    async move(
      handle: string,
      names: readonly string[],
      from: number,
      to: number,
      send: (api: typeof c.api, store: string, names: string[]) => Promise<unknown>,
    ) {
      const next = moved(names, from, to);
      if (busy || off() !== null || next.every((n, i) => n === names[i])) return;
      const s = store();
      busy = true;
      try {
        await send(c.api, s, next);
      } catch (e) {
        toast({ message: errorMessage(e) });
      }
      try {
        await c.schema.reload(s);
      } catch (e) {
        toast({ message: errorMessage(e) });
      }
      busy = false;
      await tick();
      for (const el of document.querySelectorAll<HTMLElement>(`[${HANDLE}]`)) {
        if (el.getAttribute(HANDLE) === handle) el.focus();
      }
    },
  };
}
