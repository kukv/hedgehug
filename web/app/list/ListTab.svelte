<script lang="ts">
  import {
    DEFAULT_KANBAN,
    DEFAULT_TABLE,
    type KanbanSettings,
    type TableSettings,
  } from "../../core/views";
  import KanbanView from "../kanban/KanbanView.svelte";
  import TableView from "../table/TableView.svelte";
  import SaveViewButton from "./SaveViewButton.svelte";
  import type { ListScope } from "./scope";

  // The body of the kanban tab and the table tab of a workspace or a project. The columns, the
  // swimlane and the group are kept here, for as long as the screen is open and not saved (D7). The
  // filter is the page's, so that it stays across the three tabs.
  type Props = {
    view: ListScope;
    layout: "kanban" | "table";
    query: string;
    onquery: (q: string) => void;
  };
  let { view, layout, query, onquery }: Props = $props();

  let kanban = $state.raw<KanbanSettings>(DEFAULT_KANBAN);
  let table = $state.raw<TableSettings>(DEFAULT_TABLE);
</script>

{#snippet save()}
  <SaveViewButton {view} {layout} filter={query} {kanban} {table} />
{/snippet}

{#if layout === "kanban"}
  <KanbanView
    {view}
    {query}
    {onquery}
    settings={kanban}
    onsettings={(s) => (kanban = s)}
    actions={save}
  />
{:else}
  <TableView
    {view}
    {query}
    {onquery}
    settings={table}
    onsettings={(s) => (table = s)}
    actions={save}
  />
{/if}
