<script lang="ts">
  import {
    categoryOf,
    dueLabel,
    type Item,
    stepProgress,
    type WorkspaceSettings,
  } from "@hedgehug/core";
  import { BlockedBadge } from "@hedgehug/ui";

  // ツリーの行の、タイトルとステータスのボタンのあいだ(ブロック中・Step の進捗・期限・ポイント)。
  // TreeRow.svelte が 200 行の上限に近いので分けた。要素は行の直接の子として並ぶ(包む要素はない)。
  interface Props {
    it: Item;
    items: Item[];
    settings: WorkspaceSettings;
    today: Date;
  }

  let { it, items, settings, today }: Props = $props();

  const progress = $derived(
    it.kind === "Epic" || it.kind === "Step" ? null : stepProgress(items, it.id, settings),
  );
  const due = $derived(dueLabel(it.end, categoryOf(settings, it.status), today));
</script>

{#if it.blocked}
  <BlockedBadge />
{/if}
{#if progress}
  <span class="progress">{progress.done}/{progress.total}</span>
{/if}
{#if due}
  <span
    class="due"
    class:overdue={due.emphasis === "overdue"}
    class:today={due.emphasis === "today"}
    >{due.text}</span
  >
{/if}
{#if it.points !== null}
  <span class="points">{it.points} pt</span>
{/if}

<style>
  .progress,
  .due,
  .points {
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
  .overdue {
    color: var(--danger);
  }
  .today {
    font-weight: 600;
  }
</style>
