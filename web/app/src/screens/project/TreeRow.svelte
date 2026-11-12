<script lang="ts">
  import { itemDisplayId, type TreeNode } from "@hedgehug/core";
  import { KindIcon } from "@hedgehug/ui";
  import Self from "./TreeRow.svelte";

  // ツリーの 1 行と、その下の行(04「ツリー」の「行」のうち、A2 の計画の決定 17 の分)。
  interface Props {
    node: TreeNode;
    prefix: string;
    depth: number;
    collapsed: Set<string>;
    ontoggle: (id: string) => void;
  }

  let { node, prefix, depth, collapsed, ontoggle }: Props = $props();

  const it = $derived(node.item);
  const open = $derived(!collapsed.has(it.id));
</script>

<li class="tree-row" style:--d={depth}>
  {#if node.children.length > 0}
    <button
      type="button"
      class="twisty"
      class:open
      aria-label="開閉"
      aria-expanded={open}
      onclick={() => ontoggle(it.id)}
    >
      ›
    </button>
  {:else}
    <span class="twisty-space"></span>
  {/if}
  <KindIcon kind={it.kind} />
  <code class="id">{itemDisplayId(prefix, it.number)}</code>
  <span class="row-title">{it.title}</span>
</li>
{#if open}
  {#each node.children as c (c.item.id)}
    <Self node={c} {prefix} depth={depth + 1} {collapsed} {ontoggle} />
  {/each}
{/if}

<style>
  .tree-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    padding: 2px 8px 2px calc(8px + var(--d) * 22px);
    border-bottom: 1px solid var(--line);
  }
  .twisty,
  .twisty-space {
    width: 18px;
    height: 18px;
    flex: none;
  }
  .twisty {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    transition: transform 0.12s;
  }
  .twisty.open {
    transform: rotate(90deg);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
