<script lang="ts">
  import {
    DEFAULT_GANTT,
    DEFAULT_KANBAN,
    DEFAULT_TABLE,
    type GanttSettings,
    type KanbanSettings,
    type TableSettings,
  } from "../../core/views";
  import GanttView from "../gantt/GanttView.svelte";
  import KanbanView from "../kanban/KanbanView.svelte";
  import type { ListScope } from "../list/scope";
  import { type Layout, router } from "../router";
  import TableView from "../table/TableView.svelte";
  import TreeView from "../tree/TreeView.svelte";
  import Button from "../ui/Button.svelte";
  import Tabs from "../ui/Tabs.svelte";
  import { openViewSave } from "./dialogs-state.svelte";
  import { LAYOUTS, newDraft } from "./draft";

  // A query that is not saved (D1): it is evaluated in every Store, the layout is a tab, and the
  // query is in the address (?q=), so the address can be copied.
  type Props = { layout: Layout; q: string };
  let { layout, q }: Props = $props();

  // The view of every Store, one object, so the lists below do not start over at each render.
  const everywhere: ListScope = { kind: "all" };

  // The columns and the swimlane are for as long as the screen is open and not saved (D7).
  let kanban = $state.raw<KanbanSettings>(DEFAULT_KANBAN);
  let table = $state.raw<TableSettings>(DEFAULT_TABLE);
  let gantt = $state.raw<GanttSettings>(DEFAULT_GANTT);

  // What the filter field holds until the address says the same: the route that comes back from
  // our own replace is the same text, so the field the user is typing in is not wound back.
  let typed = $state<string | null>(null);
  const query = $derived(typed ?? q);
  $effect(() => {
    q;
    typed = null;
  });

  const at = (l: Layout, text: string) => router.href({ name: "query", layout: l, q: text });
  const items = $derived(
    LAYOUTS.map((t) => ({
      label: t.label,
      href: at(t.layout, query),
      current: t.layout === layout,
    })),
  );
  // The filter writes the address without a new entry in the history.
  function setQuery(next: string) {
    typed = next;
    router.replace(at(layout, next));
  }
  const save = () => openViewSave({ draft: newDraft({ layout, query, kanban, table, gantt }) });
</script>

{#snippet saveButton()}
  <Button size="sm" onclick={save}>ビューを保存</Button>
{/snippet}

<div class="page">
  <header class="head">
    <h1>クエリ</h1>
    <span class="eyebrow">保存していないクエリ</span>
    <span class="note">すべての Store で調べます。store: で絞れます</span>
  </header>
  <Tabs {items} label="レイアウトの切り替え" />
  <div class="body">
    {#if layout === "tree"}
      <TreeView view={everywhere} {query} onquery={setQuery} actions={saveButton} />
    {:else if layout === "gantt"}
      <GanttView
        view={everywhere}
        {query}
        onquery={setQuery}
        settings={gantt}
        onsettings={(s) => (gantt = s)}
        actions={saveButton}
      />
    {:else if layout === "kanban"}
      <KanbanView
        view={everywhere}
        {query}
        onquery={setQuery}
        settings={kanban}
        onsettings={(s) => (kanban = s)}
        actions={saveButton}
      />
    {:else}
      <TableView
        view={everywhere}
        {query}
        onquery={setQuery}
        settings={table}
        onsettings={(s) => (table = s)}
        actions={saveButton}
      />
    {/if}
  </div>
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--space-10);
    padding: var(--space-16) var(--gutter) 0;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .eyebrow {
    color: var(--label);
    font-size: var(--fs-11);
    font-weight: 600;
    letter-spacing: 0.04em;
  }
  .note {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .body {
    flex: 1 1 auto;
    min-height: 0;
  }
</style>
