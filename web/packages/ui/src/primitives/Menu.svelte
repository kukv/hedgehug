<script lang="ts">
  import type { Snippet } from "svelte";
  import type { MenuItem } from "../types";
  import { menuFocus } from "./menuFocus";

  interface Props {
    open: boolean;
    x: number;
    y: number;
    items: MenuItem[];
    onselect: (id: string) => void;
    onclose: () => void;
    // head は、項目の上に置く欄(「移動…」の検索欄など)。項目をスクロールしても上に残る。
    head?: Snippet;
  }

  let { open, x, y, items, onselect, onclose, head }: Props = $props();
  let element: HTMLDivElement | undefined = $state();
  let innerHeight = $state(0);
  let height = $state(0);

  // 画面の下端の近くで開いたら、メニューの下が画面の外に出ないよう上へ寄せる。
  $effect(() => {
    height = element?.offsetHeight ?? 0;
  });
  const top = $derived(Math.max(0, Math.min(y, innerHeight - height - 4)));

  // shownFrom は、開く前のフォーカス(閉じたら戻す先)。head の欄は出たときにフォーカスを取るので、その前に読む。
  let shownFrom = $state.raw<Element | null>(null);
  $effect.pre(() => {
    if (open) shownFrom = document.activeElement;
  });

  function onkeydown(event: KeyboardEvent) {
    if (open && event.key === "Escape") onclose();
  }

  // メニューの外を押したら閉じる。
  function onpointerdown(event: PointerEvent) {
    if (open && element && !element.contains(event.target as Node)) onclose();
  }
</script>

<svelte:window {onkeydown} {onpointerdown} bind:innerHeight />

{#if open}
  <!-- role="menu" の中に置けるのは項目だけなので、head(検索欄など)は同じ浮いた枠の中で、その外に置く。
       data-menu は、head も含めて「メニューの中」と見分けるための印(ツリーのキー操作が使う) -->
  <div
    class="menu-pop"
    data-menu
    style:left="{x}px"
    style:top="{top}px"
    bind:this={element}
    use:menuFocus={shownFrom}
  >
    {#if head}
      <div class="menu-head">{@render head()}</div>
    {/if}
    <div class="menu-list" role="menu">
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
            onclick={() => {
              // 押せない間は onselect を呼ばない(合成イベントでも)。
              if (!item.disabled) onselect(item.id);
            }}
          >
            <span class="menu-label">{item.label}</span>
            {#if item.hint}
              <span class="menu-hint">{item.hint}</span>
            {/if}
          </button>
        {/if}
      {/each}
    </div>
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
  .menu-list {
    display: flex;
    flex-direction: column;
  }
  .menu-head {
    position: sticky;
    top: -4px;
    z-index: 1;
    min-width: 260px;
    padding: 2px 2px 6px;
    background: var(--raised);
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
    border: 0;
    background: var(--line);
    margin: 4px 0;
  }
</style>
