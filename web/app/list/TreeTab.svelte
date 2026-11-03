<script lang="ts">
  import TreeView from "../tree/TreeView.svelte";
  import SaveViewButton from "./SaveViewButton.svelte";
  import type { ListScope } from "./scope";

  // The tree tab of a workspace or a project. The filter lives in the page that holds the tabs, so
  // that it stays when the tab changes, and "ビューを保存" can save what the field holds.
  type Props = {
    view: Extract<ListScope, { kind: "workspace" | "project" }>;
    query: string;
    onquery: (q: string) => void;
  };
  let { view, query, onquery }: Props = $props();
</script>

<TreeView {view} {query} {onquery}>
  {#snippet actions()}
    <SaveViewButton {view} layout="tree" filter={query} />
  {/snippet}
</TreeView>
