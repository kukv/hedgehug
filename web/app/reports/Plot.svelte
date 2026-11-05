<script lang="ts" module>
  // A line of a plot. color is the name of a token, as a canvas does not read var(). fill paints
  // from the line down to 0; stepped holds the value until it changes; bars draws columns.
  export type PlotLine = {
    label: string;
    values: (number | null)[];
    color: string;
    dashed?: boolean;
    thin?: boolean;
    fill?: boolean;
    stepped?: boolean;
    bars?: boolean;
    // A dot on each value: one day of a report has no line to see.
    dots?: boolean;
  };
  export type PlotTable = { head: string[]; rows: string[][] };
</script>

<script lang="ts">
  import type uPlot from "uplot";

  // A chart of uPlot, and the same numbers as a table: a canvas says nothing to a screen reader.
  type Props = {
    label: string;
    xs: number[];
    lines: PlotLine[];
    xLabel: (x: number) => string;
    table: PlotTable;
    height?: number;
    legend?: boolean;
  };
  let { label, xs, lines, xLabel, table, height = 210, legend = true }: Props = $props();

  let box = $state<HTMLElement>();

  $effect(() => {
    const el = box;
    if (!el || xs.length === 0) return;
    const spec = { xs, lines, xLabel, height };
    let chart: uPlot | undefined;
    let observer: ResizeObserver | undefined;
    let live = true;

    void Promise.all([import("uplot"), import("uplot/dist/uPlot.min.css")]).then(
      ([{ default: UPlot }]) => {
        if (!live) return;
        const token = (name: string) => getComputedStyle(el).getPropertyValue(`--${name}`).trim();
        const axisColor = token("muted");
        const grid = { stroke: token("line-soft"), width: 1 };
        const font = "11px sans-serif";
        const hasBars = spec.lines.some((l) => l.bars);
        const series = spec.lines.map((l): uPlot.Series => {
          const color = token(l.color);
          return {
            label: l.label,
            stroke: color,
            width: l.bars ? 0 : l.thin ? 1 : 2,
            dash: l.dashed ? [5, 4] : undefined,
            fill: l.fill || l.bars ? color : undefined,
            points: l.dots ? { show: true, size: 6 } : { show: false },
            spanGaps: false,
            paths: l.bars
              ? UPlot.paths.bars?.({ size: [0.6, 64] })
              : l.stepped
                ? UPlot.paths.stepped?.({ align: 1 })
                : undefined,
          };
        });
        chart = new UPlot(
          {
            width: el.clientWidth,
            height: spec.height,
            legend: { show: false },
            cursor: { show: false },
            padding: [8, 12, 0, 0],
            scales: {
              x: {
                time: false,
                ...(hasBars
                  ? { range: (): uPlot.Range.MinMax => [-0.5, spec.xs.length - 0.5] }
                  : {}),
              },
              y: { range: (_u, _min, max): uPlot.Range.MinMax => [0, Math.max(max, 1) * 1.1] },
            },
            axes: [
              {
                stroke: axisColor,
                font,
                grid: { show: false },
                ticks: { show: false },
                incrs: [1, 2, 3, 5, 7, 14, 30, 60, 90],
                values: (_u, splits) => splits.map((x) => spec.xLabel(x)),
              },
              { stroke: axisColor, font, grid, ticks: { show: false } },
            ],
            series: [{}, ...series],
          },
          [spec.xs, ...spec.lines.map((l) => l.values)] as uPlot.AlignedData,
          el,
        );
        observer = new ResizeObserver(() => {
          chart?.setSize({ width: el.clientWidth, height: spec.height });
        });
        observer.observe(el);
      },
    );
    return () => {
      live = false;
      observer?.disconnect();
      chart?.destroy();
    };
  });
</script>

<figure class="plot">
  {#if legend}
    <figcaption>
      {#each lines as l (l.label)}
        <span>
          <i
            data-dashed={l.dashed || undefined}
            data-thin={l.thin || undefined}
            style:--swatch="var(--{l.color})"
          ></i>{l.label}
        </span>
      {/each}
    </figcaption>
  {/if}
  <div
    class="box"
    bind:this={box}
    role="img"
    aria-label={label}
    style:min-height="{height}px"
  ></div>
  <details>
    <summary>表で見る</summary>
    <table>
      <caption class="sr">
        {label}
      </caption>
      <thead>
        <tr>
          {#each table.head as h (h)}
            <th scope="col">{h}</th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each table.rows as row, i (i)}
          <tr>
            {#each row as cell, j (j)}
              <td>{cell}</td>
            {/each}
          </tr>
        {/each}
      </tbody>
    </table>
  </details>
</figure>

<style>
  .plot {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    margin: 0;
    min-width: 0;
  }
  figcaption {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-6) 14px;
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  figcaption span {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
  }
  figcaption i {
    width: 16px;
    border-top: 2px solid var(--swatch);
  }
  figcaption i[data-dashed] {
    border-top-style: dashed;
  }
  figcaption i[data-thin] {
    border-top-width: 1px;
  }
  .box {
    min-width: 0;
  }
  details {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  table {
    margin-top: var(--space-6);
    border-collapse: collapse;
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
  }
  th,
  td {
    padding: 2px var(--space-8);
    border-bottom: 1px solid var(--line-soft);
    text-align: left;
    font-weight: 400;
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
</style>
