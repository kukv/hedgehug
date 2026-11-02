<script lang="ts">
  import { closeWorkspaceDialog, workspaceDialog } from "./dialogs-state.svelte";
  import PrefixChange from "./PrefixChange.svelte";
  import WorkspaceAdd from "./WorkspaceAdd.svelte";
  import WorkspaceDelete from "./WorkspaceDelete.svelte";
  import WorkspaceMove from "./WorkspaceMove.svelte";

  const d = $derived(workspaceDialog.current);
</script>

{#if d?.kind === "add"}
  {#key d}
    <WorkspaceAdd store={d.store} onclose={closeWorkspaceDialog} />
  {/key}
{:else if d?.kind === "delete"}
  {#key d}
    <WorkspaceDelete entry={d.entry} onclose={closeWorkspaceDialog} />
  {/key}
{:else if d?.kind === "prefix"}
  {#key d}
    <PrefixChange entry={d.entry} onclose={closeWorkspaceDialog} />
  {/key}
{:else if d?.kind === "move"}
  {#key d}
    <WorkspaceMove task={d.task} onclose={closeWorkspaceDialog} />
  {/key}
{/if}
