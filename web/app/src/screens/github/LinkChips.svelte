<script lang="ts">
  import {
    checksLabel,
    kindLabel,
    type LinkRow,
    reviewLabel,
    showsProgress,
    stateLabel,
  } from "@hedgehug/core";

  // リンクの行のチップ(07「表示と更新」): 種別と状態、Open・Draft の PR だけレビューとチェック(なしは出さない)。
  interface Props {
    row: LinkRow;
  }

  let { row }: Props = $props();
  const review = $derived(showsProgress(row) ? reviewLabel(row.review) : "");
  const checks = $derived(showsProgress(row) ? checksLabel(row.checks) : "");
</script>

<span class="chip {row.state}">{kindLabel(row.kind)} {stateLabel[row.state]}</span>
{#if review}
  <span class="chip">{review}</span>
{/if}
{#if checks}
  <span class="chip {row.checks}">{checks}</span>
{/if}

<style>
  .chip {
    padding: 0 6px;
    border-radius: 9px;
    font-size: 11.5px;
    white-space: nowrap;
    background: var(--sunken);
    color: var(--ink);
  }
  .open,
  .success {
    color: var(--success);
  }
  .merged {
    color: var(--accent-ink);
  }
  .closed,
  .failure {
    color: var(--danger);
  }
</style>
