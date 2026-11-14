<script lang="ts">
  import { averageText, type Report } from "@hedgehug/core";
  import BarChart from "./BarChart.svelte";
  import DataTable from "./DataTable.svelte";
  import { heading, num } from "./format";

  // ベロシティ(05「レポート」)。開始したスプリントがなければ、図も表も出さない。
  interface Props {
    report: Report;
  }

  let { report }: Props = $props();

  const unit = $derived(report.unit);
  const columns = $derived(["スプリント", heading("計画", unit), heading("完了", unit)]);
  const rows = $derived(
    report.velocity.map((v) => [v.sprint.name, num(v.planned, unit), num(v.done, unit)]),
  );
</script>

<section aria-label="ベロシティ">
  <h2>ベロシティ</h2>
  {#if report.velocity.length === 0}
    <p class="empty">開始したスプリントがありません</p>
  {:else}
    <p class="average">{averageText(report.average, unit)}</p>
    <BarChart items={report.velocity} {unit} />
    <DataTable caption="ベロシティの表" {columns} {rows} />
  {/if}
</section>

<style>
  section {
    margin-bottom: 28px;
  }
  h2 {
    font-size: 15px;
    margin: 0;
  }
  .average,
  .empty {
    margin: 8px 0 0;
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
