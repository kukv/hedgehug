<script lang="ts">
  import type { PaletteRow } from "../types";

  // コマンドパレットの 1 行(02「コマンドパレット」)。見出しと「ほか N 件…」は押せない。
  interface Props {
    row: PaletteRow;
    id: string;
    selected: boolean;
    // dot は、見出しの頭に付ける Store の点の色(同じ ID のアイテムが別の Store にもあるとき。A5c-9)。
    dot?: string;
    onchoose: () => void;
  }

  let { row, id, selected, dot, onchoose }: Props = $props();
</script>

{#if row.kind === "head"}
  <div class="head" role="presentation">
    {#if dot}
      <span class="store-dot" style="--dot: var(--t-{dot}-fg)"></span>
    {/if}
    {row.text}
  </div>
{:else if row.kind === "more"}
  <div class="more" role="presentation">{row.text}</div>
{:else}
  <!-- フォーカスは入力欄に置いたまま(押しても入力欄から動かさない)。選択は aria-activedescendant で示す -->
  <button
    type="button"
    role="option"
    tabindex="-1"
    {id}
    class="option"
    class:selected
    aria-selected={selected}
    aria-disabled={row.c.disabled ? "true" : undefined}
    onmousedown={(e) => e.preventDefault()}
    onclick={() => {
      if (!row.c.disabled) onchoose();
    }}
  >
    {#if row.c.checked !== undefined}
      <span class="check" aria-hidden="true">{row.c.checked ? "✓" : ""}</span>
    {/if}
    <span class="label">{row.c.label}</span>
    {#if row.c.hint}
      <span class="hint">{row.c.hint}</span>
    {/if}
  </button>
{/if}

<style>
  .head {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 10px 4px;
    font-size: 11.5px;
    color: var(--muted);
  }
  .store-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--dot);
  }
  .more {
    padding: 6px 10px;
    font-size: 12px;
    color: var(--muted);
  }
  .option {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    min-height: 34px;
    padding: 6px 10px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    text-align: left;
  }
  .option:hover:not([aria-disabled="true"]) {
    background: var(--hover);
  }
  .option.selected {
    background: var(--accent-weak);
  }
  .option[aria-disabled="true"] {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .check {
    width: 14px;
  }
  .label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hint {
    color: var(--muted);
    font-size: 11.5px;
    white-space: nowrap;
  }
</style>
