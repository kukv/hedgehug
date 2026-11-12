<script lang="ts">
  import type { MenuItem } from "../types";

  interface Props {
    open: boolean;
    x: number;
    y: number;
    items: MenuItem[];
    onselect: (id: string) => void;
    onclose: () => void;
  }

  let { open, x, y, items, onselect, onclose }: Props = $props();
  let element: HTMLDivElement | undefined = $state();

  function onkeydown(event: KeyboardEvent) {
    if (open && event.key === "Escape") onclose();
  }

  // メニューの外を押したら閉じる。
  function onpointerdown(event: PointerEvent) {
    if (open && element && !element.contains(event.target as Node)) onclose();
  }
</script>

<svelte:window {onkeydown} {onpointerdown} />

{#if open}
  <div class="menu-pop" role="menu" style:left="{x}px" style:top="{y}px" bind:this={element}>
    {#each items as item, i (i)}
      {#if "separator" in item}
        <hr class="menu-sep">
      {:else}
        <button
          type="button"
          role="menuitem"
          class="menu-item"
          class:danger={item.danger}
          disabled={item.disabled}
          onclick={() => !item.disabled && onselect(item.id)}
        >
          <span class="menu-label">{item.label}</span>
          {#if item.hint}
            <span class="menu-hint">{item.hint}</span>
          {/if}
        </button>
      {/if}
    {/each}
  </div>
{/if}

<style>
  .menu-pop {
    position: fixed;
    z-index: 60;
    background: var(--raised);
    border: 1px solid var(--line);
    border-radius: 10px;
    box-shadow: var(--shadow-pop);
    padding: 4px;
    display: flex;
    flex-direction: column;
    max-height: 420px;
    overflow-y: auto;
  }
  .menu-item {
    display: flex;
    align-items: center;
    gap: 8px;
    border: 0;
    background: transparent;
    padding: 6px 8px;
    border-radius: 6px;
    text-align: left;
    min-height: 32px;
  }
  .menu-item:hover:not(:disabled) {
    background: var(--hover);
  }
  .menu-item:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .danger {
    color: var(--danger);
  }
  .menu-label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .menu-hint {
    color: var(--muted);
    font-size: 11.5px;
    white-space: nowrap;
  }
  .menu-sep {
    height: 1px;
    background: var(--line);
    margin: 4px 0;
  }
</style>
