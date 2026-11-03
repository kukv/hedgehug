<!-- The tests of the screens that hold a kanban stand this in for KanbanView. -->
<script lang="ts">
  import type { Snippet } from "svelte";
  import type { KanbanSettings } from "../../../core/views";
  import type { ListScope } from "../../list/scope";

  type Props = {
    view: ListScope;
    query: string;
    onquery: (q: string) => void;
    settings: KanbanSettings;
    onsettings: (s: KanbanSettings) => void;
    cli?: string;
    saved?: { name: string; store: string };
    actions?: Snippet;
  };
  let { view, query, onquery, settings, onsettings, cli, saved, actions }: Props = $props();
</script>

<div
  data-testid="kanban"
  data-view={JSON.stringify(view)}
  data-cli={cli}
  data-saved={saved ? JSON.stringify(saved) : undefined}
>
  <span data-testid="kanban-query">{query}</span>
  <span data-testid="kanban-settings">{settings.columns}/{settings.swimlane}</span>
  <button type="button" onclick={() => onquery("assignee:@me")}>kanban-set-query</button>
  <button type="button" onclick={() => onsettings({ ...settings, swimlane: "assignee" })}>
    kanban-set-swimlane
  </button>
  {#if actions}
    {@render actions()}
  {/if}
</div>
