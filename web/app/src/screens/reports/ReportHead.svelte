<script lang="ts">
  import { fallbackNote, type Report, type ReportSelection } from "@hedgehug/core";
  import { Select } from "@hedgehug/ui";

  // レポートの冒頭: 集計の説明、単位のセレクト、ポイントがないときの注記(05「レポート」)。
  interface Props {
    report: Report;
    onselect: (patch: Partial<ReportSelection>) => void;
  }

  let { report, onselect }: Props = $props();

  const units = [
    { value: "points", label: "ポイント" },
    { value: "count", label: "件数" },
  ];
</script>

<p class="note">git 履歴から集計しています。</p>
<div class="unit">
  <Select
    label="単位"
    options={units}
    value={report.selection.unit}
    onchange={(v) => onselect({ unit: v === "count" ? "count" : "points" })}
  />
  {#if report.fallback}
    <span class="note">{fallbackNote}</span>
  {/if}
</div>

<style>
  .note {
    margin: 0 0 8px;
    font-size: 12.5px;
    color: var(--muted);
  }
  .unit {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
  }
  .unit .note {
    margin: 0;
  }
</style>
