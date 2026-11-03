<script lang="ts">
  import { closeViewDialog, viewDialog } from "./dialogs-state.svelte";
  import ViewDelete from "./ViewDelete.svelte";
  import ViewRename from "./ViewRename.svelte";
  import ViewSave from "./ViewSave.svelte";

  const d = $derived(viewDialog.current);
</script>

{#if d?.kind === "save"}
  {#key d}
    <ViewSave draft={d.draft} store={d.store} onclose={closeViewDialog} />
  {/key}
{:else if d?.kind === "rename"}
  {#key d}
    <ViewRename entry={d.entry} carry={d.carry} onclose={closeViewDialog} />
  {/key}
{:else if d?.kind === "delete"}
  {#key d}
    <ViewDelete entry={d.entry} onclose={closeViewDialog} />
  {/key}
{/if}
