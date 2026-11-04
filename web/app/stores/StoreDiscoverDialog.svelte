<script lang="ts">
  import { toast } from "../toast.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import StoreDiscover, { type MountedEvent } from "./StoreDiscover.svelte";

  // "Store を探す" of the Settings: the search and the mount of StoreDiscover in a dialog.
  type Props = { open: boolean; onclose: () => void };
  let { open, onclose }: Props = $props();

  function mounted(e: MountedEvent): void {
    if (e.mounted.length > 0)
      toast({ message: `${e.mounted.length} 個の Store をマウントしました` });
    // A row that failed keeps its words on the screen.
    if (e.failed === 0) onclose();
  }
</script>

<Dialog {open} title="Store を探す" width={760} {onclose}>
  <StoreDiscover mountLabel="マウント" onmounted={mounted} />
</Dialog>
