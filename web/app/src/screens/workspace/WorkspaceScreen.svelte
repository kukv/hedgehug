<script lang="ts">
  import { readOnlyBand, type StoreNode, type WorkspaceNode } from "@hedgehug/core";
  import { PageHead, ReadOnlyBand, StoreBadge } from "@hedgehug/ui";

  // ワークスペースの画面の見出しだけの枠(A2 の計画の持ち場)。タブの中身はトラック B・E・F が足す。
  interface Props {
    store: StoreNode;
    workspace: WorkspaceNode;
    ondrawer: () => void;
  }

  let { store, workspace, ondrawer }: Props = $props();
</script>

<PageHead title={workspace.name} {ondrawer}>
  {#snippet crumbs()}
    <StoreBadge name={store.name} color={store.color} />
    <code class="prefix">{workspace.prefix}</code>
  {/snippet}
</PageHead>
<div class="content">
  {#if store.readOnly}
    <ReadOnlyBand text={readOnlyBand(store.name)} />
  {/if}
</div>

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  .prefix {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
</style>
