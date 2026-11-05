<script lang="ts">
  import { closePlanDialog, planDialog } from "./dialogs-state.svelte";
  import PlanRemove from "./PlanRemove.svelte";
  import ReleaseForm from "./ReleaseForm.svelte";
  import SprintClose from "./SprintClose.svelte";
  import SprintForm from "./SprintForm.svelte";
  import SprintStart from "./SprintStart.svelte";

  const d = $derived(planDialog.current);
</script>

{#if d}
  {#key d}
    {#if d.kind === "sprint-add"}
      <SprintForm entry={d.entry} onclose={closePlanDialog} />
    {:else if d.kind === "sprint-edit"}
      <SprintForm entry={d.entry} sprint={d.sprint} onclose={closePlanDialog} />
    {:else if d.kind === "sprint-start"}
      <SprintStart
        entry={d.entry}
        sprint={d.sprint}
        sprints={d.sprints}
        onclose={closePlanDialog}
      />
    {:else if d.kind === "sprint-close"}
      <SprintClose
        entry={d.entry}
        sprint={d.sprint}
        sprints={d.sprints}
        onclose={closePlanDialog}
      />
    {:else if d.kind === "release-add"}
      <ReleaseForm entry={d.entry} onclose={closePlanDialog} />
    {:else if d.kind === "release-edit"}
      <ReleaseForm entry={d.entry} release={d.release} onclose={closePlanDialog} />
    {:else}
      <PlanRemove dialog={d} onclose={closePlanDialog} />
    {/if}
  {/key}
{/if}
