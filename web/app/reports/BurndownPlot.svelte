<script lang="ts">
  import { shortDate } from "../../core/gantt";
  import { amountLabel, type BurndownView, burndownPlot } from "../../core/reports";
  import Plot from "./Plot.svelte";

  type Props = { view: BurndownView; height?: number };
  let { view, height }: Props = $props();

  const plot = $derived(burndownPlot(view));
  const cell = (n: number | null | undefined) =>
    n === null || n === undefined ? "—" : amountLabel(n, view.by);
  const table = $derived({
    head: ["日付", "残り", "総量", "理想"],
    rows: plot.xs.map((x, i) => [
      shortDate(x),
      cell(plot.remaining[i]),
      cell(plot.scope[i]),
      cell(plot.ideal[i]),
    ]),
  });
</script>

<Plot
  label="{view.sprint.name} のバーンダウン"
  xs={plot.xs}
  xLabel={shortDate}
  {height}
  {table}
  lines={[
    { label: "総量", values: plot.scope, color: "ink", thin: true, stepped: true },
    { label: "理想", values: plot.ideal, color: "muted-2", dashed: true },
    { label: "残り", values: plot.remaining, color: "accent", dots: true },
  ]}
/>
