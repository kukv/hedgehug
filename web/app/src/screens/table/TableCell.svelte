<script lang="ts">
  import { type CellView, deletedValue, type SearchColumn } from "@hedgehug/core";
  import { Avatar } from "@hedgehug/ui";

  // テーブルのセル 1 つ(04「セル」、E2a-12)。表示の中身は core の cellView が決め、ここは描き分けるだけ。
  interface Props {
    view: CellView;
    column: SearchColumn;
  }

  let { view, column }: Props = $props();
  const num = $derived(
    column.key === "points" || column.key === "estimate" || column.type === "number",
  );
</script>

<td class:num>
  {#if view.kind === "unused"}
    <span class="faint">—</span>
  {:else if view.kind === "removed"}
    <span class="faint">{deletedValue}</span>
  {:else if view.kind === "text" || view.kind === "relative"}
    {view.text}
  {:else if view.kind === "due"}
    <span
      class="due"
      class:overdue={view.emphasis === "overdue"}
      class:today={view.emphasis === "today"}
      >{view.text}</span
    >
  {:else if view.kind === "check"}
    <input type="checkbox" checked={view.checked} disabled aria-label={column.name}>
  {:else if view.kind === "url"}
    <a href={view.href} target="_blank" rel="noopener noreferrer">{view.href}</a>
  {:else if view.kind === "person"}
    <span class="with-icon"><Avatar name={view.name} title={view.name} />{view.name}</span>
  {/if}
</td>

<style>
  td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--line);
    vertical-align: middle;
    white-space: nowrap;
  }
  .num {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .faint {
    color: var(--muted);
  }
  .due {
    color: var(--muted);
    font-size: 12px;
  }
  .overdue {
    color: var(--danger);
  }
  .today {
    font-weight: 600;
  }
  .with-icon {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
</style>
