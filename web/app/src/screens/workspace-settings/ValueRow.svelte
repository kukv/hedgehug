<script lang="ts">
  import {
    type Category,
    categoryLabels,
    type ValueRow as Row,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, IconButton, Input, Select } from "@hedgehug/ui";
  import ColorMenu from "./ColorMenu.svelte";

  // 値の 1 行(03「値の編集」)。入力の途中の名前だけを持ち、操作は親が core の run に渡す。
  interface Props {
    row: Row;
    list: string;
    readOnly: boolean;
    onrename: (name: string) => void;
    oncolor: (color: string) => void;
    oncategory: (category: Category) => void;
    oninitial: () => void;
    onmove: (direction: "up" | "down") => void;
    ondelete: () => void;
  }

  let { row, list, readOnly, onrename, oncolor, oncategory, oninitial, onmove, ondelete }: Props =
    $props();

  let name = $state("");
  // 読み直しのたびに、入力を保存済みの値に戻す(断りのときの巻き戻しもこれ)。
  $effect(() => {
    name = row.name;
  });

  const title = $derived(readOnly ? readOnlyNote : undefined);
  const categoryOptions = (Object.keys(categoryLabels) as Category[]).map((c) => ({
    value: c,
    label: categoryLabels[c],
  }));
  const initialLabel = $derived(list === "statuses" ? "初期ステータスにする" : "初期優先度にする");

  function commit() {
    if (name !== row.name) onrename(name);
  }
  function key(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) (event.target as HTMLElement).blur();
  }
</script>

<div class="row">
  <ColorMenu color={row.color} name={row.name} disabled={readOnly} {title} onselect={oncolor} />
  <div class="name">
    <Input
      label="名前"
      bind:value={name}
      disabled={readOnly}
      {title}
      onblur={commit}
      onkeydown={key}
    />
  </div>
  {#if row.category}
    <Select
      label="{row.name}の分類"
      size="sm"
      bind:value={() => row.category ?? "todo", (c) => oncategory(c as Category)}
      options={categoryOptions}
      disabled={readOnly}
      {title}
    />
  {/if}
  {#if list === "statuses" || list === "priorities"}
    {#if row.initial}
      <span class="mark">初期</span>
    {:else}
      <Button
        size="xs"
        disabled={readOnly || row.initialDisabled !== undefined}
        title={readOnly ? readOnlyNote : row.initialDisabled}
        onclick={oninitial}
        >{initialLabel}</Button
      >
    {/if}
  {/if}
  <span class="count">{row.count} 件</span>
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
  <IconButton label="削除" size="sm" disabled={readOnly} {title} onclick={ondelete}>🗑</IconButton>
</div>

<style>
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
  }
  .name {
    flex: 1;
    min-width: 0;
    display: flex;
  }
  .name :global(input) {
    flex: 1;
  }
  .mark {
    font-size: 11.5px;
    color: var(--accent);
    border: 1px solid var(--accent);
    border-radius: 999px;
    padding: 1px 8px;
    white-space: nowrap;
  }
  .count {
    font-size: 12.5px;
    color: var(--muted);
    min-width: 40px;
    text-align: right;
  }
</style>
