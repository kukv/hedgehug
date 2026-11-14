<script lang="ts">
  import { addedText, notStarted, type ReportActive } from "@hedgehug/core";
  import DataTable from "../reports/DataTable.svelte";
  import { burndownTable } from "../reports/format";
  import LineChart from "../reports/LineChart.svelte";

  // 実行中のスプリントのカードの小さいバーンダウン・「途中で追加」・「表で見る」(05)。
  // 表示する日がないとき(開始日が今日より後)は、図も「途中で追加」も出さない(F3-10)。
  interface Props {
    active: ReportActive;
    start: string;
    thisYear: number;
  }

  let { active, start, thisYear }: Props = $props();

  const b = $derived(active.burndown);
  const unit = $derived(active.unit);
  const table = $derived(burndownTable(b.days, unit, thisYear));
</script>

{#if b.shown === 0}
  <p class="note">{notStarted(start, thisYear)}</p>
{:else}
  {#if active.added > 0}
    <p class="note">{addedText(active.added, unit)}</p>
  {/if}
  <LineChart days={b.days} {unit} {thisYear} compact />
  <DataTable caption="バーンダウンの表" columns={table.columns} rows={table.rows} />
{/if}

<style>
  .note {
    margin: 0;
    font-size: 12px;
    color: var(--muted);
  }
</style>
