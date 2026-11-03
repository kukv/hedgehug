<script lang="ts">
  import ListTab from "../list/ListTab.svelte";
  import { useListScope } from "../list/scope.svelte";
  import TreeTab from "../list/TreeTab.svelte";
  import type { ProjectTab } from "../router";
  import ProjectHeader from "./ProjectHeader.svelte";

  // The page of a project: the heading with the tabs, and the tab. The tabs read the project
  // themselves and say so when it is missing; the heading has no title until it is read.
  type Props = { ulid: string; tab: ProjectTab };
  let { ulid, tab }: Props = $props();

  const view = $derived({ kind: "project" as const, ulid });
  const scope = useListScope(() => view);
</script>

<div class="page">
  <ProjectHeader {ulid} project={scope.project} workspace={scope.workspace} {tab} />
  <div class="body">
    {#if tab === "tree"}
      <TreeTab {view} />
    {:else}
      <ListTab {view} layout={tab} />
    {/if}
  </div>
</div>

<style>
  /* The tree scrolls in its own box, so the page is the height of the window and the body takes
     what the heading leaves (as the page of a workspace does). */
  .page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .body {
    flex: 1 1 auto;
    min-height: 0;
  }
</style>
