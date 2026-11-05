<script lang="ts">
  import {
    DEFAULT_GANTT,
    DEFAULT_KANBAN,
    DEFAULT_TABLE,
    type GanttSettings,
    type KanbanSettings,
    type TableSettings,
  } from "../../core/views";
  import type { Layout } from "../router";
  import { combine } from "../tree/filter";
  import Button from "../ui/Button.svelte";
  import { openViewSave } from "../views/dialogs-state.svelte";
  import { newDraft } from "../views/draft";
  import type { ListScope } from "./scope";
  import { useListScope } from "./scope.svelte";

  // The "ビューを保存" button of a screen of a workspace or a project (D8). filter is what the
  // filter field holds; the fixed part of the screen is added here.
  type Props = {
    view: ListScope;
    layout: Layout;
    filter: string;
    kanban?: KanbanSettings;
    table?: TableSettings;
    gantt?: GanttSettings;
  };
  let {
    view,
    layout,
    filter,
    kanban = DEFAULT_KANBAN,
    table = DEFAULT_TABLE,
    gantt = DEFAULT_GANTT,
  }: Props = $props();

  const scope = useListScope(() => view);

  // saveBase is the fixed part of the screen; with two Stores or more it already names the Store (D8).
  // It is empty on the screens of every Store, which combine would leave a space in front of.
  function save() {
    const query = combine(scope.saveBase, filter).trim();
    openViewSave({ draft: newDraft({ layout, query, kanban, table, gantt }), store: scope.store });
  }
</script>

<Button size="sm" disabled={!scope.ready} onclick={save}>ビューを保存</Button>
