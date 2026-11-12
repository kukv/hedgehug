<script lang="ts">
  import type { Tab } from "../types";

  interface Props {
    tabs: Tab[];
    current: string;
    onselect: (id: string) => void;
  }

  let { tabs, current, onselect }: Props = $props();
</script>

<div class="tabs" role="tablist">
  {#each tabs as tab (tab.id)}
    <button
      type="button"
      role="tab"
      class="tab"
      class:on={tab.id === current}
      aria-selected={tab.id === current}
      onclick={() => onselect(tab.id)}
    >
      {tab.label}
      {#if tab.count !== undefined}
        <span class="count">{tab.count}</span>
      {/if}
    </button>
  {/each}
</div>

<style>
  .tabs {
    display: flex;
    gap: 2px;
    padding: 0 24px;
    border-bottom: 1px solid var(--line);
    overflow-x: auto;
    scrollbar-width: none;
    flex: none;
  }
  .tab {
    border: 0;
    background: transparent;
    padding: 8px 10px 9px;
    color: var(--muted);
    border-bottom: 2px solid transparent;
    margin-bottom: -1px;
    white-space: nowrap;
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .tab:hover {
    color: var(--ink);
  }
  .on {
    color: var(--ink);
    border-bottom-color: var(--accent);
    font-weight: 500;
  }
  .count {
    font-size: 11px;
    background: var(--accent);
    color: var(--on-accent);
    border-radius: 999px;
    padding: 0 6px;
    font-variant-numeric: tabular-nums;
  }
  @container frame (max-width: 860px) {
    .tabs {
      padding: 0 max(16px, env(safe-area-inset-right, 0px)) 0
        max(16px, env(safe-area-inset-left, 0px));
    }
  }
</style>
