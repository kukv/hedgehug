<script lang="ts">
  import {
    burnStatus,
    burnTotals,
    committedNote,
    formatDay,
    notStarted,
    type Report,
    type ReportSelection,
  } from "@hedgehug/core";
  import { Select } from "@hedgehug/ui";
  import DataTable from "./DataTable.svelte";
  import { heading, num } from "./format";
  import LineChart from "./LineChart.svelte";

  // バーンダウン(05「レポート」)。スプリントを選んでいないときと、表示する日がないときは、図も表も出さない。
  interface Props {
    report: Report;
    today: string;
    thisYear: number;
    onselect: (patch: Partial<ReportSelection>) => void;
  }

  let { report, today, thisYear, onselect }: Props = $props();

  const burn = $derived(report.burn);
  const b = $derived(report.burndown);
  const unit = $derived(report.unit);
  const options = $derived([
    ...(burn ? [] : [{ value: "", label: "—" }]),
    ...report.sprints.map((s) => ({ value: s.id, label: s.name })),
  ]);
  const totals = $derived(burnTotals(b, unit));
  const columns = $derived([
    "日付",
    heading("総量", unit),
    heading("残り", unit),
    heading("理想", unit),
  ]);
  const rows = $derived(
    b.days.map((d) => [
      formatDay(d.date, thisYear),
      num(d.total, unit),
      d.remaining === null ? "—" : num(d.remaining, unit),
      num(d.ideal, unit),
    ]),
  );
</script>

<section aria-label="バーンダウン">
  <header>
    <h2>バーンダウン</h2>
    <Select
      label="スプリントを選択"
      {options}
      value={burn?.id ?? ""}
      onchange={(v) => onselect({ sprint: v })}
    />
    {#if burn}
      <span class="status">{burnStatus(burn, today)}</span>
    {/if}
  </header>
  {#if !burn}
    <p class="empty">スプリントを選んでください</p>
  {:else if b.shown === 0}
    <p class="empty">{notStarted(burn.start, thisYear)}</p>
  {:else}
    <p class="totals"><span>{totals.total}</span><span>{totals.remaining}</span></p>
    <LineChart days={b.days} {unit} {thisYear} />
    {#if b.committed !== null}
      <p class="empty">{committedNote(b.committed, unit)}</p>
    {/if}
    <DataTable caption="バーンダウンの表" {columns} {rows} />
  {/if}
</section>

<style>
  section {
    margin-bottom: 28px;
  }
  header {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  h2 {
    font-size: 15px;
    margin: 0;
  }
  .status {
    font-size: 12px;
    color: var(--muted);
  }
  .totals {
    display: flex;
    gap: 16px;
    margin: 12px 0 0;
    font-size: 14px;
  }
  .empty {
    margin: 12px 0 0;
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
