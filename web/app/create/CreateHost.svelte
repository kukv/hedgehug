<script lang="ts">
  import { closeCreate, createState, openCreate } from "../create-state.svelte";
  import { registerShortcut } from "../keys";
  import CreateProject from "./CreateProject.svelte";
  import CreateTask from "./CreateTask.svelte";

  const req = $derived(createState.request);

  // C opens the task dialog. registerShortcut leaves the key alone while typing and while a dialog is open.
  $effect(() => registerShortcut("c", () => openCreate({ kind: "task" })));
</script>

{#if req?.kind === "task"}
  {#key req}
    <CreateTask request={req} onclose={closeCreate} />
  {/key}
{:else if req?.kind === "project"}
  {#key req}
    <CreateProject request={req} onclose={closeCreate} />
  {/key}
{/if}
