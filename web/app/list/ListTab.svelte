<script lang="ts">
  import {
    DEFAULT_KANBAN,
    DEFAULT_TABLE,
    type KanbanSettings,
    type TableSettings,
  } from "../../core/views";
  import KanbanView from "../kanban/KanbanView.svelte";
  import TableView from "../table/TableView.svelte";
  import { NO_CANCELED } from "../tree/filter";
  import SaveViewButton from "./SaveViewButton.svelte";
  import type { ListScope } from "./scope";

  // The body of the kanban tab and the table tab of a workspace or a project. What is on the
  // screen (the filter, the columns, the swimlane, the group) is kept here, for as long as the
  // screen is open and not saved (D7): one ListTab serves both tabs, so the filter stays when
  // the tab changes.
  type Props = { view: ListScope; layout: "kanban" | "table" };
  let { view, layout }: Props = $props();

  // The default filter leaves the canceled tasks out, as the tree does.
  let query = $state(NO_CANCELED);
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
    onquery={(q) => (query = q)}
    settings={kanban}
    onsettings={(s) => (kanban = s)}
    actions={save}
  />
{:else}
  <TableView
    {view}
    {query}
    onquery={(q) => (query = q)}
    settings={table}
    onsettings={(s) => (table = s)}
    actions={save}
  />
{/if}
