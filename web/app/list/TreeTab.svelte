<script lang="ts">
  import { NO_CANCELED } from "../tree/filter";
  import TreeView from "../tree/TreeView.svelte";
  import SaveViewButton from "./SaveViewButton.svelte";
  import type { ListScope } from "./scope";

  // The tree tab of a workspace or a project. The filter lives here, so that "ビューを保存" can
  // save what the field holds (ListTab does the same for the kanban and the table).
  type Props = { view: Extract<ListScope, { kind: "workspace" | "project" }> };
  let { view }: Props = $props();

  let query = $state(NO_CANCELED);
</script>

<TreeView {view} {query} onquery={(q) => (query = q)}>
  {#snippet actions()}
    <SaveViewButton {view} layout="tree" filter={query} />
  {/snippet}
</TreeView>
