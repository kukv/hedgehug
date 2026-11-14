<script lang="ts">
  import { pointsCard, type ReportActive } from "@hedgehug/core";

  // 概要の統計のカード「{スプリント名} の完了ポイント」(04「統計」、F3-18)。
  interface Props {
    active: ReportActive | null;
  }

  let { active }: Props = $props();

  const card = $derived(pointsCard(active));
</script>

<article class="card" aria-label={card.heading}>
  <h3>{card.heading}</h3>
  <p class="value">{card.value}</p>
</article>

<style>
  .card {
    padding: 12px 14px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--surface);
  }
  h3 {
    margin: 0;
    font-size: 12px;
    font-weight: 500;
    color: var(--muted);
  }
  .value {
    margin: 4px 0 0;
    font-size: 20px;
    font-weight: 600;
  }
</style>
