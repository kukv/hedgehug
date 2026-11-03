<!-- The tests of the screens that hold a tree stand this in for TreeView: it shows what it was given. -->
<script lang="ts">
  import type { Snippet } from "svelte";
  import type { ListScope } from "../../list/scope";

  type Props = {
    view: ListScope;
    query?: string;
    onquery?: (q: string) => void;
    cli?: string;
    actions?: Snippet;
  };
  let { view, query, onquery, cli, actions }: Props = $props();
</script>

<div data-testid="tree" data-view={JSON.stringify(view)} data-cli={cli}>
  <span data-testid="tree-query">{query}</span>
  <button type="button" onclick={() => onquery?.("sort:title")}>tree-set-sort</button>
  {#if actions}
    {@render actions()}
  {/if}
</div>
