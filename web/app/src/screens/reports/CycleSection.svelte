<script lang="ts">
  import { durationLines, type Report } from "@hedgehug/core";

  // サイクルタイムとリードタイム(05「サイクルタイムとリードタイム」)。単位によらず、日数で書く。
  interface Props {
    report: Report;
  }

  let { report }: Props = $props();

  const lines = $derived([
    durationLines(report.cycle, "cycle"),
    durationLines(report.lead, "lead"),
  ]);
</script>

<section aria-label="サイクルタイムとリードタイム">
  <h2>サイクルタイムとリードタイム</h2>
  {#each lines as line (line.label)}
    <p class="line">
      <span class="label">{line.label}</span>
      <strong>{line.value}</strong>
      {#if line.p85}
        <span class="p85">{line.p85}</span>
      {/if}
    </p>
  {/each}
</section>

<style>
  section {
    margin-bottom: 28px;
  }
  h2 {
    font-size: 15px;
    margin: 0;
  }
  .line {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 4px 12px;
    margin: 8px 0 0;
    font-size: 13px;
  }
  .label,
  .p85 {
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
