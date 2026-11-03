<!-- The tests of the screens that hold a table stand this in for TableView. -->
<script lang="ts">
  import type { Snippet } from "svelte";
  import type { TableSettings } from "../../../core/views";
  import type { ListScope } from "../../list/scope";

  type Props = {
    view: ListScope;
    query: string;
    onquery: (q: string) => void;
    settings: TableSettings;
    onsettings: (s: TableSettings) => void;
    cli?: string;
    saved?: { name: string; store: string };
    actions?: Snippet;
  };
  let { view, query, onquery, settings, onsettings, cli, saved, actions }: Props = $props();
</script>

<div
  data-testid="table"
  data-view={JSON.stringify(view)}
  data-cli={cli}
  data-saved={saved ? JSON.stringify(saved) : undefined}
>
  <span data-testid="table-query">{query}</span>
  <span data-testid="table-settings">{settings.columns.join(",")}/{settings.groupBy}</span>
  <button type="button" onclick={() => onquery("assignee:@me")}>table-set-query</button>
  <button type="button" onclick={() => onsettings({ ...settings, groupBy: "parent" })}>
    table-set-group
  </button>
  {#if actions}
    {@render actions()}
  {/if}
</div>
