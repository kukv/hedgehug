<script lang="ts">
  import { type Member, type StepForm, stepAssigneeOptions } from "@hedgehug/core";
  import { Button, IconButton, Input, Select } from "@hedgehug/ui";

  // テンプレートの Step の一覧(03「一覧と編集」)。タイトルが空の行は、保存するときに捨てる。
  interface Props {
    steps: StepForm[];
    members: Member[];
    disabled: boolean;
    title?: string;
  }

  let { steps = $bindable(), members, disabled, title }: Props = $props();

  const options = $derived(stepAssigneeOptions(members));

  function move(i: number, to: number) {
    const next = [...steps];
    const a = next[i];
    const b = next[to];
    if (a === undefined || b === undefined) return;
    next[i] = b;
    next[to] = a;
    steps = next;
  }
</script>

<div class="list">
  <span class="k">Step</span>
  {#each steps as step, i (step)}
    <div class="row">
      <span class="grow">
        <Input label="Step のタイトル" size="sm" {disabled} {title} bind:value={step.title} />
      </span>
      <Select label="担当者" size="sm" {options} {disabled} {title} bind:value={step.assignee} />
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
        disabled={disabled || i === steps.length - 1}
        {title}
        onclick={() => move(i, i + 1)}
        >↓</IconButton
      >
      <IconButton
        label="削除"
        size="sm"
        {disabled}
        {title}
        onclick={() => (steps = steps.filter((_, j) => j !== i))}
        >×</IconButton
      >
    </div>
  {/each}
  <div>
    <Button
      size="sm"
      {disabled}
      {title}
      onclick={() => (steps = [...steps, { id: "", title: "", assignee: "@parent" }])}
      >Step を追加</Button
    >
  </div>
</div>

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
