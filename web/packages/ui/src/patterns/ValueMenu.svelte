<script lang="ts">
  import type { ValueOption } from "../types";

  // 値を選ぶメニュー(02「入力の方法と保存」)。いまの値にチェックを付け、7 件以上で検索欄を出す。
  // multi なら選んでも閉じない(関連システム・複数選択)。外を押すか Esc で閉じる(Menu と同じ)。
  interface Props {
    open: boolean;
    x: number;
    y: number;
    label: string;
    options: ValueOption[];
    multi?: boolean;
    onselect: (id: string) => void;
    onclose: () => void;
  }

  let { open, x, y, label, options, multi = false, onselect, onclose }: Props = $props();
  let element: HTMLDivElement | undefined = $state();
  let innerHeight = $state(0);
  let height = $state(0);
  let query = $state("");
  const shown = $derived(
    options.filter((o) => o.label.toLowerCase().includes(query.trim().toLowerCase())),
  );

  // 画面の下端の近くで開いたら、メニューの下が画面の外に出ないよう上へ寄せる(Menu と同じ)。
  $effect(() => {
    // 検索で件数が変わると高さも変わるので、shown を読んで測り直す。
    void shown;
    height = element?.offsetHeight ?? 0;
  });
  const top = $derived(Math.max(0, Math.min(y, innerHeight - height - 4)));

  $effect(() => {
    if (!open) query = "";
  });

  function onkeydown(event: KeyboardEvent) {
    if (open && event.key === "Escape" && !event.isComposing) onclose();
  }
  function onpointerdown(event: PointerEvent) {
    if (open && element && !element.contains(event.target as Node)) onclose();
  }
  function choose(id: string) {
    onselect(id);
    if (!multi) onclose();
  }
</script>

<svelte:window {onkeydown} {onpointerdown} bind:innerHeight />

{#if open}
  <div
    class="menu-pop"
    role="menu"
    aria-label={label}
    style:left="{x}px"
    style:top="{top}px"
    bind:this={element}
  >
    {#if options.length >= 7}
      <!-- 「検索」は読み上げの名前で、画面に出る文言ではない -->
      <input class="menu-search" type="search" aria-label="検索" bind:value={query}>
    {/if}
    {#each shown as o (o.id)}
      <button
        type="button"
        role="menuitemcheckbox"
        aria-checked={o.checked}
        class="menu-item"
        disabled={o.disabled}
        title={o.reason}
        onclick={() => {
          // 押せない間は onselect を呼ばない(合成イベントでも)。
          if (!o.disabled) choose(o.id);
        }}
      >
        <span class="check" aria-hidden="true">{o.checked ? "✓" : ""}</span>
        <span class="menu-label">{o.label}</span>
        {#if o.hint}
          <span class="menu-hint">{o.hint}</span>
        {/if}
        {#if o.reason}
          <span class="menu-reason">{o.reason}</span>
        {/if}
      </button>
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
    flex-wrap: wrap;
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
  .menu-reason {
    flex-basis: 100%;
    font-size: 11px;
    color: var(--muted);
    white-space: normal;
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
  .check {
    width: 14px;
    display: inline-block;
  }
  .menu-search {
    width: 100%;
    margin-bottom: 4px;
  }
</style>
