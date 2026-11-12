<script lang="ts">
  import Button from "../primitives/Button.svelte";
  import type { SyncCardView } from "../types";

  // サイドバーの同期カード(06「同期の状態の表示」)。要約を押すと「Store ごとの同期状態」が開く。
  interface Props {
    card: SyncCardView;
    // hint は、「すべて同期」の案内(Shift+Ctrl+S か Shift+Cmd+S)。
    hint: string;
    onsyncall: () => void;
    onsyncstore: (storeId: string) => void;
  }

  let { card, hint, onsyncall, onsyncstore }: Props = $props();
  let open = $state(false);
  let element: HTMLDivElement | undefined = $state();

  function choose(storeId: string) {
    open = false;
    onsyncstore(storeId);
  }

  function onkeydown(event: KeyboardEvent) {
    if (open && event.key === "Escape") open = false;
  }

  function onpointerdown(event: PointerEvent) {
    if (open && element && !element.contains(event.target as Node)) open = false;
  }
</script>

<svelte:window {onkeydown} {onpointerdown} />

<div class="sync-card" bind:this={element}>
  <div class="row">
    <button
      type="button"
      class="summary"
      class:error={card.error}
      aria-haspopup="menu"
      aria-expanded={open}
      onclick={() => (open = !open)}
    >
      {card.summary}
    </button>
    <Button size="sm" title={hint} onclick={onsyncall}>すべて同期</Button>
  </div>
  <p class="line">{card.last}</p>
  {#if card.stale}
    <p class="line warn">{card.stale}</p>
  {/if}
  {#if open}
    <div class="menu" role="menu" aria-label="Store ごとの同期状態">
      {#each card.rows as row (row.storeId)}
        <button type="button" role="menuitem" class="item" onclick={() => choose(row.storeId)}>
          <span class="name">{row.name}</span>
          <span class="state">{row.summary} · <span class:warn={row.stale}>{row.time}</span></span>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .sync-card {
    position: relative;
    margin-top: auto;
    padding: 10px 8px;
    border-top: 1px solid var(--line);
    font-size: 12px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .summary {
    flex: 1;
    min-width: 0;
    text-align: left;
    border: 0;
    background: none;
    padding: 2px 0;
    color: var(--ink);
    font-weight: 500;
    cursor: pointer;
  }
  .summary.error {
    color: var(--danger);
  }
  .line {
    margin: 2px 0 0;
    color: var(--muted);
  }
  .warn {
    color: var(--c-amber-fg);
  }
  .menu {
    position: absolute;
    left: 8px;
    right: 8px;
    bottom: 100%;
    display: flex;
    flex-direction: column;
    padding: 4px;
    background: var(--raised);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: var(--shadow-pop);
    z-index: 40;
  }
  .item {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    padding: 6px 8px;
    border: 0;
    border-radius: 6px;
    background: none;
    color: var(--ink);
    text-align: left;
    cursor: pointer;
  }
  .item:hover {
    background: var(--hover);
  }
  .name {
    font-weight: 500;
  }
  .state {
    color: var(--muted);
  }
</style>
