<script lang="ts" module>
  import type { IconName } from "../icons/Icon.svelte";

  export type TabItem = { label: string; href: string; current: boolean };
  /** The tab at the right end (the settings), which can have an icon. */
  export type TabEnd = TabItem & { icon?: IconName };
</script>

<script lang="ts">
  import Icon from "../icons/Icon.svelte";
  import { onLinkClick } from "../router";

  type Props = { items: TabItem[]; end?: TabEnd; label?: string };
  let { items, end, label = "ビューの切り替え" }: Props = $props();
</script>

<!-- The tabs are links, so the back button, copying the address and ⌘/Ctrl-click work. -->
<nav class="tabs" aria-label={label}>
  {#each items as it (it.href)}
    <a
      class="tab"
      class:current={it.current}
      href={it.href}
      aria-current={it.current ? "page" : undefined}
      onclick={(e) => onLinkClick(e, it.href)}
      >{it.label}</a
    >
  {/each}
  {#if end}
    <a
      class="tab end"
      class:current={end.current}
      href={end.href}
      aria-current={end.current ? "page" : undefined}
      onclick={(e) => onLinkClick(e, end.href)}
    >
      {#if end.icon}
        <Icon name={end.icon} size={14} />
      {/if}
      {end.label}
    </a>
  {/if}
</nav>

<style>
  .tabs {
    display: flex;
    gap: var(--space-4);
    padding: var(--space-10) var(--gutter) 0;
    border-bottom: var(--hairline) solid var(--line);
  }
  .tab {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    min-height: 2.25rem;
    padding: 0 var(--space-12);
    margin-bottom: calc(var(--hairline) * -1);
    border-bottom: 2px solid transparent;
    color: var(--muted);
    font-size: var(--fs-13);
    text-decoration: none;
  }
  .tab:hover {
    color: var(--ink);
  }
  .tab.current {
    border-bottom-color: var(--ink);
    color: var(--ink);
    font-weight: 600;
  }
  .end {
    margin-left: auto;
  }
</style>
