<script lang="ts">
  import { type LinkRow, linkText, linkUrl } from "@hedgehug/core";
  import { IconButton } from "@hedgehug/ui";
  import LinkChips from "./LinkChips.svelte";

  // リンクの 1 行: チップ、「{owner/name}#{番号} — {タイトル}」(GitHub を新しいタブで開く)、解除の「×」。
  // 取り直しに失敗した行は、その下に理由を赤字で出す(07「表示と更新」)。
  interface Props {
    row: LinkRow;
    disabled: boolean;
    title?: string;
    onremove: () => void;
  }

  let { row, disabled, title, onremove }: Props = $props();
</script>

<div class="link-row">
  <LinkChips {row} />
  <a class="row-title" href={linkUrl(row)} target="_blank" rel="noopener noreferrer"
    >{linkText(row)}</a
  >
  <IconButton
    label="{row.repo}#{row.number} のリンクを解除"
    size="xs"
    {disabled}
    {title}
    onclick={onremove}
    >×</IconButton
  >
</div>
{#if row.error}
  <p class="row-error" role="alert">{row.error}</p>
{/if}

<style>
  .link-row {
    display: flex;
    align-items: center;
    gap: 6px;
    min-height: 30px;
    border-bottom: 1px solid var(--line);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
    overflow-wrap: anywhere;
  }
  .row-title:hover {
    text-decoration: underline;
  }
  .row-error {
    color: var(--danger);
    font-size: 12.5px;
    margin: 0;
  }
</style>
