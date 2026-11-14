<script lang="ts">
  import {
    checkPeriod,
    defaultPeriod,
    formatDay,
    periodLabel,
    type Report,
    type ReportLayers,
  } from "@hedgehug/core";
  import { Button, Input } from "@hedgehug/ui";
  import { reports } from "../../reports";
  import AreaChart from "./AreaChart.svelte";
  import DataTable from "./DataTable.svelte";

  // 累積フロー図(05「累積フロー図」)。期間は保存しない。入力が合わないときは理由を出し、図は前の期間のまま。
  interface Props {
    report: Report;
    today: string;
    thisYear: number;
    onlayers: (layers: ReportLayers) => void;
  }

  let { report, today, thisYear, onlayers }: Props = $props();

  // custom は、初めの期間でない期間(null は直近 21 日)。入力欄は、検査に落ちた値も残す。
  let custom: { from: string; to: string } | null = $state(null);
  let from = $state("");
  let to = $state("");
  let problem: string | null = $state(null);

  // 初めの期間のあいだは、読み込んだ期間(日付が変わって読み直した期間も)に入力欄を合わせる。
  $effect(() => {
    if (custom !== null) return;
    from = report.period.from;
    to = report.period.to;
    problem = null;
  });

  async function change() {
    problem = checkPeriod(from, to, today);
    if (problem !== null) return;
    const first = defaultPeriod(today);
    custom = from === first.from && to === first.to ? null : { from, to };
    await reports.setPeriod(custom);
  }

  async function reset() {
    custom = null;
    problem = null;
    await reports.setPeriod(null);
  }

  const columns = $derived(["日付", ...report.layers.map((l) => l.name)]);
  const rows = $derived(
    report.flow.map((d) => [formatDay(d.date, thisYear), ...d.counts.map(String)]),
  );
</script>

<section aria-label="累積フロー図">
  <header>
    <h2>累積フロー図</h2>
    <span class="label">{periodLabel(custom, thisYear)}</span>
    <div class="controls">
      <Input
        type="date"
        label="開始"
        size="sm"
        value={from}
        oninput={(v) => {
          from = v;
          void change();
        }}
      />
      <Input
        type="date"
        label="終了"
        size="sm"
        value={to}
        oninput={(v) => {
          to = v;
          void change();
        }}
      />
      <Button
        size="sm"
        pressed={report.selection.layers === "category"}
        onclick={() => onlayers("category")}
        >分類</Button
      >
      <Button
        size="sm"
        pressed={report.selection.layers === "status"}
        onclick={() => onlayers("status")}
        >ステータス</Button
      >
    </div>
  </header>
  {#if custom !== null}
    <p><Button size="xs" variant="ghost" onclick={reset}>直近 21 日に戻す</Button></p>
  {/if}
  {#if problem}
    <p class="problem" role="alert">{problem}</p>
  {/if}
  <AreaChart layers={report.layers} flow={report.flow} {thisYear} />
  <DataTable caption="累積フロー図の表" {columns} {rows} />
</section>

<style>
  section {
    margin-bottom: 28px;
  }
  header {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px;
  }
  h2 {
    font-size: 15px;
    margin: 0;
  }
  .label {
    font-size: 12px;
    color: var(--muted);
  }
  .controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-left: auto;
  }
  p {
    margin: 8px 0 0;
  }
  .problem {
    font-size: 12.5px;
    color: var(--danger);
  }
</style>
