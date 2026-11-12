<script lang="ts">
  import { type PropertyRow, readOnlyNote } from "@hedgehug/core";
  import { Button, IconButton, Input, KindIcon, type KindName } from "@hedgehug/ui";
  import KindsMenu from "./KindsMenu.svelte";

  // プロパティの表の 1 行(03「一覧」)。入力の途中の名前だけを持ち、操作は親が core の run に渡す。
  interface Props {
    row: PropertyRow;
    readOnly: boolean;
    onrename: (name: string) => void;
    onkind: (kind: string) => Promise<unknown>;
    onhide: () => void;
    onmove: (direction: "up" | "down") => void;
    ondelete: () => void;
    onchoices: () => void;
  }

  let { row, readOnly, onrename, onkind, onhide, onmove, ondelete, onchoices }: Props = $props();

  let name = $state("");
  let at = $state<{ x: number; y: number } | null>(null);
  // 読み直しのたびに、入力を保存済みの値に戻す(断りのときの巻き戻しもこれ)。
  $effect(() => {
    name = row.name;
  });

  const title = $derived(readOnly ? readOnlyNote : undefined);

  function commit() {
    if (name !== row.name) onrename(name);
  }
  function key(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) (event.target as HTMLElement).blur();
  }
  function openKinds(event: MouseEvent) {
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    at = { x: rect.left, y: rect.bottom + 4 };
  }
</script>

<div class="row" class:dim={row.hidden}>
  <Input
    label="名前"
    bind:value={name}
    disabled={readOnly}
    {title}
    onblur={commit}
    onkeydown={key}
  />
  <span class="cell">
    <code>{row.key}</code>
    {#if row.builtin}
      <span class="tag">組み込み</span>
    {/if}
  </span>
  <span class="cell">
    {row.type}
    {#if row.choices !== undefined}
      <Button size="xs" onclick={onchoices}>選択肢 {row.choices}</Button>
    {/if}
  </span>
  <span class="cell">
    {#if row.kindsFixed}
      すべて
    {:else}
      <button type="button" class="kinds-btn" aria-label="{row.name}の使う種類" onclick={openKinds}>
        {#if row.kinds === "all"}
          すべて
        {:else}
          {#each row.kinds as kind (kind)}
            <KindIcon kind={kind as KindName} />
          {/each}
        {/if}
      </button>
    {/if}
  </span>
  <span class="cell">
    <IconButton
      label="上へ"
      size="sm"
      disabled={readOnly || row.first}
      {title}
      onclick={() => onmove("up")}
      >↑</IconButton
    >
    <IconButton
      label="下へ"
      size="sm"
      disabled={readOnly || row.last}
      {title}
      onclick={() => onmove("down")}
      >↓</IconButton
    >
    {#if row.hide}
      <Button size="xs" disabled={readOnly} {title} onclick={onhide}>{row.hide}</Button>
    {/if}
    {#if row.deletable}
      <IconButton label="削除" size="sm" disabled={readOnly} {title} onclick={ondelete}
        >🗑</IconButton
      >
    {/if}
  </span>
</div>
{#if at}
  <KindsMenu
    x={at.x}
    y={at.y}
    kinds={row.kinds}
    {readOnly}
    ontoggle={onkind}
    onclose={() => (at = null)}
  />
{/if}

<style>
  .row {
    display: grid;
    grid-template-columns: minmax(140px, 1.4fr) minmax(120px, 1fr) minmax(140px, 1fr) minmax(
        80px,
        0.8fr
      ) auto;
    align-items: center;
    gap: 12px;
    padding: 4px 0;
  }
  .dim {
    opacity: 0.55;
  }
  .cell {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12.5px;
  }
  .tag {
    color: var(--muted);
    font-size: 11.5px;
  }
  code {
    font-family: var(--font-mono);
  }
  .kinds-btn {
    display: inline-flex;
    gap: 3px;
    border: 1px solid var(--line-input);
    border-radius: 6px;
    background: transparent;
    padding: 2px 6px;
    min-height: 24px;
  }
</style>
