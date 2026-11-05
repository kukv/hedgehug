<script lang="ts">
  import GanttTab from "../gantt/GanttTab.svelte";
  import ListTab from "../list/ListTab.svelte";
  import { useListScope } from "../list/scope.svelte";
  import TreeTab from "../list/TreeTab.svelte";
  import type { ProjectTab } from "../router";
  import StoreBands from "../states/StoreBands.svelte";
  import { NO_CANCELED } from "../tree/filter";
  import ProjectHeader from "./ProjectHeader.svelte";
  import ProjectOverview from "./ProjectOverview.svelte";
  import ProjectSettings from "./ProjectSettings.svelte";

  // The page of a project: the heading with the tabs, and the tab. The tabs read the project
  // themselves and say so when it is missing; the heading has no title until it is read.
  type Props = { ulid: string; tab: ProjectTab };
  let { ulid, tab }: Props = $props();

  const view = $derived({ kind: "project" as const, ulid });
  const scope = useListScope(() => view);
  // The filter of the tree, the kanban and the table: one for the page, so it stays when the tab
  // changes (it is not saved: D7). The default leaves the canceled tasks out.
  let query = $state(NO_CANCELED);
</script>

<div class="page">
  {#if scope.project}
    <StoreBands store={scope.project.store} spaced />
  {/if}
  <ProjectHeader {ulid} project={scope.project} workspace={scope.workspace} {tab} />
  <div class="body">
    {#if tab === "overview"}
      <ProjectOverview {ulid} />
    {:else if tab === "tree"}
      <TreeTab {view} {query} onquery={(q) => (query = q)} />
    {:else if tab === "gantt"}
      <GanttTab {view} {query} onquery={(q) => (query = q)} />
    {:else if tab === "settings"}
      <ProjectSettings {ulid} />
    {:else}
      <ListTab {view} layout={tab} {query} onquery={(q) => (query = q)} />
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
