<script lang="ts">
  import { type ChildForm, childKinds, type SettingsState } from "@hedgehug/core";
  import { Button, IconButton, Input, Select } from "@hedgehug/ui";
  import TemplateChildDialog from "./TemplateChildDialog.svelte";

  // Epic のテンプレートの子の一覧(03「一覧と編集」)。タイトルが空の行は、保存するときに捨てる。
  interface Props {
    children: ChildForm[];
    state: SettingsState;
    disabled: boolean;
    title?: string;
    // 子のダイアログを開いている子の位置。親のダイアログは、開いている間の Escape では閉じない。
    open?: number | null;
  }

  let {
    children = $bindable(),
    state: s,
    disabled,
    title,
    open = $bindable(null),
  }: Props = $props();

  const kindOptions = childKinds.map((k) => ({ value: k, label: k }));

  function move(i: number, to: number) {
    const next = [...children];
    const a = next[i];
    const b = next[to];
    if (a === undefined || b === undefined) return;
    next[i] = b;
    next[to] = a;
    children = next;
  }

  function add() {
    children = [
      ...children,
      { id: "", kind: "Task", title: "", description: "", values: {}, steps: [] },
    ];
  }
</script>

<div class="list">
  <span class="k">子</span>
  {#each children as child, i (child)}
    <div class="row">
      <Select
        label="子の種類"
        size="sm"
        options={kindOptions}
        {disabled}
        {title}
        bind:value={child.kind as string}
      />
      <span class="grow">
        <Input label="子のタイトル" size="sm" {disabled} {title} bind:value={child.title} />
      </span>
      <Button size="sm" onclick={() => (open = i)}>編集</Button>
      <IconButton
        label="上へ"
        size="sm"
        disabled={disabled || i === 0}
        {title}
        onclick={() => move(i, i - 1)}
        >↑</IconButton
      >
      <IconButton
        label="下へ"
        size="sm"
        disabled={disabled || i === children.length - 1}
        {title}
        onclick={() => move(i, i + 1)}
        >↓</IconButton
      >
      <IconButton
        label="削除"
        size="sm"
        {disabled}
        {title}
        onclick={() => (children = children.filter((_, j) => j !== i))}
        >×</IconButton
      >
    </div>
  {/each}
  <div>
    <Button size="sm" {disabled} {title} onclick={add}>子を追加</Button>
  </div>
</div>

{#if open !== null && children[open]}
  {@const i = open}
  <TemplateChildDialog
    bind:child={() => children[i] as ChildForm, (c) => (children[i] = c)}
    state={s}
    {disabled}
    {title}
    onclose={() => (open = null)}
  />
{/if}

<style>
  .list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .k {
    font-size: 12.5px;
    color: var(--ink-2);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .grow {
    flex: 1;
    display: flex;
  }
  .grow :global(input) {
    flex: 1;
  }
</style>
